package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/content"
	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/llm"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var (
	ErrSituationNotFound = errors.New("situation not found")
	ErrSituationClosed   = errors.New("situation already closed")
	ErrSessionFinished   = errors.New("session already finished")
	ErrInvalidTarget     = errors.New("invalid escalation target")
)

const maxHistoryMessages = 20

type SituationService struct {
	store   repo.Store
	llm     llm.LLMClient
	catalog content.Catalog
}

func NewSituationService(store repo.Store, llmClient llm.LLMClient, catalog content.Catalog) *SituationService {
	return &SituationService{store: store, llm: llmClient, catalog: catalog}
}

// TurnResult is what a single player message produces.
type TurnResult struct {
	SituationID   uuid.UUID  `json:"situation_id"`
	Reply         string     `json:"reply"`
	Loyalty       int        `json:"loyalty"`
	Safety        int        `json:"safety"`
	Status        string     `json:"status"`
	Outcome       *string    `json:"outcome,omitempty"`
	TimerDeadline *time.Time `json:"timer_deadline"`
	Closed        bool       `json:"closed"`
	TurnCount     int        `json:"turn_count"`
}

// Get returns full situation details for a player: params, scales, timer and
// the whole dialog history.
func (s *SituationService) Get(ctx context.Context, playerID, situationID uuid.UUID) (*domain.Situation, []domain.Message, error) {
	sit, err := s.store.GetSituation(ctx, situationID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil, ErrSituationNotFound
		}
		return nil, nil, err
	}
	if err := s.ensureOwned(ctx, playerID, sit.SessionID); err != nil {
		return nil, nil, err
	}
	messages, err := s.store.ListMessagesBySituation(ctx, situationID)
	if err != nil {
		return nil, nil, err
	}
	return &sit, messages, nil
}

// SendMessage processes one text reply from the player.
func (s *SituationService) SendMessage(ctx context.Context, playerID, situationID uuid.UUID, text string) (*TurnResult, error) {
	sit, err := s.store.GetSituation(ctx, situationID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrSituationNotFound
		}
		return nil, err
	}
	if err := s.ensureOwned(ctx, playerID, sit.SessionID); err != nil {
		return nil, err
	}

	sess, err := s.store.GetSession(ctx, sit.SessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status == domain.SessionStatusFinished {
		return nil, ErrSessionFinished
	}

	if sit.Status == domain.SituationStatusClosed {
		return nil, ErrSituationClosed
	}

	if sit.TimerDeadline != nil && !time.Now().Before(*sit.TimerDeadline) {
		result, err := s.finishLoaded(ctx, sit, time.Now())
		if err != nil {
			return nil, err
		}
		turnCount, _ := s.store.CountPlayerMessages(ctx, situationID)
		return &TurnResult{SituationID: situationID, Loyalty: result.Loyalty, Safety: result.Safety,
			Status: domain.SituationStatusClosed, Outcome: &result.Outcome, TimerDeadline: sit.TimerDeadline,
			Closed: true, TurnCount: turnCount}, nil
	}
	history, err := s.buildHistory(ctx, sit)
	if err != nil {
		return nil, err
	}
	history = append(history, llm.Message{Role: "user", Content: text})
	reply, err := s.llm.Chat(ctx, history)
	if err != nil {
		slog.Error("passenger chat failed", "situation_id", situationID, "error", err)
		reply = "Понимаю… И что вы предлагаете сделать?"
	}
	turnCount, err := s.store.AppendTurn(ctx, situationID, playerID, text, reply, escalationTargets(text))
	if errors.Is(err, repo.ErrDeadlineExceeded) {
		result, closeErr := s.finishLoaded(ctx, sit, time.Now())
		if closeErr != nil {
			return nil, closeErr
		}
		turnCount, _ := s.store.CountPlayerMessages(ctx, situationID)
		return &TurnResult{SituationID: situationID, Loyalty: result.Loyalty, Safety: result.Safety,
			Status: domain.SituationStatusClosed, Outcome: &result.Outcome, TimerDeadline: sit.TimerDeadline,
			Closed: true, TurnCount: turnCount}, nil
	}
	if errors.Is(err, repo.ErrConflict) {
		currentSession, getErr := s.store.GetSession(ctx, sit.SessionID)
		if getErr == nil && currentSession.Status != domain.SessionStatusActive {
			return nil, ErrSessionFinished
		}
		return nil, ErrSituationClosed
	}
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrSituationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &TurnResult{
		SituationID:   situationID,
		Reply:         reply,
		Loyalty:       sit.Loyalty,
		Safety:        sit.Safety,
		Status:        sit.Status,
		Outcome:       sit.Outcome,
		TimerDeadline: sit.TimerDeadline,
		Closed:        false,
		TurnCount:     turnCount,
	}, nil
}

func (s *SituationService) ensureOwned(ctx context.Context, playerID, sessionID uuid.UUID) error {
	sess, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return ErrSituationNotFound
	}
	if sess.PlayerID != playerID {
		return ErrSituationNotFound
	}
	return nil
}

// buildHistory reconstructs the full conversation from the database (so the
// backend survives restarts) and prepends the role-play system prompt.
func (s *SituationService) buildHistory(ctx context.Context, sit domain.Situation) ([]llm.Message, error) {
	messages, err := s.store.ListMessagesBySituation(ctx, sit.ID)
	if err != nil {
		return nil, err
	}

	promptHint, _ := sit.PassengerParams["prompt_hint"].(string)
	language, _ := sit.PassengerParams["language"].(string)
	traits, _ := sit.PassengerParams["traits_text"].(string)
	opening, _ := sit.PassengerParams["opening"].(string)

	out := []llm.Message{
		{Role: "system", Content: buildSystemPrompt(promptHint, language, traits, opening)},
	}

	// Cap the tail to save tokens (system prompt always stays).
	if len(messages) >= maxHistoryMessages {
		messages = messages[len(messages)-(maxHistoryMessages-1):]
	}
	for _, m := range messages {
		role := m.Role
		switch m.Role {
		case domain.MessageRolePlayer:
			role = "user"
		case domain.MessageRolePassenger:
			role = "assistant"
		case domain.MessageRoleSystem:
			continue
		}
		out = append(out, llm.Message{Role: role, Content: m.Content})
	}
	return out, nil
}
