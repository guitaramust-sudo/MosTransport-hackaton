import { Pressable, StyleSheet, View } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { colors, radius, type } from '../helpers/theme'
import type { AppScreen } from '../types'
import { Icon, type IconName } from './Icon'
import { Text } from './Typography'

// Four sections outside the game (§9). Admin is reached from Профиль.
const items: Array<{ screen: AppScreen; icon: IconName; label: string }> = [
  { screen: 'home', icon: 'home', label: 'Главная' },
  { screen: 'practice', icon: 'practice', label: 'Практика' },
  { screen: 'progress', icon: 'progress', label: 'Прогресс' },
  { screen: 'profile', icon: 'profile', label: 'Профиль' },
]

export function BottomNav() {
  const dispatch = useAppDispatch()
  const active = useAppSelector((state) => state.app.screen)
  const insets = useSafeAreaInsets()
  const current = active === 'admin' ? 'profile' : active

  return (
    <View style={[styles.nav, { paddingBottom: Math.max(insets.bottom, 8) }]} accessibilityRole="tablist">
      {items.map((item) => {
        const selected = current === item.screen
        const color = selected ? colors.action : colors.muted
        return (
          <Pressable key={item.screen} accessibilityRole="tab" accessibilityState={{ selected }} accessibilityLabel={item.label}
            style={styles.item} onPress={() => dispatch(navigate(item.screen))}>
            <View style={[styles.iconWrap, selected && styles.iconActive]}>
              <Icon name={item.icon} size={24} color={color} strokeWidth={selected ? 2 : 1.8} />
            </View>
            <Text style={[styles.label, { color }, selected && styles.labelActive]}>{item.label}</Text>
          </Pressable>
        )
      })}
    </View>
  )
}

const styles = StyleSheet.create({
  nav: { flexDirection: 'row', backgroundColor: colors.surface, borderTopWidth: 1, borderTopColor: colors.border, paddingTop: 8 },
  item: { flex: 1, minHeight: 52, alignItems: 'center', justifyContent: 'center', gap: 2 },
  iconWrap: { width: 56, height: 30, borderRadius: radius.pill, alignItems: 'center', justifyContent: 'center' },
  iconActive: { backgroundColor: colors.blueSoft },
  label: { ...type.label },
  labelActive: { fontWeight: '600' },
})
