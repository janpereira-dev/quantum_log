package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
			_, err = fmt.Fprintln(command.Root().OutOrStdout(), "hook: forwarded")
			return err
		}
		service, err := app.Open(command.Context(), *home)
		if err != nil {
			return err
		}
		defer func() { _ = service.Close() }()
		count, err := qlogevent.Ingest(command.Context(), service, event)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(command.Root().OutOrStdout(), "hook: ingested %d\n", count)
		return err
	}})
	return hook
}

func claudeCodeHookEvent(input []byte) (qlogevent.Event, error) {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		return qlogevent.Event{}, fmt.Errorf("decode Claude Code hook JSON: %w", err)
	}
	sessionID, _ := raw["session_id"].(string)
	eventType, _ := raw["hook_event_name"].(string)
	if eventType == "" {
		eventType = "ClaudeCodeHook"
	}
	cwd, _ := raw["cwd"].(string)
	payload, err := json.Marshal(map[string]any{
		"agent_name":      "claude-code",
		"capture_quality": "lifecycle_only",
	})
	if err != nil {
		return qlogevent.Event{}, fmt.Errorf("encode Claude Code hook payload: %w", err)
	}
	return qlogevent.Event{
		Source:      "claude-code-hook",
		SessionID:   sessionID,
		EventType:   eventType,
		OccurredAt:  time.Now().UTC(),
		ProjectHint: qlogevent.ProjectHint{CWD: cwd},
		Payload:     payload,
	}, nil
}
