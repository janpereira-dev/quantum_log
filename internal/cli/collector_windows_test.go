package cli

import (
	"strings"
	"testing"
)

func TestWindowsCollectorServiceDefinition(t *testing.T) {
	definition := windowsCollectorTaskDefinition(`C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\Users\alice\AppData\Local\QUANTUM_LOG`, "127.0.0.1:4318")
	for _, want := range []string{
		"<LogonTrigger>",
		`C:\Program Files\QUANTUM_LOG\qlog.exe`,
		"collector serve --listen 127.0.0.1:4318",
		`C:\Users\alice\AppData\Local\QUANTUM_LOG`,
	} {
		if !strings.Contains(definition, want) {
			t.Fatalf("task definition missing %q: %s", want, definition)
		}
	}
}
