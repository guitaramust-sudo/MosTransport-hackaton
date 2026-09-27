import { useState } from 'react'
import { StyleSheet } from 'react-native'
import { api, setTokens } from '../api/client'
import { signedIn, useAppDispatch } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { Text, TextInput } from '../components/Typography'
import { colors, radius } from '../helpers/theme'

export function AuthPage() {
  const dispatch = useAppDispatch()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function submit() {
    setBusy(true); setError(null)
    try {
      const result = await api.login(email.trim(), password)
      setTokens(result.tokens)
      dispatch(signedIn(result))
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Ошибка подключения') }
    finally { setBusy(false) }
  }

  return <Page title="Войти">
    <Card>
      <Text style={styles.label}>Электронная почта</Text>
      <TextInput style={styles.input} value={email} onChangeText={setEmail} autoCapitalize="none" keyboardType="email-address" autoComplete="email" />
      <Text style={styles.label}>Пароль</Text>
      <TextInput style={styles.input} value={password} onChangeText={setPassword} secureTextEntry autoComplete="current-password" />
      <ErrorText error={error} />
      <Button title="Войти" onPress={submit} busy={busy} disabled={!email.trim() || !password} />
      <Text style={styles.hint}>Аккаунт создаёт администратор. Получите у него адрес почты и пароль.</Text>
    </Card>
  </Page>
}

const styles = StyleSheet.create({
  label: { color: colors.ink, fontWeight: '700', marginTop: 12, marginBottom: 5 },
  input: { borderWidth: 1, borderColor: colors.border, backgroundColor: colors.surface, borderRadius: radius.md, padding: 12, fontSize: 16 },
  hint: { color: colors.muted, fontSize: 13, marginTop: 14 },
})
