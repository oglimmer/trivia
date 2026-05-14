ALTER TABLE users
    ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS idx_users_game_last_seen ON users(game_id, last_seen);
