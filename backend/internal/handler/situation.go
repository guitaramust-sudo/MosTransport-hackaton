package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/service"
)

func (h *Handlers) GetSituation(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid situation id")
		return
	}

	sit, messages, err := h.Situation.Get(r.Context(), playerID, id)
	if err != nil {
		if errors.Is(err, service.ErrSituationNotFound) {
			writeError(w, http.StatusNotFound, "situation not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"situation": toSituationDTO(*sit, true),
		"messages":  messages,
	})
}

type messageRequest struct {
	Text string `json:"text"`
}

func (h *Handlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid situation id")
		return
	}

	var req messageRequest
	if err := decodeJSON(r, &req); err != nil || req.Text == "" {
		writeError(w, http.StatusBadRequest, "text required")
		return
	}

	result, err := h.Situation.SendMessage(r.Context(), playerID, id, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSituationNotFound):
			writeError(w, http.StatusNotFound, "situation not found")
		case errors.Is(err, service.ErrSituationClosed):
			writeError(w, http.StatusConflict, "situation already closed")
		case errors.Is(err, service.ErrSessionFinished):
			writeError(w, http.StatusConflict, "session already finished")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}
