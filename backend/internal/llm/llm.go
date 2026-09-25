package llm

import "context"

// Message is a single chat turn sent to an LLM.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// LLMClient is the single abstraction over any language model backend.
// The service layer talks only to this interface, so GigaChat can be
// swapped for a mock (offline dev / demos) without touching game logic.
type LLMClient interface {
	// Chat returns the model's next message given the full conversation history.
	Chat(ctx context.Context, messages []Message) (string, error)
	// Classify maps a single piece of text to exactly one of the given categories.
	Classify(ctx context.Context, text string, categories []string) (string, error)
}
