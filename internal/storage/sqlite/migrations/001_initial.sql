CREATE TABLE IF NOT EXISTS hosts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    os TEXT NOT NULL,
    arch TEXT NOT NULL,
    machine_fingerprint_hash TEXT NOT NULL,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    canonical_key TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    project_type TEXT NOT NULL DEFAULT '',
    repository_url_normalized TEXT NOT NULL DEFAULT '',
    vcs_provider TEXT NOT NULL DEFAULT '',
    default_branch TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    archived_at TEXT
);

CREATE TABLE IF NOT EXISTS project_locations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    host_id TEXT REFERENCES hosts(id),
    absolute_path TEXT NOT NULL UNIQUE,
    path_hash TEXT NOT NULL,
    vcs_root TEXT NOT NULL DEFAULT '',
    workspace_root TEXT NOT NULL DEFAULT '',
    worktree_name TEXT NOT NULL DEFAULT '',
    is_primary INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS work_contexts (
    id TEXT PRIMARY KEY,
    primary_project_id TEXT REFERENCES projects(id),
    project_location_id TEXT REFERENCES project_locations(id),
    task_id TEXT,
    session_id TEXT,
    host_id TEXT REFERENCES hosts(id),
    cwd TEXT NOT NULL,
    git_root TEXT NOT NULL DEFAULT '',
    workspace_root TEXT NOT NULL DEFAULT '',
    workspace_name TEXT NOT NULL DEFAULT '',
    git_branch TEXT NOT NULL DEFAULT '',
    git_commit TEXT NOT NULL DEFAULT '',
    terminal_id TEXT NOT NULL DEFAULT '',
    process_id INTEGER NOT NULL DEFAULT 0,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    resolution_method TEXT NOT NULL,
    resolution_confidence TEXT NOT NULL,
    resolution_evidence_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK(finished_at IS NULL OR finished_at >= started_at)
);

CREATE TABLE IF NOT EXISTS raw_events (
    id TEXT PRIMARY KEY,
    source_event_id TEXT,
    schema_version INTEGER NOT NULL DEFAULT 1,
    adapter_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    source_version TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    received_at TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    parent_span_id TEXT NOT NULL DEFAULT '',
    project_id TEXT REFERENCES projects(id),
    project_location_id TEXT REFERENCES project_locations(id),
    work_context_id TEXT REFERENCES work_contexts(id),
    task_id TEXT,
    session_id TEXT,
    project_resolution_method TEXT NOT NULL,
    project_resolution_confidence TEXT NOT NULL,
    project_resolution_evidence_json TEXT NOT NULL DEFAULT '{}',
    payload_json_sanitized TEXT NOT NULL,
    capture_source TEXT NOT NULL DEFAULT 'manual',
    capture_quality TEXT NOT NULL DEFAULT 'unknown',
    confidence TEXT NOT NULL DEFAULT 'unknown',
    previous_event_hash TEXT NOT NULL DEFAULT '',
    event_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(adapter_id, source_event_id)
);

CREATE TABLE IF NOT EXISTS usage_allocations (
    id TEXT PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    project_id TEXT REFERENCES projects(id),
    allocation_basis_points INTEGER NOT NULL,
    allocation_method TEXT NOT NULL,
    confidence TEXT NOT NULL,
    evidence_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    CHECK(allocation_basis_points >= 0 AND allocation_basis_points <= 10000)
);

CREATE INDEX IF NOT EXISTS idx_raw_events_occurred_at ON raw_events(occurred_at);
CREATE INDEX IF NOT EXISTS idx_raw_events_project_id ON raw_events(project_id);
CREATE INDEX IF NOT EXISTS idx_raw_events_work_context_id ON raw_events(work_context_id);
CREATE INDEX IF NOT EXISTS idx_raw_events_session_id ON raw_events(session_id);
CREATE INDEX IF NOT EXISTS idx_work_contexts_session_id ON work_contexts(session_id);
