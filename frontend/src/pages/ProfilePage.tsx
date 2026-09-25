import { ScrollView, StyleSheet, Text, View } from 'react-native'
import { colors, radius, shadow } from '../helpers/theme'

const skills = [
  { name: 'Безопасность', value: 92, color: colors.safety },
  { name: 'Коммуникация', value: 84, color: colors.loyalty },
  { name: 'Приоритизация', value: 78, color: colors.warning },
  { name: 'Эскалация', value: 88, color: colors.primary },
]

export function ProfilePage() {
  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
      <Text style={styles.kicker}>ЛИЧНЫЙ ПРОФИЛЬ</Text>
      <Text style={styles.title}>Мой прогресс</Text>
      <View style={styles.identity}>
        <View style={styles.avatar}><Text style={styles.avatarText}>АК</Text></View>
        <View><Text style={styles.name}>Алексей Крылов</Text><Text style={styles.role}>Проводник ВСМ · уровень 7</Text></View>
      </View>

      <View style={styles.summary}>
        <View style={styles.summaryItem}><Text style={styles.summaryValue}>24</Text><Text style={styles.summaryLabel}>сценария</Text></View>
        <View style={styles.separator} />
        <View style={styles.summaryItem}><Text style={styles.summaryValue}>1 240</Text><Text style={styles.summaryLabel}>опыта</Text></View>
        <View style={styles.separator} />
        <View style={styles.summaryItem}><Text style={styles.summaryValue}>86%</Text><Text style={styles.summaryLabel}>точность</Text></View>
      </View>

      <Text style={styles.sectionTitle}>Компетенции</Text>
      <View style={styles.skillsCard}>
        {skills.map((skill) => (
          <View style={styles.skill} key={skill.name}>
            <View style={styles.skillRow}><Text style={styles.skillName}>{skill.name}</Text><Text style={[styles.skillValue, { color: skill.color }]}>{skill.value}</Text></View>
            <View style={styles.track}><View style={[styles.fill, { width: `${skill.value}%`, backgroundColor: skill.color }]} /></View>
          </View>
        ))}
      </View>

      <Text style={styles.sectionTitle}>Достижения</Text>
      <View style={styles.achievements}>
        <View style={styles.achievement}><Text style={styles.achievementIcon}>◆</Text><Text style={styles.achievementTitle}>Безопасный рейс</Text><Text style={styles.achievementText}>5 смен без нарушений</Text></View>
        <View style={styles.achievement}><Text style={styles.achievementIcon}>⚡</Text><Text style={styles.achievementTitle}>Быстрое решение</Text><Text style={styles.achievementText}>Ответ менее чем за 10 сек</Text></View>
      </View>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  content: { padding: 20, paddingBottom: 30 },
  kicker: { color: colors.primary, fontSize: 11, fontWeight: '800', letterSpacing: 1.4 },
  title: { color: colors.ink, fontSize: 32, fontWeight: '900', marginTop: 5 },
  identity: { flexDirection: 'row', alignItems: 'center', marginTop: 22 },
  avatar: { width: 60, height: 60, borderRadius: 20, backgroundColor: colors.dark, alignItems: 'center', justifyContent: 'center', marginRight: 14 },
  avatarText: { color: colors.surface, fontSize: 18, fontWeight: '900' },
  name: { color: colors.ink, fontSize: 19, fontWeight: '800' },
  role: { color: colors.muted, fontSize: 13, marginTop: 4 },
  summary: { flexDirection: 'row', backgroundColor: colors.primary, borderRadius: radius.lg, paddingVertical: 20, marginTop: 22, ...shadow },
  summaryItem: { flex: 1, alignItems: 'center' },
  summaryValue: { color: colors.surface, fontSize: 20, fontWeight: '900' },
  summaryLabel: { color: '#FFDDE1', fontSize: 10, marginTop: 3 },
  separator: { width: 1, backgroundColor: 'rgba(255,255,255,0.22)' },
  sectionTitle: { color: colors.ink, fontSize: 19, fontWeight: '900', marginTop: 28, marginBottom: 12 },
  skillsCard: { backgroundColor: colors.surface, borderRadius: radius.lg, padding: 18, borderWidth: 1, borderColor: colors.border },
  skill: { marginBottom: 17 },
  skillRow: { flexDirection: 'row', justifyContent: 'space-between', marginBottom: 7 },
  skillName: { color: colors.ink, fontSize: 13, fontWeight: '700' },
  skillValue: { fontSize: 13, fontWeight: '900' },
  track: { height: 6, backgroundColor: colors.soft, borderRadius: radius.pill, overflow: 'hidden' },
  fill: { height: '100%', borderRadius: radius.pill },
  achievements: { flexDirection: 'row', gap: 10 },
  achievement: { flex: 1, backgroundColor: colors.surface, borderRadius: radius.md, padding: 15, borderWidth: 1, borderColor: colors.border },
  achievementIcon: { color: colors.primary, fontSize: 22 },
  achievementTitle: { color: colors.ink, fontSize: 13, fontWeight: '800', marginTop: 9 },
  achievementText: { color: colors.muted, fontSize: 10, lineHeight: 15, marginTop: 4 },
})
