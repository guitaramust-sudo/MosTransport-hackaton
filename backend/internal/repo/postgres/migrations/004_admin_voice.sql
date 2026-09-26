ALTER TABLE players
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user',
    ADD COLUMN IF NOT EXISTS display_name TEXT,
    ADD COLUMN IF NOT EXISTS source_system TEXT,
    ADD COLUMN IF NOT EXISTS external_user_id TEXT,
    ADD COLUMN IF NOT EXISTS assigned_class_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS depot_id TEXT,
    ADD COLUMN IF NOT EXISTS brigade_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_players_external
    ON players (source_system, external_user_id)
    WHERE source_system IS NOT NULL AND external_user_id IS NOT NULL;

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS validation_status TEXT NOT NULL DEFAULT 'draft';

ALTER TABLE messages ADD COLUMN IF NOT EXISTS input_mode TEXT;

ALTER TABLE player_competencies ADD COLUMN IF NOT EXISTS evidence_count INT NOT NULL DEFAULT 0;

INSERT INTO competencies (code, name) VALUES
    ('medical', 'Медицина'),
    ('conflict', 'Конфликты'),
    ('informational', 'Информирование')
ON CONFLICT (code) DO NOTHING;
