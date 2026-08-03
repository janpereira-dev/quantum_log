package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/domain"
	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestUnsupportedAdaptersAreNotSelectedByDefaultSetup(t *testing.T) {
	items, err := setupDefaultAdapters(context.Background(), adapters.Default().List())
	if err != nil {
		t.Fatal(err)
	}
	for _, adapter := range items {
		switch adapter.Descriptor().ID {
		case "pi", "openclaw", "hermes":
			t.Fatalf("unsupported adapter selected: %s", adapter.Descriptor().ID)
		}
	}
}

func TestAdapterCommandsExposeCapabilitiesAndSafeDryRun(t *testing.T) {
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	output, err := run("adapter", "list", "--json")
	if err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("adapter list = %q, %v", output, err)
	}
	output, err = run("adapter", "install", "opencode", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("adapter install dry run: %v", err)
	}
	var result struct {
		Changed bool `json:"changed"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil || result.Changed {
		t.Fatalf("dry-run result = %q, %#v, %v", output, result, err)
	}
}

func TestAdapterVerifyCopilotReportsMissingEvidence(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	command := New(Version{})
	output := new(bytes.Buffer)
	command.SetArgs([]string{"--home", home, "adapter", "verify", "copilot-vscode", "--json"})
	setOutput(command, output)
	if err := command.Execute(); err == nil {
		t.Fatalf("adapter verify succeeded: %s", output)
	}
	var result struct {
		AdapterID string `json:"adapter_id"`
		Ready     bool   `json:"ready"`
		Stages    []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("output = %s: %v", output.String(), err)
	}
	if result.AdapterID != "copilot-vscode" || result.Ready || len(result.Stages) == 0 {
		t.Fatalf("verify result = %#v", result)
	}
}

func TestAdapterVerifyReturnsNonZeroForMissingRequiredEvidence(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	output, err := runQLog(t, home, "adapter", "verify", "opencode", "--json")
	if err == nil {
		t.Fatalf("verify succeeded: %s", output)
	}
	if !strings.Contains(output, `"ready":false`) {
		t.Fatalf("output = %s", output)
	}
}

func TestAdapterVerifyCopilotInstalledSettingsAreNotEnough(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(append([]string{"--home", home}, args...))
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	if _, err := run("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run("adapter", "install", "copilot-vscode", "--json"); err != nil {
		t.Fatalf("install copilot-vscode: %v", err)
	}
	output, err := run("adapter", "verify", "copilot-vscode", "--since", "1h", "--json")
	if err == nil {
		t.Fatalf("adapter verify succeeded: %s", output)
	}
	var result struct {
		Ready  bool `json:"ready"`
		Stages []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"stages"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output = %s: %v", output, err)
	}
	if result.Ready {
		t.Fatalf("installed settings verified without local Copilot evidence: %#v", result)
	}
	foundEvidenceStage := false
	for _, stage := range result.Stages {
		if stage.Name == "raw_evidence" {
			foundEvidenceStage = true
			if stage.Passed {
				t.Fatalf("copilot evidence stage passed without local evidence: %#v", result)
			}
		}
	}
	if !foundEvidenceStage {
		t.Fatalf("verify result missing evidence stage: %#v", result)
	}
}

func TestAdapterVerifyCopilotRejectsGenericIngestedUsage(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runQLog(t, home, "adapter", "install", "copilot-vscode"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := runQLog(t, home, "project", "register", "--path", t.TempDir(), "--name", "Project", "--slug", "project"); err != nil {
		t.Fatalf("register: %v", err)
	}
	projectOutput, err := runQLog(t, home, "project", "show", "project", "--json")
	if err != nil {
		t.Fatalf("project show: %v", err)
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(projectOutput), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	fixture := filepath.Join(t.TempDir(), "fake-copilot.ndjson")
	event := `{"source":"fixture","session_id":"session","event_type":"model.call","project_id":"` + project.ID + `","payload":{"provider":"github","model":"gpt-5","agent_name":"GitHub Copilot Chat","input_tokens":1,"output_tokens":2,"capture_quality":"otel_reported"}}` + "\n"
	if err := os.WriteFile(fixture, []byte(event), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := runQLog(t, home, "ingest", "file", fixture); err != nil {
		t.Fatalf("ingest fake copilot: %v", err)
	}

	output, err := runQLog(t, home, "adapter", "verify", "copilot-vscode", "--project", "project", "--json")
	if err == nil {
		t.Fatalf("adapter verify succeeded: %s", output)
	}
	var result struct {
		Ready bool `json:"ready"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode verify: %v", err)
	}
	if result.Ready {
		t.Fatalf("generic ingested usage verified Copilot: %s", output)
	}
}

