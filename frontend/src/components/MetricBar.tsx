import { StyleSheet, Text, View } from 'react-native'
import { colors, radius } from '../helpers/theme'

interface MetricBarProps {
  label: string
  value: number
  tone: 'safety' | 'loyalty'
  compact?: boolean
}

export function MetricBar({ label, value, tone, compact = false }: MetricBarProps) {
  const color = tone === 'safety' ? colors.safety : colors.loyalty

  return (
    <View style={[styles.metric, compact && styles.compact]}>
      <View style={styles.row}>
        <Text style={styles.label}>{label}</Text>
        <Text style={[styles.value, { color }]}>{value}</Text>
      </View>
      <View style={styles.track}>
        <View style={[styles.fill, { width: `${value}%`, backgroundColor: color }]} />
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  metric: { flex: 1, minWidth: 120 },
  compact: { minWidth: 100 },
  row: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 7 },
  label: { color: colors.muted, fontSize: 12, fontWeight: '600' },
  value: { fontSize: 16, fontWeight: '800' },
  track: { height: 6, borderRadius: radius.pill, backgroundColor: '#E5EAEC', overflow: 'hidden' },
  fill: { height: '100%', borderRadius: radius.pill },
})
