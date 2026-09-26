package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/repo"
	"github.com/mostransport/vsm-trainer/internal/service"
)

type situationDTO struct {
	ID             uuid.UUID       `json:"id"`
	Status         string          `json:"status"`
	SituationDefID *string         `json:"situation_def_id,omitempty"`
	PassengerID    *string         `json:"passenger_id,omitempty"`
	Code           string          `json:"code"`
	Name           string          `json:"name"`
	Language       string          `json:"language,omitempty"`
	Scenario       string          `json:"scenario"`
	Opening        string          `json:"opening,omitempty"`
	Loyalty        int             `json:"loyalty"`
	Safety         int             `json:"safety"`
	TimerDeadline  *time.Time      `json:"timer_deadline"`
	Outcome        *string         `json:"outcome,omitempty"`
	Escalations    []string        `json:"escalations"`
	XP             int             `json:"xp"`
	Remarks        json.RawMessage `json:"remarks,omitempty"`
	ScoreResult    json.RawMessage `json:"score_result,omitempty"`
	Tone           string          `json:"tone,omitempty"`
	Conveyed       []string        `json:"conveyed,omitempty"`
	Missed         []string        `json:"missed,omitempty"`
}

func toSituationDTO(s domain.Situation, includeOpening bool) situationDTO {
	d := situationDTO{
		ID:             s.ID,
		Status:         s.Status,
		SituationDefID: s.SituationDefID,
		PassengerID:    s.PassengerID,
		Loyalty:        s.Loyalty,
		Safety:         s.Safety,
		TimerDeadline:  s.TimerDeadline,
		Outcome:        s.Outcome,
		Escalations:    s.Escalations,
		XP:             s.XP,
		Remarks:        s.Remarks,
		ScoreResult:    s.ScoreResult,
	}
	if p := s.PassengerParams; p != nil {
		d.Code, _ = p["code"].(string)
		d.Name, _ = p["name"].(string)
		d.Language, _ = p["language"].(string)
		d.Scenario, _ = p["scenario"].(string)
		if includeOpening {
			d.Opening, _ = p["opening"].(string)
		}
	}
	if len(s.ScoreResult) > 0 {
		var summary service.ScoreSummary
		if json.Unmarshal(s.ScoreResult, &summary) == nil {
			d.Tone, d.Conveyed, d.Missed = summary.Tone, summary.Conveyed, summary.Missed
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
		if errors.Is(err, repo.ErrConflict) {
			writeError(w, http.StatusConflict, "session changed; retry")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, breakdown)
}
