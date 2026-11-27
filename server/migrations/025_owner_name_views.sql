-- UAT 3 item 6: RP Data registered owner names are read on demand
-- (GET /api/v1/properties/{id}/owner-names) and every view is logged here.
-- One row per user and property per 10 minutes. Cleared by reset-data.
CREATE TABLE owner_name_views (
 id TEXT PRIMARY KEY,
 team_id TEXT NOT NULL,
 user_id TEXT NOT NULL,
 property_id TEXT NOT NULL,
 source TEXT NOT NULL DEFAULT '',
 at TEXT NOT NULL
);
CREATE INDEX owner_name_views_property_at ON owner_name_views(property_id, at);
CREATE INDEX owner_name_views_user_at ON owner_name_views(user_id, at);
