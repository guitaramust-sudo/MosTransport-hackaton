// Canonical VSM UX/UI tokens (VSM_UX_UI_Canonical_Direction §4, §23).
export const palette = {
  brandBlue: '#122A91',
  actionBlue: '#315BCC',
  blue50: '#EAF0FF',
  brandRed: '#E83343',
  gray: '#EBEBEB',
  n50: '#F8F9FC',
  n100: '#F2F4F8',
  n200: '#E7EAF0',
  n300: '#D2D6DF',
  n400: '#A4AAB8',
  n500: '#727B87',
  n600: '#454A57',
  n900: '#20232B',
  success: '#22C55E',
  warning: '#F59E0B',
  error: '#EF4444',
  info: '#3B82F6',
  gold: '#FFD166',
  rare: '#A78BFA',
  epic: '#60A5FA',
  completed: '#10B981',
  locked: '#94A3B8',
} as const

export const colors = {
  // Surfaces
  background: palette.n50,
  surface: '#FFFFFF',
  soft: palette.n100,
  border: palette.n200,
  divider: palette.n200,
  // Text
  ink: palette.n900,
  secondary: palette.n600,
  muted: palette.n500,
  faint: palette.n400,
  // Brand and action
  primary: palette.brandBlue,
  primaryDark: '#0A1B67',
  action: palette.actionBlue,
  primaryLight: palette.actionBlue,
  blueSoft: palette.blue50,
  brandRed: palette.brandRed,
  // Semantic
  success: palette.success,
  successSoft: '#E8F8EE',
  successInk: '#15803D',
  warning: palette.warning,
  warningSoft: '#FEF3DC',
  warningInk: '#B45309',
  error: palette.error,
  errorSoft: '#FDECEC',
  errorInk: '#B91C1C',
  info: palette.info,
  locked: palette.locked,
  gold: palette.gold,
  // Game metrics (kept for in-game screens)
  safety: palette.success,
  loyalty: palette.info,
  critical: palette.brandRed,
  dark: '#101A3D',
  overlay: 'rgba(32, 35, 43, 0.42)',
} as const

export const spacing = {
  xxs: 4,
  xs: 8,
  sm: 12,
  md: 16,
  lg: 24,
  xl: 32,
} as const

export const radius = {
  sm: 10,
  button: 14,
  md: 16,
  card: 20,
  lg: 24,
  sheet: 28,
  pill: 999,
} as const

// Onest scale from §23.2: size / line-height / weight.
export const type = {
  h1: { fontSize: 28, lineHeight: 36, fontWeight: '700' },
  h2: { fontSize: 24, lineHeight: 32, fontWeight: '600' },
  h3: { fontSize: 20, lineHeight: 28, fontWeight: '600' },
  cardTitle: { fontSize: 18, lineHeight: 26, fontWeight: '600' },
  body: { fontSize: 16, lineHeight: 24, fontWeight: '400' },
  secondary: { fontSize: 14, lineHeight: 20, fontWeight: '400' },
  label: { fontSize: 12, lineHeight: 16, fontWeight: '500' },
  number: { fontSize: 32, lineHeight: 36, fontWeight: '700' },
} as const

export const shadow = {
  shadowColor: '#122A91',
  shadowOffset: { width: 0, height: 4 },
  shadowOpacity: 0.06,
  shadowRadius: 14,
  elevation: 2,
} as const

export const maxContentWidth = 560

export const clampMetric = (value: number) => Math.max(0, Math.min(100, value))
