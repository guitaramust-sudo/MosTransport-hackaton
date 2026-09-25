package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/service"
)

type contextKey string

const playerIDKey contextKey = "player_id"

// PlayerIDFromContext returns the authenticated player id, if present.
func PlayerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(playerIDKey).(uuid.UUID)
	return id, ok
}

// JWTAuth is a self-contained JWT middleware: it extracts the Bearer token,
// validates it and injects player_id into the request context.
func JWTAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeUnauthorized(w, "missing or malformed authorization header")
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			playerID, err := auth.ParseAccess(token)
			if err != nil {
				writeUnauthorized(w, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), playerIDKey, playerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
