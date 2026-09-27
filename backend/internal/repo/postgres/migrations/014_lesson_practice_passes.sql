CREATE TABLE lesson_practice_passes (
    player_id UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    lesson_id TEXT NOT NULL,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    passed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, lesson_id, session_id)
);
