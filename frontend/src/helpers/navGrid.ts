import { Box3, Mesh, type Object3D } from 'three'

export interface FloorPoint { x: number; z: number }

/**
 * Walkability grid of the wagon floor, built from the loaded model: every mesh
 * that occupies body height (seats, walls, cabinets, tables) blocks the cells
 * under it, inflated by the character's radius. Paths are found with A* and
 * then string-pulled so characters walk in straight lines where possible.
 */
export interface NavGrid {
  minX: number
  minZ: number
  cell: number
  cols: number
  rows: number
  /** 1 = blocked for a walking character (already inflated by body radius). */
  blocked: Uint8Array
}

const CELL = 0.1
const BODY_RADIUS_CELLS = 2
// Interior bounds: side walls at x≈±1.84, closed inter-car doors at z≈±8.84.
const BOUNDS = { minX: -1.8, maxX: 1.8, minZ: -8.75, maxZ: 8.75 }
// Height band a standing person occupies; floor decals and overhead racks don't block.
const BODY_MIN_Y = 0.35
const BODY_MAX_Y = 1.1
// Car body, livery and exterior trim have bounding boxes that reach into the cabin
// (GLTFLoader names use underscores instead of spaces). Interior parts such as
// VSM_First_Class_Seat stay in.
const EXTERIOR = /^VSM[ _](Left|Right|Unified|head)|livery|beltline|window[ _-]band|pier/i

export function buildNavGrid(scene: Object3D): NavGrid {
  const cols = Math.round((BOUNDS.maxX - BOUNDS.minX) / CELL)
  const rows = Math.round((BOUNDS.maxZ - BOUNDS.minZ) / CELL)
  const raw = new Uint8Array(cols * rows)
  const box = new Box3()
  scene.updateMatrixWorld(true)
  scene.traverse((object) => {
    if (!(object instanceof Mesh) || !object.visible || EXTERIOR.test(object.name)) return
    let hidden = false
    object.traverseAncestors((parent) => { if (!parent.visible) hidden = true })
    if (hidden) return
    box.setFromObject(object)
    if (box.isEmpty() || box.min.y > BODY_MAX_Y || box.max.y < BODY_MIN_Y) return
    // Whole-car shells would block everything; real obstacles are smaller.
    if (box.max.x - box.min.x > 2.5 && box.max.z - box.min.z > 2.5) return
    const c0 = Math.max(0, Math.floor((box.min.x - BOUNDS.minX) / CELL))
    const c1 = Math.min(cols - 1, Math.floor((box.max.x - BOUNDS.minX) / CELL))
    const r0 = Math.max(0, Math.floor((box.min.z - BOUNDS.minZ) / CELL))
    const r1 = Math.min(rows - 1, Math.floor((box.max.z - BOUNDS.minZ) / CELL))
    for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) raw[r * cols + c] = 1
  })
  const blocked = new Uint8Array(cols * rows)
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const edge = r < BODY_RADIUS_CELLS || c < BODY_RADIUS_CELLS || r >= rows - BODY_RADIUS_CELLS || c >= cols - BODY_RADIUS_CELLS
      let hit = edge
      for (let dr = -BODY_RADIUS_CELLS; dr <= BODY_RADIUS_CELLS && !hit; dr++) {
        for (let dc = -BODY_RADIUS_CELLS; dc <= BODY_RADIUS_CELLS && !hit; dc++) {
          if (dr * dr + dc * dc > BODY_RADIUS_CELLS * BODY_RADIUS_CELLS) continue
          const rr = r + dr
          const cc = c + dc
          if (rr >= 0 && rr < rows && cc >= 0 && cc < cols && raw[rr * cols + cc]) hit = true
        }
      }
      blocked[r * cols + c] = hit ? 1 : 0
    }
  }
  return { minX: BOUNDS.minX, minZ: BOUNDS.minZ, cell: CELL, cols, rows, blocked }
}

function cellOf(grid: NavGrid, p: FloorPoint) {
  const c = Math.max(0, Math.min(grid.cols - 1, Math.floor((p.x - grid.minX) / grid.cell)))
  const r = Math.max(0, Math.min(grid.rows - 1, Math.floor((p.z - grid.minZ) / grid.cell)))
  return { r, c }
}

function centerOf(grid: NavGrid, r: number, c: number): FloorPoint {
  return { x: grid.minX + (c + 0.5) * grid.cell, z: grid.minZ + (r + 0.5) * grid.cell }
}

const free = (grid: NavGrid, r: number, c: number) =>
  r >= 0 && c >= 0 && r < grid.rows && c < grid.cols && grid.blocked[r * grid.cols + c] === 0

