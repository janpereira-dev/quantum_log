package jsonl

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

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
