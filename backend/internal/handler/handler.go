package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mostransport/vsm-trainer/internal/service"
)

// Handlers bundles all HTTP handlers and their dependencies.
type Handlers struct {
	Auth      *service.AuthService
	Profile   *service.ProfileService
	Session   *service.SessionService
	Situation *service.SituationService
	Admin     *service.AdminService
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
