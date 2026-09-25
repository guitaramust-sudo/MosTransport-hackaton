package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGigaChatRetriesServerFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			_, _ = fmt.Fprint(w, `{"access_token":"test-token"}`)
		case "/chat/completions":
			attempts++
			if attempts < 3 {
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
			_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewGigaChatClient(server.URL+"/auth", server.URL, "test", "id", "secret", false)
	got, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil || got != "ok" || attempts != 3 {
		t.Fatalf("answer %q, attempts %d, err %v", got, attempts, err)
	}
}

func TestGigaChatDoesNotRetryClientFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth") {
			_, _ = fmt.Fprint(w, `{"access_token":"test-token"}`)
			return
		}
		attempts++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()
	client := NewGigaChatClient(server.URL+"/auth", server.URL, "test", "id", "secret", false)
	_, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err == nil || attempts != 1 {
		t.Fatalf("attempts %d, err %v", attempts, err)
	}
}
