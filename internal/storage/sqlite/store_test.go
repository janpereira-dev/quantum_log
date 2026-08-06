package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenMigratesAndConfiguresDatabase(t *testing.T) {
	t.Parallel()

	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var foreignKeys int
	if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	for _, table := range []string{"hosts", "projects", "project_locations", "work_contexts", "raw_events", "usage_allocations", "project_tags", "tasks", "sessions", "turns", "model_calls", "tool_calls", "pricing_rules", "cost_snapshots", "budgets"} {
		var name string
		if err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name); err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}
}

func TestProjectLocationMatchesNormalizedResolvedPath(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, _, err := store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "first"))
	if err != nil {
		t.Fatalf("RegisterProject(first) error = %v", err)
	}
	_, expected, err := store.RegisterProject(ctx, "Project", project.Slug, filepath.Join(t.TempDir(), "second"))
	if err != nil {
		t.Fatalf("RegisterProject(second) error = %v", err)
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin lookup: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	resolvedPath := strings.ToLower(filepath.ToSlash(expected.AbsolutePath))
	gotProject, gotLocation, found, err := projectByLocation(ctx, tx, resolvedPath)
	if err != nil {
		t.Fatalf("projectByLocation() error = %v", err)
	}
	if !found || gotProject.ID != project.ID || gotLocation.ID != expected.ID {
		t.Fatalf("projectByLocation(%q) = project=%#v location=%#v found=%t, want project=%q location=%q", resolvedPath, gotProject, gotLocation, found, project.ID, expected.ID)
	}
}

func TestTaskAndUnattributedSummariesAndBudgets(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, _, err := store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatalf("RegisterProject() error = %v", err)
	}
	if err := store.AddProjectTag(ctx, project.ID, "team", "core"); err != nil {
		t.Fatalf("AddProjectTag() error = %v", err)
	}
	taskID, err := store.StartTask(ctx, TaskInput{ProjectID: project.ID, Title: "Agent work", TaskType: "build"})
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	now := time.Now().UTC()
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, TaskID: taskID, Provider: "provider", ModelID: "model", InputTokens: 100, EstimatedCostUSDMicros: 1_000, OccurredAt: now}); err != nil {
		t.Fatalf("RecordModelCall(task) error = %v", err)
	}
	unattributedID, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "provider", ModelID: "model", InputTokens: 30, EstimatedCostUSDMicros: 500, OccurredAt: now})
	if err != nil {
		t.Fatalf("RecordModelCall(unattributed) error = %v", err)
	}

	task, err := store.TaskSummary(ctx, taskID)
	if err != nil || task.ModelCallCount != 1 || task.ObservedTokens != 100 || task.AllocatedCostUSDMicros != 1_000 {
		t.Fatalf("TaskSummary() = %#v, %v", task, err)
	}
	unattributed, err := store.UnattributedSummary(ctx)
	if err != nil || unattributed.ModelCallCount != 1 || unattributed.ModelCalls[0].ID != unattributedID {
		t.Fatalf("UnattributedSummary() = %#v, %v", unattributed, err)
	}
	if err := store.RepairModelCallAllocation(ctx, unattributedID, project.ID); err != nil {
		t.Fatalf("RepairModelCallAllocation() error = %v", err)
	}
	if unattributed, err = store.UnattributedSummary(ctx); err != nil || unattributed.ModelCallCount != 0 {
		t.Fatalf("UnattributedSummary() after repair = %#v, %v", unattributed, err)
	}

	if _, err := store.SetBudget(ctx, BudgetInput{Scope: "project", Target: project.ID, MonthlyCostUSDMicros: 1_200, AlertPercent: 80}); err != nil {
		t.Fatalf("SetBudget(project) error = %v", err)
	}
	if _, err := store.SetBudget(ctx, BudgetInput{Scope: "tag", Target: "team=core", MonthlyCostUSDMicros: 1_800, AlertPercent: 80}); err != nil {
		t.Fatalf("SetBudget(tag) error = %v", err)
	}
	alerts, err := store.BudgetAlerts(ctx, now)
	if err != nil || len(alerts) != 2 || alerts[0].Alert != "exceeded" || alerts[1].Alert != "warning" {
		t.Fatalf("BudgetAlerts() = %#v, %v", alerts, err)
	}
	report, err := store.ProjectReport(ctx, "project", now)
	if err != nil || report.ObservedTokens != 100 || report.AllocatedCostUSDMicros != 1_500 || len(report.BudgetAlerts) != 2 {
		t.Fatalf("ProjectReport() = %#v, %v", report, err)
	}
}

