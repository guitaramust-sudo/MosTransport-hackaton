import { useCallback, useEffect, useState } from 'react'
import { StyleSheet, Text, TextInput, View } from 'react-native'
import { api } from '../api/client'
import { navigate, refreshShift, selectSituation, setBreakdown, useAppDispatch, useAppSelector } from '../app/store'
import { MetricBar } from '../components/MetricBar'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { colors, radius } from '../helpers/theme'
import type { SituationResponse } from '../types'

const targets = [
  ['train_chief', 'Начальник поезда'], ['ptb', 'ПТБ'], ['police', 'Полиция'],
  ['medic', 'Медик'], ['ambulance', 'Скорая помощь'],
] as const
const languages: Record<string, string> = { ru: 'Русский', en: 'Английский', zh: 'Китайский', de: 'Немецкий' }

export function SimulationPage() {
  const dispatch = useAppDispatch()
  const { shift, situationId } = useAppSelector((state) => state.app)
  const [detail, setDetail] = useState<SituationResponse | null>(null)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [now, setNow] = useState(Date.now())

  const reload = useCallback(async () => {
    if (!shift || !situationId) return
    const [situation, session] = await Promise.all([api.getSituation(situationId), api.getSession(shift.session.id)])
    setDetail(situation)
    dispatch(refreshShift(session))
  }, [dispatch, shift?.session.id, situationId])

  useEffect(() => {
    setDetail(null); setError(null)
    reload().catch((cause) => setError(cause instanceof Error ? cause.message : 'Не удалось загрузить ситуацию'))
  }, [reload])
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])
  useEffect(() => {
    if (!shift || !situationId || detail?.situation.status !== 'active') return
    const timer = setInterval(() => { reload().catch(() => {}) }, 3000)
    return () => clearInterval(timer)
  }, [detail?.situation.status, reload, shift?.session.id, situationId])

  async function run(action: () => Promise<unknown>, clearMessage = false) {
    setBusy(true); setError(null)
    try { await action(); if (clearMessage) setMessage(''); await reload() }
    catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Ошибка запроса')
      await reload().catch(() => {})
    } finally { setBusy(false) }
  }

  async function finishShift() {
    if (!shift) return
    setBusy(true); setError(null)
    try { dispatch(setBreakdown(await api.finishSession(shift.session.id))) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось завершить смену') }
    finally { setBusy(false) }
  }

  if (!shift || !situationId) return <Page title="Смена"><Text>Нет активной смены.</Text><Button title="На главную" onPress={() => dispatch(navigate('home'))} /></Page>
  const situation = detail?.situation ?? shift.situations.find((item) => item.id === situationId)
  const seconds = situation?.timer_deadline ? Math.max(0, Math.ceil((Date.parse(situation.timer_deadline) - now) / 1000)) : null

  return <Page title="Смена в пути">
    <Button title="Список ситуаций" onPress={() => dispatch(navigate('scenarios'))} secondary />
    <Card>
      <Text style={styles.title}>{situation?.scenario ?? 'Загрузка…'}</Text>
      {!!situation?.language && <Text style={styles.body}>Язык пассажира: {languages[situation.language] ?? situation.language}</Text>}
      <Text style={styles.body}>{situation?.opening}</Text>
      <Text style={styles.timer}>{situation?.status === 'closed' ? `Итог: ${situation.outcome}` : seconds !== null ? `Осталось: ${seconds} сек` : 'Таймер не задан'}</Text>
      <View style={styles.metrics}><MetricBar label="Безопасность" value={situation?.safety ?? 50} tone="safety" /><View style={styles.gap} /><MetricBar label="Лояльность" value={situation?.loyalty ?? 50} tone="loyalty" /></View>
      {situation?.status === 'closed' && <Text style={styles.body}>XP: {situation.xp}{situation.remarks?.length ? ` · ${situation.remarks.map((r) => r.message).join('; ')}` : ''}</Text>}
    </Card>
    <Card>
      <Text style={styles.section}>Диалог</Text>
      {detail?.messages.length ? detail.messages.map((item) => <View key={item.id} style={[styles.bubble, item.role === 'player' && styles.playerBubble]}>
        <Text style={styles.speaker}>{item.role === 'player' ? 'Вы' : item.role === 'passenger' ? 'Пассажир' : 'Система'}</Text>
        <Text style={styles.body}>{item.content}</Text>
      </View>) : <Text style={styles.body}>Начните разговор с пассажиром.</Text>}
      {situation?.status === 'active' && <>
        <TextInput style={styles.input} value={message} onChangeText={setMessage} placeholder="Ваш ответ пассажиру" multiline editable={!busy} />
        <Button title="Отправить" onPress={() => run(() => api.sendMessage(situationId, message.trim()), true)} disabled={!message.trim()} busy={busy} />
      </>}
    </Card>
    {situation?.status === 'active' && <Card>
      <Text style={styles.section}>Вызвать помощь</Text>
      <Text style={styles.body}>Вызов фиксируется отдельно от текста диалога.</Text>
      {targets.map(([id, label]) => <Button key={id} title={`${situation.escalations?.includes(id) ? '✓ ' : ''}${label}`}
        onPress={() => run(() => api.escalate(situationId, id))} disabled={busy || situation.escalations?.includes(id)} secondary />)}
      <Button title="Завершить ситуацию" onPress={() => run(() => api.finishSituation(situationId))} busy={busy} />
    </Card>}
    <ErrorText error={error} />
    <Button title="Завершить смену и увидеть разбор" onPress={finishShift} busy={busy} />
    {shift.situations.filter((item) => item.id !== situationId).map((item) => <Button key={item.id} title={`${item.status === 'closed' ? '✓ ' : ''}${item.scenario}`}
      onPress={() => dispatch(selectSituation(item.id))} secondary />)}
  </Page>
}

const styles = StyleSheet.create({
  title: { color: colors.ink, fontSize: 20, fontWeight: '900' },
  section: { color: colors.ink, fontSize: 17, fontWeight: '800', marginBottom: 8 },
  body: { color: colors.muted, fontSize: 14, lineHeight: 20, marginTop: 5 },
  timer: { color: colors.primary, fontSize: 14, fontWeight: '800', marginTop: 14 },
  metrics: { flexDirection: 'row', marginTop: 18 }, gap: { width: 16 },
  bubble: { backgroundColor: colors.soft, borderRadius: radius.md, padding: 12, marginTop: 8 },
  playerBubble: { backgroundColor: '#E9F2FD' },
  speaker: { color: colors.ink, fontWeight: '800', fontSize: 12 },
  input: { minHeight: 75, borderWidth: 1, borderColor: colors.border, borderRadius: radius.md, padding: 12, marginTop: 15, textAlignVertical: 'top' },
})
