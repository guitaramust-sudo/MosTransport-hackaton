package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

func TestStartWagonSessionSeatsPassengersAndStartsRuntime(t *testing.T) {
	store := newFakeWagonStore()
	catalog := testWagonCatalog()
	classes := content.WagonClasses{"standard": {SeatAnchors: []string{"seat_1", "seat_2"}, ServicePointAnchor: "service_point", SessionDurationS: 480, TickS: 3600}, "comfort": {Status: "coming_soon"}}
	mgr := NewWagonManager(store, catalog)
	svc := NewWagonService(store, catalog, classes, mgr)
	playerID := uuid.New()
	sess, err := svc.StartSession(context.Background(), playerID, "standard")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Stop(sess.ID)
	if sess.WagonState == nil || len(sess.WagonState.Seats) != 2 || sess.WagonState.Player.At != "service_point" {
		t.Fatalf("bad starting state: %+v", sess.WagonState)
	}
	for _, seat := range sess.WagonState.Seats {
		if seat.PassengerDefID != "p1" || seat.Actor.At != seat.Anchor {
			t.Fatalf("bad seat: %+v", seat)
		}
	}
	if mgr.runtime(sess.ID) == nil {
		t.Fatal("runtime not started")
	}
	if _, err := svc.GetOwnedSession(context.Background(), uuid.New(), sess.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("wrong owner: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), playerID, "comfort"); !errors.Is(err, ErrWagonClassNotPlayable) {
		t.Fatalf("comfort: %v", err)
	}
}
