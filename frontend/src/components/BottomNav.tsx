import { Pressable, StyleSheet, View } from 'react-native'
import { Text } from './Typography'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { colors } from '../helpers/theme'
import type { AppScreen } from '../types'

const items: Array<{ screen: AppScreen; icon: string; label: string }> = [
  { screen: 'home', icon: '⌂', label: 'Главная' },
  { screen: 'scenarios', icon: '▤', label: 'Сценарии' },
  { screen: 'profile', icon: '◉', label: 'Профиль' },
]

export function BottomNav() {
  const dispatch = useAppDispatch()
  const active = useAppSelector((state) => state.app.screen)

  return (
    <View style={styles.nav}>
      {items.map((item) => {
        const selected = active === item.screen
        return (
          <Pressable key={item.screen} style={styles.item} onPress={() => dispatch(navigate(item.screen))}>
            <Text style={[styles.icon, selected && styles.active]}>{item.icon}</Text>
            <Text style={[styles.label, selected && styles.active]}>{item.label}</Text>
          </Pressable>
        )
      })}
    </View>
  )
}

const styles = StyleSheet.create({
  nav: { height: 74, flexDirection: 'row', backgroundColor: colors.surface, borderTopWidth: 1, borderTopColor: colors.border, paddingBottom: 8 },
  item: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 2 },
  icon: { color: '#89959B', fontSize: 23, fontWeight: '700' },
  label: { color: '#89959B', fontSize: 11, fontWeight: '600' },
  active: { color: colors.primary },
})
