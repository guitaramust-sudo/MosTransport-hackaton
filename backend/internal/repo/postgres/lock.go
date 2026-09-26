package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

type lockedSituation struct {
	tx  pgx.Tx
	sit domain.Situation
}

func (l *lockedSituation) Situation() domain.Situation { return l.sit }

func (l *lockedSituation) ListMessages(ctx context.Context) ([]domain.Message, error) {
	rows, err := l.tx.Query(ctx,
		`SELECT `+messageColumns+` FROM messages WHERE situation_id = $1 ORDER BY id`, l.sit.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(scanMessage(&m)...); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (l *lockedSituation) Close(ctx context.Context, result domain.Situation) error {
	tag, err := l.tx.Exec(ctx,
		`UPDATE situations SET status = 'closed', loyalty = $2, safety = $3, outcome = $4,
		 remarks = $5, score_result = $6, xp = $7, closed_at = $8
		 WHERE id = $1 AND status = 'active'`,
		l.sit.ID, result.Loyalty, result.Safety, result.Outcome,
		result.Remarks, result.ScoreResult, result.XP, result.ClosedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return repo.ErrConflict
	}
	return nil
}

func (s *Store) WithSituationLock(ctx context.Context, id uuid.UUID, fn func(repo.LockedSituation) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var sit domain.Situation
	err = tx.QueryRow(ctx,
		`SELECT `+situationColumns+` FROM situations WHERE id = $1 FOR UPDATE`, id,
	).Scan(situationScan(&sit)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := fn(&lockedSituation{tx: tx, sit: sit}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
