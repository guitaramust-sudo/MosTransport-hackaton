import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { makeChoice, navigate, useAppDispatch, useAppSelector } from '../app/store'
import { MetricBar } from '../components/MetricBar'
import { scenarios } from '../data/scenarios'
import { colors, radius, shadow } from '../helpers/theme'

export function SimulationPage() {
  const dispatch = useAppDispatch()
  const state = useAppSelector((root) => root.app)
  const scenario = scenarios.find((item) => item.id === state.scenarioId) ?? scenarios[0]
  const event = scenario.events[state.eventIndex]
  const pendingCount = Math.max(0, scenario.events.length - state.eventIndex - 1)

  if (!event) return null

  return (
    <View style={styles.screen}>
      <View style={styles.header}>
        <Pressable style={styles.close} onPress={() => dispatch(navigate('scenarios'))}><Text style={styles.closeText}>×</Text></Pressable>
        <View style={styles.headerCenter}>
          <Text style={styles.route}>ВСМ 001 · МОСКВА → СПБ</Text>
          <Text style={styles.clock}>До прибытия 06:42</Text>
        </View>
        <View style={styles.live}><View style={styles.liveDot} /><Text style={styles.liveText}>СМЕНА</Text></View>
      </View>

      <View style={styles.metrics}>
        <MetricBar label="Безопасность" value={state.safety} tone="safety" compact />
        <View style={styles.metricGap} />
        <MetricBar label="Лояльность" value={state.loyalty} tone="loyalty" compact />
      </View>

      <ScrollView contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
        {pendingCount > 0 && (
          <View style={styles.queue}>
            <View style={styles.queueIcon}><Text style={styles.queueIconText}>2</Text></View>
            <View style={styles.queueCopy}>
              <Text style={styles.queueTitle}>Параллельные события</Text>
              <Text style={styles.queueText}>Ещё {pendingCount} ситуация ожидает решения</Text>
            </View>
            <Text style={styles.queueArrow}>⌄</Text>
          </View>
        )}

        <View style={styles.eventCard}>
          <View style={styles.eventTop}>
            <View style={[styles.priority, event.priority === 'critical' ? styles.critical : styles.normal]}>
              <Text style={styles.priorityText}>{event.priority === 'critical' ? 'КРИТИЧЕСКОЕ' : 'СЕРВИС'}</Text>
            </View>
            <Text style={styles.time}>{event.time}</Text>
          </View>
          <Text style={styles.eventTitle}>{event.title}</Text>
          <Text style={styles.location}>⌖  {event.location}</Text>
          <Text style={styles.description}>{event.description}</Text>
          {event.timer && (
            <View style={styles.timer}>
              <View style={styles.timerRing}><Text style={styles.timerValue}>{event.timer}</Text></View>
              <View><Text style={styles.timerTitle}>Время на решение</Text><Text style={styles.timerHint}>Бездействие повлияет на безопасность</Text></View>
            </View>
          )}
        </View>

        <Text style={styles.question}>Как вы поступите?</Text>
        {event.choices.map((choice, index) => (
          <Pressable key={choice.id} onPress={() => dispatch(makeChoice(choice))} style={({ pressed }) => [styles.choice, pressed && styles.choicePressed]}>
            <View style={styles.choiceNumber}><Text style={styles.choiceNumberText}>{index + 1}</Text></View>
            <View style={styles.choiceCopy}>
              <Text style={styles.choiceTitle}>{choice.title}</Text>
              <Text style={styles.choiceSubtitle}>{choice.subtitle}</Text>
            </View>
            <Text style={styles.choiceArrow}>›</Text>
          </Pressable>
        ))}
        <Text style={styles.hint}>Решение нельзя будет отменить</Text>
      </ScrollView>
    </View>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#F4F7F8' },
  header: { flexDirection: 'row', alignItems: 'center', backgroundColor: colors.surface, paddingHorizontal: 16, paddingVertical: 12, borderBottomWidth: 1, borderBottomColor: colors.border },
  close: { width: 38, height: 38, borderRadius: 13, backgroundColor: colors.soft, alignItems: 'center', justifyContent: 'center' },
  closeText: { color: colors.ink, fontSize: 26, lineHeight: 28, fontWeight: '300' },
  headerCenter: { flex: 1, alignItems: 'center' },
  route: { color: colors.ink, fontSize: 11, fontWeight: '800', letterSpacing: 0.5 },
  clock: { color: colors.muted, fontSize: 11, marginTop: 3 },
  live: { width: 52, alignItems: 'center' },
  liveDot: { width: 8, height: 8, borderRadius: 4, backgroundColor: colors.primary, marginBottom: 3 },
  liveText: { color: colors.primary, fontSize: 8, fontWeight: '900' },
  metrics: { flexDirection: 'row', backgroundColor: colors.surface, paddingHorizontal: 20, paddingVertical: 12 },
  metricGap: { width: 22 },
  content: { padding: 16, paddingBottom: 32 },
  queue: { flexDirection: 'row', alignItems: 'center', backgroundColor: colors.dark, borderRadius: radius.md, padding: 13, marginBottom: 14 },
  queueIcon: { width: 34, height: 34, borderRadius: 11, backgroundColor: colors.primary, alignItems: 'center', justifyContent: 'center' },
  queueIconText: { color: colors.surface, fontWeight: '900' },
  queueCopy: { flex: 1, marginLeft: 11 },
  queueTitle: { color: colors.surface, fontSize: 13, fontWeight: '800' },
  queueText: { color: '#AFC0C7', fontSize: 11, marginTop: 2 },
  queueArrow: { color: colors.surface, fontSize: 18 },
  eventCard: { backgroundColor: colors.surface, borderRadius: radius.lg, padding: 20, borderWidth: 1, borderColor: '#E9EDEF', ...shadow },
  eventTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  priority: { borderRadius: radius.pill, paddingVertical: 6, paddingHorizontal: 10 },
  critical: { backgroundColor: '#FBE8EB' },
  normal: { backgroundColor: '#FFF0DD' },
  priorityText: { color: colors.primary, fontSize: 10, fontWeight: '900', letterSpacing: 0.8 },
  time: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  eventTitle: { color: colors.ink, fontSize: 25, lineHeight: 31, fontWeight: '900', marginTop: 18 },
  location: { color: colors.primary, fontSize: 12, fontWeight: '700', marginTop: 8 },
  description: { color: colors.muted, fontSize: 15, lineHeight: 23, marginTop: 16 },
  timer: { flexDirection: 'row', alignItems: 'center', backgroundColor: '#FFF6F0', borderRadius: radius.md, padding: 12, marginTop: 18 },
  timerRing: { width: 44, height: 44, borderRadius: 22, borderWidth: 4, borderColor: colors.primary, alignItems: 'center', justifyContent: 'center', marginRight: 12 },
  timerValue: { color: colors.primary, fontSize: 16, fontWeight: '900' },
  timerTitle: { color: colors.ink, fontSize: 12, fontWeight: '800' },
  timerHint: { color: colors.muted, fontSize: 10, marginTop: 3 },
  question: { color: colors.ink, fontSize: 18, fontWeight: '900', marginTop: 22, marginBottom: 12 },
  choice: { flexDirection: 'row', alignItems: 'center', backgroundColor: colors.surface, borderRadius: radius.md, padding: 14, marginBottom: 10, borderWidth: 1, borderColor: colors.border },
  choicePressed: { borderColor: colors.primary, backgroundColor: '#FFF8F8' },
  choiceNumber: { width: 32, height: 32, borderRadius: 10, backgroundColor: colors.soft, alignItems: 'center', justifyContent: 'center' },
  choiceNumberText: { color: colors.ink, fontWeight: '800' },
  choiceCopy: { flex: 1, marginLeft: 12 },
  choiceTitle: { color: colors.ink, fontSize: 14, fontWeight: '800' },
  choiceSubtitle: { color: colors.muted, fontSize: 11, lineHeight: 16, marginTop: 3 },
  choiceArrow: { color: colors.muted, fontSize: 26 },
  hint: { color: '#8B969B', fontSize: 11, textAlign: 'center', marginTop: 5 },
})
