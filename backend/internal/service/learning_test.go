package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/push"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

// fakeLearningStore implements repo.Store by embedding it as a nil interface
// field: any method not explicitly overridden below panics on a nil
// interface call if LearningService ever calls it, which is a clear enough
// signal in a unit test that the fake needs another method -- so only the
// handful of methods LearningService actually exercises need real bodies,
// rather than ~45 no-op stubs.
type fakeLearningStore struct {
	repo.Store

	mu           sync.Mutex
	players      map[uuid.UUID]domain.Player
	sessions     map[uuid.UUID]domain.Session
	progress     map[string]domain.LessonProgress
	correct      map[string]bool
	awarded      map[string]bool
	prizeEntries map[string]domain.PrizeCreditEntry
	pushSent     map[string]bool
}

func newFakeLearningStore() *fakeLearningStore {
	return &fakeLearningStore{
		players:      map[uuid.UUID]domain.Player{},
		sessions:     map[uuid.UUID]domain.Session{},
		progress:     map[string]domain.LessonProgress{},
		correct:      map[string]bool{},
		awarded:      map[string]bool{},
		prizeEntries: map[string]domain.PrizeCreditEntry{},
		pushSent:     map[string]bool{},
	}
}

// MarkPushNotificationSent mirrors the postgres implementation's dedup
// guarantee (insert-once on (player_id, event_type, source_id)) using an
// in-memory map instead of a DB unique constraint.
func (f *fakeLearningStore) MarkPushNotificationSent(_ context.Context, playerID uuid.UUID, eventType, sourceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := playerID.String() + "|" + eventType + "|" + sourceID
	if f.pushSent[key] {
		return false, nil
	}
	f.pushSent[key] = true
	return true, nil
}

// ListPushSubscriptions always returns no devices: FinalizePractice's push
// trigger only needs to exercise the dedup path in these tests, not actual
// delivery.
func (f *fakeLearningStore) ListPushSubscriptions(_ context.Context, _ uuid.UUID) ([]domain.PushSubscription, error) {
	return nil, nil
}

// newTestLearningService builds a LearningService backed by store, wired to
// a real PrizeService and PushService over the same fake store -- so
// FinalizePractice's prize-award/push-notify calls and GetMyLearning's
// balance read exercise the same idempotency/expiry logic those services
// ship with, rather than a second-guessed fake.
func newTestLearningService(store *fakeLearningStore, catalog content.Catalog, classes content.WagonClasses, curriculum content.Curriculum, wagon *WagonService, manager *WagonManager) *LearningService {
	return NewLearningService(store, catalog, classes, curriculum, wagon, manager, NewPrizeService(store), NewPushService(store, push.NewLogSender()))
}

// AwardPrizeCredit mirrors the postgres implementation's idempotency
// guarantee (insert-once on (player_id, source_type, source_id)) using an
// in-memory map instead of a DB unique constraint.
func (f *fakeLearningStore) AwardPrizeCredit(_ context.Context, playerID uuid.UUID, sourceType, sourceID string, amount int, awardedAt, expiresAt time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := playerID.String() + "|" + sourceType + "|" + sourceID
	if _, ok := f.prizeEntries[key]; ok {
		return false, nil
	}
	f.prizeEntries[key] = domain.PrizeCreditEntry{
		PlayerID: playerID, SourceType: sourceType, SourceID: sourceID,
		Amount: amount, AwardedAt: awardedAt, ExpiresAt: expiresAt,
	}
	return true, nil
}

// PrizeCreditBalance mirrors the postgres implementation: sums amounts of
// entries whose ExpiresAt is after now, and reports the nearest ExpiresAt
// among those same active entries (nil if none).
func (f *fakeLearningStore) PrizeCreditBalance(_ context.Context, playerID uuid.UUID, now time.Time) (int, *time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	balance := 0
	var nextExpiry *time.Time
	for _, e := range f.prizeEntries {
		if e.PlayerID != playerID || !e.ExpiresAt.After(now) {
			continue
		}
		balance += e.Amount
		if nextExpiry == nil || e.ExpiresAt.Before(*nextExpiry) {
			expiry := e.ExpiresAt
			nextExpiry = &expiry
		}
	}
	return balance, nextExpiry, nil
}

