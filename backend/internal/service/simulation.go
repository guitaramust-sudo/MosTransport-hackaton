package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
	"github.com/mostransport/vsm-trainer/internal/simulation"
)

type SimulationService struct {
	store     repo.SimulationStore
	template  simulation.Template
	namespace string
}

var ErrSimulationActive = errors.New("simulation is still active")

type SimulationView struct {
	Run            SimulationStateView   `json:"run"`
	Event          *SimulationEventView  `json:"event,omitempty"`
	Events         []SimulationEventView `json:"events"`
	ObservableCues []string              `json:"observable_cues"`
}

type SimulationStateView struct {
	ID                      uuid.UUID  `json:"id"`
	ScenarioID              string     `json:"scenario_id"`
	ScenarioVersion         string     `json:"scenario_version"`
	ContentValidationStatus string     `json:"content_validation_status"`
	StateVersion            int        `json:"state_version"`
	Status                  string     `json:"status"`
	Loyalty                 int        `json:"loyalty"`
	Safety                  int        `json:"safety"`
	Path                    []string   `json:"path"`
	Location                string     `json:"location"`
	GameTimeS               int        `json:"game_time_s"`
	SeedVariant             string     `json:"seed_variant"`
	DeadlineAt              *time.Time `json:"deadline_at,omitempty"`
	TimedOut                bool       `json:"timed_out"`
}

type SimulationChoiceView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type SimulationEventView struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Location string                 `json:"location"`
	Choices  []SimulationChoiceView `json:"choices"`
}

func NewSimulationService(store repo.SimulationStore, template simulation.Template, namespace string) *SimulationService {
	return &SimulationService{store: store, template: template, namespace: namespace}
}

func (s *SimulationService) Start(ctx context.Context, playerID uuid.UUID) (SimulationView, error) {
	if s.template.ValidationStatus == "blocked" || (s.namespace != "demo" && s.template.ValidationStatus != "approved") {
		return SimulationView{}, ErrNoEligibleScenarios
	}
	snapshot, err := json.Marshal(s.template)
	if err != nil {
		return SimulationView{}, err
	}
	now := time.Now().UTC()
	var deadline *time.Time
	if s.template.Timer != nil {
		due := now.Add(time.Duration(s.template.Timer.DurationS) * time.Second)
		deadline = &due
	}
	run, err := s.store.CreateSimulationRun(ctx, domain.SimulationRun{
		PlayerID: playerID, ScenarioID: s.template.ID, ScenarioVersion: s.template.Version,
		Status: "active", CurrentEventID: s.template.StartEvent,
		ActiveEventIDs: append([]string(nil), s.template.StartEvents...), ObservedEvents: map[string]bool{},
		Location: s.template.StartLocation, Flags: map[string]bool{}, Loyalty: 80, Safety: 100,
		Path: []string{}, StartedAt: now, DeadlineAt: deadline, ActionLog: []domain.SimulationLogEntry{}, TemplateSnapshot: snapshot,
	})
	if err != nil {
		return SimulationView{}, err
	}
	return s.view(run)
}

func (s *SimulationService) Get(ctx context.Context, playerID, runID uuid.UUID) (SimulationView, error) {
	run, err := s.advanceTimer(ctx, playerID, runID, time.Now())
	if err != nil {
		return SimulationView{}, err
	}
	return s.view(run)
}

func (s *SimulationService) Action(ctx context.Context, playerID, runID, commandID uuid.UUID, version int, choiceID string) (SimulationView, error) {
	return s.ActionCommand(ctx, playerID, runID, commandID, version, simulation.Command{ChoiceID: choiceID})
}

