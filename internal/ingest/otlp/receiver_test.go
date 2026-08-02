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
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	collectorlogpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
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

func TestReceiverReportsDuplicateTrace(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	worktree := filepath.Join(t.TempDir(), "project")
	if _, _, err := service.Store.RegisterProject(ctx, "Project", "project", worktree); err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"resourceSpans":[{"resource":{"attributes":[]},"scopeSpans":[{"spans":[{"traceId":"trace-duplicate","startTimeUnixNano":"1763294400000000000","attributes":[{"key":"qlog.project","value":{"stringValue":"project"}}]}]}]}]}`
	handler := NewHandler(service)
	for attempt, want := range []map[string]int{{"accepted": 1, "duplicates": 0}, {"accepted": 0, "duplicates": 1}} {
		request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
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
		if !responseCountsEqual(got, want) {
			t.Fatalf("attempt %d response = %#v, want %#v", attempt+1, got, want)
		}
	}
}

func TestReceiverImportsCodexResponseCompletedLogsAndDeduplicatesReplay(t *testing.T) {
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
	payload := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"codex"}}]},"scopeLogs":[{"logRecords":[{"traceId":"codex-trace","spanId":"codex-span","timeUnixNano":"1763294400000000000","attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"conversation.id","value":{"stringValue":"conversation-1"}},{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"model","value":{"stringValue":"gpt-5"}},{"key":"input_tokens","value":{"intValue":"41"}},{"key":"output_tokens","value":{"intValue":"43"}},{"key":"cached_input_tokens","value":{"intValue":"47"}},{"key":"reasoning_output_tokens","value":{"intValue":"53"}},{"key":"user.prompt","value":{"stringValue":"must-not-persist"}},{"key":"authorization","value":{"stringValue":"Bearer secret"}},{"key":"tool.result","value":{"stringValue":"private tool output"}}]}]}]}]}`
	handler := NewHandler(service)
	for attempt, want := range []map[string]int{{"accepted": 1, "duplicates": 0}, {"accepted": 0, "duplicates": 1}} {
		request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(payload))
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
		if !responseCountsEqual(got, want) {
			t.Fatalf("attempt %d response = %#v, want %#v", attempt+1, got, want)
		}
	}
	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("usage rows = %#v", report.Rows)
	}
	row := report.Rows[0]
	if row.ProjectSlug != project.Slug || row.AgentName != "codex" || row.Provider != "openai" || row.Model != "gpt-5" || row.InputTokens != 41 || row.OutputTokens != 43 || row.CachedInputTokens != 47 || row.ReasoningTokens != 53 || row.TotalTokens != 184 || row.CaptureQuality != "otel_reported" {
		t.Fatalf("usage row = %#v", row)
	}
}

func TestReceiverRejectsGenericOrSpoofedLogs(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	payload := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"other-agent"}}]},"scopeLogs":[{"logRecords":[{"traceId":"trace","spanId":"span","attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"model","value":{"stringValue":"gpt-5"}},{"key":"input_tokens","value":{"intValue":"1"}},{"key":"output_tokens","value":{"intValue":"1"}}]}]}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
}

func TestReceiverSkipsUnsupportedCodexLogsWhenBatchContainsSanctionedRecord(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, _, err := service.Store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"codex"}}]},"scopeLogs":[{"logRecords":[{"traceId":"trace-invalid","spanId":"span-invalid","attributes":[{"key":"event.name","value":{"stringValue":"other"}}]},{"traceId":"trace-valid","spanId":"span-valid","attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"model","value":{"stringValue":"gpt-5"}},{"key":"input_tokens","value":{"intValue":"1"}},{"key":"output_tokens","value":{"intValue":"2"}}]}]}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var result map[string]int
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !responseCountsEqual(result, map[string]int{"accepted": 1, "duplicates": 1}) {
		t.Fatalf("response = %#v", result)
	}
	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{ProjectSlug: project.Slug, GroupBy: []string{"project"}})
	if err != nil || report.TotalTokens != 3 {
		t.Fatalf("usage = %#v, %v", report, err)
	}
}

func TestReceiverReturnsProtobufExportResponseForProtobufLogs(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	payload := &collectorlogpb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{}}
	body, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/x-protobuf")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/x-protobuf" {
		t.Fatalf("response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	var result collectorlogpb.ExportLogsServiceResponse
	if err := proto.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode protobuf response: %v", err)
	}
}

func TestReceiverKeepsDistinctCodexRecordsSharingTraceAndSpan(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if _, _, err := service.Store.RegisterProject(ctx, "Project", "project", filepath.Join(t.TempDir(), "project")); err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"codex"}}]},"scopeLogs":[{"logRecords":[{"traceId":"shared-trace","spanId":"shared-span","attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"response.id","value":{"stringValue":"response-1"}},{"key":"model","value":{"stringValue":"gpt-5"}},{"key":"input_tokens","value":{"intValue":"1"}},{"key":"output_tokens","value":{"intValue":"2"}}]},{"traceId":"shared-trace","spanId":"shared-span","attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"response.id","value":{"stringValue":"response-2"}},{"key":"model","value":{"stringValue":"gpt-5"}},{"key":"input_tokens","value":{"intValue":"3"}},{"key":"output_tokens","value":{"intValue":"4"}}]}]}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var result map[string]int
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !responseCountsEqual(result, map[string]int{"accepted": 2, "duplicates": 0}) {
		t.Fatalf("response = %#v", result)
	}
}

