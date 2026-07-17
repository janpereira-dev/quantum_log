package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/pricing"
)

func TestUsageGroupingPreservesTotalsAndAllocation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	projectA, _, err := store.RegisterProject(ctx, "Project A", "project-a", filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatalf("RegisterProject(A) error = %v", err)
	}
	projectB, _, err := store.RegisterProject(ctx, "Project B", "project-b", filepath.Join(t.TempDir(), "b"))
	if err != nil {
		t.Fatalf("RegisterProject(B) error = %v", err)
	}
	if err := store.AddProjectTag(ctx, projectA.ID, "environment", "work"); err != nil {
		t.Fatalf("AddProjectTag() error = %v", err)
	}

	callID, err := store.RecordModelCall(ctx, ModelCallInput{
		ProjectID:              projectA.ID,
		Provider:               "example-provider",
		ModelID:                "example-model",
		InputTokens:            100,
		OutputTokens:           50,
		EstimatedCostUSDMicros: 1_000_000,
		OccurredAt:             time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	if err := store.ReplaceAllocations(ctx, "model_call", callID, []AllocationInput{{ProjectID: projectA.ID, BasisPoints: 6000}, {ProjectID: projectB.ID, BasisPoints: 4000}}); err != nil {
		t.Fatalf("ReplaceAllocations() error = %v", err)
	}

	first, err := store.Usage(ctx, UsageQuery{From: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), GroupBy: []string{"project", "provider", "model"}})
	if err != nil {
		t.Fatalf("Usage(project,provider,model) error = %v", err)
	}
	second, err := store.Usage(ctx, UsageQuery{From: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), GroupBy: []string{"provider", "model", "project"}})
	if err != nil {
		t.Fatalf("Usage(provider,model,project) error = %v", err)
	}
	if first.TotalTokens != 150 || second.TotalTokens != 150 {
		t.Fatalf("usage totals = %d and %d, want 150", first.TotalTokens, second.TotalTokens)
	}
	if first.AllocatedCostUSDMicros != 1_000_000 || second.AllocatedCostUSDMicros != 1_000_000 {
		t.Fatalf("allocated totals = %d and %d, want 1000000", first.AllocatedCostUSDMicros, second.AllocatedCostUSDMicros)
	}
	if len(first.Rows) != 2 || len(second.Rows) != 2 {
		t.Fatalf("usage rows = %d and %d, want 2", len(first.Rows), len(second.Rows))
	}
}

func TestReplaceAllocationsRejectsInvalidSplit(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.ReplaceAllocations(context.Background(), "model_call", "missing", []AllocationInput{{ProjectID: "a", BasisPoints: 6000}, {ProjectID: "b", BasisPoints: 5000}}); err == nil {
		t.Fatal("ReplaceAllocations() accepted 11000 basis points")
	}
}

func TestTasksProjectsPricingAndAllocationsPersist(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	project, _, err := store.RegisterProject(ctx, "Project A", "project-a", filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatalf("RegisterProject() error = %v", err)
	}
	if err := store.AddProjectTag(ctx, project.ID, "Cost-Center", "Research"); err != nil {
		t.Fatalf("AddProjectTag() error = %v", err)
	}
	if projects, err := store.ListProjects(ctx); err != nil || len(projects) != 1 || projects[0].Slug != "project-a" {
		t.Fatalf("ListProjects() = %#v, %v", projects, err)
	}
	if tags, err := store.ProjectTags(ctx, project.ID); err != nil || len(tags) != 1 || tags[0].Key != "cost-center" || tags[0].Value != "research" {
		t.Fatalf("ProjectTags() = %#v, %v", tags, err)
	}

	taskID, err := store.StartTask(ctx, TaskInput{ProjectID: project.ID, Title: "Implement reporting", TaskType: "build"})
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if err := store.FinishTask(ctx, taskID, "success"); err != nil {
		t.Fatalf("FinishTask() error = %v", err)
	}
	if tasks, err := store.ListTasks(ctx, "project-a"); err != nil || len(tasks) != 1 || tasks[0].Status != "finished" || tasks[0].Result != "success" {
		t.Fatalf("ListTasks() = %#v, %v", tasks, err)
	}

	callID, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, Provider: "example", ModelID: "model", InputTokens: 1_000_000, OccurredAt: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	rule := pricing.Rule{SchemaVersion: 1, Provider: "example", ModelPattern: "model", ValidFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), BillingMode: "token", Currency: "USD", UnitTokens: 1_000_000, Prices: pricing.Prices{InputMicros: 3_000_000}, Version: "2026.07.1"}
	if _, err := store.AddPricingRule(ctx, rule); err != nil {
		t.Fatalf("AddPricingRule() error = %v", err)
	}
	if rules, err := store.ListPricingRules(ctx); err != nil || len(rules) != 1 || rules[0].Rule.Version != rule.Version {
		t.Fatalf("ListPricingRules() = %#v, %v", rules, err)
	}
	if count, err := store.RecalculateCosts(ctx, PricingRecalculateQuery{}); err != nil || count != 1 {
		t.Fatalf("RecalculateCosts() = %d, %v", count, err)
	}
	if allocations, err := store.ModelCallAllocations(ctx, callID); err != nil || len(allocations) != 1 || allocations[0].BasisPoints != 10_000 {
		t.Fatalf("ModelCallAllocations() = %#v, %v", allocations, err)
	}
	if err := store.RepairModelCallAllocation(ctx, callID, project.ID); err != nil {
		t.Fatalf("RepairModelCallAllocation() error = %v", err)
	}
}
