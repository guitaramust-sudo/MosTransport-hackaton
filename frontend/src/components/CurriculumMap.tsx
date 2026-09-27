import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { ActivityIndicator, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { openLesson, useAppDispatch } from '../app/store'
import { badgeTitles } from '../assets/badges'
import { lessonBadge } from '../helpers/achievements'
import { colors, radius, spacing, type } from '../helpers/theme'
import type { LearningLessonSummary, LearningMap } from '../types'
import { Icon } from './Icon'
import { ChapterHeader, Station, type StationState } from './RouteMap'
import { Text } from './Typography'
import { Badge, Button, Card, ErrorText, Pill, Sheet } from './UI'

export function useLearningMap() {
  return useQuery({ queryKey: ['learning-map'], queryFn: api.learningMap, refetchOnMount: 'always' })
}

/** The first unlocked lesson in curriculum order is where the learner is. */
export function currentLesson(map: LearningMap | undefined): LearningLessonSummary | null {
  for (const chapter of map?.chapters ?? []) {
    const lesson = chapter.lessons.find((item) => item.status === 'unlocked')
    if (lesson) return lesson
  }
  return null
}

export function lessonCounts(map: LearningMap | undefined) {
  const lessons = map?.chapters.flatMap((chapter) => chapter.lessons) ?? []
  return { total: lessons.length, completed: lessons.filter((item) => item.status === 'completed').length }
}

/** "Глава 1. Первый рейс" -> { number: 1, name: "Первый рейс" }; "(скоро)" marks unreleased chapters. */
function splitChapterTitle(title: string, order: number) {
  const name = title.replace(/^Глава\s*\d+\.?\s*/i, '').replace(/\(скоро\)/i, '').trim()
  return { number: order, name }
}

function stationState(lesson: LearningLessonSummary, isCurrent: boolean): StationState {
  if (lesson.status === 'completed') return 'passed'
  if (lesson.status === 'locked') return 'locked'
  return isCurrent ? 'current' : 'available'
}

function statusLabel(state: StationState) {
  return state === 'passed' ? 'Пройден' : state === 'locked' ? 'Закрыт' : state === 'current' ? 'Текущий урок' : 'Доступен'
}

export function CurriculumMap() {
  const dispatch = useAppDispatch()
  const map = useLearningMap()
  const [locked, setLocked] = useState<LearningLessonSummary | null>(null)
  const current = currentLesson(map.data)
  const allLessons = map.data?.chapters.flatMap((chapter) => chapter.lessons) ?? []

  return (
    <View>
      {map.isPending && <ActivityIndicator style={styles.loader} color={colors.action} size="large" />}
      <ErrorText error={map.error ? map.error.message || 'Не удалось загрузить программу' : null} />
      {map.data?.chapters.map((chapter) => {
        const { number, name } = splitChapterTitle(chapter.title, chapter.order)
        if (chapter.lessons.length === 0) {
          return (
            <View key={chapter.chapter_id} style={styles.soon}>
              <View style={styles.soonIcon}><Icon name="lock" size={18} color={colors.locked} /></View>
              <Text style={styles.soonTitle}>Глава {number}</Text>
              <Pill label="Скоро" tone="neutral" />
            </View>
          )
        }
        return (
          <View key={chapter.chapter_id}>
            <ChapterHeader kicker={`ГЛАВА ${number}`} title={name || chapter.title} />
            {chapter.lessons.map((lesson, index) => {
              const state = stationState(lesson, lesson.lesson_id === current?.lesson_id)
              const flatIndex = allLessons.indexOf(lesson)
              const next = allLessons[flatIndex + 1]
              const badge = lessonBadge(lesson.lesson_id)
              return (
                <Station key={lesson.lesson_id}
                  kicker={`Урок ${lesson.order}`} title={lesson.title} state={state} statusLabel={statusLabel(state)}
                  isFirst={index === 0} isLast={index === chapter.lessons.length - 1}
                  travelledIn={lesson.status !== 'locked'} travelledOut={Boolean(next && next.status !== 'locked')}
                  trailing={badge ? <Badge id={badge} size={44} locked={lesson.status !== 'completed'} /> : undefined}
                  onPress={() => (lesson.status === 'locked' ? setLocked(lesson) : dispatch(openLesson(lesson.lesson_id)))} />
              )
            })}
          </View>
        )
      })}

      <Sheet visible={Boolean(locked)} title="Урок закрыт" onClose={() => setLocked(null)}>
        {locked && (() => {
          const index = allLessons.indexOf(locked)
          const previous = index > 0 ? allLessons[index - 1] : null
          const badge = lessonBadge(locked.lesson_id)
          return <>
            <Card style={styles.lockedCard}>
              {badge && <Badge id={badge} size={64} locked />}
              <View style={{ flex: 1 }}>
                <Text style={styles.lockedTitle}>{locked.title}</Text>
                {badge && <Text style={styles.lockedBadge}>Бейдж «{badgeTitles[badge]}»</Text>}
                <Text style={styles.lockedText}>{previous ? `Пройдите урок «${previous.title}», чтобы открыть этот.` : 'Урок пока недоступен.'}</Text>
              </View>
            </Card>
            <Button title="Понятно" variant="secondary" onPress={() => setLocked(null)} />
          </>
        })()}
      </Sheet>
    </View>
  )
}

const styles = StyleSheet.create({
  loader: { marginTop: 40 },
  soon: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, minHeight: 56, paddingHorizontal: spacing.sm, marginTop: spacing.xs, borderRadius: radius.md, backgroundColor: colors.soft },
  soonIcon: { width: 36, height: 36, borderRadius: 18, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.surface },
  soonTitle: { ...type.body, fontWeight: '600', color: colors.muted, flex: 1 },
  lockedCard: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  lockedTitle: { ...type.cardTitle, color: colors.ink },
  lockedBadge: { ...type.label, color: colors.muted, marginTop: 2 },
  lockedText: { ...type.secondary, color: colors.secondary, marginTop: 6 },
})
