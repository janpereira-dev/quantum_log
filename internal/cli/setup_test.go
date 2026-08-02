package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
)

func TestSetupYesBootstrapsCollectorBeforeAdapterFiles(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &fakeCollectorManager{}

	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), true, false, adapters.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Consent || !manager.installed || !manager.started {
		t.Fatalf("bootstrap = %#v", result)
	}
	if got := adapterIDs(result.Adapters); !slices.Equal(got, []string{"claude-code", "codex", "copilot-vscode", "opencode"}) {
		t.Fatalf("adapters = %v", got)
	}
}

func TestSetupWithoutConsentOnlyPrintsPlan(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &fakeCollectorManager{}

	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), false, false, adapters.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	if result.Consent || manager.installed || manager.started {
		t.Fatalf("mutated without consent: %#v", result)
	}
}

func TestSetupAllIncludesNonStableSetupAdapters(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	output, err := runQLog(t, t.TempDir(), "setup", "--all", "--dry-run")
	if err != nil {
		t.Fatalf("setup --all --dry-run: %v\n%s", err, output)
	}
	for _, adapterID := range []string{"claude-code", "codex", "copilot-vscode", "opencode", "pi", "openclaw", "hermes"} {
		if !strings.Contains(output, adapterID+" |") {
			t.Fatalf("setup --all output missing %q:\n%s", adapterID, output)
		}
	}
}

func TestSetupYesInitializesLedgerBeforeCollectorInstall(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	manager := &ledgerCheckingCollectorManager{}

	if _, err := bootstrapSupportedAdapters(context.Background(), home, true, false, adapters.Default(), manager); err != nil {
		t.Fatal(err)
	}
	if !manager.ledgerExistedAtInstall {
		t.Fatal("collector install ran before ledger initialization")
	}
}

type fakeCollectorManager struct {
	installed bool
	started   bool
}

type ledgerCheckingCollectorManager struct {
	fakeCollectorManager
	ledgerExistedAtInstall bool
}

func (m *ledgerCheckingCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if _, err := os.Stat(filepath.Join(home, "qlog.db")); err != nil {
		return CollectorStatus{}, fmt.Errorf("ledger unavailable at collector install: %w", err)
	}
	m.ledgerExistedAtInstall = true
	return m.fakeCollectorManager.Install(home, listen)
}

func (m *fakeCollectorManager) Install(_, listen string) (CollectorStatus, error) {
	m.installed = true
	return CollectorStatus{Installed: true, Listen: listen, ServiceID: "test.collector", Message: "collector installed"}, nil
}

func (m *fakeCollectorManager) Start(_, listen string) (CollectorStatus, error) {
	m.started = true
	return CollectorStatus{Installed: true, Running: true, Listen: listen, Message: "collector started"}, nil
}

func (*fakeCollectorManager) Stop() (CollectorStatus, error) {
	return CollectorStatus{Message: "collector stopped"}, nil
}

func (m *fakeCollectorManager) Restart(home, listen string) (CollectorStatus, error) {
	return m.Start(home, listen)
}

func (*fakeCollectorManager) Logs() (string, error) { return "", nil }

func (*fakeCollectorManager) Uninstall() (CollectorStatus, error) {
	return CollectorStatus{Message: "collector uninstalled"}, nil
}

func adapterIDs(plans []adapters.SetupPlan) []string {
	ids := make([]string, 0, len(plans))
	for _, plan := range plans {
		ids = append(ids, plan.AdapterID)
	}
	return ids
}
