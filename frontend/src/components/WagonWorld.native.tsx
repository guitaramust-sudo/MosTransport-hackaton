import { Suspense, useEffect, useMemo, useRef, useState, type MutableRefObject } from 'react'
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native'
import { Canvas, useFrame, useThree } from '@react-three/fiber/native'
import { OrthographicCamera, useAnimations, useGLTF } from '@react-three/drei/native'
import { Mesh, Vector3, type AnimationAction, type AnimationClip, type Group, type OrthographicCamera as ThreeOrthographicCamera } from 'three'
import { conductorAsset, wagonAsset } from '../helpers/gameAssets'
import { useNativeWagonScene } from '../helpers/useNativeWagonScene'
import { colors } from '../helpers/theme'
import { interpolateWagonActor, wagonAnchorPositions, wagonSituationIcon } from '../helpers/wagonMap'
import type { WagonActor, WagonAnchor, WagonSeat, WagonSituationType, WagonSnapshot } from '../types'
import { Text } from './Typography'

useGLTF.preload(wagonAsset)
useGLTF.preload(conductorAsset)

interface WagonWorldProps {
  snapshot: WagonSnapshot
  disabled?: boolean
  onAnchorPress: (anchor: WagonAnchor) => void
}

interface PlayerPosition { x: number; y: number; z: number }
interface Marker { anchor: WagonAnchor; x: number; y: number; visible: boolean; icon: string; active: boolean }
interface FreeWalkTarget { x: number; z: number; request: number }
const AISLE_X = 0.47
const FLOOR_Y = 0.245
const WALK_SPEED = 2.6

function conductorPoint(anchor: WagonAnchor) {
  const point = wagonAnchorPositions[anchor]
  return { x: AISLE_X, y: FLOOR_Y, z: point.z }
}

function interpolateConductor(actor: WagonActor) {
  if (!actor.moving) return { ...conductorPoint(actor.at), progress: 1 }
  const timing = interpolateWagonActor(actor)
  const from = conductorPoint(actor.moving.from)
  const to = conductorPoint(actor.moving.to)
  return {
    x: from.x + (to.x - from.x) * timing.progress,
    y: FLOOR_Y,
    z: from.z + (to.z - from.z) * timing.progress,
    progress: timing.progress,
  }
}

function Passenger({ seat, type, onPress }: { seat: WagonSeat; type?: WagonSituationType; onPress: () => void }) {
  const group = useRef<Group>(null)
  const head = useRef<Group>(null)
  const color = useMemo(() => {
    const palette = ['#3158CC', '#16A085', '#F59E0B', '#E83343', '#8B5CF6', '#0EA5E9']
    const score = [...seat.passenger_def_id].reduce((sum, char) => sum + char.charCodeAt(0), 0)
    return palette[score % palette.length]
  }, [seat.passenger_def_id])

  useFrame(({ clock }) => {
    if (!group.current) return
    const position = interpolateWagonActor(seat.actor)
    group.current.position.set(position.x, position.y, position.z)
    const seated = seat.actor.at.startsWith('seat_') && !seat.actor.moving
    group.current.rotation.y = seated ? (seat.anchor.endsWith('1') || seat.anchor.endsWith('3') || seat.anchor.endsWith('5') ? Math.PI / 2 : -Math.PI / 2) : 0
    group.current.rotation.z = type === 'cold' ? Math.sin(clock.elapsedTime * 24) * 0.035 : 0
    if (head.current) head.current.position.y = 1.19 + (type === 'tired' ? Math.sin(clock.elapsedTime * 3) * 0.07 : 0)
  })

  return (
    <group ref={group} onPointerDown={(event) => { event.stopPropagation(); onPress() }}>
      <mesh position={[0, 0.76, 0]}><capsuleGeometry args={[0.18, 0.36, 6, 12]} /><meshStandardMaterial color={color} /></mesh>
      <group ref={head} position={[0, 1.19, 0]}>
        <mesh><sphereGeometry args={[0.19, 20, 20]} /><meshStandardMaterial color="#F1BE94" /></mesh>
        <mesh position={[0, 0.09, -0.08]}><sphereGeometry args={[0.2, 16, 12, 0, Math.PI * 2, 0, Math.PI / 2]} /><meshStandardMaterial color="#27344B" /></mesh>
      </group>
      <mesh position={[0, 0.58, 0]}><sphereGeometry args={[0.42, 12, 12]} /><meshBasicMaterial transparent opacity={0} /></mesh>
      {type && <mesh position={[0, 1.58, 0]}><sphereGeometry args={[0.075, 16, 16]} /><meshStandardMaterial color={type === 'zone_intrusion' ? colors.critical : colors.loyalty} emissive={type === 'zone_intrusion' ? colors.critical : colors.primary} emissiveIntensity={1.1} /></mesh>}
    </group>
  )
}

