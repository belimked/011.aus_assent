-- Per-agent activity feed (Home "My activity"); synced like other B3 kinds so it survives re-login and new devices.
CREATE TABLE activity (id TEXT PRIMARY KEY, team_id TEXT NOT NULL DEFAULT 'default', owner_id TEXT, data TEXT NOT NULL DEFAULT '{}', created_at TEXT, updated_at TEXT, version INTEGER NOT NULL DEFAULT 1, deleted_at TEXT);
CREATE INDEX activity_owner_sync ON activity(owner_id,updated_at);
CREATE TRIGGER activity_b3_insert AFTER INSERT ON activity BEGIN INSERT INTO sync_changes(kind,id) VALUES ('activity',NEW.id); END;
CREATE TRIGGER activity_b3_update AFTER UPDATE ON activity BEGIN INSERT INTO sync_changes(kind,id,old_a) VALUES ('activity',NEW.id,OLD.owner_id); END;
