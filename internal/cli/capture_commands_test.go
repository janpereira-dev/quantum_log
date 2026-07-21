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

	output, err := run("setup", "--dry-run", "--json")
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
