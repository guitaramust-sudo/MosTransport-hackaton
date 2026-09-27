import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { navigate, useAppDispatch } from '../app/store'
import { Button, Card, ErrorText, Page } from '../components/UI'
import { Text } from '../components/Typography'
import { colors } from '../helpers/theme'

const statusLabel = { locked: 'Закрыт', unlocked: 'Доступен', completed: 'Пройден' }

export function LearningMapPage() {
  const dispatch = useAppDispatch()
  const map = useQuery({ queryKey: ['learning-map'], queryFn: api.getLearningMap })
  return <Page title="Маршрут обучения">
    <ErrorText error={map.error instanceof Error ? map.error.message : null} />
    {map.isLoading && <Text>Загружаем уроки…</Text>}
    {map.data?.chapters.map((chapter) => <Card key={chapter.chapter_id}>
      <Text style={{ color: colors.ink, fontSize: 18, fontWeight: '800' }}>{chapter.order}. {chapter.title}</Text>
      {chapter.lessons.map((lesson) => <Text key={lesson.lesson_id} style={{ color: lesson.status === 'locked' ? colors.muted : colors.primary, marginTop: 12 }}>
        {lesson.order}. {lesson.title} · {statusLabel[lesson.status]}
      </Text>)}
    </Card>)}
    <Button title="К игровому вагону" onPress={() => dispatch(navigate('wagon_lobby'))} secondary />
  </Page>
}
