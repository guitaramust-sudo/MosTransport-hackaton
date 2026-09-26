package service

import (
	"context"
	"errors"
	"math"

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
	Player       domain.Player                `json:"player"`
	Level        int                          `json:"level"`
	Competencies []domain.CompetencyAssessment `json:"competencies"`
	Achievements []string                     `json:"achievements"`
}

// levelForXP is a prototype level curve; it is not a normative ВСМ value.
func levelForXP(xp int) int {
	return xp/100 + 1
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
	all, err := s.store.ListCompetencies(ctx)
	if err != nil {
		return nil, err
	}

	assessments := assessCompetencies(all, comps)
	return &Profile{
		Player:       player,
		Level:        levelForXP(player.TotalXP),
		Competencies: assessments,
		Achievements: []string{},
	}, nil
}

// assessCompetencies turns raw competency rows into score/confidence/status.
func assessCompetencies(all []domain.Competency, earned []domain.PlayerCompetency) []domain.CompetencyAssessment {
	earnedByID := map[int]domain.PlayerCompetency{}
	for _, pc := range earned {
		earnedByID[pc.CompetencyID] = pc
	}
	out := make([]domain.CompetencyAssessment, 0, len(all))
	for _, c := range all {
		a := domain.CompetencyAssessment{
			CompetencyID: c.ID,
			Code:         c.Code,
			Name:         c.Name,
			Status:       domain.CompetencyInsufficient,
		}
		if pc, ok := earnedByID[c.ID]; ok {
			xp := pc.XP
			a.Score = &xp
			a.Confidence = pc.EvidenceCount
			a.EvidenceCount = pc.EvidenceCount
			if pc.EvidenceCount > 0 {
				a.Status = domain.CompetencyProvisional
			}
		}
		out = append(out, a)
	}
	return out
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

type ScopedLeaderboardEntry struct {
	Rank                   int       `json:"rank"`
	PlayerID               uuid.UUID `json:"player_id"`
	Username               string    `json:"username"`
	LeaderboardPointsTotal int       `json:"leaderboard_points_total"`
	Percentile             float64   `json:"percentile"`
}

type ScopedLeaderboard struct {
	GroupScope string                   `json:"group_scope"`
	GroupID    string                   `json:"group_id"`
	GroupSize  int                      `json:"group_size"`
	Entries    []ScopedLeaderboardEntry `json:"entries"`
}

func (s *ProfileService) ScopedLeaderboard(ctx context.Context, scope, groupID string, limit int) (*ScopedLeaderboard, error) {
	if scope != "company" && scope != "depot" && scope != "brigade" {
		return nil, errors.New("scope must be company, depot or brigade")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	players, err := s.store.LeaderboardScoped(ctx, scope, groupID, limit)
	if err != nil {
		return nil, err
	}
	groupSize := len(players)
	entries := make([]ScopedLeaderboardEntry, 0, groupSize)
	for i, p := range players {
		rank := i + 1
		percentile := 0.0
		if groupSize > 0 {
			percentile = math.Round((1-float64(rank-1)/float64(groupSize))*10000) / 100
		}
		entries = append(entries, ScopedLeaderboardEntry{
			Rank:                   rank,
			PlayerID:               p.ID,
			Username:               p.Username,
			LeaderboardPointsTotal: p.TotalXP,
			Percentile:             percentile,
		})
	}
	return &ScopedLeaderboard{GroupScope: scope, GroupID: groupID, GroupSize: groupSize, Entries: entries}, nil
}
