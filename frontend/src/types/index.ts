export type AppScreen =
  | "auth"
  | "home"
  | "practice"
  | "progress"
  | "lesson"
  | "scenarios"
  | "simulation"
  | "live_simulation"
  | "wagon"
  | "debrief"
  | "profile"
  | "admin";

export interface Player {
  id: string;
  email: string;
  username: string;
  role: "user" | "admin";
  brigade_id?: string | null;
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
export interface CompetencyAssessment {
  competency_id: number;
  code: string;
  name: string;
  /** Accumulated XP; null while there is no evidence. */
  score: number | null;
  confidence: number;
  status: "insufficient" | "provisional" | "assessed" | string;
  evidence_count: number;
}
export type AchievementId = "first_complete" | "first_signal" | string;
export interface Profile {
  player: Player;
  level: number;
  competencies: CompetencyAssessment[];
  achievements: AchievementId[];
  leaderboard_points_total: number;
}
export type LeaderboardScope = "brigade" | "depot" | "company";
export interface ScopedLeaderboardEntry {
  rank: number;
  player_id: string;
  username: string;
  leaderboard_points_total: number;
  percentile: number;
}
export interface ScopedLeaderboard {
  group_scope: LeaderboardScope;
  group_id: string;
  group_size: number;
  entries: ScopedLeaderboardEntry[];
}
export interface ChallengeProgress {
  challenge_id: string;
  seed_variants: number;
  target: number;
  completed: boolean;
  reward_xp: number;
}
export interface AppNotification {
  id: number;
  type: string;
  subject_key: string;
  payload: unknown;
  created_at: string;
}

export interface AdminLearningSummary {
  subject: { user_id: string; display_name?: string | null; source_system?: string | null; external_user_id?: string | null; assigned_class_ids: string[] };
  total_xp: number;
  player_level: number;
  leaderboard_points: number;
  achievements: string[];
  session_outcomes: { completed_count: number; passed_count: number; recent_assessments: Array<{ session_id: string; completed_at: string; session_pass: boolean; world_safety_current: number; session_safety_score: number; loyalty: number; critical_violations: number; unresolved_commitments: number }> };
  competencies: Array<{ competency_id: number; code: string; name: string; score: number | null; confidence: number; status: string; evidence_count: number }>;
  wagon_progression: { current_progress: number; levels: Array<{ level_id: string; order: number; title: string; status: 'locked' | 'unlocked' | 'passed'; attempts: number; passed: boolean; last_attempt_at?: string }> };
  lesson_progression: { lessons: Array<{ lesson_id: string; title: string; theory_pass: boolean; practice_pass: boolean; completed: boolean; badge_id?: string; xp_earned: number; completed_at?: string }> };
  prize_balance: { balance: number; next_expiry_at?: string; shirt_threshold: number; shirt_progress: number };
  provenance: { as_of: string; scenario_version: string; scoring_rule_version: string };
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
  | 'service_zone'
  | 'sanitary_zone'
  | 'cab_entrance_boundary'

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
  class_id: 'standard' | 'first'
  level_id: string
  restricted_anchors: WagonAnchor[]
  seats: WagonSeat[]
  player: WagonActor
  carried_items: WagonItem[] | null
  started_at: string
  duration_s: number
  visited_anchors?: WagonAnchor[] | null
  inspected_objects?: string[] | null
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

export interface WagonLevel {
  id: string
  order: number
  title: string
  intro?: string
  status: 'locked' | 'unlocked' | 'passed'
}

export interface WagonLevelsResponse {
  levels: WagonLevel[]
}

export type WagonPhysicalRequirement =
  | { kind: 'deliver_item'; item: WagonItem }
  | { kind: 'redirect' }

/* ---------- Curriculum (GDD §27–§29) ---------- */

export type LessonStatus = 'locked' | 'unlocked' | 'completed'

export interface LearningLessonSummary {
  lesson_id: string
  title: string
  order: number
  status: LessonStatus
  badge_id: string
}

export interface LearningChapter {
  chapter_id: string
  title: string
  order: number
  lessons: LearningLessonSummary[]
}

export interface LearningMap {
  chapters: LearningChapter[]
}

export interface LessonQuestion {
  question_id: string
  phase: 'theory' | 'practice'
  type: string
  prompt: string
  options?: Array<{ option_id: string; text: string }>
}

export interface LessonProgress {
  lesson_id: string
  theory_pass: boolean
  practice_session_id?: string
  practice_pass: boolean
  practice_check_pass: boolean
  completed_at?: string
  content_version: string
}

export interface LessonDetail {
  lesson_id: string
  title: string
  theory_cards: string[]
  questions: LessonQuestion[]
  progress: LessonProgress
  completion_rule: 'visit_inspect' | 'scenario_result' | string
  required_anchor_ids: string[]
  required_object_ids: string[]
  estimated_min: number
  badge_id: string
}

export interface LessonAnswerResult {
  correct: boolean
  feedback: string
  phase_pass: boolean
}

export interface LessonFinalizeResult {
  completed: boolean
  missing?: Array<'theory_pass' | 'practice_pass' | 'practice_check_pass'>
  award_granted: boolean
  xp_awarded?: number
  badge_id?: string
  debrief: string
  found_anchors?: string[]
  missing_anchors?: string[]
  found_objects?: string[]
  missing_objects?: string[]
  scenario_pass?: boolean
}

export interface MyLearning {
  lessons: Array<{ lesson_id: string; status: LessonStatus; completed_at?: string }>
  prize_balance: number
  prize_next_expiry: string | null
  prize_shirt_threshold: number
  prize_shirt_progress: number
}