func lpKey(playerID uuid.UUID, lessonID string) string { return playerID.String() + "|" + lessonID }

func (f *fakeLearningStore) GetPlayerByID(_ context.Context, id uuid.UUID) (domain.Player, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.players[id]
	if !ok {
		return domain.Player{}, repo.ErrNotFound
	}
	return p, nil
}

func (f *fakeLearningStore) GetLessonProgress(_ context.Context, playerID uuid.UUID, lessonID string) (domain.LessonProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.progress[lpKey(playerID, lessonID)]
	if !ok {
		return domain.LessonProgress{}, repo.ErrNotFound
	}
	return p, nil
}

func (f *fakeLearningStore) UpsertLessonProgress(_ context.Context, p domain.LessonProgress) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress[lpKey(p.PlayerID, p.LessonID)] = p
	return nil
}

func (f *fakeLearningStore) RecordLessonAnswer(_ context.Context, playerID uuid.UUID, lessonID, questionID, _ string, correct bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if correct {
		f.correct[playerID.String()+"|"+lessonID+"|"+questionID] = true
	}
	return nil
}

func (f *fakeLearningStore) HasCorrectLessonAnswer(_ context.Context, playerID uuid.UUID, lessonID, questionID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.correct[playerID.String()+"|"+lessonID+"|"+questionID], nil
}

// AwardLessonCompletion mirrors the postgres implementation's idempotency
// guarantee (insert-once, bump XP only on the insert that actually
// happened) using an in-memory "awarded" set instead of a DB unique
// constraint.
func (f *fakeLearningStore) AwardLessonCompletion(_ context.Context, playerID uuid.UUID, lessonID string, xpDelta int, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := playerID.String() + "|" + lessonID + "|completion"
	if f.awarded[key] {
		return false, nil
	}
	f.awarded[key] = true
	p := f.players[playerID]
	p.TotalXP += xpDelta
	f.players[playerID] = p
	return true, nil
}

func (f *fakeLearningStore) CreateWagonSession(_ context.Context, playerID uuid.UUID, state domain.WagonState) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := domain.Session{ID: uuid.New(), PlayerID: playerID, Status: domain.SessionStatusActive, WagonState: &state}
	f.sessions[s.ID] = s
	return s, nil
}

func (f *fakeLearningStore) GetSession(_ context.Context, id uuid.UUID) (domain.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return domain.Session{}, repo.ErrNotFound
	}
	return s, nil
}

func (f *fakeLearningStore) ListSituationsBySession(_ context.Context, _ uuid.UUID) ([]domain.Situation, error) {
	return nil, nil
}

