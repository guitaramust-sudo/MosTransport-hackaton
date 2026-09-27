package postgres

import (
	"context"
	"testing"
)

func TestAdvanceWagonProgressIsSequentialOnly(t *testing.T) {
	store, playerID := integrationStore(t)
	ctx := context.Background()

	advanced, err := store.AdvanceWagonProgress(ctx, playerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !advanced {
		t.Fatal("advancing from 0 to 1 should succeed")
	}
	player, err := store.GetPlayerByID(ctx, playerID)
	if err != nil || player.WagonProgress != 1 {
		t.Fatalf("progress after first advance: %+v, %v", player, err)
	}

	advanced, err = store.AdvanceWagonProgress(ctx, playerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if advanced {
		t.Fatal("re-advancing to the same order should be a no-op")
	}
	player, err = store.GetPlayerByID(ctx, playerID)
	if err != nil || player.WagonProgress != 1 {
		t.Fatalf("progress after duplicate advance: %+v, %v", player, err)
	}

	advanced, err = store.AdvanceWagonProgress(ctx, playerID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if advanced {
		t.Fatal("skipping ahead to order 3 from progress 1 should be a no-op")
	}
	player, err = store.GetPlayerByID(ctx, playerID)
	if err != nil || player.WagonProgress != 1 {
		t.Fatalf("progress after skip attempt: %+v, %v", player, err)
	}

	advanced, err = store.AdvanceWagonProgress(ctx, playerID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !advanced {
		t.Fatal("advancing from 1 to 2 should succeed")
	}
	player, err = store.GetPlayerByID(ctx, playerID)
	if err != nil || player.WagonProgress != 2 {
		t.Fatalf("progress after second advance: %+v, %v", player, err)
	}
}
