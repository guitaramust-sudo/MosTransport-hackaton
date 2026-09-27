CREATE TABLE push_subscriptions (
	player_id UUID NOT NULL REFERENCES players(id),
	platform TEXT NOT NULL,
	device_token TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (device_token)
);

CREATE TABLE push_notifications_sent (
	player_id UUID NOT NULL REFERENCES players(id),
	event_type TEXT NOT NULL,
	source_id TEXT NOT NULL,
	sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (player_id, event_type, source_id)
);
