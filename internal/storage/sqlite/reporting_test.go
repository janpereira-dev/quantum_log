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
	if first.Rows[0].TotalTokens != 90 || first.Rows[1].TotalTokens != 60 || first.Rows[0].AllocatedCostUSDMicros != 600_000 || first.Rows[1].AllocatedCostUSDMicros != 400_000 {
		t.Fatalf("split usage rows = %#v", first.Rows)
	}
	filtered, err := store.Usage(ctx, UsageQuery{ProjectSlug: projectA.Slug, GroupBy: []string{"project"}})
	if err != nil || filtered.TotalTokens != 90 || len(filtered.Rows) != 1 || filtered.Rows[0].TotalTokens != 90 || filtered.AllocatedCostUSDMicros != 600_000 {
		t.Fatalf("project filtered usage = %#v, %v", filtered, err)
	}
	if got := measurement(first.Measurements, "unknown").TotalTokens; got != 150 {
		t.Fatalf("split measurement tokens = %d, want 150", got)
	}
}

func TestUsageSplitApportionsRemaindersWithoutDroppingTokens(t *testing.T) {
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
	callID, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: projectA.ID, Provider: "example", ModelID: "model", InputTokens: 1, EstimatedCostUSDMicros: 1, OccurredAt: time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}
	if err := store.ReplaceAllocations(ctx, "model_call", callID, []AllocationInput{{ProjectID: projectA.ID, BasisPoints: 6000}, {ProjectID: projectB.ID, BasisPoints: 4000}}); err != nil {
		t.Fatalf("ReplaceAllocations() error = %v", err)
	}
	for repeat := 0; repeat < 10; repeat++ {
		report, err := store.Usage(ctx, UsageQuery{GroupBy: []string{"project"}})
		if err != nil {
			t.Fatalf("Usage() error = %v", err)
		}
		if len(report.Rows) != 2 || report.TotalTokens != 1 || report.AllocatedCostUSDMicros != 1 {
			t.Fatalf("remainder usage = %#v", report)
		}
		if report.Rows[0].TotalTokens+report.Rows[1].TotalTokens != report.TotalTokens || report.Rows[0].AllocatedCostUSDMicros+report.Rows[1].AllocatedCostUSDMicros != report.AllocatedCostUSDMicros {
			t.Fatalf("remainder allocation does not conserve totals = %#v", report.Rows)
		}
		for _, projectSlug := range []string{projectA.Slug, projectB.Slug} {
			filtered, err := store.Usage(ctx, UsageQuery{ProjectSlug: projectSlug, GroupBy: []string{"project"}})
			if err != nil {
				t.Fatalf("Usage(%s) error = %v", projectSlug, err)
			}
			var expected UsageRow
			for _, row := range report.Rows {
				if row.ProjectSlug == projectSlug {
					expected = row
					break
				}
			}
			if len(filtered.Rows) != 1 || filtered.Rows[0] != expected || filtered.TotalTokens != expected.TotalTokens || filtered.AllocatedCostUSDMicros != expected.AllocatedCostUSDMicros {
				t.Fatalf("filtered %s usage = %#v, want global row %#v", projectSlug, filtered, expected)
			}
		}
	}
}

