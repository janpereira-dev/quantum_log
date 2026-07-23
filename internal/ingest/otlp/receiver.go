// Package otlp receives a constrained, privacy-safe OTLP/HTTP trace subset.
package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/ingest/jsonl"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/proto"
)

const maxBodyBytes = 4 << 20

var errUnsupportedMediaType = errors.New("unsupported OTLP content type")

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
	payload, err := decodeTraceRequest(request, writer)
	if err != nil {
		http.Error(writer, err.Error(), statusForDecodeError(err))
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

func decodeTraceRequest(request *http.Request, writer http.ResponseWriter) (exportTraceServiceRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
	defer func() { _ = request.Body.Close() }()
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]))
	switch contentType {
	case "application/json":
		var payload exportTraceServiceRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return exportTraceServiceRequest{}, fmt.Errorf("decode OTLP JSON: %w", err)
		}
		return payload, nil
	case "application/x-protobuf", "application/protobuf":
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return exportTraceServiceRequest{}, fmt.Errorf("read OTLP protobuf: %w", err)
		}
		var payload collectortracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &payload); err != nil {
			return exportTraceServiceRequest{}, fmt.Errorf("decode OTLP protobuf: %w", err)
		}
		return fromProto(&payload), nil
	default:
		return exportTraceServiceRequest{}, errUnsupportedMediaType
	}
}

func statusForDecodeError(err error) int {
	if errors.Is(err, errUnsupportedMediaType) {
		return http.StatusUnsupportedMediaType
	}
	return http.StatusBadRequest
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
	imported, err := jsonl.ImportTrusted(ctx, r.service.Store, &lines)
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
	model := first(span, resource, "gen_ai.response.model", "gen_ai.request.model")
	eventType := "otel.span"
	if provider != "" && model != "" {
		eventType = "model.call"
	}
	occurredAt := fromUnixNano(input.StartTimeUnixNano)
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"provider":            provider,
		"model":               model,
		"agent_name":          first(span, resource, "gen_ai.agent.name", "service.name"),
		"input_tokens":        number(span, "gen_ai.usage.input_tokens", "gen_ai.usage.prompt_tokens"),
		"output_tokens":       number(span, "gen_ai.usage.output_tokens", "gen_ai.usage.completion_tokens"),
		"reasoning_tokens":    number(span, "gen_ai.usage.reasoning.output_tokens", "gen_ai.usage.reasoning_tokens"),
		"cached_input_tokens": number(span, "gen_ai.usage.cache_read.input_tokens"),
		"cache_write_tokens":  number(span, "gen_ai.usage.cache_creation.input_tokens"),
		"capture_quality":     "otel_reported",
		"working_directory":   resolved.CWD,
		"git_root":            first(span, resource, "qlog.git.root"),
		"git_branch":          first(span, resource, "github.copilot.git.branch", "copilot_chat.repo.head_branch_name", "vcs.ref.head.name"),
		"git_commit":          first(span, resource, "github.copilot.git.commit_sha", "copilot_chat.repo.head_commit_hash", "vcs.ref.head.revision"),
		"workspace":           first(span, resource, "qlog.workspace"),
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

func fromProto(input *collectortracepb.ExportTraceServiceRequest) exportTraceServiceRequest {
	output := exportTraceServiceRequest{ResourceSpans: make([]resourceSpans, 0, len(input.GetResourceSpans()))}
	for _, resourceSpan := range input.GetResourceSpans() {
		mappedResource := resourceSpans{Resource: resource{Attributes: fromProtoAttributes(resourceSpan.GetResource().GetAttributes())}}
		for _, scopeSpan := range resourceSpan.GetScopeSpans() {
			mappedScope := scopeSpans{Spans: make([]span, 0, len(scopeSpan.GetSpans()))}
			for _, protoSpan := range scopeSpan.GetSpans() {
				mappedScope.Spans = append(mappedScope.Spans, span{
					TraceID:           fmt.Sprintf("%x", protoSpan.GetTraceId()),
					StartTimeUnixNano: strconv.FormatUint(protoSpan.GetStartTimeUnixNano(), 10),
					Attributes:        fromProtoAttributes(protoSpan.GetAttributes()),
				})
			}
			mappedResource.ScopeSpans = append(mappedResource.ScopeSpans, mappedScope)
		}
		output.ResourceSpans = append(output.ResourceSpans, mappedResource)
	}
	return output
}

func fromProtoAttributes(values []*commonpb.KeyValue) []keyValue {
	result := make([]keyValue, 0, len(values))
	for _, value := range values {
		result = append(result, keyValue{Key: value.GetKey(), Value: fromProtoValue(value.GetValue())})
	}
	return result
}

func fromProtoValue(value *commonpb.AnyValue) attributeValue {
	switch typed := value.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return attributeValue{StringValue: typed.StringValue}
	case *commonpb.AnyValue_IntValue:
		return attributeValue{IntValue: json.Number(strconv.FormatInt(typed.IntValue, 10))}
	case *commonpb.AnyValue_DoubleValue:
		return attributeValue{StringValue: strconv.FormatFloat(typed.DoubleValue, 'f', -1, 64)}
	case *commonpb.AnyValue_BoolValue:
		return attributeValue{StringValue: strconv.FormatBool(typed.BoolValue)}
	default:
		return attributeValue{}
	}
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
