package push

import (
	"context"
	"log/slog"
)

// LogSender is the default Sender: it logs what would have been sent and
// always succeeds. Real delivery is blocked on mobile-side Expo/FCM setup
// (see package doc) — this keeps the rest of the system fully testable and
// demoable without it.
type LogSender struct{}

func NewLogSender() *LogSender { return &LogSender{} }

func (s *LogSender) Send(ctx context.Context, deviceToken, platform string, n Notification) error {
	slog.Info("push (mock delivery)", "device_token", deviceToken, "platform", platform, "title", n.Title, "body", n.Body)
	return nil
}

var _ Sender = (*LogSender)(nil)
