package jsonl

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestSecurityNDJSONUsesMetadataContractInsteadOfContentAliases(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "qlog.db")
	store, err := storepkg.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	line := `{"source":"fixture","session_id":"synthetic","event_type":"model.call","project_resolution_evidence":{"source":"fixture","note":"Bearer MOCK_EVIDENCE_SECRET","toolArguments":{"customerEmail":"alice@example.invalid"}},"payload":{"provider":"example","model":"model","agent_name":"fixture","input_tokens":0,"metric_observations":[{"name":"input_tokens","value":0,"source":"fixture","raw_key":"usage.input_tokens","confidence":"reported","note":"Bearer MOCK_METRIC_SECRET"}],"toolArguments":{"customerEmail":"alice@example.invalid"},"tool_arguments":{"value":"MOCK_ARGUMENT_SECRET"},"tool-arguments":{"value":"MOCK_ARGUMENT_SECRET"},"note":"Bearer MOCK_RAW_SECRET"}}`
	if _, err := Import(ctx, store, strings.NewReader(line)); err != nil {
		t.Fatal(err)
	}
	reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(database)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var payload, evidence string
	if err := reader.QueryRow(`SELECT payload_json_sanitized,project_resolution_evidence_json FROM raw_events WHERE source='fixture'`).Scan(&payload, &evidence); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"MOCK_", "alice@example.invalid", "toolArguments", "tool_arguments", "tool-arguments", `"note"`} {
		if strings.Contains(payload+evidence, forbidden) {
			t.Fatalf("content alias retained: payload=%s evidence=%s", payload, evidence)
		}
	}
	if !strings.Contains(payload, `"input_tokens":0`) || strings.Contains(payload, `"output_tokens"`) || !strings.Contains(payload, `"raw_key":"usage.input_tokens"`) {
		t.Fatalf("metadata/missing-vs-zero changed: %s", payload)
	}
}
