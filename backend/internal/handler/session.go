package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/service"
)

type situationDTO struct {
	ID            uuid.UUID  `json:"id"`
	Status        string     `json:"status"`
	Code          string     `json:"code"`
	Name          string     `json:"name"`
	Scenario      string     `json:"scenario"`
	Opening       string     `json:"opening,omitempty"`
	Loyalty       int        `json:"loyalty"`
	Safety        int        `json:"safety"`
	TimerDeadline *time.Time `json:"timer_deadline"`
	Outcome       *string    `json:"outcome,omitempty"`
}

func toSituationDTO(s domain.Situation, includeOpening bool) situationDTO {
	d := situationDTO{
		ID:            s.ID,
		Status:        s.Status,
		Loyalty:       s.Loyalty,
		Safety:        s.Safety,
		TimerDeadline: s.TimerDeadline,
		Outcome:       s.Outcome,
	}
	if p := s.PassengerParams; p != nil {
		d.Code, _ = p["code"].(string)
		d.Name, _ = p["name"].(string)
		d.Scenario, _ = p["scenario"].(string)
		if includeOpening {
			d.Opening, _ = p["opening"].(string)
		}
	}
	return d
}

func (h *Handlers) StartSession(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())

	sess, situations, err := h.Session.Start(r.Context(), playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start session")
		return
	}

	dtos := make([]situationDTO, 0, len(situations))
	for _, s := range situations {
		dtos = append(dtos, toSituationDTO(s, true))
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"session":    sess,
		"situations": dtos,
	})
}

func (h *Handlers) GetSession(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	sess, situations, err := h.Session.Get(r.Context(), playerID, id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	dtos := make([]situationDTO, 0, len(situations))
	for _, s := range situations {
		dtos = append(dtos, toSituationDTO(s, false))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session":    sess,
		"situations": dtos,
	})
}

func (h *Handlers) FinishSession(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	breakdown, err := h.Session.Finish(r.Context(), playerID, id)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, breakdown)
}
