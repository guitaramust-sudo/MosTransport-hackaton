import { useMutation, useQuery } from '@tanstack/react-query'
import { Pressable, ScrollView, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, setWagonSession, useAppDispatch, useAppSelector } from '../app/store'
import { Text } from '../components/Typography'
import { colors, radius, shadow } from '../helpers/theme'

const futureClasses = [
  { id: 'comfort', name: 'Комфорт', description: 'Повышенные требования к сервису' },
  { id: 'business', name: 'Бизнес', description: 'Персональное обслуживание пассажиров' },
  { id: 'first', name: 'Первый класс', description: 'Максимальный уровень сложности' },
]

export function WagonLobbyPage() {
  const dispatch = useAppDispatch()
  const savedSessionId = useAppSelector((state) => state.app.wagonSessionId)
  const levels = useQuery({ queryKey: ['wagon-levels'], queryFn: api.getWagonLevels })
  const start = useMutation({
    mutationFn: api.startWagonSession,
    onSuccess: (result) => dispatch(setWagonSession({ sessionId: result.session_id, wsPath: result.ws_path })),
  })

  return (
    <ScrollView style={styles.page} contentContainerStyle={styles.content}>
      <View style={styles.header}>
        <Pressable onPress={() => dispatch(navigate('home'))} style={styles.back}><Text style={styles.backText}>‹</Text></Pressable>
        <View><Text style={styles.kicker}>ТРЕНАЖЁР ВСМ</Text><Text style={styles.title}>Выберите уровень</Text></View>
      </View>
      <View style={styles.hero}>
        <View style={styles.heroLine} /><View style={[styles.heroLine, styles.heroLineRed]} />
        <Text style={styles.heroMark}>ВСМ</Text>
        <Text style={styles.heroTitle}>Смена начинается</Text>
        <Text style={styles.heroText}>Проходите смены по порядку: от знакомства с вагоном до сложных ситуаций.</Text>
      </View>
      {savedSessionId && (
        <Pressable onPress={() => dispatch(navigate('wagon'))} style={styles.resume}>
          <View><Text style={styles.resumeTitle}>Активная смена</Text><Text style={styles.resumeText}>Продолжить с последнего состояния</Text></View>
          <Text style={styles.resumeArrow}>→</Text>
        </Pressable>
      )}
      <Text style={styles.sectionTitle}>Уровни · Стандарт</Text>
      {levels.isPending && <Text style={styles.classDescription}>Загружаем уровни…</Text>}
      {levels.data?.levels.map((item) => {
        const available = item.status !== 'locked'
        const badge = item.status === 'passed' ? 'ПРОЙДЕНО' : available ? 'ОТКРЫТО' : 'ЗАКРЫТО'
        return (
          <Pressable key={item.id} disabled={!available || start.isPending} onPress={() => start.mutate(item.id)} style={[styles.classCard, available && styles.classCardPrimary, !available && styles.locked]}>
            <View style={[styles.number, available && styles.numberPrimary]}><Text style={[styles.numberText, available && styles.numberTextPrimary]}>{item.order}</Text></View>
            <View style={styles.classBody}><View style={styles.classRow}><Text style={styles.className}>{item.title}</Text><Text style={[styles.badge, available && styles.badgeAvailable]}>{start.isPending && start.variables === item.id ? 'ЗАПУСК…' : badge}</Text></View><Text style={styles.classDescription}>{item.intro ?? 'Завершите предыдущий уровень, чтобы открыть этот.'}</Text></View>
          </Pressable>
        )
      })}
      <Text style={styles.sectionTitle}>Другие классы · скоро</Text>
      {futureClasses.map((item) => <View key={item.id} style={[styles.classCard, styles.locked]}>
        <View style={styles.classBody}><View style={styles.classRow}><Text style={styles.className}>{item.name}</Text><Text style={styles.badge}>СКОРО</Text></View><Text style={styles.classDescription}>{item.description}</Text></View>
      </View>)}
      {(levels.error || start.error) && <Text style={styles.error}>{(levels.error ?? start.error)?.message ?? 'Не удалось открыть уровни'}</Text>}
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: colors.soft }, content: { padding: 20, paddingBottom: 44 },
  header: { flexDirection: 'row', alignItems: 'center', gap: 13, marginBottom: 18 }, back: { width: 42, height: 42, borderRadius: 14, backgroundColor: colors.surface, alignItems: 'center', justifyContent: 'center', borderWidth: 1, borderColor: colors.border }, backText: { color: colors.primary, fontSize: 31, lineHeight: 34, fontWeight: '500' },
  kicker: { color: colors.primary, fontSize: 10, fontWeight: '900', letterSpacing: 1.3 }, title: { color: colors.ink, fontSize: 27, fontWeight: '900', marginTop: 2 },
  hero: { minHeight: 190, overflow: 'hidden', borderRadius: 28, padding: 24, backgroundColor: colors.primaryDark, marginBottom: 17, ...shadow },
  heroLine: { position: 'absolute', width: '75%', height: 9, borderRadius: 20, right: -30, top: 35, backgroundColor: '#FFFFFF', transform: [{ rotate: '-12deg' }] }, heroLineRed: { top: 52, right: -54, width: '60%', backgroundColor: colors.critical },
  heroMark: { color: '#FFFFFF', fontSize: 34, fontWeight: '900', letterSpacing: -2 }, heroTitle: { color: '#FFFFFF', fontSize: 23, fontWeight: '900', marginTop: 25 }, heroText: { color: '#DCE6FF', fontSize: 14, lineHeight: 20, marginTop: 8, maxWidth: 310 },
  resume: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', padding: 17, borderRadius: radius.lg, backgroundColor: '#E8F6EE', borderWidth: 1, borderColor: '#B7E4C9', marginBottom: 20 }, resumeTitle: { color: '#137A48', fontSize: 15, fontWeight: '900' }, resumeText: { color: '#4F7461', fontSize: 12, marginTop: 3 }, resumeArrow: { color: '#137A48', fontSize: 25, fontWeight: '900' },
  sectionTitle: { color: colors.ink, fontSize: 18, fontWeight: '900', marginBottom: 10 },
  classCard: { minHeight: 91, flexDirection: 'row', alignItems: 'center', gap: 14, backgroundColor: colors.surface, borderRadius: radius.lg, borderWidth: 1, borderColor: colors.border, padding: 15, marginBottom: 10 }, classCardPrimary: { borderWidth: 2, borderColor: colors.primaryLight }, locked: { opacity: .55 },
  number: { width: 47, height: 47, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.soft }, numberPrimary: { backgroundColor: colors.blueSoft }, numberText: { color: colors.muted, fontSize: 18, fontWeight: '900' }, numberTextPrimary: { color: colors.primary }, classBody: { flex: 1 }, classRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }, className: { color: colors.ink, fontSize: 17, fontWeight: '900' }, classDescription: { color: colors.muted, fontSize: 12, marginTop: 6 }, badge: { color: colors.muted, backgroundColor: colors.soft, paddingHorizontal: 8, paddingVertical: 5, borderRadius: 99, fontSize: 9, fontWeight: '900' }, badgeAvailable: { color: colors.primary, backgroundColor: colors.blueSoft }, error: { color: colors.critical, textAlign: 'center', marginTop: 12 },
})
