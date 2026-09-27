package handler

import (
	"net/http"

	"github.com/mostransport/vsm-trainer/internal/middleware"
)

// RegisterPushSubscription handles POST /api/me/push-subscriptions. A
// normal authenticated player registers their own device -- this is not an
// admin action.
func (h *Handlers) RegisterPushSubscription(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	var req struct {
		Platform    string `json:"platform"`
		DeviceToken string `json:"device_token"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Platform == "" || req.DeviceToken == "" {
		writeError(w, http.StatusBadRequest, "platform and device_token required")
		return
	}
	if err := h.Push.RegisterDevice(r.Context(), playerID, req.Platform, req.DeviceToken); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}
