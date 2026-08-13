-- Model-call rows remain immutable source evidence. This flag excludes only a
-- verified aggregate duplicate from consumption totals when a child reports
-- the exact same source-native usage in the same trace.
ALTER TABLE model_calls ADD COLUMN usage_reconciled INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_raw_events_trace_span
    ON raw_events(trace_id, span_id);
CREATE INDEX IF NOT EXISTS idx_raw_events_trace_parent_span
    ON raw_events(trace_id, parent_span_id);
CREATE INDEX IF NOT EXISTS idx_model_calls_usage_reconciled
    ON model_calls(usage_reconciled, raw_event_id);
