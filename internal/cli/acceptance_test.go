package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acceptancecontract "github.com/janpereira-dev/quantum_log/internal/acceptance"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
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

func TestAcceptanceRunKeepsSyntheticPostBoundaryEvidencePending(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	version := Version{Version: "0.4.0-rc11", Commit: strings.Repeat("a", 40)}
	boundaryID := beginAcceptance(t, home, version, "codex", "0.151.0")
	startedAt := time.Now().UTC()
	appendCodexAcceptanceEvidence(t, home, startedAt)
	output := filepath.Join(t.TempDir(), "acceptance.zip")
	command := New(version)
	command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", output, "--boundary", boundaryID})
	if err := command.Execute(); err != nil {
		t.Fatalf("acceptance run: %v", err)
	}
	entries := readAcceptanceZIP(t, output)
	var packaged []acceptancecontract.RealAgentEvidence
	if err := json.Unmarshal(entries["real-agent-evidence.json"], &packaged); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if len(packaged) != 1 || packaged[0].Status != acceptancecontract.StatusPendingExternalE2E || !packaged[0].SourceEvidence || packaged[0].PrivacyStatus != acceptancecontract.StatusPass || packaged[0].ReplayStatus != acceptancecontract.StatusPendingExternalE2E || packaged[0].CaptureQuality != "otel_reported" || len(packaged[0].ObservedMetrics) == 0 {
		t.Fatalf("packaged evidence = %#v", packaged)
	}
	command = New(version)
	command.SetArgs([]string{"acceptance", "inspect", "--package", output})
	if err := command.Execute(); err != nil {
		t.Fatalf("inspect exact package: %v", err)
	}
}

func appendCodexAcceptanceEvidence(t *testing.T, home string, occurredAt time.Time) {
	t.Helper()
	service, err := app.Open(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	sessionID := "session-" + strings.ReplaceAll(occurredAt.Format(time.RFC3339Nano), ":", "-")
	if err := service.Store.EnsureSession(context.Background(), sessionID, "codex", occurredAt); err != nil {
		t.Fatal(err)
	}
	raw, err := service.Store.AppendRawEvent(context.Background(), sqlite.RawEventInput{Source: "otlp-http", SessionID: sessionID, EventType: "model.call", OccurredAt: occurredAt, Payload: []byte(`{"agent_name":"codex","capture_quality":"otel_reported","codex_response_completed":true}`)})
	if err != nil || !raw.Accepted {
		t.Fatalf("append source evidence: %#v, %v", raw, err)
	}
	inputTokens, outputTokens := int64(1), int64(2)
	if _, err := service.Store.RecordModelCall(context.Background(), sqlite.ModelCallInput{RawEventID: raw.ID, SessionID: sessionID, AgentName: "codex", Provider: "openai", ModelID: "gpt-5", CaptureQuality: "otel_reported", InputTokens: inputTokens, OutputTokens: outputTokens, OccurredAt: occurredAt, Metrics: []sqlite.MetricInput{{Name: "input_tokens", Value: &inputTokens, Source: "otel", RawKey: "input_tokens", Confidence: "reported"}, {Name: "output_tokens", Value: &outputTokens, Source: "otel", RawKey: "output_tokens", Confidence: "reported"}}}); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptanceBoundaryExcludesStaleAndFutureLedgerRows(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		before     bool
		occurredAt func() time.Time
	}{{"stale", true, func() time.Time { return time.Now().UTC().Add(-time.Minute) }}, {"future", false, func() time.Time { return time.Now().UTC().Add(time.Hour) }}} {
		t.Run(scenario.name, func(t *testing.T) {
			home := t.TempDir()
			if _, err := runQLog(t, home, "init"); err != nil {
				t.Fatal(err)
			}
			if scenario.before {
				appendCodexAcceptanceEvidence(t, home, scenario.occurredAt())
			}
			version := Version{Version: "0.4.0-rc11", Commit: strings.Repeat("a", 40)}
			boundaryID := beginAcceptance(t, home, version, "codex", "0.151.0")
			if !scenario.before {
				appendCodexAcceptanceEvidence(t, home, scenario.occurredAt())
			}
			output := filepath.Join(t.TempDir(), "acceptance.zip")
			command := New(version)
			command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", output, "--boundary", boundaryID})
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			var packaged []acceptancecontract.RealAgentEvidence
			if err := json.Unmarshal(readAcceptanceZIP(t, output)["real-agent-evidence.json"], &packaged); err != nil {
				t.Fatal(err)
			}
			if len(packaged) != 1 || packaged[0].SourceEvidence || packaged[0].Status == acceptancecontract.StatusPass {
				t.Fatalf("%s evidence = %#v", scenario.name, packaged)
			}
		})
	}
}

func TestAcceptanceRunRejectsMismatchedAndReusedBoundary(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatal(err)
	}
	version := Version{Version: "0.4.0-rc11", Commit: strings.Repeat("a", 40)}
	boundaryID := beginAcceptance(t, home, version, "codex", "0.151.0")
	command := New(Version{Version: version.Version, Commit: strings.Repeat("b", 40)})
	command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", filepath.Join(t.TempDir(), "acceptance.zip"), "--boundary", boundaryID})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "exact qlog runtime") {
		t.Fatalf("error = %v, want exact candidate mismatch", err)
	}
	output := filepath.Join(t.TempDir(), "acceptance.zip")
	command = New(version)
	command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", output, "--boundary", boundaryID})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	command = New(version)
	command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", filepath.Join(t.TempDir(), "second.zip"), "--boundary", boundaryID})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "already been used") {
		t.Fatalf("reuse error = %v", err)
	}
}

