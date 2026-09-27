import Constants from 'expo-constants'
import { Platform } from 'react-native'
import { api } from '../api/client'

const projectId = process.env.EXPO_PUBLIC_EAS_PROJECT_ID
  ?? Constants.expoConfig?.extra?.eas?.projectId
  ?? Constants.easConfig?.projectId

export const isExpoGo = Constants.appOwnership === 'expo' || Constants.executionEnvironment === 'storeClient'

// expo-notifications throws as soon as its module is evaluated in Android
// Expo Go. Load it only after confirming this is a development/production build.
async function loadNotifications() {
  const notifications = await import('expo-notifications')
  notifications.setNotificationHandler({
    handleNotification: async () => ({
      shouldShowBanner: true,
      shouldShowList: true,
      shouldPlaySound: true,
      shouldSetBadge: false,
    }),
  })
  return notifications
}

async function register(): Promise<string> {
  if (isExpoGo) {
    throw new Error('Для push-уведомлений нужен development build или APK. Expo Go их не принимает.')
  }
  if (!projectId) {
    throw new Error('Для push-уведомлений укажите EXPO_PUBLIC_EAS_PROJECT_ID вашего Expo-проекта.')
  }
  const Notifications = await loadNotifications()
  if (Platform.OS === 'android') {
    await Notifications.setNotificationChannelAsync('training', {
      name: 'Обучение ВСМ',
      importance: Notifications.AndroidImportance.HIGH,
    })
  }
  const token = (await Notifications.getExpoPushTokenAsync({ projectId })).data
  await api.registerPushSubscription(Platform.OS as 'android' | 'ios', token)
  return token
}

export async function registerForPushNotifications(): Promise<string> {
  if (isExpoGo) return register()
  const Notifications = await loadNotifications()
  const existing = await Notifications.getPermissionsAsync()
  const permission = existing.granted ? existing : await Notifications.requestPermissionsAsync()
  if (!permission.granted) throw new Error('Разрешение на уведомления не предоставлено.')
  return register()
}

export async function syncExistingPushRegistration(): Promise<void> {
  if (isExpoGo || !projectId) return
  const Notifications = await loadNotifications()
  const permission = await Notifications.getPermissionsAsync()
  if (permission.granted) await register()
}
