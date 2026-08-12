package jsonl

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestImportAppendsSanitizedNDJSONEvents(t *testing.T) {
	store, err := storepkg.Open(context.Background(), filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := strings.NewReader(`{"source":"fixture","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-16T12:00:00Z","payload":{"tokens":12,"prompt":"must not persist"}}` + "\n")
	count, err := Import(context.Background(), store, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Import() count = %d, want 1", count)
	}
	if err := store.VerifyLedger(context.Background(), "session-a"); err != nil {
		t.Fatalf("VerifyLedger() error = %v", err)
	}
}

func TestImportRejectsInvalidNDJSON(t *testing.T) {
	store, err := storepkg.Open(context.Background(), filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := Import(context.Background(), store, strings.NewReader("not-json\n")); err == nil {
		t.Fatal("Import() accepted invalid NDJSON")
	}
}

func TestImportNormalizesModelCallPayload(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	project, _, err := store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatalf("RegisterProject() error = %v", err)
	}

	input := strings.NewReader(`{"source":"fixture","session_id":"session-a","event_type":"model.call","project_id":"` + project.ID + `","occurred_at":"2026-07-16T12:00:00Z","payload":{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture"}}` + "\n")
	if _, err := Import(ctx, store, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	report, err := store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "provider", "model"}})
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if len(report.Rows) != 1 || report.Rows[0].Provider != "example" || report.Rows[0].TotalTokens != 20 {
		t.Fatalf("normalized usage = %#v", report)
	}
}

func TestImportSkipsMetricObservationWithoutValue(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := strings.NewReader(`{"source":"fixture","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-16T12:00:00Z","payload":{"provider":"example","model":"model","agent_name":"fixture","metric_observations":[{"name":"input_tokens","source":"otel","raw_key":"gen_ai.usage.input_tokens","confidence":"reported"}]}}` + "\n")
	if _, err := Import(ctx, store, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	report, err := store.CapabilityReport(ctx, storepkg.CapabilityQuery{})
	if err != nil {
		t.Fatalf("CapabilityReport() error = %v", err)
	}
	for _, metric := range report.MetricCoverage {
		if metric.Name == "input_tokens" && (metric.ReportedCount != 0 || metric.ReportedZeroCount != 0) {
			t.Fatalf("absent metric observation was fabricated as zero: %#v", metric)
		}
	}
}

func TestImportCarriesSourceVersionIntoCapabilityReport(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := strings.NewReader(`{"source":"fixture","source_version":"1.2.3","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-16T12:00:00Z","payload":{"provider":"example","model":"model","agent_name":"fixture"}}` + "\n")
	if _, err := Import(ctx, store, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	report, err := store.CapabilityReport(ctx, storepkg.CapabilityQuery{})
	if err != nil || len(report.Sources) != 1 || report.Sources[0].Version == nil || *report.Sources[0].Version != "1.2.3" {
		t.Fatalf("capability source version = %#v, %v", report.Sources, err)
	}
}

func TestImportReplayNormalizesOnlyAcceptedRawEvent(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := `{"source":"fixture","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-30T12:00:00Z","payload":{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture"}}` + "\n"
	if count, err := Import(ctx, store, strings.NewReader(input)); err != nil || count != 1 {
		t.Fatalf("first Import() = %d, %v", count, err)
	}
	if count, err := Import(ctx, store, strings.NewReader(input)); err != nil || count != 0 {
		t.Fatalf("replay Import() = %d, %v", count, err)
	}
	report, err := store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"provider", "model"}})
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if report.TotalTokens != 20 {
		t.Fatalf("replayed usage = %#v", report)
	}
}

func TestImportReplayRecoversNormalizationAfterAcceptedRawEvent(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := `{"source":"fixture","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-30T12:00:00Z","payload":{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture"}}` + "\n"
	if result, err := store.AppendRawEvent(ctx, storepkg.RawEventInput{Source: "fixture", SessionID: "session-a", EventType: "model.call", OccurredAt: mustParseTime(t, "2026-07-30T12:00:00Z"), Payload: []byte(`{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture"}`)}); err != nil || !result.Accepted {
		t.Fatalf("persist raw event before interrupted normalization = %#v, %v", result, err)
	}
	if count, err := Import(ctx, store, strings.NewReader(input)); err != nil || count != 0 {
		t.Fatalf("replay Import() = %d, %v", count, err)
	}
	if err := store.VerifyLedger(ctx, "session-a"); err != nil {
		t.Fatalf("verify recovered ledger: %v", err)
	}
	report, err := store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"provider", "model"}})
	if err != nil || report.TotalTokens != 20 {
		t.Fatalf("recovered usage = %#v, %v", report, err)
	}
}

