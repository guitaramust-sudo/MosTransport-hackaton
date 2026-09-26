import { Platform } from 'react-native'
import type { AuthResult, Breakdown, Profile, SessionResponse, SituationResponse, Tokens, TurnResult } from '../types'

const defaultHost = Platform.OS === 'android' ? '10.0.2.2' : 'localhost'
const baseUrl = process.env.EXPO_PUBLIC_API_URL?.replace(/\/$/, '') ?? `http://${defaultHost}:8088`
let tokens: Tokens | null = null

export function setTokens(value: Tokens | null) { tokens = value }

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
  profile: () => request<Profile>('/api/profile'),
  startSession: () => request<SessionResponse>('/api/session/start', 'POST', {}),
  getSession: (id: string) => request<SessionResponse>(`/api/session/${id}`),
  finishSession: (id: string) => request<Breakdown>(`/api/session/${id}/finish`, 'POST', {}),
  getSituation: (id: string) => request<SituationResponse>(`/api/situation/${id}`),
  sendMessage: (id: string, text: string) => request<TurnResult>(`/api/situation/${id}/message`, 'POST', { text }),
  escalate: (id: string, to: string) => request<{ escalations: string[] }>(`/api/situation/${id}/escalate`, 'POST', { to }),
  finishSituation: (id: string) => request<{ outcome: string }>(`/api/situation/${id}/finish`, 'POST', {}),
}
