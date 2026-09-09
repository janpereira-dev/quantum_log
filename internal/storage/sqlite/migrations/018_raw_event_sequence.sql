ALTER TABLE raw_events ADD COLUMN event_sequence INTEGER NOT NULL DEFAULT 0;

-- Assign a deterministic monotonic position to legacy rows without using
-- occurred_at as an evidence boundary. ROW_NUMBER keeps the backfill linear
-- in the ordered input instead of performing a correlated COUNT per row.
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY created_at, id) AS sequence
    FROM raw_events
)
UPDATE raw_events
SET event_sequence = (SELECT sequence FROM ranked WHERE ranked.id = raw_events.id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_raw_events_event_sequence
    ON raw_events(event_sequence);
