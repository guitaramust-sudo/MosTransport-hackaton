import { useEffect, useState } from 'react'
import { StyleSheet } from 'react-native'
import { api } from '../api/client'
import { signedOut, useAppDispatch } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { Text } from '../components/Typography'
import { colors } from '../helpers/theme'
import type { Profile } from '../types'

export function ProfilePage() {
  const dispatch = useAppDispatch()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => { api.profile().then(setProfile).catch((cause) => setError(cause instanceof Error ? cause.message : 'Не удалось загрузить профиль')) }, [])
  return <Page title="Мой прогресс">
    <ErrorText error={error} />
    <Card><Text style={styles.name}>{profile?.player.username ?? 'Загрузка…'}</Text>
      <Text style={styles.body}>{profile?.player.email}</Text>
      <Text style={styles.xp}>{profile?.player.total_xp ?? 0} XP</Text>
    </Card>
    <Card><Text style={styles.name}>Компетенции</Text>
      {profile?.competencies.map((item) => <Text key={item.competency_id} style={styles.body}>#{item.competency_id}: {item.xp} XP</Text>)}
    </Card>
    <Button title="Выйти" onPress={() => dispatch(signedOut())} secondary />
  </Page>
}
const styles = StyleSheet.create({
  name: { color: colors.ink, fontSize: 18, fontWeight: '800' },
  body: { color: colors.muted, marginTop: 8 },
  xp: { color: colors.primary, fontSize: 26, fontWeight: '900', marginTop: 12 },
})
