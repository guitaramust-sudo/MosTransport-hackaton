package service

import (
	"context"
	"encoding/json"

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

type SimulationView struct {
	Run   SimulationStateView  `json:"run"`
	Event *SimulationEventView `json:"event,omitempty"`
}

type SimulationStateView struct {
	ID                      uuid.UUID `json:"id"`
	ScenarioID              string    `json:"scenario_id"`
	ScenarioVersion         string    `json:"scenario_version"`
	ContentValidationStatus string    `json:"content_validation_status"`
	StateVersion            int       `json:"state_version"`
	Status                  string    `json:"status"`
	Loyalty                 int       `json:"loyalty"`
	Safety                  int       `json:"safety"`
	Path                    []string  `json:"path"`
}

type SimulationChoiceView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type SimulationEventView struct {
	ID      string                 `json:"id"`
	Text    string                 `json:"text"`
	Choices []SimulationChoiceView `json:"choices"`
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
	run, err := s.store.CreateSimulationRun(ctx, domain.SimulationRun{
		PlayerID: playerID, ScenarioID: s.template.ID, ScenarioVersion: s.template.Version,
		Status: "active", CurrentEventID: s.template.StartEvent,
		Flags: map[string]bool{}, Loyalty: 80, Safety: 100, Path: []string{}, TemplateSnapshot: snapshot,
	})
	if err != nil {
		return SimulationView{}, err
	}
	return s.view(run)
}

func (s *SimulationService) Get(ctx context.Context, playerID, runID uuid.UUID) (SimulationView, error) {
	run, err := s.store.GetSimulationRun(ctx, runID, playerID)
	if err != nil {
		return SimulationView{}, err
	}
	return s.view(run)
}

func (s *SimulationService) Action(ctx context.Context, playerID, runID, commandID uuid.UUID, version int, choiceID string) (SimulationView, error) {
	run, err := s.store.ApplySimulationCommand(ctx, runID, playerID, commandID, version,
		func(current domain.SimulationRun) (domain.SimulationRun, error) {
			template, err := s.templateFor(current)
			if err != nil {
				return current, err
			}
			return template.Apply(current, choiceID)
		})
	if err != nil {
		return SimulationView{}, err
	}
	return s.view(run)
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
	}}
	if event, ok := template.Event(run.CurrentEventID); ok {
		available := make([]SimulationChoiceView, 0, len(event.Choices))
		for _, choice := range event.Choices {
			allowed := true
			for _, required := range choice.Requires {
				if !run.Flags[required] {
					allowed = false
				}
			}
			if allowed {
				available = append(available, SimulationChoiceView{ID: choice.ID, Text: choice.Text})
			}
		}
		view.Event = &SimulationEventView{ID: event.ID, Text: event.Text, Choices: available}
	}
	return view, nil
}
