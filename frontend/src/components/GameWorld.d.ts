import type { GameQuest } from '../types'

interface GameWorldProps {
  targetEventId: string
  moveRequest: number
  onArrive: (questId: string) => void
  quests: GameQuest[]
  onQuestPress: (questId: string) => void
}

export function GameWorld(props: GameWorldProps): React.JSX.Element
