// Package codexsession imports usage metadata from local Codex session logs.
package codexsession

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

const Source = "codex-session-jsonl"
const maxLineBytes = 8 * 1024 * 1024

type Result struct {
	Samples    int `json:"samples"`
	Duplicates int `json:"duplicates"`
	Skipped    int `json:"skipped"`
}

// Importer reuses source project attribution across a sequential import batch.
type Importer struct {
	service  *app.Service
	projects map[string]app.ResolvedProject
}

func New(service *app.Service) *Importer {
	return &Importer{service: service, projects: make(map[string]app.ResolvedProject)}
}

type counters struct {
	Input     *int64 `json:"input_tokens,omitempty"`
	Cached    *int64 `json:"cached_input_tokens,omitempty"`
	Output    *int64 `json:"output_tokens,omitempty"`
	Reasoning *int64 `json:"reasoning_output_tokens,omitempty"`
	Total     *int64 `json:"total_tokens,omitempty"`
}

func (c counters) values() []*int64 {
	return []*int64{c.Input, c.Cached, c.Output, c.Reasoning, c.Total}
}

type envelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type metadata struct {
	ID           string          `json:"id"`
	CWD          string          `json:"cwd"`
	Version      string          `json:"cli_version"`
	Provider     string          `json:"model_provider"`
	Model        string          `json:"model"`
	Timestamp    string          `json:"timestamp"`
	ParentID     string          `json:"parent_thread_id"`
	ForkedFromID string          `json:"forked_from_id"`
	Source       json.RawMessage `json:"source"`
}

type usageEvent struct {
	Type string `json:"type"`
	Info *struct {
		Total *counters `json:"total_token_usage"`
		Last  *counters `json:"last_token_usage"`
	} `json:"info"`
}

func (m metadata) isChild() bool {
	var source struct {
		Subagent json.RawMessage `json:"subagent"`
	}
	_ = json.Unmarshal(m.Source, &source)
	return m.ParentID != "" || m.ForkedFromID != "" || len(source.Subagent) != 0
}

func sameCounters(current counters, last *counters) bool {
	if last == nil || last.Total == nil || current.Total == nil || *last.Total != *current.Total {
		return false
	}
	for index, value := range current.values() {
		other := last.values()[index]
		if value != nil && (other == nil || *value != *other) {
			return false
		}
	}
	return true
}

// Import never changes the source log and only persists whitelisted metadata.
// The caller owns the already-open service and its lifetime.
func Import(ctx context.Context, service *app.Service, reader io.Reader) (Result, error) {
	return New(service).Import(ctx, reader)
}

func (i *Importer) Import(ctx context.Context, reader io.Reader) (Result, error) {
	var result Result
	if i == nil || i.service == nil || i.service.Store == nil || reader == nil {
		return result, errors.New("codex import requires an open ledger and reader")
	}
	service := i.service
	buffer := bufio.NewReaderSize(reader, 64*1024)
	var meta metadata
	var previous *counters
	var child bool
	var childCreated time.Time
	var childStarted bool
	for lineNumber := 1; ; lineNumber++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		line, complete, err := readLine(buffer)
		if err != nil {
			return result, fmt.Errorf("codex session line %d: %w", lineNumber, err)
		}
		if len(bytes.TrimSpace(line)) == 0 {
			if !complete {
				return result, nil
			}
			continue
		}
		var event envelope
		if json.Unmarshal(line, &event) != nil {
			if !complete {
				result.Skipped++ // An active writer may not have finished the last line.
				return result, nil
			}
			return result, fmt.Errorf("codex session line %d: invalid JSON", lineNumber)
		}
		switch event.Type {
		case "session_meta":
			var next metadata
			if json.Unmarshal(event.Payload, &next) != nil || identifier(next.ID) == "" {
				return result, fmt.Errorf("codex session line %d: invalid session metadata", lineNumber)
			}
			if next.ID != meta.ID {
				previous = nil
				childStarted = false
			}
			meta = next
			child = meta.isChild()
			childCreated, _ = time.Parse(time.RFC3339Nano, meta.Timestamp)
		case "turn_context":
			var turn metadata
			if json.Unmarshal(event.Payload, &turn) != nil {
				return result, fmt.Errorf("codex session line %d: invalid turn metadata", lineNumber)
			}
			meta.Model = turn.Model
			if strings.TrimSpace(turn.CWD) != "" {
				meta.CWD = turn.CWD
			}
		case "event_msg":
			var usage usageEvent
			if json.Unmarshal(event.Payload, &usage) != nil {
				return result, fmt.Errorf("codex session line %d: invalid usage event", lineNumber)
			}
			if usage.Type != "token_count" {
				break
			}
			if usage.Info == nil || usage.Info.Total == nil || usage.Info.Total.Total == nil || meta.ID == "" {
				result.Skipped++
				break
			}
			current := *usage.Info.Total
			occurredAt, err := time.Parse(time.RFC3339Nano, event.Timestamp)
			if err != nil {
				return result, fmt.Errorf("codex session line %d: invalid usage timestamp", lineNumber)
			}
			if child {
				if childCreated.IsZero() || !occurredAt.After(childCreated) {
					previous = &current
					result.Skipped++ // Forked history belongs to the parent, not the new session ID.
					break
				}
				if !childStarted {
					if previous != nil && previous.Total != nil && *current.Total == *previous.Total {
						previous = &current
						result.Skipped++ // Re-emitting the inherited total does not prove child usage.
						break
					}
					childStarted = true
					if sameCounters(current, usage.Info.Last) {
						previous = nil // Native last usage confirms a fresh child counter.
					} else if previous == nil {
						previous = &current
						result.Skipped++ // Ambiguous inherited total: establish a baseline first.
						break
					}
				}
			}
			delta, usable := difference(current, previous)
			previous = &current
			if !usable {
				result.Skipped++
				break
			}
			var project app.ResolvedProject
			if cwd := strings.TrimSpace(meta.CWD); cwd != "" {
				var found bool
				project, found = i.projects[cwd]
				if !found {
					project, err = service.ResolveSourceProject(ctx, cwd)
					if err != nil {
						return result, fmt.Errorf("codex session line %d: resolve source project: %w", lineNumber, err)
					}
					i.projects[cwd] = project
				}
			}
			if err := persist(ctx, service, meta, current, delta, occurredAt, project, &result); err != nil {
				return result, fmt.Errorf("codex session line %d: %w", lineNumber, err)
			}
		}
		if !complete {
			return result, nil
		}
	}
}

