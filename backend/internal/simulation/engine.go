package simulation

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

//go:embed demo.json
var templates embed.FS

var ErrInvalidChoice = errors.New("choice unavailable in current event")
var ErrFinished = errors.New("simulation already finished")

type Choice struct {
	ID           string          `json:"id"`
	Text         string          `json:"text"`
	NextEvent    string          `json:"next_event,omitempty"`
	Requires     []string        `json:"requires,omitempty"`
	Effects      map[string]bool `json:"effects,omitempty"`
	LoyaltyDelta int             `json:"loyalty_delta,omitempty"`
	SafetyDelta  int             `json:"safety_delta,omitempty"`
	Explanation  string          `json:"explanation"`
}

type Event struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Location string   `json:"location"`
	Hidden   bool     `json:"hidden,omitempty"`
	Cue      string   `json:"cue,omitempty"`
	Choices  []Choice `json:"choices"`
}

type Edge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	TravelS int    `json:"travel_s"`
}

type Template struct {
	ID               string   `json:"id"`
	Version          string   `json:"version"`
	ValidationStatus string   `json:"validation_status"`
	ReviewerID       string   `json:"reviewer_id,omitempty"`
	SourceRefs       []string `json:"source_refs,omitempty"`
	StartEvent       string   `json:"start_event"`
	StartEvents      []string `json:"start_events"`
	StartLocation    string   `json:"start_location"`
	Edges            []Edge   `json:"edges"`
	Timer            *Timer   `json:"timer,omitempty"`
	Events           []Event  `json:"events"`
}

type Timer struct {
	EventID          string `json:"event_id"`
	DurationS        int    `json:"duration_s"`
	TimeoutNextEvent string `json:"timeout_next_event"`
	Explanation      string `json:"explanation"`
}

func Load() (Template, error) {
	raw, err := templates.ReadFile("demo.json")
	if err != nil {
		return Template{}, err
	}
	var template Template
	if err := json.Unmarshal(raw, &template); err != nil {
		return Template{}, err
	}
	return template, template.Validate()
}

func (t Template) Validate() error {
	if t.ID == "" || t.Version == "" || t.StartEvent == "" || (t.ValidationStatus != "draft" && t.ValidationStatus != "approved" && t.ValidationStatus != "blocked") {
		return errors.New("simulation template metadata is incomplete")
	}
	if t.ValidationStatus == "approved" && (t.ReviewerID == "" || len(t.SourceRefs) == 0) {
		return errors.New("approved simulation needs reviewer and source references")
	}
	events := map[string]bool{}
	for _, event := range t.Events {
		if event.ID == "" || events[event.ID] || len(event.Choices) == 0 {
			return fmt.Errorf("invalid event %q", event.ID)
		}
		events[event.ID] = true
	}
	if !events[t.StartEvent] || t.StartLocation == "" || len(t.StartEvents) < 2 {
		return errors.New("start event is missing")
	}
	starts := map[string]bool{}
	for _, id := range t.StartEvents {
		if !events[id] || starts[id] {
			return fmt.Errorf("start event %q is missing", id)
		}
		starts[id] = true
	}
	if !starts[t.StartEvent] {
		return errors.New("primary start event is not active")
	}
	reachable := map[string]bool{t.StartLocation: true}
	for changed := true; changed; {
		changed = false
		for _, edge := range t.Edges {
			if reachable[edge.From] && !reachable[edge.To] {
				reachable[edge.To], changed = true, true
			}
			if reachable[edge.To] && !reachable[edge.From] {
				reachable[edge.From], changed = true, true
			}
		}
	}
	for _, edge := range t.Edges {
		if edge.From == "" || edge.To == "" || edge.TravelS <= 0 {
			return errors.New("invalid travel edge")
		}
	}
	for _, event := range t.Events {
		if event.Location == "" || (event.Hidden && event.Cue == "") {
			return fmt.Errorf("event %q needs location and hidden cue", event.ID)
		}
		if !reachable[event.Location] {
			return fmt.Errorf("event %q has unreachable location %q", event.ID, event.Location)
		}
		choices := map[string]bool{}
		for _, choice := range event.Choices {
			if choice.ID == "" || choices[choice.ID] || choice.Explanation == "" || (choice.NextEvent != "" && !events[choice.NextEvent]) {
				return fmt.Errorf("invalid choice %q in event %q", choice.ID, event.ID)
			}
			choices[choice.ID] = true
		}
	}
	if t.Timer != nil && (!events[t.Timer.EventID] || !events[t.Timer.TimeoutNextEvent] || t.Timer.DurationS <= 0 || t.Timer.Explanation == "") {
		return errors.New("invalid timer configuration")
	}
	return nil
}

