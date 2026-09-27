import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native'
import { api } from '../api/client'
import { navigate, setWagonSession, useAppDispatch, useAppSelector } from '../app/store'
import { Text } from '../components/Typography'
import { colors, radius, shadow } from '../helpers/theme'
import type { WagonLevel } from '../types'

const NODE_SIZE = 78
const ROW_HEIGHT = 154
const nodePositions = [0.12, 0.66, 0.2, 0.58]

function nodeX(index: number, width: number) {
  const available = Math.max(0, width - NODE_SIZE - 24)
  return 12 + available * nodePositions[index % nodePositions.length]
}

function LevelNode({ level, index, width, selected, hasNext, onPress }: {
  level: WagonLevel
  index: number
  width: number
  selected: boolean
  hasNext: boolean
  onPress: () => void
}) {
  const x = nodeX(index, width)
  const nextX = nodeX(index + 1, width)
  const currentCenter = x + NODE_SIZE / 2
  const nextCenter = nextX + NODE_SIZE / 2
  const passed = level.status === 'passed'
  const unlocked = level.status === 'unlocked'
  const locked = level.status === 'locked'
  const pathColor = passed ? '#8060DF' : unlocked ? '#E8B820' : '#D7DCE7'

  return (
    <View style={[styles.levelRow, { width }]}>
      {hasNext && <>
        <View style={[styles.connectorVertical, { left: currentCenter - 3, backgroundColor: pathColor }]} />
        <View style={[styles.connectorHorizontal, { left: Math.min(currentCenter, nextCenter), width: Math.abs(nextCenter - currentCenter) + 6, backgroundColor: pathColor }]} />
        <View style={[styles.connectorNext, { left: nextCenter - 3, backgroundColor: pathColor }]} />
      </>}
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`${level.title}. ${passed ? 'Пройден' : unlocked ? 'Доступен' : 'Закрыт'}`}
        onPress={onPress}
        style={({ pressed }) => [
          styles.nodeWrap,
          { left: x },
          pressed && !locked && styles.nodePressed,
        ]}
      >
        <View style={[
          styles.nodeShadow,
          passed && styles.nodeShadowPassed,
          unlocked && styles.nodeShadowCurrent,
          locked && styles.nodeShadowLocked,
        ]} />
        <View style={[
          styles.node,
          passed && styles.nodePassed,
          unlocked && styles.nodeCurrent,
          locked && styles.nodeLocked,
          selected && styles.nodeSelected,
        ]}>
          <Text style={[styles.nodeIcon, unlocked && styles.nodeIconCurrent, locked && styles.nodeIconLocked]}>{passed ? '✓' : unlocked ? '⚡' : '◆'}</Text>
        </View>
        <View style={[styles.orderBadge, unlocked && styles.orderBadgeCurrent]}><Text style={[styles.orderText, unlocked && styles.orderTextCurrent]}>{level.order}</Text></View>
      </Pressable>
      <View style={[styles.nodeLabel, { left: Math.max(0, Math.min(width - 146, x - 34)) }]}>
        <Text numberOfLines={2} style={[styles.nodeTitle, locked && styles.nodeTitleLocked]}>{level.title}</Text>
        <Text style={[styles.nodeStatus, passed && styles.nodeStatusPassed, unlocked && styles.nodeStatusCurrent]}>{passed ? 'Пройдено' : unlocked ? 'Текущий уровень' : 'Закрыто'}</Text>
      </View>
    </View>
  )
}

