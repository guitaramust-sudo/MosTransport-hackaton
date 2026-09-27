package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mostransport/vsm-trainer/internal/middleware"
	"github.com/mostransport/vsm-trainer/internal/repo"
	"github.com/mostransport/vsm-trainer/internal/service"
)

// GetLearningMap handles GET /api/learning/map.
func (h *Handlers) GetLearningMap(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	m, err := h.Learning.GetMap(r.Context(), playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// GetLesson handles GET /api/learning/lessons/{id}.
func (h *Handlers) GetLesson(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	lessonID := chi.URLParam(r, "id")
	detail, err := h.Learning.GetLesson(r.Context(), playerID, lessonID)
	if err != nil {
		writeLearningError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

type submitAnswerRequest struct {
	QuestionID string `json:"question_id"`
	OptionID   string `json:"option_id"`
}

// SubmitLessonAnswer handles POST /api/learning/lessons/{id}/answers.
func (h *Handlers) SubmitLessonAnswer(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	lessonID := chi.URLParam(r, "id")
	var req submitAnswerRequest
	if err := decodeJSON(r, &req); err != nil || req.QuestionID == "" || req.OptionID == "" {
		writeError(w, http.StatusBadRequest, "question_id and option_id required")
		return
	}
	result, err := h.Learning.SubmitAnswer(r.Context(), playerID, lessonID, req.QuestionID, req.OptionID)
	if err != nil {
		writeLearningError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// StartLessonPractice handles POST /api/learning/lessons/{id}/practice.
func (h *Handlers) StartLessonPractice(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	lessonID := chi.URLParam(r, "id")
	sess, err := h.Learning.StartPractice(r.Context(), playerID, lessonID)
	if err != nil {
		writeLearningError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"session_id": sess.ID, "ws_path": "/api/wagon/" + sess.ID.String() + "/ws"})
}

// FinalizeLessonPractice handles POST /api/learning/lessons/{id}/finalize.
func (h *Handlers) FinalizeLessonPractice(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	lessonID := chi.URLParam(r, "id")
	result, err := h.Learning.FinalizePractice(r.Context(), playerID, lessonID)
	if err != nil {
		writeLearningError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetMyLearning handles GET /api/me/learning.
func (h *Handlers) GetMyLearning(w http.ResponseWriter, r *http.Request) {
	playerID, _ := middleware.PlayerIDFromContext(r.Context())
	summary, err := h.Learning.GetMyLearning(r.Context(), playerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// writeLearningError maps LearningService's sentinel errors to HTTP status
// codes, following wagon.go's errors.Is-based mapping style.
func writeLearningError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLessonNotFound):
		writeError(w, http.StatusNotFound, "lesson not found")
	case errors.Is(err, repo.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, service.ErrLessonLocked):
		writeError(w, http.StatusConflict, "lesson is locked")
	case errors.Is(err, service.ErrTheoryNotPassed):
		writeError(w, http.StatusConflict, "theory has not been passed yet")
	case errors.Is(err, service.ErrPracticeNotStarted):
		writeError(w, http.StatusConflict, "practice has not been started yet")
	case errors.Is(err, service.ErrQuestionNotInLesson):
		writeError(w, http.StatusBadRequest, "question is not part of this lesson")
	case errors.Is(err, service.ErrWagonClassNotPlayable):
		writeJSON(w, http.StatusConflict, map[string]string{"status": "coming_soon"})
	case errors.Is(err, service.ErrNoEligibleScenarios):
		writeError(w, http.StatusConflict, "no eligible passengers available")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
