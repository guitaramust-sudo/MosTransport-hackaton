package domain

import (
	"encoding/json"

	"github.com/google/uuid"
)

// SimulationRun is the server-owned state of the new branching shift flow.
type SimulationRun struct {
	ID               uuid.UUID       `json:"id"`
	PlayerID         uuid.UUID       `json:"player_id"`
	ScenarioID       string          `json:"scenario_id"`
	ScenarioVersion  string          `json:"scenario_version"`
	StateVersion     int             `json:"state_version"`
	Status           string          `json:"status"`
	CurrentEventID   string          `json:"current_event_id,omitempty"`
	Flags            map[string]bool `json:"flags"`
	Loyalty          int             `json:"loyalty"`
	Safety           int             `json:"safety"`
	Path             []string        `json:"path"`
	TemplateSnapshot json.RawMessage `json:"template_snapshot"`
}