func TestUsageGroupsByProjectAgentProviderModelAndCaptureQuality(t *testing.T) {
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

	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, AgentName: "copilot-chat", Provider: "github", ModelID: "gpt-5", InputTokens: 10, OutputTokens: 5, CaptureQuality: "otel_reported", OccurredAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("RecordModelCall(reported) error = %v", err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, AgentName: "opencode", Provider: "anthropic", ModelID: "claude-sonnet", InputTokens: 7, OutputTokens: 3, CaptureQuality: "estimated", OccurredAt: time.Date(2026, 7, 20, 10, 1, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("RecordModelCall(estimated) error = %v", err)
	}

	report, err := store.Usage(ctx, UsageQuery{GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %d, want 2: %#v", len(report.Rows), report.Rows)
	}
	if report.Rows[0].AgentName != "copilot-chat" || report.Rows[0].CaptureQuality != "otel_reported" || report.Rows[0].TotalTokens != 15 {
		t.Fatalf("first row = %#v", report.Rows[0])
	}
	if report.Rows[1].AgentName != "opencode" || report.Rows[1].CaptureQuality != "estimated" || report.Rows[1].TotalTokens != 10 {
		t.Fatalf("second row = %#v", report.Rows[1])
	}
}

func TestUsageSeparatesReportedLifecycleAndEstimatedMeasurements(t *testing.T) {
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
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, AgentName: "opencode", Provider: "anthropic", ModelID: "claude", InputTokens: 7, OutputTokens: 5, CaptureQuality: "agent_reported"}); err != nil {
		t.Fatalf("RecordModelCall(agent_reported) error = %v", err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, AgentName: "opencode", Provider: "anthropic", ModelID: "claude", InputTokens: 4, OutputTokens: 5, CaptureQuality: "estimated"}); err != nil {
		t.Fatalf("RecordModelCall(estimated) error = %v", err)
	}
	if _, err := store.AppendRawEvent(ctx, RawEventInput{Source: "claude-code-hook", SessionID: "lifecycle-session", EventType: "lifecycle.stop", Payload: []byte(`{"agent_name":"claude-code","capture_quality":"lifecycle_only"}`), OccurredAt: time.Now().UTC(), ProjectID: project.ID, ResolutionMethod: "explicit", ResolutionConfidence: "exact"}); err != nil {
		t.Fatalf("AppendRawEvent(lifecycle_only) error = %v", err)
	}

	report, err := store.Usage(ctx, UsageQuery{GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if got := measurement(report.Measurements, "agent_reported").TotalTokens; got != 12 {
		t.Fatalf("agent-reported tokens = %d, want 12", got)
	}
	if got := measurement(report.Measurements, "estimated").TotalTokens; got != 9 {
		t.Fatalf("estimated tokens = %d, want 9", got)
	}
	if got := measurement(report.Measurements, "lifecycle_only"); got.ModelCallCount != 0 || got.TotalTokens != 0 {
		t.Fatalf("lifecycle-only measurement = %#v, want zero model calls and tokens", got)
	}
}

func TestSessionSnapshotPreservesResolutionConfidenceAndLifecycleEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.EnsureSession(ctx, "session-1", "stored-agent", time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("EnsureSession() error = %v", err)
	}
	if _, err := store.AppendRawEvent(ctx, RawEventInput{Source: "claude-code-hook", SessionID: "session-1", EventType: "lifecycle.stop", Payload: []byte(`{"agent_name":"claude-code","capture_quality":"lifecycle_only"}`), OccurredAt: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), ResolutionMethod: "explicit", ResolutionConfidence: "exact"}); err != nil {
		t.Fatalf("AppendRawEvent() error = %v", err)
	}

	snapshot, err := store.SessionSnapshot(ctx, "session-1")
	if err != nil {
		t.Fatalf("SessionSnapshot() error = %v", err)
	}
	if snapshot.ResolutionMethod != "explicit" || snapshot.ResolutionConfidence != "exact" {
		t.Fatalf("resolution = %q/%q", snapshot.ResolutionMethod, snapshot.ResolutionConfidence)
	}
	if snapshot.AgentName != "stored-agent" {
		t.Fatalf("agent = %q, want stored-agent", snapshot.AgentName)
	}
	if snapshot.RawEventCount != 1 || snapshot.LifecycleEventCount != 1 {
		t.Fatalf("raw lifecycle evidence = %d/%d, want 1/1", snapshot.RawEventCount, snapshot.LifecycleEventCount)
	}
	if got := measurement(snapshot.Measurements, "lifecycle_only"); got.ModelCallCount != 0 || got.TotalTokens != 0 {
		t.Fatalf("lifecycle-only measurement = %#v, want zero model calls and tokens", got)
	}
}

