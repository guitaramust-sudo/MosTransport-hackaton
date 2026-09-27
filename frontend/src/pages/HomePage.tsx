import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Pressable, ScrollView, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, setLiveSimulation, setPlayer, setShift, useAppDispatch, useAppSelector } from '../app/store'
import { Text } from '../components/Typography'
import { colors, radius, shadow } from '../helpers/theme'

export function HomePage() {
  const dispatch = useAppDispatch()
  const { auth, shift, liveSimulation, wagonSessionId } = useAppSelector((state) => state.app)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const wagonLevels = useQuery({ queryKey: ['wagon-levels'], queryFn: api.getWagonLevels })
  const passedLevels = wagonLevels.data?.levels.filter((level) => level.status === 'passed').length ?? 0
  const totalLevels = wagonLevels.data?.levels.length ?? 0
  const wagonProgress = totalLevels ? Math.round((passedLevels / totalLevels) * 100) : 0
  useEffect(() => { api.profile().then((profile) => dispatch(setPlayer(profile.player))).catch(() => {}) }, [dispatch])

  async function startClassic() {
    setBusy(true); setError(null)
    try { dispatch(setShift(await api.startSession())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать смену') }
    finally { setBusy(false) }
  }

  async function startLive() {
    setBusy(true); setError(null)
    try { dispatch(setLiveSimulation(await api.startLiveSimulation())) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось начать симуляцию') }
    finally { setBusy(false) }
  }

  return (
    <ScrollView style={styles.page} contentContainerStyle={styles.content}>
      <View style={styles.header}>
        <View><Text style={styles.brand}>ВСМ</Text><Text style={styles.brandCaption}>ВЫСОКОСКОРОСТНАЯ МАГИСТРАЛЬ</Text></View>
        <Pressable onPress={() => dispatch(navigate('profile'))} style={styles.avatar}><Text style={styles.avatarText}>{auth?.player.username?.slice(0, 1).toUpperCase() ?? 'П'}</Text></Pressable>
      </View>

      <View style={styles.hero}>
        <View style={styles.speedLine} /><View style={[styles.speedLine, styles.speedLineWhite]} /><View style={[styles.speedLine, styles.speedLineRed]} />
        <Text style={styles.heroKicker}>ДВИЖЕНИЕ ОБЪЕДИНЯЕТ</Text>
        <Text style={styles.heroTitle}>Добро пожаловать,{`\n`}{auth?.player.username ?? 'проводник'}</Text>
        <Text style={styles.heroText}>Интерактивный тренажёр обслуживания пассажиров на борту ВСМ</Text>
        <Pressable onPress={() => dispatch(navigate(wagonSessionId ? 'wagon' : 'wagon_lobby'))} style={styles.primaryButton}>
          <Text style={styles.primaryButtonText}>{wagonSessionId ? 'Продолжить смену' : 'Начать обучение'}</Text><Text style={styles.primaryArrow}>→</Text>
        </Pressable>
      </View>

      <View style={styles.progressCard}>
        <View><Text style={styles.cardKicker}>ВАШ ПРОГРЕСС</Text><Text style={styles.xp}>{auth?.player.total_xp ?? 0} <Text style={styles.xpUnit}>XP</Text></Text></View>
        <View style={styles.level}><Text style={styles.levelNumber}>{Math.max(1, Math.floor((auth?.player.total_xp ?? 0) / 250) + 1)}</Text><Text style={styles.levelText}>уровень</Text></View>
      </View>

      <View style={styles.sectionRow}><Text style={styles.sectionTitle}>Модули обучения</Text><Pressable onPress={() => dispatch(navigate('scenarios'))}><Text style={styles.allLink}>Все →</Text></Pressable></View>
      <Pressable onPress={() => dispatch(navigate('learning_map'))} style={styles.moduleCard}>
        <View style={[styles.moduleIcon, styles.moduleIconBlue]}><Text style={styles.moduleIconText}>✦</Text></View>
        <View style={styles.moduleBody}><Text style={styles.moduleTag}>МАРШРУТ</Text><Text style={styles.moduleTitle}>Уроки проводника</Text><Text style={styles.moduleText}>Открытые и пройденные уроки</Text></View><Text style={styles.moduleArrow}>→</Text>
      </Pressable>
      <Pressable disabled={busy} onPress={() => dispatch(navigate('wagon_lobby'))} style={styles.moduleCard}>
        <View style={[styles.moduleIcon, styles.moduleIconBlue]}><Text style={styles.moduleIconText}>▣</Text></View>
        <View style={styles.moduleBody}><Text style={styles.moduleTag}>{passedLevels} ИЗ {totalLevels || '—'} УРОВНЕЙ</Text><Text style={styles.moduleTitle}>Real-time вагон</Text><Text style={styles.moduleText}>Свободное перемещение, живые ситуации и пассажиры</Text><View style={styles.moduleProgress}><View style={[styles.moduleProgressFill, { width: `${Math.max(wagonSessionId ? 4 : 0, wagonProgress)}%` }]} /></View></View>
        <Text style={styles.moduleArrow}>→</Text>
      </Pressable>
      <Pressable disabled={busy} onPress={shift?.session.status === 'active' ? () => dispatch(navigate('simulation')) : startClassic} style={styles.moduleCard}>
        <View style={[styles.moduleIcon, styles.moduleIconRed]}><Text style={styles.moduleIconText}>✓</Text></View>
        <View style={styles.moduleBody}><Text style={styles.moduleTag}>БАЗОВЫЙ КУРС</Text><Text style={styles.moduleTitle}>Стандарты обслуживания</Text><Text style={styles.moduleText}>Последовательные обращения пассажиров</Text></View><Text style={styles.moduleArrow}>→</Text>
      </Pressable>
      <Pressable disabled={busy} onPress={liveSimulation?.run.status === 'active' ? () => dispatch(navigate('live_simulation')) : startLive} style={styles.moduleCard}>
        <View style={[styles.moduleIcon, styles.moduleIconGold]}><Text style={styles.moduleIconText}>◎</Text></View>
        <View style={styles.moduleBody}><Text style={styles.moduleTag}>ДИАЛОГОВЫЙ ТРЕНАЖЁР</Text><Text style={styles.moduleTitle}>Разговор с пассажиром</Text><Text style={styles.moduleText}>Ветвящийся сценарий и оценка решений</Text></View><Text style={styles.moduleArrow}>→</Text>
      </Pressable>
      {error && <Text style={styles.error}>{error}</Text>}
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: colors.soft }, content: { padding: 18, paddingBottom: 36 },
  header: { minHeight: 54, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }, brand: { color: colors.primary, fontSize: 29, fontWeight: '900', letterSpacing: -2 }, brandCaption: { color: colors.primary, fontSize: 7, fontWeight: '900', letterSpacing: .45, marginTop: -3 }, avatar: { width: 44, height: 44, borderRadius: 15, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft, borderWidth: 1, borderColor: '#CBD8FF' }, avatarText: { color: colors.primary, fontSize: 17, fontWeight: '900' },
  hero: { minHeight: 290, overflow: 'hidden', borderRadius: 30, padding: 24, justifyContent: 'flex-end', backgroundColor: colors.primaryDark, ...shadow }, speedLine: { position: 'absolute', top: 46, right: -33, width: '80%', height: 40, borderRadius: 50, backgroundColor: colors.primaryLight, transform: [{ rotate: '-10deg' }] }, speedLineWhite: { top: 63, right: -50, height: 13, backgroundColor: '#FFFFFF' }, speedLineRed: { top: 82, right: -65, height: 9, width: '63%', backgroundColor: colors.critical }, heroKicker: { color: '#AFC4FF', fontSize: 9, fontWeight: '900', letterSpacing: 1.2 }, heroTitle: { color: '#FFFFFF', fontSize: 29, lineHeight: 33, fontWeight: '900', marginTop: 7 }, heroText: { color: '#D4DDF6', fontSize: 13, lineHeight: 19, marginTop: 9, maxWidth: 310 }, primaryButton: { minHeight: 51, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', borderRadius: 17, backgroundColor: colors.primaryLight, paddingHorizontal: 18, marginTop: 19 }, primaryButtonText: { color: '#FFFFFF', fontSize: 14, fontWeight: '900' }, primaryArrow: { color: '#FFFFFF', fontSize: 22, fontWeight: '900' },
  progressCard: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', borderRadius: radius.lg, padding: 17, backgroundColor: colors.surface, marginTop: 14, borderWidth: 1, borderColor: colors.border }, cardKicker: { color: colors.muted, fontSize: 9, fontWeight: '900', letterSpacing: 1 }, xp: { color: colors.ink, fontSize: 26, fontWeight: '900', marginTop: 3 }, xpUnit: { color: colors.primary, fontSize: 13 }, level: { width: 62, height: 62, borderRadius: 31, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft, borderWidth: 5, borderColor: '#BED0FF' }, levelNumber: { color: colors.primary, fontSize: 18, fontWeight: '900' }, levelText: { color: colors.primary, fontSize: 7, fontWeight: '700' },
  sectionRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginTop: 25, marginBottom: 10 }, sectionTitle: { color: colors.ink, fontSize: 19, fontWeight: '900' }, allLink: { color: colors.primary, fontSize: 12, fontWeight: '800' },
  moduleCard: { minHeight: 112, flexDirection: 'row', alignItems: 'center', gap: 12, borderRadius: radius.lg, padding: 14, backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.border, marginBottom: 10 }, moduleIcon: { width: 53, height: 72, borderRadius: 17, alignItems: 'center', justifyContent: 'center' }, moduleIconBlue: { backgroundColor: colors.blueSoft }, moduleIconRed: { backgroundColor: '#FFE9EC' }, moduleIconGold: { backgroundColor: '#FFF2D8' }, moduleIconText: { color: colors.primary, fontSize: 21, fontWeight: '900' }, moduleBody: { flex: 1 }, moduleTag: { color: colors.primary, fontSize: 8, fontWeight: '900', letterSpacing: .7 }, moduleTitle: { color: colors.ink, fontSize: 15, fontWeight: '900', marginTop: 4 }, moduleText: { color: colors.muted, fontSize: 11, lineHeight: 15, marginTop: 4 }, moduleArrow: { color: colors.primary, fontSize: 20, fontWeight: '900' }, moduleProgress: { height: 5, borderRadius: 5, overflow: 'hidden', backgroundColor: '#E6EBF4', marginTop: 8 }, moduleProgressFill: { height: '100%', backgroundColor: colors.loyalty }, error: { color: colors.critical, textAlign: 'center', marginTop: 8 },
})
