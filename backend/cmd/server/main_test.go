package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mostransport/vsm-trainer/internal/handler"
)

func TestCORSAllowsConfiguredOriginOnly(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "http://localhost:8081")
	handler := localWebCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, tc := range []struct {
		origin string
		status int
		allow  string
	}{
		{"http://localhost:8081", http.StatusNoContent, "http://localhost:8081"},
		{"https://unknown.example", http.StatusForbidden, ""},
	} {
		req := httptest.NewRequest(http.MethodOptions, "/api/session/start", nil)
		req.Header.Set("Origin", tc.origin)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status || res.Header().Get("Access-Control-Allow-Origin") != tc.allow {
			t.Fatalf("origin %s: status %d, allow %q", tc.origin, res.Code, res.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}

func TestWagonWebSocketRouteBypassesHeaderJWTMiddleware(t *testing.T) {
	router := routes(&handler.Handlers{}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/wagon/not-a-uuid/ws?token=invalid", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("websocket route status = %d, want 400 from session ID validation", res.Code)
	}
}
