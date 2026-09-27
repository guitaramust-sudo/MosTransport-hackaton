import Constants from 'expo-constants'
import { Platform } from 'react-native'
import type { AdminLearningSummary, AuthResult, Breakdown, LiveResult, LiveSimulation, Player, Profile, SessionResponse, Situation, SituationResponse, Tokens, TurnResult, WagonClassesResponse, WagonLevelsResponse, WagonStartResponse } from '../types'

const expoHost = Constants.expoConfig?.hostUri?.split(':')[0]
const defaultHost = Platform.OS === 'android' ? (expoHost || '10.0.2.2') : 'localhost'
// The production web container proxies API routes through Nginx, so web can
// stay on the same origin. Native clients connect to the backend directly.
const defaultBaseUrl = Platform.OS === 'web' ? '' : `http://${defaultHost}:8088`
const baseUrl = process.env.EXPO_PUBLIC_API_URL?.replace(/\/$/, '') ?? defaultBaseUrl
let tokens: Tokens | null = null

export function setTokens(value: Tokens | null) { tokens = value }
export function getAccessToken() { return tokens?.access_token ?? null }

export function getApiBaseUrl() { return baseUrl }

export function getWagonWebSocketUrl(path: string) {
  const token = getAccessToken()
  const separator = path.includes('?') ? '&' : '?'
  const authenticatedPath = `${path}${separator}token=${encodeURIComponent(token ?? '')}`
  if (baseUrl) return `${baseUrl.replace(/^http/, 'ws')}${authenticatedPath}`
  const location = (globalThis as typeof globalThis & { location?: { origin?: string } }).location
  const origin = location?.origin?.replace(/^http/, 'ws') ?? ''
  return `${origin}${authenticatedPath}`
}

function normalizeSituation(situation: Situation): Situation {
  return {
    ...situation,
    escalations: situation.escalations ?? [],
    remarks: situation.remarks ?? [],
    conveyed: situation.conveyed ?? [],
    missed: situation.missed ?? [],
  }
}

function normalizeSession(response: SessionResponse): SessionResponse {
  return {
    ...response,
    situations: (response.situations ?? []).map(normalizeSituation),
  }
}

function normalizeSituationResponse(response: SituationResponse): SituationResponse {
  return {
    situation: normalizeSituation(response.situation),
    messages: response.messages ?? [],
  }
}

function normalizeBreakdown(response: Breakdown): Breakdown {
  return {
    ...response,
    competencies_xp: response.competencies_xp ?? {},
    situations: (response.situations ?? []).map((situation) => ({
      ...situation,
      remarks: situation.remarks ?? [],
    })),
  }
}

function normalizeProfile(response: Profile): Profile {
  return { ...response, competencies: response.competencies ?? [] }
}

async function request<T>(path: string, method = 'GET', body?: object, retry = true): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', ...(tokens ? { Authorization: `Bearer ${tokens.access_token}` } : {}) },
    ...(body ? { body: JSON.stringify(body) } : {}),
  })
  if (response.status === 401 && retry && tokens?.refresh_token && path !== '/auth/refresh') {
    const refreshed = await request<AuthResult>('/auth/refresh', 'POST', { refresh_token: tokens.refresh_token }, false)
    setTokens(refreshed.tokens)
    return request<T>(path, method, body, false)
  }
  if (!response.ok) {
    const error = await response.json().catch(() => ({})) as { error?: string }
    throw new Error(error.error ?? `HTTP ${response.status}`)
  }
  return response.json() as Promise<T>
}

export const api = {
  login: (email: string, password: string) => request<AuthResult>('/auth/login', 'POST', { email, password }),
  createPlayer: (body: { email: string; username: string; password: string; brigade_name: string }) => request<{ player: Player }>('/admin/players', 'POST', body),
  upsertExternalUser: (body: { source_system: string; external_user_id: string; display_name?: string; depot_id?: string; brigade_id?: string; assigned_class_ids: string[] }) => request<{ user_id: string; created: boolean; player: Player }>('/admin/users', 'POST', body),
  learningSummary: (id: string) => request<AdminLearningSummary>(`/admin/users/${encodeURIComponent(id)}/learning-summary`),
  approveSession: (id: string) => request<{ session_id: string; validation_status: string }>(`/admin/sessions/${encodeURIComponent(id)}/approve`, 'POST', {}),
  profile: async () => normalizeProfile(await request<Profile>('/api/profile')),
  startSession: async () => normalizeSession(await request<SessionResponse>('/api/session/start', 'POST', {})),
  getSession: async (id: string) => normalizeSession(await request<SessionResponse>(`/api/session/${id}`)),
  finishSession: async (id: string) => normalizeBreakdown(await request<Breakdown>(`/api/session/${id}/finish`, 'POST', {})),
  getSituation: async (id: string) => normalizeSituationResponse(await request<SituationResponse>(`/api/situation/${id}`)),
  sendMessage: (id: string, text: string, inputMode: 'text' | 'voice' = 'text') =>
    request<TurnResult>(`/api/situation/${id}/message`, 'POST', { text, input_mode: inputMode }),
  escalate: (id: string, to: string) => request<{ escalations: string[] }>(`/api/situation/${id}/escalate`, 'POST', { to }),
  finishSituation: (id: string) => request<{ outcome: string }>(`/api/situation/${id}/finish`, 'POST', {}),
  startLiveSimulation: () => request<LiveSimulation>('/api/session/simulations', 'POST', {}),
  getLiveSimulation: (id: string) => request<LiveSimulation>(`/api/session/simulations/${id}`),
  getLiveResult: (id: string) => request<LiveResult>(`/api/session/simulations/${id}/result`),
  liveAction: (id: string, body: { command_id: string; expected_state_version: number; action_id?: string; event_id?: string; target?: string; choice_id?: string }) =>
    request<LiveSimulation>(`/api/session/simulations/${id}/actions`, 'POST', body),
  liveDialogue: (id: string, body: { command_id: string; expected_state_version: number; event_id: string; text: string }) =>
    request<LiveSimulation>(`/api/session/simulations/${id}/dialogue`, 'POST', body),
  getWagonClasses: () => request<WagonClassesResponse>('/api/wagon/classes'),
  getWagonLevels: () => request<WagonLevelsResponse>('/api/wagon/levels'),
  startWagonSession: (levelId: string) =>
    request<WagonStartResponse>('/api/session/wagon/start', 'POST', { level_id: levelId }),
}
