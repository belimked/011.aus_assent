ALTER TABLE rex_links ADD COLUMN account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE rex_links ADD COLUMN etag TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX rex_links_remote ON rex_links(entity,account_id,rex_id);
ALTER TABLE properties ADD COLUMN owner_id TEXT REFERENCES users(id);
CREATE INDEX properties_import_owner ON properties(owner_id) WHERE owner_id IS NOT NULL;
