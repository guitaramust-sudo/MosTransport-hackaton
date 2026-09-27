import { badgeSvgs, type BadgeId } from '../assets/badges'
import type { IconName } from '../components/Icon'

// MVP achievements the backend actually awards (GDD §17, AS BUILT). They get
// their own medals: the BAGES artwork belongs to lessons, one badge per lesson.
export const achievements: Array<{ id: string; icon: IconName; tone: 'gold' | 'violet'; title: string; reason: string }> = [
  { id: 'first_complete', icon: 'flag', tone: 'gold', title: 'Первый рейс', reason: 'Первый зачтённый рейс в тренажёре' },
  { id: 'first_signal', icon: 'target', tone: 'violet', title: 'Внимательный взгляд', reason: 'Первый самостоятельно замеченный косвенный сигнал' },
]

/** Lesson badges share their id with the lesson (BAGES pack, GDD §29). */
export function lessonBadge(lessonId: string): BadgeId | null {
  return lessonId in badgeSvgs ? lessonId as BadgeId : null
}
