package service

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/push"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

// fakePushStore implements repo.Store by embedding it as a nil interface
// field, same pattern as fakeLearningStore in learning_test.go: only the
// handful of methods PushService actually exercises need real bodies.
type fakePushStore struct {
	repo.Store

	mu      sync.Mutex
	subs    map[uuid.UUID][]domain.PushSubscription
	sent    map[string]bool
	entries []domain.PrizeCreditEntry
	nextID  int64
}

func newFakePushStore() *fakePushStore {
	return &fakePushStore{
		subs: map[uuid.UUID][]domain.PushSubscription{},
		sent: map[string]bool{},
	}
}

func (f *fakePushStore) RegisterPushSubscription(_ context.Context, playerID uuid.UUID, platform, deviceToken string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Mirror the postgres upsert-on-device_token behavior: a token belongs
	// to exactly one player, so remove it from any other player's list
	// before appending it to playerID's.
	for pid, list := range f.subs {
		filtered := make([]domain.PushSubscription, 0, len(list))
		for _, s := range list {
			if s.DeviceToken != deviceToken {
				filtered = append(filtered, s)
			}
		}
		f.subs[pid] = filtered
	}
	f.subs[playerID] = append(f.subs[playerID], domain.PushSubscription{
		PlayerID: playerID, Platform: platform, DeviceToken: deviceToken, CreatedAt: time.Now(),
	})
	return nil
}

func (f *fakePushStore) ListPushSubscriptions(_ context.Context, playerID uuid.UUID) ([]domain.PushSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.PushSubscription(nil), f.subs[playerID]...), nil
}

func (f *fakePushStore) MarkPushNotificationSent(_ context.Context, playerID uuid.UUID, eventType, sourceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := playerID.String() + "|" + eventType + "|" + sourceID
	if f.sent[key] {
		return false, nil
	}
	f.sent[key] = true
	return true, nil
}

func (f *fakePushStore) ListExpiringPrizeCreditsUnnotified(_ context.Context, within time.Duration, now time.Time) ([]domain.PrizeCreditEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	until := now.Add(within)
	var out []domain.PrizeCreditEntry
	for _, e := range f.entries {
		if !(e.ExpiresAt.After(now) && !e.ExpiresAt.After(until)) {
			continue
		}
		key := e.PlayerID.String() + "|prize_credit_expiring|" + strconv.FormatInt(e.ID, 10)
		if f.sent[key] {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// addPrizeEntry hand-inserts a prize_credit_entries-shaped row directly into
// the fake store, mirroring the DB-gated test's raw SQL inserts, and returns
// the assigned id.
func (f *fakePushStore) addPrizeEntry(playerID uuid.UUID, expiresAt time.Time) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.entries = append(f.entries, domain.PrizeCreditEntry{
		ID: f.nextID, PlayerID: playerID, SourceType: "lesson_completion", SourceID: "B01",
		Amount: 10, AwardedAt: time.Now(), ExpiresAt: expiresAt,
	})
	return f.nextID
}

// fakeSender is a tiny push.Sender that records every call it receives,
// used instead of push.LogSender so tests can assert exactly how many times
// (and with what) Send was invoked.
type fakeSender struct {
	mu    sync.Mutex
	sends []push.Notification
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sends)
}

func (f *fakeSender) Send(_ context.Context, _, _ string, n push.Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, n)
	return nil
}

var _ push.Sender = (*fakeSender)(nil)

// TestNotifyLessonUnlockedSendsOnceOnRepeat verifies that calling
// NotifyLessonUnlocked twice for the same (player, lessonID) only ever
// results in one call to the Sender -- dedup is enforced by
// MarkPushNotificationSent's DB-level unique constraint (mirrored here by
// fakePushStore's map), not an application-level check-then-act.
func TestNotifyLessonUnlockedSendsOnceOnRepeat(t *testing.T) {
	store := newFakePushStore()
	sender := &fakeSender{}
	svc := NewPushService(store, sender)
	ctx := context.Background()
	playerID := uuid.New()

	if err := store.RegisterPushSubscription(ctx, playerID, "android", "tok-1"); err != nil {
		t.Fatal(err)
	}

	if err := svc.NotifyLessonUnlocked(ctx, playerID, "B02"); err != nil {
		t.Fatal(err)
	}
	if err := svc.NotifyLessonUnlocked(ctx, playerID, "B02"); err != nil {
		t.Fatal(err)
	}

	if got := sender.count(); got != 1 {
		t.Fatalf("expected exactly 1 send after two NotifyLessonUnlocked calls for the same lesson, got %d", got)
	}
	// The push text must never contain lesson content, only a generic title.
	sender.mu.Lock()
	notif := sender.sends[0]
	sender.mu.Unlock()
	if notif.Title == "" || notif.Data["lesson_id"] != "B02" {
		t.Fatalf("unexpected notification shape: %+v", notif)
	}
}

// TestSweepExpiringPrizeCreditsOnlyNotifiesNearExpiry constructs one entry
// expiring in 12h and one expiring in 48h, and verifies only the near one
// (inside the 24h window) triggers a send; a second sweep call must never
// double-send for the entry already notified.
func TestSweepExpiringPrizeCreditsOnlyNotifiesNearExpiry(t *testing.T) {
	store := newFakePushStore()
	sender := &fakeSender{}
	svc := NewPushService(store, sender)
	ctx := context.Background()

	nearPlayer := uuid.New()
	farPlayer := uuid.New()
	if err := store.RegisterPushSubscription(ctx, nearPlayer, "ios", "tok-near"); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterPushSubscription(ctx, farPlayer, "ios", "tok-far"); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	store.addPrizeEntry(nearPlayer, now.Add(12*time.Hour))
	store.addPrizeEntry(farPlayer, now.Add(48*time.Hour))

	if err := svc.SweepExpiringPrizeCredits(ctx); err != nil {
		t.Fatal(err)
	}
	if got := sender.count(); got != 1 {
		t.Fatalf("expected exactly 1 send for the entry expiring in 12h, got %d", got)
	}

	// A second sweep pass must not double-send for the entry already
	// notified (it's still active and still within the window).
	if err := svc.SweepExpiringPrizeCredits(ctx); err != nil {
		t.Fatal(err)
	}
	if got := sender.count(); got != 1 {
		t.Fatalf("expected sweep to remain idempotent, still 1 send, got %d", got)
	}
}
