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
	var event pluginEvent
	if err := json.NewDecoder(request.Body).Decode(&event); err != nil {
		http.Error(writer, "decode event: "+err.Error(), http.StatusBadRequest)
		return
	}
	count, err := h.ingest(request.Context(), event)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]int{"accepted": count})
}

func (h Handler) ingest(ctx context.Context, event pluginEvent) (int, error) {
	resolved, err := h.service.ResolveProject(ctx, event.ProjectHint.Project, "", event.ProjectHint.CWD)
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
		"payload":                       sanitizePluginPayload(event.Payload),
	}
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(line); err != nil {
		return 0, err
	}
	count, err := jsonl.Import(ctx, h.service.Store, &buffer)
	if err != nil {
		return 0, fmt.Errorf("import plugin event: %w", err)
	}
	return count, nil
}

func normalizeCodexRawResponse(event pluginEvent) pluginEvent {
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

type pluginEvent struct {
	Source      string          `json:"source"`
	SessionID   string          `json:"session_id"`
	EventType   string          `json:"event_type"`
	OccurredAt  time.Time       `json:"occurred_at"`
	ProjectHint projectHint     `json:"project_hint"`
	Payload     json.RawMessage `json:"payload"`
}

type projectHint struct {
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
	for _, key := range []string{"prompt", "response", "arguments", "result", "tool_arguments", "tool_result", "authorization", "api_key", "token"} {
		delete(object, key)
	}
	next, err := json.Marshal(object)
	if err != nil {
		return json.RawMessage("{}")
	}
	return next
}
