CREATE TABLE IF NOT EXISTS points_ledger (
    id BIGSERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES simulation_runs(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL CHECK (namespace IN ('demo', 'official')),
    scenario_family TEXT NOT NULL,
    seed_variant TEXT NOT NULL,
    points_delta INT NOT NULL CHECK (points_delta > 0),
    awarded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (namespace, run_id),
    UNIQUE (namespace, player_id, scenario_family, seed_variant)
);

CREATE INDEX IF NOT EXISTS idx_points_ledger_namespace_player ON points_ledger (namespace, player_id);
