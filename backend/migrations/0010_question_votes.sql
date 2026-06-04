-- Best-question voting. Once the game is finished, each player may cast a
-- single vote for the question they liked most. The (game_id, user_id) unique
-- constraint enforces one vote per player per game; votes are final once cast.
CREATE TABLE IF NOT EXISTS question_votes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id     UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (game_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_question_votes_question ON question_votes(question_id);
