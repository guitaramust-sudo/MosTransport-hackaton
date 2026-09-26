export type AppScreen =
  | "auth"
  | "home"
  | "scenarios"
  | "simulation"
  | "debrief"
  | "profile";

export interface Player {
  id: string;
  email: string;
  username: string;
  total_xp: number;
}
export interface Tokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}
export interface AuthResult {
  player: Player;
  tokens: Tokens;
}
export interface Remark {
  code: string;
  xp: number;
  safety: number;
  loyalty: number;
  message: string;
}
export interface Situation {
  id: string;
  status: "active" | "closed";
  situation_def_id?: string;
  passenger_id?: string;
  code: string;
  name: string;
  language?: string;
  scenario: string;
  opening?: string;
  loyalty: number;
  safety: number;
  timer_deadline: string | null;
  outcome?: string;
  escalations: string[];
  xp: number;
  remarks?: Remark[];
  tone?: string;
  conveyed?: string[];
  missed?: string[];
}
export interface Session {
  id: string;
  status: "active" | "finished";
  created_at: string;
  finished_at: string | null;
}
export interface SessionResponse {
  session: Session;
  situations: Situation[];
}
export interface Message {
  id: number;
  role: "player" | "passenger" | "system";
  content: string;
  created_at: string;
}
export interface SituationResponse {
  situation: Situation;
  messages: Message[];
}
export interface TurnResult {
  reply: string;
  closed: boolean;
  status: string;
  outcome?: string;
}
export interface SituationBreakdown {
  situation_id: string;
  code: string;
  name: string;
  outcome: string;
  loyalty: number;
  safety: number;
  xp: number;
  remarks: Remark[];
}
export interface Breakdown {
  session_id: string;
  total_xp: number;
  competencies_xp: Record<string, number>;
  situations: SituationBreakdown[];
}
export interface Profile {
  player: Player;
  competencies: Array<{ competency_id: number; xp: number }>;
}

export type Priority = "critical" | "high" | "normal";

export interface ScenarioChoice {
  id: string;
  title: string;
  subtitle: string;
  safety: number;
  loyalty: number;
  competency: string;
  feedback: string;
}

export interface ScenarioEvent {
  id: string;
  time: string;
  title: string;
  location: string;
  description: string;
  priority: Priority;
  timer?: number;
  choices: ScenarioChoice[];
}

export interface Scenario {
  id: string;
  title: string;
  subtitle: string;
  duration: string;
  difficulty: string;
  progress: number;
  tag: string;
  events: ScenarioEvent[];
}

export interface ActionRecord {
  eventTitle: string;
  choiceTitle: string;
  feedback: string;
  safety: number;
  loyalty: number;
  competency: string;
}

export interface GameQuest {
  id: string
  title: string
  location: string
  priority: 'critical' | 'normal'
  seatIndex: number
}
