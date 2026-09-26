import { useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StatusBar } from 'expo-status-bar'
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native'
import { Provider } from 'react-redux'
import { SafeAreaProvider } from 'react-native-safe-area-context'
import { store } from './src/app/store'
import { RootNavigator } from './src/components/RootNavigator'
import { preloadGameAssets } from './src/helpers/gameAssets'
import { colors } from './src/helpers/theme'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
})

export default function App() {
  const [assetsReady, setAssetsReady] = useState(false)

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

  if (!assetsReady) {
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
          <RootNavigator />
        </QueryClientProvider>
      </Provider>
    </SafeAreaProvider>
  )
}

const styles = StyleSheet.create({
  loader: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 16, backgroundColor: '#F4F7F8' },
  logo: { width: 72, height: 72, borderRadius: 24, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.primary },
  logoText: { color: colors.surface, fontSize: 22, fontWeight: '900' },
  title: { color: colors.ink, fontSize: 16, fontWeight: '800' },
})
