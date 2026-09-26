import { Platform, SafeAreaView, StatusBar, StyleSheet, View } from 'react-native'
import { useAppSelector } from '../app/store'
import { colors } from '../helpers/theme'
import { DebriefPage } from '../pages/DebriefPage'
import { HomePage } from '../pages/HomePage'
import { ProfilePage } from '../pages/ProfilePage'
import { ScenariosPage } from '../pages/ScenariosPage'
import { SimulationPage } from '../pages/SimulationPage'
import { BottomNav } from './BottomNav'

export function RootNavigator() {
  const screen = useAppSelector((state) => state.app.screen)
  const isFocusedMode = screen === 'simulation' || screen === 'debrief'

  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.app}>
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
  safeArea: { flex: 1, backgroundColor: colors.surface, paddingTop: Platform.OS === 'android' ? StatusBar.currentHeight : 0 },
  app: { flex: 1, backgroundColor: '#F4F7F8' },
})

