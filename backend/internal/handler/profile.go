package handler

import (
	"net/http"

	"github.com/mostransport/vsm-trainer/internal/middleware"
)

func (h *Handlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	playerID, ok := middleware.PlayerIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	profile, err := h.Profile.Get(r.Context(), playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *Handlers) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.Profile.Leaderboard(r.Context(), 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leaderboard": entries})
}

func (h *Handlers) GetScopedLeaderboard(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	groupID := r.URL.Query().Get("group_id")
	if scope == "" {
		scope = "company"
	}
	board, err := h.Profile.ScopedLeaderboard(r.Context(), scope, groupID, 50)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, board)
}
