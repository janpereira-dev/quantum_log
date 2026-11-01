package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
	collectorlogpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func nativeCodexUsage() map[string]string {
	return map[string]string{"event.name": "codex.sse_event", "event.kind": "response.completed", "conversation.id": "synthetic-conversation", "model": "synthetic-model", "input_token_count": "20", "output_token_count": "8", "cached_token_count": "4", "cache_write_token_count": "2", "reasoning_token_count": "3", "tool_token_count": "28", "app.version": "0.162.1"}
}

func TestNativeCodexLogsWithoutTraceUseNativeIdentityAndInclusiveCounters(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	if _, _, err := service.Store.RegisterProject(ctx, "Synthetic", "synthetic", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	record := nativeCodexUsage()
	record["qlog.project"] = "synthetic"
	line, accepted, err := (Receiver{service: service}).codexLogEvent(ctx, map[string]string{"service.name": "codex"}, record, logRecord{TimeUnixNano: "1760000000000000000"})
	if err != nil || !accepted {
		t.Fatalf("native logs-only usage rejected: accepted=%t err=%v", accepted, err)
	}
	if line["trace_id"] != "" || line["span_id"] != "" {
		t.Fatalf("native trace context was fabricated: %v/%v", line["trace_id"], line["span_id"])
	}
	identity, _ := line["upstream_event_id"].(string)
	payload := line["payload"].(map[string]any)
	if !strings.HasPrefix(identity, "codex-log:") || payload["interaction_upstream_id"] != identity {
		t.Fatal("missing native content-free log identity")
	}
	if payload["input_tokens"] != int64(14) || payload["output_tokens"] != int64(5) || payload["cached_input_tokens"] != int64(4) || payload["cache_write_tokens"] != int64(2) || payload["reasoning_tokens"] != int64(3) {
		t.Fatalf("inclusive subsets double counted: %+v", payload)
	}
	if line["source_version"] != "0.162.1" {
		t.Fatalf("native version lost: %v", line["source_version"])
	}
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			record["input_tokens"] = "20"
			record["output_tokens"] = "8"
			record["prompt"] = "MOCK_PRIVATE_PROMPT"
		}
		wireAttrs := make([]keyValue, 0, len(record))
		for key, value := range record {
			wireAttrs = append(wireAttrs, keyValue{Key: key, Value: attributeValue{StringValue: value}})
		}
		wire, _ := json.Marshal(exportLogsServiceRequest{ResourceLogs: []resourceLogs{{Resource: resource{Attributes: []keyValue{{Key: "service.name", Value: attributeValue{StringValue: "codex"}}}}, ScopeLogs: []scopeLogs{{LogRecords: []logRecord{{TimeUnixNano: "1760000000000000000", Attributes: wireAttrs}}}}}}})
		request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(wire))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("attempt%d status=%d %s", attempt, response.Code, response.Body.String())
		}
		var result map[string]int
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["accepted"] != 1-attempt || result["duplicates"] != attempt {
			t.Fatalf("native/fullalias replay changed identity: %v", result)
		}
	}
	usage, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"project"}})
	if err != nil || usage.TotalTokens != 28 || len(usage.Rows) != 1 {
		t.Fatalf("native exact total changed: %+v %v", usage, err)
	}
}

func TestNativeCodexLogsRejectMissingIdentityAndInconsistentUsage(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	for _, test := range []struct{ name, key, value, time, trace, span string }{
		{name: "missing conversation", key: "conversation.id", time: "1760000000000000000"},
		{name: "missing input", key: "input_token_count", time: "1760000000000000000"},
		{name: "missing output", key: "output_token_count", time: "1760000000000000000"},
		{name: "negative input", key: "input_token_count", value: "-1", time: "1760000000000000000"},
		{name: "malformed output", key: "output_token_count", value: "invalid", time: "1760000000000000000"},
		{name: "cached exceeds input", key: "cached_token_count", value: "21", time: "1760000000000000000"},
		{name: "cache write overlaps input", key: "cache_write_token_count", value: "17", time: "1760000000000000000"},
		{name: "reasoning exceeds output", key: "reasoning_token_count", value: "9", time: "1760000000000000000"},
		{name: "total mismatch", key: "tool_token_count", value: "30", time: "1760000000000000000"},
		{name: "negative time", time: "-1"}, {name: "missing time"}, {name: "zero time", time: "0"}, {name: "malformed time", time: "invalid"},
		{name: "partial trace", time: "1760000000000000000", trace: "trace"}, {name: "partial span", time: "1760000000000000000", span: "span"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := nativeCodexUsage()
			if test.key != "" {
				if test.value == "" {
					delete(record, test.key)
				} else {
					record[test.key] = test.value
				}
			}
			_, accepted, err := (Receiver{service: service}).codexLogEvent(ctx, map[string]string{"service.name": "codex"}, record, logRecord{TimeUnixNano: test.time, TraceID: test.trace, SpanID: test.span})
			if err != nil || accepted {
				t.Fatalf("invalid native record accepted=%t err=%v", accepted, err)
			}
		})
	}
}

func TestNativeCodexLogsKeepMissingCountersDistinctFromZero(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	record := nativeCodexUsage()
	record["input_token_count"] = "0"
	record["output_token_count"] = "0"
	delete(record, "cached_token_count")
	delete(record, "cache_write_token_count")
	delete(record, "reasoning_token_count")
	delete(record, "tool_token_count")
	line, accepted, err := (Receiver{service: service}).codexLogEvent(ctx, map[string]string{"service.name": "codex"}, record, logRecord{TimeUnixNano: "1760000000000000000"})
	if err != nil || !accepted {
		t.Fatalf("source zero rejected: %t %v", accepted, err)
	}
	payload := line["payload"].(map[string]any)
	if payload["input_tokens"] != int64(0) || payload["output_tokens"] != int64(0) {
		t.Fatal("source zero lost")
	}
	if _, ok := payload["cached_input_tokens"]; ok {
		t.Fatal("absent cached count manufactured")
	}
	if got := len(payload["metric_observations"].([]map[string]any)); got != 2 {
		t.Fatalf("unreported metrics manufactured: %d", got)
	}
}

