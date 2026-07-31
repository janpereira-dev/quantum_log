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
