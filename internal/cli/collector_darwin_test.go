//go:build darwin

package cli

import (
	"strings"
	"testing"
)

func TestDarwinCollectorServiceDefinition(t *testing.T) {
	definition := darwinCollectorLaunchAgentDefinition("/Users/alice/bin/qlog", "/Users/alice/.qlog", "127.0.0.1:4318")
	for _, want := range []string{
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"/Users/alice/bin/qlog",
		"collector",
		"127.0.0.1:4318",
		"/Users/alice/.qlog/collector/collector.log",
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("launch agent definition missing %q: %s", want, definition)
		}
	}
}

func TestDarwinCollectorRejectsTransientExecutable(t *testing.T) {
	for _, executable := range []string{
		"/var/folders/tmp/go-build1234/b001/exe/qlog",
		"/var/folders/tmp/cli.test",
	} {
		if err := validateCollectorExecutable(executable); err == nil {
			t.Fatalf("validateCollectorExecutable(%q) error = nil", executable)
		}
	}
}
