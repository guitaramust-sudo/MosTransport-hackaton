import type { ReactNode } from 'react'
import {
  ActivityIndicator, Modal, Pressable, ScrollView, StyleSheet, View,
  type StyleProp, type ViewStyle,
} from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'
import { SvgXml } from 'react-native-svg'
import { badgeSvgs, type BadgeId } from '../assets/badges'
import { Icon, type IconName } from './Icon'
import { Text } from './Typography'
import { colors, maxContentWidth, radius, shadow, spacing, type } from '../helpers/theme'

/* ---------- Layout ---------- */

export function Screen({ title, subtitle, onBack, right, children, bottomInset = true }: {
  title?: string
  subtitle?: string
  onBack?: () => void
  right?: ReactNode
  children: ReactNode
  bottomInset?: boolean
}) {
  return (
    <ScrollView style={styles.page} contentContainerStyle={[styles.content, !bottomInset && { paddingBottom: spacing.lg }]}
      keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false}>
      <View style={styles.column}>
        {(title || onBack || right) && (
          <View style={styles.header}>
            {onBack && <IconButton icon="chevronLeft" label="Назад" onPress={onBack} />}
            <View style={styles.headerText}>
              {title && <Text style={styles.title} accessibilityRole="header">{title}</Text>}
              {subtitle && <Text style={styles.subtitle}>{subtitle}</Text>}
            </View>
            {right}
          </View>
        )}
        {children}
      </View>
    </ScrollView>
  )
}

/** Backwards-compatible page wrapper used by focused/admin screens. */
export function Page({ title, children, onBack }: { title: string; children: ReactNode; onBack?: () => void }) {
  return <Screen title={title} onBack={onBack}>{children}</Screen>
}

export function Section({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <View style={styles.section}>
      <View style={styles.sectionHeader}>
        <Text style={styles.sectionTitle} accessibilityRole="header">{title}</Text>
        {action}
      </View>
      {children}
    </View>
  )
}

/** Thin blue/red speed line — the VSM brand accent (§17, §24.2). */
export function SpeedLine({ style }: { style?: StyleProp<ViewStyle> }) {
  return (
    <View style={[styles.speed, style]} accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      <View style={styles.speedBlue} />
      <View style={styles.speedRed} />
    </View>
  )
}

/* ---------- Surfaces ---------- */

export function Card({ children, style, onPress, accessibilityLabel }: {
  children: ReactNode
  style?: StyleProp<ViewStyle>
  onPress?: () => void
  accessibilityLabel?: string
}) {
  if (!onPress) return <View style={[styles.card, style]}>{children}</View>
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={accessibilityLabel} onPress={onPress}
      style={({ pressed }) => [styles.card, style, pressed && styles.pressed]}>
      {children}
    </Pressable>
  )
}

export function ListRow({ icon, title, caption, onPress, tone = 'default', right }: {
  icon: IconName
  title: string
  caption?: string
  onPress?: () => void
  tone?: 'default' | 'danger'
  right?: ReactNode
}) {
  const color = tone === 'danger' ? colors.errorInk : colors.ink
  return (
    <Pressable accessibilityRole="button" onPress={onPress} disabled={!onPress}
      style={({ pressed }) => [styles.row, pressed && styles.rowPressed]}>
      <View style={[styles.rowIcon, tone === 'danger' && { backgroundColor: colors.errorSoft }]}>
        <Icon name={icon} size={22} color={tone === 'danger' ? colors.errorInk : colors.action} />
      </View>
      <View style={styles.rowBody}>
        <Text style={[styles.rowTitle, { color }]}>{title}</Text>
        {caption && <Text style={styles.rowCaption}>{caption}</Text>}
      </View>
      {right ?? (onPress && <Icon name="chevronRight" size={20} color={colors.faint} />)}
    </Pressable>
  )
}

/* ---------- Controls ---------- */

type ButtonVariant = 'primary' | 'secondary' | 'tertiary' | 'destructive'

