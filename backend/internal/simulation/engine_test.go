package simulation

import (
	"strings"
	"testing"
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
