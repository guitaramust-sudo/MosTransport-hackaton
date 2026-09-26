package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

const weeklyChallengeID = "weekly_variety_1"

func weekStart(now time.Time) time.Time {
	utc := now.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}

// FinalizeSimulationRewards can be retried after a crash or network error.
// The player lock serializes challenge progress across simultaneous runs.
func (s *Store) FinalizeSimulationRewards(ctx context.Context, run domain.SimulationRun) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var playerID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM players WHERE id = $1 FOR UPDATE`, run.PlayerID).Scan(&playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT state FROM simulation_runs WHERE id = $1 AND player_id = $2 FOR UPDATE`, run.ID, run.PlayerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	if err != nil {
		return err
	}
	var current domain.SimulationRun
	if err := json.Unmarshal(raw, &current); err != nil {
		return err
	}
	if current.Status != "finished" {
		return repo.ErrConflict
	}
	// Only passed, published content can earn official points. Demo points live
	// in their own namespace and never inherit XP from legacy dialogue shifts.
	var template struct {
		ValidationStatus string `json:"validation_status"`
	}
	if err := json.Unmarshal(current.TemplateSnapshot, &template); err != nil {
		return err
	}
	eligible := current.Passed && current.SeedVariant != "" &&
		((current.PointsNamespace == "official" && template.ValidationStatus == "approved") ||
			(current.PointsNamespace == "demo" && (template.ValidationStatus == "draft" || template.ValidationStatus == "approved")))
	if eligible {
		var existing int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM points_ledger WHERE player_id = $1 AND namespace = $2 AND scenario_family = $3`,
			current.PlayerID, current.PointsNamespace, current.ScenarioID).Scan(&existing); err != nil {
			return err
		}
		if existing < 2 {
			points := 20
			if existing == 1 {
				points = 10
			}
			if _, err := tx.Exec(ctx, `INSERT INTO points_ledger (player_id, run_id, namespace, scenario_family, seed_variant, points_delta)
				VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
				current.PlayerID, current.ID, current.PointsNamespace, current.ScenarioID, current.SeedVariant, points); err != nil {
				return err
			}
		}
	}
	if current.Passed {
		if _, err := tx.Exec(ctx, `INSERT INTO achievements (player_id, code) VALUES ($1, 'first_complete') ON CONFLICT DO NOTHING`, run.PlayerID); err != nil {
			return err
		}
	}
	for _, entry := range current.ActionLog {
		if entry.EffectID == "cue_discovered" {
			if _, err := tx.Exec(ctx, `INSERT INTO achievements (player_id, code) VALUES ($1, 'first_signal') ON CONFLICT DO NOTHING`, run.PlayerID); err != nil {
				return err
			}
			break
		}
	}
	period := weekStart(current.StartedAt)
	if current.Passed && current.SeedVariant != "" {
		var variants int
		err := tx.QueryRow(ctx, `SELECT COUNT(DISTINCT state->>'seed_variant') FROM simulation_runs
			WHERE player_id = $1 AND created_at >= $2 AND created_at < $3
			AND state->>'passed' = 'true' AND state->>'seed_variant' IN ('A', 'B')`,
			run.PlayerID, period, period.AddDate(0, 0, 7)).Scan(&variants)
		if err != nil {
			return err
		}
		if variants >= 2 {
			result, err := tx.Exec(ctx, `INSERT INTO challenge_awards (player_id, challenge_id, period_start, xp)
				VALUES ($1, $2, $3, 5) ON CONFLICT DO NOTHING`, run.PlayerID, weeklyChallengeID, period)
			if err != nil {
				return err
			}
			if result.RowsAffected() == 1 {
				if _, err := tx.Exec(ctx, `UPDATE players SET total_xp = total_xp + 5 WHERE id = $1`, run.PlayerID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO notifications (player_id, type, subject_key, payload)
					VALUES ($1, 'challenge_completed', $2::text, jsonb_build_object('challenge_id', $3::text, 'xp', 5)) ON CONFLICT DO NOTHING`,
					run.PlayerID, weeklyChallengeID+":"+period.Format("2006-01-02"), weeklyChallengeID); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetSimulationPoints(ctx context.Context, runID uuid.UUID, namespace string) (int, error) {
	var points int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(points_delta), 0) FROM points_ledger WHERE run_id = $1 AND namespace = $2`, runID, namespace).Scan(&points)
	return points, err
}

func (s *Store) GetPlayerPointsTotal(ctx context.Context, playerID uuid.UUID, namespace string) (int, error) {
	var points int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(points_delta), 0) FROM points_ledger WHERE player_id = $1 AND namespace = $2`, playerID, namespace).Scan(&points)
	return points, err
}

func (s *Store) ListNotifications(ctx context.Context, playerID uuid.UUID) ([]domain.Notification, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, type, subject_key, payload, created_at FROM notifications WHERE player_id = $1 ORDER BY id DESC LIMIT 100`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Notification{}
	for rows.Next() {
		var item domain.Notification
		if err := rows.Scan(&item.ID, &item.Type, &item.SubjectKey, &item.Payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetChallengeProgress(ctx context.Context, playerID uuid.UUID, now time.Time) (domain.ChallengeProgress, error) {
	period := weekStart(now)
	progress := domain.ChallengeProgress{ChallengeID: weeklyChallengeID, Target: 2, RewardXP: 5}
	err := s.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT state->>'seed_variant') FROM simulation_runs
		WHERE player_id = $1 AND created_at >= $2 AND created_at < $3
		AND state->>'passed' = 'true' AND state->>'seed_variant' IN ('A', 'B')`,
		playerID, period, period.AddDate(0, 0, 7)).Scan(&progress.SeedVariants)
	if err != nil {
		return domain.ChallengeProgress{}, err
	}
	err = s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM challenge_awards WHERE player_id = $1 AND challenge_id = $2 AND period_start = $3)`,
		playerID, weeklyChallengeID, period).Scan(&progress.Completed)
	return progress, err
}
