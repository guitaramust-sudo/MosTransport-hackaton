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
var ErrNoEligibleScenarios = errors.New("no eligible scenarios for this points namespace")

type SessionService struct {
	store           repo.Store
	catalog         content.Catalog
	situationsNum   int
	situations      *SituationService
	pointsNamespace string
}

func NewSessionService(store repo.Store, catalog content.Catalog, situationsNum int, situations *SituationService, pointsNamespace string) *SessionService {
	return &SessionService{store: store, catalog: catalog, situationsNum: situationsNum, situations: situations, pointsNamespace: pointsNamespace}
}

func (s *SessionService) Start(ctx context.Context, playerID uuid.UUID) (*domain.Session, []domain.Situation, error) {
	if err := s.catalog.Validate(); err != nil {
		return nil, nil, err
	}
	pool := make([]content.Scenario, 0, len(s.catalog.Scenarios))
	for _, scenario := range s.catalog.Scenarios {
		if scenario.ValidationStatus == "approved" || (s.pointsNamespace == "demo" && scenario.ValidationStatus == "draft") {
			pool = append(pool, scenario)
		}
	}
	if len(pool) == 0 {
		return nil, nil, ErrNoEligibleScenarios
	}
	selected := pickScenarios(pool, s.situationsNum)

	first := selected[0]
	pending := make([]string, 0, len(selected)-1)
	for _, scenario := range selected[1:] {
		pending = append(pending, scenario.ID)
	}

	draft := buildSituationDraft(first, pickPassenger(s.catalog))
	sess, situations, err := s.store.CreateSessionWithSituations(ctx, playerID, pending, []domain.Situation{draft})
	if err != nil {
		return nil, nil, err
	}
	return &sess, situations, nil
}

// pickPassenger selects a random passenger for a situation.
func pickPassenger(catalog content.Catalog) content.Passenger {
	return catalog.Passengers[rand.Intn(len(catalog.Passengers))]
}

// buildSituationDraft turns a scenario and a passenger into a situation with a
// fresh timer deadline.
func buildSituationDraft(scenario content.Scenario, passenger content.Passenger) domain.Situation {
	deadline := time.Now().Add(time.Duration(scenario.TimeLimitSec) * time.Second)
	scenarioID, passengerID := scenario.ID, passenger.ID
	params := map[string]any{
		"code":                      scenario.ID,
		"name":                      passenger.ID,
		"persona":                   passenger.PromptHint,
		"scenario":                  scenario.Title,
		"content_validation_status": scenario.ValidationStatus,
		"opening":                   scenario.Opening,
		"situation_def_id":          scenario.ID,
		"passenger_id":              passenger.ID,
		"prompt_hint":               passenger.PromptHint,
		"language":                  passenger.Language,
		"traits":                    passenger.Traits,
		"traits_text":               strings.Join(passenger.Traits, ", "),
	}
	return domain.Situation{
		Status:          domain.SituationStatusActive,
		SituationDefID:  &scenarioID,
		PassengerID:     &passengerID,
		PassengerParams: params,
		Loyalty:         50,
		Safety:          50,
		TimerDeadline:   &deadline,
	}
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
	SessionID              uuid.UUID            `json:"session_id"`
	ScenarioVersion        string               `json:"scenario_version"`
	ScoringRuleVersion     string               `json:"scoring_rule_version"`
	ValidationStatus       string               `json:"validation_status"`
	PointsNamespace        string               `json:"points_namespace"`
	TotalXP                int                  `json:"total_xp"`
	SessionPass            bool                 `json:"session_pass"`
	WorldSafetyCurrent     int                  `json:"world_safety_current"`
	SessionSafetyScore     int                  `json:"session_safety_score"`
	Loyalty                int                  `json:"loyalty"`
	CriticalViolations     int                  `json:"critical_violations"`
	UnresolvedCommitments  int                  `json:"unresolved_commitments"`
	Competencies           map[string]int       `json:"competencies_xp"` // code -> xp gained
	LeaderboardPointsDelta int                  `json:"leaderboard_points_delta"`
	LeaderboardPointsTotal int                  `json:"leaderboard_points_total"`
	LeaderboardEligible    bool                 `json:"leaderboard_eligible"`
	Situations             []SituationBreakdown `json:"situations"`
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
	for attempt := 0; attempt <= s.situationsNum; attempt++ {
		breakdown, err := s.finishOnce(ctx, playerID, sessionID)
		if !errors.Is(err, repo.ErrConflict) {
			return breakdown, err
		}
	}
	return nil, repo.ErrConflict
}

func (s *SessionService) finishOnce(ctx context.Context, playerID, sessionID uuid.UUID) (*Breakdown, error) {
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
		sess, err = s.store.GetSession(ctx, sessionID)
		if err != nil {
			return nil, err
		}
	}

	breakdown := &Breakdown{
		SessionID:          sessionID,
		ScenarioVersion:    domain.ScenarioVersion,
		ScoringRuleVersion: domain.ScoringRuleVersion,
		ValidationStatus:   sess.ValidationStatus,
		PointsNamespace:    s.pointsNamespace,
		Competencies:       map[string]int{},
	}
	breakdown.UnresolvedCommitments = len(sess.PendingSituations)

	competencyXP := map[string]int{}
	competencyEvidence := map[string]int{}
	loyaltySum, safetySum := 0, 0
	allResolved := len(sess.PendingSituations) == 0

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

		switch sb.Outcome {
		case "fail":
			breakdown.CriticalViolations++
			allResolved = false
		case "timeout":
			breakdown.CriticalViolations++
			allResolved = false
		case "unfinished":
			breakdown.UnresolvedCommitments++
			allResolved = false
		}

		loyaltySum += sit.Loyalty
		safetySum += sit.Safety

		if code := s.competencyCodeFor(sit); code != "" && sb.Outcome != "unfinished" {
			competencyXP[code] += sit.XP
			competencyEvidence[code]++
			breakdown.Competencies[code] += sit.XP
		}
	}

	if n := len(situations); n > 0 {
		breakdown.Loyalty = loyaltySum / n
		breakdown.SessionSafetyScore = safetySum / n
		breakdown.WorldSafetyCurrent = breakdown.SessionSafetyScore
	} else {
		breakdown.SessionSafetyScore = 100
		breakdown.WorldSafetyCurrent = 100
	}
	breakdown.SessionPass = allResolved && len(situations) > 0
	breakdown.LeaderboardPointsDelta = breakdown.TotalXP

	if wasActive {
		awards := make(map[string]repo.CompetencyAward, len(competencyXP))
		for code, xp := range competencyXP {
			awards[code] = repo.CompetencyAward{XP: xp, Evidence: competencyEvidence[code]}
		}
		if _, err := s.store.FinishSessionAndAwardXP(ctx, sessionID, playerID, breakdown.TotalXP, awards, len(situations), len(sess.PendingSituations), time.Now()); err != nil {
			return nil, err
		}
	}

	player, err := s.store.GetPlayerByID(ctx, playerID)
	if err != nil {
		return nil, err
	}
	breakdown.LeaderboardPointsTotal = player.TotalXP
	breakdown.LeaderboardEligible = s.pointsNamespace == "official"

	return breakdown, nil
}

// competencyCodeFor maps a situation back to the competency trained by its
// scenario type (service, conflict, medical, safety, informational).
func (s *SessionService) competencyCodeFor(sit domain.Situation) string {
	if sit.SituationDefID == nil {
		return ""
	}
	for _, scenario := range s.catalog.Scenarios {
		if scenario.ID == *sit.SituationDefID {
			return string(scenario.Type)
		}
	}
	return ""
}
