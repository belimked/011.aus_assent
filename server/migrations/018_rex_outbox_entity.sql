-- Per-entity outbox lookups (sync status, property note jobs) seek instead of scanning.
CREATE INDEX IF NOT EXISTS rex_outbox_entity ON rex_outbox(entity,entity_id,id);
