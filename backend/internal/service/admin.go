package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUnapprovedContent = errors.New("session contains unapproved content")

type AdminService struct {
	store   repo.Store
	catalog content.Catalog
	levels  content.Levels
}

func NewAdminService(store repo.Store, catalog content.Catalog, levels content.Levels) *AdminService {
	return &AdminService{store: store, catalog: catalog, levels: levels}
}

// CreateExternalUserInput is the payload of POST /admin/users.
type CreateExternalUserInput struct {
	SourceSystem     string   `json:"source_system"`
	ExternalUserID   string   `json:"external_user_id"`
	AssignedClassIDs []string `json:"assigned_class_ids"`
	CommandID        string   `json:"command_id"`
	DisplayName      *string  `json:"display_name"`
	DepotID          *string  `json:"depot_id"`
	BrigadeID        *string  `json:"brigade_id"`
}

// CreateExternalUser registers or refreshes a profile linked to an external HR
// system. Idempotent by (source_system, external_user_id).
func (a *AdminService) CreateExternalUser(ctx context.Context, in CreateExternalUserInput) (domain.Player, bool, error) {
	if in.SourceSystem == "" || in.ExternalUserID == "" {
		return domain.Player{}, false, errors.New("source_system and external_user_id are required")
	}
	return a.store.UpsertExternalUser(ctx, in.SourceSystem, in.ExternalUserID, in.DisplayName, in.DepotID, in.BrigadeID, in.AssignedClassIDs)
}

// CreatePlayerAccountInput is the payload of POST /admin/players.
type CreatePlayerAccountInput struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	BrigadeName string `json:"brigade_name"`
}

// CreatePlayerAccount creates a normal, password-login player account on an
// admin's behalf — the only way to get a new account now that public
// self-registration is disabled. It does not issue tokens: the new player
// logs in themselves via POST /auth/login.
func (a *AdminService) CreatePlayerAccount(ctx context.Context, in CreatePlayerAccountInput) (domain.Player, error) {
	if in.Email == "" || in.Username == "" || len(in.Password) < 6 || in.BrigadeName == "" {
		return domain.Player{}, errors.New("email, username, password (min 6 chars) and brigade_name are required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.Player{}, err
	}
	brigade := in.BrigadeName
	player, err := a.store.CreatePlayerWithBrigade(ctx, in.Email, in.Username, string(hash), &brigade)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Player{}, ErrEmailTaken
		}
		return domain.Player{}, err
	}
	return player, nil
}

func (a *AdminService) ApproveSession(ctx context.Context, sessionID uuid.UUID) error {
	sess, err := a.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.Status != domain.SessionStatusFinished {
		return repo.ErrConflict
	}
	situations, err := a.store.ListSituationsBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	approved := map[string]bool{}
	for _, scenario := range a.catalog.Scenarios {
		approved[scenario.ID] = scenario.ValidationStatus == "approved"
	}
	if len(situations) == 0 {
		return ErrUnapprovedContent
	}
	for _, sit := range situations {
		if sit.SituationDefID == nil || !approved[*sit.SituationDefID] {
			return ErrUnapprovedContent
		}
	}
	return a.store.ApproveSession(ctx, sessionID)
}

type LearningSummary struct {
	DataStatus       string                        `json:"data_status"`
	Subject          LearningSubject               `json:"subject"`
	TrainingScope    LearningTrainingScope         `json:"training_scope"`
	SessionOutcomes  LearningSessionOutcomes       `json:"session_outcomes"`
	Competencies     []domain.CompetencyAssessment `json:"competencies"`
	WagonProgression WagonProgressionSummary       `json:"wagon_progression"`
	Provenance       LearningProvenance            `json:"provenance"`
}

// WagonProgressionSummary is the §-adjacent wagon-mode ladder view: one entry
// per content-defined level (even never-attempted ones), plus the player's
// current unlocked progress.
type WagonProgressionSummary struct {
	CurrentProgress int                 `json:"current_progress"` // highest Level.Order fully passed
	Levels          []WagonLevelHistory `json:"levels"`
}

