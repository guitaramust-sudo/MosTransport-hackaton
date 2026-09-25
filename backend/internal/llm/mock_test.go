package llm

import (
	"context"
	"strings"
	"testing"
)

func TestMockChatUsesPassengerLanguage(t *testing.T) {
	for _, tc := range []struct {
		language string
		want     string
	}{
		{"en", "Could you tell me"},
		{"zh", "接下来"},
		{"de", "Können Sie"},
		{"ru", "билет"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			reply, err := NewMockLLM().Chat(context.Background(), []Message{
				{Role: "system", Content: "Язык: " + tc.language + "."},
				{Role: "user", Content: "Где билет?"},
			})
			if err != nil || !strings.Contains(reply, tc.want) {
				t.Fatalf("reply = %q, err = %v; want %q", reply, err, tc.want)
			}
		})
	}
}
