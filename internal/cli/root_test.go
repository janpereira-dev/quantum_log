package cli

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/app"
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

func TestDoctorIsReadOnly(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "uninitialized")
	before := snapshotTree(t, home)
	if _, err := runQLog(t, home, "doctor", "--json"); err == nil || !strings.Contains(err.Error(), "qlog init") {
		t.Fatalf("qlog doctor uninitialized error = %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))

	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	before = snapshotTree(t, home)
	if _, err := runQLog(t, home, "doctor", "--json"); err != nil {
		t.Fatalf("qlog doctor: %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))

	makeSchemaPending(t, filepath.Join(home, "qlog.db"))
	before = snapshotTree(t, home)
	if _, err := runQLog(t, home, "doctor", "--json"); err == nil || !strings.Contains(err.Error(), "pending migration") || !strings.Contains(err.Error(), "qlog init") {
		t.Fatalf("qlog doctor pending schema error = %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))
}

func TestVerifyIsReadOnly(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "uninitialized")
	before := snapshotTree(t, home)
	if _, err := runQLog(t, home, "verify"); err == nil || !strings.Contains(err.Error(), "qlog init") {
		t.Fatalf("qlog verify uninitialized error = %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))

	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	before = snapshotTree(t, home)
	if _, err := runQLog(t, home, "verify"); err != nil {
		t.Fatalf("qlog verify: %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))

	makeSchemaPending(t, filepath.Join(home, "qlog.db"))
	before = snapshotTree(t, home)
	if _, err := runQLog(t, home, "verify"); err == nil || !strings.Contains(err.Error(), "pending migration") || !strings.Contains(err.Error(), "qlog init") {
		t.Fatalf("qlog verify pending schema error = %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))
}

func TestDiagnosticsRejectActiveWriter(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	service, err := app.Open(t.Context(), home)
	if err != nil {
		t.Fatalf("open active writer: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })
	if _, _, err := service.Store.RegisterProject(t.Context(), "Writer", "writer", t.TempDir()); err != nil {
		t.Fatalf("write active WAL: %v", err)
	}
	if info, err := os.Stat(service.Paths.Database + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("active WAL missing or empty: info=%#v err=%v", info, err)
	}

	for _, args := range [][]string{{"doctor", "--json"}, {"verify"}} {
		if _, err := runQLog(t, home, args...); err == nil || !strings.Contains(err.Error(), "quiescence lock is held") {
			t.Fatalf("qlog %s active writer error = %v", strings.Join(args, " "), err)
		}
	}
}

func TestDiagnosticsBlockWhileReadOnlyClientHoldsQuiescence(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	service, err := app.OpenReadOnly(t.Context(), home)
	if err != nil {
		t.Fatalf("open read-only client: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })

	for _, args := range [][]string{{"doctor", "--json"}, {"verify"}} {
		if _, err := runQLog(t, home, args...); err == nil || !strings.Contains(err.Error(), "quiescence lock is held") {
			t.Fatalf("qlog %s active reader error = %v", strings.Join(args, " "), err)
		}
	}
}

func TestDoctorBlocksPendingWALWithoutMutation(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	databasePath := filepath.Join(home, "qlog.db")
	if err := os.WriteFile(databasePath+"-wal", []byte("pending WAL"), 0o600); err != nil {
		t.Fatalf("write pending WAL: %v", err)
	}
	before := snapshotTree(t, home)
	if _, err := runQLog(t, home, "doctor", "--json"); err == nil || !strings.Contains(err.Error(), "active WAL") {
		t.Fatalf("qlog doctor pending WAL error = %v", err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))
}

func TestDoctorWarnsForIsolatedSHMWithoutMutation(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "qlog.db-shm"), []byte("stale SHM"), 0o600); err != nil {
		t.Fatalf("write isolated SHM: %v", err)
	}
	before := snapshotTree(t, home)
	output, err := runQLog(t, home, "doctor", "--json")
	if err != nil {
		t.Fatalf("qlog doctor isolated SHM: %v", err)
	}
	var result struct {
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil || !strings.Contains(result.Warning, "isolated SQLite SHM") {
		t.Fatalf("qlog doctor isolated SHM output = %q, %v", output, err)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))
}

func TestVerifyWarnsForIsolatedSHMWithoutMutation(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "qlog.db-shm"), []byte("stale SHM"), 0o600); err != nil {
		t.Fatalf("write isolated SHM: %v", err)
	}
	before := snapshotTree(t, home)
	output, err := runQLog(t, home, "verify")
	if err != nil {
		t.Fatalf("qlog verify isolated SHM: %v", err)
	}
	if !strings.Contains(output, "isolated SQLite SHM") || !strings.Contains(output, "ledger: verified") {
		t.Fatalf("qlog verify isolated SHM output = %q", output)
	}
	assertTreeEqual(t, before, snapshotTree(t, home))
}

func TestMaintenanceCommandSurface(t *testing.T) {
	home := t.TempDir()
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("qlog init: %v", err)
	}
	if _, err := runQLog(t, home, "maintenance", "status"); err != nil {
		t.Fatalf("qlog maintenance status: %v", err)
	}
	if _, err := runQLog(t, home, "maintenance", "checkpoint"); err != nil {
		t.Fatalf("qlog maintenance checkpoint: %v", err)
	}
	for _, operation := range []string{"recover", "rebuild-anchor"} {
		if _, err := runQLog(t, home, "maintenance", operation); err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Fatalf("qlog maintenance %s error = %v", operation, err)
		}
	}
}

func TestSnapshotTreeRecordsModificationTimes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chtimes(path, time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("set initial fixture time: %v", err)
	}
	before := snapshotTree(t, root)
	if err := os.Chtimes(path, time.Date(2020, time.January, 1, 0, 0, 1, 0, time.UTC), time.Date(2020, time.January, 1, 0, 0, 1, 0, time.UTC)); err != nil {
		t.Fatalf("set changed fixture time: %v", err)
	}
	after := snapshotTree(t, root)
	if before["fixture"] == after["fixture"] {
		t.Fatal("snapshot did not record fixture modification time")
	}
}

type treeSnapshot map[string]string

func runQLog(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	command := New(Version{Version: "0.1.0", Commit: "test", BuildDate: "2026-07-16"})
	output := new(bytes.Buffer)
	command.SetArgs(append([]string{"--home", home}, args...))
	setOutput(command, output)
	err := command.Execute()
	return output.String(), err
}

func runQLogWithInput(t *testing.T, home string, input io.Reader, args ...string) (string, error) {
	t.Helper()
	command := New(Version{Version: "0.1.0", Commit: "test", BuildDate: "2026-07-16"})
	output := new(bytes.Buffer)
	command.SetArgs(append([]string{"--home", home}, args...))
	command.SetIn(input)
	setOutput(command, output)
	err := command.Execute()
	return output.String(), err
}

func snapshotTree(t *testing.T, root string) treeSnapshot {
	t.Helper()
	snapshot := treeSnapshot{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[relativePath] = "directory:" + info.Mode().String() + ":" + info.ModTime().UTC().Format(time.RFC3339Nano)
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(contents)
		snapshot[relativePath] = "file:" + info.Mode().String() + ":" + hex.EncodeToString(hash[:]) + ":" + info.ModTime().UTC().Format(time.RFC3339Nano)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

func assertTreeEqual(t *testing.T, want, got treeSnapshot) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("filesystem snapshot count = %d, want %d; got=%#v want=%#v", len(got), len(want), got, want)
	}
	for path, wantEntry := range want {
		if gotEntry, found := got[path]; !found || gotEntry != wantEntry {
			t.Fatalf("filesystem snapshot %q = %q, want %q", path, gotEntry, wantEntry)
		}
	}
}

func makeSchemaPending(t *testing.T, databasePath string) {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	result, err := database.Exec(`DELETE FROM schema_migrations WHERE version = (SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1)`)
	if err != nil {
		t.Fatalf("remove migration: %v", err)
	}
	if deleted, err := result.RowsAffected(); err != nil || deleted != 1 {
		t.Fatalf("remove migration rows = %d, %v", deleted, err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close pending database: %v", err)
	}
}

func TestProjectCurrentSelectsLocationMatchingWorkingDirectory(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home symlinks: %v", err)
	}
	firstLocation, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve first location symlinks: %v", err)
	}
	matchingLocation, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve matching location symlinks: %v", err)
	}
	t.Setenv("QLOG_PROJECT", "")
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

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
	if _, err := run("project", "register", "--path", firstLocation, "--name", "Project"); err != nil {
		t.Fatalf("register first location: %v", err)
	}
	if _, err := run("project", "register", "--path", matchingLocation, "--name", "Project"); err != nil {
		t.Fatalf("register matching location: %v", err)
	}
	service, err := app.Open(t.Context(), home)
	if err != nil {
		t.Fatalf("open project store: %v", err)
	}
	_, expectedLocation, found, err := service.Store.ProjectByLocation(t.Context(), matchingLocation)
	if closeErr := service.Close(); closeErr != nil {
		t.Fatalf("close project store: %v", closeErr)
	}
	if err != nil || !found {
		t.Fatalf("find matching location: found=%t err=%v", found, err)
	}
	if err := os.Chdir(matchingLocation); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWorkingDirectory) })

	output, err := run("project", "current", "--json")
	if err != nil {
		t.Fatalf("qlog project current: %v", err)
	}
	var current struct {
		ProjectSlug  string `json:"project_slug"`
		LocationID   string `json:"project_location_id"`
		LocationPath string `json:"location_path"`
		Method       string `json:"method"`
		Confidence   string `json:"confidence"`
		Evidence     string `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(output), &current); err != nil {
		t.Fatalf("project current JSON: %v; output=%q", err, output)
	}
	if current.ProjectSlug != "project" || current.LocationID != expectedLocation.ID || current.LocationPath != matchingLocation || current.Method != "cwd" || current.Confidence != "high" || current.Evidence != strings.ToLower(filepath.ToSlash(matchingLocation)) {
		t.Fatalf("project current = %#v", current)
	}
}

func TestProjectRegisterIsIdempotentForCurrentDirectory(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home symlinks: %v", err)
	}
	projectDirectory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project directory symlinks: %v", err)
	}
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDirectory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWorkingDirectory) })

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
	for range 2 {
		if _, err := run("project", "register", "--path", ".", "--name", "Project"); err != nil {
			t.Fatalf("qlog project register: %v", err)
		}
	}
	output, err := run("project", "list", "--json")
	if err != nil {
		t.Fatalf("qlog project list: %v", err)
	}
	var projects []struct {
		Slug          string `json:"slug"`
		LocationCount int64  `json:"location_count"`
	}
	if err := json.Unmarshal([]byte(output), &projects); err != nil {
		t.Fatalf("project list JSON: %v; output=%q", err, output)
	}
	if len(projects) != 1 || projects[0].Slug != "project" || projects[0].LocationCount != 1 {
		t.Fatalf("project list = %#v, want one project location", projects)
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

func TestMilestoneSixAgentCommands(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "project")
	run := func(args ...string) (string, error) {
		command := New(Version{Version: "0.1.0"})
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
		t.Fatalf("register: %v", err)
	}
	taskID, err := run("task", "start", "--project", "project", "--title", "Agent task")
	if err != nil {
		t.Fatalf("task start: %v", err)
	}
	taskID = string(bytes.TrimSpace([]byte(taskID)))
	if _, err := run("task", "finish", taskID, "--result", "complete"); err != nil {
		t.Fatalf("task finish: %v", err)
	}
	if output, err := run("task", "summary", taskID, "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("task summary = %q, %v", output, err)
	}
	if _, err := run("budget", "set-project", "project", "--monthly-usd-micros", "1000"); err != nil {
		t.Fatalf("set project budget: %v", err)
	}
	if output, err := run("budget", "status", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("budget status = %q, %v", output, err)
	}
	if output, err := run("unattributed", "list", "--json"); err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("unattributed list = %q, %v", output, err)
	}
	command := New(Version{})
	if found, _, err := command.Find([]string{"mcp", "serve"}); err != nil || found == nil || found.Name() != "serve" {
		t.Fatalf("mcp serve command = %#v, %v", found, err)
	}
}

func TestUsageProjectReportsAgentAndCaptureQuality(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "project")
	run := func(args ...string) (string, error) {
		command := New(Version{Version: "0.3.0"})
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
	fixture := filepath.Join(t.TempDir(), "calls.ndjson")
	event := `{"source":"fixture","session_id":"session","event_type":"model.call","project_id":"` + project.ID + `","occurred_at":"2026-07-20T12:00:00Z","payload":{"provider":"anthropic","model":"claude-sonnet","agent_name":"opencode","input_tokens":10,"output_tokens":20,"capture_quality":"agent_reported"}}` + "\n"
	if err := os.WriteFile(fixture, []byte(event), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := run("ingest", "file", fixture); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	output, err := run("usage", "project", "project")
	if err != nil {
		t.Fatalf("usage project: %v", err)
	}
	if !strings.Contains(output, "project | opencode | anthropic/claude-sonnet | agent_reported | 30 tokens") {
		t.Fatalf("usage project output = %q", output)
	}
}

func TestCollectorHandlerDoesNotHoldWriterLockBetweenRequests(t *testing.T) {
	home := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "project")
	if _, err := runQLog(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runQLog(t, home, "project", "register", "--path", worktree, "--name", "Project", "--slug", "project"); err != nil {
		t.Fatalf("register project: %v", err)
	}

	handler := newCollectorMux(home)
	request := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"copilot-chat"}}]},"scopeSpans":[{"spans":[{"traceId":"trace-live","attributes":[{"key":"qlog.project","value":{"stringValue":"project"}},{"key":"gen_ai.provider.name","value":{"stringValue":"github"}},{"key":"gen_ai.request.model","value":{"stringValue":"gpt-5"}},{"key":"gen_ai.usage.input_tokens","value":{"intValue":"1"}},{"key":"gen_ai.usage.output_tokens","value":{"intValue":"2"}}]}]}]}]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("collector response = %d: %s", response.Code, response.Body.String())
	}

	if output, err := runQLog(t, home, "adapter", "verify", "copilot-vscode", "--json"); err != nil {
		t.Fatalf("adapter verify should read after collector request: output=%q err=%v", output, err)
	}
	if output, err := runQLog(t, home, "usage", "project", "project", "--json"); err != nil {
		t.Fatalf("usage should read after collector request: output=%q err=%v", output, err)
	}
}
