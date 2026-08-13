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
	collectorlogpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/proto"
)

const maxBodyBytes = 4 << 20

var errUnsupportedMediaType = errors.New("unsupported OTLP content type")
var errUnsupportedCodexLog = errors.New("unsupported Codex OTLP log record")
var errUnsupportedCopilotSpan = errors.New("unsupported Copilot OTLP span")
var errUnsupportedClaudeSpan = errors.New("unsupported Claude Code OTLP span")

type Receiver struct {
	service *app.Service
}

func NewHandler(service *app.Service) http.Handler { return Receiver{service: service} }

func (r Receiver) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/v1/traces" && request.URL.Path != "/v1/logs" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method must be POST", http.StatusMethodNotAllowed)
		return
	}
	count, total, err := r.ingestRequest(request.Context(), request, writer)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errUnsupportedMediaType) {
			status = statusForDecodeError(err)
		}
		http.Error(writer, err.Error(), status)
		return
	}
	if isProtobufRequest(request) {
		writer.Header().Set("Content-Type", "application/x-protobuf")
		var response proto.Message
		if request.URL.Path == "/v1/logs" {
			response = &collectorlogpb.ExportLogsServiceResponse{}
		} else {
			response = &collectortracepb.ExportTraceServiceResponse{}
		}
		encoded, err := proto.Marshal(response)
		if err != nil {
			http.Error(writer, "encode OTLP protobuf response: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write(encoded)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]int{"accepted": count, "duplicates": total - count})
}

func isProtobufRequest(request *http.Request) bool {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]))
	return contentType == "application/x-protobuf" || contentType == "application/protobuf"
}

func (r Receiver) ingestRequest(ctx context.Context, request *http.Request, writer http.ResponseWriter) (int, int, error) {
	if request.URL.Path == "/v1/traces" {
		payload, err := decodeTraceRequest(request, writer)
		if err != nil {
			return 0, 0, err
		}
		accepted, total, err := r.ingest(ctx, payload)
		return accepted, total, err
	}
	payload, err := decodeLogRequest(request, writer)
	if err != nil {
		return 0, 0, err
	}
	count, err := r.ingestLogs(ctx, payload)
	return count, logCount(payload), err
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

func decodeLogRequest(request *http.Request, writer http.ResponseWriter) (exportLogsServiceRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
	defer func() { _ = request.Body.Close() }()
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]))
	switch contentType {
	case "application/json":
		var payload exportLogsServiceRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return exportLogsServiceRequest{}, fmt.Errorf("decode OTLP logs JSON: %w", err)
		}
		return payload, nil
	case "application/x-protobuf", "application/protobuf":
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return exportLogsServiceRequest{}, fmt.Errorf("read OTLP logs protobuf: %w", err)
		}
		var payload collectorlogpb.ExportLogsServiceRequest
		if err := proto.Unmarshal(body, &payload); err != nil {
			return exportLogsServiceRequest{}, fmt.Errorf("decode OTLP logs protobuf: %w", err)
		}
		return logsFromProto(&payload), nil
	default:
		return exportLogsServiceRequest{}, errUnsupportedMediaType
	}
}

func (r Receiver) ingest(ctx context.Context, request exportTraceServiceRequest) (int, int, error) {
	var lines bytes.Buffer
	count := 0
	var unsupportedAgentSpan error
	for _, resourceSpan := range request.ResourceSpans {
		resource := attributes(resourceSpan.Resource.Attributes)
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				line, err := r.event(ctx, resource, attributes(span.Attributes), span)
				if errors.Is(err, errUnsupportedCopilotSpan) || errors.Is(err, errUnsupportedClaudeSpan) {
					unsupportedAgentSpan = err
					continue
				}
				if err != nil {
					return count, count, err
				}
				if err := json.NewEncoder(&lines).Encode(line); err != nil {
					return count, count, err
				}
				count++
			}
		}
	}
	if count == 0 {
		if unsupportedAgentSpan != nil {
			return 0, 0, unsupportedAgentSpan
		}
		return 0, 0, nil
	}
	imported, err := jsonl.ImportTrusted(ctx, r.service.Store, &lines)
	if err != nil {
		return 0, count, fmt.Errorf("import OTLP spans: %w", err)
	}
	return imported, count, nil
}

