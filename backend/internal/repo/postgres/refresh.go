package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/repo"
)

func (s *Store) CreateRefreshToken(ctx context.Context, playerID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (player_id, token, expires_at) VALUES ($1, $2, $3)`,
		playerID, tokenHash, expiresAt)
	return err
}

func (s *Store) ConsumeRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var playerID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`UPDATE refresh_tokens SET revoked = true
		 WHERE token = $1 AND revoked = false AND expires_at > now()
		 RETURNING player_id`, tokenHash,
	).Scan(&playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, repo.ErrNotFound
	}
	return playerID, err
}
