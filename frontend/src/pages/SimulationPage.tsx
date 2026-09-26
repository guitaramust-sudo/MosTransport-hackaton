import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, refreshShift, selectSituation, setBreakdown, useAppDispatch, useAppSelector } from '../app/store'
import { GameWorld } from '../components/GameWorld'
import { MetricBar } from '../components/MetricBar'
import { Text, TextInput } from '../components/Typography'
import { colors, radius, shadow } from '../helpers/theme'
import type { GameQuest, SessionResponse, Situation, SituationResponse } from '../types'

const escalationTargets = [
  ['train_chief', 'Начальник поезда'], ['ptb', 'ПТБ'], ['police', 'Полиция'],
  ['medic', 'Медик'], ['ambulance', 'Скорая'],
] as const

const languageLabels: Record<string, string> = { ru: 'Русский', en: 'Английский', zh: 'Китайский', de: 'Немецкий' }
const seatLabels = ['Ряд 2 · слева', 'Ряд 4 · справа', 'Ряд 6 · слева', 'Ряд 8 · справа']

function isCritical(code: string) {
  return /medical|health|security|conflict|danger|emergency/i.test(code)
}

function toQuest(situation: Situation, seatIndex: number): GameQuest {
  return {
    id: situation.id,
    title: situation.scenario || situation.name || situation.code.replaceAll('_', ' '),
    location: seatLabels[seatIndex % seatLabels.length],
    priority: isCritical(situation.code) ? 'critical' : 'normal',
    seatIndex,
  }
}

