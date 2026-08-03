package qlogevent

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	_ "modernc.org/sqlite"
)

func TestHandlerKeepsOpenCodePluginEventsLifecycleOnly(t *testing.T) {
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
	if row.ProjectSlug != project.Slug || row.AgentName != "opencode" || row.TotalTokens != 0 || row.CaptureQuality != "lifecycle_only" {
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

func TestHandlerReportsDuplicateEvent(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repo := filepath.Join(t.TempDir(), "repo")
	if _, _, err := service.Store.RegisterProject(ctx, "Repo", "repo", repo); err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"source":"qlog-plugin","session_id":"session-1","event_type":"agent.event","occurred_at":"2026-07-20T10:00:00Z","project_hint":{"cwd":"` + filepath.ToSlash(repo) + `"},"upstream_event_id":"event-1","payload":{}}`
	handler := NewHandler(service)
	for attempt, want := range []map[string]int{{"accepted": 1, "duplicates": 0}, {"accepted": 0, "duplicates": 1}} {
		request := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewBufferString(payload))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d response = %d: %s", attempt+1, response.Code, response.Body.String())
		}
		got := map[string]int{}
		if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
			t.Fatalf("attempt %d decode response: %v", attempt+1, err)
		}
		if !mapsEqual(got, want) {
			t.Fatalf("attempt %d response = %#v, want %#v", attempt+1, got, want)
		}
	}
}

func TestIngestExportsReusableSanitizedEventImport(t *testing.T) {
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
	payload := json.RawMessage(`{"agent_name":"claude-code","capture_quality":"lifecycle_only","prompt":"secret prompt","response":"secret response","content":"secret content","tool_args":{"value":"secret"},"tool_results":"secret result","authorization":"bearer secret","tokens":"secret tokens","api keys":"secret key","secrets":"secret"}`)
	count, err := Ingest(ctx, service, Event{
		Source:      "claude-code-hook",
		SessionID:   "session-1",
		EventType:   "Stop",
		OccurredAt:  time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
		ProjectHint: ProjectHint{CWD: repo},
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Ingest() count = %d", count)
	}
	var stored string
	reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(service.Paths.Database)+"?mode=ro")
	if err != nil {
		t.Fatalf("open raw event reader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	if err := reader.QueryRowContext(ctx, `SELECT payload_json_sanitized FROM raw_events WHERE source = 'claude-code-hook' AND session_id = 'session-1'`).Scan(&stored); err != nil {
		t.Fatalf("read raw event: %v", err)
	}
	for _, forbidden := range []string{"secret", "prompt", "response", "content", "tool_args", "tool_results", "authorization", "tokens", "api keys", "secrets"} {
		if strings.Contains(stored, forbidden) {
			t.Fatalf("payload retained forbidden %q: %s", forbidden, stored)
		}
	}
	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "agent", "capture_quality"}})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if len(report.Rows) != 0 || report.TotalTokens != 0 {
		t.Fatalf("lifecycle event invented usage for %s: %#v", project.Slug, report)
	}
}

func TestPluginPayloadAllowlistDropsRemoteURLAndTranscript(t *testing.T) {
	got := sanitizePluginPayload(json.RawMessage(`{"provider":"x","model":"y","remote_url":"https://user:password@example.test","transcript_path":"private","input_tokens":1}`))
	if bytes.Contains(got, []byte("remote_url")) || bytes.Contains(got, []byte("transcript_path")) {
		t.Fatalf("payload = %s", got)
	}
}

func TestPluginPayloadDropsNegativeUsageCounters(t *testing.T) {
	got := sanitizePluginPayload(json.RawMessage(`{"agent_name":"opencode","input_tokens":-1,"output_tokens":2,"capture_quality":"agent_reported"}`))
	if bytes.Contains(got, []byte("input_tokens")) {
		t.Fatalf("payload retained negative usage = %s", got)
	}
}

func mapsEqual(got, want map[string]int) bool {
	if len(got) != len(want) {
		return false
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}
