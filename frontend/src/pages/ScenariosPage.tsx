import { StyleSheet } from 'react-native'
import { navigate, selectSituation, useAppDispatch, useAppSelector } from '../app/store'
import { Button, Card, Page } from '../components/UI'
import { Text } from '../components/Typography'
import { colors } from '../helpers/theme'
import { translateBackendField } from '../helpers/backendTranslations'

const outcomeLabels: Record<string, string> = { success: 'успешно', partial: 'частично', fail: 'с ошибкой', timeout: 'время вышло' }

export function ScenariosPage() {
  const dispatch = useAppDispatch()
  const shift = useAppSelector((state) => state.app.shift)
  return <Page title="Ситуации смены" onBack={() => dispatch(navigate(shift ? 'simulation' : 'practice'))}>
    {!shift ? <Card><Text style={styles.body}>Начните смену в разделе «Практика». Сервер выберет ситуации и пассажиров.</Text>
      <Button title="К практике" onPress={() => dispatch(navigate('practice'))} /></Card> :
      shift.situations.map((situation) => <Card key={situation.id}>
        <Text style={styles.title}>{situation.scenario}</Text>
        <Text style={styles.body}>{situation.opening ?? translateBackendField(situation.code, 'Обращение пассажира')}</Text>
        <Text style={styles.status}>{situation.status === 'closed' ? `Закрыта${situation.outcome ? ` · ${outcomeLabels[situation.outcome] ?? situation.outcome}` : ''}` : 'Активна'}</Text>
        <Button title="Открыть" onPress={() => { dispatch(selectSituation(situation.id)); dispatch(navigate('simulation')) }} secondary />
      </Card>)}
  </Page>
}
const styles = StyleSheet.create({ title: { color: colors.ink, fontSize: 18, lineHeight: 26, fontWeight: '600' }, body: { color: colors.muted, lineHeight: 20, marginTop: 8 }, status: { color: colors.action, marginTop: 10, fontWeight: '600' } })
