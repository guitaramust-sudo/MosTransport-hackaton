import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import { StyleSheet, View } from 'react-native'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { Icon } from '../components/Icon'
import { MetricBar } from '../components/MetricBar'
import { Medal, Button, Card, EmptyState, Pill, type PillTone, Screen, Section, Sheet } from '../components/UI'
import { achievements } from '../helpers/achievements'
import { levelProgress } from '../helpers/progression'
import type { Profile } from '../types'
import { Text } from '../components/Typography'
import { colors, radius, spacing, type } from '../helpers/theme'
import { translateBackendField } from '../helpers/backendTranslations'

const outcomes: Record<string, { label: string; tone: PillTone }> = {
  success: { label: 'Успешно', tone: 'success' },
  partial: { label: 'Частично', tone: 'warning' },
  fail: { label: 'Ошибка', tone: 'critical' },
  timeout: { label: 'Время вышло', tone: 'critical' },
}

export function DebriefPage() {
  const dispatch = useAppDispatch()
  const queryClient = useQueryClient()
  const breakdown = useAppSelector((state) => state.app.breakdown)
  const shift = useAppSelector((state) => state.app.shift)

  const [reward, setReward] = useState<{ newAchievements: typeof achievements; newLevel: number | null } | null>(null)
  const rewardChecked = useRef<string | null>(null)

  // A finished run changes levels, XP and points: refresh what the tabs show and
  // celebrate only what the server confirms is new compared with the cached profile.
  useEffect(() => {
    if (!breakdown || rewardChecked.current === breakdown.session_id) return
    rewardChecked.current = breakdown.session_id
    const before = queryClient.getQueryData<Profile>(['profile'])
    void queryClient.invalidateQueries({ queryKey: ['wagon-levels'] })
    void queryClient.invalidateQueries({ queryKey: ['weekly-challenge'] })
    queryClient.fetchQuery({ queryKey: ['profile'], queryFn: api.profile, staleTime: 0 }).then((after) => {
      if (!before) return
      const had = new Set(before.achievements)
      const newAchievements = achievements.filter((item) => after.achievements.includes(item.id) && !had.has(item.id))
      const levelBefore = levelProgress(before.player.total_xp, before.level).level
      const levelAfter = levelProgress(after.player.total_xp, after.level).level
      const newLevel = levelAfter > levelBefore ? levelAfter : null
      if (newAchievements.length || newLevel) setReward({ newAchievements, newLevel })
    }).catch(() => undefined)
  }, [breakdown?.session_id, queryClient])

  // Titles live on the session's situations; the breakdown only carries ids.
  const session = useQuery({
    queryKey: ['session', breakdown?.session_id],
    queryFn: () => api.getSession(breakdown!.session_id),
    enabled: Boolean(breakdown && !shift?.situations.some((item) => breakdown.situations.some((b) => b.situation_id === item.id))),
  })
  const situationInfo = (id: string) => shift?.situations.find((item) => item.id === id) ?? session.data?.situations.find((item) => item.id === id)

  const successCount = breakdown?.situations.filter((item) => item.outcome === 'success').length ?? 0

  return (
    <Screen title="Разбор рейса" subtitle="Что вы заметили, какие решения приняли и к чему они привели">
      {!breakdown ? (
        <Card><EmptyState icon="flag" title="Разбор пока недоступен" text="Он появится после завершения рейса." /></Card>
      ) : <>
        <Card style={styles.summary}>
          <View style={styles.summaryIcon}><Icon name="flag" size={28} color={colors.action} /></View>
          <View style={{ flex: 1 }}>
            <Text style={styles.summaryTitle}>Рейс завершён</Text>
            <Text style={styles.summaryText}>Ситуаций: {breakdown.situations.length}</Text>
            <Text style={styles.summaryText}>Решено успешно: {successCount}</Text>
          </View>
          <View style={styles.xp}>
            <Text style={styles.xpValue}>{breakdown.total_xp > 0 ? '+' : ''}{breakdown.total_xp}</Text>
            <Text style={styles.xpUnit}>XP</Text>
          </View>
        </Card>

        <Section title="Ситуации">
          {breakdown.situations.map((item) => {
            const outcome = outcomes[item.outcome] ?? { label: item.outcome, tone: 'neutral' as PillTone }
            const info = situationInfo(item.situation_id)
            const title = info?.scenario || translateBackendField(item.code, 'Обращение пассажира')
            const passenger = translateBackendField(info?.name ?? item.name, '')
            return (
              <Card key={item.situation_id}>
                <View style={styles.sitTop}>
                  <View style={{ flex: 1 }}>
                    <Text style={styles.sitTitle}>{title}</Text>
                    {passenger ? <Text style={styles.sitPassenger}>{passenger}</Text> : null}
                  </View>
                  <Pill label={outcome.label} tone={outcome.tone} />
                </View>
                <Text style={styles.sitXp}>{item.xp > 0 ? '+' : ''}{item.xp} XP</Text>
                <View style={styles.metrics}>
                  <MetricBar label="Безопасность" value={item.safety} tone="safety" />
                  <MetricBar label="Лояльность" value={item.loyalty} tone="loyalty" />
                </View>
                {item.remarks.length > 0 && (
                  <View style={styles.remarks}>
                    {item.remarks.map((remark, index) => (
                      <View key={`${remark.code}-${index}`} style={styles.remark}>
                        <View style={styles.remarkDot} />
                        <Text style={styles.remarkText}>{remark.message}</Text>
                      </View>
                    ))}
                  </View>
                )}
              </Card>
            )
          })}
        </Section>
      </>}
      <Button title="Продолжить маршрут" icon="chevronRight" onPress={() => dispatch(navigate('home'))} />
      <Button title="К практике" variant="tertiary" onPress={() => dispatch(navigate('practice'))} />

      <Sheet visible={!!reward} title="Поздравляем!" onClose={() => setReward(null)}>
        {reward?.newAchievements.map((item) => (
          <View key={item.id} style={styles.reward}>
            <Medal icon={item.icon} tone={item.tone} size={120} />
            <Text style={styles.rewardTitle}>Новое достижение: «{item.title}»</Text>
            <Text style={styles.rewardText}>{item.reason}</Text>
          </View>
        ))}
        {reward?.newLevel && (
          <View style={styles.levelUp}>
            <View style={styles.levelUpChip}><Icon name="star" size={18} color={colors.surface} strokeWidth={2.2} /><Text style={styles.levelUpText}>Уровень {reward.newLevel}</Text></View>
            <Text style={styles.rewardText}>Вы перешли на новый игровой уровень.</Text>
          </View>
        )}
        <Button title="Отлично" onPress={() => setReward(null)} />
      </Sheet>
    </Screen>
  )
}

