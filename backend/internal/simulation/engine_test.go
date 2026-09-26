package simulation

import (
	"strings"
	"testing"
	"time"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

func TestDemoTemplateRequiresReachableHiddenCue(t *testing.T) {
	template, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	template.Edges = nil
	if err := template.Validate(); err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("unreachable event accepted: %v", err)
	}
	template, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	for i := range template.Events {
		if template.Events[i].Hidden {
			template.Events[i].Cue = ""
		}
	}
	if err := template.Validate(); err == nil || !strings.Contains(err.Error(), "hidden cue") {
		t.Fatalf("hidden event without cue accepted: %v", err)
	}
}

func TestChoiceFinishingAfterDeadlineGetsTimeoutFirst(t *testing.T) {
	template, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(time.Minute)
	run := domain.SimulationRun{Status: "active", CurrentEventID: "service_request",
		ActiveEventIDs: []string{"service_request", "seat_conflict"}, Location: "passenger_zone",
		GameTimeS: 55, DeadlineAt: &due, Flags: map[string]bool{}, Loyalty: 80, Safety: 100}
	next, err := template.Apply(run, Command{ChoiceID: "check_availability"})
	if err != nil || !next.TimedOut || next.CurrentEventID != "service_window_closed" || next.Flags["availability_checked"] || len(next.ActionLog) != 1 {
		t.Fatalf("late choice beat timeout: %+v, %v", next, err)
	}
}
