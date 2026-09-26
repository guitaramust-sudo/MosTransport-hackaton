import { StyleSheet } from 'react-native'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { MetricBar } from '../components/MetricBar'
import { Button, Card, Page } from '../components/UI'
import { Text } from '../components/Typography'
import { colors } from '../helpers/theme'

const outcomeLabels: Record<string, string> = { success: 'Успех', partial: 'Частично', fail: 'Ошибка', timeout: 'Время вышло' }

export function DebriefPage() {
  const dispatch = useAppDispatch()
  const breakdown = useAppSelector((state) => state.app.breakdown)
  const shift = useAppSelector((state) => state.app.shift)
  return <Page title="Разбор смены">
    {!breakdown ? <Text>Разбор пока недоступен.</Text> : <>
      <Card><Text style={styles.score}>{breakdown.total_xp > 0 ? '+' : ''}{breakdown.total_xp} XP</Text><Text style={styles.body}>Результат завершённой смены.</Text></Card>
      {breakdown.situations.map((item) => <Card key={item.situation_id}>
        <Text style={styles.title}>{shift?.situations.find((situation) => situation.id === item.situation_id)?.scenario ?? item.code.replaceAll('_', ' ')}</Text>
        <Text style={styles.outcome}>{outcomeLabels[item.outcome] ?? item.outcome} · {item.xp} XP</Text>
        <MetricBar label="Безопасность" value={item.safety} tone="safety" />
        <MetricBar label="Лояльность" value={item.loyalty} tone="loyalty" />
        {item.remarks?.map((remark, index) => <Text key={`${remark.code}-${index}`} style={styles.body}>• {remark.message}</Text>)}
      </Card>)}
    </>}
    <Button title="На главную" onPress={() => dispatch(navigate('home'))} />
  </Page>
}
const styles = StyleSheet.create({
  score: { color: colors.primary, fontSize: 34, fontWeight: '900' },
  title: { color: colors.ink, fontSize: 17, fontWeight: '800' },
  outcome: { color: colors.primary, marginVertical: 10, fontWeight: '800' },
  body: { color: colors.muted, marginTop: 8, lineHeight: 20 },
})
