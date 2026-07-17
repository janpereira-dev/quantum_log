CREATE TABLE IF NOT EXISTS project_tags (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    tag_key TEXT NOT NULL,
    tag_value TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(project_id, tag_key, tag_value)
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    primary_project_id TEXT REFERENCES projects(id),
    initial_work_context_id TEXT REFERENCES work_contexts(id),
    title TEXT NOT NULL,
    task_type TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    duration_ms INTEGER,
    result TEXT NOT NULL DEFAULT '',
    human_outcome TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    agent_name TEXT NOT NULL DEFAULT '',
    agent_version TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS turns (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    work_context_id TEXT REFERENCES work_contexts(id),
    started_at TEXT NOT NULL,
    finished_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS model_calls (
    id TEXT PRIMARY KEY,
    primary_project_id TEXT REFERENCES projects(id),
    project_location_id TEXT REFERENCES project_locations(id),
    work_context_id TEXT REFERENCES work_contexts(id),
    task_id TEXT REFERENCES tasks(id),
    session_id TEXT REFERENCES sessions(id),
    turn_id TEXT REFERENCES turns(id),
    started_at TEXT NOT NULL,
    finished_at TEXT,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    agent_name TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL,
    model_id TEXT NOT NULL,
    model_version TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    cached_input_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost_usd_micros INTEGER NOT NULL DEFAULT 0,
    estimated_cost_eur_micros INTEGER NOT NULL DEFAULT 0,
    actual_cost_usd_micros INTEGER,
    actual_cost_eur_micros INTEGER,
    capture_quality TEXT NOT NULL DEFAULT 'unknown',
    project_resolution_method TEXT NOT NULL DEFAULT 'unresolved',
    project_resolution_confidence TEXT NOT NULL DEFAULT 'unknown',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_model_calls_started_at ON model_calls(started_at);
CREATE INDEX IF NOT EXISTS idx_model_calls_project_id ON model_calls(primary_project_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_provider_model ON model_calls(provider, model_id);
CREATE INDEX IF NOT EXISTS idx_project_tags_project_id ON project_tags(project_id);
