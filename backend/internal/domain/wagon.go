package domain

import (
	"time"

	"github.com/google/uuid"
)

// WagonState stores symbolic positions and timing for a live wagon shift.
type WagonState struct {
	ClassID           string      `json:"class_id"`
	LevelID           string      `json:"level_id"`
	RestrictedAnchors []string    `json:"restricted_anchors"`
	Seats             []WagonSeat `json:"seats"`
	Player            WagonActor  `json:"player"`
	CarriedItems      []string    `json:"carried_items"`
	StartedAt         time.Time   `json:"started_at"`
	DurationS         int         `json:"duration_s"`
}

type WagonSeat struct {
	Anchor            string     `json:"anchor"`
	PassengerDefID    string     `json:"passenger_def_id"`
	SituationID       *uuid.UUID `json:"situation_id,omitempty"`
	SituationDefID    *string    `json:"situation_def_id,omitempty"`
	RestrictedReached bool       `json:"restricted_reached,omitempty"`
	Actor             WagonActor `json:"actor"`
}

type WagonActor struct {
	At     string     `json:"at"`
	Moving *WagonMove `json:"moving,omitempty"`
}

type WagonMove struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	StartedAt time.Time `json:"started_at"`
	DurationS int       `json:"duration_s"`
}
