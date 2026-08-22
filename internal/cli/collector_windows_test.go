//go:build windows

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestWindowsCollectorServiceDefinition(t *testing.T) {
	definition := windowsCollectorTaskDefinition(`C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\Users\alice\AppData\Local\QUANTUM_LOG`, "127.0.0.1:4318", `CONTOSO\alice`, `C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`)
	for _, want := range []string{
		"<LogonTrigger>",
		`C:\Program Files\QUANTUM_LOG\qlog.exe`,
		"collector serve --listen 127.0.0.1:4318",
		`C:\Users\alice\AppData\Local\QUANTUM_LOG`,
		`--log-file &#34;C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log&#34;`,
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("task definition missing %q: %s", want, definition)
		}
	}
}

func TestWindowsCollectorTaskDefinitionUsesInteractiveCurrentUser(t *testing.T) {
	definition := windowsCollectorTaskDefinition(
		`C:\Program Files\QUANTUM_LOG\qlog.exe`,
		`C:\Users\alice\AppData\Local\QUANTUM_LOG`,
		"127.0.0.1:4318",
		`CONTOSO\alice`,
		`C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`,
	)

	for _, want := range []string{
		`<UserId>CONTOSO\alice</UserId>`,
		"<LogonType>InteractiveToken</LogonType>",
		"<RunLevel>LeastPrivilege</RunLevel>",
	} {
		if !strings.Contains(definition, want) {
			t.Errorf("task definition missing %q: %s", want, definition)
		}
	}
}

func TestWindowsCollectorTaskDefinitionBoundsRestartOnFailure(t *testing.T) {
	definition := windowsCollectorTaskDefinition(
		`C:\Program Files\QUANTUM_LOG\qlog.exe`,
		`C:\Users\alice\AppData\Local\QUANTUM_LOG`,
		"127.0.0.1:4318",
		`CONTOSO\alice`,
		`C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`,
	)
	for _, want := range []string{
		"<RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure>",
		"<LogonType>InteractiveToken</LogonType>",
		"<RunLevel>LeastPrivilege</RunLevel>",
		"collector serve --listen 127.0.0.1:4318",
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("task definition missing %q: %s", want, definition)
		}
	}
}

func TestReadWindowsCollectorTaskSettings(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", stateDir)
	home := `C:\Users\alice\AppData\Local\QUANTUM_LOG`
	listen := "127.0.0.1:14318"
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsCollectorTaskDefinition(collectorTaskDefinitionPath(), `C:\Program Files\QUANTUM_LOG\qlog.exe`, home, listen, `CONTOSO\alice`, `C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`); err != nil {
		t.Fatal(err)
	}
	gotHome, gotListen, err := readWindowsCollectorTaskSettings()
	if err != nil {
		t.Fatal(err)
	}
	if gotHome != home || gotListen != listen {
		t.Fatalf("settings = (%q, %q), want (%q, %q)", gotHome, gotListen, home, listen)
	}
}

func TestWindowsCollectorResolveSettingsPrefersActiveScheduledTask(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsCollectorTaskDefinition(collectorTaskDefinitionPath(), `C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\active-ledger`, "127.0.0.1:14318", `CONTOSO\alice`, `C:\collector.log`); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsCollectorFallbackState(windowsCollectorFallbackState{Home: `C:\stale-ledger`, Listen: "127.0.0.1:4318"}); err != nil {
		t.Fatal(err)
	}
	originalStatus := windowsCollectorStatusFn
	t.Cleanup(func() { windowsCollectorStatusFn = originalStatus })
	windowsCollectorStatusFn = func(context.Context, string) (CollectorStatus, error) {
		return CollectorStatus{Installed: true, Mode: windowsCollectorSchedulerMode}, nil
	}

	home, listen := (windowsCollectorManager{}).ResolveManagedCollectorSettings(`C:\default-ledger`, "127.0.0.1:4318", false, false)
	if home != `C:\active-ledger` || listen != "127.0.0.1:14318" {
		t.Fatalf("settings = (%q, %q), want active task settings", home, listen)
	}
}

func TestWindowsCollectorStatusUsesPersistedTaskWhenSchedulerQueryFails(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsCollectorTaskDefinition(collectorTaskDefinitionPath(), `C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\active-ledger`, "127.0.0.1:14318", `CONTOSO\alice`, `C:\collector.log`); err != nil {
		t.Fatal(err)
	}
	originalQuery := queryWindowsCollectorTask
	originalExists := windowsCollectorTaskExists
	t.Cleanup(func() {
		queryWindowsCollectorTask = originalQuery
		windowsCollectorTaskExists = originalExists
	})
	queryWindowsCollectorTask = func(context.Context) ([]byte, error) { return nil, errors.New("access denied") }
	windowsCollectorTaskExists = func() (bool, error) { return true, nil }

	status, err := windowsCollectorStatus(context.Background(), "127.0.0.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.Mode != windowsCollectorSchedulerMode || status.Listen != "127.0.0.1:14318" {
		t.Fatalf("status = %#v, want persisted scheduled task", status)
	}
}

