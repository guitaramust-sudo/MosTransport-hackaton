import { useEffect, useRef } from 'react'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { isExpoGo, syncExistingPushRegistration } from '../helpers/pushNotifications'
import type { NotificationResponse } from 'expo-notifications'
import type { AppScreen } from '../types'

function destination(data: Record<string, unknown>): AppScreen | null {
  // The curriculum map lives on the home tab.
  if (data.screen === 'learning_map') return 'home'
  if (data.screen === 'prize_balance') return 'profile'
  return null
}

export function PushNotificationsBridge() {
  const dispatch = useAppDispatch()
  const signedIn = useAppSelector((state) => Boolean(state.app.auth))
  const pending = useRef<AppScreen | null>(null)
  const lastResponseId = useRef<string | null>(null)

  useEffect(() => {
    if (isExpoGo) return
    let disposed = false
    let listener: { remove: () => void } | undefined
    function handle(response: NotificationResponse) {
      if (disposed) return
      const id = response.notification.request.identifier
      if (lastResponseId.current === id) return
      lastResponseId.current = id
      const screen = destination(response.notification.request.content.data ?? {})
      if (!screen) return
      if (signedIn) dispatch(navigate(screen))
      else pending.current = screen
    }
    void import('expo-notifications').then((Notifications) => {
      if (disposed) return
      listener = Notifications.addNotificationResponseReceivedListener((response) => {
        handle(response)
        void Notifications.clearLastNotificationResponseAsync().catch(() => {})
      })
      void Notifications.getLastNotificationResponseAsync().then((response) => {
        if (response) {
          handle(response)
          void Notifications.clearLastNotificationResponseAsync().catch(() => {})
        }
      }).catch(() => {})
    }).catch((error) => console.warn('Push notifications unavailable', error))
    return () => { disposed = true; listener?.remove() }
  }, [dispatch, signedIn])

  useEffect(() => {
    if (!signedIn) return
    if (pending.current) {
      dispatch(navigate(pending.current))
      pending.current = null
    }
    void syncExistingPushRegistration().catch((error) => console.warn('Push registration failed', error))
  }, [dispatch, signedIn])

  return null
}
