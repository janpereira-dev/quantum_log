CREATE TABLE IF NOT EXISTS tool_calls (
    id TEXT PRIMARY KEY,
    primary_project_id TEXT REFERENCES projects(id),
    project_location_id TEXT REFERENCES project_locations(id),
    work_context_id TEXT REFERENCES work_contexts(id),
    model_call_id TEXT REFERENCES model_calls(id),
    task_id TEXT REFERENCES tasks(id),
    session_id TEXT REFERENCES sessions(id),
    tool_name TEXT NOT NULL,
    tool_type TEXT NOT NULL DEFAULT '',
    mcp_server TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    success INTEGER NOT NULL DEFAULT 1,
    input_size_bytes INTEGER NOT NULL DEFAULT 0,
    output_size_bytes INTEGER NOT NULL DEFAULT 0,
    project_resolution_method TEXT NOT NULL DEFAULT 'unresolved',
    project_resolution_confidence TEXT NOT NULL DEFAULT 'unknown',
    capture_quality TEXT NOT NULL DEFAULT 'unknown',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pricing_rules (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model_pattern TEXT NOT NULL,
    valid_from TEXT NOT NULL,
    valid_until TEXT,
    billing_mode TEXT NOT NULL,
    currency TEXT NOT NULL,
    unit_tokens INTEGER NOT NULL,
    rule_json TEXT NOT NULL,
    catalog_version TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cost_snapshots (
    id TEXT PRIMARY KEY,
    model_call_id TEXT NOT NULL REFERENCES model_calls(id),
    pricing_rule_id TEXT REFERENCES pricing_rules(id),
    pricing_catalog_version TEXT NOT NULL,
    calculation_formula_version TEXT NOT NULL,
    calculated_at TEXT NOT NULL,
    estimated_cost_usd_micros INTEGER NOT NULL DEFAULT 0,
    actual_cost_usd_micros INTEGER,
    allocated_cost_usd_micros INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tool_calls_model_call_id ON tool_calls(model_call_id);
CREATE INDEX IF NOT EXISTS idx_cost_snapshots_model_call_id ON cost_snapshots(model_call_id);
