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

func TestCreateSessionRollsBackPartialSituations(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	first := draftSituation(time.Now().Add(time.Minute))
	second := draftSituation(time.Now().Add(time.Minute))
	second.PassengerParams = map[string]any{"bad": make(chan int)}
	if _, _, err := store.CreateSessionWithSituations(ctx, playerID, []domain.Situation{first, second}); err == nil {
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
	sess, situations, err := store.CreateSessionWithSituations(ctx, playerID, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == uuid.Nil || len(situations) != 1 {
		t.Fatalf("unexpected created shift: %+v, %+v", sess, situations)
	}
	id := situations[0].ID
	turnCount, err := store.AppendTurn(ctx, id, playerID, "Вызову врача", "Хорошо", []string{"medic"})
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
	if _, err := store.AppendTurn(ctx, id, playerID, "Поздно", "Ответ", nil); !errors.Is(err, repo.ErrDeadlineExceeded) {
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
	_, situations, err := store.CreateSessionWithSituations(ctx, playerID, []domain.Situation{draftSituation(time.Now().Add(time.Minute))})
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
		_, err := store.AppendTurn(ctx, id, playerID, "Поздняя реплика", "Ответ", nil)
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
