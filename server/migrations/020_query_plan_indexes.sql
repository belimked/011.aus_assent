-- Indexes found by the query-plan check (docs/db-performance.md). Each one
-- replaces a full table scan on a request or background path; all are plain
-- CREATE INDEX on existing columns, no data is rewritten.

-- /sync and property reads: the newest relation job for a property is found by
-- its payload propertyId (entity_id is "<contactId>:..."), once per property.
-- Partial, so only relation payloads (always JSON from json.Marshal) are parsed.
CREATE INDEX IF NOT EXISTS rex_outbox_relation_property ON rex_outbox(json_extract(payload,'$.relation.propertyId'),id) WHERE entity='rex_contact_relation';

-- Contact reads (/sync, details, merge) load notes and owned properties by
-- contact; both are also the child side of a foreign key to contacts.
CREATE INDEX IF NOT EXISTS contact_notes_contact ON contact_notes(contact_id,created_at,id);
CREATE INDEX IF NOT EXISTS property_owners_contact ON property_owners(contact_id);

-- Agent to-do scope is "assignee_id=? OR created_by=?": the second column of
-- todos_scope cannot serve the OR branch on its own. contact_id: merge.
CREATE INDEX IF NOT EXISTS todos_created_by ON todos(created_by,updated_at);
CREATE INDEX IF NOT EXISTS todos_contact ON todos(contact_id) WHERE contact_id IS NOT NULL;

-- Every /sync/push prunes processed operations older than seven days.
CREATE INDEX IF NOT EXISTS sync_ops_created ON sync_ops(created_at);

-- Admin users list (last session per user); sessions grow with every login.
CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id,created_at);

-- audit_log grows with every write and had no index: merged-contact checks in
-- the Rex dedupe, admin metrics (at>=?) and the exports list (action='export').
CREATE INDEX IF NOT EXISTS audit_log_entity ON audit_log(entity,entity_id,id);
CREATE INDEX IF NOT EXISTS audit_log_at ON audit_log(at);
CREATE INDEX IF NOT EXISTS audit_log_action ON audit_log(action,id);

-- Admin market page and metrics: newest imports first, latest completed one.
CREATE INDEX IF NOT EXISTS market_imports_created ON market_imports(created_at);
CREATE INDEX IF NOT EXISTS market_imports_status ON market_imports(status,created_at);
