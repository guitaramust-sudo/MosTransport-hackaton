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

type AdminService struct {
	store      repo.Store
	catalog    content.Catalog
	levels     content.Levels
	curriculum content.Curriculum
	prize      *PrizeService
	namespace  string
}

func NewAdminService(store repo.Store, catalog content.Catalog, levels content.Levels, curriculum content.Curriculum, prize *PrizeService, namespace string) *AdminService {
	return &AdminService{store: store, catalog: catalog, levels: levels, curriculum: curriculum, prize: prize, namespace: namespace}
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

type LearningSummary struct {
	Subject           LearningSubject               `json:"subject"`
	TotalXP           int                           `json:"total_xp"`
	PlayerLevel       int                           `json:"player_level"`
	LeaderboardPoints int                           `json:"leaderboard_points"`
	Achievements      []string                      `json:"achievements"`
	TrainingScope     LearningTrainingScope         `json:"training_scope"`
	SessionOutcomes   LearningSessionOutcomes       `json:"session_outcomes"`
	Competencies      []domain.CompetencyAssessment `json:"competencies"`
	WagonProgression  WagonProgressionSummary       `json:"wagon_progression"`
	LessonProgression LessonProgressionSummary      `json:"lesson_progression"`
	PrizeBalance      PrizeBalance                  `json:"prize_balance"`
	Provenance        LearningProvenance            `json:"provenance"`
}

// LessonProgressionSummary is the admin-facing curriculum-lesson ladder view:
// one entry per content-defined lesson (even never-attempted ones), mirroring
// WagonProgressionSummary's "show everything" convention.
type LessonProgressionSummary struct {
	Lessons []LessonProgressionEntry `json:"lessons"`
}

type LessonProgressionEntry struct {
	LessonID     string     `json:"lesson_id"`
	Title        string     `json:"title"`
	TheoryPass   bool       `json:"theory_pass"`
	PracticePass bool       `json:"practice_pass"`
	Completed    bool       `json:"completed"`
	BadgeID      string     `json:"badge_id,omitempty"`
	XPEarned     int        `json:"xp_earned"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
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
	ScenarioVersion    string     `json:"scenario_version"`
	ScoringRuleVersion string     `json:"scoring_rule_version"`
	AssessedAt         *time.Time `json:"assessed_at"`
}

type LearningSessionOutcomes struct {
	CompletedCount    int                `json:"completed_count"`
	PassedCount       int                `json:"passed_count"`
	RecentAssessments []RecentAssessment `json:"recent_assessments"`
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
}

// LearningSummary builds the §4.1 HR/learning read model for a user,
// counting every finished session in the outcome aggregates.
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

	completed, passed := 0, 0
	recent := []RecentAssessment{}
	var lastAssessed *time.Time
	earnedCompetencies := map[string]repo.CompetencyAward{}
	scenarioTypes := map[string]string{}
	for _, scenario := range a.catalog.Scenarios {
		scenarioTypes[scenario.ID] = string(scenario.Type)
	}

	for _, sess := range sessions {
		if sess.Status != domain.SessionStatusFinished {
			continue
		}
		completed++

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
					award := earnedCompetencies[code]
					award.XP += sit.XP
					award.Evidence++
					earnedCompetencies[code] = award
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
			passed++
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
		if award, ok := earnedCompetencies[competency.Code]; ok {
			comps = append(comps, domain.PlayerCompetency{CompetencyID: competency.ID, XP: award.XP, EvidenceCount: award.Evidence})
		}
	}
	assessments := assessCompetencies(all, comps)

	wagonProgression, err := a.wagonProgression(ctx, player, sessions)
	if err != nil {
		return nil, err
	}

	lessonProgression, err := a.lessonProgression(ctx, userID)
	if err != nil {
		return nil, err
	}

	achievements, err := a.store.ListAchievementCodes(ctx, userID)
	if err != nil {
		return nil, err
	}

	leaderboardPoints, err := a.store.GetPlayerPointsTotal(ctx, userID, a.namespace)
	if err != nil {
		return nil, err
	}

	var prizeBalance PrizeBalance
	if a.prize != nil {
		prizeBalance, err = a.prize.Balance(ctx, userID)
		if err != nil {
			return nil, err
		}
	}

	return &LearningSummary{
		Subject: LearningSubject{
			UserID:           player.ID,
			SourceSystem:     player.SourceSystem,
			ExternalUserID:   player.ExternalUserID,
			AssignedClassIDs: player.AssignedClassIDs,
			DisplayName:      player.DisplayName,
		},
		TotalXP:           player.TotalXP,
		PlayerLevel:       levelForXP(player.TotalXP),
		LeaderboardPoints: leaderboardPoints,
		Achievements:      achievements,
		TrainingScope: LearningTrainingScope{
			ClassIDs:           player.AssignedClassIDs,
			ScenarioVersion:    domain.ScenarioVersion,
			ScoringRuleVersion: domain.ScoringRuleVersion,
			AssessedAt:         lastAssessed,
		},
		SessionOutcomes: LearningSessionOutcomes{
			CompletedCount:    completed,
			PassedCount:       passed,
			RecentAssessments: recent,
		},
		Competencies:      assessments,
		WagonProgression:  wagonProgression,
		LessonProgression: lessonProgression,
		PrizeBalance:      prizeBalance,
		Provenance: LearningProvenance{
			AsOf:               time.Now(),
			ScenarioVersion:    domain.ScenarioVersion,
			ScoringRuleVersion: domain.ScoringRuleVersion,
		},
	}, nil
}

// lessonProgression builds the per-lesson curriculum progress view for a
// player, in curriculum order, including lessons the player has never
// touched. XPEarned/BadgeID come from the matching lesson_awards row (the
// "completion" award) when one exists, zero-value/empty otherwise.
func (a *AdminService) lessonProgression(ctx context.Context, playerID uuid.UUID) (LessonProgressionSummary, error) {
	progressRows, err := a.store.ListLessonProgressByPlayer(ctx, playerID)
	if err != nil {
		return LessonProgressionSummary{}, err
	}
	progressByLesson := map[string]domain.LessonProgress{}
	for _, p := range progressRows {
		progressByLesson[p.LessonID] = p
	}

	awardRows, err := a.store.ListLessonAwardsByPlayer(ctx, playerID)
	if err != nil {
		return LessonProgressionSummary{}, err
	}
	awardByLesson := map[string]domain.LessonAward{}
	for _, aw := range awardRows {
		if aw.AwardType == "completion" {
			awardByLesson[aw.LessonID] = aw
		}
	}

	entries := make([]LessonProgressionEntry, 0, len(a.curriculum.Lessons))
	for _, lesson := range a.curriculum.Lessons {
		entry := LessonProgressionEntry{
			LessonID: lesson.LessonID,
			Title:    lesson.Title,
		}
		if p, ok := progressByLesson[lesson.LessonID]; ok {
			entry.TheoryPass = p.TheoryPass
			entry.PracticePass = p.PracticePass
			entry.Completed = p.CompletedAt != nil
			entry.CompletedAt = p.CompletedAt
		}
		if award, ok := awardByLesson[lesson.LessonID]; ok {
			entry.XPEarned = award.XPDelta
			entry.BadgeID = award.BadgeID
		}
		entries = append(entries, entry)
	}

	return LessonProgressionSummary{Lessons: entries}, nil
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