/** True when a character can stand at p. */
export function isWalkable(grid: NavGrid, p: FloorPoint) {
  const { r, c } = cellOf(grid, p)
  return free(grid, r, c)
}

/** Closest walkable point to p (breadth-first over the grid). */
export function nearestFree(grid: NavGrid, p: FloorPoint): FloorPoint {
  const start = cellOf(grid, p)
  if (free(grid, start.r, start.c)) return p
  const seen = new Uint8Array(grid.rows * grid.cols)
  const queue = [start]
  seen[start.r * grid.cols + start.c] = 1
  for (let i = 0; i < queue.length; i++) {
    const { r, c } = queue[i]
    if (free(grid, r, c)) return centerOf(grid, r, c)
    for (const [dr, dc] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
      const rr = r + dr
      const cc = c + dc
      if (rr < 0 || cc < 0 || rr >= grid.rows || cc >= grid.cols || seen[rr * grid.cols + cc]) continue
      seen[rr * grid.cols + cc] = 1
      queue.push({ r: rr, c: cc })
    }
  }
  return p
}

/** True when a straight walk between a and b never touches a blocked cell. */
function clearLine(grid: NavGrid, a: FloorPoint, b: FloorPoint) {
  const steps = Math.ceil(Math.hypot(b.x - a.x, b.z - a.z) / (grid.cell * 0.5))
  for (let i = 0; i <= steps; i++) {
    const t = steps === 0 ? 0 : i / steps
    const { r, c } = cellOf(grid, { x: a.x + (b.x - a.x) * t, z: a.z + (b.z - a.z) * t })
    if (!free(grid, r, c)) return false
  }
  return true
}

/** Shortest walkable path from a to b (both snapped to walkable space), or null. */
export function findPath(grid: NavGrid, a: FloorPoint, b: FloorPoint): FloorPoint[] | null {
  const from = nearestFree(grid, a)
  const to = nearestFree(grid, b)
  if (clearLine(grid, from, to)) return [from, to]
  const s = cellOf(grid, from)
  const t = cellOf(grid, to)
  const n = grid.rows * grid.cols
  const g = new Float32Array(n).fill(Infinity)
  const parent = new Int32Array(n).fill(-1)
  const closed = new Uint8Array(n)
  const open: number[] = []
  const f = new Float32Array(n).fill(Infinity)
  const h = (r: number, c: number) => {
    const dr = Math.abs(r - t.r)
    const dc = Math.abs(c - t.c)
    return (dr + dc) + (Math.SQRT2 - 2) * Math.min(dr, dc)
  }
  const startIndex = s.r * grid.cols + s.c
  const goalIndex = t.r * grid.cols + t.c
  g[startIndex] = 0
  f[startIndex] = h(s.r, s.c)
  open.push(startIndex)
  while (open.length) {
    // Grid is small (~6k cells): a linear scan beats a heap here in practice.
    let best = 0
    for (let i = 1; i < open.length; i++) if (f[open[i]] < f[open[best]]) best = i
    const current = open.splice(best, 1)[0]
    if (current === goalIndex) break
    closed[current] = 1
    const r = Math.floor(current / grid.cols)
    const c = current % grid.cols
    for (let dr = -1; dr <= 1; dr++) {
      for (let dc = -1; dc <= 1; dc++) {
        if (!dr && !dc) continue
        const rr = r + dr
        const cc = c + dc
        if (!free(grid, rr, cc)) continue
        // No corner cutting through blocked cells.
        if (dr && dc && (!free(grid, r + dr, c) || !free(grid, r, c + dc))) continue
        const next = rr * grid.cols + cc
        if (closed[next]) continue
        const cost = g[current] + (dr && dc ? Math.SQRT2 : 1)
        if (cost >= g[next]) continue
        g[next] = cost
        f[next] = cost + h(rr, cc)
        parent[next] = current
        if (!open.includes(next)) open.push(next)
      }
    }
  }
  if (parent[goalIndex] === -1 && goalIndex !== startIndex) return null
  const cells: FloorPoint[] = []
  for (let i = goalIndex; i !== -1; i = parent[i]) cells.push(centerOf(grid, Math.floor(i / grid.cols), i % grid.cols))
  cells.reverse()
  cells[0] = from
  cells[cells.length - 1] = to
  // String-pull: keep only the corners a straight walk can't skip.
  const path = [cells[0]]
  let anchor = 0
  for (let i = 2; i < cells.length; i++) {
    if (!clearLine(grid, cells[anchor], cells[i])) {
      path.push(cells[i - 1])
      anchor = i - 1
    }
  }
  path.push(cells[cells.length - 1])
  return path
}

let activeGrid: NavGrid | null = null
export function setActiveNavGrid(grid: NavGrid | null) { activeGrid = grid }
export function getActiveNavGrid() { return activeGrid }
