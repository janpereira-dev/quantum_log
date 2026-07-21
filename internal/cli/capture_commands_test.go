package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
)

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
	if err := command.Execute(); err != nil {
		t.Fatalf("adapter verify: %v", err)
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
	if err != nil {
		t.Fatalf("adapter verify: %v", err)
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
		if stage.Name == "copilot_model_call" {
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

func TestCollectorRejectsPublicBindingWithoutExplicitOptIn(t *testing.T) {
	if err := validateListenAddress("0.0.0.0:4318", false); err == nil {
		t.Fatal("public binding was accepted")
	}
	if err := validateListenAddress("127.0.0.1:4318", false); err != nil {
		t.Fatalf("loopback binding rejected: %v", err)
	}
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
	output, err := run("collector", "status", "--json")
	if err != nil {
		t.Fatalf("collector status: %v", err)
	}
	var status struct {
		Listen    string   `json:"listen"`
		Endpoints []string `json:"endpoints"`
	}
	if err := json.Unmarshal([]byte(output), &status); err != nil || status.Listen != "127.0.0.1:4318" || len(status.Endpoints) != 2 {
		t.Fatalf("collector status output = %q, %#v, %v", output, status, err)
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
	var plans []adapters.SetupPlan
	if err := json.Unmarshal([]byte(output), &plans); err != nil {
		t.Fatalf("decode setup output = %q: %v", output, err)
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %#v", plans)
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