func (s *SimulationService) ActionCommand(ctx context.Context, playerID, runID, commandID uuid.UUID, version int, command simulation.Command) (SimulationView, error) {
	if _, err := s.advanceTimer(ctx, playerID, runID, time.Now()); err != nil {
		return SimulationView{}, err
	}
	command.CommandID = commandID
	run, err := s.store.ApplySimulationCommand(ctx, runID, playerID, commandID, version,
		func(current domain.SimulationRun) (domain.SimulationRun, error) {
			template, err := s.templateFor(current)
			if err != nil {
				return current, err
			}
			if due, changed := template.ApplyDue(current, time.Now()); changed {
				return due, nil
			}
			next, err := template.Apply(current, command)
			if err == nil && next.Status == "finished" && current.Status != "finished" {
				now := time.Now().UTC()
				next.FinishedAt = &now
				next.Passed = simulationPass(next)
			}
			return next, err
		})
	if err != nil {
		return SimulationView{}, err
	}
	if run.Status == "finished" {
		if err := s.store.FinalizeSimulationRewards(ctx, run); err != nil {
			return SimulationView{}, err
		}
	}
	return s.view(run)
}

func (s *SimulationService) Notifications(ctx context.Context, playerID uuid.UUID) ([]domain.Notification, error) {
	return s.store.ListNotifications(ctx, playerID)
}

func (s *SimulationService) Challenge(ctx context.Context, playerID uuid.UUID) (domain.ChallengeProgress, error) {
	return s.store.GetChallengeProgress(ctx, playerID, time.Now())
}

type SimulationDebriefEntry struct {
	domain.SimulationLogEntry
	BetterOptions []string `json:"better_options"`
}

type SimulationResult struct {
	SessionID          uuid.UUID                `json:"session_id"`
	ScenarioVersion    string                   `json:"scenario_version"`
	ScoringRuleVersion string                   `json:"scoring_rule_version"`
	ValidationStatus   string                   `json:"validation_status"`
	CompletedAt        *time.Time               `json:"completed_at"`
	WorldSafetyCurrent int                      `json:"world_safety_current"`
	SessionSafetyScore int                      `json:"session_safety_score"`
	Loyalty            int                      `json:"loyalty"`
	TimedOut           bool                     `json:"timed_out"`
	SessionPass        bool                     `json:"session_pass"`
	ActionLogHash      string                   `json:"action_log_hash"`
	Debrief            []SimulationDebriefEntry `json:"debrief"`
}

func (s *SimulationService) Result(ctx context.Context, playerID, runID uuid.UUID) (SimulationResult, error) {
	run, err := s.advanceTimer(ctx, playerID, runID, time.Now())
	if err != nil {
		return SimulationResult{}, err
	}
	if run.Status != "finished" {
		return SimulationResult{}, ErrSimulationActive
	}
	if err := s.store.FinalizeSimulationRewards(ctx, run); err != nil {
		return SimulationResult{}, err
	}
	template, err := s.templateFor(run)
	if err != nil {
		return SimulationResult{}, err
	}
	raw, err := json.Marshal(run.ActionLog)
	if err != nil {
		return SimulationResult{}, err
	}
	debrief := make([]SimulationDebriefEntry, 0, len(run.ActionLog))
	for _, entry := range run.ActionLog {
		row := SimulationDebriefEntry{SimulationLogEntry: entry, BetterOptions: []string{}}
		if event, ok := template.Event(entry.EventID); ok {
			var chosen *simulation.Choice
			for i := range event.Choices {
				if event.Choices[i].ID == entry.EffectID {
					chosen = &event.Choices[i]
					break
				}
			}
			for _, choice := range event.Choices {
				if entry.ActionID == "timeout" || (chosen != nil && choice.ID != entry.EffectID && (choice.SafetyDelta > chosen.SafetyDelta || (choice.SafetyDelta == chosen.SafetyDelta && choice.LoyaltyDelta > chosen.LoyaltyDelta))) {
					row.BetterOptions = append(row.BetterOptions, choice.Text)
				}
			}
		}
		debrief = append(debrief, row)
	}
	safetyScore := simulationSafetyScore(run.ActionLog)
	return SimulationResult{SessionID: run.ID, ScenarioVersion: run.ScenarioVersion,
		ScoringRuleVersion: "simulation-demo-v1", ValidationStatus: template.ValidationStatus,
		CompletedAt: run.FinishedAt, WorldSafetyCurrent: run.Safety, SessionSafetyScore: safetyScore,
		Loyalty: run.Loyalty, TimedOut: run.TimedOut,
		SessionPass:   simulationPass(run),
		ActionLogHash: fmt.Sprintf("%x", sha256.Sum256(raw)), Debrief: debrief}, nil
}

