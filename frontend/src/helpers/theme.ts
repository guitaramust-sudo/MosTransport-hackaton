export const colors = {
  primary: '#C91C2B',
  primaryDark: '#981523',
  ink: '#172126',
  muted: '#67757D',
  soft: '#F1F4F5',
  surface: '#FFFFFF',
  border: '#DDE4E7',
  safety: '#1B8A67',
  loyalty: '#2878C8',
  warning: '#E58B21',
  critical: '#C91C2B',
  dark: '#19282E',
} as const

export const spacing = {
  xs: 6,
  sm: 10,
  md: 16,
  lg: 24,
  xl: 32,
} as const

export const radius = {
  sm: 10,
  md: 16,
  lg: 24,
  pill: 999,
} as const

export const shadow = {
  shadowColor: '#14252C',
  shadowOffset: { width: 0, height: 6 },
  shadowOpacity: 0.08,
  shadowRadius: 16,
  elevation: 3,
} as const

export const clampMetric = (value: number) => Math.max(0, Math.min(100, value))
