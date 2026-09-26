import type { GameQuest } from '../types'

interface GameWorldProps {
  targetEventId: string
  targetSeatIndex: number
  moveRequest: number
  questCardsVisible: boolean
  onArrive: (questId: string) => void
  quests: GameQuest[]
  onQuestPress: (questId: string) => void
}

export function GameWorld(props: GameWorldProps): React.JSX.Element
