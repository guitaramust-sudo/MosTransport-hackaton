package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

func integrationStore(t *testing.T) (*Store, uuid.UUID) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("atomic-%s@example.com", uuid.NewString()), "atomic-test", "unused")
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, player.ID)
		store.Close()
	})
	return store, player.ID
}

func draftSituation(deadline time.Time) domain.Situation {
	return domain.Situation{
		Status: domain.SituationStatusActive, Loyalty: 50, Safety: 50,
		TimerDeadline: &deadline, PassengerParams: map[string]any{"opening": "Начало"},
	}
}

func TestRefreshTokenCanOnlyBeConsumedOnce(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	const token = "one-time-test-token"
	if err := store.CreateRefreshToken(ctx, playerID, token, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := store.ConsumeRefreshToken(ctx, token); results <- err }()
	}
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("want exactly one success, got %v and %v", first, second)
	}
}

func TestFinishAwardsXPAndCompetenciesAtomically(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	sess, situations, err := store.CreateSessionWithSituations(ctx, playerID, nil, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.FinishSessionAndAwardXP(ctx, sess.ID, playerID, 12, nil, 1, 0, time.Now()); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("active situation should prevent finishing: %v", err)
	}
	closeTestSituation(t, store, situations[0])
	if _, err := store.FinishSessionAndAwardXP(ctx, sess.ID, playerID, 12, nil, 1, 1, time.Now()); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("stale pending count should prevent finishing: %v", err)
	}
	if _, err := store.FinishSessionAndAwardXP(ctx, sess.ID, playerID, 12,
		map[string]repo.CompetencyAward{"unknown-code": {XP: 12, Evidence: 1}}, 1, 0, time.Now()); err == nil {
		t.Fatal("unknown competency should roll back session and XP")
	}
	sess, err = store.GetSession(ctx, sess.ID)
	if err != nil || sess.Status != domain.SessionStatusActive {
		t.Fatalf("session after rollback: %+v, %v", sess, err)
	}
	player, err := store.GetPlayerByID(ctx, playerID)
	if err != nil || player.TotalXP != 0 {
		t.Fatalf("XP after rollback: %+v, %v", player, err)
	}
	awards := map[string]repo.CompetencyAward{"service": {XP: 12, Evidence: 1}}
	awarded, err := store.FinishSessionAndAwardXP(ctx, sess.ID, playerID, 12, awards, 1, 0, time.Now())
	if err != nil || !awarded {
		t.Fatalf("first award: %v, %v", awarded, err)
	}
	awarded, err = store.FinishSessionAndAwardXP(ctx, sess.ID, playerID, 12, awards, 1, 0, time.Now())
	if err != nil || awarded {
		t.Fatalf("duplicate award: %v, %v", awarded, err)
	}
	player, err = store.GetPlayerByID(ctx, playerID)
	if err != nil || player.TotalXP != 12 {
		t.Fatalf("final XP: %+v, %v", player, err)
	}
	competencies, err := store.GetPlayerCompetencies(ctx, playerID)
	if err != nil || len(competencies) != 1 || competencies[0].XP != 12 || competencies[0].EvidenceCount != 1 {
		t.Fatalf("final competencies: %+v, %v", competencies, err)
	}
}

func TestConcurrentSpawnCreatesOnlyOneSituation(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	sess, situations, err := store.CreateSessionWithSituations(ctx, playerID, []string{"next"}, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
	if err != nil {
		t.Fatal(err)
	}
	closeTestSituation(t, store, situations[0])
	results := make(chan *domain.Situation, 2)
	errorsCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			sit, err := store.SpawnNextSituation(ctx, sess.ID, func(id string) (domain.Situation, error) {
				if id != "next" {
					return domain.Situation{}, fmt.Errorf("unexpected scenario %q", id)
				}
				return draftSituation(time.Now().Add(time.Minute)), nil
			})
			results <- sit
			errorsCh <- err
		}()
	}
	spawned := 0
	for i := 0; i < 2; i++ {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
		if <-results != nil {
			spawned++
		}
	}
	if spawned != 1 {
		t.Fatalf("spawned %d situations, want 1", spawned)
	}
	all, err := store.ListSituationsBySession(ctx, sess.ID)
	if err != nil || len(all) != 2 {
		t.Fatalf("situations = %d, %v", len(all), err)
	}
}

