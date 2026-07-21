package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newHookCommand() *cobra.Command {
	hook := &cobra.Command{Use: "hook", Short: "Receive local agent hook payloads"}
	hook.AddCommand(&cobra.Command{Use: "claude-code", Short: "Forward Claude Code lifecycle hooks", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		body, err := io.ReadAll(command.InOrStdin())
		if err != nil {
			return fmt.Errorf("read hook input: %w", err)
		}
		event, err := claudeCodeHookEvent(body)
		if err != nil {
			return err
		}
		endpoint := os.Getenv("QLOG_COLLECTOR_URL")
		if endpoint == "" {
			endpoint = "http://127.0.0.1:4318/v1/events"
		}
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
	}})
	return hook
}

func claudeCodeHookEvent(input []byte) (map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		return nil, fmt.Errorf("decode Claude Code hook JSON: %w", err)
	}
	sessionID, _ := raw["session_id"].(string)
	eventType, _ := raw["hook_event_name"].(string)
	if eventType == "" {
		eventType = "ClaudeCodeHook"
	}
	cwd, _ := raw["cwd"].(string)
	return map[string]any{
		"source":      "claude-code-hook",
		"session_id":  sessionID,
		"event_type":  eventType,
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
		"project_hint": map[string]any{
			"cwd": cwd,
		},
		"payload": map[string]any{
			"agent_name":      "claude-code",
			"capture_quality": "lifecycle_only",
		},
	}, nil
}
