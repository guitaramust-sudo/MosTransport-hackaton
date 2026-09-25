import type { ReactNode } from 'react'
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { colors, radius } from '../helpers/theme'

export function Page({ title, children }: { title: string; children: ReactNode }) {
  return <ScrollView style={styles.page} contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
    <Text style={styles.kicker}>ТРЕНАЖЁР ВСМ</Text><Text style={styles.title}>{title}</Text>{children}
  </ScrollView>
}

export function Card({ children }: { children: ReactNode }) { return <View style={styles.card}>{children}</View> }

export function Button({ title, onPress, busy, disabled, secondary }: {
  title: string; onPress: () => void; busy?: boolean; disabled?: boolean; secondary?: boolean
}) {
  return <Pressable accessibilityRole="button" disabled={busy || disabled} onPress={onPress}
    style={[styles.button, secondary && styles.secondary, (busy || disabled) && styles.disabled]}>
    {busy ? <ActivityIndicator color={secondary ? colors.primary : colors.surface} /> :
      <Text style={[styles.buttonText, secondary && styles.secondaryText]}>{title}</Text>}
  </Pressable>
}

export function ErrorText({ error }: { error: string | null }) {
  return error ? <Text style={styles.error}>{error}</Text> : null
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: '#F4F7F8' },
  content: { padding: 20, paddingBottom: 40 },
  kicker: { color: colors.primary, fontSize: 11, fontWeight: '800', letterSpacing: 1.2 },
  title: { color: colors.ink, fontSize: 30, fontWeight: '900', marginTop: 5, marginBottom: 18 },
  card: { backgroundColor: colors.surface, borderRadius: radius.lg, padding: 18, marginBottom: 12, borderWidth: 1, borderColor: colors.border },
  button: { backgroundColor: colors.primary, borderRadius: radius.md, padding: 14, alignItems: 'center', marginTop: 10 },
  secondary: { backgroundColor: colors.soft },
  disabled: { opacity: 0.55 },
  buttonText: { color: colors.surface, fontSize: 14, fontWeight: '800' },
  secondaryText: { color: colors.primary },
  error: { color: '#B42332', fontSize: 13, marginVertical: 10 },
})
