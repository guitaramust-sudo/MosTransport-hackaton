package handler

import (
	"errors"
	"net/http"

	"github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/service"
)

type startWagonRequest struct {
	ClassID string `json:"class_id"`
}

func (h *Handlers) StartWagonSession(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	var req startWagonRequest
	if err := decodeJSON(r, &req); err != nil || req.ClassID == "" {
		writeError(w, http.StatusBadRequest, "class_id required")
		return
	}
	sess, err := h.Wagon.StartSession(r.Context(), playerID, req.ClassID)
	if err != nil {
		if errors.Is(err, service.ErrWagonClassNotPlayable) {
			writeJSON(w, http.StatusConflict, map[string]string{"status": "coming_soon"})
			return
		}
		if errors.Is(err, service.ErrNoEligibleScenarios) {
			writeError(w, http.StatusConflict, "no approved wagon scenarios available")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to start wagon session")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"session_id": sess.ID, "ws_path": "/api/wagon/" + sess.ID.String() + "/ws"})
}

func (h *Handlers) ListWagonClasses(w http.ResponseWriter, r *http.Request) {
	out := make(map[string]string, len(h.WagonClasses))
	for id, cfg := range h.WagonClasses {
		if cfg.Status == "coming_soon" {
			out[id] = "coming_soon"
		} else {
			out[id] = "available"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"classes": out})
}
