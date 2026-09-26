package simulation

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"

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
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Choices []Choice `json:"choices"`
}

type Template struct {
	ID               string   `json:"id"`
	Version          string   `json:"version"`
	ValidationStatus string   `json:"validation_status"`
	ReviewerID       string   `json:"reviewer_id,omitempty"`
	SourceRefs       []string `json:"source_refs,omitempty"`
	StartEvent       string   `json:"start_event"`
	Events           []Event  `json:"events"`
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
	if !events[t.StartEvent] {
		return errors.New("start event is missing")
	}
	for _, event := range t.Events {
		choices := map[string]bool{}
		for _, choice := range event.Choices {
			if choice.ID == "" || choices[choice.ID] || choice.Explanation == "" || (choice.NextEvent != "" && !events[choice.NextEvent]) {
				return fmt.Errorf("invalid choice %q in event %q", choice.ID, event.ID)
			}
			choices[choice.ID] = true
		}
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

func (t Template) Apply(run domain.SimulationRun, choiceID string) (domain.SimulationRun, error) {
	if run.Status != "active" {
		return run, ErrFinished
	}
	event, ok := t.Event(run.CurrentEventID)
	if !ok {
		return run, ErrInvalidChoice
	}
	for _, choice := range event.Choices {
		if choice.ID != choiceID {
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
		run.Loyalty = clamp(run.Loyalty + choice.LoyaltyDelta)
		run.Safety = clamp(run.Safety + choice.SafetyDelta)
		run.Path = append(run.Path, event.ID+":"+choice.ID)
		run.CurrentEventID = choice.NextEvent
		if choice.NextEvent == "" {
			run.Status = "finished"
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
