import { useEffect, useState } from 'react'
import { StyleSheet } from 'react-native'
import { api } from '../api/client'
import { navigate, setLiveSimulation, setPlayer, setShift, useAppDispatch, useAppSelector } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { Text } from '../components/Typography'
import { colors } from '../helpers/theme'

export function HomePage() {
  const dispatch = useAppDispatch()
  const { auth, shift, liveSimulation } = useAppSelector((state) => state.app)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => { api.profile().then((profile) => dispatch(setPlayer(profile.player))).catch(() => {}) }, [dispatch])
  async function start() {
    setBusy(true); setError(null)
    try { dispatch(setShift(await api.startSession())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать смену') }
    finally { setBusy(false) }
  }
  async function startLive() {
    setBusy(true); setError(null)
    try { dispatch(setLiveSimulation(await api.startLiveSimulation())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать симуляцию') }
    finally { setBusy(false) }
  }
  return <Page title={`Привет, ${auth?.player.username ?? 'проводник'}!`}>
    <Card><Text style={styles.hero}>Ваш опыт: {auth?.player.total_xp ?? 0} XP</Text>
      <Text style={styles.body}>Проведите смену: общайтесь с пассажирами, вызывайте помощь и закрывайте ситуации.</Text>
      <ErrorText error={error} />
      {shift?.session.status === 'active' && <Button title="Продолжить смену" onPress={() => dispatch(navigate('simulation'))} secondary />}
      <Button title="Начать новую смену" onPress={start} busy={busy} />
    </Card>
    <Card>
      <Text style={styles.hero}>Новая симуляция</Text>
      <Text style={styles.body}>Случайный пассажир и живой разговор. GigaChat отвечает при настроенном сервере; локально доступен Mock. Решения меняют ход смены.</Text>
      {liveSimulation?.run.status === 'active' && <Button title="Продолжить симуляцию" onPress={() => dispatch(navigate('live_simulation'))} secondary />}
      <Button title="Начать симуляцию" onPress={startLive} busy={busy} />
    </Card>
  </Page>
}
const styles = StyleSheet.create({ hero: { color: colors.ink, fontSize: 20, fontWeight: '800' }, body: { color: colors.muted, marginTop: 10, lineHeight: 21 } })
