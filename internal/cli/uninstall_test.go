package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestUninstallCommandRemovesAllQlogOwnedSetup(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return &fakeCollectorManager{} }
	t.Cleanup(func() { newUninstallCollectorManager = original })

	command := New(Version{})
	output := new(bytes.Buffer)
	command.SetArgs([]string{"uninstall", "--json"})
	setOutput(command, output)
	if err := command.Execute(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	var result uninstallResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("uninstall JSON = %q: %v", output.String(), err)
	}
	if result.Collector.Message != "collector uninstalled" {
		t.Fatalf("collector result = %#v", result.Collector)
	}
	for _, id := range []string{"claude-code", "codex", "copilot", "copilot-vscode", "opencode"} {
		if _, found := result.Adapters[id]; !found {
			t.Fatalf("uninstall result lacks adapter %q: %#v", id, result.Adapters)
		}
	}
}
