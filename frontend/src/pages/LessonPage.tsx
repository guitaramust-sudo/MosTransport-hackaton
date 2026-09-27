import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { navigate, startLessonPractice, useAppDispatch, useAppSelector } from '../app/store'
import { badgeTitles } from '../assets/badges'
import { lessonBadge } from '../helpers/achievements'
import { Icon } from '../components/Icon'
import { Text } from '../components/Typography'
import { Badge, Button, Card, EmptyState, ErrorText, Pill, ProgressBar, Screen } from '../components/UI'
import { colors, radius, spacing, type } from '../helpers/theme'
import { wagonAnchorLabels, wagonObjectLabels } from '../helpers/wagonMap'
import type { LessonDetail, LessonFinalizeResult, LessonQuestion, WagonAnchor } from '../types'

type Step = 'theory' | 'theory_check' | 'practice' | 'practice_check' | 'result'

const steps: Array<{ id: Step; label: string }> = [
  { id: 'theory', label: 'Теория' },
  { id: 'theory_check', label: 'Тест' },
  { id: 'practice', label: 'Практика' },
  { id: 'practice_check', label: 'Тест' },
  { id: 'result', label: 'Разбор' },
]

/** Lesson state machine from GDD §27.3, resumed from the server's progress row. */
function initialStep(lesson: LessonDetail): Step {
  const p = lesson.progress
  if (p.completed_at) return 'result'
  if (!p.theory_pass) return 'theory'
  if (!p.practice_session_id) return 'practice'
  if (!p.practice_check_pass) return 'practice_check'
  return 'result'
}

function Stepper({ current }: { current: Step }) {
  const index = steps.findIndex((item) => item.id === current)
  return (
    <View style={styles.stepper} accessibilityLabel={`Шаг ${index + 1} из ${steps.length}: ${steps[index].label}`}>
      {steps.map((item, i) => (
        <View key={i} style={styles.stepItem}>
          <View style={[styles.stepBar, i < index && styles.stepBarDone, i === index && styles.stepBarCurrent]} />
          <Text style={[styles.stepLabel, i === index && styles.stepLabelCurrent]}>{item.label}</Text>
        </View>
      ))}
    </View>
  )
}

function Theory({ cards, onDone }: { cards: string[]; onDone: () => void }) {
  const [index, setIndex] = useState(0)
  const last = index >= cards.length - 1
  return (
    <>
      <Card style={styles.theoryCard}>
        <Text style={styles.theoryKicker}>КАДР {index + 1} ИЗ {cards.length}</Text>
        <Text style={styles.theoryText}>{cards[index]}</Text>
        <View style={styles.dots}>
          {cards.map((_, i) => <View key={i} style={[styles.dot, i === index && styles.dotActive]} />)}
        </View>
      </Card>
      <Button title={last ? 'К проверке' : 'Далее'} icon="chevronRight" onPress={() => (last ? onDone() : setIndex(index + 1))} />
      {index > 0 && <Button title="Назад" variant="tertiary" onPress={() => setIndex(index - 1)} />}
    </>
  )
}

