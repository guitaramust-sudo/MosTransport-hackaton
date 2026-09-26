package service

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
	"github.com/mostransport/vsm-trainer/internal/simulation"
)

func TestSimulationAchievementsChallengeAndNotifications(t *testing.T) {
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
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("rewards-%s@example.invalid", uuid.NewString()), "rewards-test", "unused")
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
	template, err := simulation.Load()
	if err != nil {
		t.Fatal(err)
	}
	svc := NewSimulationService(store, template, "demo")
	firstID, firstVariant := completeCleanSimulation(t, ctx, svc, player.ID)
	if firstVariant != "A" {
		t.Fatalf("first variant = %q", firstVariant)
	}
	progress, err := svc.Challenge(ctx, player.ID)
	if err != nil || progress.SeedVariants != 1 || progress.Completed {
		t.Fatalf("first progress: %+v, %v", progress, err)
	}
	secondID, secondVariant := completeCleanSimulation(t, ctx, svc, player.ID)
	if secondVariant != "B" {
		t.Fatalf("second variant = %q", secondVariant)
	}
	progress, err = svc.Challenge(ctx, player.ID)
	if err != nil || progress.SeedVariants != 2 || !progress.Completed {
		t.Fatalf("challenge completion: %+v, %v", progress, err)
	}
	profile, err := NewProfileService(store).Get(ctx, player.ID)
	if err != nil || profile.Player.TotalXP != 5 || len(profile.Achievements) != 2 {
		t.Fatalf("profile rewards: %+v, %v", profile, err)
	}
	notifications, err := svc.Notifications(ctx, player.ID)
	if err != nil || len(notifications) != 3 {
		t.Fatalf("notifications: %+v, %v", notifications, err)
	}
	seen := map[string]bool{}
	for _, item := range notifications {
		seen[item.Type] = true
	}
	if !seen["new_scenario"] || !seen["challenge_started"] || !seen["challenge_completed"] {
		t.Fatalf("notification types: %+v", seen)
	}
	for _, id := range []uuid.UUID{firstID, secondID, secondID} {
		if _, err := svc.Result(ctx, player.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	profile, err = NewProfileService(store).Get(ctx, player.ID)
	if err != nil || profile.Player.TotalXP != 5 {
		t.Fatalf("duplicate challenge reward: %+v, %v", profile, err)
	}
	notifications, err = svc.Notifications(ctx, player.ID)
	if err != nil || len(notifications) != 3 {
		t.Fatalf("duplicate notifications: %+v, %v", notifications, err)
	}
}

func completeCleanSimulation(t *testing.T, ctx context.Context, svc *SimulationService, playerID uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	view, err := svc.Start(ctx, playerID)
	if err != nil {
		t.Fatal(err)
	}
	id, variant := view.Run.ID, view.Run.SeedVariant
	steps := []simulation.Command{
		{EventID: "service_request", ChoiceID: "check_availability"},
		{EventID: "confirmed_request", ChoiceID: "explain_next_step"},
		{EventID: "seat_conflict", ChoiceID: "check_tickets"},
		{ActionID: "move_to", Target: "luggage_zone"},
		{ActionID: "inspect"},
		{EventID: "wet_floor", ChoiceID: "report_spill"},
	}
	for _, command := range steps {
		view, err = svc.ActionCommand(ctx, playerID, id, uuid.New(), view.Run.StateVersion, command)
		if err != nil {
			t.Fatal(err)
		}
	}
	if view.Run.Status != "finished" {
		t.Fatalf("run did not finish: %+v", view.Run)
	}
	return id, variant
}
