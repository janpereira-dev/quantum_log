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
		"/home/alice/bin/qlog",
		"collector serve --listen 127.0.0.1:4318",
		"--home /home/alice/.qlog",
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("unit definition missing %q: %s", want, definition)
		}
	}
}
