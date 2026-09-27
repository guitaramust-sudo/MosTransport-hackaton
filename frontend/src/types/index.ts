export type AppScreen =
  | "auth"
  | "home"
  | "scenarios"
  | "simulation"
  | "live_simulation"
  | "wagon_lobby"
  | "wagon"
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
  seat_anchor?: WagonAnchor;
  physical_requirement?: WagonPhysicalRequirement | null;
  physical_action_done?: boolean;
}
export interface Session {
  id: string;
  status: "active" | "finished";
  created_at: string;
  finished_at: string | null;
  wagon_state?: WagonState | null;
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

export interface LivePassenger {
  name: string
  temperament: string
  tension: number
  request: string
  opening: string
}

export interface LiveDialogueTurn {
  command_id: string
  event_id: string
  player: string
  passenger: string
  choice_id?: string
  at_game_time_s: number
}

export interface LiveEvent {
  id: string
  text: string
  location: string
  choices: Array<{ id: string; text: string }>
}

export interface LiveSimulation {
  run: {
    id: string
    scenario_id: string
    content_validation_status: string
    state_version: number
    status: 'active' | 'finished'
    loyalty: number
    safety: number
    location: string
    game_time_s: number
    seed_variant: string
    deadline_at?: string
    timed_out: boolean
  }
  event?: LiveEvent
  events: LiveEvent[]
  observable_cues: string[]
  passenger: LivePassenger
  dialogue: LiveDialogueTurn[]
}

export interface LiveDebriefEntry {
  event_id: string
  action_id: string
  effect_id: string
  explanation: string
  player_text?: string
  passenger_reply?: string
  better_options: string[]
}

export interface LiveResult {
  session_id: string
  session_pass: boolean
  session_safety_score: number
  loyalty: number
  timed_out: boolean
  validation_status: string
  leaderboard_points_delta: number
  leaderboard_points_total: number
  debrief: LiveDebriefEntry[]
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

export type WagonClassId = 'standard' | 'comfort' | 'business' | 'first'
export type WagonClassStatus = 'available' | 'coming_soon'
export type WagonAnchor =
  | 'seat_1'
  | 'seat_2'
  | 'seat_3'
  | 'seat_4'
  | 'seat_5'
  | 'seat_6'
  | 'service_point'
  | 'staff_zone'

export type WagonItem = 'blanket' | 'water' | 'coffee'
export type WagonSituationType = 'cold' | 'thirsty' | 'tired' | 'zone_intrusion' | string

export interface WagonMove {
  from: WagonAnchor
  to: WagonAnchor
  started_at: string
  duration_s: number
}

export interface WagonActor {
  at: WagonAnchor
  moving?: WagonMove | null
}

export interface WagonSeat {
  anchor: WagonAnchor
  passenger_def_id: string
  situation_id?: string | null
  situation_def_id?: string | null
  restricted_reached?: boolean
  actor: WagonActor
}

export interface WagonState {
  class_id: 'standard'
  restricted_anchors: WagonAnchor[]
  seats: WagonSeat[]
  player: WagonActor
  carried_items: WagonItem[] | null
  started_at: string
  duration_s: number
}

export interface WagonActiveSituation {
  situation_id: string
  seat_anchor: WagonAnchor
  type: WagonSituationType
  pool: string
}

export interface WagonSnapshot {
  type: 'state'
  game_time_s: number
  wagon_state: WagonState
  active_situations: WagonActiveSituation[]
}

export interface WagonError {
  type: 'error'
  message: string
}

export interface WagonStartResponse {
  session_id: string
  ws_path: string
}

export interface WagonClassesResponse {
  classes: Record<WagonClassId, WagonClassStatus>
}

export type WagonPhysicalRequirement =
  | { kind: 'deliver_item'; item: WagonItem }
  | { kind: 'redirect' }
