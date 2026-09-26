ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS pending_situations JSONB NOT NULL DEFAULT '[]'::jsonb;
