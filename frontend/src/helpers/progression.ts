// Prototype level curve; mirrors backend levelForXP (100 XP per level).
// It is a game progress measure, not a professional admission level.
export const XP_PER_LEVEL = 100

export function levelProgress(xp: number, serverLevel?: number) {
  const safeXp = Math.max(0, xp)
  const inLevel = safeXp % XP_PER_LEVEL
  return {
    level: serverLevel ?? Math.floor(safeXp / XP_PER_LEVEL) + 1,
    inLevel,
    toNext: XP_PER_LEVEL - inLevel,
    percent: (inLevel / XP_PER_LEVEL) * 100,
  }
}
