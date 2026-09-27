package handler

import (
	"net/http/httptest"
	"testing"
)

func TestWagonWebSocketOriginPolicy(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "http://localhost:8081")
	for _, tc := range []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:8088", true},
		{"http://localhost:8081", true},
		{"https://untrusted.example", false},
	} {
		req := httptest.NewRequest("GET", "http://localhost:8088/api/wagon/id/ws", nil)
		req.Header.Set("Origin", tc.origin)
		if got := wagonOriginAllowed(req); got != tc.allowed {
			t.Fatalf("origin %q allowed=%v want %v", tc.origin, got, tc.allowed)
		}
	}
}
