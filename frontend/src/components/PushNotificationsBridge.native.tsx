import { useEffect, useRef } from 'react'
import * as Notifications from 'expo-notifications'
import { navigate, useAppDispatch, useAppSelector } from '../app/store'
import { syncExistingPushRegistration } from '../helpers/pushNotifications'
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
    function handle(response: Notifications.NotificationResponse) {
      const id = response.notification.request.identifier
      if (lastResponseId.current === id) return
      lastResponseId.current = id
      void Notifications.clearLastNotificationResponseAsync().catch(() => {})
      const screen = destination(response.notification.request.content.data ?? {})
      if (!screen) return
      if (signedIn) dispatch(navigate(screen))
      else pending.current = screen
    }
    const listener = Notifications.addNotificationResponseReceivedListener(handle)
    Notifications.getLastNotificationResponseAsync().then((response) => { if (response) handle(response) }).catch(() => {})
    return () => listener.remove()
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
