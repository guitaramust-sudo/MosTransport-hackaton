import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { MetricBar } from '../components/MetricBar'
import { colors, radius, shadow } from '../helpers/theme'

export function DebriefPage() {
  const dispatch = useAppDispatch()
  const { actions, safety, loyalty, score } = useAppSelector((state) => state.app)
  const positive = actions.filter((action) => action.safety + action.loyalty >= 0).length

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
      <View style={styles.badge}><Text style={styles.badgeText}>✓</Text></View>
      <Text style={styles.eyebrow}>СЦЕНАРИЙ ЗАВЕРШЁН</Text>
      <Text style={styles.title}>Смена окончена</Text>
      <Text style={styles.subtitle}>Вы сохранили контроль над ситуацией и завершили рейс.</Text>

      <View style={styles.scoreCard}>
        <Text style={styles.scoreLabel}>РЕЗУЛЬТАТ</Text>
        <Text style={styles.score}>{Math.round((safety + loyalty) / 2)}%</Text>
        <Text style={styles.scoreHint}>+{Math.max(40, positive * 85)} XP · {score} всего</Text>
        <View style={styles.metrics}>
          <MetricBar label="Безопасность" value={safety} tone="safety" />
          <View style={styles.gap} />
          <MetricBar label="Лояльность" value={loyalty} tone="loyalty" />
        </View>
      </View>

      <Text style={styles.sectionTitle}>Разбор решений</Text>
      {actions.map((action, index) => {
        const good = action.safety + action.loyalty >= 0
        return (
          <View style={styles.action} key={`${action.eventTitle}-${index}`}>
            <View style={[styles.actionIcon, good ? styles.actionGood : styles.actionBad]}><Text style={styles.actionIconText}>{good ? '✓' : '!'}</Text></View>
            <View style={styles.actionCopy}>
              <Text style={styles.actionEvent}>{action.eventTitle}</Text>
              <Text style={styles.actionChoice}>{action.choiceTitle}</Text>
              <Text style={styles.actionFeedback}>{action.feedback}</Text>
              <Text style={styles.competency}>Навык: {action.competency}</Text>
            </View>
          </View>
        )
      })}

      <Pressable style={styles.primaryButton} onPress={() => dispatch(navigate('scenarios'))}><Text style={styles.primaryText}>К другим сценариям</Text></Pressable>
      <Pressable style={styles.secondaryButton} onPress={() => dispatch(navigate('home'))}><Text style={styles.secondaryText}>На главную</Text></Pressable>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#F4F7F8' },
  content: { padding: 20, paddingTop: 30, paddingBottom: 40 },
  badge: { width: 62, height: 62, borderRadius: 22, backgroundColor: '#E3F3ED', alignSelf: 'center', alignItems: 'center', justifyContent: 'center' },
  badgeText: { color: colors.safety, fontSize: 30, fontWeight: '900' },
  eyebrow: { color: colors.safety, textAlign: 'center', fontSize: 10, fontWeight: '900', letterSpacing: 1.3, marginTop: 16 },
  title: { color: colors.ink, textAlign: 'center', fontSize: 29, fontWeight: '900', marginTop: 6 },
  subtitle: { color: colors.muted, textAlign: 'center', fontSize: 14, lineHeight: 20, marginTop: 7, paddingHorizontal: 20 },
  scoreCard: { backgroundColor: colors.surface, borderRadius: radius.lg, padding: 20, marginTop: 24, ...shadow },
  scoreLabel: { color: colors.muted, textAlign: 'center', fontSize: 10, letterSpacing: 1.2, fontWeight: '800' },
  score: { color: colors.ink, textAlign: 'center', fontSize: 50, fontWeight: '900', marginTop: 2 },
  scoreHint: { color: colors.primary, textAlign: 'center', fontSize: 12, fontWeight: '800' },
  metrics: { flexDirection: 'row', marginTop: 22 },
  gap: { width: 20 },
  sectionTitle: { color: colors.ink, fontSize: 19, fontWeight: '900', marginTop: 28, marginBottom: 12 },
  action: { flexDirection: 'row', backgroundColor: colors.surface, borderRadius: radius.md, padding: 15, marginBottom: 10, borderWidth: 1, borderColor: colors.border },
  actionIcon: { width: 34, height: 34, borderRadius: 11, alignItems: 'center', justifyContent: 'center' },
  actionGood: { backgroundColor: '#E3F3ED' },
  actionBad: { backgroundColor: '#FBE8EB' },
  actionIconText: { color: colors.ink, fontWeight: '900' },
  actionCopy: { flex: 1, marginLeft: 12 },
  actionEvent: { color: colors.muted, fontSize: 10, fontWeight: '800', textTransform: 'uppercase' },
  actionChoice: { color: colors.ink, fontSize: 14, fontWeight: '800', marginTop: 3 },
  actionFeedback: { color: colors.muted, fontSize: 12, lineHeight: 17, marginTop: 6 },
  competency: { color: colors.primary, fontSize: 11, fontWeight: '700', marginTop: 7 },
  primaryButton: { backgroundColor: colors.primary, borderRadius: radius.md, padding: 16, alignItems: 'center', marginTop: 16 },
  primaryText: { color: colors.surface, fontSize: 15, fontWeight: '800' },
  secondaryButton: { padding: 15, alignItems: 'center' },
  secondaryText: { color: colors.muted, fontSize: 14, fontWeight: '700' },
})