func TestCodexLogEventAllowlistsOnlyModelAndUsageFields(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	line, accepted, err := (Receiver{service: service}).codexLogEvent(context.Background(), map[string]string{"service.name": "codex"}, map[string]string{
		"event.name":             "codex.sse_event",
		"event.kind":             "response.completed",
		"model":                  "gpt-5",
		"input_tokens":           "1",
		"output_tokens":          "2",
		"user.prompt":            "must-not-persist",
		"response.content":       "private-response",
		"tool.result":            "private-tool-result",
		"authorization":          "Bearer secret",
		"unrecognized.attribute": "private-value",
	}, logRecord{TraceID: "trace", SpanID: "span"})
	if err != nil || !accepted {
		t.Fatalf("Codex log event = %#v, accepted=%t, error=%v", line, accepted, err)
	}
	payload, err := json.Marshal(line["payload"])
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, forbidden := range []string{"must-not-persist", "private-response", "private-tool-result", "Bearer secret", "private-value"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("payload retained %q: %s", forbidden, payload)
		}
	}
}

func TestReceiverImportsDistinctSpansInSameTraceAndDeduplicatesRetry(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	worktree := filepath.Join(t.TempDir(), "project")
	if _, _, err := service.Store.RegisterProject(ctx, "Project", "project", worktree); err != nil {
		t.Fatalf("register project: %v", err)
	}
	payload := `{"resourceSpans":[{"resource":{"attributes":[]},"scopeSpans":[{"spans":[{"traceId":"trace-shared","spanId":"span-one","startTimeUnixNano":"1763294400000000000","attributes":[{"key":"qlog.project","value":{"stringValue":"project"}}]},{"traceId":"trace-shared","spanId":"span-two","startTimeUnixNano":"1763294400000000001","attributes":[{"key":"qlog.project","value":{"stringValue":"project"}}]}]}]}]}`
	handler := NewHandler(service)
	for attempt, want := range []map[string]int{{"accepted": 2, "duplicates": 0}, {"accepted": 0, "duplicates": 2}} {
		request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
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
		if !responseCountsEqual(got, want) {
			t.Fatalf("attempt %d response = %#v, want %#v", attempt+1, got, want)
		}
	}
}

