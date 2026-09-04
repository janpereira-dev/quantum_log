ALTER TABLE raw_events ADD COLUMN event_sequence INTEGER NOT NULL DEFAULT 0;

-- Assign a deterministic monotonic position to legacy rows without using
-- occurred_at as an evidence boundary.
UPDATE raw_events
SET event_sequence = (
    SELECT COUNT(*)
    FROM raw_events prior
    WHERE prior.created_at < raw_events.created_at
       OR (prior.created_at = raw_events.created_at AND prior.id <= raw_events.id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_raw_events_event_sequence
    ON raw_events(event_sequence);
