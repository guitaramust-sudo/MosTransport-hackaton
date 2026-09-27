package push

import "context"

// Notification is one outbound push message. Data must never leak content
// the design doc says the player shouldn't see ahead of time (e.g. no
// hidden-situation details) — both call sites in this task only ever send
// generic titles/bodies, never situation-specific text, by construction.
type Notification struct {
	Title string
	Body  string
	Data  map[string]string // e.g. {"screen": "learning_map"} for client-side deep link routing
}

// Sender delivers one notification to one device token. A real
// implementation (Expo push API / FCM) is a follow-up once the mobile team
// has credentials and a development build with expo-notifications — see
// internal/llm's gigachat.go/mock.go split for the pattern to follow when
// that lands.
type Sender interface {
	Send(ctx context.Context, deviceToken, platform string, n Notification) error
}
