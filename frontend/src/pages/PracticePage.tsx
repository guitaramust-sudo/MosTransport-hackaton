import { useState } from 'react'
import { StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, setLiveSimulation, setShift, useAppDispatch, useAppSelector } from '../app/store'
import { Icon, type IconName } from '../components/Icon'
import { classLabel, currentLevel, LevelSheet, levelStatusLabel, useWagonLevels } from '../components/RouteMap'
import { Text } from '../components/Typography'
import { Card, ErrorText, Pill, type PillTone, Screen, Section } from '../components/UI'
import { colors, radius, spacing, type } from '../helpers/theme'
import type { WagonLevel } from '../types'

function statusTone(level: WagonLevel): PillTone {
  if (level.status === 'passed') return 'success'
  if (level.status === 'locked') return 'neutral'
  return 'progress'
}

function ModeCard({ icon, title, text, meta, onPress, disabled }: {
  icon: IconName; title: string; text: string; meta: string; onPress: () => void; disabled?: boolean
}) {
  return (
    <Card onPress={disabled ? undefined : onPress} accessibilityLabel={title} style={styles.mode}>
      <View style={styles.modeIcon}><Icon name={icon} size={26} color={colors.action} /></View>
      <View style={styles.modeBody}>
        <Text style={styles.modeTitle}>{title}</Text>
        <Text style={styles.modeText}>{text}</Text>
        <Text style={styles.modeMeta}>{meta}</Text>
      </View>
      <Icon name="chevronRight" size={22} color={colors.faint} />
    </Card>
  )
}

export function PracticePage() {
  const dispatch = useAppDispatch()
  const { shift, liveSimulation, wagonSessionId } = useAppSelector((state) => state.app)
  const levelsQuery = useWagonLevels()
  const levels = levelsQuery.data?.levels ?? []
  const current = currentLevel(levels)
  const [openId, setOpenId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function startClassic() {
    if (shift?.session.status === 'active') { dispatch(navigate('simulation')); return }
    setBusy(true); setError(null)
    try { dispatch(setShift(await api.startSession())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать смену') }
    finally { setBusy(false) }
  }

  async function startLive() {
    if (liveSimulation?.run.status === 'active') { dispatch(navigate('live_simulation')); return }
    setBusy(true); setError(null)
    try { dispatch(setLiveSimulation(await api.startLiveSimulation())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать симуляцию') }
    finally { setBusy(false) }
  }

  return (
    <Screen title="Практика" subtitle="Выбор и повтор рейсов">
      <Card style={styles.context}>
        <View style={styles.contextIcon}><Icon name="train" size={26} color={colors.primary} /></View>
        <View style={{ flex: 1 }}>
          <Text style={styles.contextLabel}>Назначенный класс</Text>
          <Text style={styles.contextTitle}>{classLabel}</Text>
          <Text style={styles.contextText}>Класс назначает наставник. Это рабочий контекст, а не уровень сложности.</Text>
        </View>
      </Card>

      {wagonSessionId && (
        <Card onPress={() => dispatch(navigate('wagon'))} accessibilityLabel="Вернуться в активный рейс" style={styles.resume}>
          <Icon name="play" size={22} color={colors.action} />
          <View style={{ flex: 1 }}>
            <Text style={styles.resumeTitle}>Активный рейс</Text>
            <Text style={styles.resumeText}>Вернуться в вагон и продолжить</Text>
          </View>
          <Icon name="chevronRight" size={22} color={colors.action} />
        </Card>
      )}

      <Section title="Рейсы в вагоне">
        <Card style={styles.list}>
          {levelsQuery.isPending && <Text style={styles.muted}>Загружаем рейсы…</Text>}
          <ErrorText error={levelsQuery.error ? levelsQuery.error.message || 'Не удалось загрузить рейсы' : null} />
          {levels.map((level, index) => {
            const isCurrent = level.id === current?.id
            return (
              <View key={level.id}>
                {index > 0 && <View style={styles.divider} />}
                <Card onPress={() => setOpenId(level.id)} accessibilityLabel={`${level.title}. ${levelStatusLabel(level, isCurrent)}`} style={styles.levelRow}>
                  <View style={[styles.levelNum, level.status === 'passed' && styles.levelNumDone, level.status === 'locked' && styles.levelNumLocked]}>
                    {level.status === 'passed'
                      ? <Icon name="check" size={20} color={colors.surface} strokeWidth={2.4} />
                      : level.status === 'locked'
                        ? <Icon name="lock" size={18} color={colors.locked} />
                        : <Text style={styles.levelNumText}>{level.order}</Text>}
                  </View>
                  <View style={{ flex: 1 }}>
                    <Text style={[styles.levelTitle, level.status === 'locked' && { color: colors.muted }]}>{level.title}</Text>
                    <View style={{ marginTop: 6 }}><Pill label={levelStatusLabel(level, isCurrent)} tone={statusTone(level)} /></View>
                  </View>
                  <Icon name="chevronRight" size={20} color={colors.faint} />
                </Card>
              </View>
            )
          })}
        </Card>
      </Section>

      <Section title="Другие тренировки">
        <ModeCard icon="chat" title="Разговор с пассажиром" disabled={busy}
          text="Ветвящийся сценарий: решения меняют ситуацию и её исход."
          meta={liveSimulation?.run.status === 'active' ? 'Продолжить начатый рейс' : 'Участвует в челлендже недели'}
          onPress={startLive} />
        <ModeCard icon="shield" title="Стандарты обслуживания" disabled={busy}
          text="Последовательные обращения пассажиров в одной смене."
          meta={shift?.session.status === 'active' ? 'Продолжить смену' : 'Короткая смена'}
          onPress={startClassic} />
        <ErrorText error={error} />
      </Section>

      <LevelSheet level={levels.find((item) => item.id === openId) ?? null} levels={levels} onClose={() => setOpenId(null)} />
    </Screen>
  )
}

const styles = StyleSheet.create({
  context: { flexDirection: 'row', gap: spacing.sm, alignItems: 'flex-start' },
  contextIcon: { width: 52, height: 52, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  contextLabel: { ...type.label, color: colors.muted },
  contextTitle: { ...type.cardTitle, color: colors.primary, marginTop: 2 },
  contextText: { ...type.secondary, color: colors.secondary, marginTop: 4 },
  resume: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, backgroundColor: colors.blueSoft, borderColor: '#D3DEFF' },
  resumeTitle: { ...type.cardTitle, color: colors.primary },
  resumeText: { ...type.secondary, color: colors.secondary },
  list: { paddingVertical: 4, paddingHorizontal: 16 },
  divider: { height: 1, backgroundColor: colors.divider },
  levelRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, borderWidth: 0, borderRadius: radius.md, paddingHorizontal: 0, paddingVertical: 14, marginBottom: 0 },
  levelNum: { width: 40, height: 40, borderRadius: 20, alignItems: 'center', justifyContent: 'center', borderWidth: 2, borderColor: colors.action, backgroundColor: colors.surface },
  levelNumDone: { backgroundColor: colors.action },
  levelNumLocked: { borderColor: colors.border, backgroundColor: colors.soft },
  levelNumText: { fontSize: 16, fontWeight: '700', color: colors.action },
  levelTitle: { fontSize: 16, lineHeight: 22, fontWeight: '600', color: colors.ink },
  muted: { ...type.secondary, color: colors.secondary, paddingVertical: spacing.md },
  mode: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  modeIcon: { width: 52, height: 52, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  modeBody: { flex: 1 },
  modeTitle: { ...type.cardTitle, color: colors.ink },
  modeText: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  modeMeta: { ...type.label, color: colors.action, marginTop: 6 },
})