func TestSetBudgetNormalizesTagTarget(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	budget, err := store.SetBudget(ctx, BudgetInput{Scope: "tag", Target: " Team = Core ", MonthlyCostUSDMicros: 1_000})
	if err != nil {
		t.Fatalf("SetBudget() error = %v", err)
	}
	if budget.Target != "team=core" {
		t.Fatalf("budget target = %q, want team=core", budget.Target)
	}
}

func TestBudgetAlertsExcludeFutureMonthUsage(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, _, err := store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatalf("RegisterProject() error = %v", err)
	}
	month := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, Provider: "provider", ModelID: "model", EstimatedCostUSDMicros: 1_000, OccurredAt: month}); err != nil {
		t.Fatalf("RecordModelCall(current month) error = %v", err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, Provider: "provider", ModelID: "model", EstimatedCostUSDMicros: 10_000, OccurredAt: month.AddDate(0, 1, 0)}); err != nil {
		t.Fatalf("RecordModelCall(next month) error = %v", err)
	}
	if _, err := store.SetBudget(ctx, BudgetInput{Scope: "project", Target: project.ID, MonthlyCostUSDMicros: 2_000}); err != nil {
		t.Fatalf("SetBudget() error = %v", err)
	}

	alerts, err := store.BudgetAlerts(ctx, month)
	if err != nil {
		t.Fatalf("BudgetAlerts() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].AllocatedCostUSDMicros != 1_000 {
		t.Fatalf("BudgetAlerts() = %#v, want only current-month usage", alerts)
	}
}

func TestProjectReportIncludesMatchingTagBudget(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, _, err := store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatalf("RegisterProject() error = %v", err)
	}
	if err := store.AddProjectTag(ctx, project.ID, "team", "core"); err != nil {
		t.Fatalf("AddProjectTag() error = %v", err)
	}
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, Provider: "provider", ModelID: "model", EstimatedCostUSDMicros: 1_000, OccurredAt: now}); err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	if _, err := store.SetBudget(ctx, BudgetInput{Scope: "tag", Target: "team=core", MonthlyCostUSDMicros: 2_000}); err != nil {
		t.Fatalf("SetBudget() error = %v", err)
	}

	report, err := store.ProjectReport(ctx, "project", now)
	if err != nil {
		t.Fatalf("ProjectReport() error = %v", err)
	}
	if len(report.BudgetAlerts) != 1 || report.BudgetAlerts[0].Scope != "tag" || report.BudgetAlerts[0].Target != "team=core" {
		t.Fatalf("ProjectReport() budget alerts = %#v, want matching tag budget", report.BudgetAlerts)
	}
}

