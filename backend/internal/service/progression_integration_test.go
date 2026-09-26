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
	firstResult, err := svc.Result(ctx, player.ID, firstID)
	if err != nil || firstResult.LeaderboardPointsDelta != 20 || firstResult.LeaderboardPointsTotal != 30 {
		t.Fatalf("first points: %+v, %v", firstResult, err)
	}
	secondResult, err := svc.Result(ctx, player.ID, secondID)
	if err != nil || secondResult.LeaderboardPointsDelta != 10 || secondResult.LeaderboardPointsTotal != 30 {
		t.Fatalf("second points: %+v, %v", secondResult, err)
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
	if profile.LeaderboardPointsTotal != 30 {
		t.Fatalf("points changed on replay: %+v", profile)
	}
	thirdID, _ := completeCleanSimulation(t, ctx, svc, player.ID)
	thirdResult, err := svc.Result(ctx, player.ID, thirdID)
	if err != nil || thirdResult.LeaderboardPointsDelta != 0 || thirdResult.LeaderboardPointsTotal != 30 {
		t.Fatalf("repeat variant earned points: %+v, %v", thirdResult, err)
	}
	officialPoints, err := store.GetPlayerPointsTotal(ctx, player.ID, "official")
	if err != nil || officialPoints != 0 {
		t.Fatalf("official points = %d, %v", officialPoints, err)
	}
	notifications, err = svc.Notifications(ctx, player.ID)
	if err != nil || len(notifications) != 3 {
		t.Fatalf("duplicate notifications: %+v, %v", notifications, err)
	}
}

func TestPointsLeaderboardUsesFullCohort(t *testing.T) {
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
	ids := make([]uuid.UUID, 0, 52)
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE id = ANY($1)`, ids)
			_ = conn.Close(context.Background())
		}
	})
	for i := 0; i < 52; i++ {
		player, err := store.CreatePlayer(ctx, fmt.Sprintf("cohort-%s@example.invalid", uuid.NewString()), fmt.Sprintf("cohort-%d", i), "unused")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, player.ID)
	}
	board, err := NewProfileService(store, "demo").ScopedLeaderboard(ctx, "company", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if board.GroupSize <= 50 || len(board.Entries) != 50 {
		t.Fatalf("truncated cohort: size=%d entries=%d", board.GroupSize, len(board.Entries))
	}
	for i := 1; i < len(board.Entries); i++ {
		if board.Entries[i].LeaderboardPointsTotal == board.Entries[i-1].LeaderboardPointsTotal && board.Entries[i].Rank != board.Entries[i-1].Rank {
			t.Fatalf("tie got distinct ranks: %+v %+v", board.Entries[i-1], board.Entries[i])
		}
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