func TestAdapterVerifyCopilotRejectsSpoofedOTLPHTTPImport(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	fixture := filepath.Join(t.TempDir(), "fake-copilot.ndjson")
	event := `{"source":"otlp-http","session_id":"session","event_type":"model.call","payload":{"provider":"github","model":"gpt-5","agent_name":"GitHub Copilot Chat","input_tokens":1,"output_tokens":2,"capture_quality":"otel_reported"}}` + "\n"
	if err := os.WriteFile(fixture, []byte(event), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := runQLog(t, home, "ingest", "file", fixture); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("spoofed otlp-http import was accepted: %v", err)
	}
}

func TestAdapterVerifyCopilotAcceptsSanctionedOTLPEvidence(t *testing.T) {
	home, serverURL := setupCopilotVerification(t)
	postCopilotOTLPTrace(t, serverURL, `,{"key":"gen_ai.provider.name","value":{"stringValue":"github"}}`)

	output, err := runQLog(t, home, "adapter", "verify", "copilot-vscode", "--project", "project", "--json")
	if err != nil {
		t.Fatalf("adapter verify failed: %s: %v", output, err)
	}
	var result struct {
		Ready bool `json:"ready"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode verify: %v", err)
	}
	if !result.Ready {
		t.Fatalf("sanctioned Copilot OTLP usage did not verify: %s", output)
	}
}

func TestAdapterVerifyCopilotRejectsMissingOrNonGitHubProvider(t *testing.T) {
	for _, test := range []struct {
		name              string
		providerAttribute string
	}{
		{name: "provider omitted", providerAttribute: ""},
		{name: "provider openai", providerAttribute: `,{"key":"gen_ai.provider.name","value":{"stringValue":"openai"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			home, serverURL := setupCopilotVerification(t)
			postCopilotOTLPTrace(t, serverURL, test.providerAttribute)

			output, err := runQLog(t, home, "adapter", "verify", "copilot-vscode", "--project", "project", "--json")
			if err == nil || !strings.Contains(output, `"ready":false`) {
				t.Fatalf("Copilot evidence incorrectly verified: output=%s err=%v", output, err)
			}
		})
	}
}

