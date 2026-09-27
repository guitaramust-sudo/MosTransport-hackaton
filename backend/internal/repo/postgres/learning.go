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

const lessonProgressColumns = `player_id, lesson_id, theory_pass, practice_session_id, practice_pass, practice_check_pass, completed_at, content_version`

func scanLessonProgress(p *domain.LessonProgress) []any {
	return []any{&p.PlayerID, &p.LessonID, &p.TheoryPass, &p.PracticeSessionID, &p.PracticePass, &p.PracticeCheckPass, &p.CompletedAt, &p.ContentVersion, &p.PracticePassCount}
}

func (s *Store) GetLessonProgress(ctx context.Context, playerID uuid.UUID, lessonID string) (domain.LessonProgress, error) {
	var p domain.LessonProgress
	err := s.pool.QueryRow(ctx,
		`SELECT `+lessonProgressColumns+`,
		 (SELECT COUNT(*) FROM lesson_practice_passes WHERE player_id = $1 AND lesson_id = $2)
		 FROM lesson_progress WHERE player_id = $1 AND lesson_id = $2`, playerID, lessonID,
	).Scan(scanLessonProgress(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, repo.ErrNotFound
	}
	return p, err
}

func (s *Store) RecordPassedLessonPractice(ctx context.Context, playerID uuid.UUID, lessonID string, sessionID uuid.UUID) (int, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lesson_practice_passes (player_id, lesson_id, session_id) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`, playerID, lessonID, sessionID)
	if err != nil {
		return 0, err
	}
	var count int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lesson_practice_passes WHERE player_id = $1 AND lesson_id = $2`, playerID, lessonID).Scan(&count)
	return count, err
}

// ListLessonProgressByPlayer returns every lesson_progress row for playerID,
// across all lessons they have touched (in no particular order — callers
// that need a specific lesson map/join it themselves).
func (s *Store) ListLessonProgressByPlayer(ctx context.Context, playerID uuid.UUID) ([]domain.LessonProgress, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+lessonProgressColumns+` FROM lesson_progress WHERE player_id = $1`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LessonProgress
	for rows.Next() {
		var p domain.LessonProgress
		if err := rows.Scan(scanLessonProgress(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const lessonAwardColumns = `player_id, lesson_id, award_type, xp_delta, badge_id, awarded_at`

// ListLessonAwardsByPlayer returns every lesson_awards row for playerID.
func (s *Store) ListLessonAwardsByPlayer(ctx context.Context, playerID uuid.UUID) ([]domain.LessonAward, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+lessonAwardColumns+` FROM lesson_awards WHERE player_id = $1`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LessonAward
	for rows.Next() {
		var a domain.LessonAward
		var badge *string
		row := []any{&a.PlayerID, &a.LessonID, &a.AwardType, &a.XPDelta, &badge, &a.AwardedAt}
		if err := rows.Scan(row...); err != nil {
			return nil, err
		}
		if badge != nil {
			a.BadgeID = *badge
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpsertLessonProgress fully replaces the (player_id, lesson_id) row.
func (s *Store) UpsertLessonProgress(ctx context.Context, p domain.LessonProgress) error {
	if p.ContentVersion == "" {
		p.ContentVersion = "1.0.0"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO lesson_progress (player_id, lesson_id, theory_pass, practice_session_id, practice_pass, practice_check_pass, completed_at, content_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (player_id, lesson_id) DO UPDATE SET
			theory_pass = EXCLUDED.theory_pass,
			practice_session_id = EXCLUDED.practice_session_id,
			practice_pass = EXCLUDED.practice_pass,
			practice_check_pass = EXCLUDED.practice_check_pass,
			completed_at = EXCLUDED.completed_at,
			content_version = EXCLUDED.content_version`,
		p.PlayerID, p.LessonID, p.TheoryPass, p.PracticeSessionID, p.PracticePass, p.PracticeCheckPass, p.CompletedAt, p.ContentVersion)
	return err
}

func (s *Store) RecordLessonAnswer(ctx context.Context, playerID uuid.UUID, lessonID, questionID, optionID string, correct bool) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lesson_answers (player_id, lesson_id, question_id, option_id, correct) VALUES ($1, $2, $3, $4, $5)`,
		playerID, lessonID, questionID, optionID, correct)
	return err
}

func (s *Store) HasCorrectLessonAnswer(ctx context.Context, playerID uuid.UUID, lessonID, questionID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM lesson_answers WHERE player_id = $1 AND lesson_id = $2 AND question_id = $3 AND correct)`,
		playerID, lessonID, questionID).Scan(&exists)
	return exists, err
}

// AwardLessonCompletion is the single place that both inserts the
// lesson_awards row AND (only if that insert actually happened) bumps
// players.total_xp, inside one transaction — mirroring
// FinishSessionAndAwardXP in session.go. This guarantees the award record
// and the XP bump can never diverge: there is no world where the award row
// exists but XP wasn't applied, or vice versa. The lesson_awards primary key
// (player_id, lesson_id, award_type) is what makes a second attempt for the
// same lesson a no-op at the database level, not an application-level
// check-then-act race.
func (s *Store) AwardLessonCompletion(ctx context.Context, playerID uuid.UUID, lessonID string, xpDelta int, badgeID string) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var badge *string
	if badgeID != "" {
		badge = &badgeID
	}
	tag, err := tx.Exec(ctx,
		`INSERT INTO lesson_awards (player_id, lesson_id, award_type, xp_delta, badge_id) VALUES ($1, $2, 'completion', $3, $4)
		 ON CONFLICT (player_id, lesson_id, award_type) DO NOTHING`,
		playerID, lessonID, xpDelta, badge)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		// Already awarded on some earlier call: no-op, don't double-apply XP.
		return false, nil
	}
	result, err := tx.Exec(ctx, `UPDATE players SET total_xp = total_xp + $2 WHERE id = $1`, playerID, xpDelta)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() != 1 {
		return false, repo.ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// AwardPrizeCredit is the prize-credit ledger's analog of
// AwardLessonCompletion, minus the second table update -- there is nothing
// else to keep transactionally in sync here, so a single INSERT with the
// (player_id, source_type, source_id) unique constraint is enough. A repeat
// award attempt (same player, same source) fails the unique constraint /
// hits DO NOTHING and RowsAffected()==0, never inserting a second row.
func (s *Store) AwardPrizeCredit(ctx context.Context, playerID uuid.UUID, sourceType, sourceID string, amount int, awardedAt, expiresAt time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO prize_credit_entries (player_id, source_type, source_id, amount, awarded_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (player_id, source_type, source_id) DO NOTHING`,
		playerID, sourceType, sourceID, amount, awardedAt, expiresAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// PrizeCreditBalance computes the player's currently-active prize-credit
// balance and the nearest expiry among still-active entries, both at read
// time -- there is no stored "expired" flag, so an entry whose expires_at
// has already passed simply never enters either aggregate. MIN() over zero
// matching rows returns SQL NULL, which scans cleanly into a nil
// *time.Time.
func (s *Store) PrizeCreditBalance(ctx context.Context, playerID uuid.UUID, now time.Time) (int, *time.Time, error) {
	var balance int
	var nextExpiry *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0), MIN(expires_at)
		 FROM prize_credit_entries
		 WHERE player_id = $1 AND expires_at > $2`,
		playerID, now).Scan(&balance, &nextExpiry)
	if err != nil {
		return 0, nil, err
	}
	return balance, nextExpiry, nil
}
