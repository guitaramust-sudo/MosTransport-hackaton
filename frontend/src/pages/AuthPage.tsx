import { useState } from 'react'
import { StyleSheet, Text, TextInput } from 'react-native'
import { api, setTokens } from '../api/client'
import { signedIn, useAppDispatch } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { colors, radius } from '../helpers/theme'

export function AuthPage() {
  const dispatch = useAppDispatch()
  const [register, setRegister] = useState(false)
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function submit() {
    setBusy(true); setError(null)
    try {
      const result = register ? await api.register(email.trim(), username.trim(), password) : await api.login(email.trim(), password)
      setTokens(result.tokens)
      dispatch(signedIn(result))
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Ошибка подключения') }
    finally { setBusy(false) }
  }

  return <Page title={register ? 'Создать аккаунт' : 'Войти'}>
    <Card>
      <Text style={styles.label}>Электронная почта</Text>
      <TextInput style={styles.input} value={email} onChangeText={setEmail} autoCapitalize="none" keyboardType="email-address" autoComplete="email" />
      {register && <><Text style={styles.label}>Имя</Text><TextInput style={styles.input} value={username} onChangeText={setUsername} autoComplete="username" /></>}
      <Text style={styles.label}>Пароль</Text>
      <TextInput style={styles.input} value={password} onChangeText={setPassword} secureTextEntry autoComplete={register ? 'new-password' : 'current-password'} />
      <ErrorText error={error} />
      <Button title={register ? 'Зарегистрироваться' : 'Войти'} onPress={submit} busy={busy} disabled={!email.trim() || !password || (register && !username.trim())} />
      <Button title={register ? 'Уже есть аккаунт' : 'Создать аккаунт'} onPress={() => { setRegister(!register); setError(null) }} secondary />
    </Card>
  </Page>
}

const styles = StyleSheet.create({
  label: { color: colors.ink, fontWeight: '700', marginTop: 12, marginBottom: 5 },
  input: { borderWidth: 1, borderColor: colors.border, backgroundColor: colors.surface, borderRadius: radius.md, padding: 12, fontSize: 16 },
})