export function WagonLobbyPage() {
  const dispatch = useAppDispatch()
  const { width: screenWidth } = useWindowDimensions()
  const pathWidth = Math.min(520, Math.max(280, screenWidth - 40))
  const savedSessionId = useAppSelector((state) => state.app.wagonSessionId)
  const levelsQuery = useQuery({ queryKey: ['wagon-levels'], queryFn: api.getWagonLevels, refetchOnMount: 'always' })
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const start = useMutation({
    mutationFn: api.startWagonSession,
    onSuccess: (result) => dispatch(setWagonSession({ sessionId: result.session_id, wsPath: result.ws_path })),
  })
  const levels = levelsQuery.data?.levels ?? []
  const selected = levels.find((item) => item.id === selectedId) ?? levels.find((item) => item.status === 'unlocked') ?? levels[0]
  const passedCount = levels.filter((item) => item.status === 'passed').length
  const progress = levels.length ? Math.round((passedCount / levels.length) * 100) : 0
  const dots = useMemo(() => Array.from({ length: 42 }, (_, index) => ({
    left: `${8 + ((index * 29) % 84)}%` as `${number}%`,
    top: 18 + Math.floor(index / 7) * 92 + (index % 3) * 9,
  })), [])

  useEffect(() => {
    if (!selectedId && levels.length) setSelectedId((levels.find((item) => item.status === 'unlocked') ?? levels[0]).id)
  }, [levels, selectedId])

  const canStart = selected && selected.status !== 'locked' && !savedSessionId
  const startTitle = savedSessionId ? 'Сначала завершите активную смену' : selected?.status === 'passed' ? 'Повторить уровень' : 'Начать уровень'

  return (
    <ScrollView style={styles.page} contentContainerStyle={styles.content} showsVerticalScrollIndicator={false}>
      <View style={styles.header}>
        <Pressable onPress={() => dispatch(navigate('home'))} style={styles.back}><Text style={styles.backText}>‹</Text></Pressable>
        <View><Text style={styles.kicker}>ПРОГРАММА ПОДГОТОВКИ ВСМ</Text><Text style={styles.title}>Карта обучения</Text></View>
      </View>

      <View style={styles.progressCard}>
        <View style={styles.progressCopy}>
          <Text style={styles.progressKicker}>СТАНДАРТНЫЙ ВАГОН</Text>
          <Text style={styles.progressTitle}>{selected?.title ?? 'Загрузка уровней'}</Text>
          <Text style={styles.progressCaption}>{passedCount} из {levels.length || '—'} уровней пройдено</Text>
        </View>
        <View style={styles.progressCircle}>
          <View style={[styles.progressArc, progress === 0 && styles.progressArcEmpty]} />
          <Text style={styles.progressValue}>{progress}%</Text>
        </View>
      </View>

      {savedSessionId && (
        <Pressable onPress={() => dispatch(navigate('wagon'))} style={styles.resume}>
          <View><Text style={styles.resumeTitle}>Активная смена</Text><Text style={styles.resumeText}>Вернуться в вагон и продолжить уровень</Text></View>
          <Text style={styles.resumeArrow}>→</Text>
        </Pressable>
      )}

      {selected && (
        <View style={styles.introCard}>
          <View style={styles.introTop}><Text style={styles.introLevel}>УРОВЕНЬ {selected.order}</Text><Text style={[styles.introStatus, selected.status === 'passed' && styles.introStatusPassed]}>{selected.status === 'passed' ? 'ПРОЙДЕН' : selected.status === 'unlocked' ? 'ДОСТУПЕН' : 'ЗАКРЫТ'}</Text></View>
          <Text style={styles.introTitle}>{selected.title}</Text>
          <Text style={styles.introText}>{selected.intro ?? 'Завершите предыдущий уровень, чтобы открыть описание и начать эту смену.'}</Text>
          <Pressable disabled={!canStart || start.isPending} onPress={() => start.mutate(selected.id)} style={[styles.startButton, (!canStart || start.isPending) && styles.startButtonDisabled]}>
            {start.isPending ? <ActivityIndicator color="#FFFFFF" size="small" /> : <><Text style={styles.startButtonText}>{startTitle}</Text><Text style={styles.startArrow}>→</Text></>}
          </Pressable>
        </View>
      )}

      <View style={styles.mapHeader}><Text style={styles.mapTitle}>Маршрут проводника</Text><Text style={styles.mapHint}>Нажмите на уровень, чтобы узнать подробности</Text></View>
      <View style={[styles.path, { width: pathWidth, minHeight: Math.max(220, levels.length * ROW_HEIGHT) }]}>
        {dots.map((dot, index) => <View key={index} style={[styles.dot, dot]} />)}
        {levelsQuery.isPending && <ActivityIndicator style={styles.pathLoader} color={colors.primary} size="large" />}
        {levels.map((level, index) => <LevelNode key={level.id} level={level} index={index} width={pathWidth} selected={selected?.id === level.id} hasNext={index < levels.length - 1} onPress={() => setSelectedId(level.id)} />)}
      </View>
      {(levelsQuery.error || start.error) && <Text style={styles.error}>{(levelsQuery.error ?? start.error)?.message ?? 'Не удалось загрузить уровни'}</Text>}
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: '#F5F6FB' }, content: { padding: 20, paddingBottom: 54, alignItems: 'center' },
  header: { width: '100%', maxWidth: 520, flexDirection: 'row', alignItems: 'center', gap: 13, marginBottom: 15 },
  back: { width: 42, height: 42, borderRadius: 14, backgroundColor: colors.surface, alignItems: 'center', justifyContent: 'center', borderWidth: 1, borderColor: colors.border },
  backText: { color: colors.primary, fontSize: 31, lineHeight: 34, fontWeight: '500' },
  kicker: { color: colors.primary, fontSize: 9, fontWeight: '900', letterSpacing: 1.1 }, title: { color: colors.ink, fontSize: 26, fontWeight: '900', marginTop: 2 },
  progressCard: { width: '100%', maxWidth: 520, minHeight: 128, flexDirection: 'row', alignItems: 'center', borderRadius: 32, padding: 20, backgroundColor: '#F0F0FA', borderWidth: 2, borderColor: '#E4E5F0', ...shadow },
  progressCopy: { flex: 1, paddingRight: 10 }, progressKicker: { color: '#8060DF', fontSize: 9, fontWeight: '900', letterSpacing: 1 }, progressTitle: { color: '#252743', fontSize: 21, lineHeight: 25, fontWeight: '900', marginTop: 5 }, progressCaption: { color: '#8588A0', fontSize: 10, marginTop: 6 },
  progressCircle: { width: 68, height: 68, borderRadius: 34, alignItems: 'center', justifyContent: 'center', backgroundColor: '#FFFFFF', borderWidth: 6, borderColor: '#E1E3EC' }, progressArc: { position: 'absolute', width: 68, height: 68, borderRadius: 34, borderWidth: 6, borderLeftColor: '#8060DF', borderTopColor: '#8060DF', borderRightColor: '#8060DF', borderBottomColor: 'transparent', transform: [{ rotate: '35deg' }] }, progressArcEmpty: { borderColor: '#E1E3EC' }, progressValue: { color: '#252743', fontSize: 15, fontWeight: '900' },
  resume: { width: '100%', maxWidth: 520, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 15, borderRadius: radius.lg, backgroundColor: '#E8F6EE', borderWidth: 1, borderColor: '#B7E4C9', marginTop: 12 }, resumeTitle: { color: '#137A48', fontSize: 14, fontWeight: '900' }, resumeText: { color: '#4F7461', fontSize: 10, marginTop: 3 }, resumeArrow: { color: '#137A48', fontSize: 23, fontWeight: '900' },
  introCard: { width: '100%', maxWidth: 520, borderRadius: 24, padding: 17, backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.border, marginTop: 12 }, introTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }, introLevel: { color: colors.primary, fontSize: 9, fontWeight: '900', letterSpacing: .8 }, introStatus: { color: '#9A7510', fontSize: 8, fontWeight: '900', backgroundColor: '#FFF3C4', paddingHorizontal: 8, paddingVertical: 4, borderRadius: 99 }, introStatusPassed: { color: '#5C3DBB', backgroundColor: '#EEE8FF' }, introTitle: { color: colors.ink, fontSize: 18, fontWeight: '900', marginTop: 8 }, introText: { color: colors.muted, fontSize: 12, lineHeight: 18, marginTop: 6 },
  startButton: { height: 45, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', borderRadius: 15, backgroundColor: '#8060DF', paddingHorizontal: 15, marginTop: 13 }, startButtonDisabled: { opacity: .42 }, startButtonText: { color: '#FFFFFF', fontSize: 12, fontWeight: '900' }, startArrow: { color: '#FFFFFF', fontSize: 19, fontWeight: '900' },
  mapHeader: { width: '100%', maxWidth: 520, marginTop: 25, marginBottom: 10 }, mapTitle: { color: colors.ink, fontSize: 18, fontWeight: '900' }, mapHint: { color: colors.muted, fontSize: 10, marginTop: 3 },
  path: { position: 'relative', overflow: 'hidden', borderRadius: 28, backgroundColor: '#F8F8FD', borderWidth: 1, borderColor: '#ECECF4', paddingTop: 16 }, pathLoader: { marginTop: 70 }, dot: { position: 'absolute', width: 8, height: 8, borderRadius: 4, backgroundColor: '#ECECF5' },
  levelRow: { position: 'relative', height: ROW_HEIGHT },
  connectorVertical: { position: 'absolute', top: NODE_SIZE + 4, width: 6, height: 30, borderRadius: 4 }, connectorHorizontal: { position: 'absolute', top: NODE_SIZE + 31, height: 6, borderRadius: 4 }, connectorNext: { position: 'absolute', top: NODE_SIZE + 31, width: 6, height: 41, borderRadius: 4 },
  nodeWrap: { position: 'absolute', top: 0, width: NODE_SIZE, height: NODE_SIZE }, nodePressed: { transform: [{ scale: .96 }] },
  nodeShadow: { position: 'absolute', left: 0, top: 10, width: NODE_SIZE, height: NODE_SIZE, borderRadius: 23, backgroundColor: '#C8CCD7' }, nodeShadowPassed: { backgroundColor: '#613BCC' }, nodeShadowCurrent: { backgroundColor: '#E8B820' }, nodeShadowLocked: { backgroundColor: '#CFD4DF' },
  node: { width: NODE_SIZE, height: NODE_SIZE, borderRadius: 23, alignItems: 'center', justifyContent: 'center', borderWidth: 4 }, nodePassed: { backgroundColor: '#A991F0', borderColor: '#8060DF' }, nodeCurrent: { backgroundColor: '#FFE88F', borderColor: '#F0C83F' }, nodeLocked: { backgroundColor: '#E8EAF0', borderColor: '#D3D7E0' }, nodeSelected: { borderWidth: 6, borderColor: colors.primary },
  nodeIcon: { color: '#7148DC', fontSize: 36, lineHeight: 40, fontWeight: '900' }, nodeIconCurrent: { color: '#DCA900' }, nodeIconLocked: { color: '#ADB3C0', fontSize: 24 },
  orderBadge: { position: 'absolute', right: -7, top: -7, width: 25, height: 25, borderRadius: 13, alignItems: 'center', justifyContent: 'center', backgroundColor: '#FFFFFF', borderWidth: 2, borderColor: '#8060DF' }, orderBadgeCurrent: { borderColor: '#E8B820' }, orderText: { color: '#8060DF', fontSize: 10, fontWeight: '900' }, orderTextCurrent: { color: '#B48600' },
  nodeLabel: { position: 'absolute', top: NODE_SIZE + 13, width: 146, alignItems: 'center' }, nodeTitle: { color: '#30324B', fontSize: 11, lineHeight: 14, textAlign: 'center', fontWeight: '900' }, nodeTitleLocked: { color: '#9DA2B0' }, nodeStatus: { color: '#999EAC', fontSize: 8, fontWeight: '800', marginTop: 3 }, nodeStatusPassed: { color: '#7048D4' }, nodeStatusCurrent: { color: '#B48600' }, error: { color: colors.critical, textAlign: 'center', marginTop: 14 },
})