func TestImportReplayLinksMatchingLegacyNormalizedModelCall(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	occurredAt := mustParseTime(t, "2026-07-30T12:00:00Z")
	payload := []byte(`{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture","metric_observations":[{"name":"input_tokens","value":12,"source":"otel","raw_key":"gen_ai.usage.input_tokens","confidence":"reported"}]}`)
	raw, err := store.AppendRawEvent(ctx, storepkg.RawEventInput{Source: "fixture", SessionID: "session-a", EventType: "model.call", OccurredAt: occurredAt, Payload: payload})
	if err != nil || !raw.Accepted {
		t.Fatalf("AppendRawEvent() = %#v, %v", raw, err)
	}
	if err := store.EnsureSession(ctx, "session-a", "fixture", occurredAt); err != nil {
		t.Fatalf("EnsureSession() error = %v", err)
	}
	_, err = store.RecordModelCall(ctx, storepkg.ModelCallInput{SessionID: "session-a", Provider: "example", ModelID: "model", AgentName: "fixture", InputTokens: 12, OutputTokens: 8, OccurredAt: occurredAt})
	if err != nil {
		t.Fatalf("RecordModelCall() error = %v", err)
	}

	input := strings.NewReader(`{"source":"fixture","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-30T12:00:00Z","payload":{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture","metric_observations":[{"name":"input_tokens","value":12,"source":"otel","raw_key":"gen_ai.usage.input_tokens","confidence":"reported"}]}}` + "\n")
	if count, err := Import(ctx, store, input); err != nil || count != 0 {
		t.Fatalf("replay Import() = %d, %v", count, err)
	}
	report, err := store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"provider", "model"}})
	if err != nil || report.TotalTokens != 20 {
		t.Fatalf("replayed usage = %#v, %v", report, err)
	}
	capability, err := store.CapabilityReport(ctx, storepkg.CapabilityQuery{})
	if err != nil {
		t.Fatalf("CapabilityReport() error = %v", err)
	}
	for _, metric := range capability.MetricCoverage {
		if metric.Name == "input_tokens" {
			if len(metric.Provenance) != 1 || metric.Provenance[0].RawKey != "gen_ai.usage.input_tokens" {
				t.Fatalf("linked metric provenance = %#v", metric)
			}
			return
		}
	}
	t.Fatal("linked metric observation missing")
}

func TestImportWithoutOccurredAtSuppressesReplayButKeepsDistinctPayloads(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	first := `{"source":"fixture","session_id":"session-a","event_type":"agent.event","payload":{"turn_id":"one"}}` + "\n"
	second := `{"source":"fixture","session_id":"session-a","event_type":"agent.event","payload":{"turn_id":"two"}}` + "\n"
	if count, err := Import(ctx, store, strings.NewReader(first)); err != nil || count != 1 {
		t.Fatalf("first Import() = %d, %v", count, err)
	}
	if count, err := Import(ctx, store, strings.NewReader(first)); err != nil || count != 0 {
		t.Fatalf("replay Import() = %d, %v", count, err)
	}
	if count, err := Import(ctx, store, strings.NewReader(second)); err != nil || count != 1 {
		t.Fatalf("distinct Import() = %d, %v", count, err)
	}
	if err := store.VerifyLedger(ctx, "session-a"); err != nil {
		t.Fatalf("verify timestamp-less ledger: %v", err)
	}
}

func TestImportCreatesOneInteractionForEachDistinctPrompt(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var input strings.Builder
	for index := 0; index < 100; index++ {
		fmt.Fprintf(&input, `{"source":"fixture-hook","upstream_event_id":"prompt-%03d","session_id":"session-a","event_type":"interaction.prompt","occurred_at":"2026-08-12T12:00:00Z","payload":{"agent_name":"fixture","interaction_hash":"hash-%03d"}}`+"\n", index, index)
	}
	if count, err := Import(ctx, store, strings.NewReader(input.String())); err != nil || count != 100 {
		t.Fatalf("Import() = %d, %v", count, err)
	}
	report, err := store.CapabilityReport(ctx, storepkg.CapabilityQuery{AgentName: "fixture"})
	if err != nil || report.Interactions != 100 || report.Prompts != 100 || report.ModelCalls != 0 {
		t.Fatalf("interaction report = %#v, %v", report, err)
	}
}

func TestImportReplayedPromptDoesNotDuplicateInteraction(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	input := `{"source":"fixture-hook","upstream_event_id":"prompt-1","session_id":"session-a","event_type":"interaction.prompt","occurred_at":"2026-08-12T12:00:00Z","payload":{"agent_name":"fixture","interaction_hash":"hash-1"}}` + "\n"
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := Import(ctx, store, strings.NewReader(input)); err != nil {
			t.Fatalf("Import(%d): %v", attempt, err)
		}
	}
	report, err := store.CapabilityReport(ctx, storepkg.CapabilityQuery{AgentName: "fixture"})
	if err != nil || report.Interactions != 1 || report.Prompts != 1 {
		t.Fatalf("deduplicated interaction report = %#v, %v", report, err)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestConcurrentReplayCreatesOneRawEventAndOneModelCall(t *testing.T) {
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	input := `{"source":"fixture","session_id":"session-a","event_type":"model.call","occurred_at":"2026-07-30T12:00:00Z","payload":{"provider":"example","model":"model","input_tokens":12,"output_tokens":8,"agent_name":"fixture"}}` + "\n"
	start := make(chan struct{})
	counts := make(chan int, 2)
	errs := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			count, err := Import(ctx, store, strings.NewReader(input))
			counts <- count
			errs <- err
		}()
	}
	close(start)
	group.Wait()
	close(counts)
	close(errs)

	accepted := 0
	for count := range counts {
		accepted += count
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Import() error = %v", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted = %d, want 1", accepted)
	}
	report, err := store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"provider", "model"}})
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if report.TotalTokens != 20 {
		t.Fatalf("concurrent usage = %#v", report)
	}
	if err := store.VerifyLedger(ctx, "session-a"); err != nil {
		t.Fatalf("VerifyLedger() error = %v", err)
	}
}