func (t Template) Event(id string) (Event, bool) {
	for _, event := range t.Events {
		if event.ID == id {
			return event, true
		}
	}
	return Event{}, false
}

type Command struct {
	CommandID uuid.UUID
	ActionID  string
	Target    string
	EventID   string
	ChoiceID  string
}

func (t Template) ApplyDue(run domain.SimulationRun, now time.Time) (domain.SimulationRun, bool) {
	if t.Timer == nil || run.TimedOut || run.Status != "active" {
		return run, false
	}
	if run.GameTimeS < t.Timer.DurationS && (run.DeadlineAt == nil || !now.After(*run.DeadlineAt)) {
		return run, false
	}
	for i, id := range run.ActiveEventIDs {
		if id != t.Timer.EventID {
			continue
		}
		run.ActiveEventIDs[i] = t.Timer.TimeoutNextEvent
		if run.CurrentEventID == id {
			run.CurrentEventID = t.Timer.TimeoutNextEvent
		}
		run.TimedOut = true
		run.DeadlineAt = nil
		if run.Flags == nil {
			run.Flags = map[string]bool{}
		}
		run.Flags["service_window_closed"] = true
		if run.GameTimeS < t.Timer.DurationS {
			run.GameTimeS = t.Timer.DurationS
		}
		run.ActionLog = append(run.ActionLog, domain.SimulationLogEntry{
			EventID: id, ActionID: "timeout", EffectID: "service_window_closed",
			Explanation: t.Timer.Explanation, AtGameTimeS: run.GameTimeS,
			LoyaltyBefore: run.Loyalty, LoyaltyAfter: run.Loyalty,
			SafetyBefore: run.Safety, SafetyAfter: run.Safety,
		})
		return run, true
	}
	run.DeadlineAt = nil
	return run, false
}

