package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
)

func TestUninstallCommandRemovesAllQlogOwnedSetup(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &recordingUninstallCollectorManager{}
	restoreUninstallManager(t, manager)

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

func TestUninstallDryRunDoesNotTearDownCollector(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &recordingUninstallCollectorManager{}
	restoreUninstallManager(t, manager)

	command := New(Version{})
	jsonOutput := new(bytes.Buffer)
	command.SetArgs([]string{"uninstall", "--dry-run", "--json"})
	setOutput(command, jsonOutput)
	if err := command.Execute(); err != nil {
		t.Fatalf("dry-run uninstall: %v", err)
	}
	if manager.uninstallCalls != 0 {
		t.Fatalf("dry-run tore down the collector %d times", manager.uninstallCalls)
	}
	var result uninstallResult
	if err := json.Unmarshal(jsonOutput.Bytes(), &result); err != nil {
		t.Fatalf("decode dry-run JSON: %v", err)
	}
	if result.Collector.Message != "dry run: collector uninstall skipped" {
		t.Fatalf("dry-run collector result = %#v", result.Collector)
	}
}

func TestUninstallPurgeDataFailsClosedAndRetainsLedger(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "ledger")
	initializeUninstallLedger(t, home)
	manager := &recordingUninstallCollectorManager{}
	restoreUninstallManager(t, manager)

	command := New(Version{})
	output := new(bytes.Buffer)
	command.SetArgs([]string{"--home", home, "uninstall", "--purge-data", "--json"})
	setOutput(command, output)
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "automatic local-data purge is temporarily unavailable") {
		t.Fatalf("purge error = %v", err)
	}
	if manager.uninstallCalls != 1 {
		t.Fatalf("collector cleanup calls = %d, want 1", manager.uninstallCalls)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("purge deleted the ledger: %v", err)
	}
	var result uninstallResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("uninstall JSON = %q: %v", output.String(), err)
	}
	if result.DataPurged {
		t.Fatalf("uninstall claimed deleted data: %#v", result)
	}
	if result.Collector.Message != "collector uninstalled" {
		t.Fatalf("collector cleanup was not reported: %#v", result.Collector)
	}
}

func TestUninstallPurgeDataDryRunRetainsLedgerAndCollector(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "ledger")
	initializeUninstallLedger(t, home)
	manager := &recordingUninstallCollectorManager{}
	restoreUninstallManager(t, manager)

	command := New(Version{})
	command.SetArgs([]string{"--home", home, "uninstall", "--purge-data", "--dry-run"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "temporarily unavailable") {
		t.Fatalf("dry-run purge error = %v", err)
	}
	if manager.uninstallCalls != 0 {
		t.Fatalf("dry-run tore down collector %d times", manager.uninstallCalls)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("dry-run purge deleted the ledger: %v", err)
	}
}

func initializeUninstallLedger(t *testing.T, home string) {
	t.Helper()
	service, err := app.Initialize(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func restoreUninstallManager(t *testing.T, manager collectorManager) {
	t.Helper()
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })
}

type recordingUninstallCollectorManager struct {
	status         CollectorStatus
	uninstallCalls int
}

func (m *recordingUninstallCollectorManager) ResolveManagedCollectorSettings(home, listen string, _ bool, _ bool) (string, string) {
	return home, listen
}
func (m *recordingUninstallCollectorManager) Install(_, _ string) (CollectorStatus, error) {
	return CollectorStatus{}, nil
}
func (m *recordingUninstallCollectorManager) Start(_, _ string) (CollectorStatus, error) {
	return CollectorStatus{}, nil
}
func (m *recordingUninstallCollectorManager) Stop() (CollectorStatus, error) {
	return CollectorStatus{}, nil
}
func (m *recordingUninstallCollectorManager) Restart(_, _ string) (CollectorStatus, error) {
	return CollectorStatus{}, nil
}
func (m *recordingUninstallCollectorManager) Status(context.Context, string) (CollectorStatus, error) {
	return m.status, nil
}
func (m *recordingUninstallCollectorManager) Logs() (string, error) { return "", nil }
func (m *recordingUninstallCollectorManager) Uninstall() (CollectorStatus, error) {
	m.uninstallCalls++
	return CollectorStatus{Message: "collector uninstalled"}, nil
}
