package service

import (
	"strings"
	"testing"

	"github.com/mostransport/vsm-trainer/internal/content"
)

func TestClamp(t *testing.T) {
	if clamp(120, 0, 100) != 100 || clamp(-5, 0, 100) != 0 || clamp(42, 0, 100) != 42 {
		t.Fatal("clamp failed")
	}
}

func TestBuildSystemPromptUsesCatalogFields(t *testing.T) {
	prompt := buildSystemPrompt("тревожный пассажир", "ru", "плохо слышит", "В вагоне запах дыма")
	for _, want := range []string{"тревожный пассажир", "ru", "плохо слышит", "В вагоне запах дыма"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q lacks %q", prompt, want)
		}
	}
}

func TestPickScenarios(t *testing.T) {
	catalog, err := content.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := pickScenarios(catalog.Scenarios, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 scenarios, got %d", len(got))
	}
	seen := map[string]bool{}
	for _, scenario := range got {
		if seen[scenario.ID] {
			t.Fatalf("duplicate scenario %s", scenario.ID)
		}
		seen[scenario.ID] = true
	}
}
