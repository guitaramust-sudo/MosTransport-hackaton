import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { StyleSheet, Switch, View } from 'react-native'
import { api } from '../api/client'
import { navigate, openLesson, signedOut, useAppDispatch, useAppSelector } from '../app/store'
import { badgeTitles } from '../assets/badges'
import { Icon } from '../components/Icon'
import { RatingSheet } from '../components/Rating'
import { classLabel } from '../components/RouteMap'
import { Text } from '../components/Typography'
import { Avatar, Badge, Medal, Button, Card, ErrorText, ListRow, ProgressBar, Screen, Section, SpeedLine } from '../components/UI'
import { achievements, lessonBadge } from '../helpers/achievements'
import { levelProgress } from '../helpers/progression'
import { useMusicSettings } from '../helpers/musicSettings'
import { registerForPushNotifications } from '../helpers/pushNotifications'
import { colors, radius, spacing, type } from '../helpers/theme'

function formatExpiry(iso: string) {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleString('ru-RU', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })
}

export function ProfilePage() {
  const dispatch = useAppDispatch()
  const auth = useAppSelector((state) => state.app.auth)
  const profile = useQuery({ queryKey: ['profile'], queryFn: api.profile })
  const [ratingOpen, setRatingOpen] = useState(false)
  const { enabled: musicEnabled, ready: musicReady, setEnabled: setMusicEnabled } = useMusicSettings()

  const player = profile.data?.player ?? auth?.player
  const xp = Math.max(0, player?.total_xp ?? 0)
  const { level, toNext, percent } = levelProgress(xp, profile.data?.level)
  const earned = new Set(profile.data?.achievements ?? [])
  const points = profile.data?.leaderboard_points_total ?? 0
  const learning = useQuery({ queryKey: ['my-learning'], queryFn: api.myLearning })
  const map = useQuery({ queryKey: ['learning-map'], queryFn: api.learningMap })
  const prize = learning.data
  const lessons = map.data?.chapters.flatMap((chapter) => chapter.lessons) ?? []

  const [pushBusy, setPushBusy] = useState(false)
  const [pushStatus, setPushStatus] = useState<string | null>(null)
  async function enablePush() {
    setPushBusy(true); setPushStatus(null)
    try { await registerForPushNotifications(); setPushStatus('Уведомления включены на этом устройстве') }
    catch (cause) { setPushStatus(cause instanceof Error ? cause.message : 'Не удалось включить уведомления') }
    finally { setPushBusy(false) }
  }

  function signOut() {
    dispatch(signedOut())
  }

  return (
    <Screen title="Профиль">
      <ErrorText error={profile.error ? 'Не удалось обновить профиль. Показаны сохранённые данные.' : null} />

      <Card style={styles.identity}>
        <SpeedLine style={styles.identityLine} />
        <View style={styles.identityRow}>
          <Avatar name={player?.username} size={72} />
          <View style={{ flex: 1 }}>
            <Text style={styles.name}>{player?.username ?? 'Загрузка…'}</Text>
            <Text style={styles.role}>{player?.role === 'admin' ? 'Администратор' : 'Проводник ВСМ'}</Text>
            {player?.email && <Text style={styles.email} numberOfLines={1}>{player.email}</Text>}
          </View>
        </View>
        <View style={styles.classRow}>
          <Text style={styles.classLabel}>Назначенный класс</Text>
          <Text style={styles.classValue}>{classLabel}</Text>
        </View>
      </Card>

      <Card>
        <View style={styles.levelTop}>
          <View>
            <Text style={styles.levelKicker}>Игровой уровень</Text>
            <Text style={styles.levelValue}>{level}</Text>
          </View>
          <View style={{ alignItems: 'flex-end' }}>
            <Text style={styles.levelKicker}>Всего</Text>
            <Text style={styles.xpValue}>{xp} XP</Text>
          </View>
        </View>
        <View style={{ marginTop: spacing.sm }}>
          <ProgressBar value={percent} height={10} label="XP до следующего уровня" />
        </View>
        <Text style={styles.levelCaption}>
          До уровня {level + 1}: {toNext} XP. XP отражает игровой прогресс и не является допуском к работе.
        </Text>
      </Card>

      <Card style={styles.compact}>
        <ListRow icon="progress" title="Мой прогресс" caption={`Освоение класса «${classLabel}» и компетенции`}
          onPress={() => dispatch(navigate('progress'))} />
      </Card>

      <Section title="Бейджи уроков">
        <View style={styles.badges}>
          {lessons.map((lesson) => {
            const badge = lessonBadge(lesson.lesson_id)
            if (!badge) return null
            const got = lesson.status === 'completed'
            return (
              <Card key={lesson.lesson_id} style={styles.badgeCard} onPress={lesson.status === 'locked' ? undefined : () => dispatch(openLesson(lesson.lesson_id))} accessibilityLabel={`Бейдж ${badgeTitles[badge]}`}>
                <Badge id={badge} size={76} locked={!got} />
                <Text style={[styles.badgeTitle, !got && { color: colors.secondary }]}>{badgeTitles[badge]}</Text>
                <Text style={styles.badgeReason}>{got ? `Урок «${lesson.title}» пройден` : `Пройдите урок «${lesson.title}»`}</Text>
              </Card>
            )
          })}
        </View>
      </Section>

      <Section title="Призовые очки">
        <Card>
          <View style={styles.prizeTop}>
            <View style={{ flex: 1 }}>
              <Text style={styles.pointsValue}>{prize?.prize_balance ?? 0}<Text style={styles.prizeOf}> / {prize?.prize_shirt_threshold ?? 50}</Text></Text>
              <Text style={styles.pointsCaption}>до футболки ВСМ</Text>
            </View>
            <View style={styles.prizeIcon}><Icon name="trophy" size={26} color={colors.warningInk} /></View>
          </View>
          <View style={{ marginTop: spacing.sm }}>
            <ProgressBar value={(prize?.prize_shirt_progress ?? 0) * 100} color={colors.gold} height={10} label="Прогресс к футболке" />
          </View>
          <Text style={styles.levelCaption}>
            {prize?.prize_next_expiry
              ? `Ближайшее сгорание: ${formatExpiry(prize.prize_next_expiry)}. Каждая запись действует 6 суток.`
              : '+10 за первый зачёт каждого урока. Каждая запись действует 6 суток.'}
          </Text>
        </Card>
      </Section>

      <Section title="Достижения">
        <View style={styles.badges}>
          {achievements.map((item) => {
            const got = earned.has(item.id)
            return (
              <Card key={item.id} style={styles.badgeCard}>
                <Medal icon={item.icon} tone={item.tone} size={76} locked={!got} />
                <Text style={[styles.badgeTitle, !got && { color: colors.secondary }]}>{item.title}</Text>
                <Text style={styles.badgeReason}>{got ? item.reason : `Как получить: ${item.reason.toLowerCase()}`}</Text>
              </Card>
            )
          })}
        </View>
      </Section>

      <Section title="Накопленные очки">
        <Card onPress={() => setRatingOpen(true)} accessibilityLabel="Открыть рейтинг" style={styles.points}>
          <View style={{ flex: 1 }}>
            <Text style={styles.pointsValue}>{points}</Text>
            <Text style={styles.pointsCaption}>очков за зачтённые прохождения. Отдельно от XP.</Text>
          </View>
          <View style={styles.pointsLink}><Text style={styles.pointsLinkText}>Рейтинг</Text></View>
        </Card>
      </Section>

      <Section title="Настройки">
        <Card style={styles.compact}>
          <View style={styles.settingRow}>
            <View style={styles.settingIcon}><Icon name="settings" size={22} color={colors.action} /></View>
            <View style={styles.settingText}>
              <Text style={styles.settingTitle}>Фоновая музыка</Text>
              <Text style={styles.settingCaption}>Тихая музыка в 3D вагоне</Text>
            </View>
            <Switch accessibilityLabel="Фоновая музыка в 3D вагоне" value={musicEnabled} disabled={!musicReady}
              onValueChange={setMusicEnabled} trackColor={{ false: colors.border, true: colors.primary }} />
          </View>
          <ListRow icon="bell" title="Push-уведомления"
            caption={pushBusy ? 'Подключаем…' : pushStatus ?? 'О новых уроках и сгорании призовых очков'}
            onPress={pushBusy ? undefined : () => void enablePush()} />
        </Card>
      </Section>

      <Card style={[styles.compact, { marginTop: spacing.lg }]}>
        {player?.role === 'admin' && (
          <ListRow icon="admin" title="Админ-консоль" caption="Учётные записи и учебные итоги" onPress={() => dispatch(navigate('admin'))} />
        )}
        <ListRow icon="logout" title="Выйти" tone="danger" onPress={signOut} right={<View />} />
      </Card>

      <RatingSheet visible={ratingOpen} points={points} onClose={() => setRatingOpen(false)} />
    </Screen>
  )
}

