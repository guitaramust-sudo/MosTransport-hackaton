package service

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

func TestBuildWagonSeatsRandomizesOccupancy(t *testing.T) {
	classes, err := content.LoadWagonClasses()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, classID := range []string{"standard", "first"} {
		cfg := classes[classID]
		allowed := make(map[string]bool, len(cfg.SeatAnchors))
		for _, anchor := range cfg.SeatAnchors {
			allowed[anchor] = true
		}
		layouts := map[string]bool{}
		counts := map[int]bool{}
		for seed := int64(0); seed < 100; seed++ {
			seats := buildWagonSeats(catalog, cfg, rand.New(rand.NewSource(seed)))
			if len(seats) < 5 || len(seats) > 12 {
				t.Fatalf("%s seed %d: %d passengers", classID, seed, len(seats))
			}
			counts[len(seats)] = true
			occupied := map[string]bool{}
			passengers := map[string]bool{}
			for _, seat := range seats {
				if !allowed[seat.Anchor] || occupied[seat.Anchor] || seat.Actor.At != seat.Anchor {
					t.Fatalf("%s seed %d: invalid occupied seat %+v", classID, seed, seat)
				}
				occupied[seat.Anchor] = true
				if passengers[seat.PassengerDefID] {
					t.Fatalf("%s seed %d: duplicate passenger %q", classID, seed, seat.PassengerDefID)
				}
				passengers[seat.PassengerDefID] = true
			}
			for anchor := range occupied {
				layouts[anchor] = true
			}
		}
		if len(counts) != 8 || len(layouts) != 12 {
			t.Fatalf("%s: counts=%v occupied anchors=%v", classID, counts, layouts)
		}
	}
}

func TestStartWagonSessionSeatsPassengersAndStartsRuntime(t *testing.T) {
	store := newFakeWagonStore()
	catalog := testWagonCatalog()
	classes := content.WagonClasses{"standard": {SeatAnchors: []string{"seat_1", "seat_2"}, ServicePointAnchor: "service_point", SessionDurationS: 480, TickS: 3600}, "comfort": {Status: "coming_soon"}}
	levels := content.Levels{
		{ID: "standard_level", Order: 1, ClassID: "standard", Title: "T", Intro: "I", TypeWeights: map[string]float64{string(content.TypeService): 1}},
		{ID: "comfort_level", Order: 2, ClassID: "comfort", Title: "T2", Intro: "I2", TypeWeights: map[string]float64{string(content.TypeService): 1}},
	}
	mgr := NewWagonManager(store, catalog, levels)
	svc := NewWagonService(store, catalog, classes, levels, mgr)
	playerID := uuid.New()
	sess, err := svc.StartSession(context.Background(), playerID, "standard_level")
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
	if _, err := svc.StartSession(context.Background(), playerID, "comfort_level"); !errors.Is(err, ErrWagonClassNotPlayable) {
		t.Fatalf("comfort: %v", err)
	}
	if _, err := svc.StartSession(context.Background(), playerID, "does_not_exist"); !errors.Is(err, ErrWagonLevelNotFound) {
		t.Fatalf("missing level: %v", err)
	}
}

func TestStartWagonSessionEnforcesSequentialUnlock(t *testing.T) {
	store := newFakeWagonStore()
	catalog := testWagonCatalog()
	classes := content.WagonClasses{"standard": {SeatAnchors: []string{"seat_1"}, ServicePointAnchor: "service_point", SessionDurationS: 480, TickS: 3600}}
	levels := content.Levels{
		{ID: "level_1", Order: 1, ClassID: "standard", Title: "T1", Intro: "I1", TypeWeights: map[string]float64{string(content.TypeService): 1}},
		{ID: "level_2", Order: 2, ClassID: "standard", Title: "T2", Intro: "I2", TypeWeights: map[string]float64{string(content.TypeService): 1}},
	}
	mgr := NewWagonManager(store, catalog, levels)
	svc := NewWagonService(store, catalog, classes, levels, mgr)
	playerID := uuid.New()

	if _, err := svc.StartSession(context.Background(), playerID, "level_2"); !errors.Is(err, ErrWagonLevelLocked) {
		t.Fatalf("level 2 with no progress: %v", err)
	}

	store.players[playerID] = domain.Player{ID: playerID, WagonProgress: 1}
	sess, err := svc.StartSession(context.Background(), playerID, "level_2")
	if err != nil {
		t.Fatalf("level 2 with progress 1: %v", err)
	}
	mgr.Stop(sess.ID)
}

