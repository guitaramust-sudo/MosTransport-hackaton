package content

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed scenarios.json passengers.json
var files embed.FS

type ScenarioType string

const (
	TypeService       ScenarioType = "service"
	TypeConflict      ScenarioType = "conflict"
	TypeMedical       ScenarioType = "medical"
	TypeSafety        ScenarioType = "safety"
	TypeInformational ScenarioType = "informational"
)

type Criticality string

const (
	CritCritical Criticality = "critical"
	CritHigh     Criticality = "high"
	CritMedium   Criticality = "medium"
	CritLow      Criticality = "low"
)

const (
	TargetTrainChief = "train_chief"
	TargetPTB        = "ptb"
	TargetPolice     = "police"
	TargetMedic      = "medic"
	TargetAmbulance  = "ambulance"
)

type MustConvey struct {
	ID   string `json:"id"`
	Desc string `json:"desc"`
	// EscalationTarget marks a point that is satisfied by escalating to the
	// given address (e.g. "Сообщить начальнику поезда" is satisfied by calling
	// train_chief). Empty means the point is independent of escalation.
	EscalationTarget string `json:"escalation_target,omitempty"`
}

type Escalation struct {
	Required bool     `json:"required"`
	To       []string `json:"to"`
}

type CorrectCompletion struct {
	MustConvey []MustConvey `json:"must_convey"`
	Escalation Escalation   `json:"escalation"`
}

type Scenario struct {
	ID                string            `json:"id"`
	Type              ScenarioType      `json:"type"`
	Criticality       Criticality       `json:"criticality"`
	ValidationStatus  string            `json:"validation_status"`
	ReviewerID        string            `json:"reviewer_id,omitempty"`
	SourceRefs        []string          `json:"source_refs,omitempty"`
	Title             string            `json:"title"`
	Opening           string            `json:"opening"`
	CorrectCompletion CorrectCompletion `json:"correct_completion"`
	TimeLimitSec      int               `json:"time_limit_sec"`
}

type Passenger struct {
	ID         string   `json:"id"`
	Age        string   `json:"age"`
	Tone       string   `json:"tone"`
	Language   string   `json:"language"`
	Traits     []string `json:"traits"`
	PromptHint string   `json:"prompt_hint"`
}

// Catalog is immutable after loading. Callers can pick a scenario and a
// passenger independently, giving the scenario engine a combinatorial pool.
type Catalog struct {
	Scenarios  []Scenario
	Passengers []Passenger
}

func Load() (Catalog, error) {
	scenarios, err := files.ReadFile("scenarios.json")
	if err != nil {
		return Catalog{}, err
	}
	passengers, err := files.ReadFile("passengers.json")
	if err != nil {
		return Catalog{}, err
	}
	return Parse(scenarios, passengers)
}

// Parse also allows editors and tests to validate a candidate catalog before
// embedding it in a build.
func Parse(scenariosJSON, passengersJSON []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(scenariosJSON, &c.Scenarios); err != nil {
		return Catalog{}, fmt.Errorf("scenarios.json: %w", err)
	}
	if err := json.Unmarshal(passengersJSON, &c.Passengers); err != nil {
		return Catalog{}, fmt.Errorf("passengers.json: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

func (c Catalog) Validate() error {
	if len(c.Scenarios) == 0 || len(c.Passengers) == 0 {
		return fmt.Errorf("catalog needs scenarios and passengers")
	}
	scenarioIDs := map[string]bool{}
	for i, s := range c.Scenarios {
		where := fmt.Sprintf("scenario[%d]", i)
		if blank(s.ID) || scenarioIDs[s.ID] {
			return fmt.Errorf("%s: empty or duplicate id %q", where, s.ID)
		}
		scenarioIDs[s.ID] = true
		if !oneOf(string(s.Type), "service", "conflict", "medical", "safety", "informational") {
			return fmt.Errorf("%s: invalid type %q", where, s.Type)
		}
		if !oneOf(string(s.Criticality), "critical", "high", "medium", "low") {
			return fmt.Errorf("%s: invalid criticality %q", where, s.Criticality)
		}
		if !oneOf(s.ValidationStatus, "draft", "approved", "blocked") {
			return fmt.Errorf("%s: invalid validation_status %q", where, s.ValidationStatus)
		}
		if s.ValidationStatus == "approved" && (blank(s.ReviewerID) || len(s.SourceRefs) == 0) {
			return fmt.Errorf("%s: approved scenario requires reviewer_id and source_refs", where)
		}
		if blank(s.Title) || blank(s.Opening) || s.TimeLimitSec <= 0 {
			return fmt.Errorf("%s: title, opening and positive time_limit_sec required", where)
		}
		if len(s.CorrectCompletion.MustConvey) == 0 {
			return fmt.Errorf("%s: must_convey must be nonempty", where)
		}
		pointIDs := map[string]bool{}
		for _, p := range s.CorrectCompletion.MustConvey {
			if blank(p.ID) || blank(p.Desc) || pointIDs[p.ID] {
				return fmt.Errorf("%s: empty or duplicate must_convey id/desc %q", where, p.ID)
			}
			pointIDs[p.ID] = true
			if p.EscalationTarget != "" && !oneOf(p.EscalationTarget, TargetTrainChief, TargetPTB, TargetPolice, TargetMedic, TargetAmbulance) {
				return fmt.Errorf("%s: invalid escalation_target %q", where, p.EscalationTarget)
			}
		}
		targets := map[string]bool{}
		for _, target := range s.CorrectCompletion.Escalation.To {
			if !oneOf(target, TargetTrainChief, TargetPTB, TargetPolice, TargetMedic, TargetAmbulance) || targets[target] {
				return fmt.Errorf("%s: invalid or duplicate escalation target %q", where, target)
			}
			targets[target] = true
		}
		if s.CorrectCompletion.Escalation.Required && len(targets) == 0 {
			return fmt.Errorf("%s: required escalation needs a target", where)
		}
	}
	passengerIDs := map[string]bool{}
	for i, p := range c.Passengers {
		where := fmt.Sprintf("passenger[%d]", i)
		if blank(p.ID) || passengerIDs[p.ID] {
			return fmt.Errorf("%s: empty or duplicate id %q", where, p.ID)
		}
		passengerIDs[p.ID] = true
		if !oneOf(p.Age, "child", "young", "middle", "elderly") ||
			!oneOf(p.Tone, "calm", "anxious", "aggressive", "confused", "indifferent") ||
			!oneOf(p.Language, "ru", "en") {
			return fmt.Errorf("%s: invalid age, tone or language", where)
		}
		if blank(p.PromptHint) {
			return fmt.Errorf("%s: prompt_hint required", where)
		}
		for _, trait := range p.Traits {
			if blank(trait) {
				return fmt.Errorf("%s: empty trait", where)
			}
		}
	}
	return nil
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
