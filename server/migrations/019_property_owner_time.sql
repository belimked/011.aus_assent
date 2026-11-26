-- Contact merge resolves same-property relation conflicts by time ("latest wins"),
-- so each property_owners row records when its relation was last written.
-- History is unknown: backfill with the contact's updated_at (the latest moment the
-- relation can be attributed to), falling back to the property's, then created times.
ALTER TABLE property_owners ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
UPDATE property_owners SET updated_at=COALESCE(
 (SELECT NULLIF(c.updated_at,'') FROM contacts c WHERE c.id=property_owners.contact_id),
 (SELECT NULLIF(p.updated_at,'') FROM properties p WHERE p.id=property_owners.property_id),
 '');
-- Writers that do not set the column (older code paths, imports, demo seed) get the
-- write time. A relation change without an explicit time is stamped as well.
CREATE TRIGGER property_owners_stamp_insert AFTER INSERT ON property_owners WHEN NEW.updated_at='' BEGIN
 UPDATE property_owners SET updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE property_id=NEW.property_id AND contact_id=NEW.contact_id;
END;
CREATE TRIGGER property_owners_stamp_update AFTER UPDATE OF relation ON property_owners WHEN NEW.relation<>OLD.relation AND NEW.updated_at=OLD.updated_at BEGIN
 UPDATE property_owners SET updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE property_id=NEW.property_id AND contact_id=NEW.contact_id;
END;
