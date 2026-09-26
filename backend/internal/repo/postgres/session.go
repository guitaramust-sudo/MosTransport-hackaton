package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

const sessionColumns = `id, player_id, status, pending_situations, validation_status, created_at, finished_at`

func scanSession(sess *domain.Session) []any {
	return []any{&sess.ID, &sess.PlayerID, &sess.Status, &sess.PendingSituations, &sess.ValidationStatus, &sess.CreatedAt, &sess.FinishedAt}
}

func (s *Store) CreateSession(ctx context.Context, playerID uuid.UUID) (domain.Session, error) {
	var sess domain.Session
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sessions (player_id) VALUES ($1) RETURNING `+sessionColumns,
		playerID,
	).Scan(scanSession(&sess)...)
	return sess, err
}

func (s *Store) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	var sess domain.Session
	err := s.pool.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id,
	).Scan(scanSession(&sess)...)
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

func (s *Store) FinishSessionAndAwardXP(ctx context.Context, sessionID, playerID uuid.UUID, xp int, awards map[string]repo.CompetencyAward, expectedSituations, expectedPending int, finishedAt time.Time) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var status string
	var pending []string
	err = tx.QueryRow(ctx,
		`SELECT status, pending_situations FROM sessions WHERE id = $1 AND player_id = $2 FOR UPDATE`, sessionID, playerID).Scan(&status, &pending)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, repo.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if status != domain.SessionStatusActive {
		return false, nil
	}
	if len(pending) != expectedPending {
		return false, repo.ErrConflict
	}
	var activeCount, situationCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'active') FROM situations WHERE session_id = $1`, sessionID,
	).Scan(&situationCount, &activeCount); err != nil {
		return false, err
	}
	if activeCount > 0 || situationCount != expectedSituations {
		return false, repo.ErrConflict
	}
	var finishedID uuid.UUID
	err = tx.QueryRow(ctx,
		`UPDATE sessions SET status = 'finished', finished_at = $3
		 WHERE id = $1 AND player_id = $2 AND status = 'active'
		 RETURNING id`, sessionID, playerID, finishedAt).Scan(&finishedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(ctx, `UPDATE players SET total_xp = total_xp + $2 WHERE id = $1`, playerID, xp)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() != 1 {
		return false, repo.ErrNotFound
	}
	for code, award := range awards {
		result, err := tx.Exec(ctx,
			`INSERT INTO player_competencies (player_id, competency_id, xp, evidence_count)
			 SELECT $1, id, $3, $4 FROM competencies WHERE code = $2
			 ON CONFLICT (player_id, competency_id)
			 DO UPDATE SET xp = player_competencies.xp + EXCLUDED.xp,
			               evidence_count = player_competencies.evidence_count + EXCLUDED.evidence_count`,
			playerID, code, award.XP, award.Evidence)
		if err != nil {
			return false, err
		}
		if result.RowsAffected() != 1 {
			return false, fmt.Errorf("unknown competency %q", code)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) ApproveSession(ctx context.Context, id uuid.UUID) error {
	var approvedID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`UPDATE sessions SET validation_status = 'approved'
		 WHERE id = $1 AND status = 'finished' RETURNING id`, id).Scan(&approvedID)
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var status string
	err = s.pool.QueryRow(ctx, `SELECT status FROM sessions WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	return repo.ErrConflict
}

func (s *Store) ListPlayerSessions(ctx context.Context, playerID uuid.UUID) ([]domain.Session, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+sessionColumns+` FROM sessions WHERE player_id = $1 ORDER BY created_at DESC`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		var sess domain.Session
		if err := rows.Scan(scanSession(&sess)...); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
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
