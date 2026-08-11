CREATE TABLE IF NOT EXISTS interactions (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    upstream_id TEXT NOT NULL,
    raw_event_id TEXT REFERENCES raw_events(id),
    primary_project_id TEXT REFERENCES projects(id),
    project_location_id TEXT REFERENCES project_locations(id),
    work_context_id TEXT REFERENCES work_contexts(id),
    prompt_capture_mode TEXT NOT NULL DEFAULT 'hash',
    prompt_hash TEXT NOT NULL DEFAULT '',
    prompt_redacted TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(source, session_id, upstream_id)
);

ALTER TABLE model_calls ADD COLUMN interaction_id TEXT REFERENCES interactions(id);
ALTER TABLE tool_calls ADD COLUMN interaction_id TEXT REFERENCES interactions(id);

CREATE INDEX IF NOT EXISTS idx_interactions_occurred_at ON interactions(occurred_at);
CREATE INDEX IF NOT EXISTS idx_interactions_project_id ON interactions(primary_project_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_interaction_id ON model_calls(interaction_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_interaction_id ON tool_calls(interaction_id);
