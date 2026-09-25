import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { navigate, startScenario, useAppDispatch, useAppSelector } from '../app/store'
import { ScenarioCard } from '../components/ScenarioCard'
import { scenarios } from '../data/scenarios'
import { colors, radius, shadow } from '../helpers/theme'

export function HomePage() {
  const dispatch = useAppDispatch()
  const score = useAppSelector((state) => state.app.score)

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
      <View style={styles.header}>
        <View>
          <Text style={styles.eyebrow}>ДОБРОЕ УТРО</Text>
          <Text style={styles.name}>Алексей, к смене готов?</Text>
        </View>
        <Pressable style={styles.avatar} onPress={() => dispatch(navigate('profile'))}>
          <Text style={styles.avatarText}>АК</Text>
          <View style={styles.online} />
        </Pressable>
      </View>

      <View style={styles.hero}>
        <View style={styles.trainLine} />
        <Text style={styles.heroLabel}>ВАШ УРОВЕНЬ</Text>
        <View style={styles.levelRow}>
          <Text style={styles.level}>7</Text>
          <View style={styles.levelCopy}>
            <Text style={styles.levelTitle}>Проводник-эксперт</Text>
            <Text style={styles.levelHint}>Ещё 260 XP до нового уровня</Text>
          </View>
        </View>
        <View style={styles.xpTrack}><View style={styles.xpFill} /></View>
        <View style={styles.heroStats}>
          <Text style={styles.heroStat}>◆ {score} XP</Text>
          <Text style={styles.heroStat}>⚡ 4 дня подряд</Text>
        </View>
      </View>

      <View style={styles.sectionHead}>
        <View>
          <Text style={styles.sectionTitle}>Продолжить обучение</Text>
          <Text style={styles.sectionSubtitle}>Тренируйтесь в реальных ситуациях</Text>
        </View>
        <Pressable onPress={() => dispatch(navigate('scenarios'))}><Text style={styles.all}>Все ›</Text></Pressable>
      </View>

      <ScenarioCard scenario={scenarios[0]} onPress={() => dispatch(startScenario(scenarios[0].id))} />

      <Text style={styles.sectionTitle}>Прогресс недели</Text>
      <View style={styles.weekCard}>
        <View style={styles.weekScore}>
          <Text style={styles.weekValue}>86%</Text>
          <Text style={styles.weekLabel}>точность решений</Text>
        </View>
        <View style={styles.divider} />
        <View style={styles.statItem}><Text style={styles.statValue}>4</Text><Text style={styles.statLabel}>сценария</Text></View>
        <View style={styles.statItem}><Text style={styles.statValue}>38</Text><Text style={styles.statLabel}>минут</Text></View>
      </View>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  content: { padding: 20, paddingBottom: 30 },
  header: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 22 },
  eyebrow: { color: colors.primary, fontSize: 11, letterSpacing: 1.4, fontWeight: '800' },
  name: { color: colors.ink, fontSize: 23, lineHeight: 29, fontWeight: '800', marginTop: 4 },
  avatar: { width: 48, height: 48, borderRadius: 17, backgroundColor: colors.dark, alignItems: 'center', justifyContent: 'center' },
  avatarText: { color: colors.surface, fontWeight: '800' },
  online: { position: 'absolute', right: -1, bottom: -1, width: 13, height: 13, borderRadius: 7, backgroundColor: '#28A978', borderWidth: 2, borderColor: colors.surface },
  hero: { backgroundColor: colors.primary, borderRadius: 26, padding: 22, overflow: 'hidden', ...shadow },
  trainLine: { position: 'absolute', width: 180, height: 180, borderRadius: 90, borderWidth: 32, borderColor: 'rgba(255,255,255,0.07)', right: -45, top: -60 },
  heroLabel: { color: '#FFDDE1', fontSize: 11, fontWeight: '800', letterSpacing: 1.3 },
  levelRow: { flexDirection: 'row', alignItems: 'center', marginTop: 12 },
  level: { color: colors.surface, fontSize: 52, lineHeight: 58, fontWeight: '900' },
  levelCopy: { marginLeft: 14 },
  levelTitle: { color: colors.surface, fontSize: 18, fontWeight: '800' },
  levelHint: { color: '#FFDDE1', fontSize: 12, marginTop: 4 },
  xpTrack: { height: 6, backgroundColor: 'rgba(255,255,255,0.23)', borderRadius: radius.pill, marginTop: 16, overflow: 'hidden' },
  xpFill: { width: '74%', height: '100%', backgroundColor: colors.surface, borderRadius: radius.pill },
  heroStats: { flexDirection: 'row', gap: 22, marginTop: 16 },
  heroStat: { color: colors.surface, fontSize: 12, fontWeight: '700' },
  sectionHead: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-end', marginTop: 28, marginBottom: 14 },
  sectionTitle: { color: colors.ink, fontSize: 19, fontWeight: '800' },
  sectionSubtitle: { color: colors.muted, fontSize: 12, marginTop: 3 },
  all: { color: colors.primary, fontSize: 14, fontWeight: '800' },
  weekCard: { flexDirection: 'row', alignItems: 'center', backgroundColor: colors.surface, marginTop: 14, padding: 18, borderRadius: radius.lg, borderWidth: 1, borderColor: '#EDF0F1' },
  weekScore: { flex: 1.4 },
  weekValue: { color: colors.safety, fontSize: 27, fontWeight: '900' },
  weekLabel: { color: colors.muted, fontSize: 11, marginTop: 2 },
  divider: { width: 1, height: 42, backgroundColor: colors.border, marginHorizontal: 16 },
  statItem: { flex: 0.7, alignItems: 'center' },
  statValue: { color: colors.ink, fontSize: 21, fontWeight: '800' },
  statLabel: { color: colors.muted, fontSize: 11 },
})