func TestApproveRequiresFinishedSession(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	sess, situations, err := store.CreateSessionWithSituations(ctx, playerID, nil, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApproveSession(ctx, sess.ID); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("active approval: %v", err)
	}
	closeTestSituation(t, store, situations[0])
	if _, err := store.FinishSessionAndAwardXP(ctx, sess.ID, playerID, 0, nil, 1, 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.ApproveSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.ApproveSession(ctx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("missing approval: %v", err)
	}
}

func closeTestSituation(t *testing.T, store *Store, sit domain.Situation) {
	t.Helper()
	outcome := "success"
	sit.Outcome = &outcome
	closed, err := store.CloseSituation(context.Background(), sit)
	if err != nil || !closed {
		t.Fatalf("close test situation: %v, %v", closed, err)
	}
}

func TestExternalUserAcceptsEmptyClasses(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	store, err := New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	externalID := uuid.NewString()
	player, created, err := store.UpsertExternalUser(ctx, "test", externalID, nil, nil, nil, nil)
	if err != nil || !created {
		t.Fatalf("create external: %+v, %v, %v", player, created, err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, player.ID)
		store.Close()
	})
	_, created, err = store.UpsertExternalUser(ctx, "test", externalID, nil, nil, nil, nil)
	if err != nil || created {
		t.Fatalf("idempotent external: %v, %v", created, err)
	}
}

func TestCreateSessionRollsBackPartialSituations(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	first := draftSituation(time.Now().Add(time.Minute))
	second := draftSituation(time.Now().Add(time.Minute))
	second.PassengerParams = map[string]any{"bad": make(chan int)}
	if _, _, err := store.CreateSessionWithSituations(ctx, playerID, nil, []domain.Situation{first, second}); err == nil {
		t.Fatal("expected invalid JSON to abort session creation")
	}
	var count int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE player_id = $1`, playerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("partial session persisted: %d rows", count)
	}
}

func TestAppendTurnAndEscalationAreAtomic(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	sess, situations, err := store.CreateSessionWithSituations(ctx, playerID, nil, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == uuid.Nil || len(situations) != 1 {
		t.Fatalf("unexpected created shift: %+v, %+v", sess, situations)
	}
	id := situations[0].ID
	turnCount, err := store.AppendTurn(ctx, id, playerID, "Вызову врача", "Хорошо", []string{"medic"}, nil)
	if err != nil || turnCount != 1 {
		t.Fatalf("AppendTurn = %d, %v", turnCount, err)
	}
	actual, err := store.RecordEscalation(ctx, id, playerID, "medic")
	if err != nil || len(actual) != 1 || actual[0] != "medic" {
		t.Fatalf("deduplicated escalation = %v, %v", actual, err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE situations SET timer_deadline = now() - interval '1 second' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendTurn(ctx, id, playerID, "Поздно", "Ответ", nil, nil); !errors.Is(err, repo.ErrDeadlineExceeded) {
		t.Fatalf("expired AppendTurn error = %v", err)
	}
	messages, err := store.ListMessagesBySituation(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 { // opening + one complete turn; no duplicate escalation message
		t.Fatalf("messages after rejected turn = %d, want 3", len(messages))
	}
}

func TestLockedCloseRejectsConcurrentTurn(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	_, situations, err := store.CreateSessionWithSituations(ctx, playerID, nil, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
	if err != nil {
		t.Fatal(err)
	}
	id := situations[0].ID
	locked := make(chan struct{})
	release := make(chan struct{})
	closeErr := make(chan error, 1)
	go func() {
		closeErr <- store.WithSituationLock(ctx, id, func(row repo.LockedSituation) error {
			close(locked)
			<-release
			result := row.Situation()
			outcome, closedAt := "fail", time.Now()
			result.Outcome, result.ClosedAt = &outcome, &closedAt
			return row.Close(ctx, result)
		})
	}()
	<-locked
	turnErr := make(chan error, 1)
	go func() {
		_, err := store.AppendTurn(ctx, id, playerID, "Поздняя реплика", "Ответ", nil, nil)
		turnErr <- err
	}()
	select {
	case err := <-turnErr:
		t.Fatalf("turn completed while close lock held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-closeErr; err != nil {
		t.Fatal(err)
	}
	if err := <-turnErr; !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("turn after close error = %v", err)
	}
	messages, err := store.ListMessagesBySituation(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("late message persisted: %v", messages)
	}
}
