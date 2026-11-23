ALTER TABLE rex_request_log ADD COLUMN entity TEXT NOT NULL DEFAULT '';
ALTER TABLE rex_request_log ADD COLUMN local_id TEXT NOT NULL DEFAULT '';
ALTER TABLE rex_request_log ADD COLUMN rex_id TEXT NOT NULL DEFAULT '';
CREATE INDEX rex_request_log_entity ON rex_request_log(entity,local_id,rex_id);
CREATE TABLE rex_sync_events (id INTEGER PRIMARY KEY AUTOINCREMENT, at TEXT NOT NULL, direction TEXT NOT NULL, entity TEXT NOT NULL, local_id TEXT NOT NULL, rex_id TEXT NOT NULL, changes TEXT NOT NULL, result TEXT NOT NULL, error TEXT NOT NULL DEFAULT '');
CREATE INDEX rex_sync_events_entity ON rex_sync_events(entity,local_id,at);
CREATE TABLE rex_poll_status (id INTEGER PRIMARY KEY CHECK(id=1), at TEXT NOT NULL, ok INTEGER NOT NULL, error TEXT NOT NULL DEFAULT '');
-- One-time quarantine of legacy demonstration jobs; the worker also guards all future jobs.
UPDATE rex_outbox SET status='skipped',last_error='not linked to Rex sandbox record',next_attempt_at=NULL
 WHERE status IN ('pending','waiting_auth') AND NOT EXISTS (
 SELECT 1 FROM rex_links l WHERE l.entity=rex_outbox.entity AND l.local_id=rex_outbox.entity_id AND l.account_id='2057'
 );
