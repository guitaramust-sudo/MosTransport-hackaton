package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var (
	ErrWagonClassNotPlayable = errors.New("wagon class is not playable")
	ErrWagonLevelNotFound    = errors.New("wagon level not found")
	ErrWagonLevelLocked      = errors.New("wagon level is locked")
)

type WagonService struct {
	store   wagonStore
	catalog content.Catalog
	classes content.WagonClasses
	levels  content.Levels
	manager *WagonManager
}

func NewWagonService(store wagonStore, catalog content.Catalog, classes content.WagonClasses, levels content.Levels, manager *WagonManager) *WagonService {
	return &WagonService{store: store, catalog: catalog, classes: classes, levels: levels, manager: manager}
}

// buildWagonSeats chooses occupied seats and passengers for each new session.
// A class without a passenger range uses every configured seat. Shared by
// WagonService.StartSession (random level starts) and
// LearningService.StartPractice (lesson-driven starts) so this loop lives
// in exactly one place.
func buildWagonSeats(catalog content.Catalog, cfg content.WagonClassConfig, rng *rand.Rand) []domain.WagonSeat {
	count := len(cfg.SeatAnchors)
	if cfg.MinPassengers > 0 {
		count = cfg.MinPassengers + rng.Intn(cfg.MaxPassengers-cfg.MinPassengers+1)
	}
	seats := make([]domain.WagonSeat, count)
	anchors := rng.Perm(len(cfg.SeatAnchors))
	passengers := rng.Perm(len(catalog.Passengers))
	for i := range seats {
		anchor := cfg.SeatAnchors[anchors[i]]
		passengerIndex := passengers[i%len(passengers)]
		passenger := catalog.Passengers[passengerIndex]
		seats[i] = domain.WagonSeat{Anchor: anchor, PassengerDefID: passenger.ID, Actor: domain.WagonActor{At: anchor}}
	}
	return seats
}

func (s *WagonService) StartSession(ctx context.Context, playerID uuid.UUID, levelID string) (domain.Session, error) {
	level, ok := levelByID(s.levels, levelID)
	if !ok {
		return domain.Session{}, ErrWagonLevelNotFound
	}
	cfg, ok := s.classes[level.ClassID]
	if !ok || cfg.Status == "coming_soon" {
		return domain.Session{}, ErrWagonClassNotPlayable
	}
	player, err := s.store.GetPlayerByID(ctx, playerID)
	if err != nil {
		return domain.Session{}, err
	}
	if level.Order > player.WagonProgress+1 {
		return domain.Session{}, ErrWagonLevelLocked
	}
	if len(s.catalog.Passengers) == 0 || len(s.catalog.Scenarios) == 0 {
		return domain.Session{}, ErrNoEligibleScenarios
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seats := buildWagonSeats(s.catalog, cfg, rng)
	state := domain.WagonState{ClassID: level.ClassID, LevelID: level.ID, RestrictedAnchors: append([]string(nil), cfg.RestrictedAnchors...), Seats: seats,
		Player: domain.WagonActor{At: cfg.ServicePointAnchor}, CarriedItems: []string{}, StartedAt: time.Now(), DurationS: cfg.SessionDurationS}
	sess, err := s.store.CreateWagonSession(ctx, playerID, state)
	if err != nil {
		return domain.Session{}, fmt.Errorf("create wagon session: %w", err)
	}
	s.manager.Start(sess.ID, cfg, level, state)
	return sess, nil
}

func (s *WagonService) GetOwnedSession(ctx context.Context, playerID, sessionID uuid.UUID) (domain.Session, error) {
	sess, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return domain.Session{}, err
	}
	if sess.PlayerID != playerID || sess.WagonState == nil {
		return domain.Session{}, repo.ErrNotFound
	}
	return sess, nil
}

// LevelStatus reports one level's unlock status for a specific player.
type LevelStatus struct {
	ID     string `json:"id"`
	Order  int    `json:"order"`
	Title  string `json:"title"`
	Intro  string `json:"intro,omitempty"`
	Status string `json:"status"` // "locked" | "unlocked" | "passed"
}

// ListLevels reports every level's unlock status for playerID, in Order.
// Intro is omitted for locked levels so the client doesn't get spoiled
// content it can't play yet.
func (s *WagonService) ListLevels(ctx context.Context, playerID uuid.UUID) ([]LevelStatus, error) {
	player, err := s.store.GetPlayerByID(ctx, playerID)
	if err != nil {
		return nil, err
	}
	out := make([]LevelStatus, 0, len(s.levels))
	for _, lvl := range s.levels {
		status := "locked"
		switch {
		case lvl.Order <= player.WagonProgress:
			status = "passed"
		case lvl.Order == player.WagonProgress+1:
			status = "unlocked"
		}
		entry := LevelStatus{ID: lvl.ID, Order: lvl.Order, Status: status, Title: lvl.Title}
		if status != "locked" {
			entry.Intro = lvl.Intro
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, nil
}

// AdvanceIfPassed checks whether the given finished session was a wagon
// session for a level, and if every situation in it resolved without a
// "fail" outcome, advances the player's wagon_progress (a no-op if it
// wasn't the next sequential level, or if any situation failed, or if this
// wasn't a wagon session at all).
func (s *WagonService) AdvanceIfPassed(ctx context.Context, session domain.Session) error {
	if session.WagonState == nil || session.WagonState.LevelID == "" {
		return nil
	}
	level, ok := levelByID(s.levels, session.WagonState.LevelID)
	if !ok {
		return nil
	}
	situations, err := s.store.ListSituationsBySession(ctx, session.ID)
	if err != nil {
		return err
	}
	for _, sit := range situations {
		if sit.Outcome != nil && *sit.Outcome == "fail" {
			return nil
		}
	}
	_, err = s.store.AdvanceWagonProgress(ctx, session.PlayerID, level.Order)
	return err
}
