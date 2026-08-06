//go:build linux

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLinuxCollectorServiceDefinition(t *testing.T) {
	definition := linuxCollectorUnitDefinition("/home/alice/bin/qlog", "/home/alice/.qlog", "127.0.0.1:4318")
	for _, want := range []string{
		"[Install]",
		"WantedBy=default.target",
		"Restart=on-failure",
		`ExecStart="/home/alice/bin/qlog" --home "/home/alice/.qlog" collector serve --listen "127.0.0.1:4318"`,
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("unit definition missing %q: %s", want, definition)
		}
	}
}

func TestLinuxCollectorRejectsTransientExecutable(t *testing.T) {
	for _, executable := range []string{
		"/tmp/go-build1234/b001/exe/qlog",
		"/tmp/cli.test",
	} {
		if err := validateCollectorExecutable(executable); err == nil {
			t.Fatalf("validateCollectorExecutable(%q) error = nil", executable)
		}
	}
}

func TestLinuxCollectorStatePersistsListenerAndMigratesLegacyHome(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "collector.state")
	home := filepath.Join(t.TempDir(), "qlog")
	if err := writeLinuxCollectorState(statePath, linuxCollectorState{Home: home, Listen: "127.0.0.1:4319"}); err != nil {
		t.Fatalf("writeLinuxCollectorState() error = %v", err)
	}
	state := readLinuxCollectorState(statePath)
	if state.Home != home || state.Listen != "127.0.0.1:4319" {
		t.Fatalf("state = %#v", state)
	}
	if got := linuxCollectorListen(state, defaultCollectorListen, false); got != "127.0.0.1:4319" {
		t.Fatalf("omitted listener = %q, want installed listener", got)
	}
	if got := linuxCollectorListen(state, defaultCollectorListen, true); got != defaultCollectorListen {
		t.Fatalf("explicit listener = %q, want %q", got, defaultCollectorListen)
	}

	if err := os.WriteFile(statePath, []byte(home), 0o600); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}
	legacy := readLinuxCollectorState(statePath)
	if legacy.Home != home || legacy.Listen != "" {
		t.Fatalf("legacy state = %#v", legacy)
	}
	if err := os.WriteFile(statePath, []byte(`{"home":"relative"}`), 0o600); err != nil {
		t.Fatalf("write relative state: %v", err)
	}
	if state := readLinuxCollectorState(statePath); state != (linuxCollectorState{}) {
		t.Fatalf("relative state = %#v, want empty", state)
	}
}

func TestLinuxCollectorUninstallRemovesUnitBeforeDaemonReload(t *testing.T) {
	var calls []string
	home := t.TempDir()
	resetLinuxCollectorUninstallSeams(t)
	linuxCollectorUnitExists = func(string) bool { return true }
	stopLinuxCollector = func() (CollectorStatus, error) {
		calls = append(calls, "stop")
		return CollectorStatus{}, nil
	}
	runLinuxSystemctl = func(args ...string) error {
		switch args[1] {
		case "disable":
			calls = append(calls, "disable")
		case "daemon-reload":
			calls = append(calls, "daemon-reload")
		}
		return nil
	}
	removeLinuxCollectorUnit = func(string) error {
		calls = append(calls, "remove-unit")
		return nil
	}
	readManagedLinuxCollectorState = func(string) linuxCollectorState { return linuxCollectorState{Home: home} }
	removeLinuxCollectorTree = func(string) error {
		calls = append(calls, "remove-logs")
		return nil
	}
	removeLinuxCollectorState = func(string) error {
		calls = append(calls, "remove-state")
		return nil
	}

	if _, err := (linuxCollectorManager{}).Uninstall(); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop", "disable", "remove-unit", "daemon-reload", "remove-logs", "remove-state"}
	if !slices.Equal(calls, want) {
		t.Fatalf("uninstall calls = %q, want %q", calls, want)
	}
}

