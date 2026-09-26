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

func (s *Store) CreateSimulationRun(ctx context.Context, run domain.SimulationRun) (domain.SimulationRun, error) {
	run.ID = uuid.New()
	raw, err := json.Marshal(run)
	if err != nil {
		return domain.SimulationRun{}, err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO simulation_runs (id, player_id, state, deadline_at) VALUES ($1, $2, $3, $4)`, run.ID, run.PlayerID, raw, run.DeadlineAt)
	return run, err
}

func (s *Store) GetSimulationRun(ctx context.Context, runID, playerID uuid.UUID) (domain.SimulationRun, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT state FROM simulation_runs WHERE id = $1 AND player_id = $2`, runID, playerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SimulationRun{}, repo.ErrNotFound
	}
	if err != nil {
		return domain.SimulationRun{}, err
	}
	var run domain.SimulationRun
	err = json.Unmarshal(raw, &run)
	return run, err
}

func (s *Store) ApplySimulationCommand(ctx context.Context, runID, playerID, commandID uuid.UUID, expectedVersion int, apply func(domain.SimulationRun) (domain.SimulationRun, error)) (domain.SimulationRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.SimulationRun{}, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT state FROM simulation_runs WHERE id = $1 AND player_id = $2 FOR UPDATE`, runID, playerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SimulationRun{}, repo.ErrNotFound
	}
	if err != nil {
		return domain.SimulationRun{}, err
	}
	var duplicate []byte
	err = tx.QueryRow(ctx, `SELECT result_state FROM simulation_commands WHERE run_id = $1 AND command_id = $2`, runID, commandID).Scan(&duplicate)
	if err == nil {
		var previous domain.SimulationRun
		if err := json.Unmarshal(duplicate, &previous); err != nil {
			return domain.SimulationRun{}, err
		}
		return previous, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.SimulationRun{}, err
	}
	var current domain.SimulationRun
	if err := json.Unmarshal(raw, &current); err != nil {
		return domain.SimulationRun{}, err
	}
	if current.StateVersion != expectedVersion {
		return domain.SimulationRun{}, repo.ErrConflict
	}
	next, err := apply(current)
	if err != nil {
		return domain.SimulationRun{}, err
	}
	next.ID, next.PlayerID = current.ID, current.PlayerID
	next.StateVersion = current.StateVersion + 1
	result, err := json.Marshal(next)
	if err != nil {
		return domain.SimulationRun{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE simulation_runs SET state = $2, deadline_at = $3 WHERE id = $1`, runID, result, next.DeadlineAt); err != nil {
		return domain.SimulationRun{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO simulation_commands (run_id, command_id, result_state) VALUES ($1, $2, $3)`, runID, commandID, result); err != nil {
		return domain.SimulationRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.SimulationRun{}, err
	}
	return next, nil
}

func (s *Store) AdvanceSimulationTimer(ctx context.Context, runID, playerID uuid.UUID, apply func(domain.SimulationRun) (domain.SimulationRun, bool)) (domain.SimulationRun, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.SimulationRun{}, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT state FROM simulation_runs WHERE id = $1 AND player_id = $2 FOR UPDATE`, runID, playerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SimulationRun{}, repo.ErrNotFound
	}
	if err != nil {
		return domain.SimulationRun{}, err
	}
	var current domain.SimulationRun
	if err := json.Unmarshal(raw, &current); err != nil {
		return domain.SimulationRun{}, err
	}
	next, changed := apply(current)
	if changed {
		next.StateVersion = current.StateVersion + 1
		result, err := json.Marshal(next)
		if err != nil {
			return domain.SimulationRun{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE simulation_runs SET state = $2, deadline_at = $3 WHERE id = $1`, runID, result, next.DeadlineAt); err != nil {
			return domain.SimulationRun{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.SimulationRun{}, err
		}
	}
	return next, nil
}

func (s *Store) ListDueSimulationRuns(ctx context.Context, now time.Time) ([]domain.SimulationDueRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, player_id FROM simulation_runs WHERE deadline_at <= $1 ORDER BY deadline_at LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SimulationDueRun
	for rows.Next() {
		var run domain.SimulationDueRun
		if err := rows.Scan(&run.ID, &run.PlayerID); err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}
