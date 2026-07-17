package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCoreCommandsInitializeAndReportProject(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "quantum_log")

	run := func(args ...string) (string, error) {
		command := New(Version{Version: "0.1.0", Commit: "test", BuildDate: "2026-07-16"})
		output := new(bytes.Buffer)
		command.SetArgs(append([]string{"--home", home}, args...))
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}

	if _, err := run("init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	if _, err := run("project", "register", "--path", worktree, "--name", "QUANTUM_LOG"); err != nil {
		t.Fatalf("qlog project register: %v", err)
	}
	output, err := run("project", "current", "--project", "quantum-log", "--json")
	if err != nil {
		t.Fatalf("qlog project current: %v", err)
	}
	var current struct {
		ProjectSlug  string `json:"project_slug"`
		Method       string `json:"method"`
		LocationPath string `json:"location_path"`
	}
	if err := json.Unmarshal([]byte(output), &current); err != nil {
		t.Fatalf("project current JSON: %v; output=%q", err, output)
	}
	wantLocation, err := filepath.Abs(worktree)
	if err != nil {
		t.Fatalf("resolve expected location: %v", err)
	}
	if current.ProjectSlug != "quantum-log" || current.Method != "explicit" || current.LocationPath != wantLocation {
		t.Fatalf("project current = %#v", current)
	}
	fixture := filepath.Join(t.TempDir(), "events.ndjson")
	if err := os.WriteFile(fixture, []byte(`{"source":"fixture","session_id":"s-1","event_type":"model.call","payload":{"tokens":1}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := run("ingest", "file", fixture); err != nil {
		t.Fatalf("qlog ingest file: %v", err)
	}

	if output, err := run("doctor", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("qlog doctor --json = %q, %v", output, err)
	}
	if _, err := run("verify"); err != nil {
		t.Fatalf("qlog verify: %v", err)
	}
}

func setOutput(command *cobra.Command, output io.Writer) {
	command.SetOut(output)
	command.SetErr(output)
	for _, child := range command.Commands() {
		setOutput(child, output)
	}
}

func TestVersionIncludesBuildMetadata(t *testing.T) {
	command := New(Version{Version: "0.1.0", Commit: "abc123", BuildDate: "2026-07-16"})
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetArgs([]string{"--version"})
	if err := command.Execute(); err != nil {
		t.Fatalf("qlog --version: %v", err)
	}
	if got := output.String(); got != "qlog 0.1.0 (commit abc123, built 2026-07-16)\n" {
		t.Fatalf("version output = %q", got)
	}
}

func TestRootWithoutArgumentsKeepsHelpForNonTerminalOutput(t *testing.T) {
	command := New(Version{Version: "0.1.0", Commit: "test", BuildDate: "2026-07-16"})
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetErr(output)
	if err := command.Execute(); err != nil {
		t.Fatalf("qlog without arguments: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("Usage:")) {
		t.Fatalf("non-terminal output should show help, got %q", output.String())
	}
}

func TestMilestoneTwoCommandWorkflow(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "project")
	run := func(args ...string) (string, error) {
		command := New(Version{Version: "0.1.0", Commit: "test", BuildDate: "2026-07-16"})
		output := new(bytes.Buffer)
		command.SetArgs(append([]string{"--home", home}, args...))
		setOutput(command, output)
		err := command.Execute()
		return output.String(), err
	}
	if _, err := run("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run("project", "register", "--path", worktree, "--name", "Project", "--slug", "project"); err != nil {
		t.Fatalf("register project: %v", err)
	}
	if _, err := run("project", "tag", "environment=work", "--project", "project"); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	if output, err := run("project", "tag", "list", "--project", "project", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("list tags = %q, %v", output, err)
	}
	projectOutput, err := run("project", "show", "project", "--json")
	if err != nil {
		t.Fatalf("show project: %v", err)
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(projectOutput), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if output, err := run("project", "list", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("list projects = %q, %v", output, err)
	}

	taskID, err := run("task", "start", "--project", "project", "--type", "build", "--title", "Milestone two")
	if err != nil {
		t.Fatalf("start task: %v", err)
	}
	taskID = string(bytes.TrimSpace([]byte(taskID)))
	if _, err := run("task", "finish", taskID, "--result", "success"); err != nil {
		t.Fatalf("finish task: %v", err)
	}
	if output, err := run("task", "list", "--project", "project", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("list tasks = %q, %v", output, err)
	}

	fixture := filepath.Join(t.TempDir(), "calls.ndjson")
	event := `{"source":"fixture","session_id":"session","event_type":"model.call","project_id":"` + project.ID + `","occurred_at":"2026-07-16T12:00:00Z","payload":{"provider":"example","model":"model","input_tokens":1000000}}` + "\n"
	if err := os.WriteFile(fixture, []byte(event), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := run("ingest", "file", fixture); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	rule := filepath.Join(t.TempDir(), "pricing.json")
	if err := os.WriteFile(rule, []byte(`{"schemaVersion":1,"provider":"example","modelPattern":"model","validFrom":"2026-07-01T00:00:00Z","billingMode":"token","currency":"USD","unitTokens":1000000,"prices":{"inputMicros":3000000,"cachedInputMicros":0,"cacheWriteMicros":0,"outputMicros":0,"reasoningMicros":0},"version":"test"}`), 0o600); err != nil {
		t.Fatalf("write pricing rule: %v", err)
	}
	if _, err := run("pricing", "add", rule); err != nil {
		t.Fatalf("add pricing: %v", err)
	}
	if output, err := run("pricing", "list", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("list pricing = %q, %v", output, err)
	}
	if _, err := run("pricing", "show", "example/model"); err != nil {
		t.Fatalf("show pricing: %v", err)
	}
	if _, err := run("pricing", "recalculate", "--from", "2026-07-01", "--to", "2026-08-01"); err != nil {
		t.Fatalf("recalculate pricing: %v", err)
	}
	if output, err := run("report", "summary", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("report summary = %q, %v", output, err)
	}
	exported, err := run("export", "--format", "json")
	if err != nil {
		t.Fatalf("export json: %v", err)
	}
	var calls []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(exported), &calls); err != nil || len(calls) != 1 {
		t.Fatalf("decode export = %#v, %v", calls, err)
	}
	if _, err := run("allocation", "show", calls[0].ID); err != nil {
		t.Fatalf("show allocation: %v", err)
	}
	if _, err := run("allocation", "repair", calls[0].ID, "--project", "project"); err != nil {
		t.Fatalf("repair allocation: %v", err)
	}
	if output, err := run("export", "--format", "csv", "--redact-paths"); err != nil || !bytes.Contains([]byte(output), []byte("allocation_project_slug")) {
		t.Fatalf("export csv = %q, %v", output, err)
	}
}