func TestAcceptanceRunRejectsCallerGateSpoofingAndDuplicateBoundaryKeys(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatal(err)
	}
	version := Version{Version: "0.4.0-rc11", Commit: strings.Repeat("a", 40)}
	command := New(version)
	command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", filepath.Join(t.TempDir(), "acceptance.zip"), "--real-agent-evidence", `{"privacy_status":"PASS","replay_status":"PASS"}`})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("spoofing error = %v", err)
	}
	boundaryID := beginAcceptance(t, home, version, "codex", "0.151.0")
	path := filepath.Join(home, "acceptance", "boundaries", boundaryID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"agent_id":"codex"`), []byte(`"agent_id":"codex","agent_id":"codex"`), 1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	command = New(version)
	command.SetArgs([]string{"--home", home, "acceptance", "run", "--output", filepath.Join(t.TempDir(), "duplicate.zip"), "--boundary", boundaryID})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
		t.Fatalf("duplicate-key error = %v", err)
	}
}

func beginAcceptance(t *testing.T, home string, version Version, agentID, agentVersion string) string {
	t.Helper()
	output := new(bytes.Buffer)
	command := New(version)
	command.SetOut(output)
	command.SetArgs([]string{"--home", home, "acceptance", "begin", "--agent", agentID, "--agent-version", agentVersion})
	if err := command.Execute(); err != nil {
		t.Fatalf("begin acceptance: %v", err)
	}
	return strings.TrimSpace(output.String())
}

func readAcceptanceZIP(t *testing.T, path string) map[string][]byte {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	entries := make(map[string][]byte, len(archive.File))
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = data
	}
	return entries
}

func TestAcceptanceRunReadsSnapshotWhileCollectorWriterIsActive(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	collector, err := app.Open(context.Background(), home)
	if err != nil {
		t.Fatalf("open simulated collector writer: %v", err)
	}
	t.Cleanup(func() { _ = collector.Close() })

	output := filepath.Join(t.TempDir(), "acceptance.zip")
	if err := writeAcceptancePackage(context.Background(), home, Version{}, output); err != nil {
		t.Fatalf("acceptance run while collector writer is active: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("acceptance package missing: %v", err)
	}
}

func TestAcceptanceStatusTreatsMissingEventsAsPending(t *testing.T) {
	result := acceptanceAgentStatus("codex", false, false, false)
	if result.Status != acceptancePendingExternalE2E {
		t.Fatalf("missing evidence status = %s, want %s", result.Status, acceptancePendingExternalE2E)
	}
}

func TestAcceptanceExternalStatusPropagatesFailure(t *testing.T) {
	results := []acceptanceAgentResult{
		{AdapterID: "claude-code", Status: acceptancePendingExternalE2E},
		{AdapterID: "codex", Status: acceptanceFail},
	}
	if got := acceptanceExternalStatus(results); got != acceptanceFail {
		t.Fatalf("external status = %q, want %q", got, acceptanceFail)
	}
}

func TestAcceptancePrivacyScanRejectsForbiddenFieldsAndValues(t *testing.T) {
	for name, files := range map[string]map[string][]byte{
		"field": {"evidence.json": []byte(`{"prompt_body":"redacted"}`)},
		"value": {"report.txt": []byte("github_pat_example")},
	} {
		t.Run(name, func(t *testing.T) {
			if got := acceptancePackagePrivacyStatus(files, nil); got != acceptancecontract.StatusFail {
				t.Fatalf("privacy status = %q", got)
			}
		})
	}
}

func TestRealAgentRunnersCannotOverrideChecksOrPrintFalseSuccess(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "scripts", "acceptance", "real-agent-posix.sh"),
		filepath.Join("..", "..", "scripts", "acceptance", "real-agent-windows.ps1"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"QLOG_BIN", "PrivacyStatus", "ReplayStatus", "CandidateCommit", "CandidateTag"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s exposes forbidden override %s", path, forbidden)
			}
		}
		inspect := strings.Index(text, "acceptance inspect")
		success := strings.LastIndex(text, "Sanitized acceptance package verified")
		if !strings.Contains(text, "acceptance begin") || inspect < 0 || success < inspect || !strings.Contains(text, "exit") {
			t.Fatalf("%s can report success without the required boundary/inspection failure gates", path)
		}
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

func TestAcceptancePackageHashesUserControlledIdentifiers(t *testing.T) {
	home := t.TempDir()
	fixture := filepath.Join(t.TempDir(), "events.ndjson")
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture, []byte(`{"source":"fixture","session_id":"session-secret-123","event_type":"model.call","payload":{"provider":"provider-secret","model":"model-secret","agent_name":"agent-secret","input_tokens":1,"capture_quality":"agent_reported"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runQLog(t, home, "ingest", "file", fixture); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "acceptance.zip")
	if _, err := runQLog(t, home, "acceptance", "run", "--output", output); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(reader)
		_ = reader.Close()
		for _, identifier := range []string{"session-secret-123", "agent-secret", "provider-secret", "model-secret"} {
			if strings.Contains(data.String(), identifier) {
				t.Fatalf("acceptance entry %s leaked identifier %q: %s", file.Name, identifier, data.String())
			}
		}
	}
}

