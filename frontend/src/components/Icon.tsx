import type { ReactNode } from 'react'
import Svg, { Circle, Path, Rect } from 'react-native-svg'
import { colors } from '../helpers/theme'

// Outline icon set: 24px grid, single ~1.8px stroke, simple geometry (§11).
export type IconName =
  | 'home' | 'practice' | 'progress' | 'profile' | 'settings' | 'chevronRight' | 'chevronLeft'
  | 'close' | 'lock' | 'check' | 'star' | 'clock' | 'train' | 'route' | 'bell' | 'trophy'
  | 'flag' | 'logout' | 'shield' | 'chat' | 'play' | 'refresh' | 'target' | 'users' | 'help' | 'admin'

const paths: Record<IconName, (color: string) => ReactNode> = {
  home: (c) => <><Path d="M3.5 10.5 12 4l8.5 6.5" stroke={c} /><Path d="M5.5 9v10a1 1 0 0 0 1 1H10v-5.5h4V20h3.5a1 1 0 0 0 1-1V9" stroke={c} /></>,
  practice: (c) => <><Rect x="5" y="3.5" width="14" height="13" rx="3.5" stroke={c} /><Path d="M5 11h14M9 20.5l1.5-4M15 20.5l-1.5-4" stroke={c} /><Circle cx="9" cy="13.8" r=".6" fill={c} stroke={c} /><Circle cx="15" cy="13.8" r=".6" fill={c} stroke={c} /></>,
  progress: (c) => <><Path d="M4 20h16" stroke={c} /><Rect x="5.5" y="12" width="3" height="5.5" rx="1" stroke={c} /><Rect x="10.5" y="8" width="3" height="9.5" rx="1" stroke={c} /><Rect x="15.5" y="4.5" width="3" height="13" rx="1" stroke={c} /></>,
  profile: (c) => <><Circle cx="12" cy="8.5" r="4" stroke={c} /><Path d="M4.5 20c1.2-3.6 4-5.5 7.5-5.5s6.3 1.9 7.5 5.5" stroke={c} /></>,
  settings: (c) => <><Circle cx="12" cy="12" r="3" stroke={c} /><Path d="M12 3.5v2.2M12 18.3v2.2M20.5 12h-2.2M5.7 12H3.5M18 6l-1.6 1.6M7.6 16.4 6 18M18 18l-1.6-1.6M7.6 7.6 6 6" stroke={c} /></>,
  chevronRight: (c) => <Path d="m9.5 6 6 6-6 6" stroke={c} />,
  chevronLeft: (c) => <Path d="m14.5 6-6 6 6 6" stroke={c} />,
  close: (c) => <Path d="M6.5 6.5l11 11M17.5 6.5l-11 11" stroke={c} />,
  lock: (c) => <><Rect x="5.5" y="10.5" width="13" height="9.5" rx="2.5" stroke={c} /><Path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5" stroke={c} /></>,
  check: (c) => <Path d="m5.5 12.5 4 4 9-9.5" stroke={c} />,
  star: (c) => <Path d="m12 4 2.4 5 5.4.6-4 3.7 1.1 5.3L12 16l-4.9 2.6 1.1-5.3-4-3.7 5.4-.6L12 4Z" stroke={c} />,
  clock: (c) => <><Circle cx="12" cy="12" r="8" stroke={c} /><Path d="M12 7.5V12l3 2" stroke={c} /></>,
  train: (c) => <><Path d="M7 3.5h10a3 3 0 0 1 3 3V15a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3V6.5a3 3 0 0 1 3-3Z" stroke={c} /><Path d="M4 11h16M8 21l1.5-3M16 21l-1.5-3" stroke={c} /></>,
  route: (c) => <><Circle cx="6" cy="18" r="2" stroke={c} /><Circle cx="18" cy="6" r="2" stroke={c} /><Path d="M8 18h7.5a3.5 3.5 0 0 0 0-7h-7a3.5 3.5 0 0 1 0-7H16" stroke={c} /></>,
  bell: (c) => <><Path d="M6 16.5V11a6 6 0 0 1 12 0v5.5l1.5 1.5h-15L6 16.5Z" stroke={c} /><Path d="M10 20.5a2 2 0 0 0 4 0" stroke={c} /></>,
  trophy: (c) => <><Path d="M8 4h8v5a4 4 0 0 1-8 0V4Z" stroke={c} /><Path d="M8 6H5v1.5A3 3 0 0 0 8 10.5M16 6h3v1.5a3 3 0 0 1-3 3M12 13v4M8.5 20h7M9.5 17h5" stroke={c} /></>,
  flag: (c) => <Path d="M6 21V4M6 4.5h10l-2 3.5 2 3.5H6" stroke={c} />,
  logout: (c) => <><Path d="M10 4.5H6a1.5 1.5 0 0 0-1.5 1.5v12A1.5 1.5 0 0 0 6 19.5h4" stroke={c} /><Path d="M14 8l4 4-4 4M18 12H9" stroke={c} /></>,
  shield: (c) => <Path d="M12 3.5 19 6v5.5c0 4.3-3 7.6-7 9-4-1.4-7-4.7-7-9V6l7-2.5Z" stroke={c} />,
  chat: (c) => <Path d="M5 5.5h14a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5h-8L7 20.5v-3H5A1.5 1.5 0 0 1 3.5 16V7A1.5 1.5 0 0 1 5 5.5Z" stroke={c} />,
  play: (c) => <Path d="M8 5.5v13l10-6.5-10-6.5Z" stroke={c} />,
  refresh: (c) => <><Path d="M19.5 12a7.5 7.5 0 1 1-2.2-5.3" stroke={c} /><Path d="M19.5 4.5v4h-4" stroke={c} /></>,
  target: (c) => <><Circle cx="12" cy="12" r="8" stroke={c} /><Circle cx="12" cy="12" r="4" stroke={c} /><Circle cx="12" cy="12" r=".8" fill={c} stroke={c} /></>,
  users: (c) => <><Circle cx="9" cy="8.5" r="3.5" stroke={c} /><Path d="M3 19.5c.8-3 3.1-4.5 6-4.5s5.2 1.5 6 4.5M16 5.2a3.5 3.5 0 0 1 0 6.6M17.5 15.2c1.8.5 3 1.9 3.5 4.3" stroke={c} /></>,
  help: (c) => <><Circle cx="12" cy="12" r="8.5" stroke={c} /><Path d="M9.7 9.5a2.4 2.4 0 1 1 3.4 2.2c-.7.3-1.1.9-1.1 1.6v.4" stroke={c} /><Circle cx="12" cy="16.8" r=".6" fill={c} stroke={c} /></>,
  admin: (c) => <><Path d="M12 3.5 19 6v5.5c0 4.3-3 7.6-7 9-4-1.4-7-4.7-7-9V6l7-2.5Z" stroke={c} /><Path d="m9 12 2 2 4-4" stroke={c} /></>,
}

export function Icon({ name, size = 24, color = colors.secondary, strokeWidth = 1.8 }: {
  name: IconName
  size?: number
  color?: string
  strokeWidth?: number
}) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none" strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round">
      {paths[name](color)}
    </Svg>
  )
}
