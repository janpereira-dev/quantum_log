//go:build linux

package cli

import (
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
