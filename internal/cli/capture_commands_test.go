package cli

import (
	"bytes"
	"encoding/json"
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
	configPath := filepath.Join(configHome, ".config", "opencode", "AGENTS.md")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read opencode setup file: %v", err)
	}
	if !strings.Contains(string(contents), "qlog:begin agent-auto-capture") {
		t.Fatalf("opencode setup file missing marker: %q", contents)
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
