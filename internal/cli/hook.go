package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/ingest/qlogevent"
	"github.com/spf13/cobra"
)

func newHookCommand(home *string) *cobra.Command {
	hook := &cobra.Command{Use: "hook", Short: "Receive local agent hook payloads"}
	hook.AddCommand(&cobra.Command{Use: "claude-code", Short: "Capture Claude Code lifecycle hooks", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		body, err := io.ReadAll(command.InOrStdin())
		if err != nil {
			return fmt.Errorf("read hook input: %w", err)
		}
		event, err := claudeCodeHookEvent(body)
		if err != nil {
			return err
		}
		if err := bestEffortHook(command, home, event); err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), "hook: ingested")
		return err
	}})
	var copilotEvent string
	copilot := &cobra.Command{Use: "copilot-cli", Short: "Capture Copilot CLI lifecycle hooks", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		body, err := io.ReadAll(command.InOrStdin())
		if err != nil {
			return fmt.Errorf("read hook input: %w", err)
		}
		event, err := copilotCLIHookEvent(body, copilotEvent)
		if err != nil {
			return err
		}
		return bestEffortHook(command, home, event)
	}}
	copilot.Flags().StringVar(&copilotEvent, "event", "", "Copilot CLI lifecycle hook event")
	hook.AddCommand(copilot)
	return hook
}

func claudeCodeHookEvent(input []byte) (qlogevent.Event, error) {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		return qlogevent.Event{}, fmt.Errorf("decode Claude Code hook JSON: %w", err)
	}
	sessionID, _ := raw["session_id"].(string)
	prompt, _ := raw["prompt"].(string)
	eventType, _ := raw["hook_event_name"].(string)
	if eventType == "" {
		eventType = "ClaudeCodeHook"
	}
	cwd, _ := raw["cwd"].(string)
	upstreamID, _ := raw["event_id"].(string)
	if upstreamID == "" && strings.EqualFold(eventType, "UserPromptSubmit") {
		// Claude hooks do not consistently include an event ID. Scope a
		// deterministic identity to the session and event timestamp so repeated
		// identical prompts remain separate interactions.
		timestamp, _ := raw["timestamp"].(string)
		if timestamp == "" {
			timestamp = time.Now().UTC().Format(time.RFC3339Nano)
		}
		digest := sha256.Sum256([]byte(sessionID + "\x00" + timestamp + "\x00" + prompt))
		upstreamID = "claude-prompt:" + hex.EncodeToString(digest[:])
	}
	payload, err := json.Marshal(map[string]any{
		"agent_name":      "claude-code",
		"capture_quality": "lifecycle_only",
		"prompt":          prompt,
	})
	if err != nil {
		return qlogevent.Event{}, fmt.Errorf("encode Claude Code hook payload: %w", err)
	}
	return qlogevent.Event{
		Source:          "claude-code-hook",
		SessionID:       sessionID,
		UpstreamEventID: upstreamID,
		EventType:       eventType,
		OccurredAt:      time.Now().UTC(),
		ProjectHint:     qlogevent.ProjectHint{CWD: cwd},
		Payload:         payload,
	}, nil
}

func copilotCLIHookEvent(input []byte, eventType string) (qlogevent.Event, error) {
	// Keep this in lockstep with copilotCLIHooksConfig. Hooks are deliberately
	// best-effort, but silently rejecting a configured event loses its
	// lifecycle evidence before bestEffortHook can protect the agent process.
	allowedEvents := map[string]bool{
		"sessionStart":        true,
		"sessionEnd":          true,
		"agentStop":           true,
		"errorOccurred":       true,
		"preToolUse":          true,
		"postToolUse":         true,
		"userPromptSubmitted": true,
		"subagentStart":       true,
		"subagentStop":        true,
	}
	if !allowedEvents[eventType] {
		return qlogevent.Event{}, fmt.Errorf("unsupported Copilot CLI hook event %q", eventType)
	}
	var raw struct {
		SessionID string `json:"sessionId"`
		CWD       string `json:"cwd"`
		EventID   string `json:"eventId"`
		Timestamp int64  `json:"timestamp"`
		Prompt    string `json:"prompt"`
	}
	if err := json.Unmarshal(input, &raw); err != nil {
		return qlogevent.Event{}, fmt.Errorf("decode Copilot CLI hook JSON: %w", err)
	}
	normalizedEvent := eventType
	occurredAt := time.Now().UTC()
	if raw.Timestamp > 0 {
		occurredAt = time.UnixMilli(raw.Timestamp).UTC()
	}
	if eventType == "userPromptSubmitted" {
		normalizedEvent = "interaction.prompt"
		if raw.EventID == "" {
			// Copilot's documented camelCase prompt payload has no event ID.
			// The session/timestamp/prompt tuple is source-native and remains
			// stable for a retried hook invocation without collapsing turns.
			digest := sha256.Sum256([]byte(raw.SessionID + "\x00" + strconv.FormatInt(raw.Timestamp, 10) + "\x00" + raw.Prompt))
			raw.EventID = "copilot-prompt:" + hex.EncodeToString(digest[:])
		}
	}
	payload, err := json.Marshal(map[string]any{"agent_name": "copilot", "capture_quality": "lifecycle_only", "prompt": raw.Prompt})
	if err != nil {
		return qlogevent.Event{}, fmt.Errorf("encode Copilot CLI hook payload: %w", err)
	}
	return qlogevent.Event{Source: "copilot-cli-hook", SessionID: raw.SessionID, EventType: normalizedEvent, UpstreamEventID: raw.EventID, OccurredAt: occurredAt, ProjectHint: qlogevent.ProjectHint{CWD: raw.CWD}, Payload: payload}, nil
}

func ingestOrForwardHook(command *cobra.Command, home *string, event qlogevent.Event) error {
	if endpoint := os.Getenv("QLOG_COLLECTOR_URL"); endpoint != "" {
		encoded, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode hook event: %w", err)
		}
		request, err := http.NewRequestWithContext(command.Context(), http.MethodPost, endpoint, bytes.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("create hook request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return fmt.Errorf("post hook event: %w", err)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode < 200 || response.StatusCode > 299 {
			return fmt.Errorf("collector rejected hook event: %s", response.Status)
		}
		return nil
	}
	service, err := app.Open(command.Context(), *home)
	if err != nil {
		return err
	}
	defer func() { _ = service.Close() }()
	_, err = qlogevent.Ingest(command.Context(), service, event)
	if err != nil {
		return err
	}
	return nil
}

// bestEffortHook deliberately never changes the agent's exit status. The
// collector can be unavailable during upgrades, restarts, or a transient
// SQLite lock; the next native event will retry ingestion.
func bestEffortHook(command *cobra.Command, home *string, event qlogevent.Event) error {
	ctx, cancel := context.WithTimeout(command.Context(), 2*time.Second)
	defer cancel()
	command.SetContext(ctx)
	if err := ingestOrForwardHook(command, home, event); err != nil {
		_, _ = fmt.Fprintf(command.ErrOrStderr(), "qlog hook ignored: %s\n", strings.TrimSpace(err.Error()))
	}
	return nil
}
