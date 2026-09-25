package domain

import (
	"time"

	"github.com/google/uuid"
)

type Player struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	TotalXP      int       `json:"total_xp"`
	CreatedAt    time.Time `json:"created_at"`
}

type Competency struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type PlayerCompetency struct {
	PlayerID     uuid.UUID `json:"player_id"`
	CompetencyID int       `json:"competency_id"`
	XP           int       `json:"xp"`
}

const (
	SessionStatusActive   = "active"
	SessionStatusFinished = "finished"
)

type Session struct {
	ID         uuid.UUID  `json:"id"`
	PlayerID   uuid.UUID  `json:"player_id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

const (
	SituationStatusActive = "active"
	SituationStatusClosed = "closed"
)

type Situation struct {
	ID              uuid.UUID      `json:"id"`
	SessionID       uuid.UUID      `json:"session_id"`
	Status          string         `json:"status"`
	PassengerParams map[string]any `json:"passenger_params"`
	Loyalty         int            `json:"loyalty"`
	Safety          int            `json:"safety"`
	TimerDeadline   *time.Time     `json:"timer_deadline"`
	Outcome         *string        `json:"outcome"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Message struct {
	ID          int       `json:"id"`
	SituationID uuid.UUID `json:"situation_id"`
	Role        string    `json:"role"`
	Content     string    `json:"content"`
	Category    *string   `json:"category,omitempty"`
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
