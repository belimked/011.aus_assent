CREATE TABLE rex_links (entity TEXT NOT NULL, local_id TEXT NOT NULL, rex_id TEXT NOT NULL, rex_service TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(entity,local_id));
ALTER TABLE users ADD COLUMN rex_user_id TEXT;
ALTER TABLE rex_outbox ADD COLUMN next_attempt_at TEXT;
CREATE INDEX rex_outbox_ready ON rex_outbox(status,next_attempt_at,id);
