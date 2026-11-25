-- Phones map properties only through data.ll ([lat,lng]). Bundled DCDB parcels
-- (and everything Pricefinder matched onto them) were stored with lat/lng only.
-- Backfill ll from numeric, plausible Australian lat/lng (same bounds as
-- property_ll.go). The UPDATE fires properties_sync_update, so incremental
-- /sync pulls deliver the new coordinates; version bumps keep merges ordered.
UPDATE properties
SET data=json_set(data,'$.ll',json_array(json_extract(data,'$.lat'),json_extract(data,'$.lng'))),
    updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),
    version=version+1
WHERE deleted_at IS NULL AND json_valid(data)
  AND json_type(data,'$.lat') IN ('integer','real') AND json_type(data,'$.lng') IN ('integer','real')
  AND json_extract(data,'$.lat') BETWEEN -44 AND -9 AND json_extract(data,'$.lng') BETWEEN 112 AND 154
  AND NOT (json_type(data,'$.ll') IS 'array' AND json_array_length(data,'$.ll')=2
           AND json_type(data,'$.ll[0]') IN ('integer','real') AND json_type(data,'$.ll[1]') IN ('integer','real'));
