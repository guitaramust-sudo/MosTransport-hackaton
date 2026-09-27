import { StyleSheet, View } from 'react-native'
import Svg, { Path } from 'react-native-svg'
import { colors } from '../helpers/theme'
import { Text } from './Typography'

/** ВСМ wordmark with the blue/red speed swoosh from the brand board. */
export function Logo({ size = 40, inverted = false, caption = true }: { size?: number; inverted?: boolean; caption?: boolean }) {
  const ink = inverted ? colors.surface : colors.primary
  return (
    <View accessibilityRole="image" accessibilityLabel="ВСМ — Высокоскоростная магистраль">
      <View style={styles.row}>
        <Text style={[styles.word, { fontSize: size, lineHeight: size * 1.05, color: ink }]}>ВСМ</Text>
        <Svg width={size * 1.5} height={size * 0.62} viewBox="0 0 90 36" style={{ marginLeft: size * 0.06 }}>
          <Path d="M2 26 C 30 22, 58 10, 88 6 C 70 14, 44 22, 10 30 Z" fill={inverted ? colors.surface : colors.action} />
          <Path d="M8 33 C 36 30, 62 20, 88 13 C 72 22, 48 30, 16 35 Z" fill={colors.brandRed} />
        </Svg>
      </View>
      {caption && (
        <Text style={[styles.caption, { fontSize: Math.max(9, size * 0.24), color: ink }]}>
          ВЫСОКОСКОРОСТНАЯ МАГИСТРАЛЬ
        </Text>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'flex-end' },
  word: { fontWeight: '800', letterSpacing: -1.5, fontStyle: 'italic' },
  caption: { fontWeight: '700', letterSpacing: 0.3, marginTop: 2 },
})