func (r Receiver) ingestLogs(ctx context.Context, request exportLogsServiceRequest) (int, error) {
	var lines bytes.Buffer
	count := 0
	for _, resourceLog := range request.ResourceLogs {
		resource := attributes(resourceLog.Resource.Attributes)
		for _, scopeLog := range resourceLog.ScopeLogs {
			for _, record := range scopeLog.LogRecords {
				line, ok, err := r.codexLogEvent(ctx, resource, attributes(record.Attributes), record)
				if err != nil {
					return count, err
				}
				if !ok {
					continue
				}
				if err := json.NewEncoder(&lines).Encode(line); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	if count == 0 {
		if logCount(request) > 0 {
			return 0, errUnsupportedCodexLog
		}
		return 0, nil
	}
	imported, err := jsonl.ImportTrusted(ctx, r.service.Store, &lines)
	if err != nil {
		return 0, fmt.Errorf("import OTLP logs: %w", err)
	}
	return imported, nil
}

func (r Receiver) event(ctx context.Context, resource, span map[string]string, input span) (map[string]any, error) {
	if isCopilotTelemetryCandidate(resource, span) {
		return r.copilotSpanEvent(ctx, resource, span, input)
	}
	if isClaudeTelemetryCandidate(resource, span) {
		return r.claudeSpanEvent(ctx, resource, span, input)
	}
	cwd := first(span, resource, "process.cwd", "qlog.cwd")
	adapterProject := first(span, resource, "qlog.project")
	resolved, err := r.service.ResolveProject(ctx, "", adapterProject, cwd)
	if err != nil {
		return nil, err
	}
	provider := first(span, resource, "gen_ai.provider.name", "gen_ai.system")
	model := first(span, resource, "gen_ai.response.model", "gen_ai.request.model")
	eventType := "otel.span"
	if isInteractionRoot(input.Name, span) {
		eventType = "interaction.prompt"
	}
	if provider != "" && model != "" && eventType != "interaction.prompt" {
		eventType = "model.call"
	}
	occurredAt := fromUnixNano(input.StartTimeUnixNano)
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := map[string]any{
		"provider":        provider,
		"model":           model,
		"agent_name":      first(span, resource, "gen_ai.agent.name", "service.name"),
		"capture_quality": "otel_reported",
	}
	observations := metricObservations(span, "otel", []metricKeySet{
		{name: "input_tokens", keys: []string{"gen_ai.usage.input_tokens", "gen_ai.usage.prompt_tokens"}},
		{name: "output_tokens", keys: []string{"gen_ai.usage.output_tokens", "gen_ai.usage.completion_tokens"}},
		{name: "reasoning_tokens", keys: []string{"gen_ai.usage.reasoning.output_tokens", "gen_ai.usage.reasoning_tokens"}},
		{name: "cached_input_tokens", keys: []string{"gen_ai.usage.cache_read.input_tokens"}},
		{name: "cache_write_tokens", keys: []string{"gen_ai.usage.cache_creation.input_tokens"}},
		{name: "total_tokens", keys: []string{"gen_ai.usage.total_tokens"}},
	})
	for _, observation := range observations {
		payload[observation["name"].(string)] = observation["value"]
	}
	if len(observations) > 0 {
		payload["metric_observations"] = observations
	}
	sessionID := first(span, resource, "session.id", "gen_ai.conversation.id")
	if sessionID == "" {
		sessionID = input.TraceID
	}
	line := map[string]any{
		"source":                        "otlp-http",
		"source_version":                first(resource, span, "service.version"),
		"session_id":                    sessionID,
		"event_type":                    eventType,
		"occurred_at":                   occurredAt,
		"project_id":                    resolved.ProjectID,
		"project_location_id":           resolved.LocationID,
		"project_resolution_method":     string(resolved.Resolution.Method),
		"project_resolution_confidence": string(resolved.Resolution.Confidence),
		"project_resolution_evidence":   map[string]string{"source": "central-project-resolver"},
		"payload":                       payload,
	}
	if upstreamEventID := otlpUpstreamEventID(input); upstreamEventID != "" {
		line["upstream_event_id"] = upstreamEventID
	}
	return line, nil
}

func (r Receiver) claudeSpanEvent(ctx context.Context, resource, attributes map[string]string, input span) (map[string]any, error) {
	if resource["service.name"] != "claude-code" || input.TraceID == "" || input.SpanID == "" {
		return nil, errUnsupportedClaudeSpan
	}
	model := first(attributes, resource, "model", "gen_ai.response.model", "gen_ai.request.model")
	if model == "" {
		return nil, errUnsupportedClaudeSpan
	}
	payload := map[string]any{"provider": "anthropic", "model": model, "agent_name": "claude-code", "capture_quality": "otel_reported"}
	observations := metricObservations(attributes, "otel", []metricKeySet{
		{name: "input_tokens", keys: []string{"input_tokens", "gen_ai.usage.input_tokens"}},
		{name: "output_tokens", keys: []string{"output_tokens", "gen_ai.usage.output_tokens"}},
		{name: "cached_input_tokens", keys: []string{"cache_read_input_tokens", "gen_ai.usage.cache_read.input_tokens"}},
		{name: "cache_write_tokens", keys: []string{"cache_creation_input_tokens", "gen_ai.usage.cache_creation.input_tokens"}},
	})
	if len(observations) == 0 {
		return nil, errUnsupportedClaudeSpan
	}
	for _, observation := range observations {
		payload[observation["name"].(string)] = observation["value"]
	}
	payload["metric_observations"] = observations
	resolved, err := r.service.ResolveProject(ctx, "", first(attributes, resource, "qlog.project"), first(attributes, resource, "process.cwd", "qlog.cwd"))
	if err != nil {
		return nil, err
	}
	occurredAt := fromUnixNano(input.StartTimeUnixNano)
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	sessionID := first(attributes, resource, "session.id")
	if sessionID == "" {
		sessionID = input.TraceID
	}
	return map[string]any{
		"source": "otlp-http", "source_version": first(resource, attributes, "service.version"), "session_id": sessionID, "event_type": "model.call", "occurred_at": occurredAt,
		"project_id": resolved.ProjectID, "project_location_id": resolved.LocationID,
		"project_resolution_method": string(resolved.Resolution.Method), "project_resolution_confidence": string(resolved.Resolution.Confidence),
		"project_resolution_evidence": map[string]string{"source": "central-project-resolver"},
		"upstream_event_id":           input.TraceID + "/" + input.SpanID, "payload": payload,
	}, nil
}

func (r Receiver) copilotSpanEvent(ctx context.Context, resource, attributes map[string]string, input span) (map[string]any, error) {
	serviceName := resource["service.name"]
	agentName := attributes["gen_ai.agent.name"]
	if input.TraceID == "" || input.SpanID == "" || (serviceName != "copilot-chat" && serviceName != "github-copilot") || (serviceName == "copilot-chat" && agentName != "GitHub Copilot Chat") {
		return nil, errUnsupportedCopilotSpan
	}
	if agentName == "" {
		agentName = "GitHub Copilot CLI"
	}
	interactionRoot := isInteractionRoot(input.Name, attributes)
	model := first(attributes, resource, "gen_ai.response.model", "gen_ai.request.model")
	if model == "" && !interactionRoot {
		return nil, errUnsupportedCopilotSpan
	}
	payload := map[string]any{
		"provider":        first(attributes, resource, "gen_ai.provider.name", "gen_ai.system"),
		"model":           model,
		"agent_name":      agentName,
		"capture_quality": "otel_reported",
	}
	if payload["provider"] == "" {
		payload["provider"] = "github"
	}
	observations := metricObservations(attributes, "otel", []metricKeySet{
		{name: "input_tokens", keys: []string{"gen_ai.usage.input_tokens", "gen_ai.usage.prompt_tokens"}},
		{name: "output_tokens", keys: []string{"gen_ai.usage.output_tokens", "gen_ai.usage.completion_tokens"}},
		{name: "reasoning_tokens", keys: []string{"gen_ai.usage.reasoning.output_tokens", "gen_ai.usage.reasoning_tokens"}},
		{name: "cached_input_tokens", keys: []string{"gen_ai.usage.cache_read.input_tokens"}},
		{name: "cache_write_tokens", keys: []string{"gen_ai.usage.cache_creation.input_tokens"}},
		{name: "total_tokens", keys: []string{"gen_ai.usage.total_tokens"}},
	})
	if len(observations) == 0 && !interactionRoot {
		return nil, errUnsupportedCopilotSpan
	}
	for _, observation := range observations {
		payload[observation["name"].(string)] = observation["value"]
	}
	if len(observations) > 0 {
		payload["metric_observations"] = observations
	}
	resolved, err := r.resolveCopilotProject(ctx, resource, attributes)
	if err != nil {
		return nil, err
	}
	occurredAt := fromUnixNano(input.StartTimeUnixNano)
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	sessionID := first(attributes, resource, "gen_ai.conversation.id", "session.id")
	if sessionID == "" {
		sessionID = input.TraceID
	}
	if windowSessionID := first(attributes, resource, "session.id"); windowSessionID != "" && windowSessionID != sessionID {
		payload["copilot_window_session_id"] = windowSessionID
	}
	// Copilot's trace is the only transport-independent turn identity present on
	// every exported span. It creates a single privacy-safe root even when a
	// CLI hook is disabled or an exporter omits invoke_agent.
	payload["interaction_upstream_id"] = input.TraceID
	eventType := "model.call"
	upstreamEventID := input.TraceID + "/" + input.SpanID
	if interactionRoot {
		eventType = "interaction.prompt"
		upstreamEventID = input.TraceID
		payload["prompt_available"] = false
		payload["prompt_capture_mode"] = "hash"
	}
	return map[string]any{
		"source":                        "otlp-http",
		"source_version":                first(resource, attributes, "service.version"),
		"session_id":                    sessionID,
		"event_type":                    eventType,
		"occurred_at":                   occurredAt,
		"project_id":                    resolved.ProjectID,
		"project_location_id":           resolved.LocationID,
		"project_resolution_method":     string(resolved.Resolution.Method),
		"project_resolution_confidence": string(resolved.Resolution.Confidence),
		"project_resolution_evidence":   map[string]string{"source": "central-project-resolver"},
		"upstream_event_id":             upstreamEventID,
		"payload":                       payload,
	}, nil
}
func (r Receiver) resolveCopilotProject(ctx context.Context, resource, attributes map[string]string) (app.ResolvedProject, error) {
	if adapterProject := first(attributes, resource, "qlog.project"); adapterProject != "" {
		return r.service.ResolveProject(ctx, "", adapterProject, "")
	}
	cwd := first(attributes, resource, "process.cwd", "qlog.cwd")
	if cwd != "" {
		return r.service.ResolveProject(ctx, "", "", cwd)
	}
	return r.service.ResolveProjectFromVerifiedGitContext(ctx,
		first(attributes, resource, "github.copilot.git.root", "github.copilot.git.repository_root"),
		first(attributes, resource, "github.copilot.git.repository", "github.copilot.git.remote_url"),
	)
}

func codexLogIdentity(record map[string]string, input logRecord) string {
	identity := input.TraceID + "/" + input.SpanID
	if responseID := record["response.id"]; responseID != "" {
		return identity + "/" + responseID
	}
	return identity
}

func otlpUpstreamEventID(input span) string {
	if input.TraceID == "" {
		return ""
	}
	if input.SpanID == "" {
		return input.TraceID
	}
	return input.TraceID + "/" + input.SpanID
}

func (r Receiver) codexLogEvent(ctx context.Context, resource, record map[string]string, input logRecord) (map[string]any, bool, error) {
	if resource["service.name"] != "codex" || record["event.name"] != "codex.sse_event" || record["event.kind"] != "response.completed" || input.TraceID == "" || input.SpanID == "" {
		return nil, false, nil
	}
	model := record["model"]
	inputTokens, hasInput := requiredNumber(record, "input_tokens")
	outputTokens, hasOutput := requiredNumber(record, "output_tokens")
	if model == "" || !hasInput || !hasOutput {
		return nil, false, nil
	}
	resolved, err := r.service.ResolveProject(ctx, "", record["qlog.project"], first(record, resource, "process.cwd", "qlog.cwd"))
	if err != nil {
		return nil, false, err
	}
	payload := map[string]any{
		"provider":                 "openai",
		"model":                    model,
		"agent_name":               "codex",
		"capture_quality":          "otel_reported",
		"codex_response_completed": true,
		"input_tokens":             inputTokens,
		"output_tokens":            outputTokens,
	}
	observations := []map[string]any{
		{"name": "input_tokens", "value": inputTokens, "source": "otel", "raw_key": "input_tokens", "confidence": "reported"},
		{"name": "output_tokens", "value": outputTokens, "source": "otel", "raw_key": "output_tokens", "confidence": "reported"},
	}
	if value, found := requiredNumber(record, "cached_input_tokens"); found {
		payload["cached_input_tokens"] = value
		observations = append(observations, map[string]any{"name": "cached_input_tokens", "value": value, "source": "otel", "raw_key": "cached_input_tokens", "confidence": "reported"})
	}
	if value, found := requiredNumber(record, "reasoning_output_tokens"); found {
		payload["reasoning_tokens"] = value
		observations = append(observations, map[string]any{"name": "reasoning_tokens", "value": value, "source": "otel", "raw_key": "reasoning_output_tokens", "confidence": "reported"})
	}
	payload["metric_observations"] = observations
	occurredAt := fromUnixNano(input.TimeUnixNano)
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	sessionID := first(record, resource, "conversation.id")
	if sessionID == "" {
		sessionID = input.TraceID
	}
	return map[string]any{
		"source":                        "otlp-http",
		"source_version":                first(resource, record, "service.version"),
		"session_id":                    sessionID,
		"event_type":                    "model.call",
		"occurred_at":                   occurredAt,
		"project_id":                    resolved.ProjectID,
		"project_location_id":           resolved.LocationID,
		"project_resolution_method":     string(resolved.Resolution.Method),
		"project_resolution_confidence": string(resolved.Resolution.Confidence),
		"project_resolution_evidence":   map[string]string{"source": "central-project-resolver"},
		"upstream_event_id":             codexLogIdentity(record, input),
		"payload":                       payload,
	}, true, nil
}

func isCopilotTelemetryCandidate(resource, attributes map[string]string) bool {
	return strings.Contains(strings.ToLower(resource["service.name"]), "copilot") || strings.Contains(strings.ToLower(attributes["gen_ai.agent.name"]), "copilot")
}

func isClaudeTelemetryCandidate(resource, attributes map[string]string) bool {
	return resource["service.name"] == "claude-code" || attributes["agent.name"] == "claude-code"
}

type metricKeySet struct {
	name string
	keys []string
}

func metricObservations(values map[string]string, source string, sets []metricKeySet) []map[string]any {
	result := make([]map[string]any, 0, len(sets))
	for _, set := range sets {
		for _, key := range set.keys {
			value, found := optionalNumber(values, key)
			if !found {
				continue
			}
			result = append(result, map[string]any{"name": set.name, "value": value, "source": source, "raw_key": key, "confidence": "reported"})
			break
		}
	}
	return result
}

type exportTraceServiceRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type exportLogsServiceRequest struct {
	ResourceLogs []resourceLogs `json:"resourceLogs"`
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
type resourceLogs struct {
	Resource  resource    `json:"resource"`
	ScopeLogs []scopeLogs `json:"scopeLogs"`
}
type scopeLogs struct {
	LogRecords []logRecord `json:"logRecords"`
}
type span struct {
	Name              string     `json:"name"`
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	Attributes        []keyValue `json:"attributes"`
}
type logRecord struct {
	TraceID      string     `json:"traceId"`
	SpanID       string     `json:"spanId"`
	TimeUnixNano string     `json:"timeUnixNano"`
	Attributes   []keyValue `json:"attributes"`
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
					Name:              protoSpan.GetName(),
					TraceID:           fmt.Sprintf("%x", protoSpan.GetTraceId()),
					SpanID:            fmt.Sprintf("%x", protoSpan.GetSpanId()),
					ParentSpanID:      fmt.Sprintf("%x", protoSpan.GetParentSpanId()),
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

func isInteractionRoot(name string, attributes map[string]string) bool {
	operation := strings.ToLower(first(attributes, map[string]string{}, "gen_ai.operation.name", "operation.name", "span.kind"))
	name = strings.ToLower(name)
	return operation == "invoke_agent" || strings.Contains(name, "invoke_agent")
}

func logsFromProto(input *collectorlogpb.ExportLogsServiceRequest) exportLogsServiceRequest {
	output := exportLogsServiceRequest{ResourceLogs: make([]resourceLogs, 0, len(input.GetResourceLogs()))}
	for _, resourceLog := range input.GetResourceLogs() {
		mappedResource := resourceLogs{Resource: resource{Attributes: fromProtoAttributes(resourceLog.GetResource().GetAttributes())}}
		for _, scopeLog := range resourceLog.GetScopeLogs() {
			mappedScope := scopeLogs{LogRecords: make([]logRecord, 0, len(scopeLog.GetLogRecords()))}
			for _, protoRecord := range scopeLog.GetLogRecords() {
				mappedScope.LogRecords = append(mappedScope.LogRecords, logRecord{
					TraceID:      fmt.Sprintf("%x", protoRecord.GetTraceId()),
					SpanID:       fmt.Sprintf("%x", protoRecord.GetSpanId()),
					TimeUnixNano: strconv.FormatUint(protoRecord.GetTimeUnixNano(), 10),
					Attributes:   fromProtoAttributes(protoRecord.GetAttributes()),
				})
			}
			mappedResource.ScopeLogs = append(mappedResource.ScopeLogs, mappedScope)
		}
		output.ResourceLogs = append(output.ResourceLogs, mappedResource)
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

func requiredNumber(values map[string]string, key string) (int64, bool) {
	value, err := strconv.ParseInt(values[key], 10, 64)
	return value, err == nil && value >= 0
}

func optionalNumber(values map[string]string, keys ...string) (int64, bool) {
	for _, key := range keys {
		if values[key] == "" {
			continue
		}
		return requiredNumber(values, key)
	}
	return 0, false
}

func logCount(request exportLogsServiceRequest) int {
	count := 0
	for _, resourceLog := range request.ResourceLogs {
		for _, scopeLog := range resourceLog.ScopeLogs {
			count += len(scopeLog.LogRecords)
		}
	}
	return count
}

func fromUnixNano(value string) time.Time {
	nanoseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, nanoseconds).UTC()
}
