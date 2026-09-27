package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestAwardLessonPrizeGrantsOnceOnRepeat verifies the +10 lesson-completion
// prize credit is granted on first call and is a silent no-op (granted=false,
// no new entry, no error) on a repeat call for the same lesson -- the same
// idempotency shape as lesson XP, enforced here by fakeLearningStore's
// unique-key map (mirroring the DB's unique constraint in production).
func TestAwardLessonPrizeGrantsOnceOnRepeat(t *testing.T) {
	store := newFakeLearningStore()
	svc := NewPrizeService(store)
	ctx := context.Background()
	playerID := uuid.New()

	granted, err := svc.AwardLessonPrize(ctx, playerID, "B01", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("expected first award for B01 to be granted")
	}

	granted, err = svc.AwardLessonPrize(ctx, playerID, "B01", 10)
	if err != nil {
		t.Fatal(err)
	}
	if granted {
		t.Fatal("expected repeat award for the same lesson to be a no-op")
	}

	balance, err := svc.Balance(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Balance != 10 {
		t.Fatalf("expected balance=10 after one grant and one no-op repeat, got %d", balance.Balance)
	}
}

// TestBalanceExcludesExpiredEntries constructs one already-expired entry and
// one still-active entry directly in the fake store, and asserts Balance
// sums only the active one and reports its ExpiresAt as the nearest expiry
// -- never the expired entry's, even though nothing here makes the expired
// entry's absolute expiry time later than the active one's.
func TestBalanceExcludesExpiredEntries(t *testing.T) {
	store := newFakeLearningStore()
	svc := NewPrizeService(store)
	ctx := context.Background()
	playerID := uuid.New()
	now := time.Now()

	// Expired entry: awarded long ago, already past its 6x24h expiry.
	if _, err := store.AwardPrizeCredit(ctx, playerID, "lesson_completion", "B01", 10, now.Add(-10*24*time.Hour), now.Add(-4*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Active entry: expires a couple of hours from now.
	activeExpiry := now.Add(2 * time.Hour)
	if _, err := store.AwardPrizeCredit(ctx, playerID, "lesson_completion", "B02", 10, now, activeExpiry); err != nil {
		t.Fatal(err)
	}

	balance, err := svc.Balance(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Balance != 10 {
		t.Fatalf("expected balance=10 (only the active entry), got %d", balance.Balance)
	}
	if balance.NextExpiryAt == nil || !balance.NextExpiryAt.Equal(activeExpiry) {
		t.Fatalf("expected next expiry to be the active entry's expiry (%v), got %v", activeExpiry, balance.NextExpiryAt)
	}
}

// TestShirtProgressCapsAtOne verifies ShirtProgress reaches exactly 1.0 once
// balance reaches the 50-point threshold, and never exceeds 1.0 even when
// balance overshoots it (e.g. the demo seed's +40 stacked with real lesson
// credits).
func TestShirtProgressCapsAtOne(t *testing.T) {
	store := newFakeLearningStore()
	svc := NewPrizeService(store)
	ctx := context.Background()
	playerID := uuid.New()
	now := time.Now()

	// 9 entries of 10 = 90, well past the 50-point threshold.
	for i := 0; i < 9; i++ {
		sourceID := uuid.NewString()
		if _, err := store.AwardPrizeCredit(ctx, playerID, "lesson_completion", sourceID, 10, now, now.Add(6*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

	balance, err := svc.Balance(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Balance != 90 {
		t.Fatalf("expected raw balance=90, got %d", balance.Balance)
	}
	if balance.ShirtProgress != 1.0 {
		t.Fatalf("expected shirt_progress capped at 1.0, got %v", balance.ShirtProgress)
	}
}

// TestSeedDemoEntryIsIdempotent verifies the fixed demo-seed entry (+40,
// awarded_at=now-5d, expires_at=now+24h) is granted once and a repeat call
// is a no-op, exactly like a real lesson-completion award.
func TestSeedDemoEntryIsIdempotent(t *testing.T) {
	store := newFakeLearningStore()
	svc := NewPrizeService(store)
	ctx := context.Background()
	playerID := uuid.New()

	granted, err := svc.SeedDemoEntry(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("expected first demo-seed call to be granted")
	}

	granted, err = svc.SeedDemoEntry(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if granted {
		t.Fatal("expected repeat demo-seed call to be a no-op")
	}

	balance, err := svc.Balance(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Balance != 40 {
		t.Fatalf("expected balance=40 from the single demo-seed grant, got %d", balance.Balance)
	}
}