func TestNativeCodexExecUsesSourceTimestampAndPreservesNativeObservedPrecision(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	record := nativeCodexUsage()
	record["event.timestamp"] = "2026-11-01T19:44:56.057Z"
	attrs := make([]keyValue, 0, len(record))
	for key, value := range record {
		attrs = append(attrs, keyValue{Key: key, Value: attributeValue{StringValue: value}})
	}
	for attempt, observed := range []string{"1793562296057358600", "1793562296057358600", "1793562296057358601"} {
		wire, _ := json.Marshal(map[string]any{"resourceLogs": []any{map[string]any{"resource": resource{Attributes: []keyValue{{Key: "service.name", Value: attributeValue{StringValue: "codex_exec"}}}}, "scopeLogs": []any{map[string]any{"logRecords": []any{map[string]any{"timeUnixNano": "0", "observedTimeUnixNano": observed, "eventName": `event otel\src\events\session_telemetry.rs:1132`, "attributes": attrs}}}}}}})
		request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(wire))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("actual native shape rejected: %d %s", response.Code, response.Body.String())
		}
		var result map[string]int
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		wantAccepted, wantDuplicates := 1, 0
		if attempt == 1 {
			wantAccepted, wantDuplicates = 0, 1
		}
		if result["accepted"] != wantAccepted || result["duplicates"] != wantDuplicates {
			t.Fatalf("native observed precision/replay lost: attempt=%d result=%v", attempt, result)
		}
	}
	usage, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"model"}})
	if err != nil || usage.TotalTokens != 56 {
		t.Fatalf("distinct same-millisecond native calls collapsed: %+v %v", usage, err)
	}
	for _, serviceName := range []string{"other", "fake-codex_exec", "codex_exec-untrusted"} {
		if _, accepted, err := (Receiver{service: service}).codexLogEvent(ctx, map[string]string{"service.name": serviceName}, record, logRecord{TimeUnixNano: "0"}); err != nil || accepted {
			t.Fatalf("arbitrary service accepted: %q %t %v", serviceName, accepted, err)
		}
	}
	for _, at := range []string{"", "invalid", "0001-01-01T00:00:00Z", "1970-01-01T00:00:00Z", "9999-12-31T00:00:00Z"} {
		record["event.timestamp"] = at
		if _, accepted, err := (Receiver{service: service}).codexLogEvent(ctx, map[string]string{"service.name": "codex_exec"}, record, logRecord{TimeUnixNano: "0", ObservedTimeUnixNano: "1793562296057358600"}); err != nil || accepted {
			t.Fatalf("missing/impossible source time accepted: %q %t %v", at, accepted, err)
		}
	}
}

func TestNativeCodexObservedTimeDeduplicatesJSONAndProtobufAndRejectsMissingPrecision(t *testing.T) {
	ctx := context.Background()
	service, err := app.Initialize(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	record := nativeCodexUsage()
	record["event.timestamp"] = "2026-11-01T19:44:56.057Z"
	for _, observed := range []string{"", "0", "-1", "invalid", "18446744073709551615"} {
		if _, accepted, err := (Receiver{service: service}).codexLogEvent(ctx, map[string]string{"service.name": "codex_exec"}, record, logRecord{TimeUnixNano: "0", ObservedTimeUnixNano: observed}); err != nil || accepted {
			t.Fatalf("ambiguous or invalid native precision accepted: %q %t %v", observed, accepted, err)
		}
	}
	attrs := make([]keyValue, 0, len(record))
	protoAttrs := make([]*commonpb.KeyValue, 0, len(record))
	for key, value := range record {
		attrs = append(attrs, keyValue{Key: key, Value: attributeValue{StringValue: value}})
		protoAttrs = append(protoAttrs, &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}})
	}
	jsonWire, _ := json.Marshal(exportLogsServiceRequest{ResourceLogs: []resourceLogs{{Resource: resource{Attributes: []keyValue{{Key: "service.name", Value: attributeValue{StringValue: "codex_exec"}}}}, ScopeLogs: []scopeLogs{{LogRecords: []logRecord{{TimeUnixNano: "0", ObservedTimeUnixNano: "1793562296057358600", Attributes: attrs}}}}}}})
	protoWire, err := proto.Marshal(&collectorlogpb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "codex_exec"}}}}}, ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{{ObservedTimeUnixNano: 1793562296057358600, Attributes: protoAttrs}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []struct {
		contentType string
		body        []byte
	}{{"application/json", jsonWire}, {"application/x-protobuf", protoWire}, {"application/json", jsonWire}} {
		request := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(message.body))
		request.Header.Set("Content-Type", message.contentType)
		response := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("cross-encoding replay failed: %d %s", response.Code, response.Body.String())
		}
	}
	usage, err := service.Store.Usage(ctx, storepkg.UsageQuery{GroupBy: []string{"model"}})
	if err != nil || usage.TotalTokens != 28 || len(usage.Rows) != 1 {
		t.Fatalf("same native record counted twice across encodings: %+v %v", usage, err)
	}
	capability, err := service.Store.CapabilityReport(ctx, storepkg.CapabilityQuery{})
	if err != nil || capability.ModelCalls != 1 || capability.Interactions != 1 {
		t.Fatalf("native replay created additional roots/calls: %+v %v", capability, err)
	}
}
