package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
)

func TestContentGateSeparatesDemoAndOfficialRuns(t *testing.T) {
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
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("content-%s@example.invalid", uuid.NewString()), "content-test", "unused")
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
	if _, _, err := NewSessionService(store, catalog, 2, nil, "official").Start(ctx, player.ID); !errors.Is(err, ErrNoEligibleScenarios) {
		t.Fatalf("official run used draft content: %v", err)
	}
	sess, situations, err := NewSessionService(store, catalog, 2, nil, "demo").Start(ctx, player.ID)
	if err != nil || len(situations) != 1 || len(sess.PendingSituations) != 1 {
		t.Fatalf("demo start: %+v, %+v, %v", sess, situations, err)
	}
	if situations[0].PassengerParams["content_validation_status"] != "draft" {
		t.Fatalf("demo content status missing: %+v", situations[0].PassengerParams)
	}
}
