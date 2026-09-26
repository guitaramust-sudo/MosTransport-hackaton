import { configureStore, createSlice, type PayloadAction } from '@reduxjs/toolkit'
import { useDispatch, useSelector } from 'react-redux'
import { setTokens } from '../api/client'
import type { AppScreen, AuthResult, Breakdown, Player, SessionResponse } from '../types'

interface AppState {
  screen: AppScreen
  auth: AuthResult | null
  shift: SessionResponse | null
  situationId: string | null
  breakdown: Breakdown | null
}
const initialState: AppState = { screen: 'auth', auth: null, shift: null, situationId: null, breakdown: null }
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
  },
})
export const { navigate, signedIn, signedOut, setShift, selectSituation, refreshShift, setBreakdown, setPlayer } = slice.actions
export const store = configureStore({ reducer: { app: slice.reducer } })
export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
export const useAppDispatch = useDispatch.withTypes<AppDispatch>()
export const useAppSelector = useSelector.withTypes<RootState>()