func TestListLevelsReportsStatusByProgress(t *testing.T) {
	store := newFakeWagonStore()
	catalog := testWagonCatalog()
	classes := content.WagonClasses{"standard": {SeatAnchors: []string{"seat_1"}, ServicePointAnchor: "service_point"}}
	levels := content.Levels{
		{ID: "level_1", Order: 1, ClassID: "standard", Title: "T1", Intro: "I1", TypeWeights: map[string]float64{string(content.TypeService): 1}},
		{ID: "level_2", Order: 2, ClassID: "standard", Title: "T2", Intro: "I2", TypeWeights: map[string]float64{string(content.TypeService): 1}},
		{ID: "level_3", Order: 3, ClassID: "standard", Title: "T3", Intro: "I3", TypeWeights: map[string]float64{string(content.TypeService): 1}},
	}
	mgr := NewWagonManager(store, catalog, levels)
	svc := NewWagonService(store, catalog, classes, levels, mgr)
	playerID := uuid.New()
	store.players[playerID] = domain.Player{ID: playerID, WagonProgress: 1}

	statuses, err := svc.ListLevels(context.Background(), playerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 {
		t.Fatalf("want 3 levels, got %d", len(statuses))
	}
	want := map[string]string{"level_1": "passed", "level_2": "unlocked", "level_3": "locked"}
	for _, st := range statuses {
		if st.Status != want[st.ID] {
			t.Fatalf("level %s: status = %q, want %q", st.ID, st.Status, want[st.ID])
		}
		if st.Status == "locked" && st.Intro != "" {
			t.Fatalf("locked level %s leaked intro %q", st.ID, st.Intro)
		}
		if st.Status != "locked" && st.Intro == "" {
			t.Fatalf("unlocked/passed level %s missing intro", st.ID)
		}
	}
	// Order must be ascending.
	for i := 1; i < len(statuses); i++ {
		if statuses[i].Order <= statuses[i-1].Order {
			t.Fatalf("levels not sorted by order: %+v", statuses)
		}
	}
}

func TestAdvanceIfPassed(t *testing.T) {
	store := newFakeWagonStore()
	catalog := testWagonCatalog()
	classes := content.WagonClasses{"standard": {SeatAnchors: []string{"seat_1"}, ServicePointAnchor: "service_point"}}
	levels := content.Levels{
		{ID: "level_1", Order: 1, ClassID: "standard", Title: "T1", Intro: "I1", TypeWeights: map[string]float64{string(content.TypeService): 1}},
	}
	mgr := NewWagonManager(store, catalog, levels)
	svc := NewWagonService(store, catalog, classes, levels, mgr)

	t.Run("all passed advances progress", func(t *testing.T) {
		playerID := uuid.New()
		sessionID := uuid.New()
		okOutcome := "resolved_positive"
		store.situations[uuid.New()] = domain.Situation{SessionID: sessionID, Outcome: &okOutcome}
		session := domain.Session{ID: sessionID, PlayerID: playerID, WagonState: &domain.WagonState{LevelID: "level_1"}}
		if err := svc.AdvanceIfPassed(context.Background(), session); err != nil {
			t.Fatal(err)
		}
		if store.players[playerID].WagonProgress != 1 {
			t.Fatalf("progress = %d, want 1", store.players[playerID].WagonProgress)
		}
	})

	t.Run("a fail outcome does not advance", func(t *testing.T) {
		playerID := uuid.New()
		sessionID := uuid.New()
		failOutcome := "fail"
		store.situations[uuid.New()] = domain.Situation{SessionID: sessionID, Outcome: &failOutcome}
		session := domain.Session{ID: sessionID, PlayerID: playerID, WagonState: &domain.WagonState{LevelID: "level_1"}}
		if err := svc.AdvanceIfPassed(context.Background(), session); err != nil {
			t.Fatal(err)
		}
		if store.players[playerID].WagonProgress != 0 {
			t.Fatalf("progress = %d, want 0", store.players[playerID].WagonProgress)
		}
	})

	t.Run("non-wagon session is a no-op", func(t *testing.T) {
		playerID := uuid.New()
		session := domain.Session{ID: uuid.New(), PlayerID: playerID}
		if err := svc.AdvanceIfPassed(context.Background(), session); err != nil {
			t.Fatal(err)
		}
		if _, exists := store.players[playerID]; exists {
			t.Fatalf("non-wagon session should not touch player progress")
		}
	})
}
