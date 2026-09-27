package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/llm"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
)

func TestWagonSessionEndToEnd(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("wagon-%s@example.invalid", uuid.NewString()), "wagon-test", "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE id = $1`, player.ID)
			_ = conn.Close(context.Background())
		}
	})
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := content.WagonClassConfig{Anchors: []string{"seat_1", "service_point"}, SeatAnchors: []string{"seat_1"}, ServicePointAnchor: "service_point",
		SessionDurationS: 60, TickS: 1, SpawnCheckIntervalS: 3600, SpawnProbability: 1, MaxConcurrentSituations: 1, MoveDurationS: 1,
		PoolUnlock: map[string]float64{"easy": 0, "medium": 0.35, "hard": 0.7}, SituationPoolIDs: []string{"cold"}}
	classes := content.WagonClasses{"standard": cfg}
	mgr := NewWagonManager(store, catalog)
	svc := NewWagonService(store, catalog, classes, mgr)
	sess, err := svc.StartSession(ctx, player.ID, "standard")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.StopAll()
	deadline := time.Now().Add(5 * time.Second)
	var situationID uuid.UUID
	for time.Now().Before(deadline) {
		reloaded, err := store.GetSession(ctx, sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.WagonState.Seats[0].SituationID != nil {
			situationID = *reloaded.WagonState.Seats[0].SituationID
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if situationID == uuid.Nil {
		t.Fatal("cold situation did not spawn")
	}
	mgr.Stop(sess.ID)
	recovered := NewWagonManager(store, catalog)
	if err := recovered.Recover(ctx, classes); err != nil {
		t.Fatal(err)
	}
	defer recovered.StopAll()
	if recovered.runtime(sess.ID) == nil {
		t.Fatal("active wagon did not recover")
	}
	situations := NewSituationService(store, llm.NewMockLLM(), catalog)
	turn, err := situations.SendMessage(ctx, player.ID, situationID, "Вам холодно? Я помогу.", nil)
	if err != nil || turn.Reply == "" {
		t.Fatalf("dialogue: %+v %v", turn, err)
	}
	if err := recovered.Dispatch(ctx, sess.ID, NewWagonCommand("pick_item", "blanket", "", uuid.Nil)); err != nil {
		t.Fatal(err)
	}
	if err := recovered.Dispatch(ctx, sess.ID, NewWagonCommand("move_to", "", "seat_1", uuid.Nil)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	if err := recovered.Dispatch(ctx, sess.ID, NewWagonCommand("give_item", "blanket", "", situationID)); err != nil {
		t.Fatal(err)
	}
	sit, err := store.GetSituation(ctx, situationID)
	if err != nil || !sit.PhysicalActionDone {
		t.Fatalf("physical action: %+v %v", sit, err)
	}
	if _, err := situations.Finish(ctx, player.ID, situationID); err != nil {
		t.Fatal(err)
	}
	recovered.Stop(sess.ID)
	sessionService := NewSessionService(store, catalog, 4, situations, "demo")
	breakdown, err := sessionService.Finish(ctx, player.ID, sess.ID)
	if err != nil || len(breakdown.Situations) != 1 {
		t.Fatalf("finish: %+v %v", breakdown, err)
	}
}
