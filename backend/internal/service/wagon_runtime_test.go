package service

import (
	"context"
	"encoding/json"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

type fakeWagonStore struct {
	mu         sync.Mutex
	sessions   map[uuid.UUID]domain.Session
	situations map[uuid.UUID]domain.Situation
}

func newFakeWagonStore() *fakeWagonStore {
	return &fakeWagonStore{sessions: map[uuid.UUID]domain.Session{}, situations: map[uuid.UUID]domain.Situation{}}
}
func (f *fakeWagonStore) CreateWagonSession(_ context.Context, playerID uuid.UUID, state domain.WagonState) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := domain.Session{ID: uuid.New(), PlayerID: playerID, Status: domain.SessionStatusActive, WagonState: &state}
	f.sessions[s.ID] = s
	return s, nil
}
func (f *fakeWagonStore) UpdateWagonState(_ context.Context, id uuid.UUID, state domain.WagonState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return repo.ErrNotFound
	}
	s.WagonState = &state
	f.sessions[id] = s
	return nil
}
func (f *fakeWagonStore) ListActiveWagonSessions(context.Context) ([]domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Session
	for _, s := range f.sessions {
		if s.Status == domain.SessionStatusActive && s.WagonState != nil {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeWagonStore) CreateSituation(_ context.Context, s domain.Situation) (domain.Situation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s.ID = uuid.New()
	f.situations[s.ID] = s
	return s, nil
}
func (f *fakeWagonStore) GetSituation(_ context.Context, id uuid.UUID) (domain.Situation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.situations[id]
	if !ok {
		return s, repo.ErrNotFound
	}
	return s, nil
}
func (f *fakeWagonStore) ListSituationsBySession(_ context.Context, id uuid.UUID) ([]domain.Situation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Situation
	for _, s := range f.situations {
		if s.SessionID == id {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeWagonStore) SetPhysicalActionDone(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.situations[id]
	if !ok || s.Status != domain.SituationStatusActive {
		return repo.ErrConflict
	}
	s.PhysicalActionDone = true
	f.situations[id] = s
	return nil
}
func (f *fakeWagonStore) RecordRestrictedArrival(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.situations[id]
	if !ok {
		return repo.ErrNotFound
	}
	if s.PassengerParams == nil {
		s.PassengerParams = map[string]any{}
	}
	s.PassengerParams["restricted_reached"] = true
	f.situations[id] = s
	return nil
}
func (f *fakeWagonStore) GetSession(_ context.Context, id uuid.UUID) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return s, repo.ErrNotFound
	}
	return s, nil
}

func testWagonCatalog() content.Catalog {
	return content.Catalog{Scenarios: []content.Scenario{{ID: "cold", Type: content.TypeService, Criticality: content.CritLow, Opening: "O", TimeLimitSec: 90, PhysicalRequirement: &content.PhysicalRequirement{Kind: "deliver_item", Item: "blanket"}}}, Passengers: []content.Passenger{{ID: "p1", PromptHint: "H", Language: "ru"}}}
}

func TestWagonRuntimeTickSpawnsAndPersists(t *testing.T) {
	f := newFakeWagonStore()
	catalog := testWagonCatalog()
	start := time.Now().Add(-10 * time.Second)
	state := domain.WagonState{ClassID: "standard", StartedAt: start, DurationS: 100, Player: domain.WagonActor{At: "seat_1"}, Seats: []domain.WagonSeat{{Anchor: "seat_1", PassengerDefID: "p1", Actor: domain.WagonActor{At: "seat_1"}}}}
	sess, err := f.CreateWagonSession(context.Background(), uuid.New(), state)
	if err != nil {
		t.Fatal(err)
	}
	rt := &wagonRuntime{sessionID: sess.ID, store: f, catalog: catalog, cfg: content.WagonClassConfig{SessionDurationS: 100, MaxConcurrentSituations: 1, SpawnProbability: 1, PoolUnlock: map[string]float64{"easy": 0}}, state: state, rng: rand.New(rand.NewSource(1))}
	rt.tick(time.Now())
	got, err := f.GetSession(context.Background(), sess.ID)
	if err != nil || got.WagonState.Seats[0].SituationID == nil {
		t.Fatalf("spawn not persisted: %+v %v", got.WagonState, err)
	}
	sit, err := f.GetSituation(context.Background(), *got.WagonState.Seats[0].SituationID)
	if err != nil {
		t.Fatal(err)
	}
	var req content.PhysicalRequirement
	if json.Unmarshal(sit.PhysicalRequirement, &req) != nil || req.Item != "blanket" {
		t.Fatalf("physical snapshot missing: %+v", sit)
	}
}

func TestWagonRuntimeRejectsWrongItemAndRemoteDelivery(t *testing.T) {
	f := newFakeWagonStore()
	state := domain.WagonState{Player: domain.WagonActor{At: "service_point"}, CarriedItems: []string{"blanket"}, Seats: []domain.WagonSeat{{Anchor: "seat_1", Actor: domain.WagonActor{At: "seat_1"}}}}
	sess, _ := f.CreateWagonSession(context.Background(), uuid.New(), state)
	sit, _ := f.CreateSituation(context.Background(), domain.Situation{SessionID: sess.ID, Status: domain.SituationStatusActive, PhysicalRequirement: []byte(`{"kind":"deliver_item","item":"blanket"}`)})
	state.Seats[0].SituationID = &sit.ID
	rt := &wagonRuntime{sessionID: sess.ID, store: f, catalog: testWagonCatalog(), cfg: content.WagonClassConfig{ServicePointAnchor: "service_point"}, state: state}
	if err := rt.handle(time.Now(), NewWagonCommand("give_item", "water", "", sit.ID)); err == nil {
		t.Fatal("wrong item accepted")
	}
	if err := rt.handle(time.Now(), NewWagonCommand("give_item", "blanket", "", sit.ID)); err != ErrWrongAnchor {
		t.Fatalf("remote delivery accepted: %v", err)
	}
	rt.state.Player.At = "seat_1"
	if err := rt.handle(time.Now(), NewWagonCommand("give_item", "blanket", "", sit.ID)); err != nil {
		t.Fatal(err)
	}
	got, _ := f.GetSituation(context.Background(), sit.ID)
	if !got.PhysicalActionDone {
		t.Fatal("physical action not persisted")
	}
}