function Quiz({ lessonId, questions, onPassed }: { lessonId: string; questions: LessonQuestion[]; onPassed: () => void }) {
  const [index, setIndex] = useState(0)
  const [selected, setSelected] = useState<string | null>(null)
  const [solved, setSolved] = useState<Set<string>>(new Set())
  const answer = useMutation({
    mutationFn: (optionId: string) => api.answerLesson(lessonId, questions[index].question_id, optionId),
    onSuccess: (result) => {
      if (result.correct) setSolved((prev) => new Set(prev).add(questions[index].question_id))
    },
  })
  const question = questions[index]
  const result = answer.data
  const phasePassed = Boolean(result?.phase_pass)

  function next() {
    if (phasePassed) { onPassed(); return }
    // Repeat only the questions that are still unanswered or wrong (GDD §27.3).
    const nextIndex = questions.findIndex((q, i) => i > index && !solved.has(q.question_id))
    const fallback = questions.findIndex((q) => !solved.has(q.question_id))
    setIndex(nextIndex >= 0 ? nextIndex : Math.max(0, fallback))
    setSelected(null)
    answer.reset()
  }

  return (
    <>
      <Text style={styles.quizCount}>Вопрос {index + 1} из {questions.length}</Text>
      <Text style={styles.quizPrompt}>{question.prompt}</Text>
      <View style={styles.options}>
        {(question.options ?? []).map((option) => {
          const isSelected = selected === option.option_id
          const verdict = isSelected && result ? (result.correct ? 'right' : 'wrong') : null
          return (
            <Pressable key={option.option_id} accessibilityRole="radio" accessibilityState={{ selected: isSelected }}
              disabled={answer.isPending || Boolean(result?.correct)}
              onPress={() => { setSelected(option.option_id); answer.reset() }}
              style={[styles.option, isSelected && styles.optionSelected, verdict === 'right' && styles.optionRight, verdict === 'wrong' && styles.optionWrong]}>
              <View style={[styles.radio, isSelected && styles.radioSelected, verdict === 'right' && styles.radioRight, verdict === 'wrong' && styles.radioWrong]}>
                {verdict === 'right' && <Icon name="check" size={14} color={colors.surface} strokeWidth={3} />}
                {verdict === 'wrong' && <Icon name="close" size={14} color={colors.surface} strokeWidth={3} />}
              </View>
              <Text style={styles.optionText}>{option.text}</Text>
            </Pressable>
          )
        })}
      </View>
      {result && (
        <View style={[styles.feedback, result.correct ? styles.feedbackRight : styles.feedbackWrong]} accessibilityRole="alert">
          <Text style={[styles.feedbackTitle, { color: result.correct ? colors.successInk : colors.errorInk }]}>{result.correct ? 'Верно' : 'Не совсем'}</Text>
          <Text style={styles.feedbackText}>{result.feedback}</Text>
        </View>
      )}
      <ErrorText error={answer.error ? answer.error.message || 'Ответ не отправлен' : null} />
      {!result?.correct ? (
        <Button title={result ? 'Ответить ещё раз' : 'Проверить'} disabled={!selected} busy={answer.isPending} onPress={() => selected && answer.mutate(selected)} />
      ) : (
        <Button title={phasePassed ? 'Продолжить' : 'Следующий вопрос'} icon="chevronRight" onPress={next} />
      )}
    </>
  )
}

function PracticeIntro({ lesson, onStart, busy, error }: { lesson: LessonDetail; onStart: () => void; busy: boolean; error: string | null }) {
  const anchors = lesson.required_anchor_ids.map((id) => wagonAnchorLabels[id as WagonAnchor] ?? id)
  const objects = lesson.required_object_ids.map((id) => wagonObjectLabels[id] ?? id)
  return (
    <>
      <Card>
        <View style={styles.practiceIcon}><Icon name="train" size={28} color={colors.action} /></View>
        <Text style={styles.practiceTitle}>Практика в вагоне</Text>
        {lesson.completion_rule === 'visit_inspect' ? <>
          <Text style={styles.practiceText}>Пройдите по вагону. Отметка появится, когда вы подойдёте к точке, а объект засчитается после «Осмотреть».</Text>
          {anchors.length > 0 && <Text style={styles.practiceGoal}>Посетить: {anchors.join(', ')}</Text>}
          {objects.length > 0 && <Text style={styles.practiceGoal}>Осмотреть: {objects.join(', ')}</Text>}
        </> : (
          <Text style={styles.practiceText}>В вагоне один пассажир. Заметьте его, подойдите и выясните просьбу в разговоре — не угадывайте её заранее.</Text>
        )}
        <View style={styles.metaRow}><Icon name="clock" size={16} color={colors.secondary} /><Text style={styles.metaText}>≈ {lesson.estimated_min || 5} мин · ориентировочно</Text></View>
      </Card>
      <ErrorText error={error} />
      <Button title="Начать практику" icon="chevronRight" busy={busy} onPress={onStart} />
    </>
  )
}

