CREATE TABLE lesson_progress (
	player_id UUID NOT NULL REFERENCES players(id),
	lesson_id TEXT NOT NULL,
	theory_pass BOOLEAN NOT NULL DEFAULT false,
	practice_session_id UUID,
	practice_pass BOOLEAN NOT NULL DEFAULT false,
	practice_check_pass BOOLEAN NOT NULL DEFAULT false,
	completed_at TIMESTAMPTZ,
	content_version TEXT NOT NULL DEFAULT '1.0.0',
	PRIMARY KEY (player_id, lesson_id)
);

CREATE TABLE lesson_answers (
	id BIGSERIAL PRIMARY KEY,
	player_id UUID NOT NULL REFERENCES players(id),
	lesson_id TEXT NOT NULL,
	question_id TEXT NOT NULL,
	option_id TEXT NOT NULL,
	correct BOOLEAN NOT NULL,
	answered_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lesson_awards (
	player_id UUID NOT NULL REFERENCES players(id),
	lesson_id TEXT NOT NULL,
	award_type TEXT NOT NULL,
	xp_delta INT NOT NULL,
	badge_id TEXT,
	awarded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (player_id, lesson_id, award_type)
);