const styles = StyleSheet.create({
  identity: { paddingTop: 0, overflow: 'hidden' },
  identityLine: { marginHorizontal: -20, marginBottom: spacing.md },
  identityRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  name: { ...type.h3, color: colors.ink },
  role: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  email: { ...type.label, color: colors.muted, marginTop: 2 },
  classRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginTop: spacing.md, paddingTop: spacing.sm, borderTopWidth: 1, borderTopColor: colors.divider },
  classLabel: { ...type.secondary, color: colors.secondary },
  classValue: { ...type.secondary, fontWeight: '600', color: colors.primary },
  levelTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-end' },
  levelKicker: { ...type.label, color: colors.muted },
  levelValue: { ...type.number, color: colors.primary, marginTop: 2 },
  xpValue: { ...type.h3, color: colors.ink, marginTop: 2 },
  levelCaption: { ...type.secondary, color: colors.secondary, marginTop: spacing.sm },
  compact: { paddingVertical: 4, paddingHorizontal: 16 },
  settingRow: { minHeight: 68, flexDirection: 'row', alignItems: 'center', gap: spacing.sm, paddingHorizontal: 10, paddingVertical: 8 },
  settingIcon: { width: 40, height: 40, borderRadius: 12, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  settingText: { flex: 1 },
  settingTitle: { ...type.secondary, fontWeight: '600', color: colors.ink },
  settingCaption: { ...type.label, color: colors.secondary, marginTop: 2 },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  prizeTop: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  prizeOf: { fontSize: 20, color: colors.muted },
  prizeIcon: { width: 52, height: 52, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.warningSoft },
  badgeCard: { flexGrow: 1, flexBasis: '45%', alignItems: 'center', paddingVertical: 18, paddingHorizontal: 12, marginBottom: 0 },
  badgeTitle: { fontSize: 15, lineHeight: 20, fontWeight: '600', color: colors.ink, marginTop: spacing.sm, textAlign: 'center' },
  badgeReason: { ...type.label, color: colors.secondary, marginTop: 4, textAlign: 'center' },
  points: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  pointsValue: { ...type.number, color: colors.ink },
  pointsCaption: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  pointsLink: { paddingHorizontal: 14, paddingVertical: 8, borderRadius: radius.pill, backgroundColor: colors.blueSoft },
  pointsLinkText: { ...type.secondary, fontWeight: '600', color: colors.action },
})