const styles = StyleSheet.create({
  summary: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  summaryIcon: { width: 56, height: 56, borderRadius: radius.md, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  summaryTitle: { ...type.cardTitle, color: colors.ink },
  summaryText: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  xp: { alignItems: 'center' },
  xpValue: { ...type.h2, color: colors.primary },
  xpUnit: { ...type.label, color: colors.muted },
  sitTop: { flexDirection: 'row', alignItems: 'flex-start', justifyContent: 'space-between', gap: spacing.xs },
  sitTitle: { ...type.cardTitle, color: colors.ink },
  sitPassenger: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  sitXp: { ...type.secondary, color: colors.action, fontWeight: '600', marginTop: 4 },
  metrics: { flexDirection: 'row', gap: spacing.md, marginTop: spacing.md },
  remarks: { marginTop: spacing.md, paddingTop: spacing.sm, borderTopWidth: 1, borderTopColor: colors.divider, gap: spacing.xs },
  remark: { flexDirection: 'row', gap: spacing.xs },
  remarkDot: { width: 6, height: 6, borderRadius: 3, backgroundColor: colors.action, marginTop: 9 },
  remarkText: { ...type.secondary, color: colors.secondary, flex: 1 },
  reward: { alignItems: 'center', marginBottom: spacing.md },
  rewardTitle: { ...type.h3, color: colors.ink, textAlign: 'center', marginTop: spacing.sm },
  rewardText: { ...type.secondary, color: colors.secondary, textAlign: 'center', marginTop: 4 },
  levelUp: { alignItems: 'center', paddingVertical: spacing.sm },
  levelUpChip: { flexDirection: 'row', alignItems: 'center', gap: 6, height: 36, paddingHorizontal: 16, borderRadius: radius.pill, backgroundColor: colors.action },
  levelUpText: { ...type.secondary, fontWeight: '600', color: colors.surface },
})
