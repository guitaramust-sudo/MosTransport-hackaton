import { useEffect, useState } from 'react'
import { Pressable, StyleSheet, TextInput, View } from 'react-native'
import { api } from '../api/client'
import { navigate, updateLiveSimulation, useAppDispatch, useAppSelector } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { Text } from '../components/Typography'
import { colors, radius } from '../helpers/theme'
import type { LiveResult } from '../types'

const locations: Record<string, string> = {
  passenger_zone: 'Пассажирский салон',
  luggage_zone: 'Багажная зона',
}

function commandID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (letter) => {
    const value = Math.floor(Math.random() * 16)
    return (letter === 'x' ? value : (value & 3) | 8).toString(16)
  })
}

export function LiveSimulationPage() {
  const dispatch = useAppDispatch()
  const live = useAppSelector((state) => state.app.liveSimulation)
  const [selectedEventID, setSelectedEventID] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<LiveResult | null>(null)
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => { setResult(null); setMessage(''); setSelectedEventID('') }, [live?.run.id])

  useEffect(() => {
    if (!live || live.run.status !== 'active') return
    let mounted = true
    const timer = setInterval(() => {
      api.getLiveSimulation(live.run.id)
        .then((updated) => { if (mounted && !busy) dispatch(updateLiveSimulation(updated)) })
        .catch(() => undefined)
    }, 5000)
    return () => { mounted = false; clearInterval(timer) }
  }, [busy, dispatch, live?.run.id, live?.run.status])

  useEffect(() => {
    if (!live || live.run.status !== 'finished') return
    let mounted = true
    api.getLiveResult(live.run.id)
      .then((summary) => { if (mounted) setResult(summary) })
      .catch((cause) => { if (mounted) setError(cause instanceof Error ? cause.message : 'Не удалось загрузить разбор') })
    return () => { mounted = false }
  }, [live?.run.id, live?.run.status])

  if (!live) return <Page title="Новая симуляция"><Button title="На главную" onPress={() => dispatch(navigate('home'))} /></Page>

  const localEvents = live.events.filter((event) => event.location === live.run.location)
  const selectedEvent = localEvents.find((event) => event.id === selectedEventID) ?? localEvents[0]
  const remaining = live.run.deadline_at ? Math.max(0, Math.ceil((new Date(live.run.deadline_at).getTime() - now) / 1000)) : null

  async function apply(operation: () => ReturnType<typeof api.liveAction> | ReturnType<typeof api.liveDialogue>) {
    if (!live || busy) return false
    setBusy(true); setError(null)
    try {
      const updated = await operation()
      dispatch(updateLiveSimulation(updated))
      return true
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Не удалось выполнить действие')
      api.getLiveSimulation(live.run.id).then((updated) => dispatch(updateLiveSimulation(updated))).catch(() => undefined)
      return false
    } finally { setBusy(false) }
  }

  function sendDialogue() {
    if (!live || !selectedEvent || !message.trim()) return
    const current = live
    const text = message.trim()
    apply(() => api.liveDialogue(current.run.id, {
      command_id: commandID(), expected_state_version: current.run.state_version,
      event_id: selectedEvent.id, text,
    })).then((success) => { if (success) setMessage('') })
  }

  function sendChoice(eventID: string, choiceID: string) {
    if (!live) return
    const current = live
    apply(() => api.liveAction(current.run.id, {
      command_id: commandID(), expected_state_version: current.run.state_version,
      event_id: eventID, choice_id: choiceID,
    }))
  }

  function sendWorldAction(actionID: string, target?: string) {
    if (!live) return
    const current = live
    apply(() => api.liveAction(current.run.id, {
      command_id: commandID(), expected_state_version: current.run.state_version,
      action_id: actionID, target,
    }))
  }

  return <Page title="Живая симуляция">
    <Card>
      <Text style={styles.kicker}>ДЕМО · {live.run.content_validation_status.toUpperCase()}</Text>
      <Text style={styles.heading}>{live.passenger.name || 'Пассажир'} · {live.passenger.temperament}</Text>
      <Text style={styles.body}>Запрос: {live.passenger.request}. Напряжение: {live.passenger.tension}/3.</Text>
      <Text style={styles.quote}>«{live.passenger.opening}»</Text>
      <Text style={styles.meta}>Место: {locations[live.run.location] ?? live.run.location} · Игровое время: {live.run.game_time_s} с
        {remaining !== null ? ` · До срока: ${remaining} с` : ''}</Text>
      <Text style={styles.meta}>Лояльность {live.run.loyalty} · Безопасность {live.run.safety}</Text>
      {live.run.timed_out && <Text style={styles.warning}>Срок ответа пропущен; ситуация изменилась.</Text>}
    </Card>

    {live.run.status === 'active' ? <>
      <Card>
        <Text style={styles.heading}>Что происходит</Text>
        {live.events.map((event) => <Pressable key={event.id} onPress={() => setSelectedEventID(event.id)}
          style={[styles.event, selectedEvent?.id === event.id && styles.eventSelected]}>
          <Text style={styles.eventTitle}>{locations[event.location] ?? event.location}</Text>
          <Text style={styles.body}>{event.text}</Text>
        </Pressable>)}
        {live.observable_cues.map((cue) => <Text key={cue} style={styles.warning}>Замечено: {cue}</Text>)}
        {live.run.location === 'passenger_zone'
          ? <Button title="Перейти в багажную зону" onPress={() => sendWorldAction('move_to', 'luggage_zone')} disabled={busy} secondary />
          : <Button title="Вернуться в салон" onPress={() => sendWorldAction('move_to', 'passenger_zone')} disabled={busy} secondary />}
        {live.observable_cues.length > 0 && <Button title="Осмотреть зону" onPress={() => sendWorldAction('inspect')} disabled={busy} />}
      </Card>

      {selectedEvent && <Card>
        <Text style={styles.heading}>Разговор и решение</Text>
        {selectedEvent.location === live.run.location ? <>
          <Text style={styles.body}>Напишите пассажиру своими словами. Если действие выражено ясно, сервер применит соответствующую ветку.</Text>
          <TextInput value={message} onChangeText={setMessage} editable={!busy} multiline maxLength={600}
            placeholder="Ваш ответ пассажиру…" style={styles.input} accessibilityLabel="Ответ пассажиру" />
          <Button title="Отправить пассажиру" onPress={sendDialogue} busy={busy} disabled={!message.trim()} />
          <Text style={styles.hint}>Или выберите действие напрямую:</Text>
          {selectedEvent.choices.map((choice) => <Button key={choice.id} title={choice.text}
            onPress={() => sendChoice(selectedEvent.id, choice.id)} disabled={busy} secondary />)}
        </> : <Text style={styles.body}>Перейдите в нужную зону.</Text>}
      </Card>}
    </> : <Card>
      <Text style={styles.heading}>Смена завершена</Text>
      {result && <>
        <Text style={styles.body}>{result.session_pass ? 'Смена зачтена' : 'Смена не зачтена'} · Очки рейтинга +{result.leaderboard_points_delta}</Text>
        <Text style={styles.body}>Безопасность: {result.session_safety_score} · Лояльность: {result.loyalty}</Text>
        <Text style={styles.hint}>Разбор решений:</Text>
        {result.debrief.map((entry, index) => <View key={`${entry.event_id}-${index}`} style={styles.debrief}>
          <Text style={styles.eventTitle}>{entry.player_text || entry.action_id}</Text>
          {entry.passenger_reply && <Text style={styles.quote}>«{entry.passenger_reply}»</Text>}
          <Text style={styles.body}>{entry.explanation}</Text>
          {entry.better_options.length > 0 && <Text style={styles.hint}>Лучше: {entry.better_options.join('; ')}</Text>}
        </View>)}
      </>}
      <Button title="На главную" onPress={() => dispatch(navigate('home'))} />
    </Card>}

    {live.dialogue.length > 0 && <Card>
      <Text style={styles.heading}>История разговора</Text>
      {live.dialogue.map((turn) => <View key={turn.command_id} style={styles.turn}>
        <Text style={styles.eventTitle}>Вы: {turn.player}</Text>
        <Text style={styles.body}>{live.passenger.name}: {turn.passenger}</Text>
        {turn.choice_id && <Text style={styles.hint}>Решение: {turn.choice_id}</Text>}
      </View>)}
    </Card>}
    <ErrorText error={error} />
    <Button title="Выйти на главную" onPress={() => dispatch(navigate('home'))} secondary />
  </Page>
}

