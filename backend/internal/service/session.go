package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionService struct {
	store         repo.Store
	catalog       content.Catalog
	situationsNum int
	situations    *SituationService
}

func NewSessionService(store repo.Store, catalog content.Catalog, situationsNum int, situations *SituationService) *SessionService {
	return &SessionService{store: store, catalog: catalog, situationsNum: situationsNum, situations: situations}
}

func (s *SessionService) Start(ctx context.Context, playerID uuid.UUID) (*domain.Session, []domain.Situation, error) {
	if err := s.catalog.Validate(); err != nil {
		return nil, nil, err
	}
	sess, err := s.store.CreateSession(ctx, playerID)
	if err != nil {
		return nil, nil, err
	}

	selected := pickScenarios(s.catalog.Scenarios, s.situationsNum)
	situations := make([]domain.Situation, 0, len(selected))
	for _, scenario := range selected {
		passenger := s.catalog.Passengers[rand.Intn(len(s.catalog.Passengers))]
		deadline := time.Now().Add(time.Duration(scenario.TimeLimitSec) * time.Second)
		scenarioID, passengerID := scenario.ID, passenger.ID
		params := map[string]any{
			"code":             scenario.ID,
			"name":             passenger.ID,
			"persona":          passenger.PromptHint,
			"scenario":         scenario.Title,
			"opening":          scenario.Opening,
			"situation_def_id": scenario.ID,
			"passenger_id":     passenger.ID,
			"prompt_hint":      passenger.PromptHint,
			"language":         passenger.Language,
			"traits":           passenger.Traits,
			"traits_text":      strings.Join(passenger.Traits, ", "),
		}
		sit, err := s.store.CreateSituation(ctx, domain.Situation{
			SessionID:       sess.ID,
			Status:          domain.SituationStatusActive,
			SituationDefID:  &scenarioID,
			PassengerID:     &passengerID,
			PassengerParams: params,
			Loyalty:         50,
			Safety:          50,
			TimerDeadline:   &deadline,
		})
		if err != nil {
			return nil, nil, err
		}
		if _, err := s.store.CreateMessage(ctx, sit.ID, domain.MessageRoleSystem, scenario.Opening, nil); err != nil {
			return nil, nil, err
		}
		situations = append(situations, sit)
	}

	return &sess, situations, nil
}

func pickScenarios(scenarios []content.Scenario, n int) []content.Scenario {
	pool := append([]content.Scenario(nil), scenarios...)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	if n < 1 {
		n = 1
	}
	return pool[:n]
}

func (s *SessionService) Get(ctx context.Context, playerID, sessionID uuid.UUID) (*domain.Session, []domain.Situation, error) {
	sess, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil, ErrSessionNotFound
		}
		return nil, nil, err
	}
	if sess.PlayerID != playerID {
		return nil, nil, ErrSessionNotFound
	}
	situations, err := s.store.ListSituationsBySession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return &sess, situations, nil
}

// Breakdown is the post-shift debrief returned to the player.
type Breakdown struct {
	SessionID    uuid.UUID            `json:"session_id"`
	TotalXP      int                  `json:"total_xp"`
	Competencies map[string]int       `json:"competencies_xp"` // code -> xp gained
	Situations   []SituationBreakdown `json:"situations"`
}

type SituationBreakdown struct {
	SituationID uuid.UUID       `json:"situation_id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Outcome     string          `json:"outcome"`
	Loyalty     int             `json:"loyalty"`
	Safety      int             `json:"safety"`
	XP          int             `json:"xp"`
	Remarks     json.RawMessage `json:"remarks,omitempty"`
	ScoreResult json.RawMessage `json:"score_result,omitempty"`
}

// Finish closes any remaining situations, computes the debrief and awards XP.
// It is idempotent: XP is only awarded on the active→finished transition.
func (s *SessionService) Finish(ctx context.Context, playerID, sessionID uuid.UUID) (*Breakdown, error) {
	sess, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	if sess.PlayerID != playerID {
		return nil, ErrSessionNotFound
	}

	situations, err := s.store.ListSituationsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	wasActive := sess.Status == domain.SessionStatusActive
	if wasActive {
		for i := range situations {
			if situations[i].Status == domain.SituationStatusActive {
				if _, err := s.situations.finishLoaded(ctx, situations[i], time.Now()); err != nil {
					return nil, err
				}
			}
		}
		situations, err = s.store.ListSituationsBySession(ctx, sessionID)
		if err != nil {
			return nil, err
		}
	}

	breakdown := &Breakdown{
		SessionID:    sessionID,
		Competencies: map[string]int{},
	}

	for _, sit := range situations {
		sb := SituationBreakdown{
			SituationID: sit.ID,
			Loyalty:     sit.Loyalty,
			Safety:      sit.Safety,
			Outcome:     "unfinished",
		}
		if p := sit.PassengerParams; p != nil {
			sb.Code, _ = p["code"].(string)
			sb.Name, _ = p["name"].(string)
		}
		if sit.Outcome != nil {
			sb.Outcome = *sit.Outcome
		}
		sb.XP = sit.XP
		sb.Remarks = sit.Remarks
		sb.ScoreResult = sit.ScoreResult
		breakdown.TotalXP += sb.XP
		breakdown.Situations = append(breakdown.Situations, sb)
	}

	if wasActive {
		if _, err := s.store.FinishSessionAndAwardXP(ctx, sessionID, playerID, breakdown.TotalXP, time.Now()); err != nil {
			return nil, err
		}
	}

	return breakdown, nil
}
