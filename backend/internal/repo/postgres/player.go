package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

const playerColumns = `id, email, username, password_hash, role, display_name, source_system, external_user_id, assigned_class_ids, depot_id, brigade_id, total_xp, created_at`

func scanPlayer(p *domain.Player) []any {
	return []any{
		&p.ID, &p.Email, &p.Username, &p.PasswordHash, &p.Role,
		&p.DisplayName, &p.SourceSystem, &p.ExternalUserID, &p.AssignedClassIDs,
		&p.DepotID, &p.BrigadeID, &p.TotalXP, &p.CreatedAt,
	}
}

func (s *Store) CreatePlayer(ctx context.Context, email, username, passwordHash string) (domain.Player, error) {
	var p domain.Player
	err := s.pool.QueryRow(ctx,
		`INSERT INTO players (email, username, password_hash) VALUES ($1, $2, $3) RETURNING `+playerColumns,
		email, username, passwordHash,
	).Scan(scanPlayer(&p)...)
	return p, err
}

func (s *Store) GetPlayerByEmail(ctx context.Context, email string) (domain.Player, error) {
	return s.scanPlayer(ctx, `SELECT `+playerColumns+` FROM players WHERE email = $1`, email)
}

func (s *Store) GetPlayerByID(ctx context.Context, id uuid.UUID) (domain.Player, error) {
	return s.scanPlayer(ctx, `SELECT `+playerColumns+` FROM players WHERE id = $1`, id)
}

func (s *Store) GetPlayerByExternal(ctx context.Context, sourceSystem, externalUserID string) (domain.Player, error) {
	return s.scanPlayer(ctx,
		`SELECT `+playerColumns+` FROM players WHERE source_system = $1 AND external_user_id = $2`,
		sourceSystem, externalUserID)
}

func (s *Store) scanPlayer(ctx context.Context, q string, args ...any) (domain.Player, error) {
	var p domain.Player
	err := s.pool.QueryRow(ctx, q, args...).Scan(scanPlayer(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, repo.ErrNotFound
	}
	return p, err
}

func (s *Store) AddTotalXP(ctx context.Context, playerID uuid.UUID, xp int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE players SET total_xp = total_xp + $2 WHERE id = $1`, playerID, xp)
	return err
}

func (s *Store) AddCompetencyXP(ctx context.Context, playerID uuid.UUID, competencyCode string, xp, evidence int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO player_competencies (player_id, competency_id, xp, evidence_count)
		 SELECT $1, id, $3, $4 FROM competencies WHERE code = $2
		 ON CONFLICT (player_id, competency_id)
		 DO UPDATE SET xp = player_competencies.xp + EXCLUDED.xp,
		               evidence_count = player_competencies.evidence_count + EXCLUDED.evidence_count`,
		playerID, competencyCode, xp, evidence)
	return err
}