func simulationSafetyScore(log []domain.SimulationLogEntry) int {
	negative := 0
	for _, entry := range log {
		if entry.SafetyAfter < entry.SafetyBefore {
			negative += entry.SafetyAfter - entry.SafetyBefore
		}
	}
	return clamp(100+negative, 0, 100)
}

func simulationPass(run domain.SimulationRun) bool {
	return run.Status == "finished" && !run.TimedOut && simulationSafetyScore(run.ActionLog) >= 90 && run.Safety >= 90 && run.Loyalty >= 60
}

func (s *SimulationService) advanceTimer(ctx context.Context, playerID, runID uuid.UUID, now time.Time) (domain.SimulationRun, error) {
	return s.store.AdvanceSimulationTimer(ctx, runID, playerID, func(run domain.SimulationRun) (domain.SimulationRun, bool) {
		template, err := s.templateFor(run)
		if err != nil {
			return run, false
		}
		return template.ApplyDue(run, now)
	})
}

func (s *SimulationService) CloseExpired(ctx context.Context) error {
	runs, err := s.store.ListDueSimulationRuns(ctx, time.Now())
	if err != nil {
		return err
	}
	for _, run := range runs {
		if _, err := s.advanceTimer(ctx, run.PlayerID, run.ID, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func (s *SimulationService) templateFor(run domain.SimulationRun) (simulation.Template, error) {
	var template simulation.Template
	if err := json.Unmarshal(run.TemplateSnapshot, &template); err != nil {
		return simulation.Template{}, err
	}
	return template, nil
}

func (s *SimulationService) view(run domain.SimulationRun) (SimulationView, error) {
	template, err := s.templateFor(run)
	if err != nil {
		return SimulationView{}, err
	}
	view := SimulationView{Run: SimulationStateView{
		ID: run.ID, ScenarioID: run.ScenarioID, ScenarioVersion: run.ScenarioVersion,
		ContentValidationStatus: template.ValidationStatus, StateVersion: run.StateVersion,
		Status: run.Status, Loyalty: run.Loyalty, Safety: run.Safety, Path: run.Path,
		Location: run.Location, GameTimeS: run.GameTimeS, SeedVariant: run.SeedVariant,
		DeadlineAt: run.DeadlineAt, TimedOut: run.TimedOut,
	}}
	view.Events = []SimulationEventView{}
	view.ObservableCues = []string{}
	active := run.ActiveEventIDs
	if len(active) == 0 && run.CurrentEventID != "" {
		active = []string{run.CurrentEventID}
	}
	for _, id := range active {
		event, ok := template.Event(id)
		if !ok {
			continue
		}
		if event.Hidden && !run.ObservedEvents[id] {
			if event.Location == run.Location {
				view.ObservableCues = append(view.ObservableCues, event.Cue)
			}
			continue
		}
		available := make([]SimulationChoiceView, 0, len(event.Choices))
		for _, choice := range event.Choices {
			allowed := event.Location == run.Location
			for _, required := range choice.Requires {
				if !run.Flags[required] {
					allowed = false
				}
			}
			if allowed {
				available = append(available, SimulationChoiceView{ID: choice.ID, Text: choice.Text})
			}
		}
		visible := SimulationEventView{ID: event.ID, Text: event.Text, Location: event.Location, Choices: available}
		view.Events = append(view.Events, visible)
		if view.Event == nil || (event.Location == run.Location && view.Event.Location != run.Location) {
			selected := visible
			view.Event = &selected
		}
	}
	return view, nil
}
