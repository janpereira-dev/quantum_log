CREATE TABLE IF NOT EXISTS allocation_revisions (
    revision_id TEXT NOT NULL,
    entry_id TEXT PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    revision_number INTEGER NOT NULL,
    parent_revision_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    project_id TEXT REFERENCES projects(id),
    allocation_basis_points INTEGER NOT NULL,
    allocation_method TEXT NOT NULL,
    confidence TEXT NOT NULL,
    author TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    CHECK(allocation_basis_points >= 0 AND allocation_basis_points <= 10000),
    UNIQUE(subject_type, subject_id, idempotency_key, entry_id)
);

CREATE INDEX IF NOT EXISTS idx_allocation_revisions_subject
    ON allocation_revisions(subject_type, subject_id, revision_number, entry_id);

-- Preserve the pre-014 projection as the first immutable revision for each subject.
INSERT INTO allocation_revisions (
    revision_id, entry_id, subject_type, subject_id, revision_number,
    idempotency_key, project_id, allocation_basis_points, allocation_method,
    confidence, author, source, reason, created_at
)
SELECT
    'legacy:' || a.subject_type || ':' || a.subject_id,
    'legacy-entry:' || a.id,
    a.subject_type, a.subject_id, 1,
    'legacy:' || a.id,
    a.project_id, a.allocation_basis_points, a.allocation_method,
    a.confidence, 'migration', 'legacy_projection', 'pre-014 allocation', a.created_at
FROM usage_allocations a
WHERE NOT EXISTS (
    SELECT 1 FROM allocation_revisions r
    WHERE r.subject_type = a.subject_type AND r.subject_id = a.subject_id
);
