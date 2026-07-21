package qlogevent

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestHandlerImportsPluginModelCallThroughSQLiteReport(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repo := filepath.Join(t.TempDir(), "repo")
	project, _, err := service.Store.RegisterProject(ctx, "Repo", "repo", repo)
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"source":"opencode-plugin","session_id":"session-1","event_type":"model.call","occurred_at":"` + time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC).Format(time.RFC3339) + `","project_hint":{"cwd":"` + filepath.ToSlash(repo) + `"},"payload":{"provider":"anthropic","model":"claude-sonnet","agent_name":"opencode","input_tokens":31,"output_tokens":37,"capture_quality":"agent_reported","prompt":"must not persist"}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %#v", report.Rows)
	}
	row := report.Rows[0]
	if row.ProjectSlug != project.Slug || row.AgentName != "opencode" || row.TotalTokens != 68 || row.CaptureQuality != "agent_reported" {
		t.Fatalf("row = %#v", row)
	}
}

func TestHandlerMapsCodexRawResponseCompletedUsage(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repo := filepath.Join(t.TempDir(), "repo")
	project, _, err := service.Store.RegisterProject(ctx, "Repo", "repo", repo)
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"source":"codex-app-server","session_id":"thread-1","event_type":"rawResponse/completed","occurred_at":"` + time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC).Format(time.RFC3339) + `","project_hint":{"cwd":"` + filepath.ToSlash(repo) + `"},"payload":{"model":"gpt-5","usage":{"input_tokens":41,"output_tokens":43,"input_tokens_details":{"cached_tokens":47},"output_tokens_details":{"reasoning_tokens":53}}}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %#v", report.Rows)
	}
	row := report.Rows[0]
	if row.ProjectSlug != project.Slug || row.AgentName != "codex" || row.Provider != "openai" || row.Model != "gpt-5" || row.TotalTokens != 184 || row.CaptureQuality != "agent_reported" || row.CachedInputTokens != 47 || row.ReasoningTokens != 53 {
		t.Fatalf("row = %#v", row)
	}
}
