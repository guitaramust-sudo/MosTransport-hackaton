import { useState, type ComponentProps } from 'react'
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, View } from 'react-native'
import { api, setTokens } from '../api/client'
import { signedIn, useAppDispatch } from '../app/store'
import { Logo } from '../components/Logo'
import { Button, ErrorText, SpeedLine } from '../components/UI'
import { Text, TextInput } from '../components/Typography'
import { colors, radius, spacing, type } from '../helpers/theme'

function Field({ label, value, onChangeText, error, ...props }: {
  label: string
  value: string
  onChangeText: (value: string) => void
  error?: boolean
} & ComponentProps<typeof TextInput>) {
  const [focused, setFocused] = useState(false)
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      <TextInput {...props} value={value} onChangeText={onChangeText}
        onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
        placeholderTextColor={colors.faint} accessibilityLabel={label}
        style={[styles.input, focused && styles.inputFocused, error && styles.inputError]} />
    </View>
  )
}

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

  const canSubmit = !!email.trim() && !!password

  return (
    <KeyboardAvoidingView style={styles.page} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
        <View style={styles.column}>
          <Logo size={44} />
          <SpeedLine style={styles.speed} />

          <Text style={styles.title}>Добро пожаловать в команду ВСМ</Text>
          <Text style={styles.subtitle}>Тренажёр проводника. Войдите в учётную запись — её создаёт администратор.</Text>

          <Field label="Электронная почта" value={email} onChangeText={setEmail} error={!!error}
            autoCapitalize="none" keyboardType="email-address" autoComplete="email" placeholder="name@company.ru" />
          <Field label="Пароль" value={password} onChangeText={setPassword} error={!!error}
            secureTextEntry autoComplete="current-password" onSubmitEditing={canSubmit ? submit : undefined} />
          <ErrorText error={error} />
          <Button title="Войти" onPress={submit} busy={busy} disabled={!canSubmit} style={styles.submit} />
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: colors.surface },
  content: { flexGrow: 1, padding: spacing.md, alignItems: 'center', justifyContent: 'center' },
  column: { width: '100%', maxWidth: 440 },
  speed: { marginTop: spacing.md, marginBottom: spacing.xl },
  title: { ...type.h1, color: colors.ink },
  subtitle: { ...type.secondary, color: colors.secondary, marginTop: spacing.xs, marginBottom: spacing.lg },
  field: { marginBottom: spacing.md },
  label: { ...type.secondary, fontWeight: '500', color: colors.ink, marginBottom: 6 },
  input: { outlineWidth: 0, minHeight: 52, borderWidth: 1.5, borderColor: colors.border, backgroundColor: colors.background, borderRadius: radius.button, paddingHorizontal: 16, fontSize: 16, color: colors.ink },
  inputFocused: { borderColor: colors.action, backgroundColor: colors.surface },
  inputError: { borderColor: colors.error },
  submit: { marginTop: spacing.xs },
})
