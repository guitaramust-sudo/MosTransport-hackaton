package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
