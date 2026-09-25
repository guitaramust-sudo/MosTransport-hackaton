package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var ErrPlayerNotFound = errors.New("player not found")

type ProfileService struct {
	store repo.Store
}

func NewProfileService(store repo.Store) *ProfileService {
	return &ProfileService{store: store}
}

type Profile struct {
	Player       domain.Player             `json:"player"`
	Competencies []domain.PlayerCompetency `json:"competencies"`
}

func (s *ProfileService) Get(ctx context.Context, playerID uuid.UUID) (*Profile, error) {
	player, err := s.store.GetPlayerByID(ctx, playerID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrPlayerNotFound
		}
		return nil, err
	}
	comps, err := s.store.GetPlayerCompetencies(ctx, playerID)
	if err != nil {
		return nil, err
	}
	return &Profile{Player: player, Competencies: comps}, nil
}

type LeaderboardEntry struct {
	Rank     int       `json:"rank"`
	PlayerID uuid.UUID `json:"player_id"`
	Username string    `json:"username"`
	TotalXP  int       `json:"total_xp"`
}

func (s *ProfileService) Leaderboard(ctx context.Context, limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	players, err := s.store.Leaderboard(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]LeaderboardEntry, 0, len(players))
	for i, p := range players {
		out = append(out, LeaderboardEntry{
			Rank:     i + 1,
			PlayerID: p.ID,
			Username: p.Username,
			TotalXP:  p.TotalXP,
		})
	}
	return out, nil
}
