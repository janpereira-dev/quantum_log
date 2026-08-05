//go:build windows

package cli

import (
	"bytes"
	"errors"
	"os"
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
