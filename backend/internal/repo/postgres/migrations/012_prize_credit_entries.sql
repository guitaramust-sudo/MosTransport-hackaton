CREATE TABLE prize_credit_entries (
	id BIGSERIAL PRIMARY KEY,
	player_id UUID NOT NULL REFERENCES players(id),
	source_type TEXT NOT NULL, -- 'lesson_completion' | 'demo_seed'
	source_id TEXT NOT NULL,   -- lesson_id for lesson_completion; a fixed demo label for demo_seed
	amount INT NOT NULL,
	awarded_at TIMESTAMPTZ NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	UNIQUE (player_id, source_type, source_id)
);