function Conductor({ actor, freeTarget, playerPosition }: { actor: WagonActor; freeTarget: FreeWalkTarget | null; playerPosition: MutableRefObject<PlayerPosition> }) {
  const model = useGLTF(conductorAsset) as unknown as { scene: Group; animations: AnimationClip[] }
  const group = useRef<Group>(null)
  const previousMoving = useRef(false)
  const visualPosition = useRef<PlayerPosition>(interpolateConductor(actor))
  const localTarget = useRef<PlayerPosition>(visualPosition.current)
  const handledFreeRequest = useRef(0)
  const localMode = useRef(false)
  const serverMoveKey = useRef('')
  const serverTransition = useRef<{ from: PlayerPosition; to: PlayerPosition; startedAt: number; durationMs: number } | null>(null)
  const { actions } = useAnimations(model.animations, model.scene) as unknown as { actions: Record<string, AnimationAction | null> }

  useEffect(() => {
    model.scene.scale.setScalar(1.35)
    model.scene.traverse((object) => { if (object instanceof Mesh) object.frustumCulled = false })
    actions.Idle?.reset().play()
    return () => Object.values(actions).forEach((action) => action?.stop())
  }, [actions, model.scene])

  useFrame((_, delta) => {
    if (!group.current) return
    let moving = false

    if (actor.moving) {
      const key = `${actor.moving.from}:${actor.moving.to}:${actor.moving.started_at}`
      if (serverMoveKey.current !== key) {
        serverMoveKey.current = key
        localMode.current = false
        const to = conductorPoint(actor.moving.to)
        const serverEnd = Date.parse(actor.moving.started_at) + actor.moving.duration_s * 1000
        serverTransition.current = {
          from: { ...visualPosition.current },
          to,
          startedAt: Date.now(),
          durationMs: Math.max(120, serverEnd - Date.now()),
        }
      }
      const transition = serverTransition.current
      if (transition) {
        const progress = Math.max(0, Math.min(1, (Date.now() - transition.startedAt) / transition.durationMs))
        visualPosition.current = {
          x: transition.from.x + (transition.to.x - transition.from.x) * progress,
          y: FLOOR_Y,
          z: transition.from.z + (transition.to.z - transition.from.z) * progress,
        }
        group.current.rotation.y = Math.atan2(transition.to.x - visualPosition.current.x, transition.to.z - visualPosition.current.z)
        moving = progress < 1
      }
    } else {
      if (serverMoveKey.current) {
        serverMoveKey.current = ''
        serverTransition.current = null
        visualPosition.current = conductorPoint(actor.at)
      }
      if (freeTarget && freeTarget.request !== handledFreeRequest.current) {
        handledFreeRequest.current = freeTarget.request
        localTarget.current = { x: freeTarget.x, y: FLOOR_Y, z: freeTarget.z }
        localMode.current = true
      }
      if (localMode.current) {
        const dx = localTarget.current.x - visualPosition.current.x
        const dz = localTarget.current.z - visualPosition.current.z
        const distance = Math.hypot(dx, dz)
        if (distance > 0.025) {
          const step = Math.min(distance, delta * WALK_SPEED)
          visualPosition.current.x += (dx / distance) * step
          visualPosition.current.z += (dz / distance) * step
          group.current.rotation.y = Math.atan2(dx, dz)
          moving = true
        } else {
          visualPosition.current = { ...localTarget.current }
        }
      } else {
        visualPosition.current = conductorPoint(actor.at)
      }
    }

    group.current.position.set(visualPosition.current.x, visualPosition.current.y, visualPosition.current.z)
    playerPosition.current = { ...visualPosition.current }
    if (moving !== previousMoving.current) {
      previousMoving.current = moving
      if (moving) { actions.Idle?.fadeOut(0.15); actions.Walk?.reset().fadeIn(0.15).play() }
      else { actions.Walk?.fadeOut(0.15); actions.Idle?.reset().fadeIn(0.15).play() }
    }
  })
  return <group ref={group}><primitive object={model.scene} /></group>
}

