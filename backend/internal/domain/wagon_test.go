package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWagonStateJSONRoundTrip(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	state := WagonState{
		ClassID: "standard", RestrictedAnchors: []string{"staff_zone"},
		StartedAt: start, DurationS: 480, CarriedItems: []string{"blanket"},
		Player: WagonActor{At: "seat_1", Moving: &WagonMove{From: "seat_1", To: "service_point", StartedAt: start, DurationS: 3}},
		Seats:  []WagonSeat{{Anchor: "seat_1", PassengerDefID: "p1", SituationID: &id, Actor: WagonActor{At: "seat_1"}}},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var got WagonState
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ClassID != state.ClassID || got.Player.Moving == nil || got.Player.Moving.To != "service_point" || len(got.Seats) != 1 || *got.Seats[0].SituationID != id || got.DurationS != 480 {
		t.Fatalf("wagon state round trip lost data: %+v", got)
	}
}
