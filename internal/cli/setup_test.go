package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
)

func TestSetupYesBootstrapsCollectorBeforeAdapterFiles(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &fakeCollectorManager{}

	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), temporaryDurableExecutable(t), true, false, adapters.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Consent || !manager.installed || !manager.started {
		t.Fatalf("bootstrap = %#v", result)
	}
	if got := adapterIDs(result.Adapters); !slices.Equal(got, []string{"claude-code", "codex", "copilot", "copilot-vscode", "opencode"}) {
		t.Fatalf("adapters = %v", got)
	}
}

func TestSetupWithoutConsentOnlyPrintsPlan(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &fakeCollectorManager{}

	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), "", false, false, adapters.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	if result.Consent || manager.installed || manager.started {
		t.Fatalf("mutated without consent: %#v", result)
	}
}

func TestSetupContinuesAfterCollectorExternalPolicyDenial(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &policyDeniedCollectorManager{}

	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), temporaryDurableExecutable(t), true, false, adapters.Default(), manager)
	if err != nil {
		t.Fatalf("bootstrapSupportedAdapters() error = %v", err)
	}
	if result.Collector.Installed || result.Collector.Started {
		t.Fatalf("collector = %#v, want external-policy diagnosis without activation", result.Collector)
	}
	if !strings.Contains(strings.Join(result.Collector.Actions, "\n"), "Acceso denegado") {
		t.Fatalf("collector actions = %q, want exact scheduler diagnosis", result.Collector.Actions)
	}
}

func TestSetupAllIncludesNonStableSetupAdapters(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	output, err := runQLog(t, t.TempDir(), "setup", "--all", "--dry-run", "--executable", temporaryDurableExecutable(t))
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

	if _, err := bootstrapSupportedAdapters(context.Background(), home, temporaryDurableExecutable(t), true, false, adapters.Default(), manager); err != nil {
		t.Fatal(err)
	}
	if !manager.ledgerExistedAtInstall {
		t.Fatal("collector install ran before ledger initialization")
	}
}

func TestSetupInstallOptionsDeriveDurableExecutableForManualSetup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	options, err := setupInstallOptions(t.TempDir(), "")
	if transientErr := validateCollectorExecutable(executable); transientErr != nil {
		if err == nil {
			t.Fatalf("setup accepted transient test executable %q", executable)
		}
		return
	}
	if err != nil {
		t.Fatalf("setup install options: %v", err)
	}
	if !filepath.IsAbs(options.ExecutablePath) {
		t.Fatalf("manual setup executable path is not absolute: %q", options.ExecutablePath)
	}
	if _, err := os.Stat(options.ExecutablePath); err != nil {
		t.Fatalf("manual setup executable path is unusable: %v", err)
	}
}

func TestSetupInstallOptionsRejectsTransientExecutablePaths(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "go test binary", path: filepath.Join(root, "qlog.test"), wantErr: true},
		{name: "windows go test binary", path: filepath.Join(root, "qlog.test.exe"), wantErr: true},
		{name: "go build cache binary", path: filepath.Join(root, "go-build123", "qlog"), wantErr: true},
		{name: "installed binary", path: filepath.Join(root, "bin", "qlog"), wantErr: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Dir(test.path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(test.path, nil, 0o700); err != nil {
				t.Fatal(err)
			}
			options, err := setupInstallOptions(t.TempDir(), test.path)
			if (err != nil) != test.wantErr {
				t.Fatalf("setupInstallOptions() error = %v, wantErr %t", err, test.wantErr)
			}
			if !test.wantErr && options.ExecutablePath != filepath.Clean(test.path) {
				t.Fatalf("ExecutablePath = %q", options.ExecutablePath)
			}
		})
	}
}

func TestBuiltArtifactSetupWritesDurableHookCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and executes qlog artifact")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	artifact := filepath.Join(t.TempDir(), "qlog.exe")
	build := exec.Command("go", "build", "-o", artifact, "./cmd/qlog")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build qlog artifact: %v\n%s", err, output)
	}

	configHome := t.TempDir()
	setup := exec.Command(artifact, "--home", t.TempDir(), "setup", "claude-code", "--yes")
	setup.Env = append(os.Environ(), "QLOG_ADAPTER_CONFIG_HOME="+configHome)
	if output, err := setup.CombinedOutput(); err != nil {
		t.Fatalf("run built artifact setup: %v\n%s", err, output)
	}

	settings, err := os.ReadFile(filepath.Join(configHome, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read generated Claude Code settings: %v", err)
	}
	contents := string(settings)
	escapedArtifact := strings.ReplaceAll(artifact, `\`, `\\`)
	if !strings.Contains(contents, escapedArtifact) {
		t.Fatalf("generated hook does not reference built artifact %q: %s", artifact, contents)
	}
	if strings.Contains(contents, "go-build") || strings.Contains(contents, ".test") {
		t.Fatalf("generated hook references transient Go executable: %s", contents)
	}
}

func temporaryDurableExecutable(t *testing.T) string {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read test executable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bin", "qlog.exe")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create durable executable directory: %v", err)
	}
	if err := os.WriteFile(path, contents, 0o700); err != nil {
		t.Fatalf("write durable executable: %v", err)
	}
	return path
}

type fakeCollectorManager struct {
	installed bool
	started   bool
}

type ledgerCheckingCollectorManager struct {
	fakeCollectorManager
	ledgerExistedAtInstall bool
}

type policyDeniedCollectorManager struct{ fakeCollectorManager }

func (*policyDeniedCollectorManager) Install(_, _ string) (CollectorStatus, error) {
	return CollectorStatus{}, errors.New(`task scheduler operation /Create for task "QUANTUM_LOG Collector" failed: exit status 1: Error: Acceso denegado.`)
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