func TestAcceptanceSanitizesUnknownNestedVocabulary(t *testing.T) {
	report := acceptanceSafeReport(sqlite.CapabilityReport{
		Sources: []sqlite.SourceCoverage{
			{Source: "opencode-plugin", Quality: "agent_reported"},
			{Source: "source-secret", Quality: "quality-secret"},
		},
		MetricCoverage: []sqlite.MetricCoverage{{Name: "input_tokens", Provenance: []sqlite.MetricProvenance{
			{Source: "opencode", RawKey: "tokens.reasoning", Confidence: "reported"},
			{Source: "opencode", RawKey: "tokens.cache.write", Confidence: "reported"},
			{Source: "source-secret", RawKey: "raw-key-secret", Confidence: "confidence-secret"},
		}}},
	})
	jsonData, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	csvData := new(bytes.Buffer)
	if err := writeCapabilityCSV(csvData, report); err != nil {
		t.Fatal(err)
	}
	textData := new(bytes.Buffer)
	if err := writeCapabilityReport(textData, report); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{jsonData, csvData.Bytes(), textData.Bytes()} {
		for _, secret := range []string{"source-secret", "quality-secret", "raw-key-secret", "confidence-secret"} {
			if strings.Contains(string(data), secret) {
				t.Fatalf("acceptance export leaked %q: %s", secret, data)
			}
		}
	}
	if report.Sources[0].Quality != "agent_reported" || report.MetricCoverage[0].Provenance[0].RawKey != "tokens.reasoning" || report.MetricCoverage[0].Provenance[1].RawKey != "tokens.cache.write" {
		t.Fatalf("allowlisted acceptance vocabulary changed: %#v", report)
	}
	if !strings.HasPrefix(report.Sources[1].Quality, "sha256:") {
		t.Fatalf("unknown capture quality was not opaque: %#v", report.Sources[1])
	}
}
