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

func TestWagonSpawnRespectsConcurrentLimitAndSessionWindow(t *testing.T) {
	// Pool/unlock-threshold gating (previously WagonClassConfig.SituationPoolIDs
	// / PoolUnlock) was removed with the move to content.Levels; this test now
	// covers what PickWagonSpawns still owns: the concurrency cap and the
	// session-duration cutoff.
	cfg := content.WagonClassConfig{SessionDurationS: 100, MaxConcurrentSituations: 1, SpawnProbability: 1}
	level := content.Level{TypeWeights: map[string]float64{
		string(content.TypeService): 1, string(content.TypeConflict): 1, string(content.TypeSafety): 1,
	}}
	scenarios := []content.Scenario{
		{ID: "easy", Type: content.TypeService, Criticality: content.CritLow},
		{ID: "medium", Type: content.TypeConflict, Criticality: content.CritHigh},
		{ID: "hard", Type: content.TypeSafety, Criticality: content.CritCritical},
	}
	state := domain.WagonState{Seats: []domain.WagonSeat{{Anchor: "seat_1"}, {Anchor: "seat_2"}}}
	rng := rand.New(rand.NewSource(1))
	got := PickWagonSpawns(state, cfg, level, scenarios, 10*time.Second, rng)
	if len(got) != 1 {
		t.Fatalf("expected exactly one spawn under the concurrency cap: %+v", got)
	}
	if got := PickWagonSpawns(state, cfg, level, scenarios, 101*time.Second, rng); len(got) != 0 {
		t.Fatalf("spawned after shift: %+v", got)
	}
}

// TestWagonSpawnOnlyChoosesWeightedTypes exercises the core behavior added by
// this task: PickWagonSpawns samples scenario Type weighted by
// level.TypeWeights, and never selects a scenario whose Type has zero (or
// missing) weight, across many seeded draws.
func TestWagonSpawnOnlyChoosesWeightedTypes(t *testing.T) {
	cfg := content.WagonClassConfig{SessionDurationS: 100, MaxConcurrentSituations: 1, SpawnProbability: 1}
	level := content.Level{TypeWeights: map[string]float64{
		string(content.TypeService): 3,
		string(content.TypeSafety):  1,
		// conflict and medical are intentionally absent/zero-weight.
	}}
	scenarios := []content.Scenario{
		{ID: "svc-1", Type: content.TypeService},
		{ID: "svc-2", Type: content.TypeService},
		{ID: "safety-1", Type: content.TypeSafety},
		{ID: "conflict-1", Type: content.TypeConflict},
		{ID: "medical-1", Type: content.TypeMedical},
	}
	byID := map[string]content.Scenario{}
	for _, s := range scenarios {
		byID[s.ID] = s
	}
	seenTypes := map[content.ScenarioType]bool{}
	for seed := int64(0); seed < 200; seed++ {
		state := domain.WagonState{Seats: []domain.WagonSeat{{Anchor: "seat_1"}}}
		rng := rand.New(rand.NewSource(seed))
		for _, decision := range PickWagonSpawns(state, cfg, level, scenarios, 0, rng) {
			chosen, ok := byID[decision.ScenarioID]
			if !ok {
				t.Fatalf("unknown scenario id: %q", decision.ScenarioID)
			}
			seenTypes[chosen.Type] = true
			if level.TypeWeights[string(chosen.Type)] <= 0 {
				t.Fatalf("spawned scenario %q of zero-weight type %q (seed %d)", chosen.ID, chosen.Type, seed)
			}
		}
	}
	if !seenTypes[content.TypeService] || !seenTypes[content.TypeSafety] {
		t.Fatalf("expected both weighted types to be drawn across seeds, got %+v", seenTypes)
	}
	if seenTypes[content.TypeConflict] || seenTypes[content.TypeMedical] {
		t.Fatalf("zero-weight types must never be drawn, got %+v", seenTypes)
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
