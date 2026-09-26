package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var ErrUserNotFound = errors.New("user not found")

type AdminService struct {
	store repo.Store
}

func NewAdminService(store repo.Store) *AdminService {
	return &AdminService{store: store}
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

func (a *AdminService) ApproveSession(ctx context.Context, sessionID uuid.UUID) error {
	return a.store.ApproveSession(ctx, sessionID)
}

type LearningSummary struct {
	Subject         LearningSubject         `json:"subject"`
	TrainingScope   LearningTrainingScope   `json:"training_scope"`
	SessionOutcomes LearningSessionOutcomes `json:"session_outcomes"`
	Competencies    []domain.CompetencyAssessment `json:"competencies"`
	Provenance      LearningProvenance      `json:"provenance"`
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
	AsOf                time.Time `json:"as_of"`
	ScenarioVersion     string    `json:"scenario_version"`
	ScoringRuleVersion  string    `json:"scoring_rule_version"`
	ExcludedDraftCount  int       `json:"excluded_draft_count"`
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
	var recent []RecentAssessment
	var lastAssessed *time.Time

	for _, sess := range sessions {
		if sess.ValidationStatus != domain.ValidationApproved {
			excludedDraft++
			continue
		}
		approvedCompleted++

		situations, err := a.store.ListSituationsBySession(ctx, sess.ID)
		if err != nil {
			return nil, err
		}

		ra := RecentAssessment{
			SessionID:          sess.ID,
			CompletedAt:        sess.FinishedAt,
			WorldSafetyCurrent: 100,
			SessionSafetyScore: 100,
			SessionPass:        true,
		}
		loyaltySum, safetySum := 0, 0
		for _, sit := range situations {
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

	comps, err := a.store.GetPlayerCompetencies(ctx, userID)
	if err != nil {
		return nil, err
	}
	all, err := a.store.ListCompetencies(ctx)
	if err != nil {
		return nil, err
	}
	assessments := assessCompetencies(all, comps)

	validation := domain.ValidationDraft
	if approvedCompleted > 0 {
		validation = domain.ValidationApproved
	}

	return &LearningSummary{
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
		Competencies: assessments,
		Provenance: LearningProvenance{
			AsOf:               time.Now(),
			ScenarioVersion:    domain.ScenarioVersion,
			ScoringRuleVersion: domain.ScoringRuleVersion,
			ExcludedDraftCount: excludedDraft,
		},
	}, nil
}