// testLearningCurriculum fabricates a two-lesson curriculum, independent of
// the real B01/B02 content, specifically so a "lesson locked until the
// previous one completes" shape can be exercised (the real B01/B02 pair
// isn't set up to make that easy to test in isolation).
func testLearningCurriculum() content.Curriculum {
	chapters := []content.Chapter{
		{ChapterID: "ch1", Order: 1, Title: "Chapter 1", LessonIDs: []string{"L1", "L2"}, Scope: "class", ClassID: "first"},
	}
	opts := []content.QuestionOption{{OptionID: "a", Text: "A"}, {OptionID: "b", Text: "B"}}
	lessons := []content.Lesson{
		{
			LessonID: "L1", ChapterID: "ch1", Order: 1, Title: "Lesson 1", BadgeID: "badge1",
			TheoryQuestionIDs: []string{"Q1", "Q2"}, PracticeQuestionIDs: []string{"PQ1"},
			RequiredAnchorIDs: []string{"a1"}, RequiredObjectIDs: []string{"o1"},
			CompletionRule: "visit_inspect", DebriefIntro: "debrief1",
		},
		{
			LessonID: "L2", ChapterID: "ch1", Order: 2, Title: "Lesson 2", BadgeID: "badge2",
			TheoryQuestionIDs: []string{"Q3"}, PracticeQuestionIDs: []string{"PQ2"},
			RequiredAnchorIDs: []string{"a2"}, RequiredObjectIDs: []string{"o2"},
			CompletionRule: "visit_inspect", DebriefIntro: "debrief2",
		},
	}
	questions := []content.Question{
		{QuestionID: "Q1", Phase: "theory", Type: "single_choice", Prompt: "Q1?", Options: opts, CorrectOptionID: "a"},
		{QuestionID: "Q2", Phase: "theory", Type: "single_choice", Prompt: "Q2?", Options: opts, CorrectOptionID: "a"},
		{QuestionID: "Q3", Phase: "theory", Type: "single_choice", Prompt: "Q3?", Options: opts, CorrectOptionID: "a"},
		{QuestionID: "PQ1", Phase: "practice", Type: "single_choice", Prompt: "PQ1?", Options: opts, CorrectOptionID: "a"},
		{QuestionID: "PQ2", Phase: "practice", Type: "single_choice", Prompt: "PQ2?", Options: opts, CorrectOptionID: "a"},
	}
	return content.Curriculum{Chapters: chapters, Lessons: lessons, Questions: questions}
}

func TestStartPracticeRequiresTheoryPass(t *testing.T) {
	store := newFakeLearningStore()
	svc := newTestLearningService(store, content.Catalog{}, content.WagonClasses{}, testLearningCurriculum(), nil, nil)
	playerID := uuid.New()
	if _, err := svc.StartPractice(context.Background(), playerID, "L1"); !errors.Is(err, ErrTheoryNotPassed) {
		t.Fatalf("expected ErrTheoryNotPassed, got %v", err)
	}
}

func TestSubmitAnswerSetsTheoryPassOnceBothCorrect(t *testing.T) {
	store := newFakeLearningStore()
	svc := newTestLearningService(store, content.Catalog{}, content.WagonClasses{}, testLearningCurriculum(), nil, nil)
	ctx := context.Background()
	playerID := uuid.New()

	res, err := svc.SubmitAnswer(ctx, playerID, "L1", "Q1", "a")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Correct || res.PhasePass {
		t.Fatalf("expected correct but not yet phase-passing after 1 of 2: %+v", res)
	}
	progress, err := store.GetLessonProgress(ctx, playerID, "L1")
	if err != nil || progress.TheoryPass {
		t.Fatalf("theory_pass should not be set yet: %+v, %v", progress, err)
	}

	res, err = svc.SubmitAnswer(ctx, playerID, "L1", "Q2", "a")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Correct || !res.PhasePass {
		t.Fatalf("expected phase pass after both theory questions answered correctly: %+v", res)
	}
	progress, err = store.GetLessonProgress(ctx, playerID, "L1")
	if err != nil || !progress.TheoryPass {
		t.Fatalf("theory_pass should be set: %+v, %v", progress, err)
	}
}

func TestGetLessonRejectsLockedLesson(t *testing.T) {
	store := newFakeLearningStore()
	svc := newTestLearningService(store, content.Catalog{}, content.WagonClasses{}, testLearningCurriculum(), nil, nil)
	ctx := context.Background()
	playerID := uuid.New()

	// L1 (Order 1) is the first lesson overall, so it must always be reachable.
	if _, err := svc.GetLesson(ctx, playerID, "L1"); err != nil {
		t.Fatalf("L1 should be unlocked: %v", err)
	}
	// L2 (Order 2) is locked until L1's CompletedAt is set, which it never is
	// for this player.
	if _, err := svc.GetLesson(ctx, playerID, "L2"); !errors.Is(err, ErrLessonLocked) {
		t.Fatalf("expected ErrLessonLocked for L2, got %v", err)
	}
}

