package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

// ErrNotFound is the sentinel returned when a record does not exist.
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("state conflict")
var ErrDeadlineExceeded = errors.New("situation deadline exceeded")

type CompetencyAward struct {
	XP       int
	Evidence int
}

type PointsStanding struct {
	PlayerID uuid.UUID
	Username string
	Points   int
}

// SimulationStore is separate from the legacy dialogue store while clients
// migrate to the branching flow. The callback executes under a row lock.
type SimulationStore interface {
	CreateSimulationRun(ctx context.Context, run domain.SimulationRun) (domain.SimulationRun, error)
	GetSimulationRun(ctx context.Context, runID, playerID uuid.UUID) (domain.SimulationRun, error)
	ApplySimulationCommand(ctx context.Context, runID, playerID, commandID uuid.UUID, expectedVersion int, apply func(domain.SimulationRun) (domain.SimulationRun, error)) (domain.SimulationRun, error)
	AdvanceSimulationTimer(ctx context.Context, runID, playerID uuid.UUID, apply func(domain.SimulationRun) (domain.SimulationRun, bool)) (domain.SimulationRun, error)
	ListDueSimulationRuns(ctx context.Context, now time.Time) ([]domain.SimulationDueRun, error)
	FinalizeSimulationRewards(ctx context.Context, run domain.SimulationRun) error
	GetSimulationPoints(ctx context.Context, runID uuid.UUID, namespace string) (int, error)
	GetPlayerPointsTotal(ctx context.Context, playerID uuid.UUID, namespace string) (int, error)
	ListNotifications(ctx context.Context, playerID uuid.UUID) ([]domain.Notification, error)
	GetChallengeProgress(ctx context.Context, playerID uuid.UUID, now time.Time) (domain.ChallengeProgress, error)
}

// LockedSituation is a transaction-scoped view used while closing a situation.
// The row lock freezes the dialog and escalations until scoring is saved.
type LockedSituation interface {
	Situation() domain.Situation
	ListMessages(ctx context.Context) ([]domain.Message, error)
	Close(ctx context.Context, result domain.Situation) error
}

// Store is the data-access boundary consumed by the service layer.
// The postgres package implements it; tests may substitute a fake.
type Store interface {
	// Players
	CreatePlayer(ctx context.Context, email, username, passwordHash string) (domain.Player, error)
	GetPlayerByEmail(ctx context.Context, email string) (domain.Player, error)
	GetPlayerByID(ctx context.Context, id uuid.UUID) (domain.Player, error)
	AddTotalXP(ctx context.Context, playerID uuid.UUID, xp int) error
	AddCompetencyXP(ctx context.Context, playerID uuid.UUID, competencyCode string, xp, evidence int) error
	ListCompetencies(ctx context.Context) ([]domain.Competency, error)
	GetPlayerCompetencies(ctx context.Context, playerID uuid.UUID) ([]domain.PlayerCompetency, error)
	ListAchievementCodes(ctx context.Context, playerID uuid.UUID) ([]string, error)
	Leaderboard(ctx context.Context, limit int) ([]domain.Player, error)
	LeaderboardScoped(ctx context.Context, scope, groupID string, limit int) ([]domain.Player, error)
	PointsLeaderboard(ctx context.Context, namespace, scope, groupID string) ([]PointsStanding, error)
	GetPlayerPointsTotal(ctx context.Context, playerID uuid.UUID, namespace string) (int, error)
	UpsertExternalUser(ctx context.Context, sourceSystem, externalUserID string, displayName, depotID, brigadeID *string, assignedClassIDs []string) (domain.Player, bool, error)
	GetPlayerByExternal(ctx context.Context, sourceSystem, externalUserID string) (domain.Player, error)

	// Sessions
	CreateSession(ctx context.Context, playerID uuid.UUID) (domain.Session, error)
	CreateSessionWithSituations(ctx context.Context, playerID uuid.UUID, pending []string, situations []domain.Situation) (domain.Session, []domain.Situation, error)
	SpawnNextSituation(ctx context.Context, sessionID uuid.UUID, resolve func(scenarioID string) (domain.Situation, error)) (*domain.Situation, error)
	GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error)
	FinishSession(ctx context.Context, id uuid.UUID, finishedAt time.Time) error
	FinishSessionAndAwardXP(ctx context.Context, sessionID, playerID uuid.UUID, xp int, awards map[string]CompetencyAward, expectedSituations, expectedPending int, finishedAt time.Time) (bool, error)
	ApproveSession(ctx context.Context, id uuid.UUID) error
	ListPlayerSessions(ctx context.Context, playerID uuid.UUID) ([]domain.Session, error)

	// Refresh tokens
	CreateRefreshToken(ctx context.Context, playerID uuid.UUID, tokenHash string, expiresAt time.Time) error
	ConsumeRefreshToken(ctx context.Context, tokenHash string) (playerID uuid.UUID, err error)

	// Situations
	CreateSituation(ctx context.Context, s domain.Situation) (domain.Situation, error)
	GetSituation(ctx context.Context, id uuid.UUID) (domain.Situation, error)
	ListSituationsBySession(ctx context.Context, sessionID uuid.UUID) ([]domain.Situation, error)
	UpdateSituation(ctx context.Context, s domain.Situation) error
	CloseSituation(ctx context.Context, s domain.Situation) (bool, error)
	WithSituationLock(ctx context.Context, id uuid.UUID, fn func(LockedSituation) error) error
	AddEscalation(ctx context.Context, situationID uuid.UUID, target string) ([]string, error)
	RecordEscalation(ctx context.Context, situationID, playerID uuid.UUID, target string) ([]string, error)
	ListExpiredSituationIDs(ctx context.Context, now time.Time) ([]uuid.UUID, error)

	// Messages
	CreateMessage(ctx context.Context, situationID uuid.UUID, role, content string, category *string) (domain.Message, error)
	AppendTurn(ctx context.Context, situationID, playerID uuid.UUID, text, reply string, targets []string, inputMode *string) (int, error)
	CreateEscalationMessage(ctx context.Context, situationID uuid.UUID, target string) error
	ListMessagesBySituation(ctx context.Context, situationID uuid.UUID) ([]domain.Message, error)
	CountPlayerMessages(ctx context.Context, situationID uuid.UUID) (int, error)
	UpdateMessageCategory(ctx context.Context, id int, category string) error
}
