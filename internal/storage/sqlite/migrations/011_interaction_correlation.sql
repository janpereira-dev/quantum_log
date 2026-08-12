-- Preserve the source-native parent identity on children so that a prompt root
-- arriving after its OTel children can backfill their canonical interaction.
ALTER TABLE model_calls ADD COLUMN interaction_upstream_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_model_calls_interaction_correlation
    ON model_calls(session_id, interaction_upstream_id)
    WHERE interaction_id IS NULL AND interaction_upstream_id <> '';
