import { useMutation, useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, setWagonSession, useAppDispatch, useAppSelector } from '../app/store'
import { colors, palette, radius, shadow, spacing, type } from '../helpers/theme'
import type { WagonLevel } from '../types'
import { Icon } from './Icon'
import { Button, Card, ErrorText, Pill, Sheet } from './UI'
import { Text } from './Typography'

const NODE = 56
const RAIL = 6

export const classLabel = 'Стандарт'

export function useWagonLevels() {
  return useQuery({ queryKey: ['wagon-levels'], queryFn: api.getWagonLevels, refetchOnMount: 'always' })
}

export function levelStatusLabel(level: WagonLevel, isCurrent: boolean) {
  if (level.status === 'passed') return 'Пройдено'
  if (level.status === 'locked') return 'Закрыто'
  return isCurrent ? 'Текущий этап' : 'Доступно'
}

/** The first unlocked level is where the learner currently is on the route. */
export function currentLevel(levels: WagonLevel[]) {
  return levels.find((item) => item.status === 'unlocked') ?? null
}

export type StationState = 'passed' | 'current' | 'available' | 'locked'

/** One stop on a metro-style route line (Home curriculum map). */
export function Station({ kicker, title, statusLabel, state, isFinal, isFirst, isLast, travelledIn, travelledOut, trailing, onPress }: {
  kicker: string
  title: string
  statusLabel: string
  state: StationState
  isFinal?: boolean
  isFirst: boolean
  isLast: boolean
  travelledIn: boolean
  travelledOut: boolean
  trailing?: ReactNode
  onPress: () => void
}) {
  const passed = state === 'passed'
  const locked = state === 'locked'
  const isCurrent = state === 'current'
  const icon = passed ? 'check' : locked ? 'lock' : isFinal ? 'flag' : 'train'

  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${title}. ${statusLabel}`}
      onPress={onPress}
      style={({ pressed }) => [styles.row, pressed && styles.rowPressed]}
    >
      <View style={styles.track}>
        <View style={[styles.rail, isFirst ? styles.railHidden : travelledIn ? styles.railDone : styles.railFuture]} />
        <View style={[
          styles.node,
          passed && styles.nodePassed,
          isCurrent && styles.nodeCurrent,
          state === 'available' && styles.nodeAvailable,
          locked && styles.nodeLocked,
        ]}>
          <Icon name={icon} size={24} strokeWidth={2.2}
            color={passed ? colors.surface : locked ? colors.locked : colors.action} />
        </View>
        <View style={[styles.rail, isLast ? styles.railHidden : travelledOut ? styles.railDone : styles.railFuture]} />
      </View>
      <View style={[styles.stop, isCurrent && styles.stopCurrent]}>
        {isCurrent && (
          <View style={styles.here}>
            <Icon name="train" size={14} color={colors.surface} strokeWidth={2} />
            <Text style={styles.hereText}>Вы здесь</Text>
          </View>
        )}
        <View style={styles.stopBody}>
          <View style={{ flex: 1 }}>
            <Text style={styles.stopOrder}>{kicker}</Text>
            <Text numberOfLines={2} style={[styles.stopTitle, locked && styles.stopLocked]}>{title}</Text>
            <Text style={[styles.stopStatus, isCurrent && { color: colors.action }, passed && { color: colors.successInk }]}>{statusLabel}</Text>
          </View>
          {trailing}
        </View>
      </View>
      <Icon name="chevronRight" size={20} color={colors.faint} />
    </Pressable>
  )
}

export function ChapterHeader({ kicker, title }: { kicker: string; title: string }) {
  return (
    <View style={styles.chapter}>
      <View style={styles.chapterLine} />
      <View style={styles.chapterText}>
        <Text style={styles.chapterKicker}>{kicker}</Text>
        <Text style={styles.chapterTitle}>{title}</Text>
      </View>
      <View style={styles.chapterLine} />
    </View>
  )
}

export function LevelSheet({ level, levels, onClose }: { level: WagonLevel | null; levels: WagonLevel[]; onClose: () => void }) {
  const dispatch = useAppDispatch()
  const activeSessionId = useAppSelector((state) => state.app.wagonSessionId)
  const start = useMutation({
    mutationFn: api.startWagonSession,
    onSuccess: (result) => { onClose(); dispatch(setWagonSession({ sessionId: result.session_id, wsPath: result.ws_path })) },
  })
  if (!level) return null

  const index = levels.findIndex((item) => item.id === level.id)
  const previous = index > 0 ? levels[index - 1] : null
  const current = currentLevel(levels)
  const passed = level.status === 'passed'
  const locked = level.status === 'locked'
  const title = passed ? 'Завершённый рейс' : locked ? 'Рейс закрыт' : 'Текущий рейс'

  return (
    <Sheet visible title={title} onClose={onClose}>
      <Card style={styles.sheetCard}>
        <View style={styles.sheetTop}>
          <Text style={styles.sheetTitle}>{level.title}</Text>
          {passed && <Pill label="Завершён" tone="success" />}
          {locked && <Pill label="Закрыт" tone="neutral" icon="lock" />}
          {!passed && !locked && <Pill label={current?.id === level.id ? 'Текущий этап' : 'Доступен'} tone="progress" />}
        </View>
        <Text style={styles.sheetMeta}>Класс «{classLabel}» · этап {level.order} из {levels.length}</Text>
        {locked ? (
          <Text style={styles.sheetBody}>
            {previous ? `Завершите рейс «${previous.title}», чтобы открыть этот этап.` : 'Этап пока недоступен.'}
          </Text>
        ) : level.intro ? <Text style={styles.sheetBody}>{level.intro}</Text> : null}
      </Card>

      <ErrorText error={start.error ? start.error.message || 'Не удалось начать рейс' : null} />

      {activeSessionId ? (
        <Button title="Вернуться в вагон" icon="chevronRight" onPress={() => { onClose(); dispatch(navigate('wagon')) }} />
      ) : locked ? (
        <Button title="Понятно" variant="secondary" onPress={onClose} />
      ) : passed ? (
        <Button title="Повторить рейс" variant="secondary" icon="refresh" busy={start.isPending} onPress={() => start.mutate(level.id)} />
      ) : (
        <Button title="Начать рейс" icon="chevronRight" busy={start.isPending} onPress={() => start.mutate(level.id)} />
      )}
    </Sheet>
  )
}

const styles = StyleSheet.create({
  chapter: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, marginTop: spacing.lg, marginBottom: spacing.sm },
  chapterLine: { flex: 1, height: 1, backgroundColor: colors.border },
  chapterText: { alignItems: 'center', maxWidth: '70%' },
  chapterKicker: { ...type.label, color: colors.muted, letterSpacing: 0.6 },
  chapterTitle: { ...type.cardTitle, color: colors.ink, textAlign: 'center', marginTop: 2 },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, minHeight: 104 },
  rowPressed: { opacity: 0.8 },
  track: { width: NODE, alignSelf: 'stretch', alignItems: 'center' },
  rail: { flex: 1, width: RAIL },
  railDone: { backgroundColor: colors.action },
  railFuture: { width: 4, backgroundColor: palette.n200 },
  railHidden: { backgroundColor: 'transparent' },
  node: { width: NODE, height: NODE, borderRadius: NODE / 2, alignItems: 'center', justifyContent: 'center', borderWidth: 3, backgroundColor: colors.surface, borderColor: colors.border },
  nodePassed: { backgroundColor: colors.action, borderColor: colors.action },
  nodeCurrent: { borderColor: colors.action, borderWidth: 4, ...shadow },
  nodeAvailable: { borderColor: colors.action },
  nodeLocked: { backgroundColor: colors.soft, borderColor: colors.border },
  stop: { flex: 1, paddingVertical: spacing.sm, paddingHorizontal: 0 },
  stopCurrent: { backgroundColor: colors.surface, borderRadius: radius.card, borderWidth: 1, borderColor: colors.border, paddingHorizontal: spacing.md, marginVertical: spacing.xs, ...shadow },
  here: { flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start', gap: 5, height: 24, paddingHorizontal: 10, borderRadius: radius.pill, backgroundColor: colors.primary, marginBottom: 6 },
  hereText: { ...type.label, color: colors.surface, fontWeight: '600' },
  stopBody: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  stopOrder: { ...type.label, color: colors.muted },
  stopTitle: { fontSize: 16, lineHeight: 22, fontWeight: '600', color: colors.ink, marginTop: 2 },
  stopLocked: { color: colors.muted },
  stopStatus: { ...type.label, color: colors.muted, marginTop: 2 },
  sheetCard: { ...shadow, borderColor: colors.border },
  sheetTop: { flexDirection: 'row', alignItems: 'flex-start', justifyContent: 'space-between', gap: spacing.xs },
  sheetTitle: { ...type.h3, color: colors.ink, flex: 1 },
  sheetMeta: { ...type.secondary, color: colors.secondary, marginTop: 6 },
  sheetBody: { ...type.body, color: colors.secondary, marginTop: spacing.sm },
})
