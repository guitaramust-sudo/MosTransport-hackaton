package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

func (s *Store) CreatePlayer(ctx context.Context, email, username, passwordHash string) (domain.Player, error) {
	var p domain.Player
	err := s.pool.QueryRow(ctx,
		`INSERT INTO players (email, username, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, username, password_hash, total_xp, created_at`,
		email, username, passwordHash,
	).Scan(&p.ID, &p.Email, &p.Username, &p.PasswordHash, &p.TotalXP, &p.CreatedAt)
	return p, err
}

func (s *Store) GetPlayerByEmail(ctx context.Context, email string) (domain.Player, error) {
	return s.scanPlayer(ctx, `SELECT id, email, username, password_hash, total_xp, created_at FROM players WHERE email = $1`, email)
}

func (s *Store) GetPlayerByID(ctx context.Context, id uuid.UUID) (domain.Player, error) {
	return s.scanPlayer(ctx, `SELECT id, email, username, password_hash, total_xp, created_at FROM players WHERE id = $1`, id)
}

func (s *Store) scanPlayer(ctx context.Context, q string, args ...any) (domain.Player, error) {
	var p domain.Player
	err := s.pool.QueryRow(ctx, q, args...).Scan(&p.ID, &p.Email, &p.Username, &p.PasswordHash, &p.TotalXP, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, repo.ErrNotFound
	}
	return p, err
}

func (s *Store) AddTotalXP(ctx context.Context, playerID uuid.UUID, xp int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE players SET total_xp = total_xp + $2 WHERE id = $1`, playerID, xp)
	return err
}

func (s *Store) AddCompetencyXP(ctx context.Context, playerID uuid.UUID, competencyCode string, xp int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO player_competencies (player_id, competency_id, xp)
		 SELECT $1, id, $3 FROM competencies WHERE code = $2
		 ON CONFLICT (player_id, competency_id) DO UPDATE SET xp = player_competencies.xp + EXCLUDED.xp`,
		playerID, competencyCode, xp)
	return err
}

func (s *Store) ListCompetencies(ctx context.Context) ([]domain.Competency, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, code, name FROM competencies ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Competency
	for rows.Next() {
		var c domain.Competency
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetPlayerCompetencies(ctx context.Context, playerID uuid.UUID) ([]domain.PlayerCompetency, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT pc.player_id, pc.competency_id, pc.xp
		 FROM player_competencies pc
		 WHERE pc.player_id = $1
		 ORDER BY pc.competency_id`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PlayerCompetency
	for rows.Next() {
		var pc domain.PlayerCompetency
		if err := rows.Scan(&pc.PlayerID, &pc.CompetencyID, &pc.XP); err != nil {
			return nil, err
		}
		out = append(out, pc)
	}
	return out, rows.Err()
}

func (s *Store) Leaderboard(ctx context.Context, limit int) ([]domain.Player, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, username, password_hash, total_xp, created_at
		 FROM players ORDER BY total_xp DESC, created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Player
	for rows.Next() {
		var p domain.Player
		if err := rows.Scan(&p.ID, &p.Email, &p.Username, &p.PasswordHash, &p.TotalXP, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
