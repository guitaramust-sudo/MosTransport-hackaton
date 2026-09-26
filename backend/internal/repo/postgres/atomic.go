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

// CreateSessionWithSituations commits the shift, the pending situation queue
// and the initial situations together. A failure leaves no partial shift behind.
func (s *Store) CreateSessionWithSituations(ctx context.Context, playerID uuid.UUID, pending []string, drafts []domain.Situation) (domain.Session, []domain.Situation, error) {
	if len(drafts) == 0 {
		return domain.Session{}, nil, fmt.Errorf("session requires situations")
	}
	if pending == nil {
		pending = []string{}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Session{}, nil, err
	}
	defer tx.Rollback(ctx)
	var sess domain.Session
	err = tx.QueryRow(ctx,
		`INSERT INTO sessions (player_id, pending_situations) VALUES ($1, $2)
		 RETURNING `+sessionColumns, playerID, pending,
	).Scan(scanSession(&sess)...)
	if err != nil {
		return domain.Session{}, nil, err
	}
	created := make([]domain.Situation, 0, len(drafts))
	for _, draft := range drafts {
		draft.SessionID = sess.ID
		err := tx.QueryRow(ctx,
			`INSERT INTO situations (session_id, status, passenger_params, loyalty, safety, timer_deadline, situation_def_id, passenger_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+situationColumns,
			draft.SessionID, draft.Status, draft.PassengerParams, draft.Loyalty, draft.Safety,
			draft.TimerDeadline, draft.SituationDefID, draft.PassengerID,
		).Scan(situationScan(&draft)...)
		if err != nil {
			return domain.Session{}, nil, err
		}
		opening, _ := draft.PassengerParams["opening"].(string)
		if _, err := tx.Exec(ctx,
			`INSERT INTO messages (situation_id, role, content) VALUES ($1, 'system', $2)`, draft.ID, opening); err != nil {
			return domain.Session{}, nil, err
		}
		created = append(created, draft)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Session{}, nil, err
	}
	return sess, created, nil
}

// AppendTurn checks the session and deadline under row locks, then writes both
// dialog messages and any text-detected escalation in one transaction.
func (s *Store) AppendTurn(ctx context.Context, situationID, playerID uuid.UUID, text, reply string, targets []string, inputMode *string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var owner uuid.UUID
	var sessionStatus, situationStatus string
	var deadline *time.Time
	err = tx.QueryRow(ctx,
		`SELECT sess.player_id, sess.status, sit.status, sit.timer_deadline
		 FROM situations AS sit JOIN sessions AS sess ON sess.id = sit.session_id
		 WHERE sit.id = $1 FOR UPDATE OF sess, sit`, situationID,
	).Scan(&owner, &sessionStatus, &situationStatus, &deadline)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != playerID) {
		return 0, repo.ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if sessionStatus != domain.SessionStatusActive || situationStatus != domain.SituationStatusActive {
		return 0, repo.ErrConflict
	}
	if deadline != nil && !time.Now().Before(*deadline) {
		return 0, repo.ErrDeadlineExceeded
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO messages (situation_id, role, content, input_mode) VALUES ($1, 'player', $2, $3)`, situationID, text, inputMode); err != nil {
		return 0, err
	}
	for _, target := range targets {
		if _, err := tx.Exec(ctx,
			`UPDATE situations
			 SET escalations = CASE WHEN escalations ? $2 THEN escalations ELSE escalations || to_jsonb($2::text) END
			 WHERE id = $1`, situationID, target); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO messages (situation_id, role, content) VALUES ($1, 'passenger', $2)`, situationID, reply); err != nil {
		return 0, err
	}
	var turnCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE situation_id = $1 AND role = 'player'`, situationID).Scan(&turnCount); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return turnCount, nil
}

// RecordEscalation updates the deduplicated target list and audit message in
// one transaction, after checking ownership, status and deadline under locks.
func (s *Store) RecordEscalation(ctx context.Context, situationID, playerID uuid.UUID, target string) ([]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var owner uuid.UUID
	var sessionStatus, situationStatus string
	var deadline *time.Time
	var actual []string
	err = tx.QueryRow(ctx,
		`SELECT sess.player_id, sess.status, sit.status, sit.timer_deadline, sit.escalations
		 FROM situations AS sit JOIN sessions AS sess ON sess.id = sit.session_id
		 WHERE sit.id = $1 FOR UPDATE OF sess, sit`, situationID,
	).Scan(&owner, &sessionStatus, &situationStatus, &deadline, &actual)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != playerID) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if sessionStatus != domain.SessionStatusActive || situationStatus != domain.SituationStatusActive {
		return nil, repo.ErrConflict
	}
	if deadline != nil && !time.Now().Before(*deadline) {
		return nil, repo.ErrDeadlineExceeded
	}
	if !stringIn(actual, target) {
		err = tx.QueryRow(ctx,
			`UPDATE situations SET escalations = escalations || to_jsonb($2::text)
			 WHERE id = $1 RETURNING escalations`, situationID, target).Scan(&actual)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO messages (situation_id, role, content, kind)
			 VALUES ($1, 'system', $2, 'escalation')`, situationID, "Вызван адресат: "+target); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return actual, nil
}

// SpawnNextSituation atomically pops the next pending scenario and creates its
// situation, provided the session is still active and no other situation is
// running. It returns nil when there is nothing left to spawn.
func (s *Store) SpawnNextSituation(ctx context.Context, sessionID uuid.UUID, resolve func(scenarioID string) (domain.Situation, error)) (*domain.Situation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	var pending []string
	err = tx.QueryRow(ctx,
		`SELECT status, pending_situations FROM sessions WHERE id = $1 FOR UPDATE`, sessionID,
	).Scan(&status, &pending)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var active int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM situations WHERE session_id = $1 AND status = 'active'`, sessionID,
	).Scan(&active); err != nil {
		return nil, err
	}
	if status != domain.SessionStatusActive || active > 0 || len(pending) == 0 {
		return nil, nil
	}

	scenarioID := pending[0]
	pending = pending[1:]

	draft, err := resolve(scenarioID)
	if err != nil {
		return nil, err
	}
	draft.SessionID = sessionID

	err = tx.QueryRow(ctx,
		`INSERT INTO situations (session_id, status, passenger_params, loyalty, safety, timer_deadline, situation_def_id, passenger_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+situationColumns,
		draft.SessionID, draft.Status, draft.PassengerParams, draft.Loyalty, draft.Safety,
		draft.TimerDeadline, draft.SituationDefID, draft.PassengerID,
	).Scan(situationScan(&draft)...)
	if err != nil {
		return nil, err
	}

	opening, _ := draft.PassengerParams["opening"].(string)
	if _, err := tx.Exec(ctx,
		`INSERT INTO messages (situation_id, role, content) VALUES ($1, 'system', $2)`, draft.ID, opening); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE sessions SET pending_situations = $2 WHERE id = $1`, sessionID, pending); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &draft, nil
}

func stringIn(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
