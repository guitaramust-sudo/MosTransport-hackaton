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

func (s *Store) GetRefreshToken(ctx context.Context, tokenHash string) (uuid.UUID, time.Time, bool, error) {
	var playerID uuid.UUID
	var expiresAt time.Time
	var revoked bool
	err := s.pool.QueryRow(ctx,
		`SELECT player_id, expires_at, revoked FROM refresh_tokens WHERE token = $1`, tokenHash,
	).Scan(&playerID, &expiresAt, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, time.Time{}, false, repo.ErrNotFound
	}
	return playerID, expiresAt, revoked, err
}

func (s *Store) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE token = $1`, tokenHash)
	return err
}
