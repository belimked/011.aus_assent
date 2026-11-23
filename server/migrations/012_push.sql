CREATE TABLE device_tokens (user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, platform TEXT NOT NULL, token TEXT NOT NULL UNIQUE, updated_at TEXT NOT NULL, PRIMARY KEY(user_id,platform,token));
CREATE INDEX device_tokens_user ON device_tokens(user_id);
