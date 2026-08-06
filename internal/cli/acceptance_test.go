package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptanceRunWritesSanitizedEvidencePackage(t *testing.T) {
	home := t.TempDir()
	fixture := filepath.Join(t.TempDir(), "events.ndjson")
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(fixture, []byte(`{"source":"fixture","session_id":"session-1","event_type":"session.completed","payload":{"agent_name":"opencode","capture_quality":"lifecycle_only","prompt":"must-not-export","tool_args":"must-not-export","authorization":"must-not-export"}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := runQLog(t, home, "ingest", "file", fixture); err != nil {
		t.Fatalf("ingest fixture: %v", err)
	}

	output := filepath.Join(t.TempDir(), "acceptance.zip")
	if _, err := runQLog(t, home, "acceptance", "run", "--output", output); err != nil {
		t.Fatalf("acceptance run: %v", err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("open acceptance package: %v", err)
	}
	defer func() { _ = archive.Close() }()
	entries := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		data := new(bytes.Buffer)
		if _, err := data.ReadFrom(reader); err != nil {
			_ = reader.Close()
			t.Fatalf("read %s: %v", file.Name, err)
		}
		_ = reader.Close()
		entries[file.Name] = data.Bytes()
	}
	for _, name := range []string{"manifest.json", "report.json", "report.csv", "report.txt", "sessions.json", "diagnostics.json", "SHA256SUMS"} {
		if _, found := entries[name]; !found {
			t.Errorf("package missing %s", name)
		}
	}
	for name, data := range entries {
		if strings.Contains(string(data), "must-not-export") {
			t.Errorf("package entry %s leaked sensitive fixture data", name)
		}
	}
	if !strings.Contains(string(entries["manifest.json"]), "PENDING_EXTERNAL_E2E") {
		t.Errorf("manifest missing pending external E2E status: %s", entries["manifest.json"])
	}
	var manifest struct {
		ImplementationStatus string `json:"implementation_status"`
		Agents               []struct {
			AdapterID string `json:"adapter_id"`
			Source    string `json:"source"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.ImplementationStatus != acceptanceImplementationComplete || len(manifest.Agents) != 5 {
		t.Fatalf("manifest = %#v", manifest)
	}
	for _, agent := range manifest.Agents {
		if agent.AdapterID == "" || agent.Source == "" {
			t.Fatalf("agent capability matrix entry = %#v", agent)
		}
	}
}

func TestAcceptanceStatusTreatsMissingEventsAsPending(t *testing.T) {
	result := acceptanceAgentStatus("codex", false, false, false)
	if result.Status != acceptancePendingExternalE2E {
		t.Fatalf("missing evidence status = %s, want %s", result.Status, acceptancePendingExternalE2E)
	}
}

func TestAcceptanceOwnedPathRejectsCollectorLogOutsideHome(t *testing.T) {
	home := t.TempDir()
	inside := filepath.Join(home, "collector.log")
	if err := os.WriteFile(inside, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "collector.log")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !acceptanceOwnedPath(home, inside) || acceptanceOwnedPath(home, outside) {
		t.Fatalf("owned path check accepted outside collector log")
	}
}