func setupCopilotVerification(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	fakeCode := "code"
	if runtime.GOOS == "windows" {
		fakeCode += ".exe"
	}
	codeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(codeDir, fakeCode), nil, 0o700); err != nil {
		t.Fatalf("create fake code executable: %v", err)
	}
	t.Setenv("PATH", codeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runQLog(t, home, "adapter", "install", "copilot-vscode"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := runQLog(t, home, "project", "register", "--path", filepath.Join(t.TempDir(), "project"), "--name", "Project", "--slug", "project"); err != nil {
		t.Fatalf("register: %v", err)
	}
	server := httptest.NewServer(newCollectorMux(home))
	t.Cleanup(server.Close)
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/traces")
	return home, server.URL
}

func postCopilotOTLPTrace(t *testing.T, serverURL, providerAttribute string) {
	t.Helper()
	payload := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"copilot-chat"}}]},"scopeSpans":[{"spans":[{"traceId":"trace-copilot","spanId":"span-copilot","attributes":[{"key":"qlog.project","value":{"stringValue":"project"}}` + providerAttribute + `,{"key":"gen_ai.agent.name","value":{"stringValue":"GitHub Copilot Chat"}},{"key":"gen_ai.request.model","value":{"stringValue":"gpt-5"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"1"}},{"key":"gen_ai.usage.output_tokens","value":{"intValue":"2"}}]}]}]}]}`
	request, err := http.NewRequest(http.MethodPost, serverURL+"/v1/traces", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("collector request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("collector response = %d", response.StatusCode)
	}
}

func TestAdapterVerifyCodexRejectsGenericOTLPEvidence(t *testing.T) {
	home, project := setupCodexVerification(t)
	service, err := app.Open(context.Background(), home)
	if err != nil {
		t.Fatalf("open service: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	now := time.Now().UTC()
	if err := service.Store.EnsureSession(context.Background(), "generic-codex", "codex", now); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	raw, err := service.Store.AppendRawEvent(context.Background(), sqlite.RawEventInput{
		Source: "otlp-http", SessionID: "generic-codex", EventType: "model.call", OccurredAt: now,
		Payload: []byte(`{"agent_name":"codex","capture_quality":"otel_reported"}`),
	})
	if err != nil || !raw.Accepted {
		t.Fatalf("append raw event = %#v, %v", raw, err)
	}
	if _, err := service.Store.RecordModelCall(context.Background(), sqlite.ModelCallInput{
		RawEventID: raw.ID, ProjectID: project.ID, SessionID: "generic-codex", AgentName: "codex", Provider: "openai", ModelID: "gpt-5", InputTokens: 1, CaptureQuality: "otel_reported", OccurredAt: now,
	}); err != nil {
		t.Fatalf("record model call: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}

	output, err := runQLog(t, home, "adapter", "verify", "codex", "--project", "project", "--json")
	if err == nil || !strings.Contains(output, `"ready":false`) {
		t.Fatalf("generic Codex OTLP evidence verified: output=%s err=%v", output, err)
	}
}

func TestAdapterVerifyCodexAcceptsNormalizedResponseCompletedEvidence(t *testing.T) {
	home, _ := setupCodexVerification(t)
	server := httptest.NewServer(newCollectorMux(home))
	t.Cleanup(server.Close)
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/logs")

	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/logs", strings.NewReader(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"codex"}}]},"scopeLogs":[{"logRecords":[{"traceId":"codex-trace","spanId":"codex-span","attributes":[{"key":"event.name","value":{"stringValue":"codex.sse_event"}},{"key":"event.kind","value":{"stringValue":"response.completed"}},{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"model","value":{"stringValue":"gpt-5"}},{"key":"input_tokens","value":{"intValue":"1"}},{"key":"output_tokens","value":{"intValue":"2"}}]}]}]}]}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("collector request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("collector response = %d", response.StatusCode)
	}

	output, err := runQLog(t, home, "adapter", "verify", "codex", "--project", "project", "--json")
	if err != nil || !strings.Contains(output, `"ready":true`) {
		t.Fatalf("normalized Codex response.completed evidence did not verify: output=%s err=%v", output, err)
	}
}

func setupCodexVerification(t *testing.T) (string, domain.Project) {
	t.Helper()
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	codeDir := t.TempDir()
	codex := "codex"
	if runtime.GOOS == "windows" {
		codex += ".exe"
	}
	if err := os.WriteFile(filepath.Join(codeDir, codex), nil, 0o700); err != nil {
		t.Fatalf("create fake codex executable: %v", err)
	}
	t.Setenv("PATH", codeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runQLog(t, home, "adapter", "install", "codex"); err != nil {
		t.Fatalf("install Codex: %v", err)
	}
	service, err := app.Open(context.Background(), home)
	if err != nil {
		t.Fatalf("open service: %v", err)
	}
	project, _, err := service.Store.RegisterProject(context.Background(), "Project", "project", t.TempDir())
	if err != nil {
		_ = service.Close()
		t.Fatalf("register project: %v", err)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}
	return home, project
}

func TestCollectorRejectsPublicBindingWithoutExplicitOptIn(t *testing.T) {
	if err := validateListenAddress("0.0.0.0:4318", false); err == nil {
		t.Fatal("public binding was accepted")
	}
	if err := validateListenAddress("127.0.0.1:4318", false); err != nil {
		t.Fatalf("loopback binding rejected: %v", err)
	}
}

func TestVerifyCollectorReachabilityUsesConfiguredURLOrigin(t *testing.T) {
	for _, configuredPath := range []string{"/v1/events", "/v1/logs"} {
		t.Run(configuredPath, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/healthz" {
					t.Fatalf("probe = %s %s", request.Method, request.URL.Path)
				}
				writer.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)
			t.Setenv("QLOG_COLLECTOR_URL", server.URL+configuredPath)

			reachable, message := verifyCollectorReachability(context.Background())
			if !reachable {
				t.Fatalf("reachable = false: %s", message)
			}
		})
	}
}

func TestCollectorLifecycleCommandResolvesDefaultHome(t *testing.T) {
	home := ""
	listen := defaultCollectorListen
	var receivedHome string
	command := collectorLifecycleCommand("test", "test", func(_ collectorManager, resolvedHome, _ string) (CollectorStatus, error) {
		receivedHome = resolvedHome
		return CollectorStatus{}, nil
	}, &home, &listen)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if receivedHome == "" || !filepath.IsAbs(receivedHome) {
		t.Fatalf("resolved lifecycle home = %q", receivedHome)
	}
}

func TestAdapterStatusAddsRuntimeEvidence(t *testing.T) {
	base := adapters.SetupStatus{AdapterID: "copilot-vscode", CaptureQuality: adapters.CaptureOTELReported}
	for _, test := range []struct {
		name      string
		access    fakeAdapterStatusAccess
		reachable bool
		evidence  bool
	}{
		{name: "reachable collector with recent evidence", access: fakeAdapterStatusAccess{reachable: true, evidence: true}, reachable: true, evidence: true},
		{name: "unreachable collector without evidence", access: fakeAdapterStatusAccess{reachable: false, evidence: false}},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := enrichAdapterStatus(context.Background(), t.TempDir(), base, test.access)
			if status.CollectorReachable != test.reachable || status.RecentEvidence != test.evidence {
				t.Fatalf("status = %#v", status)
			}
		})
	}
}

type fakeAdapterStatusAccess struct {
	reachable bool
	evidence  bool
}

func (f fakeAdapterStatusAccess) CollectorReachable(context.Context) bool { return f.reachable }

func (f fakeAdapterStatusAccess) HasRecentEvidence(context.Context, string, adapters.SetupStatus) (bool, error) {
	return f.evidence, nil
}

func TestCollectorStatusShowsLocalEndpoints(t *testing.T) {
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	output, err := run("collector", "status", "--json", "--listen", "127.0.0.1:1")
	if err != nil {
		t.Fatalf("collector status: %v", err)
	}
	var status struct {
		Listen    string   `json:"listen"`
		Home      string   `json:"home"`
		Database  string   `json:"database"`
		Endpoints []string `json:"endpoints"`
		Reachable bool     `json:"reachable"`
		Running   bool     `json:"running"`
		Health    string   `json:"health"`
	}
	if err := json.Unmarshal([]byte(output), &status); err != nil || status.Listen != "127.0.0.1:1" || len(status.Endpoints) != 4 || !containsString(status.Endpoints, "/v1/logs") || !containsString(status.Endpoints, "/healthz") || status.Home == "" || status.Database == "" || status.Reachable || status.Running || status.Health == "" {
		t.Fatalf("collector status output = %q, %#v, %v", output, status, err)
	}
}

func TestCodexEvidenceContractUsesDocumentedOTLPLogs(t *testing.T) {
	contract := evidenceContract("codex")
	if contract.Source != "otlp-http" || contract.Quality != adapters.CaptureOTELReported || !contract.RequireCodexResponseCompleted || !contract.SourceEvidence {
		t.Fatalf("Codex evidence contract = %#v", contract)
	}
}

func TestCollectorLifecycleCommandsExist(t *testing.T) {
	command := New(Version{})
	collector, _, err := command.Find([]string{"collector"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"install", "start", "stop", "restart", "logs", "uninstall"} {
		found := false
		for _, child := range collector.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("collector command %q not found", name)
		}
	}
}

func TestHookClaudeCodePostsLifecycleEvent(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/events" {
			t.Fatalf("path = %s", request.URL.Path)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		received = body
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"accepted":1}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/events")
	command := New(Version{})
	output := new(bytes.Buffer)
	command.SetArgs([]string{"hook", "claude-code"})
	command.SetIn(strings.NewReader(`{"session_id":"session-1","hook_event_name":"Stop","cwd":"C:/repo","transcript_path":"must-not-forward"}`))
	setOutput(command, output)
	if err := command.Execute(); err != nil {
		t.Fatalf("hook claude-code: %v output=%q", err, output.String())
	}
	if !bytes.Contains(received, []byte(`"source":"claude-code-hook"`)) || !bytes.Contains(received, []byte(`"capture_quality":"lifecycle_only"`)) || bytes.Contains(received, []byte("transcript_path")) {
		t.Fatalf("posted body = %s", received)
	}
}

func TestHookClaudeCodeIngestsDirectlyByDefault(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "project")
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runQLog(t, home, "project", "register", "--path", worktree, "--name", "Project", "--slug", "project"); err != nil {
		t.Fatalf("register: %v", err)
	}
	output, err := runQLogWithInput(t, home, strings.NewReader(`{"session_id":"session-1","hook_event_name":"Stop","cwd":"`+filepath.ToSlash(worktree)+`","prompt":"must-not-store","transcript_path":"must-not-store"}`), "hook", "claude-code")
	if err != nil {
		t.Fatalf("hook claude-code: %v output=%q", err, output)
	}
	if !strings.Contains(output, "hook: ingested") {
		t.Fatalf("hook output = %q", output)
	}
	verify, err := runQLog(t, home, "verify")
	if err != nil {
		t.Fatalf("verify after hook ingest: %v output=%q", err, verify)
	}
	report, err := runQLog(t, home, "report", "usage", "--group-by", "project,agent,capture_quality", "--json")
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	var usage struct {
		Rows        []any `json:"rows"`
		TotalTokens int64 `json:"total_tokens"`
	}
	if err := json.Unmarshal([]byte(report), &usage); err != nil {
		t.Fatalf("decode usage = %s: %v", report, err)
	}
	if len(usage.Rows) != 0 || usage.TotalTokens != 0 {
		t.Fatalf("Claude lifecycle event invented usage: %s", report)
	}
}

func TestAdapterStatusTestAndUninstallCommands(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}

	output, err := run("adapter", "status", "--json")
	if err != nil {
		t.Fatalf("adapter status: %v", err)
	}
	var statuses []adapters.SetupStatus
	if err := json.Unmarshal([]byte(output), &statuses); err != nil || len(statuses) == 0 {
		t.Fatalf("adapter status output = %q, %#v, %v", output, statuses, err)
	}

	output, err = run("adapter", "status", "opencode", "--json")
	if err != nil {
		t.Fatalf("adapter status opencode: %v", err)
	}
	var status adapters.SetupStatus
	if err := json.Unmarshal([]byte(output), &status); err != nil || status.AdapterID != "opencode" || status.CaptureQuality == "" {
		t.Fatalf("adapter status opencode output = %q, %#v, %v", output, status, err)
	}

	output, err = run("adapter", "test", "opencode", "--json")
	if err != nil {
		t.Fatalf("adapter test opencode: %v", err)
	}
	var result adapters.TestResult
	if err := json.Unmarshal([]byte(output), &result); err != nil || result.AdapterID != "opencode" || result.CaptureQuality == "" {
		t.Fatalf("adapter test output = %q, %#v, %v", output, result, err)
	}

	output, err = run("adapter", "uninstall", "opencode", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("adapter uninstall opencode: %v", err)
	}
	var uninstall adapters.InstallResult
	if err := json.Unmarshal([]byte(output), &uninstall); err != nil || uninstall.Changed {
		t.Fatalf("adapter uninstall output = %q, %#v, %v", output, uninstall, err)
	}
}

func TestSetupCommandPlansInstallsAndIsIdempotent(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}

	output, err := run("setup", "opencode", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("setup dry-run: %v", err)
	}
	var plans []adapters.SetupPlan
	if err := json.Unmarshal([]byte(output), &plans); err != nil || len(plans) == 0 {
		t.Fatalf("setup dry-run output = %q, %#v, %v", output, plans, err)
	}

	output, err = run("setup", "opencode", "--yes", "--json")
	if err != nil {
		t.Fatalf("setup opencode: %v", err)
	}
	var installed []adapters.SetupPlan
	if err := json.Unmarshal([]byte(output), &installed); err != nil || len(installed) != 1 || installed[0].AdapterID != "opencode" {
		t.Fatalf("setup opencode output = %q, %#v, %v", output, installed, err)
	}
	configPath := filepath.Join(configHome, ".config", "opencode", "plugins", "quantum-log.ts")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read opencode plugin file: %v", err)
	}
	if !strings.Contains(string(contents), "/v1/events") || !strings.Contains(string(contents), "QuantumLogPlugin") {
		t.Fatalf("opencode plugin missing event forwarding: %q", contents)
	}

	output, err = run("setup", "opencode", "--yes", "--json")
	if err != nil {
		t.Fatalf("setup opencode second run: %v", err)
	}
	var rerun []adapters.SetupPlan
	if err := json.Unmarshal([]byte(output), &rerun); err != nil || len(rerun) != 1 || len(rerun[0].Changes) != 1 || rerun[0].Changes[0].Action != "unchanged" {
		t.Fatalf("setup opencode rerun = %q, %#v, %v", output, rerun, err)
	}
}

func TestSetupDefaultWithoutAllSkipsUnavailableAdapters(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	t.Setenv("PATH", "")
	previousManager := newSetupCollectorManager
	newSetupCollectorManager = func() collectorManager { return &fakeCollectorManager{} }
	t.Cleanup(func() { newSetupCollectorManager = previousManager })
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	output, err := run("setup", "--yes", "--json")
	if err != nil {
		t.Fatalf("setup default: %v", err)
	}
	var result BootstrapResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode setup output = %q: %v", output, err)
	}
	if !result.Consent || len(result.Adapters) != 4 {
		t.Fatalf("bootstrap result = %#v", result)
	}
	for _, plan := range result.Adapters {
		if len(plan.Changes) != 1 || plan.Changes[0].Action != "skipped" {
			t.Fatalf("adapter plan = %#v", plan)
		}
	}
	if _, err := os.Stat(filepath.Join(configHome, ".config")); !os.IsNotExist(err) {
		t.Fatalf("default setup created config for unavailable adapters: %v", err)
	}
}

func TestSetupAppliedJSONPreservesPathAndBackup(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	run := func(args ...string) (string, error) {
		command := New(Version{})
		output := new(bytes.Buffer)
		command.SetArgs(args)
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	if _, err := run("setup", "opencode", "--yes", "--json"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	pluginPath := filepath.Join(configHome, ".config", "opencode", "plugins", "quantum-log.ts")
	if err := os.WriteFile(pluginPath, []byte("custom"), 0o600); err != nil {
		t.Fatalf("modify plugin: %v", err)
	}
	output, err := run("setup", "opencode", "--yes", "--json")
	if err != nil {
		t.Fatalf("second setup: %v", err)
	}
	var plans []adapters.SetupPlan
	if err := json.Unmarshal([]byte(output), &plans); err != nil || len(plans) != 1 || len(plans[0].Changes) != 1 {
		t.Fatalf("decode setup output = %q %#v %v", output, plans, err)
	}
	change := plans[0].Changes[0]
	if change.Path != pluginPath || change.BackupPath == "" || change.Action != "updated" {
		t.Fatalf("applied change = %#v", change)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