export function Button({ title, onPress, busy, disabled, secondary, variant, icon, style }: {
  title: string
  onPress: () => void
  busy?: boolean
  disabled?: boolean
  /** @deprecated use variant="secondary" */
  secondary?: boolean
  variant?: ButtonVariant
  icon?: IconName
  style?: StyleProp<ViewStyle>
}) {
  const kind: ButtonVariant = variant ?? (secondary ? 'secondary' : 'primary')
  const textColor = kind === 'primary' || kind === 'destructive' ? colors.surface : colors.ink
  return (
    <Pressable accessibilityRole="button" accessibilityState={{ disabled: !!(busy || disabled), busy: !!busy }}
      disabled={busy || disabled} onPress={onPress}
      style={({ pressed }) => [styles.button, buttonStyles[kind], (busy || disabled) && styles.disabled, pressed && styles.pressed, style]}>
      {busy ? <ActivityIndicator color={textColor} /> : <>
        <Text style={[styles.buttonText, { color: textColor }]}>{title}</Text>
        {icon && <Icon name={icon} size={20} color={textColor} strokeWidth={2} />}
      </>}
    </Pressable>
  )
}

export function IconButton({ icon, label, onPress, color = colors.secondary }: {
  icon: IconName; label: string; onPress: () => void; color?: string
}) {
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={label} onPress={onPress} hitSlop={6}
      style={({ pressed }) => [styles.iconButton, pressed && styles.pressed]}>
      <Icon name={icon} size={24} color={color} />
    </Pressable>
  )
}

/* ---------- Status and progress ---------- */

export type PillTone = 'success' | 'progress' | 'neutral' | 'critical' | 'warning'

const pillTones: Record<PillTone, { bg: string; fg: string; dot: string }> = {
  success: { bg: colors.successSoft, fg: colors.successInk, dot: colors.success },
  progress: { bg: colors.blueSoft, fg: colors.action, dot: colors.action },
  neutral: { bg: colors.soft, fg: colors.secondary, dot: colors.faint },
  critical: { bg: colors.errorSoft, fg: colors.errorInk, dot: colors.error },
  warning: { bg: colors.warningSoft, fg: colors.warningInk, dot: colors.warning },
}

export function Pill({ label, tone = 'neutral', icon }: { label: string; tone?: PillTone; icon?: IconName }) {
  const t = pillTones[tone]
  return (
    <View style={[styles.pill, { backgroundColor: t.bg }]}>
      {icon ? <Icon name={icon} size={14} color={t.fg} strokeWidth={2} /> : <View style={[styles.pillDot, { backgroundColor: t.dot }]} />}
      <Text style={[styles.pillText, { color: t.fg }]}>{label}</Text>
    </View>
  )
}

export function ProgressBar({ value, color = colors.action, trackColor = colors.soft, height = 8, label }: {
  value: number; color?: string; trackColor?: string; height?: number; label?: string
}) {
  const pct = Math.max(0, Math.min(100, value))
  return (
    <View accessibilityRole="progressbar" accessibilityLabel={label}
      accessibilityValue={{ min: 0, max: 100, now: Math.round(pct) }}
      style={[styles.track, { height, borderRadius: height, backgroundColor: trackColor }]}>
      <View style={[styles.fill, { width: `${pct}%`, backgroundColor: color, borderRadius: height }]} />
    </View>
  )
}

export function Stat({ label, value, caption, accent = colors.primary }: {
  label: string; value: string; caption?: string; accent?: string
}) {
  return (
    <View style={styles.stat}>
      <Text style={styles.statLabel}>{label}</Text>
      <Text style={[styles.statValue, { color: accent }]}>{value}</Text>
      {caption && <Text style={styles.statCaption}>{caption}</Text>}
    </View>
  )
}

/* ---------- Identity ---------- */

export function Avatar({ name, size = 64 }: { name?: string | null; size?: number }) {
  const initial = name?.trim().slice(0, 1).toUpperCase() || 'П'
  return (
    <View style={[styles.avatar, { width: size, height: size, borderRadius: size / 2 }]}>
      <Text style={[styles.avatarText, { fontSize: size * 0.4 }]}>{initial}</Text>
    </View>
  )
}