func responseCountsEqual(got, want map[string]int) bool {
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

func TestReceiverImportsValidCopilotOTLPWithReportedTokensAndReplayIdentity(t *testing.T) {
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

	payload := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"copilot-chat"}},{"key":"service.version","value":{"stringValue":"1.112.0"}},{"key":"session.id","value":{"stringValue":"window-1"}}]},"scopeSpans":[{"spans":[{"traceId":"trace-copilot","spanId":"span-copilot","startTimeUnixNano":"1763294400000000000","attributes":[{"key":"gen_ai.operation.name","value":{"stringValue":"chat"}},{"key":"gen_ai.provider.name","value":{"stringValue":"github"}},{"key":"gen_ai.agent.name","value":{"stringValue":"GitHub Copilot Chat"}},{"key":"gen_ai.request.model","value":{"stringValue":"gpt-5"}},{"key":"gen_ai.response.model","value":{"stringValue":"gpt-5-resolved"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"11"}},{"key":"gen_ai.usage.output_tokens","value":{"intValue":"13"}},{"key":"gen_ai.usage.cache_read.input_tokens","value":{"intValue":"17"}},{"key":"gen_ai.usage.cache_creation.input_tokens","value":{"intValue":"19"}},{"key":"gen_ai.usage.reasoning.output_tokens","value":{"intValue":"23"}},{"key":"qlog.cwd","value":{"stringValue":"` + filepath.ToSlash(worktree) + `"}},{"key":"github.copilot.git.branch","value":{"stringValue":"main"}},{"key":"github.copilot.git.commit_sha","value":{"stringValue":"abc123"}}]}]}]}]}`
	handler := NewHandler(service)
	for attempt, want := range []map[string]int{{"accepted": 1, "duplicates": 0}, {"accepted": 0, "duplicates": 1}} {
		request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
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
		if !responseCountsEqual(got, want) {
			t.Fatalf("attempt %d response = %#v, want %#v", attempt+1, got, want)
		}
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

func TestReceiverAcceptsOTLPProtobuf(t *testing.T) {
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

	payload := &collectortracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
			{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "copilot-chat"}}},
			{Key: "session.id", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "window-1"}}},
		}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
			TraceId:           []byte{1, 2, 3},
			SpanId:            []byte{4, 5},
			StartTimeUnixNano: uint64(time.Date(2026, 7, 21, 1, 0, 0, 0, time.UTC).UnixNano()),
			Attributes: []*commonpb.KeyValue{
				{Key: "qlog.project", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: project.Slug}}},
				{Key: "gen_ai.operation.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "chat"}}},
				{Key: "gen_ai.provider.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "github"}}},
				{Key: "gen_ai.agent.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GitHub Copilot Chat"}}},
				{Key: "gen_ai.request.model", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "gpt-5"}}},
				{Key: "gen_ai.response.model", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "gpt-5-resolved"}}},
				{Key: "gen_ai.usage.input_tokens", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 11}}},
				{Key: "gen_ai.usage.output_tokens", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 13}}},
			},
		}}}},
	}}}
	body, err := proto.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/x-protobuf")
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	report, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project", "agent", "capture_quality"}})
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if len(report.Rows) != 1 || report.Rows[0].AgentName != "GitHub Copilot Chat" || report.Rows[0].TotalTokens != 24 || report.Rows[0].CaptureQuality != "otel_reported" {
		t.Fatalf("usage rows = %#v", report.Rows)
	}
}

func TestReceiverRejectsInvalidCopilotOTLPIdentityAndSpoofedService(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	for _, payload := range []string{
		`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"copilot-chat"}}]},"scopeSpans":[{"spans":[{"traceId":"trace-only","attributes":[{"key":"gen_ai.agent.name","value":{"stringValue":"GitHub Copilot Chat"}},{"key":"gen_ai.request.model","value":{"stringValue":"gpt-5"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"1"}}]}]}]}]}`,
		`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"other-agent"}}]},"scopeSpans":[{"spans":[{"traceId":"trace","spanId":"span","attributes":[{"key":"gen_ai.agent.name","value":{"stringValue":"GitHub Copilot Chat"}},{"key":"gen_ai.request.model","value":{"stringValue":"gpt-5"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"1"}}]}]}]}]}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString(payload))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("response = %d: %s", response.Code, response.Body.String())
		}
	}
}

func TestReceiverRejectsNonJSON(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewBufferString("ignored"))
	request.Header.Set("Content-Type", "text/plain")
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

func TestReceiverDoesNotUseRemoteRepositoryURLAsWorkingDirectory(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	line, err := Receiver{service: service}.event(context.Background(), map[string]string{}, map[string]string{
		"gen_ai.provider.name":          "github",
		"gen_ai.request.model":          "gpt-5",
		"copilot_chat.repo.remote_url":  "https://oauth:secret@example.com/org/private.git?token=leak",
		"github.copilot.git.repository": "https://oauth:secret@example.com/org/private.git?token=leak",
	}, span{})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	encoded, err := json.Marshal(line["payload"])
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, forbidden := range []string{"oauth", "secret", "token=leak", "private.git"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("payload retained remote URL data %q: %s", forbidden, encoded)
		}
	}
}

func TestOTLPUsesTraceIDAsUpstreamIdentity(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	line, err := Receiver{service: service}.event(context.Background(), map[string]string{}, map[string]string{
		"gen_ai.provider.name": "example",
		"gen_ai.request.model": "model",
	}, span{TraceID: "trace-1"})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if line["upstream_event_id"] != "trace-1" {
		t.Fatalf("event = %#v", line)
	}
}

func TestCopilotOTLPRejectsTokenEvidenceWithoutTraceAndSpanIdentity(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("initialize service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	_, err = Receiver{service: service}.event(context.Background(), map[string]string{"service.name": "copilot-chat"}, map[string]string{
		"service.name":              "copilot-chat",
		"gen_ai.agent.name":         "GitHub Copilot Chat",
		"gen_ai.provider.name":      "github",
		"gen_ai.request.model":      "gpt-5",
		"gen_ai.usage.input_tokens": "1",
	}, span{})
	if err == nil {
		t.Fatal("Copilot token evidence without trace/span identity was accepted")
	}
}