function CameraFollow({ playerPosition }: { playerPosition: MutableRefObject<PlayerPosition> }) {
  const { size } = useThree()
  const zoom = size.width / 3.75
  const camera = useRef<ThreeOrthographicCamera>(null)
  const desiredPosition = useRef(new Vector3())
  const lookAt = useRef(new Vector3())

  useFrame((_, delta) => {
    if (!camera.current) return

    const followedZ = Math.max(-4.5, Math.min(4.5, playerPosition.current.z))
    const smoothing = 1 - Math.exp(-6 * delta)
    desiredPosition.current.set(3.2, 18, followedZ + 9.4)
    camera.current.position.lerp(desiredPosition.current, smoothing)
    lookAt.current.set(0, FLOOR_Y, followedZ)
    camera.current.lookAt(lookAt.current)
    camera.current.updateProjectionMatrix()
  })

  return <OrthographicCamera ref={camera} makeDefault position={[0, 18, 10]} zoom={zoom} near={0.1} far={100} />
}

function MarkerProjector({ snapshot, onProject }: { snapshot: WagonSnapshot; onProject: (value: Marker[]) => void }) {
  const point = useRef(new Vector3())
  const previous = useRef('')
  useFrame(({ camera, size }) => {
    const situations = new Map(snapshot.active_situations.map((item) => [item.seat_anchor, item]))
    const anchors = [...snapshot.active_situations.map((item) => item.seat_anchor), 'service_point'] as WagonAnchor[]
    const markers = anchors.map((anchor) => {
      const seat = snapshot.wagon_state.seats.find((item) => item.anchor === anchor)
      const position = seat && situations.has(anchor) ? interpolateWagonActor(seat.actor) : wagonAnchorPositions[anchor]
      point.current.set(position.x, anchor.startsWith('seat_') ? 1.7 : 0.65, position.z).project(camera)
      const situation = situations.get(anchor)
      const x = Math.round((point.current.x * 0.5 + 0.5) * size.width)
      const y = Math.round((-point.current.y * 0.5 + 0.5) * size.height)
      return { anchor, x, y, visible: point.current.z > -1 && point.current.z < 1 && x > 18 && x < size.width - 18 && y > 150 && y < size.height - 86, icon: situation ? wagonSituationIcon(situation.type) : anchor === 'service_point' ? '+' : anchor === 'staff_zone' ? '⌁' : '•', active: Boolean(situation) }
    })
    const signature = markers.map((item) => `${item.anchor}:${item.x}:${item.y}:${item.visible}:${item.icon}`).join('|')
    if (signature !== previous.current) { previous.current = signature; onProject(markers) }
  })
  return null
}

