//go:build linux

package cli

import (
	"os"
	"path/filepath"
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
