import type { WagonAnchor, WagonItem, WagonSituationType } from '../types'

export interface WagonPoint { x: number; y: number; z: number }

export const wagonAnchorPositions: Record<WagonAnchor, WagonPoint> = {
  seat_1: { x: -0.32, y: 0.245, z: 3.7 },
  seat_2: { x: 1.26, y: 0.245, z: 3.7 },
  seat_3: { x: -0.32, y: 0.245, z: 1.25 },
  seat_4: { x: 1.26, y: 0.245, z: 1.25 },
  seat_5: { x: -0.32, y: 0.245, z: -1.35 },
  seat_6: { x: 1.26, y: 0.245, z: -1.35 },
  service_point: { x: 0.47, y: 0.245, z: -5.3 },
  staff_zone: { x: 0.47, y: 0.245, z: 5.45 },
}

export const wagonAnchorLabels: Record<WagonAnchor, string> = {
  seat_1: 'Место 1', seat_2: 'Место 2', seat_3: 'Место 3', seat_4: 'Место 4',
  seat_5: 'Место 5', seat_6: 'Место 6', service_point: 'Сервисная стойка', staff_zone: 'Служебная зона',
}

export const wagonItemLabels: Record<WagonItem, string> = {
  blanket: 'Плед', water: 'Вода', coffee: 'Кофе',
}

export function wagonSituationIcon(type: WagonSituationType) {
  if (type === 'cold') return '❄'
  if (type === 'thirsty') return '💧'
  if (type === 'tired') return '◔'
  if (type === 'zone_intrusion') return '⚠'
  return '!'
}

export function interpolateWagonActor(
  actor: { at: WagonAnchor; moving?: { from: WagonAnchor; to: WagonAnchor; started_at: string; duration_s: number } | null },
  now = Date.now(),
) {
  const fallback = wagonAnchorPositions[actor.at]
  const moving = actor.moving
  if (!moving) return { ...fallback, progress: 1 }
  const from = wagonAnchorPositions[moving.from] ?? fallback
  const to = wagonAnchorPositions[moving.to] ?? fallback
  const startedAt = Date.parse(moving.started_at)
  const durationMs = Math.max(1, moving.duration_s * 1000)
  const progress = Number.isFinite(startedAt) ? Math.max(0, Math.min(1, (now - startedAt) / durationMs)) : 0
  return {
    x: from.x + (to.x - from.x) * progress,
    y: from.y + (to.y - from.y) * progress,
    z: from.z + (to.z - from.z) * progress,
    progress,
  }
}