func (t Template) Apply(run domain.SimulationRun, command Command) (domain.SimulationRun, error) {
	if run.Status != "active" {
		return run, ErrFinished
	}
	if len(run.ActiveEventIDs) == 0 && run.CurrentEventID != "" {
		run.ActiveEventIDs = []string{run.CurrentEventID}
	}
	if t.Timer != nil && !run.TimedOut && run.GameTimeS < t.Timer.DurationS {
		cost := 0
		switch command.ActionID {
		case "move_to":
			for _, edge := range t.Edges {
				if (edge.From == run.Location && edge.To == command.Target) || (edge.To == run.Location && edge.From == command.Target) {
					cost = edge.TravelS
				}
			}
		case "inspect":
			cost = 5
		case "", "choose":
			cost = 6
		}
		if run.GameTimeS+cost > t.Timer.DurationS {
			for _, id := range run.ActiveEventIDs {
				if id == t.Timer.EventID {
					run.GameTimeS = t.Timer.DurationS
					if due, changed := t.ApplyDue(run, time.Time{}); changed {
						return due, nil
					}
				}
			}
		}
	}
	switch command.ActionID {
	case "move_to":
		for _, edge := range t.Edges {
			if (edge.From == run.Location && edge.To == command.Target) || (edge.To == run.Location && edge.From == command.Target) {
				run.Location = command.Target
				run.GameTimeS += edge.TravelS
				run.Path = append(run.Path, "move_to:"+command.Target)
				run.ActionLog = append(run.ActionLog, domain.SimulationLogEntry{CommandID: command.CommandID,
					ActionID: "move_to", EffectID: "location_changed", Explanation: "Переход в другую зону занял игровое время.",
					AtGameTimeS: run.GameTimeS, LoyaltyBefore: run.Loyalty, LoyaltyAfter: run.Loyalty, SafetyBefore: run.Safety, SafetyAfter: run.Safety})
				return run, nil
			}
		}
		return run, ErrInvalidChoice
	case "inspect":
		found := false
		if run.ObservedEvents == nil {
			run.ObservedEvents = map[string]bool{}
		}
		for _, id := range run.ActiveEventIDs {
			event, ok := t.Event(id)
			if ok && event.Hidden && event.Location == run.Location && !run.ObservedEvents[id] {
				run.ObservedEvents[id] = true
				found = true
			}
		}
		if !found {
			return run, ErrInvalidChoice
		}
		run.GameTimeS += 5
		run.Path = append(run.Path, "inspect:"+run.Location)
		run.ActionLog = append(run.ActionLog, domain.SimulationLogEntry{CommandID: command.CommandID,
			ActionID: "inspect", EffectID: "cue_discovered", Explanation: "Осмотр раскрыл наблюдаемый сигнал в текущей зоне.",
			AtGameTimeS: run.GameTimeS, LoyaltyBefore: run.Loyalty, LoyaltyAfter: run.Loyalty, SafetyBefore: run.Safety, SafetyAfter: run.Safety})
		return run, nil
	case "", "choose":
	default:
		return run, ErrInvalidChoice
	}
	eventID := command.EventID
	if eventID == "" {
		eventID = run.CurrentEventID
	}
	index := -1
	for i, id := range run.ActiveEventIDs {
		if id == eventID {
			index = i
			break
		}
	}
	if index < 0 {
		return run, ErrInvalidChoice
	}
	event, ok := t.Event(eventID)
	if !ok || event.Location != run.Location || (event.Hidden && !run.ObservedEvents[event.ID]) {
		return run, ErrInvalidChoice
	}
	for _, choice := range event.Choices {
		if choice.ID != command.ChoiceID {
			continue
		}
		for _, required := range choice.Requires {
			if !run.Flags[required] {
				return run, ErrInvalidChoice
			}
		}
		if run.Flags == nil {
			run.Flags = map[string]bool{}
		}
		for key, value := range choice.Effects {
			run.Flags[key] = value
		}
		beforeLoyalty, beforeSafety := run.Loyalty, run.Safety
		run.Loyalty = clamp(run.Loyalty + choice.LoyaltyDelta)
		run.Safety = clamp(run.Safety + choice.SafetyDelta)
		run.GameTimeS += 6
		run.Path = append(run.Path, event.ID+":"+choice.ID)
		run.ActionLog = append(run.ActionLog, domain.SimulationLogEntry{CommandID: command.CommandID,
			EventID: event.ID, ActionID: "choose", EffectID: choice.ID, Explanation: choice.Explanation,
			AtGameTimeS: run.GameTimeS, LoyaltyBefore: beforeLoyalty, LoyaltyAfter: run.Loyalty,
			SafetyBefore: beforeSafety, SafetyAfter: run.Safety})
		if t.Timer != nil && event.ID == t.Timer.EventID {
			run.DeadlineAt = nil
		}
		if choice.NextEvent == "" {
			run.ActiveEventIDs = append(run.ActiveEventIDs[:index], run.ActiveEventIDs[index+1:]...)
		} else {
			run.ActiveEventIDs[index] = choice.NextEvent
		}
		if len(run.ActiveEventIDs) == 0 {
			run.Status = "finished"
			run.CurrentEventID = ""
		} else {
			run.CurrentEventID = run.ActiveEventIDs[0]
		}
		return run, nil
	}
	return run, ErrInvalidChoice
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
