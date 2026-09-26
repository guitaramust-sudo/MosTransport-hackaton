CREATE TABLE IF NOT EXISTS simulation_runs (
    id UUID PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    state JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS simulation_commands (
    run_id UUID NOT NULL REFERENCES simulation_runs(id) ON DELETE CASCADE,
    command_id UUID NOT NULL,
    result_state JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, command_id)
);
