CREATE TABLE IF NOT EXISTS games (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL DEFAULT 'setup' CHECK (state IN ('setup','game','finished')),
    current_question_id UUID,
    question_state TEXT NOT NULL DEFAULT 'idle' CHECK (question_state IN ('idle','active','revealed')),
    question_started_at TIMESTAMPTZ,
    question_closed_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id      UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    photo_b64    TEXT NOT NULL DEFAULT '',
    token        TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_game ON users(game_id);

CREATE TABLE IF NOT EXISTS questions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id       UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text          TEXT NOT NULL,
    photo_b64     TEXT NOT NULL DEFAULT '',
    -- answer_type: yesno | choice | number
    answer_type   TEXT NOT NULL CHECK (answer_type IN ('yesno','choice','number')),
    -- options: JSON array of strings for 'choice'; for 'yesno' implicitly ["Yes","No"]; for 'number' unused
    options       JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- correct: for yesno -> "yes"/"no"; for choice -> index int; for number -> numeric value
    correct       JSONB NOT NULL,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (game_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_questions_game ON questions(game_id);

CREATE TABLE IF NOT EXISTS answers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id   UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    answer        JSONB NOT NULL,
    response_ms   INT NOT NULL,
    is_correct    BOOLEAN NOT NULL,
    points        INT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (question_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_answers_question ON answers(question_id);
CREATE INDEX IF NOT EXISTS idx_answers_user ON answers(user_id);
