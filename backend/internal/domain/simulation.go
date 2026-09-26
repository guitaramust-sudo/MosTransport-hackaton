package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SimulationRun is the server-owned state of the new branching shift flow.
type SimulationRun struct {
	ID               uuid.UUID            `json:"id"`
	PlayerID         uuid.UUID            `json:"player_id"`
	ScenarioID       string               `json:"scenario_id"`
	ScenarioVersion  string               `json:"scenario_version"`
	StateVersion     int                  `json:"state_version"`
	Status           string               `json:"status"`
	CurrentEventID   string               `json:"current_event_id,omitempty"`
	ActiveEventIDs   []string             `json:"active_event_ids"`
	ObservedEvents   map[string]bool      `json:"observed_events"`
	Location         string               `json:"location"`
	GameTimeS        int                  `json:"game_time_s"`
	StartedAt        time.Time            `json:"started_at"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	DeadlineAt       *time.Time           `json:"deadline_at,omitempty"`
	TimedOut         bool                 `json:"timed_out"`
	ActionLog        []SimulationLogEntry `json:"action_log"`
	Flags            map[string]bool      `json:"flags"`
	Loyalty          int                  `json:"loyalty"`
	Safety           int                  `json:"safety"`
	Path             []string             `json:"path"`
	TemplateSnapshot json.RawMessage      `json:"template_snapshot"`
}

type SimulationLogEntry struct {
	CommandID     uuid.UUID `json:"command_id,omitempty"`
	EventID       string    `json:"event_id"`
	ActionID      string    `json:"action_id"`
	EffectID      string    `json:"effect_id"`
	Explanation   string    `json:"explanation"`
	AtGameTimeS   int       `json:"at_game_time_s"`
	LoyaltyBefore int       `json:"loyalty_before"`
	LoyaltyAfter  int       `json:"loyalty_after"`
	SafetyBefore  int       `json:"safety_before"`
	SafetyAfter   int       `json:"safety_after"`
}

type SimulationDueRun struct {
	ID       uuid.UUID
	PlayerID uuid.UUID
}
