import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, openLesson, setPlayer, useAppDispatch, useAppSelector } from '../app/store'
import { Icon } from '../components/Icon'
import { Logo } from '../components/Logo'
import { CurriculumMap, currentLesson, lessonCounts, useLearningMap } from '../components/CurriculumMap'
import { Text } from '../components/Typography'
import { Avatar, Card, Pill, ProgressBar, Screen } from '../components/UI'
import { XP_PER_LEVEL, levelProgress } from '../helpers/progression'
import { colors, radius, spacing, type } from '../helpers/theme'

export function HomePage() {
  const dispatch = useAppDispatch()
  const { auth, wagonSessionId } = useAppSelector((state) => state.app)
  const profile = useQuery({ queryKey: ['profile'], queryFn: api.profile })
  const challenge = useQuery({ queryKey: ['weekly-challenge'], queryFn: api.weeklyChallenge })
  const map = useLearningMap()
  const current = currentLesson(map.data)
  const { total, completed } = lessonCounts(map.data)
  const allDone = total > 0 && completed === total

  useEffect(() => { if (profile.data) dispatch(setPlayer(profile.data.player)) }, [dispatch, profile.data])

  const name = auth?.player.username ?? 'проводник'
  const xp = Math.max(0, profile.data?.player.total_xp ?? auth?.player.total_xp ?? 0)
  const points = profile.data?.leaderboard_points_total ?? 0
  const { level, inLevel, percent } = levelProgress(xp, profile.data?.level)

  function continueTraining() {
    if (wagonSessionId) dispatch(navigate('wagon'))
    else if (current) dispatch(openLesson(current.lesson_id))
    else dispatch(navigate('practice'))
  }

  return (
    <Screen>
      <View style={styles.top}>
        <Logo size={30} caption={false} />
        <Pressable accessibilityRole="button" accessibilityLabel="Профиль" onPress={() => dispatch(navigate('profile'))}>
          <Avatar name={name} size={44} />
        </Pressable>
      </View>

      <Text style={styles.greeting}>Здравствуйте, {name}</Text>

      <View style={styles.levelRow} accessibilityLabel={`Уровень ${level}, ${inLevel} из ${XP_PER_LEVEL} XP до следующего`}>
        <View style={styles.levelChip}>
          <Icon name="star" size={16} color={colors.surface} strokeWidth={2.2} />
          <Text style={styles.levelChipText}>Уровень {level}</Text>
        </View>
        <View style={{ flex: 1 }}><ProgressBar value={percent} height={8} label="XP до следующего уровня" /></View>
        <Text style={styles.levelXp}>{inLevel}/{XP_PER_LEVEL} XP</Text>
      </View>
      <View style={styles.chips}>
        <View style={styles.chip}><Icon name="progress" size={18} color={colors.action} /><Text style={styles.chipText}>{xp} XP всего</Text></View>
        <View style={styles.chip}><Icon name="trophy" size={18} color={colors.action} /><Text style={styles.chipText}>{points} очков рейтинга</Text></View>
      </View>

      <View style={styles.hero}>
        <View style={styles.heroLines} pointerEvents="none">
          <View style={[styles.line, styles.lineA]} />
          <View style={[styles.line, styles.lineB]} />
          <View style={[styles.line, styles.lineC]} />
        </View>
        <Text style={styles.heroKicker}>{wagonSessionId ? 'АКТИВНЫЙ РЕЙС' : allDone ? 'ПРОГРАММА ПРОЙДЕНА' : current ? 'ТЕКУЩИЙ УРОК' : 'ПРОГРАММА'}</Text>
        <Text style={styles.heroTitle}>
          {wagonSessionId ? 'Рейс ещё идёт' : allDone ? 'Все доступные уроки пройдены' : current?.title ?? 'Загрузка программы'}
        </Text>
        <Text style={styles.heroMeta}>Пройдено уроков: {completed} из {total || '—'}</Text>
        <View style={styles.heroProgress}>
          <ProgressBar value={total ? (completed / total) * 100 : 0} color={colors.surface} trackColor="rgba(255,255,255,0.22)" height={6} label="Пройдено уроков" />
        </View>
        <Pressable accessibilityRole="button" onPress={continueTraining} style={({ pressed }) => [styles.heroButton, pressed && { opacity: 0.9 }]}>
          <Text style={styles.heroButtonText}>{wagonSessionId ? 'Вернуться в вагон' : allDone ? 'Свободная практика' : 'Продолжить тренировку'}</Text>
          <Icon name="chevronRight" size={20} color={colors.action} strokeWidth={2.2} />
        </Pressable>
      </View>

      {challenge.data && (
        <Card onPress={() => dispatch(navigate('practice'))} accessibilityLabel="Челлендж недели" style={styles.challenge}>
          <View style={styles.challengeIcon}><Icon name="target" size={24} color={colors.action} /></View>
          <View style={styles.challengeBody}>
            <View style={styles.challengeTop}>
              <Text style={styles.challengeTitle}>Челлендж недели</Text>
              {challenge.data.completed
                ? <Pill label="Выполнено" tone="success" />
                : <Text style={styles.challengeReward}>+{challenge.data.reward_xp} XP</Text>}
            </View>
            <Text style={styles.challengeText}>Разные зачтённые рейсы в диалоговом тренажёре</Text>
            <View style={styles.challengeProgress}>
              <View style={{ flex: 1 }}>
                <ProgressBar value={(Math.min(challenge.data.seed_variants, challenge.data.target) / Math.max(1, challenge.data.target)) * 100} label="Прогресс челленджа" />
              </View>
              <Text style={styles.challengeCount}>{Math.min(challenge.data.seed_variants, challenge.data.target)}/{challenge.data.target}</Text>
            </View>
          </View>
        </Card>
      )}

      <CurriculumMap />
    </Screen>
  )
}

