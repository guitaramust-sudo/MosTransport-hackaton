package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/repo"
)

// prizeEntryLifetime is the "6x24h" expiry rule from the build doc: every
// prize-credit entry (lesson-completion or demo-seed) expires 6 days after
// it was awarded, by server UTC.
const prizeEntryLifetime = 6 * 24 * time.Hour

// prizeShirtThreshold is the mock "progress toward a 50-point t-shirt"
// number. There is no real redemption -- this is display-only.
const prizeShirtThreshold = 50

// PrizeService is a separate, minimal prize-credit ledger: +10 credits for
// each of B01/B02's first successful completion (never on repeat, enforced
// by the DB unique constraint on AwardPrizeCredit), each entry expiring
// 6x24h after it was awarded, and a computed (not stored) balance/nearest
// expiry. It is entirely independent of XP/badges/the leaderboard -- it
// never touches players.total_xp, lesson_awards, or any leaderboard code.
type PrizeService struct {
	store repo.Store
}

func NewPrizeService(store repo.Store) *PrizeService {
	return &PrizeService{store: store}
}

// AwardLessonPrize grants a prize credit for lessonID's first completion.
// It is safe to call on every FinalizePractice success (including repeats)
// -- the unique (player, source_type, source_id) constraint on
// prize_credit_entries makes a repeat call a silent no-op (returns
// granted=false), matching lesson XP's own idempotency pattern.
func (p *PrizeService) AwardLessonPrize(ctx context.Context, playerID uuid.UUID, lessonID string, amount int) (bool, error) {
	now := time.Now()
	return p.store.AwardPrizeCredit(ctx, playerID, "lesson_completion", lessonID, amount, now, now.Add(prizeEntryLifetime))
}

// PrizeBalance is the player-facing view of the prize-credit ledger: the
// currently-active balance, the nearest expiry among still-active entries
// (nil if none), and a mock progress fraction toward the 50-point t-shirt.
// There is no order/redemption -- ShirtProgress is display-only.
type PrizeBalance struct {
	Balance        int        `json:"balance"`
	NextExpiryAt   *time.Time `json:"next_expiry_at,omitempty"`
	ShirtThreshold int        `json:"shirt_threshold"`
	ShirtProgress  float64    `json:"shirt_progress"` // min(balance,threshold)/threshold, 0..1, capped at 1.0 -- mock only, no real redemption
}

// Balance computes playerID's current prize-credit balance and nearest
// expiry, both derived at read time (no stored "expired" flag), plus the
// mock shirt-progress fraction.
func (p *PrizeService) Balance(ctx context.Context, playerID uuid.UUID) (PrizeBalance, error) {
	balance, nextExpiry, err := p.store.PrizeCreditBalance(ctx, playerID, time.Now())
	if err != nil {
		return PrizeBalance{}, err
	}
	capped := balance
	if capped > prizeShirtThreshold {
		capped = prizeShirtThreshold
	}
	progress := float64(capped) / float64(prizeShirtThreshold)
	return PrizeBalance{
		Balance:        balance,
		NextExpiryAt:   nextExpiry,
		ShirtThreshold: prizeShirtThreshold,
		ShirtProgress:  progress,
	}, nil
}

// SeedDemoEntry creates the fixed +40 test entry described in the build doc
// for verifying the 6-day expiry / 50-point threshold without waiting six
// real days: awarded_at=now-5d, expires_at=now+24h (the same 6x24h rule,
// just anchored 5 days in the past so it's already near expiry),
// source_type="demo_seed". This is an admin action for seeding a synthetic
// demo account, not something a real player can trigger themselves -- see
// Handlers.SeedDemoPrizeEntry, which is registered only under the
// AdminAuth-gated /admin route group.
func (p *PrizeService) SeedDemoEntry(ctx context.Context, playerID uuid.UUID) (bool, error) {
	awardedAt := time.Now().Add(-5 * 24 * time.Hour)
	return p.store.AwardPrizeCredit(ctx, playerID, "demo_seed", "demo_seed_v1", 40, awardedAt, awardedAt.Add(prizeEntryLifetime))
}
