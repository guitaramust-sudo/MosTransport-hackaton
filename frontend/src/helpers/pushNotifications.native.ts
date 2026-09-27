import Constants from 'expo-constants'
import * as Notifications from 'expo-notifications'
import { Platform } from 'react-native'
import { api } from '../api/client'

const projectId = process.env.EXPO_PUBLIC_EAS_PROJECT_ID
  ?? Constants.expoConfig?.extra?.eas?.projectId
  ?? Constants.easConfig?.projectId

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
})

async function register(): Promise<string> {
  if (Constants.appOwnership === 'expo') {
    throw new Error('Для push-уведомлений нужен development build или APK. Expo Go их не принимает.')
  }
  if (!projectId) {
    throw new Error('Для push-уведомлений укажите EXPO_PUBLIC_EAS_PROJECT_ID вашего Expo-проекта.')
  }
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
  const existing = await Notifications.getPermissionsAsync()
  const permission = existing.granted ? existing : await Notifications.requestPermissionsAsync()
  if (!permission.granted) throw new Error('Разрешение на уведомления не предоставлено.')
  return register()
}

export async function syncExistingPushRegistration(): Promise<void> {
  if (Constants.appOwnership === 'expo' || !projectId) return
  const permission = await Notifications.getPermissionsAsync()
  if (permission.granted) await register()
}
