package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/llm"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

func (s *SituationService) Finish(ctx context.Context, playerID, situationID uuid.UUID) (*ScoreSummary, error) {
	sit, err := s.store.GetSituation(ctx, situationID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrSituationNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwned(ctx, playerID, sit.SessionID); err != nil {
		return nil, err
	}
	sess, err := s.store.GetSession(ctx, sit.SessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != domain.SessionStatusActive {
		return nil, ErrSessionFinished
	}
	if sit.Status != domain.SituationStatusActive {
		return nil, ErrSituationClosed
	}
	return s.finishLoaded(ctx, sit, time.Now())
}

func (s *SituationService) Escalate(ctx context.Context, playerID, situationID uuid.UUID, target string) ([]string, error) {
	if !validTarget(target) {
		return nil, ErrInvalidTarget
	}
	sit, err := s.store.GetSituation(ctx, situationID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrSituationNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwned(ctx, playerID, sit.SessionID); err != nil {
		return nil, err
	}
	sess, err := s.store.GetSession(ctx, sit.SessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != domain.SessionStatusActive {
		return nil, ErrSessionFinished
	}
	if sit.Status != domain.SituationStatusActive {
		return nil, ErrSituationClosed
	}
	if sit.TimerDeadline != nil && !time.Now().Before(*sit.TimerDeadline) {
		if _, err := s.finishLoaded(ctx, sit, time.Now()); err != nil {
			return nil, err
		}
		return nil, ErrSituationClosed
	}
	actual, err := s.store.AddEscalation(ctx, situationID, target)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrSituationClosed
	}
	if err != nil {
		return nil, err
	}
	if !contains(sit.Escalations, target) {
		err = s.store.CreateEscalationMessage(ctx, situationID, target)
	}
	return actual, err
}

func (s *SituationService) CloseExpired(ctx context.Context) error {
	ids, err := s.store.ListExpiredSituationIDs(ctx, time.Now())
	if err != nil {
		return err
	}
	for _, id := range ids {
		sit, err := s.store.GetSituation(ctx, id)
		if err != nil || sit.Status != domain.SituationStatusActive {
			continue
		}
		if _, err := s.finishLoaded(ctx, sit, time.Now()); err != nil {
			slog.Error("timer close failed", "situation_id", id, "error", err)
		}
	}
	return nil
}

func (s *SituationService) finishLoaded(ctx context.Context, sit domain.Situation, now time.Time) (*ScoreSummary, error) {
	latest, err := s.store.GetSituation(ctx, sit.ID)
	if err != nil {
		return nil, err
	}
	if latest.Status != domain.SituationStatusActive {
		var previous ScoreSummary
		if json.Unmarshal(latest.ScoreResult, &previous) != nil {
			return nil, ErrSituationClosed
		}
		return &previous, nil
	}
	sit = latest
	scenario, passenger, err := s.definitions(sit)
	if err != nil {
		return nil, err
	}
	messages, err := s.store.ListMessagesBySituation(ctx, sit.ID)
	if err != nil {
		return nil, err
	}
	history := make([]llm.Message, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case domain.MessageRolePlayer:
			history = append(history, llm.Message{Role: "user", Content: m.Content})
		case domain.MessageRolePassenger:
			history = append(history, llm.Message{Role: "assistant", Content: m.Content})
		}
	}
	input := llm.ScoringInput{Scenario: scenario, Passenger: passenger, History: history,
		Escalations: sit.Escalations, Elapsed: now.Sub(sit.CreatedAt)}
	observed, err := s.llm.ScoreDialogue(ctx, input)
	if err != nil {
		slog.Error("dialogue scoring failed; using missed-point fallback", "situation_id", sit.ID, "error", err)
		observed = llm.ScoreResult{Tone: "neutral", Reasoning: "scoring fallback"}
	}
	timedOut := sit.TimerDeadline != nil && !now.Before(*sit.TimerDeadline)
	result := EvaluateScore(scenario, observed, sit.Escalations, input.Elapsed, timedOut)
	remarksJSON, err := json.Marshal(result.Remarks)
	if err != nil {
		return nil, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	sit.Outcome, sit.Loyalty, sit.Safety, sit.XP = &result.Outcome, result.Loyalty, result.Safety, result.XP
	sit.Remarks, sit.ScoreResult, sit.ClosedAt = remarksJSON, resultJSON, &now
	closed, err := s.store.CloseSituation(ctx, sit)
	if err != nil {
		return nil, err
	}
	if !closed {
		current, err := s.store.GetSituation(ctx, sit.ID)
		if err != nil {
			return nil, err
		}
		var previous ScoreSummary
		if len(current.ScoreResult) == 0 || json.Unmarshal(current.ScoreResult, &previous) != nil {
			return nil, ErrSituationClosed
		}
		return &previous, nil
	}
	return &result, nil
}

func (s *SituationService) definitions(sit domain.Situation) (content.Scenario, content.Passenger, error) {
	if sit.SituationDefID == nil || sit.PassengerID == nil {
		return content.Scenario{}, content.Passenger{}, errors.New("situation has no content IDs")
	}
	var scenario content.Scenario
	var passenger content.Passenger
	for _, item := range s.catalog.Scenarios {
		if item.ID == *sit.SituationDefID {
			scenario = item
			break
		}
	}
	for _, item := range s.catalog.Passengers {
		if item.ID == *sit.PassengerID {
			passenger = item
			break
		}
	}
	if scenario.ID == "" || passenger.ID == "" {
		return content.Scenario{}, content.Passenger{}, errors.New("situation content ID missing from catalog")
	}
	return scenario, passenger, nil
}

func validTarget(target string) bool {
	return contains([]string{content.TargetTrainChief, content.TargetPTB, content.TargetPolice,
		content.TargetMedic, content.TargetAmbulance}, target)
}

func escalationTargets(text string) []string {
	text = strings.ToLower(text)
	if !strings.Contains(text, "выз") && !strings.Contains(text, "приглаш") && !strings.Contains(text, "сообщ") {
		return nil
	}
	var targets []string
	patterns := []struct {
		target string
		words  []string
	}{
		{content.TargetTrainChief, []string{"начальник", "начальнику"}},
		{content.TargetPTB, []string{"служб", "безопасност", "птб"}},
		{content.TargetPolice, []string{"полиц", "наряд"}},
		{content.TargetMedic, []string{"врач", "медик"}},
		{content.TargetAmbulance, []string{"скорую", "скорой", "бригад"}},
	}
	for _, pattern := range patterns {
		for _, word := range pattern.words {
			if strings.Contains(text, word) {
				targets = append(targets, pattern.target)
				break
			}
		}
	}
	return targets
}
