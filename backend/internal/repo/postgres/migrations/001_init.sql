-- VSM-400 Conductor Trainer — initial schema

-- Players
CREATE TABLE IF NOT EXISTS players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    total_xp INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Competency reference
CREATE TABLE IF NOT EXISTS competencies (
    id SERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL
);

-- Player xp per competency
CREATE TABLE IF NOT EXISTS player_competencies (
    player_id UUID REFERENCES players(id) ON DELETE CASCADE,
    competency_id INT REFERENCES competencies(id) ON DELETE CASCADE,
    xp INT DEFAULT 0,
    PRIMARY KEY (player_id, competency_id)
);

-- Work shift (session)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID REFERENCES players(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT now(),
    finished_at TIMESTAMPTZ
);

-- Situations inside a session
CREATE TABLE IF NOT EXISTS situations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'active',
    passenger_params JSONB,
    loyalty INT DEFAULT 50,
    safety INT DEFAULT 50,
    timer_deadline TIMESTAMPTZ,
    outcome TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Dialog history
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    situation_id UUID REFERENCES situations(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Achievements (optional)
CREATE TABLE IF NOT EXISTS achievements (
    id SERIAL PRIMARY KEY,
    player_id UUID REFERENCES players(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    earned_at TIMESTAMPTZ DEFAULT now()
);

-- Refresh tokens
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID REFERENCES players(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Seed competencies
INSERT INTO competencies (code, name) VALUES
    ('empathy', 'Эмпатия'),
    ('safety', 'Безопасность'),
    ('service', 'Сервис'),
    ('communication', 'Коммуникация')
ON CONFLICT (code) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_situations_session ON situations(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_situation ON messages(situation_id);
CREATE INDEX IF NOT EXISTS idx_sessions_player ON sessions(player_id);
