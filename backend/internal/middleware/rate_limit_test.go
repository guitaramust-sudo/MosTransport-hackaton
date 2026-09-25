package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthRateLimitPerIP(t *testing.T) {
	handler := AuthRateLimit(2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for i, tc := range []struct {
		ip   string
		want int
	}{
		{"192.0.2.1:1234", 204}, {"192.0.2.1:2345", 204}, {"192.0.2.1:3456", 429}, {"192.0.2.2:1234", 204},
	} {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = tc.ip
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.want {
			t.Fatalf("request %d: got %d, want %d", i, res.Code, tc.want)
		}
	}
}
