package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/llm"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var (
	ErrSituationNotFound = errors.New("situation not found")
	ErrSituationClosed   = errors.New("situation already closed")
	ErrSessionFinished   = errors.New("session already finished")
)

const maxHistoryMessages = 20

type SituationService struct {
	store    repo.Store
	llm      llm.LLMClient
	maxTurns int
}

func NewSituationService(store repo.Store, llmClient llm.LLMClient, maxTurns int) *SituationService {
	return &SituationService{store: store, llm: llmClient, maxTurns: maxTurns}
}

// TurnResult is what a single player message produces.
type TurnResult struct {
	SituationID   uuid.UUID  `json:"situation_id"`
	Category      string     `json:"category"`
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

	// Timer forced outcome.
	if sit.TimerDeadline != nil && time.Now().After(*sit.TimerDeadline) {
		return s.closeTimeout(ctx, sit)
	}

	// 1. Persist player message.
	msg, err := s.store.CreateMessage(ctx, situationID, domain.MessageRolePlayer, text, nil)
	if err != nil {
		return nil, err
	}

	// 2. Classify with minimal context.
	rawCat, err := s.llm.Classify(ctx, text, Categories)
	if err != nil {
		log.Printf("classify failed (using default): %v", err)
		rawCat = "эмпатия"
	}
	category := normalizeCategory(rawCat)
	if err := s.store.UpdateMessageCategory(ctx, msg.ID, category); err != nil {
		return nil, err
	}

	// 3. Apply scale effects.
	eff := effectFor(category)
	sit.Loyalty = clamp(sit.Loyalty+eff.LoyaltyDelta, 0, 100)
	sit.Safety = clamp(sit.Safety+eff.SafetyDelta, 0, 100)

	// 4. Build full history and get the passenger's reply.
	history, err := s.buildHistory(ctx, sit)
	if err != nil {
		return nil, err
	}
	reply, err := s.llm.Chat(ctx, history)
	if err != nil {
		log.Printf("chat failed (using fallback): %v", err)
		reply = "Понимаю… И что вы предлагаете сделать?"
	}
	if _, err := s.store.CreateMessage(ctx, situationID, domain.MessageRolePassenger, reply, nil); err != nil {
		return nil, err
	}

	// 5. Determine whether this turn closes the situation.
	turnCount, err := s.store.CountPlayerMessages(ctx, situationID)
	if err != nil {
		return nil, err
	}
	closed := turnCount >= s.maxTurns
	if closed {
		outcome := outcomeLabel(sit.Loyalty, sit.Safety)
		sit.Status = domain.SituationStatusClosed
		sit.Outcome = &outcome
	}

	if err := s.store.UpdateSituation(ctx, sit); err != nil {
		return nil, err
	}

	return &TurnResult{
		SituationID:   situationID,
		Category:      category,
		Reply:         reply,
		Loyalty:       sit.Loyalty,
		Safety:        sit.Safety,
		Status:        sit.Status,
		Outcome:       sit.Outcome,
		TimerDeadline: sit.TimerDeadline,
		Closed:        closed,
		TurnCount:     turnCount,
	}, nil
}

func (s *SituationService) closeTimeout(ctx context.Context, sit domain.Situation) (*TurnResult, error) {
	outcome := "timeout"
	sit.Status = domain.SituationStatusClosed
	sit.Outcome = &outcome
	sit.Loyalty = clamp(sit.Loyalty-10, 0, 100)
	sit.Safety = clamp(sit.Safety-5, 0, 100)
	if err := s.store.UpdateSituation(ctx, sit); err != nil {
		return nil, err
	}
	reply := "Ситуация завершилась по таймеру — пассажир не дождался решения."
	if _, err := s.store.CreateMessage(ctx, sit.ID, domain.MessageRoleSystem, reply, nil); err != nil {
		return nil, err
	}
	turnCount, _ := s.store.CountPlayerMessages(ctx, sit.ID)
	return &TurnResult{
		SituationID:   sit.ID,
		Reply:         reply,
		Loyalty:       sit.Loyalty,
		Safety:        sit.Safety,
		Status:        sit.Status,
		Outcome:       sit.Outcome,
		TimerDeadline: sit.TimerDeadline,
		Closed:        true,
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

	name, _ := sit.PassengerParams["name"].(string)
	persona, _ := sit.PassengerParams["persona"].(string)
	scenario, _ := sit.PassengerParams["scenario"].(string)

	out := []llm.Message{
		{Role: "system", Content: buildSystemPrompt(name, persona, scenario)},
	}

	// Cap the tail to save tokens (system prompt always stays).
	if len(messages) > maxHistoryMessages {
		messages = messages[len(messages)-maxHistoryMessages:]
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
