package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/repo"
	"github.com/mostransport/vsm-trainer/internal/service"
	"github.com/mostransport/vsm-trainer/internal/simulation"
)

func (h *Handlers) StartSimulation(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	view, err := h.Simulation.Start(r.Context(), playerID)
	if err != nil {
		if errors.Is(err, service.ErrNoEligibleScenarios) {
			writeError(w, http.StatusConflict, "simulation is available only in demo namespace")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to start simulation")
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (h *Handlers) GetSimulation(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid simulation id")
		return
	}
	view, err := h.Simulation.Get(r.Context(), playerID, id)
	if errors.Is(err, repo.ErrNotFound) {
		writeError(w, http.StatusNotFound, "simulation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

type simulationActionRequest struct {
	CommandID            uuid.UUID `json:"command_id"`
	ExpectedStateVersion int       `json:"expected_state_version"`
	ChoiceID             string    `json:"choice_id"`
}

func (h *Handlers) SimulationAction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid simulation id")
		return
	}
	var req simulationActionRequest
	if err := decodeJSON(r, &req); err != nil || req.CommandID == uuid.Nil || req.ExpectedStateVersion < 0 || req.ChoiceID == "" {
		writeError(w, http.StatusBadRequest, "command_id, expected_state_version and choice_id required")
		return
	}
	view, err := h.Simulation.Action(r.Context(), playerID, id, req.CommandID, req.ExpectedStateVersion, req.ChoiceID)
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "simulation not found")
	case errors.Is(err, repo.ErrConflict):
		writeError(w, http.StatusConflict, "state version changed")
	case errors.Is(err, simulation.ErrInvalidChoice):
		writeError(w, http.StatusBadRequest, "choice unavailable in current event")
	case errors.Is(err, simulation.ErrFinished):
		writeError(w, http.StatusConflict, "simulation already finished")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal error")
	default:
		writeJSON(w, http.StatusOK, view)
	}
}
