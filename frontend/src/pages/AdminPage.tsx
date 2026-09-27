import { useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { Badge, Button, Card, ErrorText, Page } from '../components/UI'
import { Icon } from '../components/Icon'
import { Text, TextInput } from '../components/Typography'
import { achievements, lessonBadge } from '../helpers/achievements'
import { colors, radius } from '../helpers/theme'
import type { AdminLearningSummary } from '../types'

type Section = 'players' | 'summary'
const sections: Array<{ id: Section; title: string }> = [
  { id: 'players', title: 'Игроки' }, { id: 'summary', title: 'Результаты' },
]
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
function message(cause: unknown) {
  const raw = cause instanceof Error ? cause.message : ''
  const known: Record<string, string> = {
    'email already registered': 'Этот адрес почты уже зарегистрирован.',
    'user not found': 'Игрок с таким ID не найден.',
  }
  return known[raw] ?? (raw.startsWith('HTTP 403') ? 'У этого аккаунта нет прав администратора.' : raw || 'Не удалось выполнить запрос')
}
const statusLabel = { passed: 'Пройден', unlocked: 'Доступен', locked: 'Закрыт' }
function lessonStatusLabel(lesson: AdminLearningSummary['lesson_progression']['lessons'][number]) {
  if (lesson.completed) return 'Пройден'
  if (lesson.practice_pass) return 'Практика сдана'
  if (lesson.theory_pass) return 'Теория сдана'
  return 'Не начат'
}
function formatDate(iso: string) {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleDateString('ru-RU')
}

function Field({ label, value, onChangeText, secureTextEntry, autoCapitalize = 'none', hint }: {
  label: string; value: string; onChangeText: (value: string) => void; secureTextEntry?: boolean;
  autoCapitalize?: 'none' | 'sentences'; hint?: string
}) {
  return <View style={styles.field}>
    <Text style={styles.label}>{label}</Text>
    <TextInput style={styles.input} value={value} onChangeText={onChangeText} secureTextEntry={secureTextEntry} autoCapitalize={autoCapitalize} />
    {hint && <Text style={styles.hint}>{hint}</Text>}
  </View>
}

export function AdminPage() {
  const dispatch = useAppDispatch()
  const role = useAppSelector((state) => state.app.auth?.player.role)
  const [section, setSection] = useState<Section>('players')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [brigade, setBrigade] = useState('')
  const [playerId, setPlayerId] = useState('')
  const [summary, setSummary] = useState<AdminLearningSummary | null>(null)

  function selectSection(next: Section) { setSection(next); setError(null); setNotice(null) }
  async function run(action: () => Promise<void>) {
    setBusy(true); setError(null); setNotice(null)
    try { await action() } catch (cause) { setError(message(cause)) } finally { setBusy(false) }
  }

  if (role !== 'admin') return <Page title="Админка"><Card><Text>Доступен только администратору.</Text><Button title="На главную" onPress={() => dispatch(navigate('home'))} /></Card></Page>

  return <Page title="Админ-консоль" onBack={() => dispatch(navigate('profile'))}>
    <View style={styles.tabs}>{sections.map((item) => <Pressable key={item.id} onPress={() => selectSection(item.id)} style={[styles.tab, section === item.id && styles.activeTab]}><Text style={[styles.tabText, section === item.id && styles.activeTabText]}>{item.title}</Text></Pressable>)}</View>
    <ErrorText error={error} />
    {notice && <Text style={styles.notice}>{notice}</Text>}

    {section === 'players' && <Card>
      <Text style={styles.heading}>Создать игровой аккаунт</Text>
      <Text style={styles.hint}>Игрок войдёт самостоятельно с указанными почтой и паролем.</Text>
      <Field label="Электронная почта" value={email} onChangeText={setEmail} />
      <Field label="Имя игрока" value={username} onChangeText={setUsername} />
      <Field label="Пароль" value={password} onChangeText={setPassword} secureTextEntry hint="Не менее 6 символов" />
      <Field label="Бригада" value={brigade} onChangeText={setBrigade} hint="Свободный текст; по нему работает рейтинг бригады" />
      <Button title="Создать игрока" busy={busy} disabled={!email.trim() || !username.trim() || password.length < 6 || !brigade.trim()} onPress={() => run(async () => {
        const result = await api.createPlayer({ email: email.trim(), username: username.trim(), password, brigade_name: brigade.trim() })
        setNotice(`Игрок ${result.player.username} создан. ID: ${result.player.id}`)
        setPlayerId(result.player.id); setPassword('')
      })} />
    </Card>}

    {section === 'summary' && <>
      <Card><Text style={styles.heading}>Учебная сводка игрока</Text>
        <Field label="ID игрока (UUID)" value={playerId} onChangeText={setPlayerId} hint="Укажите ID, выданный при создании аккаунта" />
        <Button title="Загрузить сводку" busy={busy} disabled={!uuidPattern.test(playerId.trim())} onPress={() => run(async () => { setSummary(null); setSummary(await api.learningSummary(playerId.trim())) })} />
      </Card>
      {summary && <>
        <Card><Text style={styles.heading}>{summary.subject.display_name || 'Игрок'}</Text><Text style={styles.muted}>ID: {summary.subject.user_id}</Text>
          <Text style={styles.line}>Всего опыта: {summary.total_xp} (уровень {summary.player_level})</Text>
          <Text style={styles.line}>Очки рейтинга: {summary.leaderboard_points}</Text>
          <Text style={styles.line}>Завершено смен: {summary.session_outcomes.completed_count}</Text>
          <Text style={styles.line}>Успешных смен: {summary.session_outcomes.passed_count}</Text>
        </Card>

        <Card><Text style={styles.heading}>Достижения</Text>
          {summary.achievements.length ? summary.achievements.map((code) => {
            const entry = achievements.find((item) => item.id === code)
            return <View key={code} style={styles.row}>
              <View style={styles.achievementRow}>
                {entry && <Icon name={entry.icon} size={18} color={colors.primary} />}
                <Text style={styles.rowTitle}>{entry?.title ?? code}</Text>
              </View>
              {entry && <Text style={styles.muted}>{entry.reason}</Text>}
            </View>
          }) : <Text style={styles.muted}>Пока нет достижений</Text>}
        </Card>

        <Card><Text style={styles.heading}>Уровни вагона</Text>{summary.wagon_progression.levels.map((level) => <View key={level.level_id} style={styles.row}><Text style={styles.rowTitle}>{level.order}. {level.title}</Text><Text style={styles.muted}>{statusLabel[level.status]} · попыток: {level.attempts}</Text></View>)}</Card>

        <Card><Text style={styles.heading}>Уроки</Text>
          {summary.lesson_progression.lessons.map((lesson) => {
            const badge = lesson.badge_id ? lessonBadge(lesson.lesson_id) : null
            return <View key={lesson.lesson_id} style={styles.row}>
              <View style={styles.achievementRow}>
                {badge && <Badge id={badge} size={28} locked={!lesson.completed} />}
                <Text style={styles.rowTitle}>{lesson.title}</Text>
              </View>
              <Text style={styles.muted}>{lessonStatusLabel(lesson)}{lesson.xp_earned > 0 ? ` · XP: ${lesson.xp_earned}` : ''}</Text>
              {lesson.completed_at && <Text style={styles.muted}>Пройден: {formatDate(lesson.completed_at)}</Text>}
            </View>
          })}
        </Card>

        <Card><Text style={styles.heading}>Призовые баллы</Text>
          <Text style={styles.line}>{summary.prize_balance.balance} / {summary.prize_balance.shirt_threshold}</Text>
          <Text style={styles.muted}>Прогресс: {Math.round(summary.prize_balance.shirt_progress * 100)}%</Text>
          {summary.prize_balance.balance === 0 && <Text style={styles.muted}>Активных призовых баллов нет</Text>}
          {summary.prize_balance.next_expiry_at && <Text style={styles.muted}>Ближайшее истечение: {formatDate(summary.prize_balance.next_expiry_at)}</Text>}
        </Card>

        <Card><Text style={styles.heading}>Компетенции</Text>{summary.competencies.length ? summary.competencies.map((item) => <View key={item.competency_id} style={styles.row}><Text style={styles.rowTitle}>{item.name}</Text><Text style={styles.muted}>Баллы: {item.score ?? 'нет данных'} · свидетельств: {item.confidence}</Text></View>) : <Text style={styles.muted}>Данных пока нет</Text>}</Card>

        <Card><Text style={styles.heading}>Последние смены</Text>{summary.session_outcomes.recent_assessments.length ? summary.session_outcomes.recent_assessments.map((item) => <View key={item.session_id} style={styles.row}><Text style={styles.rowTitle}>{item.session_pass ? 'Успешно' : 'Не пройдено'} · безопасность {item.session_safety_score}</Text><Text style={styles.muted}>Лояльность {item.loyalty} · нарушений {item.critical_violations}</Text><Text style={styles.muted}>ID: {item.session_id}</Text></View>) : <Text style={styles.muted}>Смен пока нет</Text>}</Card>

        <Text style={styles.hint}>Актуально на: {formatDate(summary.provenance.as_of)}</Text>
      </>}
    </>}
  </Page>
}

const styles = StyleSheet.create({
  tabs: { flexDirection: 'row', flexWrap: 'wrap', gap: 7, marginBottom: 12 },
  tab: { paddingHorizontal: 12, paddingVertical: 10, borderRadius: radius.md, backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.border },
  activeTab: { backgroundColor: colors.primary, borderColor: colors.primary },
  tabText: { color: colors.primary, fontWeight: '700', fontSize: 12 }, activeTabText: { color: colors.surface },
  heading: { color: colors.ink, fontSize: 19, fontWeight: '800', marginBottom: 6 },
  field: { marginTop: 13 }, label: { color: colors.ink, fontWeight: '700', marginBottom: 5 },
  input: { borderWidth: 1, borderColor: colors.border, backgroundColor: colors.surface, borderRadius: radius.md, padding: 12, fontSize: 16, color: colors.ink },
  hint: { color: colors.muted, fontSize: 12, lineHeight: 18, marginTop: 5, marginBottom: 8 },
  notice: { color: colors.safety, fontWeight: '700', marginBottom: 12 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 4 }, line: { color: colors.ink, marginTop: 9 },
  row: { paddingVertical: 10, borderBottomWidth: 1, borderBottomColor: colors.border }, rowTitle: { color: colors.ink, fontWeight: '700' },
  achievementRow: { flexDirection: 'row', alignItems: 'center', gap: 6 },
})
