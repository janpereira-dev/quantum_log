ALTER TABLE tool_calls ADD COLUMN raw_event_id TEXT REFERENCES raw_events(id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tool_calls_raw_event_id ON tool_calls(raw_event_id) WHERE raw_event_id IS NOT NULL;