function Scene({ snapshot, disabled, freeTarget, onAnchorPress, onFreeTarget, onProject }: WagonWorldProps & { freeTarget: FreeWalkTarget | null; onFreeTarget: (x: number, z: number) => void; onProject: (value: Marker[]) => void }) {
  const wagonScene = useNativeWagonScene()
  const playerPosition = useRef<PlayerPosition>(interpolateConductor(snapshot.wagon_state.player))
  const situations = new Map(snapshot.active_situations.map((item) => [item.seat_anchor, item.type]))
  return (
    <>
      <color attach="background" args={['#DCE8F4']} />
      <ambientLight intensity={2.1} />
      <directionalLight position={[4, 9, -2]} intensity={2.5} />
      <primitive object={wagonScene} />
      <mesh
        position={[AISLE_X, FLOOR_Y + 0.012, 0]}
        rotation={[-Math.PI / 2, 0, 0]}
        onPointerDown={(event) => {
          event.stopPropagation()
          if (disabled) return
          onFreeTarget(
            Math.max(0.16, Math.min(0.78, event.point.x)),
            Math.max(-6.2, Math.min(6.2, event.point.z)),
          )
        }}
      >
        <planeGeometry args={[0.62, 12.4]} />
        <meshBasicMaterial transparent opacity={0} depthWrite={false} />
      </mesh>
      {snapshot.wagon_state.seats.map((seat) => <Passenger key={seat.anchor} seat={seat} type={situations.get(seat.anchor)} onPress={() => { if (!disabled && situations.has(seat.anchor)) onAnchorPress(seat.anchor) }} />)}
      <Conductor actor={snapshot.wagon_state.player} freeTarget={freeTarget} playerPosition={playerPosition} />
      <MarkerProjector snapshot={snapshot} onProject={onProject} />
      <CameraFollow playerPosition={playerPosition} />
    </>
  )
}

export function WagonWorld({ snapshot, disabled, onAnchorPress }: WagonWorldProps) {
  const [markers, setMarkers] = useState<Marker[]>([])
  const [freeTarget, setFreeTarget] = useState<FreeWalkTarget | null>(null)
  const request = useRef(0)
  const setVisualTarget = (x: number, z: number) => {
    request.current += 1
    setFreeTarget({ x, z, request: request.current })
  }
  const handleAnchorPress = (anchor: WagonAnchor) => {
    const active = snapshot.active_situations.find((item) => item.seat_anchor === anchor)
    const seat = active ? snapshot.wagon_state.seats.find((item) => item.anchor === anchor) : undefined
    const target = conductorPoint(seat?.actor.at ?? anchor)
    setVisualTarget(target.x, target.z)
    onAnchorPress(anchor)
  }
  return (
    <View style={styles.container}>
      <Canvas style={styles.canvas} shadows gl={{ antialias: false }}>
        <Suspense fallback={null}><Scene snapshot={snapshot} disabled={disabled} freeTarget={freeTarget} onAnchorPress={handleAnchorPress} onFreeTarget={setVisualTarget} onProject={setMarkers} /></Suspense>
      </Canvas>
      {markers.map((marker) => marker.visible && (
        <Pressable key={marker.anchor} disabled={disabled} onPress={() => handleAnchorPress(marker.anchor)} style={[styles.marker, marker.active && styles.markerActive, marker.anchor === 'staff_zone' && styles.markerRestricted, { left: marker.x - 20, top: marker.y - 20 }]}>
          <Text style={[styles.markerText, !marker.active && styles.markerTextQuiet]}>{marker.icon}</Text>
        </Pressable>
      ))}
      <View pointerEvents="none" style={styles.tip}><Text style={styles.tipText}>Нажмите на пассажира или точку вагона</Text></View>
      {!snapshot && <ActivityIndicator style={StyleSheet.absoluteFill} color={colors.primary} />}
    </View>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, overflow: 'hidden', backgroundColor: '#DCE8F4' }, canvas: { flex: 1 },
  marker: { position: 'absolute', width: 40, height: 40, borderRadius: 20, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,.88)', borderWidth: 2, borderColor: colors.primary, shadowColor: '#06113F', shadowOpacity: .2, shadowRadius: 5, elevation: 5 },
  markerActive: { width: 46, height: 46, borderRadius: 23, marginLeft: -3, marginTop: -3, backgroundColor: '#FFFFFF', borderWidth: 3, borderColor: colors.loyalty },
  markerRestricted: { borderColor: colors.critical }, markerText: { fontSize: 20, color: colors.primary, fontWeight: '900' }, markerTextQuiet: { fontSize: 16 },
  tip: { position: 'absolute', left: 20, right: 20, bottom: 18, alignItems: 'center' },
  tipText: { color: '#FFFFFF', fontSize: 12, fontWeight: '700', backgroundColor: 'rgba(16,26,61,.72)', paddingHorizontal: 14, paddingVertical: 8, borderRadius: 99 },
})
