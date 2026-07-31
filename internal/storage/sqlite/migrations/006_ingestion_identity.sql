CREATE TABLE raw_event_dedup (
    ingestion_identity TEXT PRIMARY KEY,
    raw_event_id TEXT NOT NULL REFERENCES raw_events(id) DEFERRABLE INITIALLY DEFERRED,
    source TEXT NOT NULL,
    first_received_at TEXT NOT NULL,
    last_received_at TEXT NOT NULL,
    suppression_count INTEGER NOT NULL DEFAULT 0 CHECK (suppression_count >= 0)
);

CREATE INDEX idx_raw_event_dedup_raw_event_id ON raw_event_dedup(raw_event_id);

ALTER TABLE model_calls ADD COLUMN raw_event_id TEXT REFERENCES raw_events(id);

CREATE UNIQUE INDEX idx_model_calls_raw_event_id ON model_calls(raw_event_id)
WHERE raw_event_id IS NOT NULL;
