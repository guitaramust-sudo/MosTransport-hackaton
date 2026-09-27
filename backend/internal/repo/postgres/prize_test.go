package postgres

import (
	"context"
	"testing"
	"time"
)

// TestAwardPrizeCreditIsIdempotent verifies AwardPrizeCredit's unique
// constraint on (player_id, source_type, source_id) really does make a
// second award attempt for the same triple a no-op -- both the returned
// bool and the row count in prize_credit_entries, not merely an
// application-level check-then-act race.
func TestAwardPrizeCreditIsIdempotent(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM prize_credit_entries WHERE player_id = $1`, playerID)
	})

	now := time.Now()
	granted, err := store.AwardPrizeCredit(ctx, playerID, "lesson_completion", "B01", 10, now, now.Add(6*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("first award should be granted")
	}

	granted, err = store.AwardPrizeCredit(ctx, playerID, "lesson_completion", "B01", 10, now, now.Add(6*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if granted {
		t.Fatal("second award for the same (player, source_type, source_id) should be a no-op")
	}

	var count int
	if err := store.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prize_credit_entries WHERE player_id = $1 AND source_type = 'lesson_completion' AND source_id = 'B01'`,
		playerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row after the repeat award attempt, got %d", count)
	}
}

// TestPrizeCreditBalanceExcludesExpiredRows hand-inserts one expired row and
// one active row and verifies PrizeCreditBalance's SUM and MIN(expires_at)
// both only consider the row whose expires_at is still in the future.
func TestPrizeCreditBalanceExcludesExpiredRows(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM prize_credit_entries WHERE player_id = $1`, playerID)
	})

	now := time.Now()
	expiredAwardedAt := now.Add(-10 * 24 * time.Hour)
	expiredExpiresAt := now.Add(-4 * 24 * time.Hour)
	if _, err := store.pool.Exec(ctx,
		`INSERT INTO prize_credit_entries (player_id, source_type, source_id, amount, awarded_at, expires_at) VALUES ($1, 'lesson_completion', 'B01', 10, $2, $3)`,
		playerID, expiredAwardedAt, expiredExpiresAt); err != nil {
		t.Fatal(err)
	}
	// PostgreSQL timestamps keep microseconds, not Go's nanoseconds.
	activeExpiresAt := now.Add(2 * time.Hour).Truncate(time.Microsecond)
	if _, err := store.pool.Exec(ctx,
		`INSERT INTO prize_credit_entries (player_id, source_type, source_id, amount, awarded_at, expires_at) VALUES ($1, 'lesson_completion', 'B02', 10, $2, $3)`,
		playerID, now, activeExpiresAt); err != nil {
		t.Fatal(err)
	}

	balance, nextExpiry, err := store.PrizeCreditBalance(ctx, playerID, now)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("expected balance=10 (only the active row), got %d", balance)
	}
	if nextExpiry == nil {
		t.Fatal("expected a non-nil nextExpiry from the active row")
	}
	if !nextExpiry.Equal(activeExpiresAt) {
		t.Fatalf("expected nextExpiry=%v (active row), got %v", activeExpiresAt, *nextExpiry)
	}
}

// TestPrizeCreditBalanceNoActiveRows verifies an all-expired (or empty)
// history reports balance=0 and nextExpiry=nil, not a zero-value time.
func TestPrizeCreditBalanceNoActiveRows(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()

	balance, nextExpiry, err := store.PrizeCreditBalance(ctx, playerID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if balance != 0 {
		t.Fatalf("expected balance=0 with no entries, got %d", balance)
	}
	if nextExpiry != nil {
		t.Fatalf("expected nextExpiry=nil with no entries, got %v", *nextExpiry)
	}
}
