import { useEffect, useMemo, useRef, useState } from 'react'
import { Pressable, StyleSheet, View, type GestureResponderEvent, type LayoutChangeEvent } from 'react-native'
import { colors } from '../helpers/theme'
import { interpolateWagonActor, wagonSituationIcon } from '../helpers/wagonMap'
import type { WagonAnchor, WagonSnapshot } from '../types'
import { Text } from './Typography'

interface WagonWorldProps {
  snapshot: WagonSnapshot
  disabled?: boolean
  onAnchorPress: (anchor: WagonAnchor) => void
}

const anchorLayout: Record<WagonAnchor, { left: `${number}%`; top: `${number}%` }> = {
  staff_zone: { left: '50%', top: '5%' },
  seat_1: { left: '25%', top: '23%' }, seat_2: { left: '75%', top: '23%' },
  seat_3: { left: '25%', top: '43%' }, seat_4: { left: '75%', top: '43%' },
  seat_5: { left: '25%', top: '63%' }, seat_6: { left: '75%', top: '63%' },
  service_point: { left: '50%', top: '88%' },
}

const conductorLayout: Record<WagonAnchor, { left: `${number}%`; top: `${number}%` }> = {
  staff_zone: { left: '50%', top: '5%' }, service_point: { left: '50%', top: '88%' },
  seat_1: { left: '50%', top: '23%' }, seat_2: { left: '50%', top: '23%' },
  seat_3: { left: '50%', top: '43%' }, seat_4: { left: '50%', top: '43%' },
  seat_5: { left: '50%', top: '63%' }, seat_6: { left: '50%', top: '63%' },
}

function actorLayout(actorState: WagonSnapshot['wagon_state']['player']) {
  if (!actorState.moving) return anchorLayout[actorState.at]
  const progress = interpolateWagonActor(actorState).progress
  const from = anchorLayout[actorState.moving.from]
  const to = anchorLayout[actorState.moving.to]
  const left = Number.parseFloat(from.left) + (Number.parseFloat(to.left) - Number.parseFloat(from.left)) * progress
  const top = Number.parseFloat(from.top) + (Number.parseFloat(to.top) - Number.parseFloat(from.top)) * progress
  return { left: `${left}%` as `${number}%`, top: `${top}%` as `${number}%` }
}

function conductorActorLayout(actorState: WagonSnapshot['wagon_state']['player']) {
  if (!actorState.moving) return conductorLayout[actorState.at]
  const progress = interpolateWagonActor(actorState).progress
  const from = conductorLayout[actorState.moving.from]
  const to = conductorLayout[actorState.moving.to]
  const left = Number.parseFloat(from.left) + (Number.parseFloat(to.left) - Number.parseFloat(from.left)) * progress
  const top = Number.parseFloat(from.top) + (Number.parseFloat(to.top) - Number.parseFloat(from.top)) * progress
  return { left: `${left}%` as `${number}%`, top: `${top}%` as `${number}%` }
}