type WagonLevelHistory struct {
	LevelID       string     `json:"level_id"`
	Order         int        `json:"order"`
	Title         string     `json:"title"`
	Status        string     `json:"status"` // "locked" | "unlocked" | "passed"
	Attempts      int        `json:"attempts"`
	Passed        bool       `json:"passed"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
}

type LearningSubject struct {
	UserID           uuid.UUID `json:"user_id"`
	SourceSystem     *string   `json:"source_system"`
	ExternalUserID   *string   `json:"external_user_id"`
	AssignedClassIDs []string  `json:"assigned_class_ids"`
	DisplayName      *string   `json:"display_name"`
}

type LearningTrainingScope struct {
	ClassIDs           []string   `json:"class_ids"`
	MasteryStage       *string    `json:"mastery_stage"`
	ValidationStatus   string     `json:"validation_status"`
	ScenarioVersion    string     `json:"scenario_version"`
	ScoringRuleVersion string     `json:"scoring_rule_version"`
	AssessedAt         *time.Time `json:"assessed_at"`
}

type LearningSessionOutcomes struct {
	ApprovedCompletedCount int                `json:"approved_completed_count"`
	ApprovedPassedCount    int                `json:"approved_passed_count"`
	RecentAssessments      []RecentAssessment `json:"recent_assessments"`
}

type RecentAssessment struct {
	SessionID             uuid.UUID  `json:"session_id"`
	CompletedAt           *time.Time `json:"completed_at"`
	SessionPass           bool       `json:"session_pass"`
	WorldSafetyCurrent    int        `json:"world_safety_current"`
	SessionSafetyScore    int        `json:"session_safety_score"`
	Loyalty               int        `json:"loyalty"`
	CriticalViolations    int        `json:"critical_violations"`
	UnresolvedCommitments int        `json:"unresolved_commitments"`
}

type LearningProvenance struct {
	AsOf               time.Time `json:"as_of"`
	ScenarioVersion    string    `json:"scenario_version"`
	ScoringRuleVersion string    `json:"scoring_rule_version"`
	ExcludedDraftCount int       `json:"excluded_draft_count"`
}

// LearningSummary builds the §4.1 HR/learning read model for a user, counting
// only approved sessions in the outcome aggregates.
func (a *AdminService) LearningSummary(ctx context.Context, userID uuid.UUID) (*LearningSummary, error) {
	player, err := a.store.GetPlayerByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	sessions, err := a.store.ListPlayerSessions(ctx, userID)
	if err != nil {
		return nil, err
	}

	approvedCompleted, approvedPassed, excludedDraft := 0, 0, 0
	recent := []RecentAssessment{}
	var lastAssessed *time.Time
	approvedCompetencies := map[string]repo.CompetencyAward{}
	scenarioTypes := map[string]string{}
	for _, scenario := range a.catalog.Scenarios {
		scenarioTypes[scenario.ID] = string(scenario.Type)
	}

	for _, sess := range sessions {
		if sess.ValidationStatus != domain.ValidationApproved || sess.Status != domain.SessionStatusFinished {
			excludedDraft++
			continue
		}
		approvedCompleted++

		situations, err := a.store.ListSituationsBySession(ctx, sess.ID)
		if err != nil {
			return nil, err
		}

		ra := RecentAssessment{
			SessionID:             sess.ID,
			CompletedAt:           sess.FinishedAt,
			WorldSafetyCurrent:    100,
			SessionSafetyScore:    100,
			SessionPass:           len(situations) > 0 && len(sess.PendingSituations) == 0,
			UnresolvedCommitments: len(sess.PendingSituations),
		}
		loyaltySum, safetySum := 0, 0
		for _, sit := range situations {
			if sit.SituationDefID != nil {
				if code := scenarioTypes[*sit.SituationDefID]; code != "" && sit.Outcome != nil && *sit.Outcome != "unfinished" {
					award := approvedCompetencies[code]
					award.XP += sit.XP
					award.Evidence++
					approvedCompetencies[code] = award
				}
			}
			outcome := "unfinished"
			if sit.Outcome != nil {
				outcome = *sit.Outcome
			}
			switch outcome {
			case "fail", "timeout":
				ra.CriticalViolations++
				ra.SessionPass = false
			case "unfinished":
				ra.UnresolvedCommitments++
				ra.SessionPass = false
			}
			loyaltySum += sit.Loyalty
			safetySum += sit.Safety
		}
		if n := len(situations); n > 0 {
			ra.Loyalty = loyaltySum / n
			ra.SessionSafetyScore = safetySum / n
			ra.WorldSafetyCurrent = ra.SessionSafetyScore
		}
		if ra.SessionPass {
			approvedPassed++
		}
		if sess.FinishedAt != nil && (lastAssessed == nil || sess.FinishedAt.After(*lastAssessed)) {
			lastAssessed = sess.FinishedAt
		}
		recent = append(recent, ra)
	}

	all, err := a.store.ListCompetencies(ctx)
	if err != nil {
		return nil, err
	}
	var comps []domain.PlayerCompetency
	for _, competency := range all {
		if award, ok := approvedCompetencies[competency.Code]; ok {
			comps = append(comps, domain.PlayerCompetency{CompetencyID: competency.ID, XP: award.XP, EvidenceCount: award.Evidence})
		}
	}
	assessments := assessCompetencies(all, comps)

	wagonProgression, err := a.wagonProgression(ctx, player, sessions)
	if err != nil {
		return nil, err
	}

	validation := domain.ValidationDraft
	if approvedCompleted > 0 {
		validation = domain.ValidationApproved
	}

	dataStatus := "no_approved_data"
	if approvedCompleted > 0 {
		dataStatus = "available"
	}
	return &LearningSummary{
		DataStatus: dataStatus,
		Subject: LearningSubject{
			UserID:           player.ID,
			SourceSystem:     player.SourceSystem,
			ExternalUserID:   player.ExternalUserID,
			AssignedClassIDs: player.AssignedClassIDs,
			DisplayName:      player.DisplayName,
		},
		TrainingScope: LearningTrainingScope{
			ClassIDs:           player.AssignedClassIDs,
			ValidationStatus:   validation,
			ScenarioVersion:    domain.ScenarioVersion,
			ScoringRuleVersion: domain.ScoringRuleVersion,
			AssessedAt:         lastAssessed,
		},
		SessionOutcomes: LearningSessionOutcomes{
			ApprovedCompletedCount: approvedCompleted,
			ApprovedPassedCount:    approvedPassed,
			RecentAssessments:      recent,
		},
		Competencies:     assessments,
		WagonProgression: wagonProgression,
		Provenance: LearningProvenance{
			AsOf:               time.Now(),
			ScenarioVersion:    domain.ScenarioVersion,
			ScoringRuleVersion: domain.ScoringRuleVersion,
			ExcludedDraftCount: excludedDraft,
		},
	}, nil
}

// wagonProgression builds the per-level wagon-mode attempt history for a
// player, in Level.Order, including levels the player has never attempted.
// Only finished wagon sessions count as attempts (an in-progress session
// isn't a completed attempt yet). Pass/fail for an attempt follows the same
// rule as WagonService.AdvanceIfPassed: passed iff no situation in the
// session resolved with outcome "fail".
func (a *AdminService) wagonProgression(ctx context.Context, player domain.Player, sessions []domain.Session) (WagonProgressionSummary, error) {
	type levelAgg struct {
		attempts      int
		passed        bool
		lastAttemptAt *time.Time
	}
	byLevel := map[string]*levelAgg{}

	for _, sess := range sessions {
		if sess.WagonState == nil || sess.WagonState.LevelID == "" {
			continue
		}
		if sess.Status != domain.SessionStatusFinished {
			continue
		}
		situations, err := a.store.ListSituationsBySession(ctx, sess.ID)
		if err != nil {
			return WagonProgressionSummary{}, err
		}
		passed := true
		for _, sit := range situations {
			if sit.Outcome != nil && *sit.Outcome == "fail" {
				passed = false
				break
			}
		}
		agg := byLevel[sess.WagonState.LevelID]
		if agg == nil {
			agg = &levelAgg{}
			byLevel[sess.WagonState.LevelID] = agg
		}
		agg.attempts++
		if passed {
			agg.passed = true
		}
		if sess.FinishedAt != nil && (agg.lastAttemptAt == nil || sess.FinishedAt.After(*agg.lastAttemptAt)) {
			agg.lastAttemptAt = sess.FinishedAt
		}
	}

	levels := make([]WagonLevelHistory, 0, len(a.levels))
	for _, lvl := range a.levels {
		status := "locked"
		switch {
		case lvl.Order <= player.WagonProgress:
			status = "passed"
		case lvl.Order == player.WagonProgress+1:
			status = "unlocked"
		}
		history := WagonLevelHistory{
			LevelID: lvl.ID,
			Order:   lvl.Order,
			Title:   lvl.Title,
			Status:  status,
		}
		if agg := byLevel[lvl.ID]; agg != nil {
			history.Attempts = agg.attempts
			history.Passed = agg.passed
			history.LastAttemptAt = agg.lastAttemptAt
		}
		levels = append(levels, history)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i].Order < levels[j].Order })

	return WagonProgressionSummary{CurrentProgress: player.WagonProgress, Levels: levels}, nil
}