const styles = StyleSheet.create({
  top: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', minHeight: 56 },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.xs, marginTop: spacing.sm, marginBottom: spacing.md },
  levelRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  levelChip: { flexDirection: 'row', alignItems: 'center', gap: 5, height: 30, paddingHorizontal: 12, borderRadius: radius.pill, backgroundColor: colors.action },
  levelChipText: { ...type.label, fontWeight: '600', color: colors.surface },
  levelXp: { ...type.label, color: colors.secondary },
  chip: { flexDirection: 'row', alignItems: 'center', gap: 6, height: 36, paddingHorizontal: 12, borderRadius: radius.pill, backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.border },
  chipText: { ...type.secondary, fontWeight: '600', color: colors.ink },
  greeting: { ...type.h1, color: colors.ink, marginTop: spacing.sm, marginBottom: spacing.sm },
  hero: { borderRadius: radius.lg, padding: 20, backgroundColor: colors.primary, overflow: 'hidden' },
  heroLines: { ...StyleSheet.absoluteFill },
  line: { position: 'absolute', right: -40, borderRadius: 40, transform: [{ rotate: '-12deg' }] },
  lineA: { top: 18, width: '72%', height: 30, backgroundColor: colors.action, opacity: 0.55 },
  lineB: { top: 54, width: '60%', height: 6, backgroundColor: colors.surface, opacity: 0.9 },
  lineC: { top: 66, width: '46%', height: 4, backgroundColor: colors.brandRed },
  heroKicker: { ...type.label, color: '#BCCBFF', letterSpacing: 0.8, marginTop: 70 },
  heroTitle: { ...type.h2, color: colors.surface, marginTop: 4 },
  heroMeta: { ...type.secondary, color: '#D5DEFA', marginTop: 4 },
  heroProgress: { marginTop: spacing.sm },
  heroButton: { minHeight: 52, marginTop: spacing.md, borderRadius: radius.button, backgroundColor: colors.surface, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: spacing.xs },
  heroButtonText: { fontSize: 16, lineHeight: 22, fontWeight: '600', color: colors.action },
  challenge: { flexDirection: 'row', gap: spacing.sm, marginTop: spacing.md, marginBottom: 0 },
  challengeIcon: { width: 48, height: 48, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  challengeBody: { flex: 1 },
  challengeTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: spacing.xs },
  challengeTitle: { ...type.cardTitle, color: colors.ink, flexShrink: 1 },
  challengeReward: { ...type.label, color: colors.warningInk, fontWeight: '600' },
  challengeText: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  challengeProgress: { flexDirection: 'row', alignItems: 'center', gap: spacing.xs, marginTop: spacing.sm },
  challengeCount: { ...type.label, color: colors.secondary },
})
