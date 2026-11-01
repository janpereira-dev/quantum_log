CREATE INDEX IF NOT EXISTS idx_raw_events_import_checkpoint
    ON raw_events(source, session_id, occurred_at DESC);
