package cli

import (
	"context"
	"slices"
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

type fakeCollectorManager struct {
	installed bool
	started   bool
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
