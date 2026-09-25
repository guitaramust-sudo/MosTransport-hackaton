package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

func (s *Store) CreateSession(ctx context.Context, playerID uuid.UUID) (domain.Session, error) {
	var sess domain.Session
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sessions (player_id) VALUES ($1)
		 RETURNING id, player_id, status, created_at, finished_at`,
		playerID,
	).Scan(&sess.ID, &sess.PlayerID, &sess.Status, &sess.CreatedAt, &sess.FinishedAt)
	return sess, err
}

func (s *Store) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	var sess domain.Session
	err := s.pool.QueryRow(ctx,
		`SELECT id, player_id, status, created_at, finished_at FROM sessions WHERE id = $1`, id,
	).Scan(&sess.ID, &sess.PlayerID, &sess.Status, &sess.CreatedAt, &sess.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return sess, repo.ErrNotFound
	}
	return sess, err
}

func (s *Store) FinishSession(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE sessions SET status = 'finished', finished_at = $2 WHERE id = $1 AND status = 'active'`,
		id, finishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	return nil
}

func (s *Store) CreateSituation(ctx context.Context, sit domain.Situation) (domain.Situation, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO situations (session_id, status, passenger_params, loyalty, safety, timer_deadline)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, session_id, status, passenger_params, loyalty, safety, timer_deadline, outcome, created_at`,
		sit.SessionID, sit.Status, sit.PassengerParams, sit.Loyalty, sit.Safety, sit.TimerDeadline,
	).Scan(&sit.ID, &sit.SessionID, &sit.Status, &sit.PassengerParams, &sit.Loyalty, &sit.Safety, &sit.TimerDeadline, &sit.Outcome, &sit.CreatedAt)
	return sit, err
}

func (s *Store) GetSituation(ctx context.Context, id uuid.UUID) (domain.Situation, error) {
	var sit domain.Situation
	err := s.pool.QueryRow(ctx,
		`SELECT id, session_id, status, passenger_params, loyalty, safety, timer_deadline, outcome, created_at
		 FROM situations WHERE id = $1`, id,
	).Scan(&sit.ID, &sit.SessionID, &sit.Status, &sit.PassengerParams, &sit.Loyalty, &sit.Safety, &sit.TimerDeadline, &sit.Outcome, &sit.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return sit, repo.ErrNotFound
	}
	return sit, err
}

func (s *Store) ListSituationsBySession(ctx context.Context, sessionID uuid.UUID) ([]domain.Situation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, status, passenger_params, loyalty, safety, timer_deadline, outcome, created_at
		 FROM situations WHERE session_id = $1 ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Situation
	for rows.Next() {
		var sit domain.Situation
		if err := rows.Scan(&sit.ID, &sit.SessionID, &sit.Status, &sit.PassengerParams, &sit.Loyalty, &sit.Safety, &sit.TimerDeadline, &sit.Outcome, &sit.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sit)
	}
	return out, rows.Err()
}

func (s *Store) UpdateSituation(ctx context.Context, sit domain.Situation) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE situations
		 SET status = $2, loyalty = $3, safety = $4, outcome = $5
		 WHERE id = $1`,
		sit.ID, sit.Status, sit.Loyalty, sit.Safety, sit.Outcome)
	return err
}
