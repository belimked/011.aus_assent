-- UAT 3 competitor marker: property reads derive competitorListing from
-- market_events (competitor_listing.go). New or re-linked events change that
-- derived field without touching the properties row, so log a property change
-- for the incremental pull (same pattern as 016 for property_ownership).
CREATE TRIGGER market_events_sync_insert AFTER INSERT ON market_events WHEN NEW.property_id IS NOT NULL BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',NEW.property_id);
END;
CREATE TRIGGER market_events_sync_update AFTER UPDATE ON market_events BEGIN
 INSERT INTO sync_changes(kind,id) SELECT 'properties',NEW.property_id WHERE NEW.property_id IS NOT NULL;
 INSERT INTO sync_changes(kind,id) SELECT 'properties',OLD.property_id WHERE OLD.property_id IS NOT NULL AND OLD.property_id IS NOT NEW.property_id;
END;
CREATE TRIGGER market_events_sync_delete AFTER DELETE ON market_events WHEN OLD.property_id IS NOT NULL BEGIN
 INSERT INTO sync_changes(kind,id) VALUES('properties',OLD.property_id);
END;
