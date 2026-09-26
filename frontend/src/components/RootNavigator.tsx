import { StyleSheet, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'
import { useAppSelector } from '../app/store'
import { colors } from '../helpers/theme'
import { AuthPage } from '../pages/AuthPage'
import { DebriefPage } from '../pages/DebriefPage'
import { HomePage } from '../pages/HomePage'
import { ProfilePage } from '../pages/ProfilePage'
import { ScenariosPage } from '../pages/ScenariosPage'
import { SimulationPage } from '../pages/SimulationPage'
import { BottomNav } from './BottomNav'

export function RootNavigator() {
  const screen = useAppSelector((state) => state.app.screen)
  const isFocusedMode = screen === 'auth' || screen === 'simulation' || screen === 'debrief'

  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.app}>
        {screen === 'auth' && <AuthPage />}
        {screen === 'home' && <HomePage />}
        {screen === 'scenarios' && <ScenariosPage />}
        {screen === 'simulation' && <SimulationPage />}
        {screen === 'debrief' && <DebriefPage />}
        {screen === 'profile' && <ProfilePage />}
        {!isFocusedMode && <BottomNav />}
      </View>
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: colors.surface },
  app: { flex: 1, backgroundColor: '#F4F7F8' },
})

