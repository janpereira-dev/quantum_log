package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/janpereira-dev/quantum_log/internal/ingest/qlogevent"
	"github.com/spf13/cobra"
)

const hookIngestTimeout = 2 * time.Second

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
			upstreamID = "claude-prompt:" + rand.Text()
		} else {
			digest := sha256.Sum256([]byte(sessionID + "\x00" + timestamp))
			upstreamID = "claude-prompt:" + hex.EncodeToString(digest[:])
		}
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
			// Prompt content must never become a publicly testable event ID.
			if raw.Timestamp <= 0 {
				raw.EventID = "copilot-prompt:" + rand.Text()
			} else {
				digest := sha256.Sum256([]byte(raw.SessionID + "\x00" + strconv.FormatInt(raw.Timestamp, 10)))
				raw.EventID = "copilot-prompt:" + hex.EncodeToString(digest[:])
			}
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
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" || !loopbackHost(parsed.Hostname()) {
			return fmt.Errorf("collector URL must use pinned HTTPS on a literal loopback address or localhost")
		}
		client, err := collectorHTTPSClient(*home, hookIngestTimeout)
		if err != nil {
			return err
		}
		defer client.CloseIdleConnections()
		paths, err := config.Resolve(*home)
		if err != nil {
			return err
		}
		token, err := config.CollectorToken(paths)
		if err != nil {
			return err
		}
		event = qlogevent.PrepareForwarding(event, paths)
		encoded, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode hook event: %w", err)
		}
		request, err := http.NewRequestWithContext(command.Context(), http.MethodPost, endpoint, bytes.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("create hook request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Qlog-Token", token)
		response, err := client.Do(request)
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

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// bestEffortHook deliberately never changes the agent's exit status. The
// collector can be unavailable during upgrades, restarts, or a transient
// SQLite lock; the next native event will retry ingestion.
func bestEffortHook(command *cobra.Command, home *string, event qlogevent.Event) error {
	// Windows CI and cold SQLite connections can exceed the old 500ms budget;
	// retain best-effort semantics while allowing one deterministic write.
	ctx, cancel := context.WithTimeout(command.Context(), hookIngestTimeout)
	defer cancel()
	command.SetContext(ctx)
	if err := ingestOrForwardHook(command, home, event); err != nil {
		_, _ = fmt.Fprintf(command.ErrOrStderr(), "qlog hook ignored: %s\n", strings.TrimSpace(err.Error()))
	}
	return nil
}
