CREATE TABLE IF NOT EXISTS model_call_metrics (
    model_call_id TEXT NOT NULL REFERENCES model_calls(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL CHECK(metric_name IN ('input_tokens', 'output_tokens', 'reasoning_tokens', 'cached_input_tokens', 'cache_write_tokens', 'total_tokens')),
    metric_value INTEGER NOT NULL CHECK(metric_value >= 0),
    source TEXT NOT NULL,
    raw_key TEXT NOT NULL,
    confidence TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (model_call_id, metric_name)
);

CREATE INDEX IF NOT EXISTS idx_model_call_metrics_name ON model_call_metrics(metric_name);