const styles = StyleSheet.create({
  kicker: { color: colors.primary, fontSize: 11, fontWeight: '800', letterSpacing: 1 },
  heading: { color: colors.ink, fontSize: 19, fontWeight: '800', marginTop: 5, marginBottom: 8 },
  body: { color: colors.ink, fontSize: 14, lineHeight: 21 },
  quote: { color: colors.muted, fontSize: 14, lineHeight: 21, marginTop: 10 },
  meta: { color: colors.muted, fontSize: 12, marginTop: 10 },
  warning: { color: colors.warning, fontSize: 13, marginTop: 10, fontWeight: '700' },
  event: { borderWidth: 1, borderColor: colors.border, borderRadius: radius.md, padding: 12, marginTop: 8 },
  eventSelected: { borderColor: colors.primary, backgroundColor: colors.soft },
  eventTitle: { color: colors.ink, fontSize: 13, fontWeight: '700', marginBottom: 3 },
  input: { borderWidth: 1, borderColor: colors.border, borderRadius: radius.md, padding: 12, marginTop: 12, minHeight: 75, color: colors.ink, textAlignVertical: 'top' },
  hint: { color: colors.muted, fontSize: 12, marginTop: 12 },
  debrief: { borderTopWidth: 1, borderTopColor: colors.border, paddingTop: 12, marginTop: 12 },
  turn: { borderTopWidth: 1, borderTopColor: colors.border, paddingTop: 10, marginTop: 10 },
})