function Result({ lesson, result, onRetryPractice, onRestart, onNext }: {
  lesson: LessonDetail
  result: LessonFinalizeResult | null
  onRetryPractice: () => void
  onRestart: () => void
  onNext: () => void
}) {
  const myLearning = useQuery({ queryKey: ['my-learning'], queryFn: api.myLearning })
  const badge = lessonBadge(lesson.lesson_id)
  const completed = Boolean(result?.completed || lesson.progress.completed_at)
  const found = [...(result?.found_anchors ?? []), ...(result?.found_objects ?? [])]
  const missing = [...(result?.missing_anchors ?? []), ...(result?.missing_objects ?? [])]
  const label = (id: string) => wagonAnchorLabels[id as WagonAnchor] ?? wagonObjectLabels[id] ?? id

  return (
    <>
      <Card style={styles.resultCard}>
        {badge && <Badge id={badge} size={112} locked={!completed} />}
        <Text style={styles.resultTitle}>{completed ? 'Урок пройден' : 'Цель пока не выполнена'}</Text>
        {badge && <Text style={styles.resultBadge}>Бейдж «{badgeTitles[badge]}»{completed ? '' : ' откроется после зачёта'}</Text>}
        {result?.award_granted && (
          <View style={styles.awards}>
            <Pill label={`+${result.xp_awarded ?? 0} XP`} tone="progress" icon="star" />
            <Pill label="Новый бейдж" tone="success" icon="trophy" />
          </View>
        )}
        {result && result.completed && !result.award_granted && (
          <Text style={styles.resultNote}>Повторное прохождение: XP, бейдж и призовые очки начисляются только за первый зачёт.</Text>
        )}
      </Card>

      {result && (
        <Card>
          <Text style={styles.debriefTitle}>Разбор</Text>
          <Text style={styles.debriefText}>{result.debrief}</Text>
          {found.map((id) => (
            <View key={`f-${id}`} style={styles.debriefRow}><Icon name="check" size={18} color={colors.successInk} strokeWidth={2.4} /><Text style={styles.debriefItem}>{label(id)} — найдено</Text></View>
          ))}
          {missing.map((id) => (
            <View key={`m-${id}`} style={styles.debriefRow}><Icon name="close" size={18} color={colors.errorInk} strokeWidth={2.4} /><Text style={styles.debriefItem}>{label(id)} — не найдено</Text></View>
          ))}
          {result.scenario_pass !== undefined && (
            <View style={styles.debriefRow}>
              <Icon name={result.scenario_pass ? 'check' : 'close'} size={18} color={result.scenario_pass ? colors.successInk : colors.errorInk} strokeWidth={2.4} />
              <Text style={styles.debriefItem}>{result.scenario_pass ? 'Обращение пассажира закрыто без ошибок' : 'Обращение пассажира закрыто с ошибкой'}</Text>
            </View>
          )}
        </Card>
      )}

      {myLearning.data && (
        <Card>
          <View style={styles.prizeTop}>
            <Text style={styles.debriefTitle}>Призовые очки</Text>
            <Text style={styles.prizeValue}>{myLearning.data.prize_balance}/{myLearning.data.prize_shirt_threshold}</Text>
          </View>
          <ProgressBar value={myLearning.data.prize_shirt_progress * 100} color={colors.gold} label="Прогресс к футболке" />
          <Text style={styles.prizeNote}>До футболки ВСМ. Каждая запись действует 6 суток.</Text>
        </Card>
      )}

      {completed ? <>
        <Button title="Дальше по маршруту" icon="chevronRight" onPress={onNext} />
        <Button title="Пройти урок ещё раз" variant="tertiary" onPress={onRestart} />
      </> : (
        <Button title="Повторить практику" icon="refresh" onPress={onRetryPractice} />
      )}
    </>
  )
}

