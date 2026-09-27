import type { WagonAnchor, WagonItem, WagonSituationType } from '../types'
import { findPath, getActiveNavGrid, type FloorPoint } from './navGrid'

export type { FloorPoint }

export interface WagonPoint { x: number; y: number; z: number }

// Seat anchors sit in real seats of TOOOPblend.glb. All selected chairs face
// +Z, matching the seated character animation.
export const wagonAnchorPositions: Record<WagonAnchor, WagonPoint> = {
  seat_1: { x: -0.32, y: 0.245, z: 4.15 },
  seat_2: { x: 1.27, y: 0.245, z: -0.53 },
  seat_3: { x: -0.32, y: 0.245, z: -2.93 },
  seat_4: { x: 1.27, y: 0.245, z: -2.93 },
  seat_5: { x: -0.32, y: 0.245, z: -5.33 },
  seat_6: { x: 1.27, y: 0.245, z: -5.33 },
  seat_7: { x: -1.27, y: 0.245, z: 4.15 }, // Seat_R02_L_Window
  seat_8: { x: -0.32, y: 0.245, z: -0.53 }, // Seat_R05_L_Aisle
  seat_9: { x: -0.32, y: 0.245, z: -1.73 }, // Seat_R06_L_Aisle
  seat_10: { x: 1.27, y: 0.245, z: -1.73 }, // Seat_R06_R_Single
  seat_11: { x: -0.32, y: 0.245, z: -4.13 }, // Seat_R08_L_Aisle
  seat_12: { x: 1.27, y: 0.245, z: -4.13 }, // Seat_R08_R_Single
  service_point: { x: 0.47, y: 0.245, z: -5.3 },
  staff_zone: { x: 0.47, y: 0.245, z: 5.45 },
  // First-class anchors, placed from TOOOPblend.glb geometry until the 3D team
  // hands over the official anchor_id -> transform table (GDD §31):
  // central galley with water bottles (z 3.6..6.6, right side), rear vestibule
  // with the WC and the fire extinguisher (z -6.8..-8.8), front vestibule at
  // the inter-car door (z 6.7..8.8), used as the "cab entrance" boundary.
  service_zone: { x: 0.47, y: 0.245, z: 5.2 },
  sanitary_zone: { x: 0.47, y: 0.245, z: -7.6 },
  cab_entrance_boundary: { x: 0.47, y: 0.245, z: 8.1 },
}

/*
 * Walkable aisle centerline (x, z) traced from the model's burgundy runner:
 * straight at x≈0.47 through the seating zone, a jog to x≈-0.32 around the
 * lounge (wardrobe and service-alcove walls sit on the right there), then back.
 * Partition doorways at z≈±6.7 and the vestibules are on the x≈0.47 line; the
 * closed inter-car doors at z≈±8.84 bound it.
 */
const AISLE_LINE: ReadonlyArray<readonly [number, number]> = [
  [0.47, -8.5], [0.47, -0.05], [-0.32, 1.0], [-0.32, 2.5], [0.47, 3.5], [0.47, 8.5],
]
export const AISLE_MIN_Z = AISLE_LINE[0][1]
export const AISLE_MAX_Z = AISLE_LINE[AISLE_LINE.length - 1][1]


/** X of the aisle centerline at depth z. */
export function aisleX(z: number) {
  const clamped = Math.max(AISLE_MIN_Z, Math.min(AISLE_MAX_Z, z))
  for (let i = 0; i < AISLE_LINE.length - 1; i++) {
    const [x0, z0] = AISLE_LINE[i]
    const [x1, z1] = AISLE_LINE[i + 1]
    if (clamped >= z0 && clamped <= z1) return z1 === z0 ? x0 : x0 + ((clamped - z0) / (z1 - z0)) * (x1 - x0)
  }
  return AISLE_LINE[0][0]
}

export function aislePoint(z: number): FloorPoint {
  const clamped = Math.max(AISLE_MIN_Z, Math.min(AISLE_MAX_Z, z))
  return { x: aisleX(clamped), z: clamped }
}

/** Fallback route along the aisle centerline, used until the model's nav grid is ready. */
function centerlineRoute(from: FloorPoint, to: FloorPoint): FloorPoint[] {
  const start = aislePoint(from.z)
  const end = aislePoint(to.z)
  const bends = AISLE_LINE
    .filter(([, z]) => z > Math.min(start.z, end.z) && z < Math.max(start.z, end.z))
    .map(([x, z]) => ({ x, z }))
  if (start.z > end.z) bends.reverse()
  return [from, start, ...bends, end, to]
}