func TestMigrationNormalizesExistingTagBudgetTargets(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(ctx, database)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO budgets (id, scope, target, monthly_cost_usd_micros, alert_percent, created_at, updated_at) VALUES (?, 'tag', 'team = core', 1000, 80, ?, ?)`, "legacy", timestamp(time.Now()), timestamp(time.Now())); err != nil {
		t.Fatalf("insert legacy budget: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO budgets (id, scope, target, monthly_cost_usd_micros, alert_percent, created_at, updated_at) VALUES (?, 'tag', 'team=core', 1000, 80, ?, ?)`, "canonical", timestamp(time.Now()), timestamp(time.Now())); err != nil {
		t.Fatalf("insert canonical budget: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO budgets (id, scope, target, monthly_cost_usd_micros, alert_percent, created_at, updated_at) VALUES (?, 'tag', ' Team=Core ', 1000, 80, ?, ?)`, "variant", timestamp(time.Now()), timestamp(time.Now())); err != nil {
		t.Fatalf("insert variant budget: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = '005_normalize_budget_tags.sql'`); err != nil {
		t.Fatalf("reset migration: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, database)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var count int
	var target string
	if err := reopened.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(target) FROM budgets WHERE scope = 'tag'`).Scan(&count, &target); err != nil {
		t.Fatalf("read migrated budget: %v", err)
	}
	if count != 1 || target != "team=core" {
		t.Fatalf("migrated budgets = %d %q, want one team=core budget", count, target)
	}
}

func TestMigrationDeduplicatesLegacyTagBudgetVariants(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(ctx, database)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for _, budget := range []struct {
		id     string
		target string
	}{{"first", "team = core"}, {"second", " Team=Core "}} {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO budgets (id, scope, target, monthly_cost_usd_micros, alert_percent, created_at, updated_at) VALUES (?, 'tag', ?, 1000, 80, ?, ?)`, budget.id, budget.target, timestamp(time.Now()), timestamp(time.Now())); err != nil {
			t.Fatalf("insert legacy budget: %v", err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = '005_normalize_budget_tags.sql'`); err != nil {
		t.Fatalf("reset migration: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, database)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var count int
	var target string
	if err := reopened.db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(target) FROM budgets WHERE scope = 'tag'`).Scan(&count, &target); err != nil {
		t.Fatalf("read migrated budgets: %v", err)
	}
	if count != 1 || target != "team=core" {
		t.Fatalf("migrated budgets = %d %q, want one team=core budget", count, target)
	}
}

func TestAssignUnattributedModelCallRejectsAlreadyAllocatedCalls(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	first, _, err := store.RegisterProject(ctx, "First", "first", filepath.Join(t.TempDir(), "first"))
	if err != nil {
		t.Fatalf("RegisterProject(first) error = %v", err)
	}
	second, _, err := store.RegisterProject(ctx, "Second", "second", filepath.Join(t.TempDir(), "second"))
	if err != nil {
		t.Fatalf("RegisterProject(second) error = %v", err)
	}
	callID, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "provider", ModelID: "model"})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	if err := store.AssignUnattributedModelCall(ctx, callID, first.ID); err != nil {
		t.Fatalf("first AssignUnattributedModelCall() error = %v", err)
	}
	if err := store.AssignUnattributedModelCall(ctx, callID, second.ID); err == nil {
		t.Fatal("second AssignUnattributedModelCall() accepted an already allocated model call")
	}
	allocations, err := store.ModelCallAllocations(ctx, callID)
	if err != nil {
		t.Fatalf("ModelCallAllocations() error = %v", err)
	}
	if len(allocations) != 1 || allocations[0].ProjectID != first.ID || allocations[0].Method != "manual" {
		t.Fatalf("allocations after rejected repair = %#v", allocations)
	}
}

func TestProjectRegistrationAndContextsStaySeparated(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	projectA, locationA, err := store.RegisterProject(ctx, "Project A", "project-a", "C:/repos/a")
	if err != nil {
		t.Fatalf("RegisterProject(A) error = %v", err)
	}
	projectAAgain, locationAAgain, err := store.RegisterProject(ctx, "Project A", "project-a", "C:/repos/a")
	if err != nil {
		t.Fatalf("RegisterProject(A again) error = %v", err)
	}
	if projectA.ID != projectAAgain.ID || locationA.ID != locationAAgain.ID {
		t.Fatal("RegisterProject() is not idempotent")
	}
	projectB, locationB, err := store.RegisterProject(ctx, "Project B", "project-b", "C:/repos/b")
	if err != nil {
		t.Fatalf("RegisterProject(B) error = %v", err)
	}

	first, err := store.CreateWorkContext(ctx, WorkContextInput{ProjectID: projectA.ID, LocationID: locationA.ID, SessionID: "session-1", CWD: "C:/repos/a", StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("CreateWorkContext(A first) error = %v", err)
	}
	second, err := store.CreateWorkContext(ctx, WorkContextInput{ProjectID: projectB.ID, LocationID: locationB.ID, SessionID: "session-1", CWD: "C:/repos/b", StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("CreateWorkContext(B) error = %v", err)
	}
	third, err := store.CreateWorkContext(ctx, WorkContextInput{ProjectID: projectA.ID, LocationID: locationA.ID, SessionID: "session-1", CWD: "C:/repos/a", StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("CreateWorkContext(A return) error = %v", err)
	}
	if first.PrimaryProjectID != projectA.ID || second.PrimaryProjectID != projectB.ID || third.PrimaryProjectID != projectA.ID {
		t.Fatalf("work contexts lost project transitions: %#v %#v %#v", first, second, third)
	}
}

func TestLedgerDetectsStoredEventTampering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.AppendRawEvent(ctx, RawEventInput{Source: "fixture", SessionID: "session-1", EventType: "model.call", Payload: []byte(`{"tokens":10}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("AppendRawEvent() first error = %v", err)
	}
	if _, err := store.AppendRawEvent(ctx, RawEventInput{Source: "fixture", SessionID: "session-1", EventType: "model.call", Payload: []byte(`{"tokens":20}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("AppendRawEvent() second error = %v", err)
	}
	if err := store.VerifyLedger(ctx, ""); err != nil {
		t.Fatalf("VerifyLedger() error = %v", err)
	}
	if _, err := store.db.ExecContext(ctx, "UPDATE raw_events SET payload_json_sanitized = '{}' WHERE id = (SELECT id FROM raw_events ORDER BY occurred_at DESC LIMIT 1)"); err != nil {
		t.Fatalf("tamper raw event: %v", err)
	}
	if err := store.VerifyLedger(ctx, ""); err == nil {
		t.Fatal("VerifyLedger() did not detect a tampered event")
	}
}

func TestAppendRawEventSuppressesReplayWithoutChangingLedger(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := RawEventInput{
		Source:     "opencode-plugin",
		SessionID:  "session-1",
		EventType:  "model.call",
		OccurredAt: time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC),
		Payload:    []byte(`{"provider":"example","model":"model","input_tokens":2}`),
	}
	first, err := store.AppendRawEvent(ctx, input)
	if err != nil || !first.Accepted {
		t.Fatalf("first append = %#v, %v", first, err)
	}
	second, err := store.AppendRawEvent(ctx, input)
	if err != nil || second.Accepted || second.ID != first.ID || second.SuppressionReason != "duplicate_ingestion_identity" {
		t.Fatalf("second append = %#v, %v", second, err)
	}
	if err := store.VerifyLedger(ctx, input.SessionID); err != nil {
		t.Fatalf("VerifyLedger() error = %v", err)
	}
	assertTableCount(t, store, "raw_events", 1)
	assertTableCount(t, store, "raw_event_dedup", 1)
}

func TestOpenBackfillsReconstructableIngestionIdentitiesForReplay(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(ctx, database)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	input := RawEventInput{Source: "fixture", SessionID: "session-1", EventType: "model.call", OccurredAt: time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC), Payload: []byte(`{"provider":"example","model":"model"}`)}
	if _, err := store.AppendRawEvent(ctx, input); err != nil {
		t.Fatalf("AppendRawEvent() error = %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM raw_event_dedup`); err != nil {
		t.Fatalf("simulate pre-upgrade dedup state: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	store, err = Open(ctx, database)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	result, err := store.AppendRawEvent(ctx, input)
	if err != nil || result.Accepted || result.SuppressionReason != "duplicate_ingestion_identity" {
		t.Fatalf("replayed append = %#v, %v", result, err)
	}
	assertTableCount(t, store, "raw_events", 1)
}

func TestDistinctEventsWithSharedSessionAndTimeAreAccepted(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := RawEventInput{
		Source:     "opencode-plugin",
		SessionID:  "session-1",
		EventType:  "model.call",
		OccurredAt: time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC),
		Payload:    []byte(`{"provider":"example","model":"model","turn_id":"first"}`),
	}
	if result, err := store.AppendRawEvent(ctx, input); err != nil || !result.Accepted {
		t.Fatalf("first append = %#v, %v", result, err)
	}
	input.Payload = []byte(`{"provider":"example","model":"model","turn_id":"second"}`)
	if result, err := store.AppendRawEvent(ctx, input); err != nil || !result.Accepted {
		t.Fatalf("second append = %#v, %v", result, err)
	}
	assertTableCount(t, store, "raw_events", 2)
}

func TestRecordModelCallLinksToAcceptedRawEvent(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	raw, err := store.AppendRawEvent(ctx, RawEventInput{Source: "fixture", SessionID: "session-1", EventType: "model.call", Payload: []byte(`{"provider":"example","model":"model"}`), OccurredAt: time.Now().UTC()})
	if err != nil || !raw.Accepted {
		t.Fatalf("AppendRawEvent() = %#v, %v", raw, err)
	}
	if err := store.EnsureSession(ctx, "session-1", "fixture", time.Now().UTC()); err != nil {
		t.Fatalf("EnsureSession() error = %v", err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{RawEventID: raw.ID, SessionID: "session-1", Provider: "example", ModelID: "model"}); err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	var linkedRawEventID string
	if err := store.db.QueryRowContext(ctx, `SELECT raw_event_id FROM model_calls`).Scan(&linkedRawEventID); err != nil {
		t.Fatalf("query model call linkage: %v", err)
	}
	if linkedRawEventID != raw.ID {
		t.Fatalf("raw event linkage = %q, want %q", linkedRawEventID, raw.ID)
	}
}

func TestAdapterEvidenceRequiresLinkedNormalizedModelCall(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Now().UTC()
	matchingRaw, err := store.AppendRawEvent(ctx, RawEventInput{
		Source:     "opencode-plugin",
		SessionID:  "session-1",
		EventType:  "model.call",
		OccurredAt: now,
		Payload:    []byte(`{"agent_name":"opencode","capture_quality":"agent_reported"}`),
	})
	if err != nil || !matchingRaw.Accepted {
		t.Fatalf("AppendRawEvent(matching) = %#v, %v", matchingRaw, err)
	}
	otherRaw, err := store.AppendRawEvent(ctx, RawEventInput{
		Source:     "fixture",
		SessionID:  "session-1",
		EventType:  "model.call",
		OccurredAt: now,
		Payload:    []byte(`{"agent_name":"fixture","capture_quality":"agent_reported"}`),
	})
	if err != nil || !otherRaw.Accepted {
		t.Fatalf("AppendRawEvent(other) = %#v, %v", otherRaw, err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{
		RawEventID:     otherRaw.ID,
		Provider:       "example",
		ModelID:        "model",
		InputTokens:    1,
		CaptureQuality: "agent_reported",
		OccurredAt:     now,
	}); err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}

	found, err := store.HasRecentAdapterEvidence(ctx, AdapterEvidenceQuery{
		AdapterID:       "opencode",
		Source:          "opencode-plugin",
		From:            now.Add(-time.Minute),
		To:              now.Add(time.Minute),
		RequiredQuality: "agent_reported",
	})
	if err != nil {
		t.Fatalf("HasRecentAdapterEvidence() error = %v", err)
	}
	if found {
		t.Fatal("matching raw event without linked model call satisfied adapter evidence")
	}
}

func TestAdapterEvidenceUsesModelCallAllocationForReportedTokens(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	for _, test := range []struct {
		name    string
		payload string
		want    bool
	}{
		{name: "normalized github provider", payload: `{"agent_name":"GitHub Copilot Chat","provider":"github","capture_quality":"otel_reported"}`, want: true},
		{name: "provider omitted", payload: `{"agent_name":"GitHub Copilot Chat","capture_quality":"otel_reported"}`, want: false},
		{name: "other provider", payload: `{"agent_name":"GitHub Copilot Chat","provider":"openai","capture_quality":"otel_reported"}`, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			t.Cleanup(func() { _ = store.Close() })

			project, _, err := store.RegisterProject(ctx, "Project", "project", t.TempDir())
			if err != nil {
				t.Fatalf("RegisterProject() error = %v", err)
			}
			if err := store.EnsureSession(ctx, test.name, "GitHub Copilot Chat", now); err != nil {
				t.Fatalf("EnsureSession() error = %v", err)
			}
			raw, err := store.AppendRawEvent(ctx, RawEventInput{
				Source:     "otlp-http",
				SessionID:  test.name,
				EventType:  "model.call",
				OccurredAt: now,
				Payload:    []byte(test.payload),
			})
			if err != nil || !raw.Accepted {
				t.Fatalf("AppendRawEvent() = %#v, %v", raw, err)
			}
			if _, err := store.RecordModelCall(ctx, ModelCallInput{
				RawEventID:     raw.ID,
				ProjectID:      project.ID,
				SessionID:      test.name,
				AgentName:      "GitHub Copilot Chat",
				Provider:       "github",
				ModelID:        "gpt-5",
				InputTokens:    1,
				CaptureQuality: "otel_reported",
				OccurredAt:     now,
			}); err != nil {
				t.Fatalf("RecordModelCall() error = %v", err)
			}

			found, err := store.HasRecentAdapterEvidence(ctx, AdapterEvidenceQuery{
				AdapterID:         "copilot-vscode",
				AllowedAgentNames: []string{"GitHub Copilot Chat"},
				Source:            "otlp-http",
				From:              now.Add(-time.Minute),
				To:                now.Add(time.Minute),
				ProjectSlug:       project.Slug,
				RequiredQuality:   "otel_reported",
				RequiredProvider:  "github",
			})
			if err != nil || found != test.want {
				t.Fatalf("HasRecentAdapterEvidence() = %t, %v; want %t", found, err, test.want)
			}
		})
	}
}

func TestAdapterEvidenceRequiresCodexResponseCompleted(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, _, err := store.RegisterProject(ctx, "Project", "project", t.TempDir())
	if err != nil {
		t.Fatalf("RegisterProject() error = %v", err)
	}
	now := time.Now().UTC()
	for _, test := range []struct {
		name    string
		payload string
		want    bool
	}{
		{name: "missing completion discriminator", payload: `{"agent_name":"codex","capture_quality":"otel_reported"}`, want: false},
		{name: "persisted completion discriminator", payload: `{"agent_name":"codex","capture_quality":"otel_reported","codex_response_completed":true}`, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := store.EnsureSession(ctx, test.name, "codex", now); err != nil {
				t.Fatalf("EnsureSession() error = %v", err)
			}
			raw, err := store.AppendRawEvent(ctx, RawEventInput{
				Source:     "otlp-http",
				SessionID:  test.name,
				EventType:  "model.call",
				OccurredAt: now,
				Payload:    []byte(test.payload),
			})
			if err != nil || !raw.Accepted {
				t.Fatalf("AppendRawEvent() = %#v, %v", raw, err)
			}
			if _, err := store.RecordModelCall(ctx, ModelCallInput{
				RawEventID:     raw.ID,
				ProjectID:      project.ID,
				SessionID:      test.name,
				AgentName:      "codex",
				Provider:       "openai",
				ModelID:        "gpt-5",
				InputTokens:    1,
				CaptureQuality: "otel_reported",
				OccurredAt:     now,
			}); err != nil {
				t.Fatalf("RecordModelCall() error = %v", err)
			}

			found, err := store.HasRecentAdapterEvidence(ctx, AdapterEvidenceQuery{
				AdapterID:                     "codex",
				Source:                        "otlp-http",
				From:                          now.Add(-time.Minute),
				To:                            now.Add(time.Minute),
				ProjectSlug:                   project.Slug,
				RequiredQuality:               "otel_reported",
				RequireCodexResponseCompleted: true,
			})
			if err != nil {
				t.Fatalf("HasRecentAdapterEvidence() error = %v", err)
			}
			if found != test.want {
				t.Fatalf("HasRecentAdapterEvidence() = %t, want %t", found, test.want)
			}
		})
	}
}

func TestAdapterEvidenceAcceptsClaudeLifecycleRawEventWithoutModelCall(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Now().UTC()
	result, err := store.AppendRawEvent(ctx, RawEventInput{
		Source:     "claude-code-hook",
		SessionID:  "session-1",
		EventType:  "lifecycle.stop",
		OccurredAt: now,
		Payload:    []byte(`{"agent_name":"claude-code","capture_quality":"lifecycle_only"}`),
	})
	if err != nil || !result.Accepted {
		t.Fatalf("AppendRawEvent() = %#v, %v", result, err)
	}

	found, err := store.HasRecentAdapterEvidence(ctx, AdapterEvidenceQuery{
		AdapterID:       "claude-code",
		Source:          "claude-code-hook",
		From:            now.Add(-time.Minute),
		To:              now.Add(time.Minute),
		RequiredQuality: "lifecycle_only",
	})
	if err != nil {
		t.Fatalf("HasRecentAdapterEvidence() error = %v", err)
	}
	if !found {
		t.Fatal("Claude lifecycle raw event was not accepted as adapter evidence")
	}
}

func TestCanonicalIngestionIdentityDoesNotPersistUpstreamValue(t *testing.T) {
	identity, err := CanonicalIngestionIdentity(RawEventInput{Source: "fixture", IngestionIdentity: "event-secret-value"}, []byte(`{}`))
	if err != nil {
		t.Fatalf("CanonicalIngestionIdentity() error = %v", err)
	}
	if identity == "" || strings.Contains(identity, "event-secret-value") {
		t.Fatalf("ingestion identity = %q", identity)
	}
}

func TestModelCallMetricObservationsPreserveReportedZeroAndOmission(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	zero := int64(0)
	callID, err := store.RecordModelCall(ctx, ModelCallInput{
		Provider: "anthropic",
		ModelID:  "claude-sonnet",
		Metrics: []MetricInput{
			{Name: "input_tokens", Value: &zero, Source: "otel", RawKey: "input_tokens", Confidence: "reported"},
		},
	})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	rows, err := store.db.QueryContext(ctx, `SELECT metric_name, metric_value, source, raw_key, confidence FROM model_call_metrics WHERE model_call_id = ?`, callID)
	if err != nil {
		t.Fatalf("query metric observations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("reported zero metric was not persisted")
	}
	var name, source, rawKey, confidence string
	var value int64
	if err := rows.Scan(&name, &value, &source, &rawKey, &confidence); err != nil {
		t.Fatalf("scan metric observation: %v", err)
	}
	if name != "input_tokens" || value != 0 || source != "otel" || rawKey != "input_tokens" || confidence != "reported" {
		t.Fatalf("metric observation = %q %d %q %q %q", name, value, source, rawKey, confidence)
	}
	if rows.Next() {
		t.Fatal("omitted metrics must not be persisted as zero observations")
	}
}

func TestVerifiedGitContextRequiresOneExactRootAndRemoteMatch(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	root := filepath.Join(t.TempDir(), "repo")
	first, firstLocation, err := store.RegisterProject(ctx, "First", "first", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetVerifiedGitContext(ctx, first.ID, firstLocation.ID, root, "https://github.com/example/repo.git"); err != nil {
		t.Fatal(err)
	}
	project, location, found, err := store.ProjectByVerifiedGitContext(ctx, root, "git@github.com:example/repo.git")
	if err != nil || !found || project.ID != first.ID || location.ID != firstLocation.ID {
		t.Fatalf("exact context = %#v %#v found=%t err=%v", project, location, found, err)
	}
	if _, _, found, err := store.ProjectByVerifiedGitContext(ctx, root, "https://github.com/example/other.git"); err != nil || found {
		t.Fatalf("remote mismatch found=%t err=%v", found, err)
	}
	second, secondLocation, err := store.RegisterProject(ctx, "Second", "second", filepath.Join(t.TempDir(), "other"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetVerifiedGitContext(ctx, second.ID, secondLocation.ID, root, "https://github.com/example/repo.git"); err != nil {
		t.Fatal(err)
	}
	if _, _, found, err := store.ProjectByVerifiedGitContext(ctx, root, "https://github.com/example/repo.git"); err != nil || found {
		t.Fatalf("collision found=%t err=%v", found, err)
	}
}

func assertTableCount(t *testing.T, store *Store, table string, want int) {
	t.Helper()
	var got int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func TestValidateAllocationRejectsInvalidTotal(t *testing.T) {
	t.Parallel()

	err := ValidateAllocations([]AllocationInput{{ProjectID: "a", BasisPoints: 6000}, {ProjectID: "b", BasisPoints: 5000}})
	if err == nil {
		t.Fatal("ValidateAllocations() accepted 11000 basis points")
	}
}
