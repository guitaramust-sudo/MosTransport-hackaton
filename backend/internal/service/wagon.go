package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var ErrWagonClassNotPlayable = errors.New("wagon class is not playable")

type WagonService struct {
	store   wagonStore
	catalog content.Catalog
	classes content.WagonClasses
	manager *WagonManager
}

func NewWagonService(store wagonStore, catalog content.Catalog, classes content.WagonClasses, manager *WagonManager) *WagonService {
	return &WagonService{store: store, catalog: catalog, classes: classes, manager: manager}
}

func (s *WagonService) StartSession(ctx context.Context, playerID uuid.UUID, classID string) (domain.Session, error) {
	cfg, ok := s.classes[classID]
	if !ok || cfg.Status == "coming_soon" || classID != "standard" {
		return domain.Session{}, ErrWagonClassNotPlayable
	}
	if len(s.catalog.Passengers) == 0 || len(s.catalog.Scenarios) == 0 {
		return domain.Session{}, ErrNoEligibleScenarios
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seats := make([]domain.WagonSeat, len(cfg.SeatAnchors))
	for i, anchor := range cfg.SeatAnchors {
		passenger := s.catalog.Passengers[rng.Intn(len(s.catalog.Passengers))]
		seats[i] = domain.WagonSeat{Anchor: anchor, PassengerDefID: passenger.ID, Actor: domain.WagonActor{At: anchor}}
	}
	state := domain.WagonState{ClassID: classID, RestrictedAnchors: append([]string(nil), cfg.RestrictedAnchors...), Seats: seats,
		Player: domain.WagonActor{At: cfg.ServicePointAnchor}, CarriedItems: []string{}, StartedAt: time.Now(), DurationS: cfg.SessionDurationS}
	sess, err := s.store.CreateWagonSession(ctx, playerID, state)
	if err != nil {
		return domain.Session{}, fmt.Errorf("create wagon session: %w", err)
	}
	s.manager.Start(sess.ID, cfg, state)
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
