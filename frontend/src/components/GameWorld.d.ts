import type { ScenarioEvent } from '../types'

interface GameWorldProps {
  targetEventId: string
  moveRequest: number
  onArrive: () => void
  quests: Array<Pick<ScenarioEvent, 'id' | 'title' | 'location' | 'priority'>>
  onQuestPress: (questId: string) => void
}

export function GameWorld(props: GameWorldProps): React.JSX.Element
