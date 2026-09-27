import { StyleSheet, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'
import { useAppSelector } from '../app/store'
import { colors } from '../helpers/theme'
import { AuthPage } from '../pages/AuthPage'
import { AdminPage } from '../pages/AdminPage'
import { DebriefPage } from '../pages/DebriefPage'
import { HomePage } from '../pages/HomePage'
import { LearningMapPage } from '../pages/LearningMapPage'
import { ProfilePage } from '../pages/ProfilePage'
import { ScenariosPage } from '../pages/ScenariosPage'
import { SimulationPage } from '../pages/SimulationPage'
import { LiveSimulationPage } from '../pages/LiveSimulationPage'
import { WagonLobbyPage } from '../pages/WagonLobbyPage'
import { WagonPage } from '../pages/WagonPage'
import { BottomNav } from './BottomNav'

export function RootNavigator() {
  const screen = useAppSelector((state) => state.app.screen)
  const isFocusedMode = screen === 'auth' || screen === 'simulation' || screen === 'live_simulation' || screen === 'wagon_lobby' || screen === 'wagon' || screen === 'debrief'

  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.app}>
        {screen === 'auth' && <AuthPage />}
        {screen === 'admin' && <AdminPage />}
        {screen === 'home' && <HomePage />}
        {screen === 'learning_map' && <LearningMapPage />}
        {screen === 'scenarios' && <ScenariosPage />}
        {screen === 'simulation' && <SimulationPage />}
        {screen === 'live_simulation' && <LiveSimulationPage />}
        {screen === 'wagon_lobby' && <WagonLobbyPage />}
        {screen === 'wagon' && <WagonPage />}
        {screen === 'debrief' && <DebriefPage />}
        {screen === 'profile' && <ProfilePage />}
        {!isFocusedMode && <BottomNav />}
      </View>
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: colors.surface },
  app: { flex: 1, backgroundColor: colors.soft },
})