const routeCache = new Map<string, FloorPoint[]>()

/**
 * Walkable route between two floor points: path-finds around seats, walls and
 * cabinets on the model's nav grid (any free floor, not only the runner), and
 * steps from/to the exact points at the ends.
 */
export function routeBetween(from: FloorPoint, to: FloorPoint): FloorPoint[] {
  const grid = getActiveNavGrid()
  let points: FloorPoint[] | null = null
  if (grid) {
    const key = `${from.x.toFixed(2)},${from.z.toFixed(2)}>${to.x.toFixed(2)},${to.z.toFixed(2)}`
    const cached = routeCache.get(key)
    if (cached) return cached
    const path = findPath(grid, from, to)
    if (path) {
      points = [from, ...path, to]
      if (routeCache.size > 300) routeCache.clear()
      routeCache.set(key, dedupe(points))
      return routeCache.get(key)!
    }
  }
  return dedupe(points ?? centerlineRoute(from, to))
}

function dedupe(points: FloorPoint[]) {
  return points.filter((point, i) => i === 0 || Math.hypot(point.x - points[i - 1].x, point.z - points[i - 1].z) > 1e-4)
}

export function routeLength(route: FloorPoint[]) {
  let total = 0
  for (let i = 1; i < route.length; i++) total += Math.hypot(route[i].x - route[i - 1].x, route[i].z - route[i - 1].z)
  return total
}

/** Point at fraction t (0..1) of the route's length, with the heading of that segment. */
export function pointAlong(route: FloorPoint[], t: number): FloorPoint & { heading: number } {
  if (route.length === 1) return { ...route[0], heading: 0 }
  let remaining = Math.max(0, Math.min(1, t)) * routeLength(route)
  for (let i = 1; i < route.length; i++) {
    const a = route[i - 1]
    const b = route[i]
    const length = Math.hypot(b.x - a.x, b.z - a.z)
    const heading = Math.atan2(b.x - a.x, b.z - a.z)
    if (remaining <= length || i === route.length - 1) {
      const k = length === 0 ? 1 : Math.min(1, remaining / length)
      return { x: a.x + (b.x - a.x) * k, z: a.z + (b.z - a.z) * k, heading }
    }
    remaining -= length
  }
  const last = route[route.length - 1]
  return { ...last, heading: 0 }
}

export const wagonAnchorLabels: Record<WagonAnchor, string> = {
  seat_1: 'Место 1', seat_2: 'Место 2', seat_3: 'Место 3', seat_4: 'Место 4',
  seat_5: 'Место 5', seat_6: 'Место 6', seat_7: 'Место 7', seat_8: 'Место 8',
  seat_9: 'Место 9', seat_10: 'Место 10', seat_11: 'Место 11', seat_12: 'Место 12',
  service_point: 'Сервисная стойка', staff_zone: 'Служебная зона',
  service_zone: 'Сервисная зона', sanitary_zone: 'Санузел', cab_entrance_boundary: 'Граница кабины',
}

/** Where each class keeps its service point (wagon_classes.json service_point_anchor). */
export function servicePointFor(classId: string): WagonAnchor {
  return classId === 'first' ? 'service_zone' : 'service_point'
}

/** Non-seat points of interest the player can walk to, per class. */
export function pointsOfInterestFor(classId: string): WagonAnchor[] {
  return classId === 'first' ? ['service_zone', 'sanitary_zone', 'cab_entrance_boundary'] : ['service_point']
}

export const wagonObjectLabels: Record<string, string> = {
  water: 'Вода',
  extinguisher_location: 'Огнетушитель',
}

/** Objects that can be inspected at an anchor (B01): galley water, rear-vestibule extinguisher. */
export const objectsAtAnchor: Partial<Record<WagonAnchor, string[]>> = {
  service_zone: ['water'],
  sanitary_zone: ['extinguisher_location'],
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
  if (!moving) return { ...fallback, progress: 1, heading: 0 }
  const from = wagonAnchorPositions[moving.from] ?? fallback
  const to = wagonAnchorPositions[moving.to] ?? fallback
  const startedAt = Date.parse(moving.started_at)
  const durationMs = Math.max(1, moving.duration_s * 1000)
  const progress = Number.isFinite(startedAt) ? Math.max(0, Math.min(1, (now - startedAt) / durationMs)) : 0
  // Walk the aisle instead of cutting through seats and partitions.
  const point = pointAlong(routeBetween(from, to), progress)
  return { x: point.x, y: from.y, z: point.z, progress, heading: point.heading }
}