func readLine(reader *bufio.Reader) ([]byte, bool, error) {
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxLineBytes {
			return nil, false, errors.New("line exceeds 8 MiB limit")
		}
		line = append(line, fragment...)
		switch err {
		case nil:
			return line, true, nil
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			return line, false, nil
		default:
			return nil, false, errors.New("cannot read source log")
		}
	}
}

func difference(current counters, previous *counters) (counters, bool) {
	var delta counters
	var changed, present bool
	currentValues, deltaValues := current.values(), []**int64{&delta.Input, &delta.Cached, &delta.Output, &delta.Reasoning, &delta.Total}
	for index, value := range currentValues {
		if value == nil {
			continue
		}
		if *value < 0 {
			return counters{}, false
		}
		amount := *value
		if previous != nil {
			before := previous.values()[index]
			if before == nil {
				continue // A newly available counter has no trustworthy interval baseline.
			}
			if *before < 0 || amount < *before {
				return counters{}, false // Skip a reset and rebase; never invent usage.
			}
			amount -= *before
		}
		present = true
		changed = changed || amount != 0
		*deltaValues[index] = &amount
	}
	if delta.Input != nil && delta.Cached != nil && *delta.Cached > *delta.Input || delta.Output != nil && delta.Reasoning != nil && *delta.Reasoning > *delta.Output {
		return counters{}, false
	}
	return delta, present && (previous == nil || changed)
}

func persist(ctx context.Context, service *app.Service, meta metadata, current, delta counters, at time.Time, project app.ResolvedProject, result *Result) error {
	provider, model := identifier(meta.Provider), identifier(meta.Model)
	if provider == "" {
		provider = "unavailable"
	}
	if model == "" {
		model = "unavailable"
	}
	// Hash only native identity and cumulative measurements, never file paths or content.
	identity, _ := json.Marshal(struct {
		Session string
		Time    string
		Usage   counters
	}{meta.ID, at.UTC().Format(time.RFC3339Nano), current})
	digest := sha256.Sum256(identity)
	payload, _ := json.Marshal(struct {
		Agent    string   `json:"agent_name"`
		Provider string   `json:"provider"`
		Model    string   `json:"model"`
		Usage    counters `json:"total_token_usage"`
	}{"codex", provider, model, current})
	if err := service.Store.EnsureSession(ctx, meta.ID, "codex", at); err != nil {
		return err
	}
	raw, err := service.Store.AppendRawEvent(ctx, storepkg.RawEventInput{Source: Source, SourceVersion: identifier(meta.Version), SessionID: meta.ID, IngestionIdentity: hex.EncodeToString(digest[:]), EventType: "model.call", OccurredAt: at, Payload: payload, ProjectID: project.ProjectID, ProjectLocationID: project.LocationID, ResolutionMethod: string(project.Resolution.Method), ResolutionConfidence: string(project.Resolution.Confidence)})
	if err != nil {
		return err
	}
	if !raw.Accepted {
		linked, err := service.Store.HasModelCallForRawEvent(ctx, raw.ID)
		if err != nil {
			return err
		}
		if linked {
			result.Duplicates++
			return nil
		}
	}
	input := storepkg.ModelCallInput{RawEventID: raw.ID, SessionID: meta.ID, Provider: provider, ModelID: model, AgentName: "codex", CaptureQuality: "agent_reported", OccurredAt: at, ProjectID: project.ProjectID, ProjectLocationID: project.LocationID}
	fields := []*int64{&input.InputTokens, &input.CachedInputTokens, &input.OutputTokens, &input.ReasoningTokens}
	names := []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_tokens", "total_tokens"}
	keys := []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens"}
	for index, value := range delta.values() {
		if value == nil {
			continue
		}
		if index < len(fields) {
			*fields[index] = *value
		}
		input.Metrics = append(input.Metrics, storepkg.MetricInput{Name: names[index], Value: value, Source: Source, RawKey: "info.total_token_usage." + keys[index], Confidence: "reported"})
	}
	// Cached and reasoning counts are subsets; legacy storage sums disjoint components.
	if delta.Input != nil && delta.Cached != nil {
		input.InputTokens -= input.CachedInputTokens
	}
	if delta.Output != nil && delta.Reasoning != nil {
		input.OutputTokens -= input.ReasoningTokens
	}
	if _, err := service.Store.RecordModelCall(ctx, input); err != nil {
		return err
	}
	if raw.Accepted {
		result.Samples++
	} else {
		result.Duplicates++
	}
	return nil
}

func identifier(value string) string {
	if len(value) > 256 {
		return ""
	}
	for _, char := range value {
		allowed := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._:/-", char)
		if !allowed {
			return ""
		}
	}
	return value
}
