package llm

import (
	"context"
	"time"

	"github.com/mostransport/vsm-trainer/internal/content"
)

// Message is a single chat turn sent to an LLM.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

type ScoringInput struct {
	Scenario    content.Scenario
	Passenger   content.Passenger
	History     []Message
	Escalations []string
	Elapsed     time.Duration
}

// ScoreResult records model observations. Game rules use the conveyed points
// and tone; escalation and outcome are always resolved by deterministic code.
type ScoreResult struct {
	Conveyed       []string `json:"conveyed"`
	Missed         []string `json:"missed"`
	Tone           string   `json:"tone"`
	EscalationDone []string `json:"escalation_done"`
	EscalationOK   bool     `json:"escalation_ok"`
	Reasoning      string   `json:"reasoning"`
}

// LLMClient is the single abstraction over any language model backend.
// The service layer talks only to this interface, so GigaChat can be
// swapped for a mock (offline dev / demos) without touching game logic.
type LLMClient interface {
	// Chat returns the model's next message given the full conversation history.
	Chat(ctx context.Context, messages []Message) (string, error)
	// Classify maps a single piece of text to exactly one of the given categories.
	Classify(ctx context.Context, text string, categories []string) (string, error)
	ScoreDialogue(ctx context.Context, input ScoringInput) (ScoreResult, error)
}
