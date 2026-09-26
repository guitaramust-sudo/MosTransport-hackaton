package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

// BootstrapAdmin is only called from the trusted local bootstrap command. An
// existing account must prove knowledge of its password before promotion.
func (s *Store) BootstrapAdmin(ctx context.Context, email, password string) (domain.Player, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 12 {
		return domain.Player{}, fmt.Errorf("admin email and password of at least 12 characters are required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Player{}, err
	}
	defer tx.Rollback(ctx)
	var existingID uuid.UUID
	var passwordHash string
	err = tx.QueryRow(ctx, `SELECT id, password_hash FROM players WHERE lower(email) = $1 FOR UPDATE`, email).Scan(&existingID, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return domain.Player{}, hashErr
		}
		var player domain.Player
		err = tx.QueryRow(ctx,
			`INSERT INTO players (email, username, password_hash, role)
			 VALUES ($1, $1, $2, 'admin') RETURNING `+playerColumns,
			email, string(hash)).Scan(scanPlayer(&player)...)
		if err != nil {
			return domain.Player{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Player{}, err
		}
		return player, nil
	}
	if err != nil {
		return domain.Player{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return domain.Player{}, fmt.Errorf("existing account password does not match")
	}
	var player domain.Player
	err = tx.QueryRow(ctx,
		`UPDATE players SET role = 'admin' WHERE id = $1 RETURNING `+playerColumns,
		existingID).Scan(scanPlayer(&player)...)
	if err != nil {
		return domain.Player{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Player{}, err
	}
	return player, nil
}
