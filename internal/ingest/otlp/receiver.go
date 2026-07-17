// Package otlp receives a constrained, privacy-safe OTLP/HTTP trace subset.
package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/ingest/jsonl"
)

const maxBodyBytes = 4 << 20

type Receiver struct {
	service *app.Service
}

func NewHandler(service *app.Service) http.Handler { return Receiver{service: service} }

func (r Receiver) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/v1/traces" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method must be POST", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(writer, "only OTLP JSON is supported", http.StatusUnsupportedMediaType)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
	defer func() { _ = request.Body.Close() }()
	var payload exportTraceServiceRequest
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&payload); err != nil {
		http.Error(writer, "decode OTLP JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	count, err := r.ingest(request.Context(), payload)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]int{"accepted": count})
}

func (r Receiver) ingest(ctx context.Context, request exportTraceServiceRequest) (int, error) {
	var lines bytes.Buffer
	count := 0
	for _, resourceSpan := range request.ResourceSpans {
		resource := attributes(resourceSpan.Resource.Attributes)
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				line, err := r.event(ctx, resource, attributes(span.Attributes), span)
				if err != nil {
					return count, err
				}
				if err := json.NewEncoder(&lines).Encode(line); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	if count == 0 {
		return 0, nil
	}
	imported, err := jsonl.Import(ctx, r.service.Store, &lines)
	if err != nil {
		return 0, fmt.Errorf("import OTLP spans: %w", err)
	}
	return imported, nil
}

func (r Receiver) event(ctx context.Context, resource, span map[string]string, input span) (map[string]any, error) {
	cwd := first(span, resource, "process.cwd", "qlog.cwd")
	adapterProject := first(span, resource, "qlog.project")
	resolved, err := r.service.ResolveProject(ctx, "", adapterProject, cwd)
	if err != nil {
		return nil, err
	}
	provider := first(span, resource, "gen_ai.provider.name", "gen_ai.system")
	model := first(span, resource, "gen_ai.request.model", "gen_ai.response.model")
	eventType := "otel.span"
	if provider != "" && model != "" {
		eventType = "model.call"
	}
	occurredAt := fromUnixNano(input.StartTimeUnixNano)
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"provider":          provider,
		"model":             model,
		"agent_name":        first(resource, span, "service.name"),
		"input_tokens":      number(span, "gen_ai.usage.input_tokens", "gen_ai.usage.prompt_tokens"),
		"output_tokens":     number(span, "gen_ai.usage.output_tokens", "gen_ai.usage.completion_tokens"),
		"capture_quality":   "otel_reported",
		"working_directory": resolved.CWD,
		"git_root":          first(span, resource, "qlog.git.root"),
		"git_branch":        first(span, resource, "vcs.ref.head.name"),
		"workspace":         first(span, resource, "qlog.workspace"),
	}
	sessionID := first(span, resource, "session.id", "gen_ai.conversation.id")
	if sessionID == "" {
		sessionID = input.TraceID
	}
	return map[string]any{
		"source":                        "otlp-http",
		"session_id":                    sessionID,
		"event_type":                    eventType,
		"occurred_at":                   occurredAt,
		"project_id":                    resolved.ProjectID,
		"project_location_id":           resolved.LocationID,
		"project_resolution_method":     string(resolved.Resolution.Method),
		"project_resolution_confidence": string(resolved.Resolution.Confidence),
		"project_resolution_evidence":   map[string]string{"source": "central-project-resolver"},
		"payload":                       payload,
	}, nil
}

type exportTraceServiceRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	Resource   resource     `json:"resource"`
	ScopeSpans []scopeSpans `json:"scopeSpans"`
}

type resource struct {
	Attributes []keyValue `json:"attributes"`
}
type scopeSpans struct {
	Spans []span `json:"spans"`
}
type span struct {
	TraceID           string     `json:"traceId"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	Attributes        []keyValue `json:"attributes"`
}
type keyValue struct {
	Key   string         `json:"key"`
	Value attributeValue `json:"value"`
}
type attributeValue struct {
	StringValue string      `json:"stringValue"`
	IntValue    json.Number `json:"intValue"`
}

func attributes(values []keyValue) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		if value.Value.StringValue != "" {
			result[value.Key] = value.Value.StringValue
		} else if value.Value.IntValue != "" {
			result[value.Key] = string(value.Value.IntValue)
		}
	}
	return result
}

func first(primary, fallback map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := primary[key]; value != "" {
			return value
		}
		if value := fallback[key]; value != "" {
			return value
		}
	}
	return ""
}

func number(values map[string]string, keys ...string) int64 {
	for _, key := range keys {
		if value, err := strconv.ParseInt(values[key], 10, 64); err == nil {
			return value
		}
	}
	return 0
}

func fromUnixNano(value string) time.Time {
	nanoseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, nanoseconds).UTC()
}
