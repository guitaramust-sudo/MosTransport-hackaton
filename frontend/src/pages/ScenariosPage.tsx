import { ScrollView, StyleSheet, Text, View } from 'react-native'
import { startScenario, useAppDispatch } from '../app/store'
import { ScenarioCard } from '../components/ScenarioCard'
import { scenarios } from '../data/scenarios'
import { colors, radius } from '../helpers/theme'

export function ScenariosPage() {
  const dispatch = useAppDispatch()

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
      <Text style={styles.kicker}>РўР Р•РќРђР–РЃР  Р’РЎРњ</Text>
      <Text style={styles.title}>РЎС†РµРЅР°СЂРёРё</Text>
      <Text style={styles.subtitle}>РћС‚СЂР°Р±РѕС‚Р°Р№С‚Рµ СЂРµС€РµРЅРёСЏ РґРѕ С‚РѕРіРѕ, РєР°Рє РѕРЅРё РїРѕРЅР°РґРѕР±СЏС‚СЃСЏ РІ СЂРµР°Р»СЊРЅРѕР№ СЃРјРµРЅРµ.</Text>
      <View style={styles.filters}>
        <View style={[styles.filter, styles.filterActive]}><Text style={styles.filterActiveText}>Р’СЃРµ</Text></View>
        <View style={styles.filter}><Text style={styles.filterText}>РќРѕРІС‹Рµ</Text></View>
        <View style={styles.filter}><Text style={styles.filterText}>РџСЂРѕР№РґРµРЅРЅС‹Рµ</Text></View>
      </View>
      {scenarios.map((scenario) => (
        <ScenarioCard key={scenario.id} scenario={scenario} onPress={() => scenario.events.length > 0 && dispatch(startScenario(scenario.id))} />
      ))}
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  content: { padding: 20, paddingBottom: 30 },
  kicker: { color: colors.primary, fontSize: 11, fontWeight: '800', letterSpacing: 1.4 },
  title: { color: colors.ink, fontSize: 32, fontWeight: '900', marginTop: 5 },
  subtitle: { color: colors.muted, fontSize: 14, lineHeight: 21, marginTop: 7, maxWidth: 340 },
  filters: { flexDirection: 'row', gap: 8, marginVertical: 22 },
  filter: { paddingVertical: 9, paddingHorizontal: 14, borderRadius: radius.pill, backgroundColor: '#E9EEF0' },
  filterActive: { backgroundColor: colors.dark },
  filterText: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  filterActiveText: { color: colors.surface, fontSize: 12, fontWeight: '700' },
})