func (s *Store) ListCompetencies(ctx context.Context) ([]domain.Competency, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, code, name FROM competencies ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Competency
	for rows.Next() {
		var c domain.Competency
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetPlayerCompetencies(ctx context.Context, playerID uuid.UUID) ([]domain.PlayerCompetency, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT pc.player_id, pc.competency_id, pc.xp, pc.evidence_count
		 FROM player_competencies pc
		 WHERE pc.player_id = $1
		 ORDER BY pc.competency_id`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PlayerCompetency
	for rows.Next() {
		var pc domain.PlayerCompetency
		if err := rows.Scan(&pc.PlayerID, &pc.CompetencyID, &pc.XP, &pc.EvidenceCount); err != nil {
			return nil, err
		}
		out = append(out, pc)
	}
	return out, rows.Err()
}

func (s *Store) ListAchievementCodes(ctx context.Context, playerID uuid.UUID) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT code FROM achievements WHERE player_id = $1 ORDER BY earned_at, code`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func (s *Store) Leaderboard(ctx context.Context, limit int) ([]domain.Player, error) {
	return s.queryLeaderboard(ctx, ``, nil, limit)
}

func (s *Store) LeaderboardScoped(ctx context.Context, scope, groupID string, limit int) ([]domain.Player, error) {
	var where string
	var args []any
	switch scope {
	case "depot":
		where = `WHERE depot_id = $1`
		args = []any{groupID}
	case "brigade":
		where = `WHERE brigade_id = $1`
		args = []any{groupID}
	case "company":
		// everyone
	default:
		return nil, fmt.Errorf("unsupported leaderboard scope %q", scope)
	}
	return s.queryLeaderboard(ctx, where, args, limit)
}

func (s *Store) queryLeaderboard(ctx context.Context, where string, args []any, limit int) ([]domain.Player, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+playerColumns+` FROM players `+where+` ORDER BY total_xp DESC, created_at ASC LIMIT $`+fmt.Sprint(len(args)+1),
		append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Player
	for rows.Next() {
		var p domain.Player
		if err := rows.Scan(scanPlayer(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PointsLeaderboard returns the full cohort so rank and percentile can be
// calculated before the API truncates the visible page.
func (s *Store) PointsLeaderboard(ctx context.Context, namespace, scope, groupID string) ([]repo.PointsStanding, error) {
	var where string
	switch scope {
	case "company":
	case "depot":
		where = `WHERE p.depot_id = $2`
	case "brigade":
		where = `WHERE p.brigade_id = $2`
	default:
		return nil, fmt.Errorf("unsupported leaderboard scope %q", scope)
	}
	args := []any{namespace}
	if scope != "company" {
		args = append(args, groupID)
	}
	rows, err := s.pool.Query(ctx, `SELECT p.id, p.username, COALESCE(SUM(l.points_delta), 0)::int AS points
		FROM players p LEFT JOIN points_ledger l ON l.player_id = p.id AND l.namespace = $1 `+where+`
		GROUP BY p.id, p.username, p.created_at ORDER BY points DESC, p.created_at ASC, p.id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []repo.PointsStanding{}
	for rows.Next() {
		var item repo.PointsStanding
		if err := rows.Scan(&item.PlayerID, &item.Username, &item.Points); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpsertExternalUser creates or refreshes a profile linked to an external HR
// system. It is idempotent by (source_system, external_user_id).
func (s *Store) UpsertExternalUser(ctx context.Context, sourceSystem, externalUserID string, displayName, depotID, brigadeID *string, assignedClassIDs []string) (domain.Player, bool, error) {
	if assignedClassIDs == nil {
		assignedClassIDs = []string{}
	}
	existing, err := s.GetPlayerByExternal(ctx, sourceSystem, externalUserID)
	if err == nil {
		_, updateErr := s.pool.Exec(ctx,
			`UPDATE players SET display_name = $2, assigned_class_ids = $3, depot_id = $4, brigade_id = $5
			 WHERE id = $1`, existing.ID, displayName, assignedClassIDs, depotID, brigadeID)
		if updateErr != nil {
			return domain.Player{}, false, updateErr
		}
		updated, err := s.GetPlayerByID(ctx, existing.ID)
		return updated, false, err
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return domain.Player{}, false, err
	}

	email := fmt.Sprintf("%s@%s.external", externalUserID, sourceSystem)
	username := externalUserID
	if displayName != nil && *displayName != "" {
		username = *displayName
	}

	var p domain.Player
	err = s.pool.QueryRow(ctx,
		`INSERT INTO players (email, username, password_hash, role, display_name, source_system, external_user_id, assigned_class_ids, depot_id, brigade_id)
		 VALUES ($1, $2, '', 'user', $3, $4, $5, $6, $7, $8)
		 RETURNING `+playerColumns,
		email, username, displayName, sourceSystem, externalUserID, assignedClassIDs, depotID, brigadeID,
	).Scan(scanPlayer(&p)...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			p, getErr := s.GetPlayerByExternal(ctx, sourceSystem, externalUserID)
			return p, false, getErr
		}
		return domain.Player{}, false, err
	}
	return p, true, nil
}
