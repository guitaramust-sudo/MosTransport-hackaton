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

// Store is the data-access boundary consumed by the service layer.
// The postgres package implements it; tests may substitute a fake.
type Store interface {
	// Players
	CreatePlayer(ctx context.Context, email, username, passwordHash string) (domain.Player, error)
	GetPlayerByEmail(ctx context.Context, email string) (domain.Player, error)
	GetPlayerByID(ctx context.Context, id uuid.UUID) (domain.Player, error)
	AddTotalXP(ctx context.Context, playerID uuid.UUID, xp int) error
	AddCompetencyXP(ctx context.Context, playerID uuid.UUID, competencyCode string, xp int) error
	ListCompetencies(ctx context.Context) ([]domain.Competency, error)
	GetPlayerCompetencies(ctx context.Context, playerID uuid.UUID) ([]domain.PlayerCompetency, error)
	Leaderboard(ctx context.Context, limit int) ([]domain.Player, error)

	// Sessions
	CreateSession(ctx context.Context, playerID uuid.UUID) (domain.Session, error)
	GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error)
	FinishSession(ctx context.Context, id uuid.UUID, finishedAt time.Time) error

	// Refresh tokens
	CreateRefreshToken(ctx context.Context, playerID uuid.UUID, tokenHash string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (playerID uuid.UUID, expiresAt time.Time, revoked bool, err error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error

	// Situations
	CreateSituation(ctx context.Context, s domain.Situation) (domain.Situation, error)
	GetSituation(ctx context.Context, id uuid.UUID) (domain.Situation, error)
	ListSituationsBySession(ctx context.Context, sessionID uuid.UUID) ([]domain.Situation, error)
	UpdateSituation(ctx context.Context, s domain.Situation) error

	// Messages
	CreateMessage(ctx context.Context, situationID uuid.UUID, role, content string, category *string) (domain.Message, error)
	ListMessagesBySituation(ctx context.Context, situationID uuid.UUID) ([]domain.Message, error)
	CountPlayerMessages(ctx context.Context, situationID uuid.UUID) (int, error)
	UpdateMessageCategory(ctx context.Context, id int, category string) error
}