export function LessonPage() {
  const dispatch = useAppDispatch()
  const queryClient = useQueryClient()
  const lessonId = useAppSelector((state) => state.app.lessonId)
  const lesson = useQuery({ queryKey: ['lesson', lessonId], queryFn: () => api.lesson(lessonId!), enabled: Boolean(lessonId) })
  const [step, setStep] = useState<Step | null>(null)
  const [finalized, setFinalized] = useState<LessonFinalizeResult | null>(null)

  useEffect(() => { if (lesson.data && step === null) setStep(initialStep(lesson.data)) }, [lesson.data, step])

  const refreshLearning = () => {
    void queryClient.invalidateQueries({ queryKey: ['learning-map'] })
    void queryClient.invalidateQueries({ queryKey: ['my-learning'] })
    void queryClient.invalidateQueries({ queryKey: ['profile'] })
    void queryClient.invalidateQueries({ queryKey: ['lesson', lessonId] })
  }

  const finalize = useMutation({
    mutationFn: () => api.finalizeLesson(lessonId!),
    onSuccess: (result) => { setFinalized(result); setStep('result'); refreshLearning() },
  })

  const start = useMutation({
    mutationFn: () => api.startLessonPractice(lessonId!),
    onSuccess: (result) => dispatch(startLessonPractice({ lessonId: lessonId!, sessionId: result.session_id, wsPath: result.ws_path })),
  })

  // Resuming after the practice check: fetch the result the server computed.
  useEffect(() => {
    if (step === 'result' && !finalized && lesson.data && !lesson.data.progress.completed_at && !finalize.isPending && !finalize.isError) finalize.mutate()
  }, [step, finalized, lesson.data])

  const theoryQuestions = useMemo(() => lesson.data?.questions.filter((q) => q.phase === 'theory') ?? [], [lesson.data])
  const practiceQuestions = useMemo(() => lesson.data?.questions.filter((q) => q.phase === 'practice') ?? [], [lesson.data])

  const back = () => dispatch(navigate('home'))

  if (!lessonId) return <Screen title="Урок" onBack={back}><Card><EmptyState icon="route" title="Урок не выбран" text="Откройте урок на карте маршрута." /></Card></Screen>
  if (!lesson.data || !step) {
    return (
      <Screen title="Урок" onBack={back}>
        <ErrorText error={lesson.error ? lesson.error.message || 'Не удалось открыть урок' : null} />
        {!lesson.error && <Text style={styles.loading}>Загружаем урок…</Text>}
      </Screen>
    )
  }

  const data = lesson.data
  return (
    <Screen title={data.title} subtitle={`Урок ${data.lesson_id}`} onBack={back}>
      <Stepper current={step} />
      {step === 'theory' && <Theory cards={data.theory_cards} onDone={() => setStep(data.progress.theory_pass ? 'practice' : 'theory_check')} />}
      {step === 'theory_check' && <Quiz lessonId={data.lesson_id} questions={theoryQuestions} onPassed={() => { refreshLearning(); setStep('practice') }} />}
      {step === 'practice' && <PracticeIntro lesson={data} busy={start.isPending} error={start.error ? start.error.message || 'Не удалось начать практику' : null} onStart={() => start.mutate()} />}
      {step === 'practice_check' && <>
        <Quiz lessonId={data.lesson_id} questions={practiceQuestions} onPassed={() => finalize.mutate()} />
        <Button title="Пройти практику заново" variant="tertiary" onPress={() => setStep('practice')} />
      </>}
      {step === 'result' && (finalize.isPending
        ? <Text style={styles.loading}>Подводим итог…</Text>
        : <>
          <ErrorText error={finalize.error ? finalize.error.message || 'Не удалось подвести итог' : null} />
          <Result lesson={data} result={finalized}
            onRetryPractice={() => { setFinalized(null); setStep('practice') }}
            onRestart={() => { setFinalized(null); setStep('theory') }}
            onNext={() => dispatch(navigate('home'))} />
        </>)}
    </Screen>
  )
}

