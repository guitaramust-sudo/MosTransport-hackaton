import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useAudioPlayer } from 'expo-audio'
import { useCallback, useEffect, useRef, useState } from 'react'
import { ActivityIndicator, KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, View } from 'react-native'
import { api, getWagonWebSocketUrl } from '../api/client'
import { lessonPracticeFinished, navigate, setWagonBreakdown, setWagonConnection, setWagonSelectedSituation, updateWagonSnapshot, useAppDispatch, useAppSelector } from '../app/store'
import { Icon, type IconName } from '../components/Icon'
import { Text, TextInput } from '../components/Typography'
import { WagonWorld } from '../components/WagonWorld'
import { colors, radius, shadow } from '../helpers/theme'
import { translateBackendField, translateBackendText } from '../helpers/backendTranslations'
import { objectsAtAnchor, pointsOfInterestFor, servicePointFor, wagonAnchorLabels, wagonItemLabels, wagonObjectLabels, wagonSituationIcon } from '../helpers/wagonMap'
import type { SessionResponse, WagonActiveSituation, WagonAnchor, WagonError, WagonItem, WagonSnapshot } from '../types'

const INVENTORY_SLOTS = 3

function itemIcon(item: WagonItem): IconName {
  return item === 'blanket' ? 'blanket' : item === 'coffee' ? 'coffee' : 'water'
}

type WagonCommand =
  | { type: 'move_to'; anchor: WagonAnchor }
  | { type: 'pick_item'; item: WagonItem }
  | { type: 'give_item'; situation_id: string; item: WagonItem }
  | { type: 'redirect'; situation_id: string }
  | { type: 'visit'; anchor: WagonAnchor }
  | { type: 'inspect'; item: string }

const escalationTargets = [
  ['train_chief', 'Начальник поезда'], ['ptb', 'ПТБ'], ['police', 'Полиция'], ['medic', 'Медик'], ['ambulance', 'Скорая'],
] as const

function restoredSnapshot(response: SessionResponse): WagonSnapshot | null {
  const state = response.session.wagon_state
  if (!state) return null
  const gameTime = Math.max(0, Math.floor((Date.now() - Date.parse(state.started_at)) / 1000))
  return {
    type: 'state', game_time_s: gameTime, wagon_state: state,
    active_situations: response.situations.filter((item) => item.status === 'active' && item.seat_anchor).map((item) => ({ situation_id: item.id, seat_anchor: item.seat_anchor!, type: item.code, pool: 'active' })),
  }
}

function formatTime(seconds: number) {
  const safe = Math.max(0, Math.round(seconds))
  return `${Math.floor(safe / 60).toString().padStart(2, '0')}:${(safe % 60).toString().padStart(2, '0')}`
}

function connectionLabel(status: string) {
  if (status === 'connected') return 'В сети'
  if (status === 'reconnecting') return 'Переподключение…'
  if (status === 'error') return 'Нет связи'
  return 'Подключение…'
}

