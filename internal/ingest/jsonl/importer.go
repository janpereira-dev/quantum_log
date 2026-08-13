// Package jsonl imports sanitized raw events from newline-delimited JSON.
package jsonl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

type event struct {
	IngestionIdentity    string          `json:"upstream_event_id"`
	Source               string          `json:"source"`
	SourceVersion        string          `json:"source_version"`
	SessionID            string          `json:"session_id"`
	EventType            string          `json:"event_type"`
	TraceID              string          `json:"trace_id"`
	SpanID               string          `json:"span_id"`
	ParentSpanID         string          `json:"parent_span_id"`
	OccurredAt           time.Time       `json:"occurred_at"`
	ProjectID            string          `json:"project_id"`
	ProjectLocationID    string          `json:"project_location_id"`
	WorkContextID        string          `json:"work_context_id"`
	ResolutionMethod     string          `json:"project_resolution_method"`
	ResolutionConfidence string          `json:"project_resolution_confidence"`
	EvidenceJSON         json.RawMessage `json:"project_resolution_evidence"`
	Payload              json.RawMessage `json:"payload"`
}

type modelCallPayload struct {
	InteractionUpstreamID  string              `json:"interaction_upstream_id"`
	Provider               string              `json:"provider"`
	Model                  string              `json:"model"`
	ModelID                string              `json:"model_id"`
	AgentName              string              `json:"agent_name"`
	TaskID                 string              `json:"task_id"`
	TurnID                 string              `json:"turn_id"`
	InputTokens            int64               `json:"input_tokens"`
	OutputTokens           int64               `json:"output_tokens"`
	ReasoningTokens        int64               `json:"reasoning_tokens"`
	CachedInputTokens      int64               `json:"cached_input_tokens"`
	CacheWriteTokens       int64               `json:"cache_write_tokens"`
	EstimatedCostUSDMicros int64               `json:"estimated_cost_usd_micros"`
	EstimatedCostEURMicros int64               `json:"estimated_cost_eur_micros"`
	CreatedAt              int64               `json:"created_at"`
	CompletedAt            int64               `json:"completed_at"`
	CaptureQuality         string              `json:"capture_quality"`
	MetricObservations     []metricObservation `json:"metric_observations"`
}

type metricObservation struct {
	Name       string `json:"name"`
	Value      *int64 `json:"value"`
	Source     string `json:"source"`
	RawKey     string `json:"raw_key"`
	Confidence string `json:"confidence"`
}

var importedPromptSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)\b(authorization|api[_ -]?key|access[_ -]?token|password|cookie)\s*[:=]\s*[^\s,;]+`),
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func Import(ctx context.Context, store *storepkg.Store, reader io.Reader) (int, error) {
	return importWithTrustAndPromptCapture(ctx, store, reader, false, "hash")
}

// ImportWithPromptCapture applies the local privacy policy at the persistence
// boundary, including for manually imported or spoofed envelopes.
func ImportWithPromptCapture(ctx context.Context, store *storepkg.Store, reader io.Reader, mode string) (int, error) {
	return importWithTrustAndPromptCapture(ctx, store, reader, false, mode)
}

func ImportTrusted(ctx context.Context, store *storepkg.Store, reader io.Reader) (int, error) {
	return importWithTrustAndPromptCapture(ctx, store, reader, true, "hash")
}

func importWithTrustAndPromptCapture(ctx context.Context, store *storepkg.Store, reader io.Reader, trusted bool, promptCaptureMode string) (int, error) {
	if promptCaptureMode != "off" && promptCaptureMode != "hash" && promptCaptureMode != "full" {
		promptCaptureMode = "hash"
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	count := 0
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var parsed event
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return count, fmt.Errorf("parse NDJSON line %d: %w", line, err)
		}
		if parsed.Source == "" {
			parsed.Source = "ndjson"
		}
		if !trusted && parsed.Source == "otlp-http" {
			return count, fmt.Errorf("import NDJSON line %d: source %q is reserved for qlog collector events", line, parsed.Source)
		}
		if parsed.Payload == nil {
			parsed.Payload = json.RawMessage("{}")
		}
		if isInteractionEvent(parsed.EventType) {
			parsed.Payload = enforcePromptCapturePolicy(parsed.Payload, promptCaptureMode)
		}
		evidence := "{}"
		if parsed.EvidenceJSON != nil {
			evidence = string(parsed.EvidenceJSON)
		}
		appendResult, err := store.AppendRawEvent(ctx, storepkg.RawEventInput{IngestionIdentity: parsed.IngestionIdentity, Source: parsed.Source, SourceVersion: parsed.SourceVersion, SessionID: parsed.SessionID, EventType: parsed.EventType, TraceID: parsed.TraceID, SpanID: parsed.SpanID, ParentSpanID: parsed.ParentSpanID, Payload: parsed.Payload, OccurredAt: parsed.OccurredAt, ProjectID: parsed.ProjectID, ProjectLocationID: parsed.ProjectLocationID, WorkContextID: parsed.WorkContextID, ResolutionMethod: parsed.ResolutionMethod, ResolutionConfidence: parsed.ResolutionConfidence, EvidenceJSON: evidence})
		if err != nil {
			return count, fmt.Errorf("import NDJSON line %d: %w", line, err)
		}
		_, err = normalizeModelCall(ctx, store, parsed, appendResult.ID)
		if err == nil {
			_, err = normalizeInteraction(ctx, store, parsed, appendResult.ID)
		}
		if err == nil {
			_, err = normalizeToolCall(ctx, store, parsed, appendResult.ID)
		}
		if err != nil {
			return count, fmt.Errorf("normalize NDJSON line %d: %w", line, err)
		}
		if appendResult.Accepted {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read NDJSON: %w", err)
	}
	return count, nil
}

func isInteractionEvent(eventType string) bool {
	eventType = strings.ReplaceAll(strings.ToLower(eventType), "_", ".")
	return eventType == "interaction.prompt" || eventType == "user.prompt" || eventType == "userpromptsubmitted" || eventType == "userpromptsubmit" || eventType == "user.message"
}

func enforcePromptCapturePolicy(payload json.RawMessage, mode string) json.RawMessage {
	var values map[string]any
	if json.Unmarshal(payload, &values) != nil {
		return json.RawMessage(`{"prompt_capture_mode":"off"}`)
	}
	values["prompt_capture_mode"] = mode
	if mode == "off" {
		delete(values, "prompt_hash")
		delete(values, "interaction_hash")
		delete(values, "interaction_redacted")
	} else if mode == "hash" {
		delete(values, "interaction_redacted")
	} else if redacted, ok := values["interaction_redacted"].(string); ok {
		for _, pattern := range importedPromptSecretPatterns {
			redacted = pattern.ReplaceAllString(redacted, "[REDACTED]")
		}
		values["interaction_redacted"] = redacted
	}
	result, err := json.Marshal(values)
	if err != nil {
		return json.RawMessage(`{"prompt_capture_mode":"off"}`)
	}
	return result
}

func normalizeToolCall(ctx context.Context, store *storepkg.Store, parsed event, rawEventID string) (bool, error) {
	eventType := strings.ToLower(strings.ReplaceAll(parsed.EventType, "_", "."))
	if !strings.Contains(eventType, "tool") {
		return false, nil
	}
	var payload struct {
		InteractionUpstreamID string `json:"interaction_upstream_id"`
		ToolName              string `json:"tool_name"`
		CaptureQuality        string `json:"capture_quality"`
	}
	_ = json.Unmarshal(parsed.Payload, &payload)
	if payload.ToolName == "" {
		payload.ToolName = eventType
	}
	if err := store.EnsureSession(ctx, parsed.SessionID, "", parsed.OccurredAt); err != nil {
		return false, err
	}
	interactionID := ""
	if payload.InteractionUpstreamID != "" {
		var found bool
		interactionID, found, _ = store.InteractionByUpstream(ctx, parsed.Source, parsed.SessionID, payload.InteractionUpstreamID)
		if !found {
			interactionID, _, _ = store.InteractionBySessionUpstream(ctx, parsed.SessionID, payload.InteractionUpstreamID)
		}
	}
	return store.RecordToolCall(ctx, storepkg.ToolCallInput{RawEventID: rawEventID, InteractionID: interactionID, InteractionUpstreamID: payload.InteractionUpstreamID, ProjectID: parsed.ProjectID, LocationID: parsed.ProjectLocationID, WorkContextID: parsed.WorkContextID, SessionID: parsed.SessionID, ToolName: payload.ToolName, ToolType: eventType, OccurredAt: parsed.OccurredAt, CaptureQuality: payload.CaptureQuality})
}

func normalizeInteraction(ctx context.Context, store *storepkg.Store, parsed event, rawEventID string) (bool, error) {
	// Codex's supported local app-server stream emits one completed response for
	// each interactive turn but not the prompt body. Treat that source-native
	// response identity as a privacy-safe interaction root rather than dropping
	// the prompt count altogether.
	if parsed.IngestionIdentity == "" {
		return false, nil
	}
	var payload struct {
		PromptHash            string `json:"prompt_hash"`
		InteractionHash       string `json:"interaction_hash"`
		Redacted              string `json:"interaction_redacted"`
		CaptureMode           string `json:"prompt_capture_mode"`
		InteractionUpstreamID string `json:"interaction_upstream_id"`
		AgentName             string `json:"agent_name"`
	}
	_ = json.Unmarshal(parsed.Payload, &payload)
	if !isInteractionEvent(parsed.EventType) && !isCodexResponseRoot(parsed) && !isAgentTraceInteraction(parsed, payload.InteractionUpstreamID, payload.AgentName) {
		return false, nil
	}
	upstreamID := parsed.IngestionIdentity
	if isAgentTraceInteraction(parsed, payload.InteractionUpstreamID, payload.AgentName) {
		upstreamID = payload.InteractionUpstreamID
	}
	_, created, err := store.RecordInteraction(ctx, storepkg.InteractionInput{
		RawEventID: rawEventID, Source: parsed.Source, SessionID: parsed.SessionID, UpstreamID: upstreamID,
		ProjectID: parsed.ProjectID, ProjectLocationID: parsed.ProjectLocationID, WorkContextID: parsed.WorkContextID,
		PromptHash: firstNonEmpty(payload.InteractionHash, payload.PromptHash), PromptRedacted: payload.Redacted,
		PromptCaptureMode: payload.CaptureMode,
		OccurredAt:        parsed.OccurredAt,
	})
	return created, err
}

func isCodexResponseRoot(parsed event) bool {
	return parsed.Source == "codex-app-server" && strings.EqualFold(parsed.EventType, "model.call")
}

func isAgentTraceInteraction(parsed event, interactionUpstreamID, agentName string) bool {
	if parsed.Source != "otlp-http" || interactionUpstreamID == "" {
		return false
	}
	if isInteractionEvent(parsed.EventType) {
		return true
	}
	if !strings.EqualFold(parsed.EventType, "model.call") {
		return false
	}
	name := strings.ToLower(agentName)
	return strings.HasPrefix(name, "github copilot") || name == "claude-code" || name == "codex"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func unixMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func modelCallStartedAt(fallback time.Time, createdAt int64) time.Time {
	if created := unixMillis(createdAt); !created.IsZero() {
		return created
	}
	return fallback
}

func normalizeModelCall(ctx context.Context, store *storepkg.Store, parsed event, rawEventID string) (bool, error) {
	eventType := strings.ReplaceAll(strings.ToLower(parsed.EventType), "_", ".")
	if eventType != "model.call" {
		return false, nil
	}
	linked, err := store.HasModelCallForRawEvent(ctx, rawEventID)
	if err != nil {
		return false, err
	}
	if linked {
		return false, nil
	}
	var payload modelCallPayload
	if err := json.Unmarshal(parsed.Payload, &payload); err != nil {
		return false, fmt.Errorf("decode model call payload: %w", err)
	}
	if payload.Model == "" {
		payload.Model = payload.ModelID
	}
	if payload.Provider == "" || payload.Model == "" {
		return false, nil
	}
	sourceStartedAt := modelCallStartedAt(parsed.OccurredAt, payload.CreatedAt)
	if err := store.EnsureSession(ctx, parsed.SessionID, payload.AgentName, sourceStartedAt); err != nil {
		return false, err
	}
	input := storepkg.ModelCallInput{
		RawEventID:             rawEventID,
		InteractionUpstreamID:  payload.InteractionUpstreamID,
		ProjectID:              parsed.ProjectID,
		ProjectLocationID:      parsed.ProjectLocationID,
		WorkContextID:          parsed.WorkContextID,
		TaskID:                 payload.TaskID,
		SessionID:              parsed.SessionID,
		TurnID:                 payload.TurnID,
		Provider:               payload.Provider,
		ModelID:                payload.Model,
		AgentName:              payload.AgentName,
		InputTokens:            payload.InputTokens,
		OutputTokens:           payload.OutputTokens,
		ReasoningTokens:        payload.ReasoningTokens,
		CachedInputTokens:      payload.CachedInputTokens,
		CacheWriteTokens:       payload.CacheWriteTokens,
		EstimatedCostUSDMicros: payload.EstimatedCostUSDMicros,
		EstimatedCostEURMicros: payload.EstimatedCostEURMicros,
		OccurredAt:             parsed.OccurredAt,
		CaptureQuality:         payload.CaptureQuality,
	}
	if payload.InteractionUpstreamID != "" {
		interactionID, found, err := store.InteractionByUpstream(ctx, parsed.Source, parsed.SessionID, payload.InteractionUpstreamID)
		if err != nil {
			return false, err
		}
		if !found {
			interactionID, found, err = store.InteractionBySessionUpstream(ctx, parsed.SessionID, payload.InteractionUpstreamID)
			if err != nil {
				return false, err
			}
		}
		if found {
			input.InteractionID = interactionID
		}
	}
	for _, metric := range payload.MetricObservations {
		if metric.Value != nil {
			input.Metrics = append(input.Metrics, storepkg.MetricInput{Name: metric.Name, Value: metric.Value, Source: metric.Source, RawKey: metric.RawKey, Confidence: metric.Confidence})
		}
	}
	// Legacy rows were keyed by their original envelope timestamp. Reconcile
	// that representation before adopting a more precise source timestamp.
	linked, err = store.LinkMatchingLegacyModelCall(ctx, input)
	if err != nil {
		return false, err
	}
	input.OccurredAt = sourceStartedAt
	if completed := unixMillis(payload.CompletedAt); !completed.IsZero() && !input.OccurredAt.IsZero() && !completed.Before(input.OccurredAt) {
		input.CompletedAt = completed
		duration := completed.Sub(input.OccurredAt).Milliseconds()
		input.DurationMS = &duration
	}
	if linked {
		if err := store.UpdateLinkedModelCallTiming(ctx, input.RawEventID, input.OccurredAt, input.CompletedAt); err != nil {
			return true, err
		}
		return true, store.ReconcileOTLPUsage(ctx, input.RawEventID)
	}
	_, err = store.RecordModelCall(ctx, input)
	if err != nil {
		return false, err
	}
	return true, store.ReconcileOTLPUsage(ctx, input.RawEventID)
}
