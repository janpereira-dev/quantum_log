package sqlite

import (
	"context"
	"path/filepath"
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

func TestValidateAllocationRejectsInvalidTotal(t *testing.T) {
	t.Parallel()

	err := ValidateAllocations([]AllocationInput{{ProjectID: "a", BasisPoints: 6000}, {ProjectID: "b", BasisPoints: 5000}})
	if err == nil {
		t.Fatal("ValidateAllocations() accepted 11000 basis points")
	}
}
