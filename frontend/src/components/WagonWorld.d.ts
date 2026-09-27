import type { WagonAnchor, WagonSnapshot } from '../types'

interface WagonWorldProps {
  snapshot: WagonSnapshot
  disabled?: boolean
  onAnchorPress: (anchor: WagonAnchor) => void
}

export function WagonWorld(props: WagonWorldProps): React.JSX.Element