export function WagonPage() {
  const dispatch = useAppDispatch()
  const queryClient = useQueryClient()
  const musicPlayer = useAudioPlayer(require('../../assets/audio/wagon_ambient.mp3'))
  const [musicEnabled, setMusicEnabled] = useState(true)

  useEffect(() => {
    musicPlayer.loop = true
    // The recording is quiet as well, for browsers that ignore the volume API.
    musicPlayer.volume = 0.35
    if (Platform.OS === 'web' && typeof document !== 'undefined') {
      // Browsers allow sound after a gesture; starting the shift may be asynchronous.
      const startOnGesture = () => musicPlayer.play()
      document.addEventListener('pointerdown', startOnGesture, { once: true })
      return () => {
        document.removeEventListener('pointerdown', startOnGesture)
        musicPlayer.pause()
      }
    }
    musicPlayer.play()
    return () => musicPlayer.pause()
  }, [musicPlayer])

  const toggleMusic = () => {
    if (musicEnabled) musicPlayer.pause()
    else musicPlayer.play()
    setMusicEnabled(!musicEnabled)
  }
  const { wagonSessionId: sessionId, wagonWsPath: wsPath, wagonSnapshot: snapshot, wagonConnection: connection, wagonSelectedSituationId: selectedId, lessonId, lessonPracticeSessionId } = useAppSelector((state) => state.app)
  const isLessonPractice = Boolean(sessionId && lessonId && lessonPracticeSessionId === sessionId)
  const lesson = useQuery({ queryKey: ['lesson', lessonId], queryFn: () => api.lesson(lessonId!), enabled: isLessonPractice })
  const visitSent = useRef(new Set<string>())
  const socketRef = useRef<WebSocket | null>(null)
  const autoFinishStarted = useRef(false)
  const [pendingSituationId, setPendingSituationId] = useState<string | null>(null)
  const [serviceRequested, setServiceRequested] = useState(false)
  const [showService, setShowService] = useState(false)
  const [serviceContext, setServiceContext] = useState<{ situationId: string; item: WagonItem } | null>(null)
  const [message, setMessage] = useState('')
  const [menuOpen, setMenuOpen] = useState(false)
  // Walked off with the joystick: the spot-bound actions (inspect) don't apply.
  const [awayFromAnchor, setAwayFromAnchor] = useState(false)
  const [toast, setToast] = useState<string | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const showToast = (text: string) => {
    setToast(text)
    if (toastTimer.current) clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToast(null), 1800)
  }
  useEffect(() => () => { if (toastTimer.current) clearTimeout(toastTimer.current) }, [])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const situation = useQuery({
    queryKey: ['wagon-situation', selectedId],
    queryFn: () => api.getSituation(selectedId!),
    enabled: Boolean(selectedId),
    refetchOnWindowFocus: false,
  })

  useEffect(() => {
    if (!sessionId) { dispatch(navigate('practice')); return }
    let disposed = false
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null
    let reconnectAttempt = 0

    const open = (reconnecting: boolean) => {
      if (disposed) return
      dispatch(setWagonConnection(reconnecting ? 'reconnecting' : 'connecting'))
      const socket = new WebSocket(getWagonWebSocketUrl(wsPath ?? `/api/wagon/${sessionId}/ws`))
      socketRef.current = socket
      socket.onopen = () => { reconnectAttempt = 0; dispatch(setWagonConnection('connected')) }
      socket.onmessage = (event) => {
        try {
          const payload = JSON.parse(String(event.data)) as WagonSnapshot | WagonError
          if (payload.type === 'state') { dispatch(updateWagonSnapshot(payload)); setError(null) }
          else setError(payload.message)
        } catch { setError('Сервер прислал неизвестное состояние вагона') }
      }
      socket.onerror = () => dispatch(setWagonConnection('error'))
      socket.onclose = async () => {
        if (disposed) return
        dispatch(setWagonConnection('reconnecting'))
        try {
          const restored = await api.getSession(sessionId)
          if (disposed) return
          const restoredState = restoredSnapshot(restored)
          if (restoredState) dispatch(updateWagonSnapshot(restoredState))
          if (restored.session.status === 'finished') {
            const breakdown = await api.finishSession(sessionId)
            dispatch(isLessonPractice ? lessonPracticeFinished() : setWagonBreakdown(breakdown))
            return
          }
        } catch { /* next reconnect attempt will retry auth and state */ }
        reconnectAttempt += 1
        reconnectTimer = setTimeout(() => open(true), Math.min(6000, 800 * 2 ** Math.min(reconnectAttempt, 3)))
      }
    }
    open(false)
    return () => {
      disposed = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      const socket = socketRef.current
      socketRef.current = null
      if (socket) { socket.onclose = null; socket.close() }
    }
  }, [dispatch, sessionId, wsPath, isLessonPractice])

  const sendCommand = useCallback((command: WagonCommand) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== 1) { setError('Связь с вагоном восстанавливается'); return false }
    socket.send(JSON.stringify(command))
    setError(null)
    return true
  }, [])
  const sendCommandRef = useRef<typeof sendCommand | null>(null)
  sendCommandRef.current = sendCommand

  const activeById = useCallback((id: string) => snapshot?.active_situations.find((item) => item.situation_id === id), [snapshot])
  const seatForSituation = useCallback((active: WagonActiveSituation) => snapshot?.wagon_state.seats.find((seat) => seat.anchor === active.seat_anchor), [snapshot])

  const isNearSituation = useCallback((active: WagonActiveSituation) => {
    const seat = seatForSituation(active)
    return Boolean(snapshot && seat && !snapshot.wagon_state.player.moving && snapshot.wagon_state.player.at === seat.actor.at)
  }, [seatForSituation, snapshot])

  const openSituation = useCallback((id: string) => {
    dispatch(setWagonSelectedSituation(id))
    setShowService(false)
    setPendingSituationId(null)
  }, [dispatch])

  useEffect(() => {
    if (!pendingSituationId || !snapshot) return
    const active = activeById(pendingSituationId)
    if (!active) { setPendingSituationId(null); return }
    if (isNearSituation(active)) openSituation(pendingSituationId)
  }, [activeById, isNearSituation, openSituation, pendingSituationId, snapshot])

  const servicePoint = servicePointFor(snapshot?.wagon_state.class_id ?? 'standard')

  // Arriving at a point of interest is the "visit" fact lessons check (GDD §31).
  useEffect(() => {
    if (!isLessonPractice || !snapshot) return
    const { player, visited_anchors: visited } = snapshot.wagon_state
    if (player.moving) return
    if (!pointsOfInterestFor(snapshot.wagon_state.class_id).includes(player.at)) return
    if (visited?.includes(player.at) || visitSent.current.has(player.at)) return
    if (sendCommandRef.current?.({ type: 'visit', anchor: player.at })) visitSent.current.add(player.at)
  }, [isLessonPractice, snapshot])

  useEffect(() => {
    if (serviceRequested && snapshot && !snapshot.wagon_state.player.moving && snapshot.wagon_state.player.at === servicePoint) {
      setServiceRequested(false); setShowService(true)
    }
  }, [serviceRequested, snapshot, servicePoint])

  useEffect(() => {
    if (selectedId && snapshot && !activeById(selectedId)) dispatch(setWagonSelectedSituation(null))
  }, [activeById, dispatch, selectedId, snapshot])

  const handleAnchorPress = (anchor: WagonAnchor, nearby: boolean, fromJoystick: boolean) => {
    if (!snapshot) return
    if (snapshot.wagon_state.player.moving) return
    if (!nearby) { showToast(`Подойдите джойстиком: ${wagonAnchorLabels[anchor]}`); return }
    const isSeat = anchor.startsWith('seat_')
    const here = snapshot.wagon_state.player.at === anchor
    // Only a joystick arrival can register a new anchor on the server.
    if (!isSeat && (isLessonPractice || anchor !== servicePoint)) {
      if (!here) {
        if (fromJoystick) {
          if (anchor === servicePoint) setServiceRequested(true)
          sendCommand({ type: 'move_to', anchor })
        }
        else showToast('Отпустите джойстик рядом с точкой')
        return
      }
      // Already standing here: act on the point instead of ignoring the tap.
      const inspectable = (objectsAtAnchor[anchor] ?? []).find((object) => !(snapshot.wagon_state.inspected_objects ?? []).includes(object) && (lesson.data?.required_object_ids ?? []).includes(object))
      if (inspectable) { sendCommand({ type: 'inspect', item: inspectable }); return }
      if (anchor === servicePoint) { setShowService(true); return }
      showToast(`Вы уже здесь: ${wagonAnchorLabels[anchor]}`)
      return
    }
    if (anchor === servicePoint) {
      if (snapshot.wagon_state.player.at === anchor) { setServiceRequested(false); setShowService(true) }
      else if (fromJoystick) { setServiceRequested(true); sendCommand({ type: 'move_to', anchor }) }
      else showToast('Отпустите джойстик рядом с сервисной точкой')
      return
    }
    const active = snapshot.active_situations.find((item) => item.seat_anchor === anchor)
    if (!active) return
    if (isNearSituation(active)) { openSituation(active.situation_id); return }
    const seat = seatForSituation(active)
    const target = seat?.actor.at ?? anchor
    if (fromJoystick) {
      setPendingSituationId(active.situation_id)
      sendCommand({ type: 'move_to', anchor: target })
    } else showToast('Отпустите джойстик рядом с пассажиром')
  }

  const perform = async (work: () => Promise<unknown>) => {
    setBusy(true); setError(null)
    try { await work(); if (selectedId) await queryClient.invalidateQueries({ queryKey: ['wagon-situation', selectedId] }) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Действие не выполнено') }
    finally { setBusy(false) }
  }

  const sendDialogue = () => {
    const text = message.trim()
    if (!selectedId || !text) return
    setMessage('')
    void perform(() => api.sendMessage(selectedId, text, 'text'))
  }

  const finishShift = async () => {
    if (!sessionId) return
    setBusy(true)
    try {
      const breakdown = await api.finishSession(sessionId)
      await queryClient.invalidateQueries({ queryKey: ['wagon-levels'] })
      dispatch(isLessonPractice ? lessonPracticeFinished() : setWagonBreakdown(breakdown))
    }
    catch (cause) { autoFinishStarted.current = false; setError(cause instanceof Error ? cause.message : 'Не удалось завершить смену'); setBusy(false) }
  }

  useEffect(() => {
    if (!sessionId || !snapshot) return
    const duration = snapshot.wagon_state.duration_s || 480
    if (snapshot.game_time_s < duration || autoFinishStarted.current) return
    autoFinishStarted.current = true
    void finishShift()
  }, [sessionId, snapshot?.game_time_s, snapshot?.wagon_state.duration_s])

  // Alert.alert is a no-op on web, so the pause menu is an in-game sheet.
  const openMenu = () => setMenuOpen(true)

  // Lesson practice ends by itself when visit/inspect goals are reached or
  // the conversation situation closes. The server evaluates its outcome.
  const sawSituation = useRef(false)
  const lessonFinishTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (!isLessonPractice || !snapshot || !lesson.data || autoFinishStarted.current) return
    const visitedNow = snapshot.wagon_state.visited_anchors ?? []
    const inspectedNow = snapshot.wagon_state.inspected_objects ?? []
    const { required_anchor_ids: anchors, required_object_ids: objects } = lesson.data
    let done = false
    if (anchors.length + objects.length > 0) {
      done = anchors.every((id) => visitedNow.includes(id as WagonAnchor)) && objects.every((id) => inspectedNow.includes(id))
    } else {
      if (snapshot.active_situations.length > 0) sawSituation.current = true
      done = sawSituation.current && snapshot.active_situations.length === 0 && !selectedId
    }
    if (!done) return
    autoFinishStarted.current = true
    showToast(anchors.length + objects.length > 0 ? 'Все задачи выполнены' : 'Обращение завершено. Проверяем результат')
    // Kept in a ref: snapshot updates re-run this effect every second and must not cancel it.
    lessonFinishTimer.current = setTimeout(() => void finishShift(), 1200)
  }, [isLessonPractice, snapshot, lesson.data, selectedId])
  useEffect(() => () => { if (lessonFinishTimer.current) clearTimeout(lessonFinishTimer.current) }, [])

  if (!sessionId) return null
  if (!snapshot) return (
    <View style={styles.loading}><View style={styles.vsm}><Text style={styles.vsmText}>ВСМ</Text></View><ActivityIndicator color={colors.primary} size="large" /><Text style={styles.loadingTitle}>Готовим вагон</Text><Text style={styles.loadingText}>{connectionLabel(connection)}</Text>{error && <Text style={styles.errorText}>{error}</Text>}</View>
  )

  const duration = snapshot.wagon_state.duration_s || 480
  const remaining = Math.max(0, duration - snapshot.game_time_s)
  const carried = snapshot.wagon_state.carried_items ?? []
  const active = selectedId ? activeById(selectedId) : undefined
  const detail = situation.data?.situation
  const passengerLabel = translateBackendField(detail?.name, 'Пассажир')
  const nearSelected = active ? isNearSituation(active) : false
  const requirement = detail?.physical_requirement
  const returnToPassenger = () => {
    if (!serviceContext) return
    const targetSituation = activeById(serviceContext.situationId)
    const targetSeat = targetSituation ? seatForSituation(targetSituation) : undefined
    if (!targetSituation || !targetSeat) { setServiceContext(null); setShowService(false); return }
    setShowService(false)
    if (snapshot.wagon_state.player.at === targetSeat.actor.at && !awayFromAnchor) openSituation(serviceContext.situationId)
    else showToast('Подойдите к пассажиру джойстиком')
  }

  const visited = new Set<string>(snapshot.wagon_state.visited_anchors ?? [])
  const inspected = new Set<string>(snapshot.wagon_state.inspected_objects ?? [])
  const goals = lesson.data ? [
    ...lesson.data.required_anchor_ids.map((id) => ({ id, label: wagonAnchorLabels[id as WagonAnchor] ?? id, done: visited.has(id), verb: 'Посетить', target: id as WagonAnchor })),
    ...lesson.data.required_object_ids.map((id) => ({
      id, label: wagonObjectLabels[id] ?? id, done: inspected.has(id), verb: 'Осмотреть',
      target: (Object.keys(objectsAtAnchor) as WagonAnchor[]).find((anchor) => objectsAtAnchor[anchor]?.includes(id)),
    })),
  ] : []
  const goalsDone = goals.filter((goal) => goal.done).length
  const player = snapshot.wagon_state.player
  const inspectHere = isLessonPractice && !player.moving && !awayFromAnchor
    ? (objectsAtAnchor[player.at] ?? []).filter((object) => !inspected.has(object) && (lesson.data?.required_object_ids ?? []).includes(object))
    : []

  // The inventory opens the service point, where items are picked up.
  const openInventory = () => {
    setServiceContext(null)
    if (player.at === servicePoint && !player.moving && !awayFromAnchor) setShowService(true)
    else showToast('Подойдите к сервисной точке джойстиком')
  }

  const sendPhysical = (command: WagonCommand) => {
    if (!sendCommand(command)) return
    setTimeout(() => { if (selectedId) void queryClient.invalidateQueries({ queryKey: ['wagon-situation', selectedId] }) }, 500)
  }

  return (
    <KeyboardAvoidingView style={styles.page} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <View style={styles.game}>
        <WagonWorld snapshot={snapshot} disabled={busy || Boolean(selectedId) || Boolean(pendingSituationId) || showService || Boolean(snapshot.wagon_state.player.moving)} onAnchorPress={handleAnchorPress} showHint={!isLessonPractice} onAwayChange={setAwayFromAnchor} />
        <View style={styles.hud} pointerEvents="box-none">
          <View style={styles.topRow}>
            <View style={styles.brand}><Text style={styles.brandText}>ВСМ</Text><View><Text style={styles.shiftLabel}>СМЕНА В ПУТИ</Text><Text style={styles.timer}>{formatTime(remaining)}</Text></View></View>
            <View style={styles.topActions}>
              <Pressable accessibilityRole="button" accessibilityLabel={musicEnabled ? 'Выключить музыку' : 'Включить музыку'} onPress={toggleMusic} style={[styles.musicButton, !musicEnabled && styles.musicButtonMuted]}><Text style={styles.musicButtonText}>{musicEnabled ? '♫' : '♪'}</Text></Pressable>
              <Pressable accessibilityRole="button" accessibilityLabel="Пауза" onPress={openMenu} style={styles.menu}><Text style={styles.menuText}>Ⅱ</Text></Pressable>
            </View>
          </View>
          {connection !== 'connected' && (
            <View style={styles.statusRow}>
              <View style={[styles.connection, styles.connectionWarn]}><View style={[styles.connectionDot, styles.connectionDotWarn]} /><Text style={styles.connectionText}>{connectionLabel(connection)}</Text></View>
            </View>
          )}
          {isLessonPractice && lesson.data && (() => {
            // One task at a time keeps the wagon visible; the counter shows the rest.
            const next = goals.find((goal) => !goal.done)
            return (
              <View style={styles.goals}>
                <View style={styles.goalsTop}>
                  <Text style={styles.goalsKicker}>ЗАДАНИЯ</Text>
                  {goals.length > 0 && <Text style={styles.goalsCount}>{goalsDone}/{goals.length}</Text>}
                </View>
                {goals.length === 0 ? (
                  <Text style={styles.goalText}>Заметьте пассажира, подойдите и выясните его просьбу в разговоре.</Text>
                ) : next ? (
                  <Pressable accessibilityRole="button" accessibilityLabel={`${next.verb}: ${next.label}`}
                    disabled={!next.target || Boolean(player.moving)}
                    onPress={() => showToast(`Подойдите джойстиком: ${next.label}`)}
                    style={styles.goalRow}>
                    <View style={styles.goalCheck} />
                    <Text style={styles.goalText}>{next.verb}: {next.label}</Text>
                    {next.target && player.at !== next.target && <Text style={styles.goalGo}>Джойстик →</Text>}
                  </Pressable>
                ) : (
                  <View style={styles.goalRow}>
                    <View style={[styles.goalCheck, styles.goalCheckDone]}><Text style={styles.goalCheckMark}>✓</Text></View>
                    <Text style={styles.goalText}>Все задачи выполнены — завершите практику</Text>
                  </View>
                )}
              </View>
            )
          })()}
          {toast && <View style={styles.movingToast}><Text style={styles.movingText}>{toast}</Text></View>}
          {pendingSituationId && <View style={styles.movingToast}><ActivityIndicator size="small" color="#FFFFFF" /><Text style={styles.movingText}>Открываем обращение…</Text></View>}
        </View>
      </View>
      {isLessonPractice && !selectedId && !showService && inspectHere.length > 0 && (
        <View style={styles.lessonBar} pointerEvents="box-none">
          {inspectHere.map((object) => (
            <Pressable key={object} accessibilityRole="button" onPress={() => sendCommand({ type: 'inspect', item: object })} style={styles.inspectButton}>
              <Text style={styles.inspectText}>Осмотреть: {wagonObjectLabels[object] ?? object}</Text>
            </Pressable>
          ))}
        </View>
      )}
      {!selectedId && !showService && !menuOpen && (
        <Pressable accessibilityRole="button" accessibilityLabel={`Инвентарь: ${carried.length} из ${INVENTORY_SLOTS}. Открыть сервисную точку`}
          disabled={Boolean(snapshot.wagon_state.player.moving)} onPress={openInventory} style={styles.inventoryBar}>
          {Array.from({ length: INVENTORY_SLOTS }, (_, i) => carried[i]).map((item, i) => (
            <View key={i} style={[styles.slot, !item && styles.slotEmpty]}>
              {item ? <>
                <Icon name={itemIcon(item)} size={24} color={colors.primary} />
                <Text style={styles.slotLabel} numberOfLines={1}>{wagonItemLabels[item]}</Text>
              </> : <Text style={styles.slotEmptyText}>Пусто</Text>}
            </View>
          ))}
        </Pressable>
      )}
      {error && <Pressable onPress={() => setError(null)} style={styles.errorBanner}><Text style={styles.errorBannerText}>{error}</Text><Text style={styles.errorClose}>×</Text></Pressable>}

      {showService && (
        <View style={styles.sheetBackdrop}>
          <Pressable style={StyleSheet.absoluteFill} onPress={() => setShowService(false)} />
          <View style={styles.sheet}>
            <View style={styles.sheetHandle} /><View style={styles.sheetHeader}><View><Text style={styles.sheetKicker}>СЕРВИСНАЯ ТОЧКА</Text><Text style={styles.sheetTitle}>Что взять с собой?</Text></View><Pressable onPress={() => setShowService(false)} style={styles.close}><Text style={styles.closeText}>×</Text></Pressable></View>
            <Text style={styles.sheetHint}>{serviceContext ? 'Возьмите предмет, который попросил пассажир.' : 'Здесь показаны предметы, которые уже у вас.'}</Text>
            {serviceContext ? <>
              <View style={styles.itemRow}>{[serviceContext.item].map((item) => {
                const selected = carried.includes(item)
                return <Pressable key={item} disabled={selected} onPress={() => sendCommand({ type: 'pick_item', item })} style={[styles.itemCard, selected && styles.itemSelected]}><Text style={styles.itemIcon}>{item === 'blanket' ? '▤' : item === 'water' ? '●' : '◒'}</Text><Text style={styles.itemName}>{wagonItemLabels[item]}</Text><Text style={styles.itemState}>{selected ? 'Предмет у вас' : 'Взять'}</Text></Pressable>
              })}</View>
              {carried.includes(serviceContext.item) && <Pressable onPress={returnToPassenger} style={styles.returnButton}><Text style={styles.returnButtonText}>Вернуться к пассажиру →</Text></Pressable>}
            </> : carried.length ? <View style={styles.carriedRow}>{carried.map((item) => <View key={item} style={styles.carriedPill}><Text style={styles.carriedText}>{wagonItemLabels[item]}</Text></View>)}</View> : <Text style={styles.emptyInventory}>Сначала откройте обращение пассажира — тренажёр подскажет, что нужно взять.</Text>}
          </View>
        </View>
      )}

      {selectedId && (
        <View style={styles.sheetBackdrop}>
          <Pressable style={StyleSheet.absoluteFill} onPress={() => dispatch(setWagonSelectedSituation(null))} />
          <View style={[styles.sheet, styles.dialogSheet]}>
            <View style={styles.sheetHandle} />
            {situation.isLoading ? <ActivityIndicator color={colors.primary} style={styles.dialogLoader} /> : detail ? <>
              <View style={styles.sheetHeader}><View style={styles.dialogHeading}><View style={styles.situationIcon}><Text style={styles.situationIconText}>{wagonSituationIcon(active?.type ?? detail.code)}</Text></View><View style={styles.headingText}><Text style={styles.sheetKicker}>{active ? wagonAnchorLabels[active.seat_anchor].toUpperCase() : 'ПАССАЖИР'}</Text><Text style={styles.sheetTitle}>{detail.scenario || translateBackendField(detail.code, 'Обращение пассажира')}</Text><Text style={styles.passengerName}>{passengerLabel}</Text></View></View><Pressable onPress={() => dispatch(setWagonSelectedSituation(null))} style={styles.close}><Text style={styles.closeText}>×</Text></Pressable></View>
              <ScrollView style={styles.messages} contentContainerStyle={styles.messagesContent}>
                {(situation.data?.messages ?? []).map((item) => <View key={item.id} style={[styles.messageBubble, item.role === 'player' && styles.playerBubble]}><Text style={[styles.messageText, item.role === 'player' && styles.playerMessageText]}>{translateBackendText(item.content)}</Text></View>)}
                {detail.opening && !(situation.data?.messages?.length) && <View style={styles.messageBubble}><Text style={styles.messageText}>{detail.opening}</Text></View>}
              </ScrollView>
              {requirement && !detail.physical_action_done && <View style={styles.physicalCard}>
                <View style={styles.physicalText}><Text style={styles.physicalTitle}>{requirement.kind === 'deliver_item' ? `Передайте: ${wagonItemLabels[requirement.item]}` : 'Верните пассажира на место'}</Text><Text style={styles.physicalHint}>Сначала подойдите к нужной точке в вагоне</Text></View>
                {requirement.kind === 'deliver_item' && carried.includes(requirement.item) && nearSelected && <Pressable onPress={() => sendPhysical({ type: 'give_item', situation_id: selectedId, item: requirement.item })} style={styles.smallButton}><Text style={styles.smallButtonText}>Передать</Text></Pressable>}
                {requirement.kind === 'deliver_item' && !carried.includes(requirement.item) && <Pressable onPress={() => { setServiceContext({ situationId: selectedId, item: requirement.item }); dispatch(setWagonSelectedSituation(null)); if (snapshot.wagon_state.player.at === servicePoint && !awayFromAnchor) setShowService(true); else showToast('Подойдите к сервисной точке джойстиком') }} style={styles.smallButton}><Text style={styles.smallButtonText}>Взять</Text></Pressable>}
                {requirement.kind === 'redirect' && nearSelected && <Pressable onPress={() => sendPhysical({ type: 'redirect', situation_id: selectedId })} style={styles.smallButton}><Text style={styles.smallButtonText}>Проводить</Text></Pressable>}
              </View>}
              <Text style={styles.helpLabel}>ВЫЗВАТЬ ПОМОЩЬ</Text>
              <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.helpRow}>{escalationTargets.map(([target, label]) => {
                const selected = detail.escalations.includes(target)
                return <Pressable key={target} hitSlop={5} disabled={busy || selected} onPress={() => void perform(() => api.escalate(selectedId, target))} style={[styles.helpChip, selected && styles.helpChipSelected]}><Text style={[styles.helpChipText, selected && styles.helpChipTextSelected]}>{selected ? '✓ ' : ''}{label}</Text></Pressable>
              })}</ScrollView>
              <View style={styles.composer}><TextInput value={message} onChangeText={setMessage} onSubmitEditing={sendDialogue} editable={!busy} placeholder="Ответить пассажиру…" placeholderTextColor="#8A95A8" style={styles.input} /><Pressable disabled={!message.trim() || busy} onPress={sendDialogue} style={[styles.send, (!message.trim() || busy) && styles.sendDisabled]}><Text style={styles.sendText}>↑</Text></Pressable></View>
              <Pressable disabled={busy} onPress={() => void perform(() => api.finishSituation(selectedId).then(() => { dispatch(setWagonSelectedSituation(null)); return undefined }))} style={styles.finishSituation}><Text style={styles.finishSituationText}>Завершить обращение</Text></Pressable>
            </> : <Text style={styles.errorText}>Не удалось открыть обращение</Text>}
          </View>
        </View>
      )}
      {menuOpen && (
        <View style={styles.sheetBackdrop}>
          <Pressable style={StyleSheet.absoluteFill} accessibilityLabel="Закрыть меню" onPress={() => setMenuOpen(false)} />
          <View style={styles.sheet}>
            <View style={styles.sheetHandle} />
            <View style={styles.sheetHeader}>
              <View><Text style={styles.sheetKicker}>ПАУЗА</Text><Text style={styles.sheetTitle}>{isLessonPractice ? 'Практика урока' : 'Смена в вагоне'}</Text></View>
              <Pressable accessibilityRole="button" accessibilityLabel="Закрыть" onPress={() => setMenuOpen(false)} style={styles.close}><Text style={styles.closeText}>×</Text></Pressable>
            </View>
            <Text style={styles.sheetHint}>{isLessonPractice ? 'Практика завершится сама, когда все задачи выполнены. Можно закончить и раньше — незавершённые задачи попадут в разбор.' : 'Смену можно закончить досрочно — результат попадёт в разбор.'}</Text>
            <Pressable accessibilityRole="button" onPress={() => setMenuOpen(false)} style={[styles.menuAction, styles.menuActionPrimary]}><Text style={styles.menuActionPrimaryText}>Продолжить</Text></Pressable>
            <Pressable accessibilityRole="button" disabled={busy} onPress={() => { setMenuOpen(false); void finishShift() }} style={styles.menuAction}><Text style={styles.menuActionDanger}>{isLessonPractice ? 'Завершить практику' : 'Завершить смену'}</Text></Pressable>
          </View>
        </View>
      )}
    </KeyboardAvoidingView>
  )
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: '#DCE8F4' }, game: { flex: 1 },
  loading: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 12, backgroundColor: colors.soft }, vsm: { width: 76, height: 76, borderRadius: 24, backgroundColor: colors.primary, alignItems: 'center', justifyContent: 'center', marginBottom: 10 }, vsmText: { color: '#FFF', fontSize: 23, fontWeight: '900' }, loadingTitle: { color: colors.ink, fontSize: 20, fontWeight: '900' }, loadingText: { color: colors.muted }, errorText: { color: colors.critical, textAlign: 'center', padding: 16 },
  hud: { position: 'absolute', left: 14, right: 14, top: 12 }, topRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }, topActions: { flexDirection: 'row', alignItems: 'center', gap: 8 }, brand: { minWidth: 160, flexDirection: 'row', alignItems: 'center', gap: 10, borderRadius: 18, backgroundColor: 'rgba(255,255,255,.94)', paddingHorizontal: 12, paddingVertical: 9, ...shadow }, brandText: { color: colors.primary, fontSize: 22, fontWeight: '900', letterSpacing: -1 }, shiftLabel: { color: colors.muted, fontSize: 8, fontWeight: '900', letterSpacing: .8 }, timer: { color: colors.ink, fontSize: 15, fontWeight: '900', marginTop: 1 }, menu: { width: 45, height: 45, borderRadius: 15, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,.94)', ...shadow }, menuText: { color: colors.primary, fontSize: 18, fontWeight: '900', transform: [{ rotate: '90deg' }] }, musicButton: { width: 45, height: 45, borderRadius: 15, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,.94)', ...shadow }, musicButtonMuted: { opacity: 0.55 }, musicButtonText: { color: colors.primary, fontSize: 24, fontWeight: '700' },
  statusRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 8 }, connection: { flexDirection: 'row', alignItems: 'center', gap: 6, paddingHorizontal: 10, paddingVertical: 7, borderRadius: 99, backgroundColor: 'rgba(255,255,255,.9)' }, connectionWarn: { backgroundColor: '#FFF5DF' }, connectionDot: { width: 7, height: 7, borderRadius: 4, backgroundColor: colors.safety }, connectionDotWarn: { backgroundColor: colors.warning }, connectionText: { color: colors.ink, fontSize: 10, fontWeight: '800' }, movingToast: { alignSelf: 'center', flexDirection: 'row', gap: 8, alignItems: 'center', marginTop: 8, paddingHorizontal: 14, paddingVertical: 8, borderRadius: 99, backgroundColor: 'rgba(18,42,145,.88)' }, movingText: { color: '#FFF', fontSize: 11, fontWeight: '800' },
  menuAction: { minHeight: 50, borderRadius: 14, alignItems: 'center', justifyContent: 'center', borderWidth: 1, borderColor: colors.border, marginTop: 10 },
  menuActionPrimary: { backgroundColor: colors.action, borderColor: colors.action },
  menuActionPrimaryText: { color: '#FFF', fontSize: 15, fontWeight: '600' },
  menuActionDanger: { color: colors.errorInk, fontSize: 15, fontWeight: '600' },
  goals: { marginTop: 10, alignSelf: 'center', minWidth: 240, maxWidth: 360, borderRadius: 18, padding: 12, backgroundColor: 'rgba(255,255,255,.95)', ...shadow },
  goalsTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 10, marginBottom: 6 },
  goalsKicker: { flex: 1, color: colors.primary, fontSize: 10, fontWeight: '700', letterSpacing: .6 },
  goalsCount: { color: colors.action, fontSize: 12, fontWeight: '700' },
  goalRow: { flexDirection: 'row', alignItems: 'center', gap: 8, paddingVertical: 3 },
  goalCheck: { width: 18, height: 18, borderRadius: 9, borderWidth: 2, borderColor: colors.border, alignItems: 'center', justifyContent: 'center' },
  goalCheckDone: { backgroundColor: colors.success, borderColor: colors.success },
  goalCheckMark: { color: '#FFF', fontSize: 11, fontWeight: '800', lineHeight: 13 },
  goalText: { flexShrink: 1, color: colors.ink, fontSize: 13, lineHeight: 18 },
  goalGo: { marginLeft: 'auto', color: colors.action, fontSize: 12, fontWeight: '600' },
  goalTextDone: { color: colors.muted, textDecorationLine: 'line-through' },
  lessonBar: { position: 'absolute', left: 16, right: 16, bottom: 104, gap: 8, alignItems: 'center' },
  inventoryBar: { position: 'absolute', alignSelf: 'center', bottom: 20, flexDirection: 'row', gap: 8, padding: 8, borderRadius: 20, backgroundColor: 'rgba(255,255,255,.95)', ...shadow },
  slot: { width: 72, height: 60, borderRadius: 14, alignItems: 'center', justifyContent: 'center', gap: 2, backgroundColor: colors.blueSoft },
  slotEmpty: { backgroundColor: 'transparent', borderWidth: 1.5, borderStyle: 'dashed', borderColor: colors.border },
  slotLabel: { color: colors.primary, fontSize: 11, fontWeight: '600' },
  slotEmptyText: { color: colors.faint, fontSize: 11, fontWeight: '500' },
  inspectButton: { minHeight: 48, maxWidth: 420, width: '100%', borderRadius: 14, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.action, ...shadow },
  inspectText: { color: '#FFF', fontSize: 15, fontWeight: '600' },
  errorBanner: { position: 'absolute', left: 18, right: 18, top: 132, zIndex: 30, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', borderRadius: 14, backgroundColor: colors.critical, padding: 12 }, errorBannerText: { flex: 1, color: '#FFF', fontSize: 12, fontWeight: '700' }, errorClose: { color: '#FFF', fontSize: 22, marginLeft: 8 },
  sheetBackdrop: { position: 'absolute', left: 0, right: 0, top: 0, bottom: 0, zIndex: 40, justifyContent: 'flex-end', backgroundColor: 'rgba(8,16,46,.28)' }, sheet: { maxHeight: '74%', borderTopLeftRadius: 28, borderTopRightRadius: 28, padding: 20, paddingBottom: 24, backgroundColor: colors.surface, ...shadow }, dialogSheet: { minHeight: '55%' }, sheetHandle: { width: 42, height: 5, borderRadius: 4, alignSelf: 'center', backgroundColor: '#D7DDE8', marginBottom: 16 }, sheetHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }, sheetKicker: { color: colors.primary, fontSize: 9, fontWeight: '900', letterSpacing: 1 }, sheetTitle: { color: colors.ink, fontSize: 21, fontWeight: '900', marginTop: 3 }, close: { width: 36, height: 36, borderRadius: 13, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.soft }, closeText: { color: colors.muted, fontSize: 24, lineHeight: 26 }, sheetHint: { color: colors.muted, fontSize: 13, marginTop: 9 },
  itemRow: { flexDirection: 'row', gap: 8, marginTop: 16 }, itemCard: { flex: 1, minHeight: 112, alignItems: 'center', justifyContent: 'center', borderRadius: 18, borderWidth: 1, borderColor: colors.border, backgroundColor: colors.soft }, itemSelected: { borderColor: '#A9D8B9', backgroundColor: '#ECF9F1' }, itemIcon: { color: colors.primary, fontSize: 24, fontWeight: '900' }, itemName: { color: colors.ink, fontSize: 13, fontWeight: '900', marginTop: 7 }, itemState: { color: colors.muted, fontSize: 10, marginTop: 4 },
  returnButton: { minHeight: 47, alignItems: 'center', justifyContent: 'center', borderRadius: 15, backgroundColor: colors.primary, marginTop: 12 }, returnButtonText: { color: '#FFF', fontSize: 13, fontWeight: '900' }, carriedRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 15 }, carriedPill: { paddingHorizontal: 13, paddingVertical: 9, borderRadius: 99, backgroundColor: colors.blueSoft }, carriedText: { color: colors.primary, fontSize: 12, fontWeight: '800' }, emptyInventory: { color: colors.muted, fontSize: 12, lineHeight: 18, paddingVertical: 24, textAlign: 'center' },
  dialogLoader: { marginVertical: 80 }, dialogHeading: { flex: 1, flexDirection: 'row', alignItems: 'center', gap: 10 }, headingText: { flex: 1 }, passengerName: { color: colors.muted, fontSize: 9, marginTop: 2 }, situationIcon: { width: 43, height: 43, borderRadius: 15, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft }, situationIconText: { fontSize: 20 }, messages: { maxHeight: 210, marginTop: 14 }, messagesContent: { gap: 8, paddingBottom: 6 }, messageBubble: { maxWidth: '88%', alignSelf: 'flex-start', backgroundColor: colors.soft, borderRadius: 16, borderBottomLeftRadius: 5, paddingHorizontal: 13, paddingVertical: 10 }, playerBubble: { alignSelf: 'flex-end', backgroundColor: colors.primary, borderBottomLeftRadius: 16, borderBottomRightRadius: 5 }, messageText: { color: colors.ink, fontSize: 13, lineHeight: 18 }, playerMessageText: { color: '#FFF' },
  physicalCard: { flexDirection: 'row', alignItems: 'center', gap: 10, padding: 12, borderRadius: 16, backgroundColor: '#FFF6DF', borderWidth: 1, borderColor: '#F6D78D', marginTop: 10 }, physicalText: { flex: 1 }, physicalTitle: { color: '#7A4D00', fontSize: 12, fontWeight: '900' }, physicalHint: { color: '#906F35', fontSize: 10, marginTop: 3 }, smallButton: { borderRadius: 12, backgroundColor: colors.warning, paddingHorizontal: 13, paddingVertical: 10 }, smallButtonText: { color: '#FFF', fontSize: 11, fontWeight: '900' },
  helpLabel: { color: colors.muted, fontSize: 8, fontWeight: '900', letterSpacing: .8, marginTop: 8, marginBottom: 4, textAlign: 'center' }, helpRow: { flexGrow: 1, justifyContent: 'center', gap: 4, paddingHorizontal: 2 }, helpChip: { height: 18, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 7, paddingVertical: 0, borderRadius: 7, borderWidth: 1, borderColor: colors.border, backgroundColor: '#FFF' }, helpChipSelected: { borderColor: '#9FD6B3', backgroundColor: '#ECF9F1' }, helpChipText: { color: colors.muted, fontSize: 7, lineHeight: 8, fontWeight: '800', textAlign: 'center' }, helpChipTextSelected: { color: '#137A48' },
  composer: { flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 12 }, input: { flex: 1, minHeight: 46, borderWidth: 1, borderColor: colors.border, borderRadius: 16, backgroundColor: colors.soft, color: colors.ink, paddingHorizontal: 14, fontSize: 13 }, send: { width: 46, height: 46, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.primary }, sendDisabled: { opacity: .4 }, sendText: { color: '#FFF', fontSize: 24, fontWeight: '900' }, finishSituation: { alignSelf: 'center', marginTop: 13, padding: 5 }, finishSituationText: { color: colors.muted, fontSize: 11, fontWeight: '700', textDecorationLine: 'underline' },
})
