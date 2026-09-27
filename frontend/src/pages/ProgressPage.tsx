import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { Icon } from '../components/Icon'
import { RatingSheet } from '../components/Rating'
import { classLabel, currentLevel, useWagonLevels } from '../components/RouteMap'
import { Text } from '../components/Typography'
import { Card, EmptyState, ErrorText, Pill, type PillTone, ProgressBar, Screen, Section } from '../components/UI'
import { colors, spacing, type } from '../helpers/theme'
import type { CompetencyAssessment } from '../types'

const competencyStatus: Record<string, { label: string; tone: PillTone }> = {
  insufficient: { label: 'Мало данных', tone: 'neutral' },
  provisional: { label: 'Предварительно', tone: 'progress' },
  assessed: { label: 'Оценено', tone: 'success' },
}

function CompetencyRow({ item, first }: { item: CompetencyAssessment; first: boolean }) {
  const status = competencyStatus[item.status] ?? { label: item.status, tone: 'neutral' as PillTone }
  return (
    <View style={[styles.comp, !first && styles.compDivider]}>
      <View style={{ flex: 1 }}>
        <Text style={styles.compName}>{item.name}</Text>
        <Text style={styles.compMeta}>
          {item.evidence_count > 0 ? `Наблюдений: ${item.evidence_count}` : 'Пока нет наблюдений'}
        </Text>
      </View>
      <Pill label={status.label} tone={status.tone} />
    </View>
  )
}

export function ProgressPage() {
  const profile = useQuery({ queryKey: ['profile'], queryFn: api.profile })
  const levelsQuery = useWagonLevels()
  const [ratingOpen, setRatingOpen] = useState(false)
  const levels = levelsQuery.data?.levels ?? []
  const passed = levels.filter((item) => item.status === 'passed').length
  const current = currentLevel(levels)
  const competencies = profile.data?.competencies ?? []

  return (
    <Screen title="Прогресс" subtitle="Освоение класса и компетенции">
      <Card>
        <View>
          <Text style={styles.classTitle}>{classLabel}</Text>
          <Text style={styles.classStage}>
            {current ? `этап «${current.title}»` : levels.length && passed === levels.length ? 'все этапы пройдены' : ''}
          </Text>
        </View>
        <View style={{ marginTop: spacing.sm }}>
          <ProgressBar value={levels.length ? (passed / levels.length) * 100 : 0} height={10} label="Освоение класса" />
        </View>
        <Text style={styles.classCaption}>
          {passed} из {levels.length || '—'} этапов пройдено. Продолжайте практику, чтобы открыть следующие ситуации.
        </Text>
      </Card>

      <Section title="Компетенции">
        <ErrorText error={profile.error ? profile.error.message || 'Не удалось загрузить компетенции' : null} />
        <Card style={styles.compCard}>
          {competencies.length === 0 && !profile.isPending
            ? <EmptyState icon="target" title="Пока нет оценок" text="Компетенции появятся после первых завершённых рейсов." />
            : competencies.map((item, index) => <CompetencyRow key={item.competency_id} item={item} first={index === 0} />)}
        </Card>
        <Text style={styles.note}>Статус показывает, хватает ли наблюдений для оценки. Это игровой прогресс, а не оценка работы.</Text>
      </Section>

      <Section title="Рейтинг">
        <Card onPress={() => setRatingOpen(true)} accessibilityLabel="Открыть рейтинг" style={styles.rating}>
          <View style={styles.ratingIcon}><Icon name="trophy" size={26} color={colors.action} /></View>
          <View style={{ flex: 1 }}>
            <Text style={styles.ratingValue}>{profile.data?.leaderboard_points_total ?? 0} очков</Text>
            <Text style={styles.ratingCaption}>Накоплены за зачтённые прохождения</Text>
          </View>
          <Icon name="chevronRight" size={22} color={colors.faint} />
        </Card>
      </Section>

      <RatingSheet visible={ratingOpen} points={profile.data?.leaderboard_points_total ?? 0} onClose={() => setRatingOpen(false)} />
    </Screen>
  )
}

const styles = StyleSheet.create({
  classTitle: { ...type.h3, color: colors.ink },
  classStage: { ...type.secondary, color: colors.action, marginTop: 2 },
  classCaption: { ...type.secondary, color: colors.secondary, marginTop: spacing.sm },
  compCard: { paddingVertical: 4 },
  comp: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, paddingVertical: 14 },
  compDivider: { borderTopWidth: 1, borderTopColor: colors.divider },
  compName: { fontSize: 16, lineHeight: 22, fontWeight: '600', color: colors.ink },
  compMeta: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  note: { ...type.secondary, color: colors.secondary },
  rating: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  ratingIcon: { width: 52, height: 52, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  ratingValue: { ...type.cardTitle, color: colors.ink },
  ratingCaption: { ...type.secondary, color: colors.secondary },
})
