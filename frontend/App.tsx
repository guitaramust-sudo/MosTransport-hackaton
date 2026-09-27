import { useEffect, useState } from 'react'
import {
  Onest_400Regular,
  Onest_500Medium,
  Onest_600SemiBold,
  Onest_700Bold,
  Onest_800ExtraBold,
  useFonts,
} from '@expo-google-fonts/onest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StatusBar } from 'expo-status-bar'
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native'
import { Provider } from 'react-redux'
import { SafeAreaProvider } from 'react-native-safe-area-context'
import { store } from './src/app/store'
import { RootNavigator } from './src/components/RootNavigator'
import { PushNotificationsBridge } from './src/components/PushNotificationsBridge'
import { preloadGameAssets } from './src/helpers/gameAssets'
import { colors } from './src/helpers/theme'
import { MusicSettingsProvider } from './src/helpers/musicSettings'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
})

export default function App() {
  const [assetsReady, setAssetsReady] = useState(false)
  const [fontsReady, fontError] = useFonts({
    Onest_400Regular,
    Onest_500Medium,
    Onest_600SemiBold,
    Onest_700Bold,
    Onest_800ExtraBold,
  })

  useEffect(() => {
    let mounted = true

    preloadGameAssets()
      .catch((error) => console.warn('Game assets preload failed', error))
      .finally(() => {
        if (mounted) setAssetsReady(true)
      })

    return () => {
      mounted = false
    }
  }, [])

  useEffect(() => {
    if (fontError) console.warn('Interface font preload failed', fontError)
  }, [fontError])

  if (!assetsReady || (!fontsReady && !fontError)) {
    return (
      <View style={styles.loader}>
        <StatusBar style="dark" />
        <View style={styles.logo}><Text style={styles.logoText}>ВСМ</Text></View>
        <Text style={styles.title}>Подготавливаем поезд</Text>
        <ActivityIndicator color={colors.primary} size="large" />
      </View>
    )
  }

  return (
    <SafeAreaProvider>
      <Provider store={store}>
        <QueryClientProvider client={queryClient}>
          <StatusBar style="dark" />
          <PushNotificationsBridge />
          <MusicSettingsProvider><RootNavigator /></MusicSettingsProvider>
        </QueryClientProvider>
      </Provider>
    </SafeAreaProvider>
  )
}

const styles = StyleSheet.create({
  loader: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 16, backgroundColor: colors.background },
  logo: { width: 72, height: 72, borderRadius: 24, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.primary },
  logoText: { color: colors.surface, fontSize: 22, fontWeight: '800' },
  title: { color: colors.ink, fontSize: 16, fontWeight: '600' },
})
