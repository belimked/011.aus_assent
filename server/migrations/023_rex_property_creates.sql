-- App → Rex property creation ledger (owner added to a property not yet in Rex).
-- state: sending (request in flight or process died mid-call) | unknown (no
-- response) | created | linked (existing Rex property found by address).
-- sending/unknown block every further create for that property until an operator
-- reconciles (axleo rex-property-backfill --retry-unknown), so a lost response
-- never produces a second Rex property.
CREATE TABLE IF NOT EXISTS rex_property_creates (
  property_id TEXT NOT NULL,
  account_id TEXT NOT NULL,
  state TEXT NOT NULL,
  rex_id TEXT,
  reason TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(property_id, account_id)
);
