package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/repo"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
	"github.com/mostransport/vsm-trainer/internal/simulation"
)

func TestSimulationBranchesAndDeduplicatesCommands(t *testing.T) {
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
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("sim-%s@example.invalid", uuid.NewString()), "sim-test", "unused")
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
	first, err := svc.Start(ctx, player.ID)
	if err != nil || first.Event == nil || len(first.Event.Choices) != 2 {
		t.Fatalf("start: %+v, %v", first, err)
	}
	if len(first.Events) != 2 {
		t.Fatalf("expected two visible events and one hidden cue: %+v", first.Events)
	}
	raw, _ := json.Marshal(first)
	if stringContainsAny(string(raw), "availability_checked", "loyalty_delta", "explanation") {
		t.Fatalf("private choice effects leaked: %s", raw)
	}
	commandID := uuid.New()
	// A running session keeps its compiled template even if a later deploy
	// changes the current catalog.
	svc.template.Events[0].Choices[0].NextEvent = "unconfirmed_promise"
	checked, err := svc.Action(ctx, player.ID, first.Run.ID, commandID, 0, "check_availability")
	if err != nil || checked.Run.StateVersion != 1 || checked.Event.ID != "confirmed_request" || checked.Run.Loyalty != 83 {
		t.Fatalf("checked path: %+v, %v", checked, err)
	}
	repeated, err := svc.Action(ctx, player.ID, first.Run.ID, commandID, 0, "promise_immediately")
	if err != nil || repeated.Run.StateVersion != 1 || repeated.Event.ID != "confirmed_request" || repeated.Run.Loyalty != 83 {
		t.Fatalf("duplicate command changed state: %+v, %v", repeated, err)
	}
	if _, err := svc.Action(ctx, player.ID, first.Run.ID, uuid.New(), 0, "explain_next_step"); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
	continued, err := svc.Action(ctx, player.ID, first.Run.ID, uuid.New(), 1, "explain_next_step")
	if err != nil || continued.Run.Status != "active" || continued.Run.Loyalty != 87 || continued.Event.ID != "seat_conflict" {
		t.Fatalf("other event remained active: %+v, %v", continued, err)
	}
	seat, err := svc.Action(ctx, player.ID, first.Run.ID, uuid.New(), 2, "check_tickets")
	if err != nil || seat.Run.StateVersion != 3 || len(seat.Events) != 0 {
		t.Fatalf("hidden event leaked or seat choice failed: %+v, %v", seat, err)
	}
	if _, err := svc.ActionCommand(ctx, player.ID, first.Run.ID, uuid.New(), 3, simulation.Command{EventID: "wet_floor", ChoiceID: "report_spill"}); !errors.Is(err, simulation.ErrInvalidChoice) {
		t.Fatalf("hidden event was playable before discovery: %v", err)
	}
	moved, err := svc.ActionCommand(ctx, player.ID, first.Run.ID, uuid.New(), 3, simulation.Command{ActionID: "move_to", Target: "luggage_zone"})
	if err != nil || moved.Run.Location != "luggage_zone" || moved.Run.GameTimeS != 26 || len(moved.Events) != 0 || len(moved.ObservableCues) != 1 {
		t.Fatalf("movement or hidden cue: %+v, %v", moved, err)
	}
	observed, err := svc.ActionCommand(ctx, player.ID, first.Run.ID, uuid.New(), 4, simulation.Command{ActionID: "inspect"})
	if err != nil || observed.Run.GameTimeS != 31 || len(observed.Events) != 1 || observed.Event.ID != "wet_floor" {
		t.Fatalf("inspection: %+v, %v", observed, err)
	}
	completed, err := svc.ActionCommand(ctx, player.ID, first.Run.ID, uuid.New(), 5, simulation.Command{EventID: "wet_floor", ChoiceID: "report_spill"})
	if err != nil || completed.Run.Status != "finished" || completed.Event != nil {
		t.Fatalf("completion: %+v, %v", completed, err)
	}
	result, err := svc.Result(ctx, player.ID, first.Run.ID)
	if err != nil || !result.SessionPass || result.SessionSafetyScore != 100 || len(result.Debrief) != 6 || result.ActionLogHash == "" {
		t.Fatalf("normal result: %+v, %v", result, err)
	}
	svc.template.Events[0].Choices[0].NextEvent = "confirmed_request"
	second, err := svc.Start(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	promised, err := svc.ActionCommand(ctx, player.ID, second.Run.ID, uuid.New(), 0,
		simulation.Command{EventID: "service_request", ChoiceID: "promise_immediately"})
	if err != nil || promised.Run.Loyalty != 76 || len(promised.Events) != 2 || promised.Events[1].ID != "unconfirmed_promise" {
		t.Fatalf("second branch: %+v, %v", promised, err)
	}
	if _, err := svc.Get(ctx, uuid.New(), first.Run.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("other player read simulation: %v", err)
	}
	third, err := svc.Start(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for _, choice := range []string{"check_availability", "promise_immediately"} {
		go func(choiceID string) {
			_, actionErr := svc.Action(ctx, player.ID, third.Run.ID, uuid.New(), 0, choiceID)
			results <- actionErr
		}(choice)
	}
	firstErr, secondErr := <-results, <-results
	if (firstErr == nil) == (secondErr == nil) || (firstErr != nil && !errors.Is(firstErr, repo.ErrConflict)) || (secondErr != nil && !errors.Is(secondErr, repo.ErrConflict)) {
		t.Fatalf("concurrent commands: %v, %v", firstErr, secondErr)
	}
}

func TestSimulationTimerClosesServiceWindowOnce(t *testing.T) {
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
	player, err := store.CreatePlayer(ctx, fmt.Sprintf("timer-%s@example.invalid", uuid.NewString()), "timer-test", "unused")
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
	started, err := svc.Start(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Run.DeadlineAt == nil {
		t.Fatal("server deadline missing")
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	due := time.Now().Add(-time.Second)
	if _, err := conn.Exec(ctx, `UPDATE simulation_runs SET deadline_at = $2, state = jsonb_set(state, '{deadline_at}', to_jsonb($3::text)) WHERE id = $1`, started.Run.ID, due, due.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseExpired(ctx); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Get(ctx, player.ID, started.Run.ID)
	if err != nil || !view.Run.TimedOut || view.Run.StateVersion != 1 || view.Event.ID != "service_window_closed" {
		t.Fatalf("timer transition: %+v, %v", view, err)
	}
	if err := svc.CloseExpired(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Get(ctx, player.ID, started.Run.ID)
	if err != nil || again.Run.StateVersion != 1 {
		t.Fatalf("timer repeated: %+v, %v", again, err)
	}
	if _, err := svc.Result(ctx, player.ID, started.Run.ID); !errors.Is(err, ErrSimulationActive) {
		t.Fatalf("active result: %v", err)
	}
	for _, choice := range []struct{ event, id string }{{"service_window_closed", "explain_closed_window"}, {"seat_conflict", "dismiss_dispute"}} {
		view, err = svc.ActionCommand(ctx, player.ID, started.Run.ID, uuid.New(), view.Run.StateVersion, simulation.Command{EventID: choice.event, ChoiceID: choice.id})
		if err != nil {
			t.Fatal(err)
		}
	}
	view, err = svc.ActionCommand(ctx, player.ID, started.Run.ID, uuid.New(), view.Run.StateVersion, simulation.Command{ActionID: "move_to", Target: "luggage_zone"})
	if err != nil {
		t.Fatal(err)
	}
	view, err = svc.ActionCommand(ctx, player.ID, started.Run.ID, uuid.New(), view.Run.StateVersion, simulation.Command{ActionID: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	view, err = svc.ActionCommand(ctx, player.ID, started.Run.ID, uuid.New(), view.Run.StateVersion, simulation.Command{EventID: "wet_floor", ChoiceID: "report_spill"})
	if err != nil || view.Run.Status != "finished" {
		t.Fatalf("finish after timeout: %+v, %v", view, err)
	}
	result, err := svc.Result(ctx, player.ID, started.Run.ID)
	if err != nil || !result.TimedOut || result.SessionPass || len(result.Debrief) != 6 || result.Debrief[0].EffectID != "service_window_closed" {
		t.Fatalf("timeout result: %+v, %v", result, err)
	}
	if result.LeaderboardPointsDelta != 0 || result.LeaderboardEligible {
		t.Fatalf("timeout earned leaderboard points: %+v", result)
	}
	profile, err := NewProfileService(store).Get(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, achievement := range profile.Achievements {
		if achievement == "first_complete" {
			t.Fatal("failed run earned first_complete")
		}
	}
	if len(result.Debrief[2].BetterOptions) != 1 || !strings.Contains(result.Debrief[2].BetterOptions[0], "билетов") {
		t.Fatalf("missing better action after poor choice: %+v", result.Debrief[2])
	}
}

func stringContainsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
