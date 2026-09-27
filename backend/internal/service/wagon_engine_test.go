package service

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
)

func TestWagonMovementUsesServerClock(t *testing.T) {
	start := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	cfg := content.WagonClassConfig{Anchors: []string{"seat_1", "service_point"}, MoveDurationS: 3}
	state := domain.WagonState{Player: domain.WagonActor{At: "seat_1"}}
	moving, err := ApplyWagonMove(state, "service_point", cfg, start)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPickItem(moving, "service_point", "blanket"); !errors.Is(err, ErrAlreadyMoving) {
		t.Fatalf("picked in transit: %v", err)
	}
	before, _ := AdvanceWagonMovements(moving, start.Add(2*time.Second))
	if before.Player.At != "seat_1" {
		t.Fatalf("arrived early: %+v", before.Player)
	}
	after, _ := AdvanceWagonMovements(moving, start.Add(3*time.Second))
	if after.Player.At != "service_point" || after.Player.Moving != nil {
		t.Fatalf("did not arrive: %+v", after.Player)
	}
	if _, err := ApplyPickItem(after, "service_point", "blanket"); err != nil {
		t.Fatal(err)
	}
}

func TestWagonSpawnRespectsUnlockAndConcurrentLimit(t *testing.T) {
	cfg := content.WagonClassConfig{SessionDurationS: 100, MaxConcurrentSituations: 1, SpawnProbability: 1,
		PoolUnlock: map[string]float64{"easy": 0, "medium": 0.5, "hard": 0.9}}
	scenarios := []content.Scenario{{ID: "easy", Criticality: content.CritLow}, {ID: "medium", Criticality: content.CritHigh}, {ID: "hard", Criticality: content.CritCritical}}
	state := domain.WagonState{Seats: []domain.WagonSeat{{Anchor: "seat_1"}, {Anchor: "seat_2"}}}
	rng := rand.New(rand.NewSource(1))
	got := PickWagonSpawns(state, cfg, scenarios, 10*time.Second, rng)
	if len(got) != 1 || got[0].ScenarioID != "easy" {
		t.Fatalf("wrong spawn: %+v", got)
	}
	if got := PickWagonSpawns(state, cfg, scenarios, 101*time.Second, rng); len(got) != 0 {
		t.Fatalf("spawned after shift: %+v", got)
	}
}

func TestWagonGiveItemRequiresActualPlayerPositionAndInventory(t *testing.T) {
	id := uuid.New()
	state := domain.WagonState{Player: domain.WagonActor{At: "service_point"}, CarriedItems: []string{"blanket"},
		Seats: []domain.WagonSeat{{Anchor: "seat_1", SituationID: &id, Actor: domain.WagonActor{At: "seat_1"}}}}
	if _, err := ApplyGiveItem(state, id, "blanket"); !errors.Is(err, ErrWrongAnchor) {
		t.Fatalf("remote delivery accepted: %v", err)
	}
	state.Player.At = "seat_1"
	if _, err := ApplyGiveItem(state, id, "water"); !errors.Is(err, ErrItemNotCarried) {
		t.Fatalf("missing item accepted: %v", err)
	}
	next, err := ApplyGiveItem(state, id, "blanket")
	if err != nil || len(next.CarriedItems) != 0 {
		t.Fatalf("delivery failed: %+v, %v", next, err)
	}
}

func TestWagonRestrictedArrivalAndLateRedirect(t *testing.T) {
	id := uuid.New()
	start := time.Now()
	state := domain.WagonState{RestrictedAnchors: []string{"staff_zone"}, Player: domain.WagonActor{At: "staff_zone"},
		Seats: []domain.WagonSeat{{Anchor: "seat_1", SituationID: &id, Actor: domain.WagonActor{At: "seat_1", Moving: &domain.WagonMove{From: "seat_1", To: "staff_zone", StartedAt: start, DurationS: 20}}}}}
	arrived, events := AdvanceWagonMovements(state, start.Add(20*time.Second))
	if len(events) != 1 || !events[0].ReachedRestricted || !arrived.Seats[0].RestrictedReached {
		t.Fatalf("restriction not recorded: %+v %+v", arrived, events)
	}
	redirected, err := ApplyRedirect(arrived, id)
	if err != nil || redirected.Seats[0].Actor.At != "seat_1" || !redirected.Seats[0].RestrictedReached {
		t.Fatalf("late redirect should keep violation: %+v, %v", redirected, err)
	}
}
