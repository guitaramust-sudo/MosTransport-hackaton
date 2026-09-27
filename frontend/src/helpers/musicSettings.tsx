import AsyncStorage from '@react-native-async-storage/async-storage'
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

const STORAGE_KEY = 'vsm.wagonMusicEnabled'

interface MusicSettings {
  enabled: boolean
  ready: boolean
  setEnabled: (enabled: boolean) => void
}

const MusicSettingsContext = createContext<MusicSettings | null>(null)

export function MusicSettingsProvider({ children }: { children: ReactNode }) {
  const [enabled, setEnabledState] = useState(true)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let active = true
    AsyncStorage.getItem(STORAGE_KEY)
      .then((stored) => { if (active && stored !== null) setEnabledState(stored !== 'false') })
      .catch((error) => console.warn('Could not load music preference', error))
      .finally(() => { if (active) setReady(true) })
    return () => { active = false }
  }, [])

  const setEnabled = useCallback((value: boolean) => {
    setEnabledState(value)
    void AsyncStorage.setItem(STORAGE_KEY, String(value))
      .catch((error) => console.warn('Could not save music preference', error))
  }, [])

  const value = useMemo(() => ({ enabled, ready, setEnabled }), [enabled, ready, setEnabled])
  return <MusicSettingsContext.Provider value={value}>{children}</MusicSettingsContext.Provider>
}

export function useMusicSettings() {
  const settings = useContext(MusicSettingsContext)
  if (!settings) throw new Error('Music settings provider is missing')
  return settings
}
