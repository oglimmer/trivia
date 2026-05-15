CREATE UNIQUE INDEX IF NOT EXISTS uq_users_game_name_lower
    ON users(game_id, lower(name));
