import { Text, StyleSheet } from 'react-native'
import { navigate, selectSituation, useAppDispatch, useAppSelector } from '../app/store'
import { Button, Card, Page } from '../components/UI'
import { colors } from '../helpers/theme'

export function ScenariosPage() {
  const dispatch = useAppDispatch()
  const shift = useAppSelector((state) => state.app.shift)
  return <Page title="Ситуации смены">
    {!shift ? <Card><Text style={styles.body}>Начните смену на главной странице. Сервер выберет ситуации и пассажиров.</Text>
      <Button title="На главную" onPress={() => dispatch(navigate('home'))} /></Card> :
      shift.situations.map((situation) => <Card key={situation.id}>
        <Text style={styles.title}>{situation.scenario}</Text>
        <Text style={styles.body}>{situation.opening ?? situation.code}</Text>
        <Text style={styles.status}>{situation.status === 'closed' ? `Закрыта · ${situation.outcome ?? ''}` : 'Активна'}</Text>
        <Button title="Открыть" onPress={() => { dispatch(selectSituation(situation.id)); dispatch(navigate('simulation')) }} secondary />
      </Card>)}
  </Page>
}
const styles = StyleSheet.create({ title: { color: colors.ink, fontSize: 17, fontWeight: '800' }, body: { color: colors.muted, lineHeight: 20, marginTop: 8 }, status: { color: colors.primary, marginTop: 10, fontWeight: '700' } })