const styles = StyleSheet.create({
  loading: { ...type.secondary, color: colors.secondary, textAlign: 'center', marginTop: spacing.lg },
  stepper: { flexDirection: 'row', gap: 6, marginBottom: spacing.md },
  stepItem: { flex: 1, gap: 6 },
  stepBar: { height: 6, borderRadius: 3, backgroundColor: colors.border },
  stepBarDone: { backgroundColor: colors.action },
  stepBarCurrent: { backgroundColor: colors.primary },
  stepLabel: { ...type.label, color: colors.muted, textAlign: 'center' },
  stepLabelCurrent: { color: colors.primary, fontWeight: '600' },
  theoryCard: { minHeight: 240, justifyContent: 'space-between' },
  theoryKicker: { ...type.label, color: colors.action, letterSpacing: 0.6 },
  theoryText: { fontSize: 19, lineHeight: 28, fontWeight: '500', color: colors.ink, marginTop: spacing.sm },
  dots: { flexDirection: 'row', gap: 6, marginTop: spacing.lg, alignSelf: 'center' },
  dot: { width: 8, height: 8, borderRadius: 4, backgroundColor: colors.border },
  dotActive: { width: 22, backgroundColor: colors.action },
  quizCount: { ...type.label, color: colors.muted },
  quizPrompt: { ...type.h3, color: colors.ink, marginTop: 4, marginBottom: spacing.md },
  options: { gap: spacing.xs },
  option: { minHeight: 56, flexDirection: 'row', alignItems: 'center', gap: spacing.sm, paddingHorizontal: spacing.md, paddingVertical: 12, borderRadius: radius.md, borderWidth: 1.5, borderColor: colors.border, backgroundColor: colors.surface },
  optionSelected: { borderColor: colors.action, backgroundColor: colors.blueSoft },
  optionRight: { borderColor: colors.success, backgroundColor: colors.successSoft },
  optionWrong: { borderColor: colors.error, backgroundColor: colors.errorSoft },
  radio: { width: 22, height: 22, borderRadius: 11, borderWidth: 2, borderColor: colors.border, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.surface },
  radioSelected: { borderColor: colors.action, borderWidth: 6 },
  radioRight: { borderWidth: 0, backgroundColor: colors.success },
  radioWrong: { borderWidth: 0, backgroundColor: colors.error },
  optionText: { ...type.body, color: colors.ink, flex: 1 },
  feedback: { marginTop: spacing.md, padding: spacing.md, borderRadius: radius.md },
  feedbackRight: { backgroundColor: colors.successSoft },
  feedbackWrong: { backgroundColor: colors.errorSoft },
  feedbackTitle: { fontSize: 16, lineHeight: 22, fontWeight: '600' },
  feedbackText: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  practiceIcon: { width: 52, height: 52, borderRadius: 16, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  practiceTitle: { ...type.h3, color: colors.ink, marginTop: spacing.sm },
  practiceText: { ...type.body, color: colors.secondary, marginTop: 4 },
  practiceGoal: { ...type.secondary, color: colors.ink, marginTop: spacing.xs },
  metaRow: { flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start', gap: 6, marginTop: spacing.md, paddingHorizontal: 10, paddingVertical: 6, borderRadius: radius.sm, backgroundColor: colors.soft },
  metaText: { ...type.secondary, color: colors.secondary },
  resultCard: { alignItems: 'center', paddingVertical: spacing.lg },
  resultTitle: { ...type.h2, color: colors.ink, marginTop: spacing.sm, textAlign: 'center' },
  resultBadge: { ...type.secondary, color: colors.secondary, marginTop: 4, textAlign: 'center' },
  resultNote: { ...type.secondary, color: colors.secondary, marginTop: spacing.sm, textAlign: 'center' },
  awards: { flexDirection: 'row', gap: spacing.xs, marginTop: spacing.sm },
  debriefTitle: { ...type.cardTitle, color: colors.ink },
  debriefText: { ...type.secondary, color: colors.secondary, marginTop: 4, marginBottom: spacing.xs },
  debriefRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.xs, paddingVertical: 4 },
  debriefItem: { ...type.body, color: colors.ink },
  prizeTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: spacing.xs },
  prizeValue: { ...type.cardTitle, color: colors.warningInk },
  prizeNote: { ...type.secondary, color: colors.secondary, marginTop: spacing.xs },
})
