package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
	"github.com/janpereira-dev/quantum_log/internal/config"
	"github.com/spf13/cobra"
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

func TestUninstallDryRunDoesNotTearDownCollector(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &recordingUninstallCollectorManager{}
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })

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

	command = New(Version{})
	textOutput := new(bytes.Buffer)
	command.SetArgs([]string{"uninstall", "--dry-run"})
	setOutput(command, textOutput)
	if err := command.Execute(); err != nil {
		t.Fatalf("text dry-run uninstall: %v", err)
	}
	if !strings.Contains(textOutput.String(), "dry run: collector uninstall skipped") {
		t.Fatalf("text dry-run omitted collector plan: %q", textOutput.String())
	}
	if manager.uninstallCalls != 0 {
		t.Fatalf("text dry-run tore down the collector %d times", manager.uninstallCalls)
	}
}

func TestUninstallPurgeDataUsesPersistedManagedHome(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	managedHome := filepath.Join(t.TempDir(), "managed")
	defaultHome := filepath.Join(t.TempDir(), "default")
	t.Setenv("QLOG_HOME", defaultHome)
	initializeUninstallLedger(t, managedHome)
	initializeUninstallLedger(t, defaultHome)
	manager := &recordingUninstallCollectorManager{managedHome: managedHome}
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })

	command := New(Version{})
	command.SetArgs([]string{"uninstall", "--purge-data"})
	if err := command.Execute(); err != nil {
		t.Fatalf("purge managed home: %v", err)
	}
	if _, err := os.Stat(managedHome); !os.IsNotExist(err) {
		t.Fatalf("managed home was retained: %v", err)
	}
	if _, err := os.Stat(defaultHome); err != nil {
		t.Fatalf("default home was deleted instead of managed home: %v", err)
	}
}

func TestUninstallPurgeDataRejectsUnownedOrUnsafeHome(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "not-qlog")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := &recordingUninstallCollectorManager{}
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })

	command := New(Version{})
	command.SetArgs([]string{"--home", home, "uninstall", "--purge-data"})
	if err := command.Execute(); err == nil {
		t.Fatal("purge accepted an unowned directory")
	}
	if manager.uninstallCalls != 1 {
		t.Fatalf("collector cleanup was not attempted once: %d", manager.uninstallCalls)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("unowned directory was deleted: %v", err)
	}
}

func TestUninstallPurgeDataRefusesReachableForegroundCollector(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "ledger")
	initializeUninstallLedger(t, home)
	manager := &recordingUninstallCollectorManager{status: CollectorStatus{Reachable: true, ManagedHealth: true}}
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })

	command := New(Version{})
	command.SetArgs([]string{"--home", home, "uninstall", "--purge-data"})
	if err := command.Execute(); err == nil {
		t.Fatal("purge accepted a reachable unmanaged foreground collector")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("ledger was deleted while foreground collector remained active: %v", err)
	}
}

func TestUninstallPurgeDataRefusesAnActiveLedger(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "ledger")
	writer, err := app.Initialize(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	manager := &recordingUninstallCollectorManager{}
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })

	command := New(Version{})
	command.SetArgs([]string{"--home", home, "uninstall", "--purge-data"})
	if err := command.Execute(); err == nil {
		t.Fatal("purge accepted a ledger with an active qlog writer")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("active ledger was deleted: %v", err)
	}
}

func TestPurgeUninstallDataKeepsLedgerUnreachableUntilDeletionCompletes(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	initializeUninstallLedger(t, home)
	originalRemove := removeUninstallDataDirectory
	removeUninstallDataDirectory = func(path string) error {
		writer, err := app.Open(context.Background(), path)
		if err == nil {
			_ = writer.Close()
			t.Fatal("writer opened a ledger while purge deletion was pending")
		}
		if !strings.Contains(err.Error(), "purge is in progress") {
			t.Fatalf("writer during purge error = %v", err)
		}
		return originalRemove(path)
	}
	t.Cleanup(func() { removeUninstallDataDirectory = originalRemove })

	command := &cobra.Command{}
	command.SetContext(context.Background())
	if err := purgeUninstallData(command, home); err != nil {
		t.Fatalf("purge data: %v", err)
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("purged home remains: %v", err)
	}
}

func TestPurgeUninstallDataRestoresLedgerAccessAfterDeletionFailure(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	initializeUninstallLedger(t, home)
	originalRemove := removeUninstallDataDirectory
	removeUninstallDataDirectory = func(string) error { return errors.New("simulated remove failure") }
	t.Cleanup(func() { removeUninstallDataDirectory = originalRemove })

	command := &cobra.Command{}
	command.SetContext(context.Background())
	if err := purgeUninstallData(command, home); err == nil || !strings.Contains(err.Error(), "simulated remove failure") {
		t.Fatalf("purge error = %v", err)
	}
	writer, err := app.Open(context.Background(), home)
	if err != nil {
		t.Fatalf("ledger remained unavailable after deletion failure: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close restored ledger: %v", err)
	}
}

func TestUninstallPurgeDataResumesInterruptedPurge(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "ledger")
	initializeUninstallLedger(t, home)
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	abandoned, err := prepareUninstallDataPurge(context.Background(), paths.Database)
	if err != nil {
		t.Fatalf("prepare interrupted purge: %v", err)
	}
	if err := abandoned.ReleaseForPurge(); err != nil {
		t.Fatalf("release interrupted purge: %v", err)
	}

	manager := &recordingUninstallCollectorManager{}
	original := newUninstallCollectorManager
	newUninstallCollectorManager = func() collectorManager { return manager }
	t.Cleanup(func() { newUninstallCollectorManager = original })
	command := New(Version{})
	command.SetArgs([]string{"--home", home, "uninstall", "--purge-data"})
	if err := command.Execute(); err != nil {
		t.Fatalf("resume purge through uninstall: %v", err)
	}
	if manager.uninstallCalls != 1 {
		t.Fatalf("collector uninstall calls = %d, want 1", manager.uninstallCalls)
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resumed purge retained home: %v", err)
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

type recordingUninstallCollectorManager struct {
	managedHome    string
	status         CollectorStatus
	uninstallCalls int
}

func (m *recordingUninstallCollectorManager) ResolveManagedCollectorSettings(home, listen string, homeExplicit, _ bool) (string, string) {
	if !homeExplicit && m.managedHome != "" {
		return m.managedHome, listen
	}
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
