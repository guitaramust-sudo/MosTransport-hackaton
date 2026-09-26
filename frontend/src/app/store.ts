import {
  configureStore,
  createSlice,
  type PayloadAction,
} from "@reduxjs/toolkit";
import { useDispatch, useSelector } from "react-redux";
import { setTokens } from "../api/client";
import { scenarios } from "../data/scenarios";
import { clampMetric } from "../helpers/theme";
import type {
  ActionRecord,
  AppScreen,
  AuthResult,
  Breakdown,
  Player,
  ScenarioChoice,
  SessionResponse,
} from "../types";

interface AppState {
  screen: AppScreen;
  auth: AuthResult | null;
  shift: SessionResponse | null;
  situationId: string | null;
  breakdown: Breakdown | null;
  previousScreen: AppScreen;
  scenarioId: string;
  eventIndex: number;
  safety: number;
  loyalty: number;
  score: number;
  actions: ActionRecord[];
  resolvedEventIds: string[];
}
const initialState: AppState = {
  screen: "home",
  auth: null,
  shift: null,
  situationId: null,
  breakdown: null,
  previousScreen: "home",
  scenarioId: scenarios[0].id,
  eventIndex: 0,
  safety: 92,
  loyalty: 84,
  score: 1240,
  actions: [],
  resolvedEventIds: [],
};
const slice = createSlice({
  name: "app",
  initialState,
  reducers: {
    navigate(state, action: PayloadAction<AppScreen>) {
      state.previousScreen = state.screen;
      state.screen = action.payload;
    },
    startScenario(state, action: PayloadAction<string>) {
      state.scenarioId = action.payload;
      state.eventIndex = 0;
      state.safety = 92;
      state.loyalty = 84;
      state.actions = [];
      state.resolvedEventIds = [];
      state.previousScreen = state.screen;
      state.screen = "simulation";
    },
    selectEvent(state, action: PayloadAction<number>) {
      const scenario = scenarios.find((item) => item.id === state.scenarioId);
      const event = scenario?.events[action.payload];
      if (event && !state.resolvedEventIds.includes(event.id)) {
        state.eventIndex = action.payload;
      }
    },
    makeChoice(state, action: PayloadAction<ScenarioChoice>) {
      const scenario = scenarios.find((item) => item.id === state.scenarioId);
      const event = scenario?.events[state.eventIndex];
      if (!event) return;

      state.safety = clampMetric(state.safety + action.payload.safety);
      state.loyalty = clampMetric(state.loyalty + action.payload.loyalty);
      state.score += Math.max(0, action.payload.safety + action.payload.loyalty) * 5;
      state.actions.push({
        eventTitle: event.title,
        choiceTitle: action.payload.title,
        feedback: action.payload.feedback,
        safety: action.payload.safety,
        loyalty: action.payload.loyalty,
        competency: action.payload.competency,
      });
      state.resolvedEventIds.push(event.id);

      const nextIndex = scenario.events.findIndex((item) => !state.resolvedEventIds.includes(item.id));
      if (nextIndex >= 0) {
        state.eventIndex = nextIndex;
      } else {
        state.previousScreen = state.screen;
        state.screen = "debrief";
      }
    },
    signedIn(state, action: PayloadAction<AuthResult>) {
      state.auth = action.payload;
      state.screen = "home";
    },
    setPlayer(state, action: PayloadAction<Player>) {
      if (state.auth) state.auth.player = action.payload;
    },
    signedOut() {
      setTokens(null);
      return initialState;
    },
    setShift(state, action: PayloadAction<SessionResponse>) {
      state.shift = action.payload;
      state.situationId =
        action.payload.situations.find((item) => item.status === "active")
          ?.id ??
        action.payload.situations[0]?.id ??
        null;
      state.breakdown = null;
      state.screen = "simulation";
    },
    selectSituation(state, action: PayloadAction<string>) {
      state.situationId = action.payload;
    },
    refreshShift(state, action: PayloadAction<SessionResponse>) {
      state.shift = action.payload;
    },
    setBreakdown(state, action: PayloadAction<Breakdown>) {
      state.breakdown = action.payload;
      if (state.shift) state.shift.session.status = "finished";
      state.screen = "debrief";
    },
  },
});
export const {
  navigate,
  startScenario,
  selectEvent,
  makeChoice,
  signedIn,
  signedOut,
  setShift,
  selectSituation,
  refreshShift,
  setBreakdown,
  setPlayer,
} = slice.actions;
export const store = configureStore({ reducer: { app: slice.reducer } });
export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
export const useAppDispatch = useDispatch.withTypes<AppDispatch>();
export const useAppSelector = useSelector.withTypes<RootState>();
