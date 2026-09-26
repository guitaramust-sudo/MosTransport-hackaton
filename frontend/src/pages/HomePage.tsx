import { useEffect, useState } from 'react'
import { StyleSheet, Text } from 'react-native'
import { api } from '../api/client'
import { navigate, setPlayer, setShift, useAppDispatch, useAppSelector } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { colors } from '../helpers/theme'

export function HomePage() {
  const dispatch = useAppDispatch()
  const { auth, shift } = useAppSelector((state) => state.app)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => { api.profile().then((profile) => dispatch(setPlayer(profile.player))).catch(() => {}) }, [dispatch])
  async function start() {
    setBusy(true); setError(null)
    try { dispatch(setShift(await api.startSession())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать смену') }
    finally { setBusy(false) }
  }
  return <Page title={`Привет, ${auth?.player.username ?? 'проводник'}!`}>
    <Card><Text style={styles.hero}>Ваш опыт: {auth?.player.total_xp ?? 0} XP</Text>
      <Text style={styles.body}>Проведите смену: общайтесь с пассажирами, вызывайте помощь и закрывайте ситуации.</Text>
      <ErrorText error={error} />
      {shift?.session.status === 'active' && <Button title="Продолжить смену" onPress={() => dispatch(navigate('simulation'))} secondary />}
      <Button title="Начать новую смену" onPress={start} busy={busy} />
    </Card>
  </Page>
}
const styles = StyleSheet.create({ hero: { color: colors.ink, fontSize: 20, fontWeight: '800' }, body: { color: colors.muted, marginTop: 10, lineHeight: 21 } })
