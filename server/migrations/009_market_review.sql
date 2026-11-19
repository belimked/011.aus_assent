ALTER TABLE market_events ADD COLUMN review_status TEXT NOT NULL DEFAULT 'pending' CHECK(review_status IN ('pending','ignored','linked'));
CREATE INDEX market_events_review ON market_events(review_status,property_id);