export function Badge({ id, size = 72, locked }: { id: BadgeId; size?: number; locked?: boolean }) {
  return (
    <View style={{ width: size, height: size }}>
      <View style={locked && styles.badgeLocked}>
        <SvgXml xml={badgeSvgs[id]} width={size} height={size} />
      </View>
      {locked && (
        <View style={styles.badgeLock}>
          <Icon name="lock" size={Math.max(16, size * 0.28)} color={colors.secondary} strokeWidth={2} />
        </View>
      )}
    </View>
  )
}

const medalTones = {
  gold: { fill: '#FFD166', ring: '#F5B82E', ink: '#7A4B00' },
  violet: { fill: '#A78BFA', ring: '#8B6CF0', ink: '#FFFFFF' },
} as const

/** Achievement medal (§13: light volume, gold/violet); dimmed with a lock until earned. */
export function Medal({ icon, tone, size = 76, locked }: { icon: IconName; tone: keyof typeof medalTones; size?: number; locked?: boolean }) {
  const t = medalTones[tone]
  return (
    <View style={[styles.medal, { width: size, height: size, borderRadius: size / 2, backgroundColor: locked ? colors.soft : t.fill, borderColor: locked ? colors.border : t.ring }]}>
      <Icon name={locked ? 'lock' : icon} size={size * 0.42} color={locked ? colors.locked : t.ink} strokeWidth={2.2} />
    </View>
  )
}

/* ---------- Feedback ---------- */

export function ErrorText({ error }: { error: string | null }) {
  return error ? <Text accessibilityRole="alert" style={styles.error}>{error}</Text> : null
}

export function EmptyState({ icon, title, text, action }: { icon: IconName; title: string; text: string; action?: ReactNode }) {
  return (
    <View style={styles.empty}>
      <View style={styles.emptyIcon}><Icon name={icon} size={26} color={colors.action} /></View>
      <Text style={styles.emptyTitle}>{title}</Text>
      <Text style={styles.emptyText}>{text}</Text>
      {action}
    </View>
  )
}

/* ---------- Bottom sheet ---------- */

export function Sheet({ visible, title, onClose, children }: {
  visible: boolean; title: string; onClose: () => void; children: ReactNode
}) {
  const insets = useSafeAreaInsets()
  // Unmount instead of animating out: react-native-web can leave a hidden
  // modal layer on top of the page when the exit animation is interrupted.
  if (!visible) return null
  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose} statusBarTranslucent>
      <View style={styles.sheetRoot}>
        <Pressable style={StyleSheet.absoluteFill} accessibilityLabel="Закрыть" onPress={onClose} />
        <View style={[styles.sheet, { paddingBottom: 20 + insets.bottom }]}>
          <View style={styles.grabber} />
          <View style={styles.sheetHeader}>
            <Text style={styles.sheetTitle} accessibilityRole="header">{title}</Text>
            <IconButton icon="close" label="Закрыть" onPress={onClose} />
          </View>
          {children}
        </View>
      </View>
    </Modal>
  )
}

const buttonStyles = StyleSheet.create({
  primary: { backgroundColor: colors.action },
  secondary: { backgroundColor: colors.blueSoft, borderWidth: 1, borderColor: '#D6E0FA' },
  tertiary: { backgroundColor: 'transparent', borderWidth: 1, borderColor: colors.border },
  destructive: { backgroundColor: colors.brandRed },
})

