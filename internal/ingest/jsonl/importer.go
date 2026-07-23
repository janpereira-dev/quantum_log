// Package jsonl imports sanitized raw events from newline-delimited JSON.
package jsonl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

type event struct {
	Source               string          `json:"source"`
	SessionID            string          `json:"session_id"`
	EventType            string          `json:"event_type"`
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
	Provider               string `json:"provider"`
	Model                  string `json:"model"`
	ModelID                string `json:"model_id"`
	AgentName              string `json:"agent_name"`
	TaskID                 string `json:"task_id"`
	TurnID                 string `json:"turn_id"`
	InputTokens            int64  `json:"input_tokens"`
	OutputTokens           int64  `json:"output_tokens"`
	ReasoningTokens        int64  `json:"reasoning_tokens"`
	CachedInputTokens      int64  `json:"cached_input_tokens"`
	CacheWriteTokens       int64  `json:"cache_write_tokens"`
	EstimatedCostUSDMicros int64  `json:"estimated_cost_usd_micros"`
	EstimatedCostEURMicros int64  `json:"estimated_cost_eur_micros"`
	CaptureQuality         string `json:"capture_quality"`
}

func Import(ctx context.Context, store *storepkg.Store, reader io.Reader) (int, error) {
	return importWithTrust(ctx, store, reader, false)
}

func ImportTrusted(ctx context.Context, store *storepkg.Store, reader io.Reader) (int, error) {
	return importWithTrust(ctx, store, reader, true)
}

func importWithTrust(ctx context.Context, store *storepkg.Store, reader io.Reader, trusted bool) (int, error) {
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
		evidence := "{}"
		if parsed.EvidenceJSON != nil {
			evidence = string(parsed.EvidenceJSON)
		}
		if _, err := store.AppendRawEvent(ctx, storepkg.RawEventInput{Source: parsed.Source, SessionID: parsed.SessionID, EventType: parsed.EventType, Payload: parsed.Payload, OccurredAt: parsed.OccurredAt, ProjectID: parsed.ProjectID, ProjectLocationID: parsed.ProjectLocationID, WorkContextID: parsed.WorkContextID, ResolutionMethod: parsed.ResolutionMethod, ResolutionConfidence: parsed.ResolutionConfidence, EvidenceJSON: evidence}); err != nil {
			return count, fmt.Errorf("import NDJSON line %d: %w", line, err)
		}
		if err := normalizeModelCall(ctx, store, parsed); err != nil {
			return count, fmt.Errorf("normalize NDJSON line %d: %w", line, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("read NDJSON: %w", err)
	}
	return count, nil
}

func normalizeModelCall(ctx context.Context, store *storepkg.Store, parsed event) error {
	eventType := strings.ReplaceAll(strings.ToLower(parsed.EventType), "_", ".")
	if eventType != "model.call" {
		return nil
	}
	var payload modelCallPayload
	if err := json.Unmarshal(parsed.Payload, &payload); err != nil {
		return fmt.Errorf("decode model call payload: %w", err)
	}
	if payload.Model == "" {
		payload.Model = payload.ModelID
	}
	if payload.Provider == "" || payload.Model == "" {
		return nil
	}
	if err := store.EnsureSession(ctx, parsed.SessionID, payload.AgentName, parsed.OccurredAt); err != nil {
		return err
	}
	_, err := store.RecordModelCall(ctx, storepkg.ModelCallInput{
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
	})
	return err
}
