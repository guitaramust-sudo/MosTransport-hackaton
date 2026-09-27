package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mostransport/vsm-trainer/internal/content"
)

func TestLegacyShiftDoesNotSelectWagonOnlyPhysicalCases(t *testing.T) {
	catalog := content.Catalog{Scenarios: []content.Scenario{{ID: "cold", Type: content.TypeService, Criticality: content.CritLow, ValidationStatus: "draft", Title: "Cold", Opening: "O", TimeLimitSec: 90,
		CorrectCompletion: content.CorrectCompletion{MustConvey: []content.MustConvey{{ID: "B", Desc: "Blanket", PhysicalAction: true}}}, PhysicalRequirement: &content.PhysicalRequirement{Kind: "deliver_item", Item: "blanket"}}},
		Passengers: []content.Passenger{{ID: "p", Age: "middle", Tone: "calm", Language: "ru", PromptHint: "H"}}}
	svc := NewSessionService(nil, catalog, 1, nil, "demo")
	_, _, err := svc.Start(context.Background(), uuid.New())
	if !errors.Is(err, ErrNoEligibleScenarios) {
		t.Fatalf("legacy flow selected an unsolvable physical case: %v", err)
	}
}
