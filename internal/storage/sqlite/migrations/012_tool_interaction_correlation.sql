-- Keep the source-native interaction identity on tools for deterministic
-- out-of-order and cross-transport root correlation.
ALTER TABLE tool_calls ADD COLUMN interaction_upstream_id TEXT NOT NULL DEFAULT '';
UPDATE tool_calls
SET interaction_upstream_id = COALESCE((
    SELECT json_extract(r.payload_json_sanitized, '$.interaction_upstream_id')
    FROM raw_events r WHERE r.id = tool_calls.raw_event_id
), '')
WHERE interaction_upstream_id = '' AND raw_event_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tool_calls_interaction_correlation
    ON tool_calls(session_id, interaction_upstream_id)
    WHERE interaction_id IS NULL AND interaction_upstream_id <> '';

-- Link already-migrated children only when the cross-source identity is unique.
UPDATE tool_calls
SET interaction_id = (
    SELECT MIN(i.id) FROM interactions i
    WHERE i.session_id = tool_calls.session_id
      AND i.upstream_id = tool_calls.interaction_upstream_id
)
WHERE interaction_id IS NULL
  AND interaction_upstream_id <> ''
  AND 1 = (SELECT COUNT(*) FROM interactions i
           WHERE i.session_id = tool_calls.session_id
             AND i.upstream_id = tool_calls.interaction_upstream_id);