func TestSessionSnapshotsReturnsAggregateEvidenceWithoutPayloads(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.AppendRawEvent(ctx, RawEventInput{Source: "fixture", SessionID: "session-b", EventType: "session.completed", OccurredAt: time.Now().UTC(), Payload: []byte(`{"agent_name":"opencode","capture_quality":"lifecycle_only","prompt":"must-not-return"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSession(ctx, "session-a", "claude-code", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	snapshots, err := store.SessionSnapshots(ctx)
	if err != nil {
		t.Fatalf("SessionSnapshots() error = %v", err)
	}
	if len(snapshots) != 2 || snapshots[0].SessionID != "session-a" || snapshots[1].SessionID != "session-b" {
		t.Fatalf("SessionSnapshots() = %#v", snapshots)
	}
	if snapshots[1].LifecycleEventCount != 1 || snapshots[1].ModelCallCount != 0 {
		t.Fatalf("lifecycle snapshot = %#v", snapshots[1])
	}
}

func TestCapabilityReportCountsRecognizedRawEvidenceWithoutInventingUsage(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	project, _, err := store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, eventType := range []string{"Stop", "UserPromptSubmit", "SubagentStop", "SessionStart", "sessionStart", "sessionEnd", "agentStop", "tool.execute.failed", "mcp.call"} {
		if _, err := store.AppendRawEvent(ctx, RawEventInput{Source: "fixture", SessionID: "session-1", EventType: eventType, ProjectID: project.ID, OccurredAt: now, Payload: []byte(`{"agent_name":"fixture","capture_quality":"lifecycle_only"}`)}); err != nil {
			t.Fatalf("append %s: %v", eventType, err)
		}
	}
	report, err := store.CapabilityReport(ctx, CapabilityQuery{ProjectSlug: project.Slug, AgentName: "fixture", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	if report.ModelCalls != 0 || report.Tokens != 0 || report.LifecycleEvents != 7 || report.ToolCalls != 1 || report.MCPCalls != 1 || report.Errors != 1 {
		t.Fatalf("capability report = %#v", report)
	}
}

func TestCapabilityReportCountsCanonicalInteractionsNotModelCalls(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	project, _, err := store.RegisterProject(ctx, "Project", "project", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, upstreamID := range []string{"prompt-1", "prompt-2"} {
		if _, _, err := store.RecordInteraction(ctx, InteractionInput{Source: "test", SessionID: "session", UpstreamID: upstreamID, ProjectID: project.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: project.ID, Provider: "provider", ModelID: "model"}); err != nil {
		t.Fatal(err)
	}
	report, err := store.CapabilityReport(ctx, CapabilityQuery{ProjectSlug: project.Slug})
	if err != nil || report.Interactions != 2 || report.Prompts != 2 || report.ModelCalls != 1 {
		t.Fatalf("report = %#v, %v", report, err)
	}
}

func TestCapabilityReportIncludesExplicitlyAllocatedCallsWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	primary, _, err := store.RegisterProject(ctx, "Primary", "primary", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allocated, _, err := store.RegisterProject(ctx, "Allocated", "allocated", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	callID, err := store.RecordModelCall(ctx, ModelCallInput{ProjectID: primary.ID, Provider: "example", ModelID: "model", InputTokens: 5})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceAllocations(ctx, "model_call", callID, []AllocationInput{{ProjectID: primary.ID, BasisPoints: 5000}, {ProjectID: allocated.ID, BasisPoints: 5000}}); err != nil {
		t.Fatal(err)
	}
	report, err := store.CapabilityReport(ctx, CapabilityQuery{ProjectSlug: allocated.Slug})
	if err != nil || report.ModelCalls != 1 || report.Tokens != 5 {
		t.Fatalf("allocated capability report = %#v, %v", report, err)
	}
}

func measurement(measurements []MeasurementSummary, quality string) MeasurementSummary {
	for _, summary := range measurements {
		if summary.Quality == quality {
			return summary
		}
	}
	return MeasurementSummary{Quality: quality}
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
