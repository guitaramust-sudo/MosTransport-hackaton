import { configureStore, createSlice, type PayloadAction } from '@reduxjs/toolkit'
import { useDispatch, useSelector } from 'react-redux'
import { clampMetric } from '../helpers/theme'
import { scenarios } from '../data/scenarios'
import type { ActionRecord, AppScreen, ScenarioChoice } from '../types'

interface AppState {
  screen: AppScreen
  previousScreen: AppScreen
  scenarioId: string
  eventIndex: number
  safety: number
  loyalty: number
  score: number
  actions: ActionRecord[]
}

const initialState: AppState = {
  screen: 'home',
  previousScreen: 'home',
  scenarioId: scenarios[0].id,
  eventIndex: 0,
  safety: 92,
  loyalty: 84,
  score: 1240,
  actions: [],
}

const appSlice = createSlice({
  name: 'app',
  initialState,
  reducers: {
    navigate(state, action: PayloadAction<AppScreen>) {
      state.previousScreen = state.screen
      state.screen = action.payload
    },
    startScenario(state, action: PayloadAction<string>) {
      state.scenarioId = action.payload
      state.eventIndex = 0
      state.safety = 92
      state.loyalty = 84
      state.actions = []
      state.previousScreen = state.screen
      state.screen = 'simulation'
    },
    makeChoice(state, action: PayloadAction<ScenarioChoice>) {
      const scenario = scenarios.find((item) => item.id === state.scenarioId)
      const event = scenario?.events[state.eventIndex]
      if (!event) return

      state.safety = clampMetric(state.safety + action.payload.safety)
      state.loyalty = clampMetric(state.loyalty + action.payload.loyalty)
      state.score += Math.max(0, action.payload.safety + action.payload.loyalty) * 5
      state.actions.push({
        eventTitle: event.title,
        choiceTitle: action.payload.title,
        feedback: action.payload.feedback,
        safety: action.payload.safety,
        loyalty: action.payload.loyalty,
        competency: action.payload.competency,
      })

      if (state.eventIndex + 1 < (scenario?.events.length ?? 0)) {
        state.eventIndex += 1
      } else {
        state.previousScreen = state.screen
        state.screen = 'debrief'
      }
    },
  },
})

export const { navigate, startScenario, makeChoice } = appSlice.actions
export const store = configureStore({ reducer: { app: appSlice.reducer } })
export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
export const useAppDispatch = useDispatch.withTypes<AppDispatch>()
export const useAppSelector = useSelector.withTypes<RootState>()
