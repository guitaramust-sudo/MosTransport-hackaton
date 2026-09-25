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

func (s *Store) FinishSessionAndAwardXP(ctx context.Context, sessionID, playerID uuid.UUID, xp int, finishedAt time.Time) (bool, error) {
	var awardedID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`WITH finished AS (
		 UPDATE sessions SET status = 'finished', finished_at = $3
		 WHERE id = $1 AND player_id = $2 AND status = 'active' RETURNING player_id
		)
		UPDATE players SET total_xp = total_xp + $4
		WHERE id = $2 AND EXISTS (SELECT 1 FROM finished WHERE player_id = $2)
		RETURNING id`, sessionID, playerID, finishedAt, xp).Scan(&awardedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) CreateSituation(ctx context.Context, sit domain.Situation) (domain.Situation, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO situations (session_id, status, passenger_params, loyalty, safety, timer_deadline, situation_def_id, passenger_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+situationColumns,
		sit.SessionID, sit.Status, sit.PassengerParams, sit.Loyalty, sit.Safety, sit.TimerDeadline, sit.SituationDefID, sit.PassengerID,
	).Scan(situationScan(&sit)...)
	return sit, err
}

const situationColumns = `id, session_id, status, situation_def_id, passenger_id, passenger_params,
	escalations, remarks, score_result, xp, loyalty, safety, timer_deadline, outcome, closed_at, created_at`

func situationScan(sit *domain.Situation) []any {
	return []any{&sit.ID, &sit.SessionID, &sit.Status, &sit.SituationDefID, &sit.PassengerID,
		&sit.PassengerParams, &sit.Escalations, &sit.Remarks, &sit.ScoreResult, &sit.XP,
		&sit.Loyalty, &sit.Safety, &sit.TimerDeadline, &sit.Outcome, &sit.ClosedAt, &sit.CreatedAt}
}

func (s *Store) GetSituation(ctx context.Context, id uuid.UUID) (domain.Situation, error) {
	var sit domain.Situation
	err := s.pool.QueryRow(ctx,
		`SELECT `+situationColumns+` FROM situations WHERE id = $1`, id,
	).Scan(situationScan(&sit)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return sit, repo.ErrNotFound
	}
	return sit, err
}

func (s *Store) ListSituationsBySession(ctx context.Context, sessionID uuid.UUID) ([]domain.Situation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+situationColumns+` FROM situations WHERE session_id = $1 ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Situation
	for rows.Next() {
		var sit domain.Situation
		if err := rows.Scan(situationScan(&sit)...); err != nil {
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

func (s *Store) CloseSituation(ctx context.Context, sit domain.Situation) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE situations SET status = 'closed', loyalty = $2, safety = $3, outcome = $4,
		 remarks = $5, score_result = $6, xp = $7, closed_at = $8
		 WHERE id = $1 AND status = 'active'`,
		sit.ID, sit.Loyalty, sit.Safety, sit.Outcome, sit.Remarks, sit.ScoreResult, sit.XP, sit.ClosedAt)
	return err == nil && tag.RowsAffected() == 1, err
}

func (s *Store) AddEscalation(ctx context.Context, situationID uuid.UUID, target string) ([]string, error) {
	var actual []string
	err := s.pool.QueryRow(ctx,
		`UPDATE situations
		 SET escalations = CASE WHEN escalations ? $2 THEN escalations ELSE escalations || to_jsonb($2::text) END
		 WHERE id = $1 AND status = 'active' RETURNING escalations`, situationID, target).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	return actual, err
}

func (s *Store) ListExpiredSituationIDs(ctx context.Context, now time.Time) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM situations WHERE status = 'active' AND timer_deadline <= $1 ORDER BY timer_deadline LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