func TestWindowsCollectorStatusIgnoresPersistedTaskWhenSchedulerTaskIsMissing(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsCollectorTaskDefinition(collectorTaskDefinitionPath(), `C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\stale-ledger`, "127.0.0.1:14318", `CONTOSO\alice`, `C:\collector.log`); err != nil {
		t.Fatal(err)
	}
	originalQuery := queryWindowsCollectorTask
	originalExists := windowsCollectorTaskExists
	t.Cleanup(func() {
		queryWindowsCollectorTask = originalQuery
		windowsCollectorTaskExists = originalExists
	})
	queryWindowsCollectorTask = func(context.Context) ([]byte, error) {
		return []byte("ERROR: No se puede encontrar el archivo especificado.\r\n"), errors.New("task not found")
	}
	windowsCollectorTaskExists = func() (bool, error) { return false, nil }

	status, err := windowsCollectorStatus(context.Background(), "127.0.0.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	if status.Installed || status.Mode != windowsCollectorNoMode || status.Listen != "127.0.0.1:4318" {
		t.Fatalf("status = %#v, want missing task", status)
	}
}

func TestWindowsCollectorTaskTargetMatchesRequiresSameTaskTarget(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeWindowsCollectorTaskDefinition(collectorTaskDefinitionPath(), `C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\active-ledger`, "127.0.0.1:4318", `CONTOSO\alice`, `C:\collector.log`); err != nil {
		t.Fatal(err)
	}
	originalStatus := windowsCollectorStatusFn
	t.Cleanup(func() { windowsCollectorStatusFn = originalStatus })
	windowsCollectorStatusFn = func(context.Context, string) (CollectorStatus, error) {
		return CollectorStatus{Installed: true, Mode: windowsCollectorSchedulerMode}, nil
	}

	executable := `C:\Program Files\QUANTUM_LOG\qlog.exe`
	if !windowsCollectorTaskTargetMatches(`C:\active-ledger`, "127.0.0.1:4318", executable) {
		t.Fatal("matching scheduled task was rejected")
	}
	if windowsCollectorTaskTargetMatches(`C:\different-ledger`, "127.0.0.1:4318", executable) {
		t.Fatal("different scheduled task target was accepted")
	}
	if windowsCollectorTaskTargetMatches(`C:\active-ledger`, "127.0.0.1:4318", `C:\Program Files\QUANTUM_LOG\qlog-next.exe`) {
		t.Fatal("different collector executable was accepted")
	}
}

func TestWriteWindowsCollectorTaskDefinitionUsesUTF16LE(t *testing.T) {
	executable := `C:\Program Files\QUANTUM_LOG\qlog.exe`
	home := `C:\Users\alice\AppData\Local\QUANTUM_LOG`
	listen := "127.0.0.1:4318"
	path := t.TempDir() + `\collector-task.xml`

	logPath := `C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`
	if err := writeWindowsCollectorTaskDefinition(path, executable, home, listen, `CONTOSO\alice`, logPath); err != nil {
		t.Fatalf("writeWindowsCollectorTaskDefinition() error = %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.HasPrefix(contents, []byte{0xFF, 0xFE}) {
		t.Fatalf("task file prefix = %x, want UTF-16LE BOM ff fe", contents[:min(len(contents), 2)])
	}
	if len(contents[2:])%2 != 0 {
		t.Fatalf("UTF-16LE content has odd byte length: %d", len(contents[2:]))
	}

	codeUnits := make([]uint16, 0, len(contents[2:])/2)
	for i := 2; i < len(contents); i += 2 {
		codeUnits = append(codeUnits, uint16(contents[i])|uint16(contents[i+1])<<8)
	}
	definition := string(utf16.Decode(codeUnits))
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-16"?>`,
		executable,
		home,
		"collector serve --listen " + listen,
		"--log-file &#34;" + logPath + "&#34;",
	} {
		if !strings.Contains(definition, want) {
			t.Errorf("UTF-16LE task definition missing %q: %s", want, definition)
		}
	}
}

func TestValidateWindowsCollectorExecutable(t *testing.T) {
	tests := []struct {
		name       string
		executable string
		wantErr    bool
	}{
		{
			name:       "rejects Go test binary",
			executable: `C:\Users\alice\AppData\Local\Temp\go-build1234\b001\cli.test.exe`,
			wantErr:    true,
		},
		{
			name:       "rejects Go build cache path",
			executable: `C:\Users\alice\AppData\Local\Temp\go-build1234\b001\exe\qlog.exe`,
			wantErr:    true,
		},
		{
			name:       "accepts installed qlog binary",
			executable: `C:\Program Files\QUANTUM_LOG\qlog.exe`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCollectorExecutable(tt.executable)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCollectorExecutable(%q) error = %v, wantErr %t", tt.executable, err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "build or install a durable qlog.exe") {
				t.Fatalf("error = %q, want durable qlog.exe guidance", err)
			}
		})
	}
}

