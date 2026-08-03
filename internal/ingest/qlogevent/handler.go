// Package qlogevent receives sanitized local events from qlog plugins and hooks.
package qlogevent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/ingest/jsonl"
)

const maxEventBodyBytes = 1 << 20

type Handler struct {
	service *app.Service
}

func NewHandler(service *app.Service) http.Handler { return Handler{service: service} }

func (h Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/v1/events" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method must be POST", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(writer, "only JSON is supported", http.StatusUnsupportedMediaType)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxEventBodyBytes)
	defer func() { _ = request.Body.Close() }()
	var event Event
	if err := json.NewDecoder(request.Body).Decode(&event); err != nil {
		http.Error(writer, "decode event: "+err.Error(), http.StatusBadRequest)
		return
	}
	count, err := Ingest(request.Context(), h.service, event)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]int{"accepted": count, "duplicates": 1 - count})
}

func Ingest(ctx context.Context, service *app.Service, event Event) (int, error) {
	resolved, err := service.ResolveProject(ctx, event.ProjectHint.Project, "", event.ProjectHint.CWD)
	if err != nil {
		return 0, err
	}
	if event.Source == "" {
		event.Source = "qlog-plugin"
	}
	if event.EventType == "" {
		event.EventType = "agent.event"
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	event = normalizeCodexRawResponse(event)
	payload := sanitizePluginPayload(event.Payload)
	if event.Source == "opencode-plugin" {
		payload = lifecycleOnlyPayload(payload)
	}
	line := map[string]any{
		"source":                        event.Source,
		"session_id":                    event.SessionID,
		"event_type":                    event.EventType,
		"occurred_at":                   event.OccurredAt,
		"project_id":                    resolved.ProjectID,
		"project_location_id":           resolved.LocationID,
		"project_resolution_method":     string(resolved.Resolution.Method),
		"project_resolution_confidence": string(resolved.Resolution.Confidence),
		"project_resolution_evidence":   map[string]string{"source": "central-project-resolver"},
		"payload":                       payload,
	}
	if event.UpstreamEventID != "" {
		line["upstream_event_id"] = event.UpstreamEventID
	}
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(line); err != nil {
		return 0, err
	}
	count, err := jsonl.Import(ctx, service.Store, &buffer)
	if err != nil {
		return 0, fmt.Errorf("import plugin event: %w", err)
	}
	return count, nil
}

func normalizeCodexRawResponse(event Event) Event {
	if event.Source != "codex-app-server" || event.EventType != "rawResponse/completed" {
		return event
	}
	var payload struct {
		Model string `json:"model"`
		Usage *struct {
			InputTokens        int64 `json:"input_tokens"`
			OutputTokens       int64 `json:"output_tokens"`
			InputTokensDetails struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			OutputTokensDetails struct {
				ReasoningTokens int64 `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Usage == nil || payload.Model == "" {
		return event
	}
	normalized := map[string]any{
		"provider":            "openai",
		"model":               payload.Model,
		"agent_name":          "codex",
		"input_tokens":        payload.Usage.InputTokens,
		"output_tokens":       payload.Usage.OutputTokens,
		"cached_input_tokens": payload.Usage.InputTokensDetails.CachedTokens,
		"reasoning_tokens":    payload.Usage.OutputTokensDetails.ReasoningTokens,
		"capture_quality":     "agent_reported",
	}
	next, err := json.Marshal(normalized)
	if err != nil {
		return event
	}
	event.EventType = "model.call"
	event.Payload = next
	return event
}

type Event struct {
	Source          string          `json:"source"`
	SessionID       string          `json:"session_id"`
	EventType       string          `json:"event_type"`
	OccurredAt      time.Time       `json:"occurred_at"`
	ProjectHint     ProjectHint     `json:"project_hint"`
	Payload         json.RawMessage `json:"payload"`
	UpstreamEventID string          `json:"upstream_event_id"`
}

type ProjectHint struct {
	Project string `json:"project"`
	CWD     string `json:"cwd"`
}

func sanitizePluginPayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage("{}")
	}
	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return json.RawMessage("{}")
	}
	allowed := make(map[string]any, 11)
	for _, key := range []string{"provider", "model", "model_id", "agent_name", "capture_quality", "task_id", "turn_id"} {
		if value, ok := object[key].(string); ok {
			allowed[key] = value
		}
	}
	for _, key := range []string{"input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens"} {
		if value, ok := nonNegativeInteger(object[key]); ok {
			allowed[key] = value
		}
	}
	next, err := json.Marshal(allowed)
	if err != nil {
		return json.RawMessage("{}")
	}
	return next
}

func lifecycleOnlyPayload(payload json.RawMessage) json.RawMessage {
	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return json.RawMessage(`{"capture_quality":"lifecycle_only"}`)
	}
	for _, key := range []string{"input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens"} {
		delete(object, key)
	}
	object["capture_quality"] = "lifecycle_only"
	next, err := json.Marshal(object)
	if err != nil {
		return json.RawMessage(`{"capture_quality":"lifecycle_only"}`)
	}
	return next
}

func nonNegativeInteger(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || number != float64(int64(number)) {
		return 0, false
	}
	return int64(number), true
}
