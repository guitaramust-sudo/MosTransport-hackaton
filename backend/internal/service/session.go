package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionService struct {
	store         repo.Store
	situationsNum int
	timeout       time.Duration
}

func NewSessionService(store repo.Store, situationsNum int, timeout time.Duration) *SessionService {
	return &SessionService{store: store, situationsNum: situationsNum, timeout: timeout}
}

func (s *SessionService) Start(ctx context.Context, playerID uuid.UUID) (*domain.Session, []domain.Situation, error) {
	sess, err := s.store.CreateSession(ctx, playerID)
	if err != nil {
		return nil, nil, err
	}

	selected := pickArchetypes(s.situationsNum)
	situations := make([]domain.Situation, 0, len(selected))
	for _, a := range selected {
		deadline := time.Now().Add(s.timeout)
		params := map[string]any{
			"code":     a.Code,
			"name":     a.Name,
			"persona":  a.Persona,
			"scenario": a.Scenario,
			"opening":  a.Opening,
		}
		sit, err := s.store.CreateSituation(ctx, domain.Situation{
			SessionID:       sess.ID,
			Status:          domain.SituationStatusActive,
			PassengerParams: params,
			Loyalty:         50,
			Safety:          50,
			TimerDeadline:   &deadline,
		})
		if err != nil {
			return nil, nil, err
		}
		if _, err := s.store.CreateMessage(ctx, sit.ID, domain.MessageRolePassenger, a.Opening, nil); err != nil {
			return nil, nil, err
		}
		situations = append(situations, sit)
	}

	return &sess, situations, nil
}

func pickArchetypes(n int) []PassengerArchetype {
	pool := append([]PassengerArchetype(nil), archetypes...)
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
	SituationID uuid.UUID `json:"situation_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Outcome     string    `json:"outcome"`
	Loyalty     int       `json:"loyalty"`
	Safety      int       `json:"safety"`
	XP          int       `json:"xp"`
	Categories  []string  `json:"categories"`
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
				outcome := "unfinished"
				situations[i].Status = domain.SituationStatusClosed
				situations[i].Outcome = &outcome
				if err := s.store.UpdateSituation(ctx, situations[i]); err != nil {
					return nil, err
				}
			}
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

		messages, err := s.store.ListMessagesBySituation(ctx, sit.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range messages {
			if m.Role == domain.MessageRolePlayer && m.Category != nil {
				sb.Categories = append(sb.Categories, *m.Category)
				eff := effectFor(normalizeCategory(*m.Category))
				if eff.Competency != "" {
					breakdown.Competencies[eff.Competency] += eff.XP
				}
			}
		}

		if sit.Outcome != nil && *sit.Outcome != "unfinished" && *sit.Outcome != "timeout" {
			sb.XP = situationXP(sit.Loyalty, sit.Safety)
		}
		breakdown.TotalXP += sb.XP
		breakdown.Situations = append(breakdown.Situations, sb)
	}

	if wasActive {
		if breakdown.TotalXP > 0 {
			if err := s.store.AddTotalXP(ctx, playerID, breakdown.TotalXP); err != nil {
				return nil, err
			}
		}
		for code, xp := range breakdown.Competencies {
			if xp > 0 {
				if err := s.store.AddCompetencyXP(ctx, playerID, code, xp); err != nil {
					return nil, err
				}
			}
		}
		if err := s.store.FinishSession(ctx, sessionID, time.Now()); err != nil {
			return nil, err
		}
	}

	return breakdown, nil
}
