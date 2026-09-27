package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/push"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

// PushService drives the two server-triggered push notifications
// (lesson_unlocked, prize_credit_expiring) behind the push.Sender seam: it
// registers devices, dedups sends at the DB level (never an
// application-level check-then-act), and fans a notification out to all of
// a player's registered devices. The default push.Sender is push.LogSender
// -- real Expo/FCM delivery is a follow-up once the mobile team has
// credentials (see internal/push's package doc).
type PushService struct {
	store  repo.Store
	sender push.Sender
}

func NewPushService(store repo.Store, sender push.Sender) *PushService {
	return &PushService{store: store, sender: sender}
}

// RegisterDevice records or reassigns a device token for playerID.
func (p *PushService) RegisterDevice(ctx context.Context, playerID uuid.UUID, platform, deviceToken string) error {
	return p.store.RegisterPushSubscription(ctx, playerID, platform, deviceToken)
}

// NotifyLessonUnlocked sends a generic "new lesson unlocked" push at most
// once per (player, unlockedLessonID) -- dedup is enforced by the
// push_notifications_sent (player_id, event_type, source_id) unique
// constraint via MarkPushNotificationSent, not an application-level
// check-then-act. The push text never names the lesson's content, only that
// something new is available, matching the design doc's "текст не
// раскрывает скрытую ситуацию" rule (there is no hidden situation here at
// all, but the same discipline applies: keep it generic).
func (p *PushService) NotifyLessonUnlocked(ctx context.Context, playerID uuid.UUID, unlockedLessonID string) error {
	sent, err := p.store.MarkPushNotificationSent(ctx, playerID, "lesson_unlocked", unlockedLessonID)
	if err != nil || !sent {
		return err
	}
	return p.sendToAllDevices(ctx, playerID, push.Notification{
		Title: "Открылся новый урок",
		Body:  "В обучении доступен следующий урок.",
		Data:  map[string]string{"screen": "learning_map", "lesson_id": unlockedLessonID},
	})
}

// SweepExpiringPrizeCredits notifies about every active prize entry expiring
// within the next 24h that hasn't been notified yet. Call this periodically
// (see cmd/server/main.go's background ticker), not per-request. Dedup is
// enforced by the same push_notifications_sent unique constraint, keyed on
// the prize-credit entry's own id (as text) as source_id -- so a second
// sweep pass over the same still-unexpired entry never sends twice.
func (p *PushService) SweepExpiringPrizeCredits(ctx context.Context) error {
	entries, err := p.store.ListExpiringPrizeCreditsUnnotified(ctx, 24*time.Hour, time.Now())
	if err != nil {
		return err
	}
	for _, e := range entries {
		sent, err := p.store.MarkPushNotificationSent(ctx, e.PlayerID, "prize_credit_expiring", strconv.FormatInt(e.ID, 10))
		if err != nil {
			slog.Error("prize expiry push dedup failed", "entry_id", e.ID, "error", err)
			continue
		}
		if !sent {
			continue
		}
		if err := p.sendToAllDevices(ctx, e.PlayerID, push.Notification{
			Title: "Призовые баллы скоро истекут",
			Body:  "Часть призового баланса истечёт в течение суток.",
			Data:  map[string]string{"screen": "prize_balance"},
		}); err != nil {
			slog.Error("prize expiry push send failed", "player_id", e.PlayerID, "error", err)
		}
	}
	return nil
}

// sendToAllDevices sends n to every device playerID has registered. A
// single device-send failure is logged and does not stop delivery to the
// player's other devices, nor does it propagate as an error that would
// abort a sweep batch or the caller's primary success path.
func (p *PushService) sendToAllDevices(ctx context.Context, playerID uuid.UUID, n push.Notification) error {
	subs, err := p.store.ListPushSubscriptions(ctx, playerID)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if err := p.sender.Send(ctx, sub.DeviceToken, sub.Platform, n); err != nil {
			slog.Error("push send failed", "player_id", playerID, "device_token", sub.DeviceToken, "error", err)
		}
	}
	return nil
}
