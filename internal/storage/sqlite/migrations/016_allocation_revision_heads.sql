CREATE TABLE IF NOT EXISTS allocation_revision_heads (
    subject_type TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    revision_hash TEXT NOT NULL,
    PRIMARY KEY(subject_type, subject_id)
);
