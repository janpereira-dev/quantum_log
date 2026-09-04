ALTER TABLE allocation_revisions ADD COLUMN previous_revision_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE allocation_revisions ADD COLUMN revision_hash TEXT NOT NULL DEFAULT '';