func TestLinuxCollectorUninstallKeepsStateWhenReloadFails(t *testing.T) {
	var calls []string
	home := t.TempDir()
	resetLinuxCollectorUninstallSeams(t)
	linuxCollectorUnitExists = func(string) bool { return true }
	stopLinuxCollector = func() (CollectorStatus, error) {
		calls = append(calls, "stop")
		return CollectorStatus{}, nil
	}
	runLinuxSystemctl = func(args ...string) error {
		switch args[1] {
		case "disable":
			calls = append(calls, "disable")
			return nil
		case "daemon-reload":
			calls = append(calls, "daemon-reload")
			return errors.New("reload failed")
		default:
			return nil
		}
	}
	removeLinuxCollectorUnit = func(string) error {
		calls = append(calls, "remove-unit")
		return nil
	}
	readManagedLinuxCollectorState = func(string) linuxCollectorState { return linuxCollectorState{Home: home} }
	removeLinuxCollectorTree = func(string) error {
		calls = append(calls, "remove-logs")
		return nil
	}
	removeLinuxCollectorState = func(string) error {
		calls = append(calls, "remove-state")
		return nil
	}

	if _, err := (linuxCollectorManager{}).Uninstall(); err == nil {
		t.Fatal("Uninstall() error = nil")
	}
	want := []string{"stop", "disable", "remove-unit", "daemon-reload"}
	if !slices.Equal(calls, want) {
		t.Fatalf("uninstall calls = %q, want %q", calls, want)
	}
}

func TestLinuxCollectorUninstallIsIdempotentWithoutUnit(t *testing.T) {
	var calls []string
	resetLinuxCollectorUninstallSeams(t)
	linuxCollectorUnitExists = func(string) bool { return false }
	stopLinuxCollector = func() (CollectorStatus, error) {
		calls = append(calls, "stop")
		return CollectorStatus{}, nil
	}
	runLinuxSystemctl = func(args ...string) error {
		calls = append(calls, strings.Join(args, " "))
		return nil
	}
	removeLinuxCollectorUnit = func(string) error { return os.ErrNotExist }
	removeLinuxCollectorTree = func(string) error { return nil }
	removeLinuxCollectorState = func(string) error { return nil }
	readManagedLinuxCollectorState = func(string) linuxCollectorState { return linuxCollectorState{} }

	if _, err := (linuxCollectorManager{}).Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if !slices.Equal(calls, []string{"stop"}) {
		t.Fatalf("uninstall calls = %q, want only idempotent stop", calls)
	}
}

func TestLinuxCollectorUninstallReloadsWhenStateRemainsAfterUnitDeletion(t *testing.T) {
	var calls []string
	resetLinuxCollectorUninstallSeams(t)
	linuxCollectorUnitExists = func(string) bool { return false }
	stopLinuxCollector = func() (CollectorStatus, error) {
		calls = append(calls, "stop")
		return CollectorStatus{}, nil
	}
	runLinuxSystemctl = func(args ...string) error {
		calls = append(calls, args[1])
		return nil
	}
	removeLinuxCollectorUnit = func(string) error { return os.ErrNotExist }
	readManagedLinuxCollectorState = func(string) linuxCollectorState { return linuxCollectorState{Home: t.TempDir()} }
	removeLinuxCollectorTree = func(string) error {
		calls = append(calls, "remove-logs")
		return nil
	}
	removeLinuxCollectorState = func(string) error {
		calls = append(calls, "remove-state")
		return nil
	}

	if _, err := (linuxCollectorManager{}).Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	want := []string{"stop", "daemon-reload", "remove-logs", "remove-state"}
	if !slices.Equal(calls, want) {
		t.Fatalf("uninstall calls = %q, want %q", calls, want)
	}
}

func resetLinuxCollectorUninstallSeams(t *testing.T) {
	t.Helper()
	previousSystemctl := runLinuxSystemctl
	previousUnitExists := linuxCollectorUnitExists
	previousUnitRemove := removeLinuxCollectorUnit
	previousTreeRemove := removeLinuxCollectorTree
	previousStateRemove := removeLinuxCollectorState
	previousStop := stopLinuxCollector
	previousStateRead := readManagedLinuxCollectorState
	t.Cleanup(func() {
		runLinuxSystemctl = previousSystemctl
		linuxCollectorUnitExists = previousUnitExists
		removeLinuxCollectorUnit = previousUnitRemove
		removeLinuxCollectorTree = previousTreeRemove
		removeLinuxCollectorState = previousStateRemove
		stopLinuxCollector = previousStop
		readManagedLinuxCollectorState = previousStateRead
	})
}