func TestCreateWindowsCollectorTaskReportsSchedulerDiagnostic(t *testing.T) {
	original := runWindowsSchedulerCommand
	t.Cleanup(func() { runWindowsSchedulerCommand = original })
	runWindowsSchedulerCommand = func(args ...string) ([]byte, error) {
		if !slices.Equal(args, []string{"/Create", "/TN", windowsCollectorTaskName, "/XML", `C:\collector-task.xml`, "/F"}) {
			t.Fatalf("Scheduler command arguments = %q", args)
		}
		return []byte("ERROR: The task XML contains a value which is incorrectly formatted or out of range."), errors.New("exit status 1")
	}

	err := createWindowsCollectorTask(`C:\collector-task.xml`)
	if err == nil {
		t.Fatal("createWindowsCollectorTask() error = nil")
	}
	for _, want := range []string{
		"task scheduler operation /Create",
		windowsCollectorTaskName,
		"exit status 1",
		"ERROR: The task XML contains a value which is incorrectly formatted or out of range.",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
}

func TestWindowsCollectorFallbackRunCommandUsesDurableIdentity(t *testing.T) {
	state := windowsCollectorFallbackState{
		Mode:       windowsCollectorFallbackMode,
		Executable: `C:\Program Files\QUANTUM_LOG\qlog.exe`,
		Home:       `C:\Users\alice\AppData\Local\QUANTUM_LOG`,
		Listen:     "127.0.0.1:4318",
		LogPath:    `C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`,
	}
	command := windowsCollectorRunCommand(state)
	for _, want := range []string{state.Executable, "--home", state.Home, "collector serve", "--listen 127.0.0.1:4318", "--log-file", state.LogPath, "--fallback-state"} {
		if !strings.Contains(command, want) {
			t.Fatalf("fallback command missing %q: %s", want, command)
		}
	}
}

func TestWindowsCollectorStartRejectsLegacyFallback(t *testing.T) {
	originalStatus := windowsCollectorStatusFn
	t.Cleanup(func() { windowsCollectorStatusFn = originalStatus })
	windowsCollectorStatusFn = func(context.Context, string) (CollectorStatus, error) {
		return CollectorStatus{Installed: true, Mode: windowsCollectorFallbackMode, Listen: "127.0.0.1:4318"}, nil
	}

	_, err := (windowsCollectorManager{}).Start(`C:\ledger`, "127.0.0.1:4318")
	if err == nil || !strings.Contains(err.Error(), "legacy Windows Run-key fallback") {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestWindowsCollectorUninstallStopsAndUnregistersFallbackWhenSchedulerWins(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	state := windowsCollectorFallbackState{
		Mode:       windowsCollectorFallbackMode,
		Executable: `C:\Program Files\QUANTUM_LOG\qlog.exe`,
		Home:       `C:\Users\alice\AppData\Local\QUANTUM_LOG`,
		Listen:     "127.0.0.1:4318",
		LogPath:    filepath.Join(collectorStateDir(), "collector.log"),
	}
	state.Command = windowsCollectorRunCommand(state)
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	if err := writeWindowsCollectorFallbackState(state); err != nil {
		t.Fatalf("write fallback state: %v", err)
	}
	originalStatus := windowsCollectorStatusFn
	originalScheduler := runWindowsSchedulerCommand
	originalStop := stopWindowsCollectorFallbackFn
	originalUnregister := unregisterWindowsCollectorFallbackFn
	t.Cleanup(func() {
		windowsCollectorStatusFn = originalStatus
		runWindowsSchedulerCommand = originalScheduler
		stopWindowsCollectorFallbackFn = originalStop
		unregisterWindowsCollectorFallbackFn = originalUnregister
	})
	windowsCollectorStatusFn = func(context.Context, string) (CollectorStatus, error) {
		return CollectorStatus{Installed: true, Mode: windowsCollectorSchedulerMode}, nil
	}
	var schedulerCalls []string
	runWindowsSchedulerCommand = func(args ...string) ([]byte, error) {
		schedulerCalls = append(schedulerCalls, strings.Join(args, " "))
		return nil, nil
	}
	stopped := false
	stopWindowsCollectorFallbackFn = func(got windowsCollectorFallbackState) error {
		stopped = got.Command == state.Command
		return nil
	}
	unregistered := false
	unregisterWindowsCollectorFallbackFn = func() error {
		unregistered = true
		return nil
	}

	if _, err := (windowsCollectorManager{}).Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if !stopped || !unregistered {
		t.Fatalf("fallback cleanup stopped=%t unregistered=%t", stopped, unregistered)
	}
	if !slices.Contains(schedulerCalls, "/End /TN "+windowsCollectorTaskName) || !slices.Contains(schedulerCalls, "/Delete /TN "+windowsCollectorTaskName+" /F") {
		t.Fatalf("scheduler calls = %q", schedulerCalls)
	}
}
