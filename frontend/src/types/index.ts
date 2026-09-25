export type AppScreen = 'home' | 'scenarios' | 'simulation' | 'debrief' | 'profile'

export type Priority = 'critical' | 'high' | 'normal'

export interface ScenarioChoice {
  id: string
  title: string
  subtitle: string
  safety: number
  loyalty: number
  competency: string
  feedback: string
}

export interface ScenarioEvent {
  id: string
  time: string
  title: string
  location: string
  description: string
  priority: Priority
  timer?: number
  choices: ScenarioChoice[]
}

export interface Scenario {
  id: string
  title: string
  subtitle: string
  duration: string
  difficulty: 'Базовый' | 'Средний' | 'Сложный'
  progress: number
  tag: string
  events: ScenarioEvent[]
}

export interface ActionRecord {
  eventTitle: string
  choiceTitle: string
  feedback: string
  safety: number
  loyalty: number
  competency: string
}
