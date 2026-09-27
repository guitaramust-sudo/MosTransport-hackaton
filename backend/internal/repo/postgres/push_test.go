package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestRegisterPushSubscriptionUpsertsOnDeviceTokenConflict verifies a device
// token belongs to exactly one player at a time: re-registering the same
// token for a different player reassigns it (the old player's list no
// longer contains it, the new player's list does), rather than creating a
// second row.
func TestRegisterPushSubscriptionUpsertsOnDeviceTokenConflict(t *testing.T) {
	store, playerA := integrationStore(t)
	ctx := context.Background()
	playerB, err := store.CreatePlayer(ctx, fmt.Sprintf("push-b-%s@example.com", uuid.NewString()), "push-test-b", "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM push_subscriptions WHERE player_id IN ($1, $2)`, playerA, playerB.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, playerB.ID)
	})

	deviceToken := "shared-device-" + uuid.NewString()
	if err := store.RegisterPushSubscription(ctx, playerA, "android", deviceToken); err != nil {
		t.Fatal(err)
	}
	subsA, err := store.ListPushSubscriptions(ctx, playerA)
	if err != nil {
		t.Fatal(err)
	}
	if len(subsA) != 1 {
		t.Fatalf("expected playerA to have 1 subscription after registering, got %d", len(subsA))
	}

	// Re-register the SAME token for playerB (device changed hands).
	if err := store.RegisterPushSubscription(ctx, playerB.ID, "ios", deviceToken); err != nil {
		t.Fatal(err)
	}

	subsA, err = store.ListPushSubscriptions(ctx, playerA)
	if err != nil {
		t.Fatal(err)
	}
	if len(subsA) != 0 {
		t.Fatalf("expected playerA to have 0 subscriptions after the token was reassigned, got %d", len(subsA))
	}
	subsB, err := store.ListPushSubscriptions(ctx, playerB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subsB) != 1 || subsB[0].DeviceToken != deviceToken || subsB[0].Platform != "ios" {
		t.Fatalf("expected playerB to now own the reassigned token, got %+v", subsB)
	}
}

// TestMarkPushNotificationSentIsIdempotent verifies the
// (player_id, event_type, source_id) unique constraint really does make a
// second dedup attempt for the same triple a no-op at the DB level.
func TestMarkPushNotificationSentIsIdempotent(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM push_notifications_sent WHERE player_id = $1`, playerID)
	})

	sent, err := store.MarkPushNotificationSent(ctx, playerID, "lesson_unlocked", "B02")
	if err != nil {
		t.Fatal(err)
	}
	if !sent {
		t.Fatal("first mark should report sent=true")
	}

	sent, err = store.MarkPushNotificationSent(ctx, playerID, "lesson_unlocked", "B02")
	if err != nil {
		t.Fatal(err)
	}
	if sent {
		t.Fatal("second mark for the same (player, event_type, source_id) should be a no-op")
	}

	var count int
	if err := store.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM push_notifications_sent WHERE player_id = $1 AND event_type = 'lesson_unlocked' AND source_id = 'B02'`,
		playerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row after the repeat mark attempt, got %d", count)
	}
}

// TestListExpiringPrizeCreditsUnnotified hand-inserts three
// prize_credit_entries rows -- one expiring in 12h, one in 48h, and one
// expiring in 12h but already marked notified -- and verifies only the
// first (near expiry, not yet notified) comes back.
func TestListExpiringPrizeCreditsUnnotified(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM push_notifications_sent WHERE player_id = $1`, playerID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM prize_credit_entries WHERE player_id = $1`, playerID)
	})

	now := time.Now()
	insert := func(sourceID string, expiresAt time.Time) int64 {
		var id int64
		if err := store.pool.QueryRow(ctx,
			`INSERT INTO prize_credit_entries (player_id, source_type, source_id, amount, awarded_at, expires_at)
			 VALUES ($1, 'lesson_completion', $2, 10, $3, $4) RETURNING id`,
			playerID, sourceID, now, expiresAt).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	nearID := insert("B01", now.Add(12*time.Hour))
	_ = insert("B02", now.Add(48*time.Hour))
	notifiedID := insert("B03", now.Add(12*time.Hour))

	if _, err := store.pool.Exec(ctx,
		`INSERT INTO push_notifications_sent (player_id, event_type, source_id) VALUES ($1, 'prize_credit_expiring', $2)`,
		playerID, fmt.Sprintf("%d", notifiedID)); err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListExpiringPrizeCreditsUnnotified(ctx, 24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 unnotified near-expiry entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].ID != nearID {
		t.Fatalf("expected the near-expiry entry (id=%d), got id=%d", nearID, entries[0].ID)
	}
}
