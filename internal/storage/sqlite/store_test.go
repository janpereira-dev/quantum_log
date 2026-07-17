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
	if err != nil || report.ObservedTokens != 100 || report.AllocatedCostUSDMicros != 1_500 || len(report.BudgetAlerts) != 1 {
		t.Fatalf("ProjectReport() = %#v, %v", report, err)
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
