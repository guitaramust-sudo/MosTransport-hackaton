import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { StyleSheet, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'
import { useAppSelector } from '../app/store'
import { colors } from '../helpers/theme'
import { AuthPage } from '../pages/AuthPage'
import { AdminPage } from '../pages/AdminPage'
import { DebriefPage } from '../pages/DebriefPage'
import { HomePage } from '../pages/HomePage'
import { LessonPage } from '../pages/LessonPage'
import { PracticePage } from '../pages/PracticePage'
import { ProfilePage } from '../pages/ProfilePage'
import { ProgressPage } from '../pages/ProgressPage'
import { ScenariosPage } from '../pages/ScenariosPage'
import { SimulationPage } from '../pages/SimulationPage'
import { LiveSimulationPage } from '../pages/LiveSimulationPage'
import { WagonPage } from '../pages/WagonPage'
import { BottomNav } from './BottomNav'

// Bottom navigation is hidden during a run, briefing and debrief (§9).
const tabScreens = new Set(['home', 'practice', 'progress', 'profile', 'admin'])

export function RootNavigator() {
  const screen = useAppSelector((state) => state.app.screen)
  const signedIn = useAppSelector((state) => !!state.app.auth)
  const queryClient = useQueryClient()
  // Cached profile, levels and rating belong to the signed-in user only.
  useEffect(() => { if (!signedIn) queryClient.clear() }, [signedIn, queryClient])
  const showNav = tabScreens.has(screen)

  return (
    <SafeAreaView style={styles.safeArea} edges={showNav ? ['top', 'left', 'right'] : ['top', 'left', 'right', 'bottom']}>
      <View style={styles.app}>
        {screen === 'auth' && <AuthPage />}
        {screen === 'admin' && <AdminPage />}
        {screen === 'home' && <HomePage />}
        {screen === 'practice' && <PracticePage />}
        {screen === 'lesson' && <LessonPage />}
        {screen === 'progress' && <ProgressPage />}
        {screen === 'scenarios' && <ScenariosPage />}
        {screen === 'simulation' && <SimulationPage />}
        {screen === 'live_simulation' && <LiveSimulationPage />}
        {screen === 'wagon' && <WagonPage />}
        {screen === 'debrief' && <DebriefPage />}
        {screen === 'profile' && <ProfilePage />}
      </View>
      {showNav && <BottomNav />}
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  safeArea: { flex: 1, backgroundColor: colors.background },
  app: { flex: 1, backgroundColor: colors.background },
})
