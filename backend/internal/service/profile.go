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
	store     repo.Store
	namespace string
}

func NewProfileService(store repo.Store, namespace ...string) *ProfileService {
	ns := "demo"
	if len(namespace) > 0 {
		ns = namespace[0]
	}
	return &ProfileService{store: store, namespace: ns}
}

type Profile struct {
	Player                 domain.Player                 `json:"player"`
	Level                  int                           `json:"level"`
	Competencies           []domain.CompetencyAssessment `json:"competencies"`
	Achievements           []string                      `json:"achievements"`
	LeaderboardPointsTotal int                           `json:"leaderboard_points_total"`
}

// levelForXP is a prototype level curve; it is not a normative ВСМ value.
func levelForXP(xp int) int {
	if xp < 0 {
		return 1
	}
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
	achievements, err := s.store.ListAchievementCodes(ctx, playerID)
	if err != nil {
		return nil, err
	}
	points, err := s.store.GetPlayerPointsTotal(ctx, playerID, s.namespace)
	if err != nil {
		return nil, err
	}
	return &Profile{
		Player:                 player,
		Level:                  levelForXP(player.TotalXP),
		Competencies:           assessments,
		Achievements:           achievements,
		LeaderboardPointsTotal: points,
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
	Rank                   int       `json:"rank"`
	PlayerID               uuid.UUID `json:"player_id"`
	Username               string    `json:"username"`
	LeaderboardPointsTotal int       `json:"leaderboard_points_total"`
}

func (s *ProfileService) Leaderboard(ctx context.Context, limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	players, err := s.store.PointsLeaderboard(ctx, s.namespace, "company", "")
	if err != nil {
		return nil, err
	}
	out := make([]LeaderboardEntry, 0, min(limit, len(players)))
	for i, p := range players {
		if i >= limit {
			break
		}
		out = append(out, LeaderboardEntry{
			Rank:                   pointsRank(players, i),
			PlayerID:               p.PlayerID,
			Username:               p.Username,
			LeaderboardPointsTotal: p.Points,
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
	players, err := s.store.PointsLeaderboard(ctx, s.namespace, scope, groupID)
	if err != nil {
		return nil, err
	}
	groupSize := len(players)
	entries := make([]ScopedLeaderboardEntry, 0, min(limit, groupSize))
	for i, p := range players {
		if i >= limit {
			break
		}
		rank := pointsRank(players, i)
		percentile := 0.0
		if groupSize > 0 {
			equal := 0
			for _, row := range players {
				if row.Points == p.Points {
					equal++
				}
			}
			lower := groupSize - (rank - 1) - equal
			percentile = math.Round((float64(lower)+0.5*float64(equal))/float64(groupSize)*10000) / 100
		}
		entries = append(entries, ScopedLeaderboardEntry{
			Rank:                   rank,
			PlayerID:               p.PlayerID,
			Username:               p.Username,
			LeaderboardPointsTotal: p.Points,
			Percentile:             percentile,
		})
	}
	return &ScopedLeaderboard{GroupScope: scope, GroupID: groupID, GroupSize: groupSize, Entries: entries}, nil
}

func pointsRank(players []repo.PointsStanding, index int) int {
	for index > 0 && players[index-1].Points == players[index].Points {
		index--
	}
	return index + 1
}
