ALTER TABLE situations
    ADD COLUMN IF NOT EXISTS situation_def_id TEXT,
    ADD COLUMN IF NOT EXISTS passenger_id TEXT,
    ADD COLUMN IF NOT EXISTS escalations JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS remarks JSONB,
    ADD COLUMN IF NOT EXISTS score_result JSONB,
    ADD COLUMN IF NOT EXISTS xp INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ;

ALTER TABLE messages ADD COLUMN IF NOT EXISTS kind TEXT;

CREATE INDEX IF NOT EXISTS idx_situations_status_deadline
    ON situations (status, timer_deadline) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_players_total_xp ON players (total_xp DESC);
