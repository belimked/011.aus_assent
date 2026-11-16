ALTER TABLE properties ADD COLUMN registered INTEGER NOT NULL DEFAULT 0;
UPDATE properties SET registered=1 WHERE id IN ('p1','p2','p3','p4','p5','p6','p7','p8','p9','p10','p11','p12','p13','p14','p15','p16','p17','p18','p19','p20');
-- old_a/old_b: owner (contacts) or assignee/creator (todos) before the change, so a user who loses access gets a tombstone.
CREATE TABLE sync_changes (seq INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, id TEXT NOT NULL, old_a TEXT, old_b TEXT);
CREATE INDEX sync_changes_lookup ON sync_changes(kind,id,seq);
INSERT INTO sync_changes(kind,id) SELECT 'contacts',id FROM contacts;
INSERT INTO sync_changes(kind,id) SELECT 'properties',id FROM properties WHERE registered=1;
INSERT INTO sync_changes(kind,id) SELECT 'todos',id FROM todos;
CREATE TRIGGER contacts_sync_insert AFTER INSERT ON contacts BEGIN INSERT INTO sync_changes(kind,id) VALUES ('contacts',NEW.id); END;
CREATE TRIGGER contacts_sync_update AFTER UPDATE ON contacts BEGIN INSERT INTO sync_changes(kind,id,old_a) VALUES ('contacts',NEW.id,OLD.owner_id); END;
CREATE TRIGGER properties_sync_insert AFTER INSERT ON properties BEGIN INSERT INTO sync_changes(kind,id) VALUES ('properties',NEW.id); END;
CREATE TRIGGER properties_sync_update AFTER UPDATE ON properties BEGIN INSERT INTO sync_changes(kind,id) VALUES ('properties',NEW.id); END;
CREATE TRIGGER todos_sync_insert AFTER INSERT ON todos BEGIN INSERT INTO sync_changes(kind,id) VALUES ('todos',NEW.id); END;
CREATE TRIGGER todos_sync_update AFTER UPDATE ON todos BEGIN INSERT INTO sync_changes(kind,id,old_a,old_b) VALUES ('todos',NEW.id,OLD.assignee_id,OLD.created_by); END;
CREATE TABLE sync_ops (user_id TEXT NOT NULL, op_id TEXT NOT NULL, result TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(user_id,op_id));
