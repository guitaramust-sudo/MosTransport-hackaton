package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Player struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	Username         string     `json:"username"`
	PasswordHash     string     `json:"-"`
	Role             string     `json:"role"`
	DisplayName      *string    `json:"display_name,omitempty"`
	SourceSystem     *string    `json:"source_system,omitempty"`
	ExternalUserID   *string    `json:"external_user_id,omitempty"`
	AssignedClassIDs []string   `json:"assigned_class_ids,omitempty"`
	DepotID          *string    `json:"depot_id,omitempty"`
	BrigadeID        *string    `json:"brigade_id,omitempty"`
	TotalXP          int        `json:"total_xp"`
	CreatedAt        time.Time  `json:"created_at"`
}

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type Competency struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type PlayerCompetency struct {
	PlayerID      uuid.UUID `json:"player_id"`
	CompetencyID  int       `json:"competency_id"`
	XP            int       `json:"xp"`
	EvidenceCount int       `json:"evidence_count"`
}

const (
	SessionStatusActive   = "active"
	SessionStatusFinished = "finished"
)

type Session struct {
	ID                uuid.UUID  `json:"id"`
	PlayerID          uuid.UUID  `json:"player_id"`
	Status            string     `json:"status"`
	PendingSituations []string   `json:"pending_situations,omitempty"`
	ValidationStatus  string     `json:"validation_status"`
	CreatedAt         time.Time  `json:"created_at"`
	FinishedAt        *time.Time `json:"finished_at"`
}

const (
	SituationStatusActive = "active"
	SituationStatusClosed = "closed"
)

type Situation struct {
	ID              uuid.UUID       `json:"id"`
	SessionID       uuid.UUID       `json:"session_id"`
	Status          string          `json:"status"`
	SituationDefID  *string         `json:"situation_def_id,omitempty"`
	PassengerID     *string         `json:"passenger_id,omitempty"`
	PassengerParams map[string]any  `json:"passenger_params"`
	Escalations     []string        `json:"escalations"`
	Remarks         json.RawMessage `json:"remarks,omitempty"`
	ScoreResult     json.RawMessage `json:"score_result,omitempty"`
	XP              int             `json:"xp"`
	Loyalty         int             `json:"loyalty"`
	Safety          int             `json:"safety"`
	TimerDeadline   *time.Time      `json:"timer_deadline"`
	Outcome         *string         `json:"outcome"`
	ClosedAt        *time.Time      `json:"closed_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

type Message struct {
	ID          int       `json:"id"`
	SituationID uuid.UUID `json:"situation_id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Category    *string   `json:"category,omitempty"`
	InputMode   *string   `json:"input_mode,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	MessageRolePlayer    = "player"
	MessageRolePassenger = "passenger"
	MessageRoleSystem    = "system"
)

// SituationOutcome describes how a situation resolved once closed.
type SituationOutcome struct {
	Label   string `json:"label"` // resolved_positive | resolved_neutral | resolved_negative | timeout | unfinished
	Loyalty int    `json:"loyalty"`
	Safety  int    `json:"safety"`
	XP      int    `json:"xp"`
}

const (
	ScenarioVersion    = "1.0.0"
	ScoringRuleVersion = "points-v1"

	ValidationDraft    = "draft"
	ValidationApproved = "approved"

	CompetencyInsufficient = "insufficient"
	CompetencyProvisional  = "provisional"
	CompetencyAssessed     = "assessed"
)

// CompetencyAssessment is a read-model view of a player's skill in one
// competency, used by profile and HR learning summaries.
type CompetencyAssessment struct {
	CompetencyID  int    `json:"competency_id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Score         *int   `json:"score"`      // accumulated XP; null when no evidence
	Confidence    int    `json:"confidence"` // number of contributing situations
	Status        string `json:"status"`     // insufficient | provisional | assessed
	EvidenceCount int    `json:"evidence_count"`
}
