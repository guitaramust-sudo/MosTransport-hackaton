ALTER TABLE simulation_runs ADD COLUMN IF NOT EXISTS deadline_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_simulation_runs_deadline ON simulation_runs (deadline_at) WHERE deadline_at IS NOT NULL;