func TestGetLessonExposesPracticeGoals(t *testing.T) {
	store := newFakeLearningStore()
	svc := newTestLearningService(store, content.Catalog{}, content.WagonClasses{}, testLearningCurriculum(), nil, nil)

	detail, err := svc.GetLesson(context.Background(), uuid.New(), "L1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.CompletionRule != "visit_inspect" {
		t.Fatalf("completion_rule = %q, want visit_inspect", detail.CompletionRule)
	}
	if len(detail.RequiredAnchorIDs) != 1 || detail.RequiredAnchorIDs[0] != "a1" {
		t.Fatalf("required_anchor_ids = %v, want [a1]", detail.RequiredAnchorIDs)
	}
	if len(detail.RequiredObjectIDs) != 1 || detail.RequiredObjectIDs[0] != "o1" {
		t.Fatalf("required_object_ids = %v, want [o1]", detail.RequiredObjectIDs)
	}
}

func TestFinalizePracticeBeforePassReportsMissing(t *testing.T) {
	store := newFakeLearningStore()
	svc := newTestLearningService(store, content.Catalog{}, content.WagonClasses{}, testLearningCurriculum(), nil, nil)
	ctx := context.Background()
	playerID := uuid.New()

	// A practice session with no visited anchors / inspected objects: L1's
	// required_anchor_ids ["a1"] / required_object_ids ["o1"] stay missing,
	// so only practice_pass should be reported as still false.
	sess, err := store.CreateWagonSession(ctx, playerID, domain.WagonState{})
	if err != nil {
		t.Fatal(err)
	}
	sessID := sess.ID
	if err := store.UpsertLessonProgress(ctx, domain.LessonProgress{
		PlayerID: playerID, LessonID: "L1", TheoryPass: true, PracticeCheckPass: true, PracticeSessionID: &sessID,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.FinalizePractice(ctx, playerID, "L1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed {
		t.Fatalf("expected not completed: %+v", result)
	}
	if len(result.Missing) != 1 || result.Missing[0] != "practice_pass" {
		t.Fatalf("expected missing=[practice_pass], got %+v", result.Missing)
	}
}

func TestFinalizePracticeAwardsXPExactlyOnceOnRepeat(t *testing.T) {
	store := newFakeLearningStore()
	svc := newTestLearningService(store, content.Catalog{}, content.WagonClasses{}, testLearningCurriculum(), nil, nil)
	ctx := context.Background()
	playerID := uuid.New()
	store.players[playerID] = domain.Player{ID: playerID}

	// This session's WagonState already satisfies L1's visit_inspect
	// requirement, so the first finalize should pass practice and award.
	state := domain.WagonState{VisitedAnchors: []string{"a1"}, InspectedObjects: []string{"o1"}}
	sess, err := store.CreateWagonSession(ctx, playerID, state)
	if err != nil {
		t.Fatal(err)
	}
	sessID := sess.ID
	if err := store.UpsertLessonProgress(ctx, domain.LessonProgress{
		PlayerID: playerID, LessonID: "L1", TheoryPass: true, PracticeCheckPass: true, PracticeSessionID: &sessID,
	}); err != nil {
		t.Fatal(err)
	}

	first, err := svc.FinalizePractice(ctx, playerID, "L1")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Completed || !first.AwardGranted || first.XPAwarded != lessonCompletionXP {
		t.Fatalf("expected first finalize to complete and award XP: %+v", first)
	}
	player, err := store.GetPlayerByID(ctx, playerID)
	if err != nil || player.TotalXP != lessonCompletionXP {
		t.Fatalf("expected total_xp=%d after first award, got %+v (%v)", lessonCompletionXP, player, err)
	}

	second, err := svc.FinalizePractice(ctx, playerID, "L1")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Completed || second.AwardGranted {
		t.Fatalf("expected second finalize to be idempotent, no new award: %+v", second)
	}
	player, err = store.GetPlayerByID(ctx, playerID)
	if err != nil || player.TotalXP != lessonCompletionXP {
		t.Fatalf("expected total_xp to stay at %d, got %+v (%v)", lessonCompletionXP, player, err)
	}
}
