package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

func TestWagonStateRoundTripsThroughSession(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	state := domain.WagonState{ClassID: "standard", RestrictedAnchors: []string{"staff_zone"}, Seats: []domain.WagonSeat{{Anchor: "seat_1", Actor: domain.WagonActor{At: "seat_1"}}}, StartedAt: time.Now(), DurationS: 480}
	sess, err := store.CreateWagonSession(ctx, playerID, state)
	if err != nil {
		t.Fatal(err)
	}
	if sess.WagonState == nil || sess.WagonState.ClassID != "standard" {
		t.Fatalf("create: %+v", sess.WagonState)
	}
	state.CarriedItems = []string{"blanket"}
	if err := store.UpdateWagonState(ctx, sess.ID, state); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.GetSession(ctx, sess.ID)
	if err != nil || reloaded.WagonState == nil || len(reloaded.WagonState.CarriedItems) != 1 {
		t.Fatalf("update: %+v, %v", reloaded.WagonState, err)
	}
	plain, err := store.CreateSession(ctx, playerID)
	if err != nil || plain.WagonState != nil {
		t.Fatalf("legacy session: %+v, %v", plain.WagonState, err)
	}
}

func TestSituationPhysicalRequirementRoundTrips(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	sess, err := store.CreateSession(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	anchor := "seat_3"
	sit := draftSituation(time.Now().Add(time.Hour))
	sit.SessionID = sess.ID
	sit.SeatAnchor = &anchor
	sit.PhysicalRequirement = []byte(`{"kind":"deliver_item","item":"blanket"}`)
	created, err := store.CreateSituation(ctx, sit)
	if err != nil {
		t.Fatal(err)
	}
	if created.SeatAnchor == nil || *created.SeatAnchor != anchor || string(created.PhysicalRequirement) == "" || created.PhysicalActionDone {
		t.Fatalf("create: %+v", created)
	}
	if err := store.SetPhysicalActionDone(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.GetSituation(ctx, created.ID)
	if err != nil || !reloaded.PhysicalActionDone {
		t.Fatalf("physical action: %+v, %v", reloaded, err)
	}
}
