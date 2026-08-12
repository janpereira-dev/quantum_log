-- Keep the source-native interaction identity on tools for deterministic
-- out-of-order and cross-transport root correlation.
ALTER TABLE tool_calls ADD COLUMN interaction_upstream_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_tool_calls_interaction_correlation
    ON tool_calls(session_id, interaction_upstream_id)
    WHERE interaction_id IS NULL AND interaction_upstream_id <> '';
