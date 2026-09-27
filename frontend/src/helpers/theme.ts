export const colors = {
  primary: '#122A91',
  primaryDark: '#0A1B67',
  primaryLight: '#3158CC',
  blueSoft: '#EAF0FF',
  ink: '#20232B',
  muted: '#687389',
  soft: '#F4F7FB',
  surface: '#FFFFFF',
  border: '#DCE3EF',
  safety: '#22C55E',
  loyalty: '#3B82F6',
  warning: '#F59E0B',
  critical: '#E83343',
  dark: '#101A3D',
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
