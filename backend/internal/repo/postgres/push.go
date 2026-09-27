package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

// RegisterPushSubscription upserts on device_token: a token belongs to
// exactly one player at a time, so re-registering the same token for a
// different player (device changed hands, demo account re-registers)
// reassigns it rather than creating a duplicate row.
func (s *Store) RegisterPushSubscription(ctx context.Context, playerID uuid.UUID, platform, deviceToken string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (player_id, platform, device_token)
		VALUES ($1, $2, $3)
		ON CONFLICT (device_token) DO UPDATE SET
			player_id = EXCLUDED.player_id,
			platform = EXCLUDED.platform,
			created_at = now()`,
		playerID, platform, deviceToken)
	return err
}

// ListPushSubscriptions returns all of playerID's registered devices.
func (s *Store) ListPushSubscriptions(ctx context.Context, playerID uuid.UUID) ([]domain.PushSubscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT player_id, platform, device_token, created_at FROM push_subscriptions WHERE player_id = $1`,
		playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []domain.PushSubscription
	for rows.Next() {
		var sub domain.PushSubscription
		if err := rows.Scan(&sub.PlayerID, &sub.Platform, &sub.DeviceToken, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// MarkPushNotificationSent is the push-dedup idiom's third use in this
// session (after lesson_awards and prize_credit_entries): an atomic
// INSERT ... ON CONFLICT DO NOTHING keyed on the exact (player_id,
// event_type, source_id) dedup key, never an application-level
// check-then-act. RowsAffected()==1 iff this call actually inserted.
func (s *Store) MarkPushNotificationSent(ctx context.Context, playerID uuid.UUID, eventType, sourceID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO push_notifications_sent (player_id, event_type, source_id) VALUES ($1, $2, $3)
		 ON CONFLICT (player_id, event_type, source_id) DO NOTHING`,
		playerID, eventType, sourceID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ListExpiringPrizeCreditsUnnotified returns every active prize_credit_entries
// row whose expires_at falls within `within` of `now` (and is still in the
// future) that has no matching push_notifications_sent row for
// event_type="prize_credit_expiring" -- the anti-join makes this the single
// source of truth for "who still needs the expiry push", not an
// application-level check-then-act.
func (s *Store) ListExpiringPrizeCreditsUnnotified(ctx context.Context, within time.Duration, now time.Time) ([]domain.PrizeCreditEntry, error) {
	// The window's far edge is computed in Go (now.Add(within)) and passed as
	// a plain timestamp, rather than passing a Go time.Duration to Postgres
	// as an INTERVAL -- same style as AwardPrizeCredit's awardedAt/expiresAt,
	// which are also computed in the caller, not derived from a duration
	// inside SQL.
	until := now.Add(within)
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.player_id, e.source_type, e.source_id, e.amount, e.awarded_at, e.expires_at
		FROM prize_credit_entries e
		WHERE e.expires_at > $1 AND e.expires_at <= $2
		AND NOT EXISTS (
			SELECT 1 FROM push_notifications_sent s
			WHERE s.player_id = e.player_id AND s.event_type = 'prize_credit_expiring' AND s.source_id = e.id::text
		)`,
		now, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.PrizeCreditEntry
	for rows.Next() {
		var e domain.PrizeCreditEntry
		if err := rows.Scan(&e.ID, &e.PlayerID, &e.SourceType, &e.SourceID, &e.Amount, &e.AwardedAt, &e.ExpiresAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
