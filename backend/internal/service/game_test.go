package service

import (
	"context"
	"testing"

	"github.com/mostransport/vsm-trainer/internal/llm"
)

func TestNormalizeCategory(t *testing.T) {
	cases := map[string]string{
		"эмпатия":           "эмпатия",
		"Давление":          "давление",
		"  вызов помощи  ":  "вызов помощи",
		"неверное действие": "неверное действие",
		"что-то странное":   "эмпатия",
	}
	for in, want := range cases {
		if got := normalizeCategory(in); got != want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOutcomeLabel(t *testing.T) {
	cases := []struct {
		l, s int
		want string
	}{
		{80, 80, "resolved_positive"},
		{50, 50, "resolved_neutral"},
		{10, 20, "resolved_negative"},
		{70, 70, "resolved_positive"},
	}
	for _, c := range cases {
		if got := outcomeLabel(c.l, c.s); got != c.want {
			t.Errorf("outcomeLabel(%d,%d) = %s, want %s", c.l, c.s, got, c.want)
		}
	}
}

func TestClamp(t *testing.T) {
	if clamp(120, 0, 100) != 100 {
		t.Error("clamp upper bound failed")
	}
	if clamp(-5, 0, 100) != 0 {
		t.Error("clamp lower bound failed")
	}
	if clamp(42, 0, 100) != 42 {
		t.Error("clamp passthrough failed")
	}
}

func TestPickArchetypes(t *testing.T) {
	got := pickArchetypes(3)
	if len(got) != 3 {
		t.Fatalf("expected 3 archetypes, got %d", len(got))
	}
	seen := map[string]bool{}
	for _, a := range got {
		if seen[a.Code] {
			t.Fatalf("duplicate archetype %s", a.Code)
		}
		seen[a.Code] = true
	}
}

func TestMockLLMClassify(t *testing.T) {
	m := llm.NewMockLLM()
	got, err := m.Classify(context.Background(), "Пожалуйста, не переживайте, я помогу вам.", Categories)
	if err != nil {
		t.Fatal(err)
	}
	if got != "эмпатия" {
		t.Fatalf("expected эмпатия, got %s", got)
	}

	got, err = m.Classify(context.Background(), "Немедленно покажите билет, иначе я вызову полицию!", Categories)
	if err != nil {
		t.Fatal(err)
	}
	if got != "давление" {
		t.Fatalf("expected давление, got %s", got)
	}
}
