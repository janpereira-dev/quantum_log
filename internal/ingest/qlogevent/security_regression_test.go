package qlogevent

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
)

func TestSecurityPromptPolicyAppliesToEveryEventType(t *testing.T) {
	for _, mode := range []string{"off", "hash"} {
		for _, kind := range []string{"agent.event", "model.call", "interaction.prompt"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				ctx := context.Background()
				service, err := app.Initialize(ctx, t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer service.Close()
				if err := config.SetPromptCaptureMode(service.Paths, mode); err != nil {
					t.Fatal(err)
				}
				_, err = Ingest(ctx, service, Event{Source: "audit", SessionID: "synthetic", EventType: kind, ProjectHint: ProjectHint{CWD: t.TempDir()}, Payload: json.RawMessage(`{"interaction_redacted":"MOCK_PRIVATE_PROMPT","prompt_capture_mode":"full","prompt_hash":"untrusted-public-hash"}`)})
				if err != nil {
					t.Fatal(err)
				}
				reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(service.Paths.Database)+"?mode=ro")
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				var payload string
				if err := reader.QueryRow(`SELECT payload_json_sanitized FROM raw_events WHERE source='audit'`).Scan(&payload); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(payload, "MOCK_PRIVATE_PROMPT") || strings.Contains(payload, `"prompt_capture_mode":"full"`) || mode == "off" && strings.Contains(payload, "untrusted-public-hash") {
					t.Fatalf("privacy mode %s bypassed: %s", mode, payload)
				}
			})
		}
	}
}
