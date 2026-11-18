CREATE TABLE photos (id TEXT PRIMARY KEY, property_id TEXT, created_by TEXT NOT NULL, mime TEXT NOT NULL, size INTEGER NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX photos_created_by ON photos(created_by);
