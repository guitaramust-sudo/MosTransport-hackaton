import { Pressable, StyleSheet, Text, View } from 'react-native'
import { colors, radius, shadow } from '../helpers/theme'
import type { Scenario } from '../types'

interface ScenarioCardProps {
  scenario: Scenario
  onPress: () => void
}

export function ScenarioCard({ scenario, onPress }: ScenarioCardProps) {
  const disabled = scenario.events.length === 0 && scenario.progress === 100

  return (
    <Pressable onPress={onPress} disabled={disabled} style={({ pressed }) => [styles.card, pressed && styles.pressed]}>
      <View style={styles.topRow}>
        <View style={[styles.icon, scenario.difficulty === 'Сложный' && styles.iconCritical]}>
          <Text style={styles.iconText}>{scenario.difficulty === 'Сложный' ? '!' : scenario.progress === 100 ? '✓' : '↗'}</Text>
        </View>
        <View style={styles.meta}>
          <Text style={styles.tag}>{scenario.tag}</Text>
          <Text style={styles.duration}>{scenario.duration} · {scenario.difficulty}</Text>
        </View>
        <Text style={styles.arrow}>›</Text>
      </View>
      <Text style={styles.title}>{scenario.title}</Text>
      <Text style={styles.subtitle}>{scenario.subtitle}</Text>
      {scenario.progress > 0 && (
        <View style={styles.progressRow}>
          <View style={styles.progressTrack}>
            <View style={[styles.progressFill, { width: `${scenario.progress}%` }]} />
          </View>
          <Text style={styles.progressText}>{scenario.progress}%</Text>
        </View>
      )}
    </Pressable>
  )
}

const styles = StyleSheet.create({
  card: { backgroundColor: colors.surface, borderRadius: radius.lg, padding: 18, marginBottom: 14, borderWidth: 1, borderColor: '#EDF0F1', ...shadow },
  pressed: { opacity: 0.8, transform: [{ scale: 0.99 }] },
  topRow: { flexDirection: 'row', alignItems: 'center', marginBottom: 14 },
  icon: { width: 40, height: 40, borderRadius: 13, backgroundColor: '#E8F3EF', alignItems: 'center', justifyContent: 'center' },
  iconCritical: { backgroundColor: '#FBE9EB' },
  iconText: { color: colors.primary, fontSize: 19, fontWeight: '900' },
  meta: { flex: 1, marginLeft: 12 },
  tag: { color: colors.primary, fontSize: 12, fontWeight: '800', textTransform: 'uppercase', letterSpacing: 0.6 },
  duration: { color: colors.muted, fontSize: 12, marginTop: 3 },
  arrow: { color: colors.ink, fontSize: 30, fontWeight: '300' },
  title: { color: colors.ink, fontSize: 20, lineHeight: 25, fontWeight: '800' },
  subtitle: { color: colors.muted, fontSize: 14, lineHeight: 20, marginTop: 6 },
  progressRow: { flexDirection: 'row', alignItems: 'center', marginTop: 16, gap: 10 },
  progressTrack: { flex: 1, height: 6, backgroundColor: colors.soft, borderRadius: radius.pill, overflow: 'hidden' },
  progressFill: { height: '100%', backgroundColor: colors.primary, borderRadius: radius.pill },
  progressText: { color: colors.muted, fontSize: 12, fontWeight: '700' },
})
