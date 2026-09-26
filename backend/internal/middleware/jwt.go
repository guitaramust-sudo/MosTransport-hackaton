package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/service"
)

type contextKey string

const (
	playerIDKey contextKey = "player_id"
	roleKey     contextKey = "role"
)

// PlayerIDFromContext returns the authenticated player id, if present.
func PlayerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(playerIDKey).(uuid.UUID)
	return id, ok
}

// RoleFromContext returns the role encoded in the token, if present.
func RoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleKey).(string)
	return role, ok
}

// authenticate parses the Bearer token and returns the player id and role.
func authenticate(auth *service.AuthService, r *http.Request) (uuid.UUID, string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return uuid.Nil, "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	playerID, role, err := auth.ParseAccess(token)
	if err != nil {
		return uuid.Nil, "", false
	}
	return playerID, role, true
}

// JWTAuth is a self-contained JWT middleware: it extracts the Bearer token,
// validates it and injects player_id and role into the request context.
func JWTAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			playerID, role, ok := authenticate(auth, r)
			if !ok {
				writeUnauthorized(w, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), playerIDKey, playerID)
			ctx = context.WithValue(ctx, roleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminAuth wraps JWTAuth and additionally requires the admin role.
func AdminAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			playerID, role, ok := authenticate(auth, r)
			if !ok {
				writeUnauthorized(w, "invalid or expired token")
				return
			}
			if role != domain.RoleAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"admin role required"}`))
				return
			}
			ctx := context.WithValue(r.Context(), playerIDKey, playerID)
			ctx = context.WithValue(ctx, roleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