const styles = StyleSheet.create({
  page: { flex: 1, backgroundColor: colors.background },
  content: { paddingHorizontal: spacing.md, paddingTop: spacing.sm, paddingBottom: 40, alignItems: 'center' },
  column: { width: '100%', maxWidth: maxContentWidth },
  header: { minHeight: 56, flexDirection: 'row', alignItems: 'center', gap: spacing.xs, marginBottom: spacing.sm },
  headerText: { flex: 1 },
  title: { ...type.h1, color: colors.ink },
  subtitle: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  section: { marginTop: spacing.lg },
  sectionHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: spacing.sm },
  sectionTitle: { ...type.h3, color: colors.ink },
  speed: { height: 6, justifyContent: 'center' },
  speedBlue: { height: 3, borderRadius: 3, backgroundColor: colors.action, width: '100%' },
  speedRed: { height: 2, borderRadius: 2, backgroundColor: colors.brandRed, width: '62%', marginTop: 1, alignSelf: 'flex-end' },
  card: { backgroundColor: colors.surface, borderRadius: radius.card, padding: 20, borderWidth: 1, borderColor: colors.border, marginBottom: spacing.sm },
  pressed: { opacity: 0.86, transform: [{ scale: 0.99 }] },
  row: { minHeight: 64, flexDirection: 'row', alignItems: 'center', gap: spacing.sm, paddingVertical: spacing.sm },
  rowPressed: { opacity: 0.7 },
  rowIcon: { width: 44, height: 44, borderRadius: 14, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  rowBody: { flex: 1 },
  rowTitle: { fontSize: 16, lineHeight: 22, fontWeight: '500' },
  rowCaption: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  button: { minHeight: 52, borderRadius: radius.button, paddingHorizontal: 20, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: spacing.xs, marginTop: spacing.sm },
  buttonText: { fontSize: 16, lineHeight: 22, fontWeight: '600' },
  disabled: { opacity: 0.45 },
  iconButton: { width: 44, height: 44, borderRadius: 22, alignItems: 'center', justifyContent: 'center' },
  pill: { flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start', gap: 6, paddingHorizontal: 10, paddingVertical: 5, borderRadius: radius.pill },
  pillDot: { width: 7, height: 7, borderRadius: 4 },
  pillText: { ...type.label },
  track: { width: '100%', backgroundColor: colors.soft, overflow: 'hidden' },
  fill: { height: '100%' },
  stat: { flex: 1, alignItems: 'center', backgroundColor: colors.surface, borderRadius: radius.card, borderWidth: 1, borderColor: colors.border, paddingVertical: spacing.md, paddingHorizontal: spacing.xs },
  statLabel: { ...type.secondary, color: colors.secondary },
  statValue: { fontSize: 22, lineHeight: 28, fontWeight: '700', marginTop: 4 },
  statCaption: { ...type.label, color: colors.muted, marginTop: 2, textAlign: 'center' },
  avatar: { alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft, borderWidth: 2, borderColor: colors.surface, ...shadow },
  avatarText: { color: colors.primary, fontWeight: '700' },
  medal: { alignItems: 'center', justifyContent: 'center', borderWidth: 5 },
  badgeLocked: { opacity: 0.28 },
  badgeLock: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center' },
  error: { ...type.secondary, color: colors.errorInk, marginVertical: spacing.xs },
  empty: { alignItems: 'center', paddingVertical: spacing.lg, paddingHorizontal: spacing.md },
  emptyIcon: { width: 56, height: 56, borderRadius: 18, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  emptyTitle: { ...type.cardTitle, color: colors.ink, marginTop: spacing.sm, textAlign: 'center' },
  emptyText: { ...type.secondary, color: colors.secondary, marginTop: 4, textAlign: 'center' },
  sheetRoot: { flex: 1, justifyContent: 'flex-end', backgroundColor: colors.overlay },
  sheet: { width: '100%', maxWidth: 640, alignSelf: 'center', backgroundColor: colors.surface, borderTopLeftRadius: radius.sheet, borderTopRightRadius: radius.sheet, paddingHorizontal: 20 },
  grabber: { alignSelf: 'center', width: 40, height: 5, borderRadius: 3, backgroundColor: colors.border, marginTop: 10 },
  sheetHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 10, marginBottom: spacing.md },
  sheetTitle: { ...type.h2, color: colors.ink, flex: 1 },
})
