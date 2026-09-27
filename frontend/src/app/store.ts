import { configureStore, createSlice, type PayloadAction } from '@reduxjs/toolkit'
import { useDispatch, useSelector } from 'react-redux'
import { setTokens } from '../api/client'
import type { AppScreen, AuthResult, Breakdown, LiveSimulation, Player, SessionResponse, WagonSnapshot } from '../types'

export type WagonConnection = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error'

interface AppState {
  screen: AppScreen
  auth: AuthResult | null
  shift: SessionResponse | null
  situationId: string | null
  breakdown: Breakdown | null
  liveSimulation: LiveSimulation | null
  wagonSessionId: string | null
  wagonWsPath: string | null
  wagonSnapshot: WagonSnapshot | null
  wagonSelectedSituationId: string | null
  wagonConnection: WagonConnection
}
const initialState: AppState = {
  screen: 'auth', auth: null, shift: null, situationId: null, breakdown: null, liveSimulation: null,
  wagonSessionId: null, wagonWsPath: null, wagonSnapshot: null, wagonSelectedSituationId: null, wagonConnection: 'idle',
}
const slice = createSlice({
  name: 'app', initialState,
  reducers: {
    navigate(state, action: PayloadAction<AppScreen>) { state.screen = action.payload },
    signedIn(state, action: PayloadAction<AuthResult>) { state.auth = action.payload; state.screen = 'home' },
    setPlayer(state, action: PayloadAction<Player>) { if (state.auth) state.auth.player = action.payload },
    signedOut() { setTokens(null); return initialState },
    setShift(state, action: PayloadAction<SessionResponse>) {
      state.shift = action.payload
      state.situationId = action.payload.situations.find((item) => item.status === 'active')?.id ?? action.payload.situations[0]?.id ?? null
      state.breakdown = null
      state.screen = 'simulation'
    },
    selectSituation(state, action: PayloadAction<string>) { state.situationId = action.payload },
    refreshShift(state, action: PayloadAction<SessionResponse>) { state.shift = action.payload },
    setBreakdown(state, action: PayloadAction<Breakdown>) {
      state.breakdown = action.payload
      if (state.shift) state.shift.session.status = 'finished'
      state.screen = 'debrief'
    },
    setLiveSimulation(state, action: PayloadAction<LiveSimulation>) {
      state.liveSimulation = action.payload
      state.screen = 'live_simulation'
    },
    updateLiveSimulation(state, action: PayloadAction<LiveSimulation>) { state.liveSimulation = action.payload },
    setWagonSession(state, action: PayloadAction<{ sessionId: string; wsPath?: string }>) {
      state.wagonSessionId = action.payload.sessionId
      state.wagonWsPath = action.payload.wsPath ?? `/api/wagon/${action.payload.sessionId}/ws`
      state.wagonSnapshot = null
      state.wagonSelectedSituationId = null
      state.wagonConnection = 'connecting'
      state.breakdown = null
      state.screen = 'wagon'
    },
    updateWagonSnapshot(state, action: PayloadAction<WagonSnapshot>) { state.wagonSnapshot = action.payload },
    setWagonSelectedSituation(state, action: PayloadAction<string | null>) { state.wagonSelectedSituationId = action.payload },
    setWagonConnection(state, action: PayloadAction<WagonConnection>) { state.wagonConnection = action.payload },
    clearWagon(state) {
      state.wagonSessionId = null
      state.wagonWsPath = null
      state.wagonSnapshot = null
      state.wagonSelectedSituationId = null
      state.wagonConnection = 'idle'
    },
    setWagonBreakdown(state, action: PayloadAction<Breakdown>) {
      state.breakdown = action.payload
      state.wagonSessionId = null
      state.wagonWsPath = null
      state.wagonSnapshot = null
      state.wagonSelectedSituationId = null
      state.wagonConnection = 'idle'
      state.screen = 'debrief'
    },
  },
})
export const {
  navigate, signedIn, signedOut, setShift, selectSituation, refreshShift, setBreakdown, setPlayer,
  setLiveSimulation, updateLiveSimulation, setWagonSession, updateWagonSnapshot,
  setWagonSelectedSituation, setWagonConnection, clearWagon, setWagonBreakdown,
} = slice.actions
export const store = configureStore({ reducer: { app: slice.reducer } })
export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
export const useAppDispatch = useDispatch.withTypes<AppDispatch>()
export const useAppSelector = useSelector.withTypes<RootState>()
