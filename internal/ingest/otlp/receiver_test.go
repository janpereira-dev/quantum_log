package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestReceiverImportsStandardOTLPJSONThroughCentralResolver(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	worktree := filepath.Join(t.TempDir(), "project")
	project, _, err := service.Store.RegisterProject(ctx, "Project", "project", worktree)
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"OpenCode"}}]},"scopeSpans":[{"spans":[{"traceId":"trace-a","startTimeUnixNano":"1763294400000000000","attributes":[{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"gen_ai.provider.name","value":{"stringValue":"example"}},{"key":"gen_ai.request.model","value":{"stringValue":"model"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"7"}},{"key":"gen_ai.usage.output_tokens","value":{"intValue":"3"}},{"key":"gen_ai.prompt","value":{"stringValue":"must not persist"}}]}]}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "provider", "model"}})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if len(report.Rows) != 1 || report.Rows[0].TotalTokens != 10 || report.Rows[0].ProjectSlug != project.Slug {
		t.Fatalf("usage = %#v", report)
	}
	if err := service.Store.VerifyLedger(ctx, "trace-a"); err != nil {
		t.Fatalf("verify ledger: %v", err)
	}
}

func TestReceiverImportsCopilotOTLPTokensCacheReasoningAndProjectMetadata(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	worktree := filepath.Join(t.TempDir(), "repo")
	project, _, err := service.Store.RegisterProject(ctx, "Repo", "repo", worktree)
	if err != nil {
		t.Fatalf("register project: %v", err)
	}

	payload := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"copilot-chat"}},{"key":"service.version","value":{"stringValue":"1.112.0"}},{"key":"session.id","value":{"stringValue":"window-1"}}]},"scopeSpans":[{"spans":[{"traceId":"trace-copilot","startTimeUnixNano":"1763294400000000000","attributes":[{"key":"gen_ai.operation.name","value":{"stringValue":"chat"}},{"key":"gen_ai.provider.name","value":{"stringValue":"github"}},{"key":"gen_ai.agent.name","value":{"stringValue":"GitHub Copilot Chat"}},{"key":"gen_ai.request.model","value":{"stringValue":"gpt-5"}},{"key":"gen_ai.response.model","value":{"stringValue":"gpt-5-resolved"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"11"}},{"key":"gen_ai.usage.output_tokens","value":{"intValue":"13"}},{"key":"gen_ai.usage.cache_read.input_tokens","value":{"intValue":"17"}},{"key":"gen_ai.usage.cache_creation.input_tokens","value":{"intValue":"19"}},{"key":"gen_ai.usage.reasoning.output_tokens","value":{"intValue":"23"}},{"key":"github.copilot.git.repository","value":{"stringValue":"` + filepath.ToSlash(worktree) + `"}},{"key":"github.copilot.git.branch","value":{"stringValue":"main"}},{"key":"github.copilot.git.commit_sha","value":{"stringValue":"abc123"}}]}]}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
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
	if row.ProjectSlug != project.Slug || row.AgentName != "GitHub Copilot Chat" || row.Provider != "github" || row.Model != "gpt-5-resolved" || row.CaptureQuality != "otel_reported" {
		t.Fatalf("row identity = %#v", row)
	}
	if row.InputTokens != 11 || row.OutputTokens != 13 || row.CachedInputTokens != 17 || row.CacheWriteTokens != 19 || row.ReasoningTokens != 23 || row.TotalTokens != 83 {
		t.Fatalf("row tokens = %#v", row)
	}
}

func TestReceiverRejectsNonJSON(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString("ignored"))
	request.Header.Set("Content-Type", "application/x-protobuf")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("response = %d", response.Code)
	}
}

func TestReceiverDoesNotCopyUnrecognizedOTLPAttributesIntoPayload(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	line, err := Receiver{service: service}.event(context.Background(), map[string]string{}, map[string]string{
		"gen_ai.provider.name":   "example",
		"gen_ai.request.model":   "model",
		"gen_ai.prompt":          "must-not-persist",
		"authorization":          "Bearer secret",
		"unrecognized.attribute": "private-value",
	}, span{})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	encoded, err := json.Marshal(line["payload"])
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, forbidden := range []string{"must-not-persist", "Bearer secret", "private-value"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("payload retained %q: %s", forbidden, encoded)
		}
	}
}
