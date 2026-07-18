CREATE TABLE IF NOT EXISTS budgets (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL CHECK(scope IN ('project', 'tag')),
    target TEXT NOT NULL,
    monthly_cost_usd_micros INTEGER NOT NULL CHECK(monthly_cost_usd_micros > 0),
    alert_percent INTEGER NOT NULL DEFAULT 80 CHECK(alert_percent BETWEEN 1 AND 100),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(scope, target)
);

CREATE INDEX IF NOT EXISTS idx_budgets_scope_target ON budgets(scope, target);