export function SimulationPage() {
  const dispatch = useAppDispatch()
  const { shift, situationId } = useAppSelector((state) => state.app)
  const [detail, setDetail] = useState<SituationResponse | null>(null)
  const [message, setMessage] = useState('')
  const [moveRequest, setMoveRequest] = useState(0)
  const [approachingId, setApproachingId] = useState<string | null>(null)
  const [panelOpen, setPanelOpen] = useState(false)
  const [loadingSituation, setLoadingSituation] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [now, setNow] = useState(Date.now())
  const finishingSession = useRef(false)

  const quests = useMemo<GameQuest[]>(() => (
    shift?.situations
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => item.status === 'active')
      .map(({ item, index }) => toQuest(item, index)) ?? []
  ), [shift?.situations])

  const selectedSummary = shift?.situations.find((item) => item.id === situationId)
  const situation = detail?.situation.id === situationId ? detail.situation : selectedSummary

  const syncSituation = useCallback(async (id: string): Promise<SessionResponse | null> => {
    if (!shift) return null
    const [nextDetail, session] = await Promise.all([
      api.getSituation(id),
      api.getSession(shift.session.id),
    ])
    setDetail(nextDetail)
    dispatch(refreshShift(session))
    return session
  }, [dispatch, shift?.session.id])

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    if (!shift || shift.session.status !== 'active') return
    const timer = setInterval(() => {
      api.getSession(shift.session.id).then((session) => dispatch(refreshShift(session))).catch(() => undefined)
    }, 3000)
    return () => clearInterval(timer)
  }, [dispatch, shift?.session.id, shift?.session.status])

  useEffect(() => { finishingSession.current = false }, [shift?.session.id])

  useEffect(() => {
    if (!shift || shift.session.status !== 'active' || shift.situations.length === 0 ||
      shift.situations.some((item) => item.status === 'active') || finishingSession.current) return

    finishingSession.current = true
    api.finishSession(shift.session.id)
      .then((breakdown) => dispatch(setBreakdown(breakdown)))
      .catch((cause) => {
        finishingSession.current = false
        setError(cause instanceof Error ? cause.message : 'Не удалось завершить смену')
      })
  }, [dispatch, shift])

  const chooseQuest = (id: string) => {
    dispatch(selectSituation(id))
    setApproachingId(id)
    setPanelOpen(false)
    setDetail(null)
    setError(null)
    setMoveRequest((value) => value + 1)
  }

  const arriveAtQuest = async (id: string) => {
    dispatch(selectSituation(id))
    setApproachingId(null)
    setPanelOpen(true)
    setLoadingSituation(true)
    setError(null)
    try { await syncSituation(id) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось загрузить ситуацию') }
    finally { setLoadingSituation(false) }
  }

  const refreshCurrent = async () => situationId ? syncSituation(situationId) : null

  const sendMessage = async () => {
    if (!situationId || !message.trim()) return
    setBusy(true); setError(null)
    try {
      await api.sendMessage(situationId, message.trim())
      setMessage('')
      await refreshCurrent()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось отправить сообщение')
      await refreshCurrent().catch(() => null)
    } finally { setBusy(false) }
  }

  const escalate = async (target: string) => {
    if (!situationId) return
    setBusy(true); setError(null)
    try { await api.escalate(situationId, target); await refreshCurrent() }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось вызвать помощь') }
    finally { setBusy(false) }
  }

  const finishSituation = async () => {
    if (!shift || !situationId) return
    setBusy(true); setError(null)
    try {
      await api.finishSituation(situationId)
      const session = await syncSituation(situationId)
      setPanelOpen(false)
      setApproachingId(null)
      const next = session?.situations.find((item) => item.status === 'active')
      if (next) dispatch(selectSituation(next.id))
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось завершить ситуацию')
      await refreshCurrent().catch(() => null)
    } finally { setBusy(false) }
  }

  const finishShift = async () => {
    if (!shift || finishingSession.current) return
    finishingSession.current = true
    setBusy(true); setError(null)
    try { dispatch(setBreakdown(await api.finishSession(shift.session.id))) }
    catch (cause) {
      finishingSession.current = false
      setError(cause instanceof Error ? cause.message : 'Не удалось завершить смену')
    } finally { setBusy(false) }
  }

  if (!shift) {
    return <View style={styles.emptyScreen}>
      <Text style={styles.emptyTitle}>Нет активной смены</Text>
      <Pressable style={styles.primaryButton} onPress={() => dispatch(navigate('home'))}><Text style={styles.primaryButtonText}>На главную</Text></Pressable>
    </View>
  }

  const seconds = situation?.timer_deadline ? Math.max(0, Math.ceil((Date.parse(situation.timer_deadline) - now) / 1000)) : null
  const targetId = approachingId ?? situationId ?? quests[0]?.id ?? ''
  const targetSeatIndex = quests.find((quest) => quest.id === targetId)?.seatIndex ?? 0

  return <View style={styles.screen}>
    <GameWorld
      targetEventId={targetId}
      targetSeatIndex={targetSeatIndex}
      moveRequest={moveRequest}
      questCardsVisible={!panelOpen}
      quests={quests}
      onQuestPress={chooseQuest}
      onArrive={arriveAtQuest}
    />

    <View style={styles.topBar} pointerEvents="box-none">
      <Pressable style={styles.menuButton} onPress={() => dispatch(navigate('scenarios'))}><Text style={styles.menuIcon}>‹</Text></Pressable>
      <View style={styles.routeBlock}><Text style={styles.route}>ВСМ · СМЕНА В ПУТИ</Text><Text style={styles.routeTime}>{quests.length} активных ситуаций</Text></View>
      <Pressable style={styles.finishButton} onPress={finishShift} disabled={busy}><Text style={styles.finishButtonText}>Итог</Text></Pressable>
    </View>

    {situation ? <View style={styles.missionCard} pointerEvents="none">
      <View style={[styles.missionIcon, isCritical(situation.code) && styles.missionIconCritical]}><Text style={styles.missionIconText}>{isCritical(situation.code) ? '!' : '◆'}</Text></View>
      <View style={styles.missionCopy}>
        <Text style={styles.missionLabel}>{situation.status === 'closed' ? 'ЗАВЕРШЕНО' : 'ТЕКУЩАЯ СИТУАЦИЯ'}</Text>
        <Text numberOfLines={1} style={styles.missionTitle}>{situation.scenario}</Text>
        {!!situation.language && <Text style={styles.missionMeta}>Язык: {languageLabels[situation.language] ?? situation.language}</Text>}
      </View>
      {seconds !== null && situation.status === 'active' ? <View style={styles.timer}><Text style={styles.timerText}>{seconds}</Text></View> : null}
    </View> : null}

    {!panelOpen ? <View style={styles.statusPill} pointerEvents="none"><Text style={styles.statusText}>
      {approachingId ? 'Проводник направляется к пассажиру…' : 'Нажмите карточку задания у кресла'}
    </Text></View> : null}
    {!!error && !panelOpen ? <View style={styles.errorToast}><Text style={styles.errorText}>{error}</Text></View> : null}

    {panelOpen ? <View style={styles.sheet}>
      <View style={styles.sheetHeader}>
        <View style={styles.sheetHeaderCopy}><Text style={styles.sheetKicker}>{situation?.name || situation?.code}</Text><Text style={styles.sheetTitle}>{situation?.scenario ?? 'Загрузка ситуации'}</Text></View>
        <Pressable style={styles.closeButton} onPress={() => setPanelOpen(false)}><Text style={styles.closeText}>×</Text></Pressable>
      </View>
      {loadingSituation ? <View style={styles.loading}><ActivityIndicator color={colors.primary} size="large" /></View> :
        <ScrollView style={styles.sheetScroll} contentContainerStyle={styles.sheetContent} keyboardShouldPersistTaps="handled">
          {!!situation?.opening && <Text style={styles.opening}>{situation.opening}</Text>}
          <View style={styles.metrics}><View style={styles.metric}><MetricBar label="Безопасность" value={situation?.safety ?? 50} tone="safety" /></View><View style={styles.metric}><MetricBar label="Лояльность" value={situation?.loyalty ?? 50} tone="loyalty" /></View></View>
          <Text style={styles.sectionTitle}>Диалог</Text>
          {detail?.messages.length ? detail.messages.map((item) => <View key={item.id} style={[styles.bubble, item.role === 'player' && styles.playerBubble]}>
            <Text style={styles.speaker}>{item.role === 'player' ? 'Вы' : item.role === 'passenger' ? 'Пассажир' : 'Система'}</Text><Text style={styles.bubbleText}>{item.content}</Text>
          </View>) : <Text style={styles.muted}>Начните разговор с пассажиром.</Text>}

          {situation?.status === 'active' ? <>
            <TextInput style={styles.input} value={message} onChangeText={setMessage} placeholder="Ваш ответ пассажиру" placeholderTextColor={colors.muted} multiline editable={!busy} />
            <Pressable style={[styles.primaryButton, (!message.trim() || busy) && styles.disabled]} onPress={sendMessage} disabled={!message.trim() || busy}>
              {busy ? <ActivityIndicator color={colors.surface} /> : <Text style={styles.primaryButtonText}>Отправить</Text>}
            </Pressable>
            <Text style={styles.sectionTitle}>Вызвать помощь</Text>
            <View style={styles.escalations}>{escalationTargets.map(([id, label]) => {
              const selected = situation.escalations?.includes(id)
              return <Pressable key={id} style={[styles.escalation, selected && styles.escalationSelected]} onPress={() => escalate(id)} disabled={busy || selected}>
                <Text style={[styles.escalationText, selected && styles.escalationTextSelected]}>{selected ? '✓ ' : ''}{label}</Text>
              </Pressable>
            })}</View>
            <Pressable style={[styles.resolveButton, busy && styles.disabled]} onPress={finishSituation} disabled={busy}><Text style={styles.resolveButtonText}>Завершить ситуацию</Text></Pressable>
          </> : <Text style={styles.result}>Итог: {situation?.outcome} · {situation?.xp ?? 0} XP</Text>}
          {!!error && <Text style={styles.errorText}>{error}</Text>}
        </ScrollView>}
    </View> : null}
  </View>
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#BFD7CF' },
  emptyScreen: { flex: 1, alignItems: 'center', justifyContent: 'center', padding: 24, backgroundColor: '#F4F7F8' },
  emptyTitle: { color: colors.ink, fontSize: 22, fontWeight: '900', marginBottom: 16 },
  topBar: { position: 'absolute', top: 10, left: 12, right: 12, flexDirection: 'row', alignItems: 'center' },
  menuButton: { width: 44, height: 44, borderRadius: 15, backgroundColor: colors.surface, alignItems: 'center', justifyContent: 'center', ...shadow },
  menuIcon: { color: colors.ink, fontSize: 31, lineHeight: 33, fontWeight: '500' },
  routeBlock: { flex: 1, alignItems: 'center' },
  route: { color: colors.ink, fontSize: 11, fontWeight: '900', backgroundColor: 'rgba(255,255,255,0.9)', paddingHorizontal: 12, paddingTop: 7, borderTopLeftRadius: 12, borderTopRightRadius: 12 },
  routeTime: { color: colors.muted, fontSize: 10, backgroundColor: 'rgba(255,255,255,0.9)', paddingHorizontal: 12, paddingBottom: 7, paddingTop: 2, borderBottomLeftRadius: 12, borderBottomRightRadius: 12 },
  finishButton: { width: 50, height: 44, borderRadius: 15, backgroundColor: colors.primary, alignItems: 'center', justifyContent: 'center' },
  finishButtonText: { color: colors.surface, fontSize: 10, fontWeight: '900' },
  missionCard: { position: 'absolute', top: 68, left: 18, right: 18, flexDirection: 'row', alignItems: 'center', backgroundColor: '#8B4EE9', borderRadius: 18, padding: 11, borderWidth: 3, borderColor: '#7035CE', ...shadow },
  missionIcon: { width: 43, height: 43, borderRadius: 13, backgroundColor: '#FFD22E', alignItems: 'center', justifyContent: 'center' },
  missionIconCritical: { backgroundColor: '#FF7280' },
  missionIconText: { color: '#4B2A00', fontSize: 20, fontWeight: '900' },
  missionCopy: { flex: 1, marginLeft: 10 },
  missionLabel: { color: '#EADFFF', fontSize: 8, fontWeight: '900', letterSpacing: 0.7 },
  missionTitle: { color: colors.surface, fontSize: 13, fontWeight: '900', marginTop: 2 },
  missionMeta: { color: '#EADFFF', fontSize: 9, marginTop: 3 },
  timer: { width: 42, height: 42, borderRadius: 21, backgroundColor: colors.surface, alignItems: 'center', justifyContent: 'center', marginLeft: 8 },
  timerText: { color: colors.primary, fontSize: 15, fontWeight: '900' },
  statusPill: { position: 'absolute', bottom: 20, left: 32, right: 32, backgroundColor: 'rgba(25,40,46,0.92)', borderRadius: radius.pill, paddingHorizontal: 18, paddingVertical: 12 },
  statusText: { color: colors.surface, textAlign: 'center', fontSize: 11, fontWeight: '800' },
  errorToast: { position: 'absolute', left: 20, right: 20, bottom: 72, backgroundColor: '#FFF0F1', borderRadius: radius.md, padding: 12 },
  errorText: { color: '#B42332', fontSize: 12, lineHeight: 17, marginTop: 10 },
  sheet: { position: 'absolute', left: 8, right: 8, bottom: 8, maxHeight: '72%', backgroundColor: colors.surface, borderRadius: 24, overflow: 'hidden', ...shadow },
  sheetHeader: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingTop: 15, paddingBottom: 10, borderBottomWidth: 1, borderBottomColor: colors.border },
  sheetHeaderCopy: { flex: 1 },
  sheetKicker: { color: colors.primary, fontSize: 9, fontWeight: '900', letterSpacing: 0.8 },
  sheetTitle: { color: colors.ink, fontSize: 18, fontWeight: '900', marginTop: 3 },
  closeButton: { width: 34, height: 34, borderRadius: 12, backgroundColor: colors.soft, alignItems: 'center', justifyContent: 'center' },
  closeText: { color: colors.ink, fontSize: 25, lineHeight: 27 },
  loading: { minHeight: 220, alignItems: 'center', justifyContent: 'center' },
  sheetScroll: { flexGrow: 0 },
  sheetContent: { padding: 16, paddingBottom: 26 },
  opening: { color: colors.ink, fontSize: 13, lineHeight: 19, backgroundColor: colors.soft, borderRadius: radius.md, padding: 12 },
  metrics: { flexDirection: 'row', gap: 12, marginTop: 14 },
  metric: { flex: 1 },
  sectionTitle: { color: colors.ink, fontSize: 14, fontWeight: '900', marginTop: 17, marginBottom: 5 },
  bubble: { alignSelf: 'flex-start', maxWidth: '88%', backgroundColor: colors.soft, borderRadius: radius.md, padding: 10, marginTop: 7 },
  playerBubble: { alignSelf: 'flex-end', backgroundColor: '#E9F2FD' },
  speaker: { color: colors.primary, fontWeight: '900', fontSize: 9 },
  bubbleText: { color: colors.ink, fontSize: 12, lineHeight: 17, marginTop: 3 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 6 },
  input: { minHeight: 72, borderWidth: 1, borderColor: colors.border, borderRadius: radius.md, padding: 11, marginTop: 12, color: colors.ink, textAlignVertical: 'top' },
  primaryButton: { minHeight: 44, backgroundColor: colors.primary, borderRadius: radius.md, paddingHorizontal: 16, alignItems: 'center', justifyContent: 'center', marginTop: 10 },
  primaryButtonText: { color: colors.surface, fontSize: 13, fontWeight: '900' },
  disabled: { opacity: 0.5 },
  escalations: { flexDirection: 'row', flexWrap: 'wrap', gap: 7 },
  escalation: { backgroundColor: colors.soft, borderRadius: radius.pill, paddingHorizontal: 11, paddingVertical: 8, borderWidth: 1, borderColor: colors.border },
  escalationSelected: { backgroundColor: '#E3F6ED', borderColor: '#63C694' },
  escalationText: { color: colors.ink, fontSize: 10, fontWeight: '800' },
  escalationTextSelected: { color: '#16734A' },
  resolveButton: { minHeight: 44, borderRadius: radius.md, borderWidth: 2, borderColor: colors.primary, alignItems: 'center', justifyContent: 'center', marginTop: 16 },
  resolveButtonText: { color: colors.primary, fontSize: 13, fontWeight: '900' },
  result: { color: colors.primary, fontSize: 14, fontWeight: '900', marginTop: 16 },
})
