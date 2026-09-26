import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { navigate, startScenario, useAppDispatch, useAppSelector } from '../app/store'
import { ScenarioCard } from '../components/ScenarioCard'
import { ModelStage } from '../components/ModelStage'
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

      <ModelStage />

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
const styles = StyleSheet.create({ hero: { color: colors.ink, fontSize: 20, fontWeight: '800' }, body: { color: colors.muted, marginTop: 10, lineHeight: 21 } })
