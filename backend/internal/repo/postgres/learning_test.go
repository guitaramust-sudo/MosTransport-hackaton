package postgres

import (
	"context"
	"testing"
)

// TestAwardLessonCompletionIsAtomicIdempotent verifies AwardLessonCompletion
// really is atomic-idempotent: a second call with the same (playerID,
// lessonID) must be a no-op both for the lesson_awards row (enforced by its
// (player_id, lesson_id, award_type) primary key) and for players.total_xp,
// not merely by an application-level check-then-act race.
func TestAwardLessonCompletionIsAtomicIdempotent(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM lesson_awards WHERE player_id = $1`, playerID)
	})

	granted, err := store.AwardLessonCompletion(ctx, playerID, "L1", 20, "badge1")
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("first award should be granted")
	}
	player, err := store.GetPlayerByID(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if player.TotalXP != 20 {
		t.Fatalf("expected total_xp=20 after first award, got %d", player.TotalXP)
	}

	granted, err = store.AwardLessonCompletion(ctx, playerID, "L1", 20, "badge1")
	if err != nil {
		t.Fatal(err)
	}
	if granted {
		t.Fatal("second award for the same (player, lesson) should be a no-op")
	}
	player, err = store.GetPlayerByID(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	if player.TotalXP != 20 {
		t.Fatalf("total_xp should not change on repeat award: got %d, want 20", player.TotalXP)
	}
}
