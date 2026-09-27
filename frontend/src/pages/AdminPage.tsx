import { useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { Text, TextInput } from '../components/Typography'
import { colors, radius } from '../helpers/theme'
import type { AdminLearningSummary } from '../types'

type Section = 'players' | 'external' | 'summary' | 'approval'
const sections: Array<{ id: Section; title: string }> = [
  { id: 'players', title: 'Игроки' }, { id: 'external', title: 'HR / SSO' },
  { id: 'summary', title: 'Результаты' }, { id: 'approval', title: 'Смены' },
]
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
function message(cause: unknown) {
  const raw = cause instanceof Error ? cause.message : ''
  const known: Record<string, string> = {
    'email already registered': 'Этот адрес почты уже зарегистрирован.',
    'user not found': 'Игрок с таким ID не найден.',
    'session not found': 'Смена с таким ID не найдена.',
    'session is not finished': 'Сначала завершите смену.',
    'session contains unapproved content': 'В смене есть сценарии без утверждения.',
  }
  return known[raw] ?? (raw.startsWith('HTTP 403') ? 'У этого аккаунта нет прав администратора.' : raw || 'Не удалось выполнить запрос')
}
const statusLabel = { passed: 'Пройден', unlocked: 'Доступен', locked: 'Закрыт' }

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
  const [source, setSource] = useState('')
  const [externalId, setExternalId] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [depot, setDepot] = useState('')
  const [externalBrigade, setExternalBrigade] = useState('')
  const [classIds, setClassIds] = useState('')
  const [playerId, setPlayerId] = useState('')
  const [sessionId, setSessionId] = useState('')
  const [summary, setSummary] = useState<AdminLearningSummary | null>(null)

  function selectSection(next: Section) { setSection(next); setError(null); setNotice(null) }
  async function run(action: () => Promise<void>) {
    setBusy(true); setError(null); setNotice(null)
    try { await action() } catch (cause) { setError(message(cause)) } finally { setBusy(false) }
  }

  if (role !== 'admin') return <Page title="Админка"><Card><Text>Доступен только администратору.</Text><Button title="На главную" onPress={() => dispatch(navigate('home'))} /></Card></Page>

  return <Page title="Админка">
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

    {section === 'external' && <Card>
      <Text style={styles.heading}>Внешний профиль HR / SSO</Text>
      <Text style={styles.hint}>Повторная отправка той же пары «система + ID» обновит профиль. Вход по паролю для него не создаётся.</Text>
      <Field label="Система-источник" value={source} onChangeText={setSource} />
      <Field label="Внешний ID сотрудника" value={externalId} onChangeText={setExternalId} />
      <Field label="Имя для отображения" value={displayName} onChangeText={setDisplayName} autoCapitalize="sentences" />
      <Field label="Депо" value={depot} onChangeText={setDepot} />
      <Field label="Бригада" value={externalBrigade} onChangeText={setExternalBrigade} />
      <Field label="Классы через запятую" value={classIds} onChangeText={setClassIds} />
      <Button title="Сохранить профиль" busy={busy} disabled={!source.trim() || !externalId.trim()} onPress={() => run(async () => {
        const result = await api.upsertExternalUser({ source_system: source.trim(), external_user_id: externalId.trim(), ...(displayName.trim() ? { display_name: displayName.trim() } : {}), ...(depot.trim() ? { depot_id: depot.trim() } : {}), ...(externalBrigade.trim() ? { brigade_id: externalBrigade.trim() } : {}), assigned_class_ids: classIds.split(',').map((id) => id.trim()).filter(Boolean) })
        setNotice(`${result.created ? 'Профиль создан' : 'Профиль обновлён'}. ID: ${result.user_id}`)
        setPlayerId(result.user_id)
      })} />
    </Card>}

    {section === 'summary' && <>
      <Card><Text style={styles.heading}>Учебная сводка игрока</Text>
        <Field label="ID игрока (UUID)" value={playerId} onChangeText={setPlayerId} hint="Укажите ID, выданный при создании аккаунта" />
        <Button title="Загрузить сводку" busy={busy} disabled={!uuidPattern.test(playerId.trim())} onPress={() => run(async () => { setSummary(null); setSummary(await api.learningSummary(playerId.trim())) })} />
      </Card>
      {summary && <>
        <Card><Text style={styles.heading}>{summary.subject.display_name || 'Игрок'}</Text><Text style={styles.muted}>ID: {summary.subject.user_id}</Text>
          <Text style={styles.line}>Утверждено смен: {summary.session_outcomes.approved_completed_count}</Text>
          <Text style={styles.line}>Успешных смен: {summary.session_outcomes.approved_passed_count}</Text>
          <Text style={styles.line}>Прогресс вагона: {summary.wagon_progression.current_progress} / {summary.wagon_progression.levels.length}</Text>
          {summary.data_status === 'no_approved_data' && <Text style={styles.hint}>Пока нет утверждённых результатов диалоговых смен.</Text>}
        </Card>
        <Card><Text style={styles.heading}>Уровни вагона</Text>{summary.wagon_progression.levels.map((level) => <View key={level.level_id} style={styles.row}><Text style={styles.rowTitle}>{level.order}. {level.title}</Text><Text style={styles.muted}>{statusLabel[level.status]} · попыток: {level.attempts}</Text></View>)}</Card>
        <Card><Text style={styles.heading}>Компетенции</Text>{summary.competencies.length ? summary.competencies.map((item) => <View key={item.competency_id} style={styles.row}><Text style={styles.rowTitle}>{item.name}</Text><Text style={styles.muted}>Баллы: {item.score ?? 'нет данных'} · свидетельств: {item.confidence}</Text></View>) : <Text style={styles.muted}>Данных пока нет</Text>}</Card>
        <Card><Text style={styles.heading}>Последние утверждённые смены</Text>{summary.session_outcomes.recent_assessments.length ? summary.session_outcomes.recent_assessments.map((item) => <View key={item.session_id} style={styles.row}><Text style={styles.rowTitle}>{item.session_pass ? 'Успешно' : 'Не пройдено'} · безопасность {item.session_safety_score}</Text><Text style={styles.muted}>Лояльность {item.loyalty} · нарушений {item.critical_violations}</Text><Text style={styles.muted}>ID: {item.session_id}</Text></View>) : <Text style={styles.muted}>Смен пока нет</Text>}</Card>
        <Text style={styles.hint}>Не включено в оценки: {summary.provenance.excluded_draft_count} черновых смен.</Text>
      </>}
    </>}

    {section === 'approval' && <Card>
      <Text style={styles.heading}>Утвердить смену</Text>
      <Text style={styles.hint}>Только завершённая диалоговая смена с утверждёнными сценариями. Вагонные уровни подтверждаются автоматически.</Text>
      <Field label="ID смены (UUID)" value={sessionId} onChangeText={setSessionId} />
      <Button title="Утвердить смену" busy={busy} disabled={!uuidPattern.test(sessionId.trim())} onPress={() => run(async () => {
        await api.approveSession(sessionId.trim())
        setNotice('Смена утверждена и войдёт в учебную сводку.')
        setSummary(null)
      })} />
    </Card>}
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
})