export function WagonWorld({ snapshot, disabled, onAnchorPress }: WagonWorldProps) {
  const [, tick] = useState(0)
  const [sceneHeight, setSceneHeight] = useState(1)
  const [freeTop, setFreeTop] = useState(() => Number.parseFloat(conductorLayout[snapshot.wagon_state.player.at].top))
  const [freeTarget, setFreeTarget] = useState<number | null>(null)
  const [freeMode, setFreeMode] = useState(false)
  const wasServerMoving = useRef(false)
  const actorsMoving = Boolean(snapshot.wagon_state.player.moving || snapshot.wagon_state.seats.some((seat) => seat.actor.moving))
  useEffect(() => {
    if (!actorsMoving && freeTarget === null) return
    const timer = setInterval(() => {
      tick((value) => value + 1)
      if (freeTarget !== null) setFreeTop((current) => {
        const distance = freeTarget - current
        if (Math.abs(distance) < 0.4) return freeTarget
        return current + Math.sign(distance) * Math.min(Math.abs(distance), 2.2)
      })
    }, 50)
    return () => clearInterval(timer)
  }, [actorsMoving, freeTarget])
  useEffect(() => {
    if (freeTarget !== null && Math.abs(freeTarget - freeTop) < 0.4) setFreeTarget(null)
  }, [freeTarget, freeTop])
  useEffect(() => {
    const moving = Boolean(snapshot.wagon_state.player.moving)
    if (wasServerMoving.current && !moving) {
      setFreeMode(false)
      setFreeTarget(null)
      setFreeTop(Number.parseFloat(conductorLayout[snapshot.wagon_state.player.at].top))
    }
    wasServerMoving.current = moving
  }, [snapshot.wagon_state.player.at, snapshot.wagon_state.player.moving])
  const situations = useMemo(() => new Map(snapshot.active_situations.map((item) => [item.seat_anchor, item.type])), [snapshot.active_situations])
  const authoritativePlayer = conductorActorLayout(snapshot.wagon_state.player)
  const player = freeMode ? { left: '50%' as const, top: `${freeTop}%` as `${number}%` } : authoritativePlayer
  const handleAislePress = (event: GestureResponderEvent) => {
    const percent = (event.nativeEvent.locationY / sceneHeight) * 100
    if (!freeMode) setFreeTop(Number.parseFloat(authoritativePlayer.top))
    setFreeMode(true)
    setFreeTarget(Math.max(3, Math.min(94, percent)))
  }
  const handleAnchorPress = (anchor: WagonAnchor) => {
    const active = snapshot.active_situations.find((item) => item.seat_anchor === anchor)
    if (!active && anchor !== 'service_point') return
    const seat = active ? snapshot.wagon_state.seats.find((item) => item.anchor === anchor) : undefined
    const targetAnchor = seat?.actor.at ?? anchor
    if (!freeMode) setFreeTop(Number.parseFloat(authoritativePlayer.top))
    setFreeMode(true)
    setFreeTarget(Number.parseFloat(conductorLayout[targetAnchor].top))
    onAnchorPress(anchor)
  }
  const handleLayout = (event: LayoutChangeEvent) => setSceneHeight(Math.max(1, event.nativeEvent.layout.height))
  return (
    <View style={styles.scene} onLayout={handleLayout}>
      <View style={styles.windowLeft} /><View style={styles.windowRight} />
      <Pressable disabled={disabled} onPress={handleAislePress} style={styles.aisle} />
      {snapshot.wagon_state.seats.map((seat) => <Pressable key={`chair-${seat.anchor}`} disabled={disabled} onPress={() => handleAnchorPress(seat.anchor)} style={[styles.seat, anchorLayout[seat.anchor]]}><View style={styles.chair} /></Pressable>)}
      {snapshot.wagon_state.seats.map((seat, index) => {
        const situation = situations.get(seat.anchor)
        return <Pressable key={`actor-${seat.anchor}`} disabled={disabled} onPress={() => handleAnchorPress(seat.anchor)} style={[styles.passengerMover, actorLayout(seat.actor)]}><View style={[styles.passenger, { backgroundColor: ['#3158CC', '#16A085', '#F59E0B', '#E83343', '#8B5CF6', '#0EA5E9'][index] }]} />{situation && <View style={styles.signal}><Text style={styles.signalText}>{wagonSituationIcon(situation)}</Text></View>}</Pressable>
      })}
      <Pressable disabled={disabled} onPress={() => handleAnchorPress('staff_zone')} style={[styles.zone, styles.staff, anchorLayout.staff_zone]}><Text style={styles.zoneText}>СЛУЖЕБНАЯ</Text></Pressable>
      <Pressable disabled={disabled} onPress={() => handleAnchorPress('service_point')} style={[styles.zone, anchorLayout.service_point]}><Text style={styles.zoneText}>СЕРВИС +</Text></Pressable>
      <View pointerEvents="none" style={[styles.player, player]}><View style={styles.cap} /><View style={styles.body} /></View>
      <View pointerEvents="none" style={styles.tip}><Text style={styles.tipText}>Нажмите на пассажира или точку вагона</Text></View>
    </View>
  )
}

const styles = StyleSheet.create({
  scene: { flex: 1, overflow: 'hidden', backgroundColor: '#E5D9C8', borderLeftWidth: 14, borderRightWidth: 14, borderColor: '#C5B29C' },
  aisle: { position: 'absolute', top: 0, bottom: 0, left: '42%', width: '16%', backgroundColor: '#8E2630', borderLeftWidth: 4, borderRightWidth: 4, borderColor: '#C2474D' },
  windowLeft: { position: 'absolute', left: 0, top: '12%', bottom: '12%', width: 12, backgroundColor: '#8ED4F2' },
  windowRight: { position: 'absolute', right: 0, top: '12%', bottom: '12%', width: 12, backgroundColor: '#8ED4F2' },
  seat: { position: 'absolute', width: 104, height: 86, marginLeft: -52, marginTop: -43, alignItems: 'center', justifyContent: 'center' },
  chair: { width: 76, height: 70, borderRadius: 18, backgroundColor: '#EEE6DB', borderWidth: 7, borderColor: '#816B60', alignItems: 'center', justifyContent: 'center' },
  passengerMover: { position: 'absolute', width: 46, height: 56, marginLeft: -23, marginTop: -28, alignItems: 'center', justifyContent: 'center' },
  passenger: { width: 34, height: 42, borderRadius: 18 },
  signal: { position: 'absolute', right: 0, top: -4, width: 38, height: 38, borderRadius: 19, backgroundColor: '#FFF', borderWidth: 2, borderColor: colors.loyalty, alignItems: 'center', justifyContent: 'center' },
  signalText: { fontSize: 19 }, zone: { position: 'absolute', width: 108, height: 42, marginLeft: -54, marginTop: -21, borderRadius: 14, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.primary },
  staff: { backgroundColor: colors.critical }, zoneText: { color: '#FFF', fontSize: 11, fontWeight: '900' },
  player: { position: 'absolute', width: 34, height: 54, marginLeft: -17, marginTop: -27, alignItems: 'center' },
  cap: { width: 27, height: 15, borderRadius: 12, backgroundColor: colors.primary, borderBottomWidth: 4, borderColor: '#EFBF3A' },
  body: { width: 31, height: 38, borderRadius: 10, backgroundColor: colors.primaryDark },
  tip: { position: 'absolute', left: 20, right: 20, bottom: 16, alignItems: 'center' },
  tipText: { color: '#FFF', fontSize: 12, fontWeight: '700', backgroundColor: 'rgba(16,26,61,.75)', paddingHorizontal: 14, paddingVertical: 8, borderRadius: 99 },
})
