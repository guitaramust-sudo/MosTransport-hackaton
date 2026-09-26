import Constants from 'expo-constants'
import { Platform } from 'react-native'
import type { AuthResult, Breakdown, Profile, SessionResponse, Situation, SituationResponse, Tokens, TurnResult } from '../types'

const expoHost = Constants.expoConfig?.hostUri?.split(':')[0]
const defaultHost = Platform.OS === 'android' ? (expoHost || '10.0.2.2') : 'localhost'
// The production web container proxies API routes through Nginx, so web can
// stay on the same origin. Native clients connect to the backend directly.
const defaultBaseUrl = Platform.OS === 'web' ? '' : `http://${defaultHost}:8088`
const baseUrl = process.env.EXPO_PUBLIC_API_URL?.replace(/\/$/, '') ?? defaultBaseUrl
let tokens: Tokens | null = null

export function setTokens(value: Tokens | null) { tokens = value }

export function getApiBaseUrl() { return baseUrl }

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
  register: (email: string, username: string, password: string) => request<AuthResult>('/auth/register', 'POST', { email, username, password }),
  profile: async () => normalizeProfile(await request<Profile>('/api/profile')),
  startSession: async () => normalizeSession(await request<SessionResponse>('/api/session/start', 'POST', {})),
  getSession: async (id: string) => normalizeSession(await request<SessionResponse>(`/api/session/${id}`)),
  finishSession: async (id: string) => normalizeBreakdown(await request<Breakdown>(`/api/session/${id}/finish`, 'POST', {})),
  getSituation: async (id: string) => normalizeSituationResponse(await request<SituationResponse>(`/api/situation/${id}`)),
  sendMessage: (id: string, text: string) => request<TurnResult>(`/api/situation/${id}/message`, 'POST', { text }),
  escalate: (id: string, to: string) => request<{ escalations: string[] }>(`/api/situation/${id}/escalate`, 'POST', { to }),
  finishSituation: (id: string) => request<{ outcome: string }>(`/api/situation/${id}/finish`, 'POST', {}),
}
