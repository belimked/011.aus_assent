-- Phones only receive properties matching propertySyncSQL (sync.go). These
-- related writes can make a public parcel eligible without touching the
-- properties row, so they log a property change for the incremental pull.
-- Rows for ids that are not properties are ignored by the pull's JOIN.
CREATE TRIGGER property_ownership_sync_insert AFTER INSERT ON property_ownership BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',NEW.property_id);
END;
CREATE TRIGGER property_ownership_sync_update AFTER UPDATE ON property_ownership BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',NEW.property_id);
END;
CREATE TRIGGER property_owners_sync_insert AFTER INSERT ON property_owners BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',NEW.property_id);
END;
CREATE TRIGGER rex_links_property_sync_insert AFTER INSERT ON rex_links WHEN NEW.entity='properties' BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',NEW.local_id);
END;
CREATE TRIGGER photos_property_sync_insert AFTER INSERT ON photos WHEN COALESCE(NEW.property_id,'')<>'' BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',NEW.property_id);
END;
-- To-dos reference their property in data.propId (property_id is rarely set).
CREATE TRIGGER todos_property_sync_insert AFTER INSERT ON todos
 WHEN COALESCE(NULLIF(NEW.property_id,''),CASE WHEN json_valid(NEW.data) THEN json_extract(NEW.data,'$.propId') END,'')<>'' BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',COALESCE(NULLIF(NEW.property_id,''),json_extract(NEW.data,'$.propId')));
END;
CREATE TRIGGER todos_property_sync_update AFTER UPDATE ON todos
 WHEN COALESCE(NULLIF(NEW.property_id,''),CASE WHEN json_valid(NEW.data) THEN json_extract(NEW.data,'$.propId') END,'')<>'' BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',COALESCE(NULLIF(NEW.property_id,''),json_extract(NEW.data,'$.propId')));
END;
