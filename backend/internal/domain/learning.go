package domain

import (
	"time"

	"github.com/google/uuid"
)

// LessonProgress tracks one player's progress through one curriculum lesson.
// It follows the LOCKED->THEORY->THEORY_CHECK->PRACTICE->PRACTICE_CHECK->
// DEBRIEF->COMPLETED lifecycle described in content.Lesson's doc comment:
// TheoryPass gates starting practice, PracticeCheckPass is set once every
// practice question has been answered correctly at least once,
// PracticePass reflects the latest attempt's outcome (visit/inspect coverage
// or a successful conversation), PracticePassCount counts distinct successful
// sessions, and CompletedAt is set exactly once on the call that awards
// lesson completion (see LearningService.FinalizePractice).
type LessonProgress struct {
	PlayerID          uuid.UUID  `json:"player_id"`
	LessonID          string     `json:"lesson_id"`
	TheoryPass        bool       `json:"theory_pass"`
	PracticeSessionID *uuid.UUID `json:"practice_session_id,omitempty"`
	PracticePass      bool       `json:"practice_pass"`
	PracticePassCount int        `json:"practice_pass_count"`
	PracticeCheckPass bool       `json:"practice_check_pass"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	ContentVersion    string     `json:"content_version"`
}

// LessonAward records one grant of lesson-completion reward for a player.
// Its (PlayerID, LessonID, AwardType) primary key is the idempotency
// guarantee: a second attempt to award the same (player, lesson, type)
// triple is a no-op, enforced at the database level, not by an
// application-level check-then-act.
type LessonAward struct {
	PlayerID  uuid.UUID `json:"player_id"`
	LessonID  string    `json:"lesson_id"`
	AwardType string    `json:"award_type"`
	XPDelta   int       `json:"xp_delta"`
	BadgeID   string    `json:"badge_id,omitempty"`
	AwardedAt time.Time `json:"awarded_at"`
}

// PrizeCreditEntry is one grant of the separate, minimal prize-credit ledger
// (independent of XP/badges/the leaderboard). Its (PlayerID, SourceType,
// SourceID) unique constraint is the idempotency guarantee -- a second
// attempt to award the same (player, source_type, source_id) triple is a
// no-op, enforced at the database level. There is no stored "expired" flag:
// active vs. expired is always computed at read time by comparing ExpiresAt
// to the current time.
type PrizeCreditEntry struct {
	ID         int64     `json:"id"`
	PlayerID   uuid.UUID `json:"player_id"`
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	Amount     int       `json:"amount"`
	AwardedAt  time.Time `json:"awarded_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// PushSubscription is one device's registration for push notifications.
// It is keyed by DeviceToken (not (PlayerID, DeviceToken)): a token belongs
// to exactly one player at a time, and re-registering it for a different
// player (device changed hands, demo account re-registers) reassigns it
// rather than creating a duplicate row -- see
// repo.Store.RegisterPushSubscription's ON CONFLICT (device_token) DO
// UPDATE.
type PushSubscription struct {
	PlayerID    uuid.UUID `json:"player_id"`
	Platform    string    `json:"platform"`
	DeviceToken string    `json:"device_token"`
	CreatedAt   time.Time `json:"created_at"`
}
