CREATE UNIQUE INDEX IF NOT EXISTS idx_achievements_player_code ON achievements (player_id, code);

CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    subject_key TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, type, subject_key)
);

CREATE TABLE IF NOT EXISTS challenge_awards (
    player_id UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    challenge_id TEXT NOT NULL,
    period_start DATE NOT NULL,
    xp INT NOT NULL,
    awarded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, challenge_id, period_start)
);
