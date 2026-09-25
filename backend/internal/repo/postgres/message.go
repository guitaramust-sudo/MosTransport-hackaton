package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
)

func (s *Store) CreateMessage(ctx context.Context, situationID uuid.UUID, role, content string, category *string) (domain.Message, error) {
	var m domain.Message
	err := s.pool.QueryRow(ctx,
		`INSERT INTO messages (situation_id, role, content, category)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, situation_id, role, content, category, created_at`,
		situationID, role, content, category,
	).Scan(&m.ID, &m.SituationID, &m.Role, &m.Content, &m.Category, &m.CreatedAt)
	return m, err
}

func (s *Store) CreateEscalationMessage(ctx context.Context, situationID uuid.UUID, target string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO messages (situation_id, role, content, kind)
		 VALUES ($1, 'system', $2, 'escalation')`, situationID, "Вызван адресат: "+target)
	return err
}

func (s *Store) ListMessagesBySituation(ctx context.Context, situationID uuid.UUID) ([]domain.Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, situation_id, role, content, category, created_at
		 FROM messages WHERE situation_id = $1 ORDER BY id`, situationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.SituationID, &m.Role, &m.Content, &m.Category, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CountPlayerMessages(ctx context.Context, situationID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE situation_id = $1 AND role = 'player'`, situationID).Scan(&n)
	return n, err
}

func (s *Store) UpdateMessageCategory(ctx context.Context, id int, category string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE messages SET category = $2 WHERE id = $1`, id, category)
	return err
}
