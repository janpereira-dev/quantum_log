package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestPollCollectorReadinessWaitsForManagedStartup(t *testing.T) {
	attempts := 0
	status, err := pollCollectorReadiness(context.Background(), func(context.Context) (CollectorStatus, error) {
		attempts++
		if attempts < 3 {
			return CollectorStatus{Installed: true, Running: true, Message: "connect: connection refused"}, nil
		}
		return CollectorStatus{Installed: true, Running: true, Reachable: true, Message: "ok"}, nil
	})
	if err != nil {
		t.Fatalf("poll readiness: %v", err)
	}
	if attempts != 3 || !status.Reachable {
		t.Fatalf("readiness attempts=%d status=%#v", attempts, status)
	}
}

func TestCollectorStartupStatusReportsReadyAfterBoundedProbe(t *testing.T) {
	status, err := collectorStartupStatus(context.Background(), CollectorStatus{Installed: true, Running: true}, func(context.Context) (CollectorStatus, error) {
		return CollectorStatus{Installed: true, Running: true, Reachable: true, Message: "ok"}, nil
	})
	if err != nil {
		t.Fatalf("collectorStartupStatus() error = %v", err)
	}
	if !status.Reachable || status.Message != "collector started and ready" {
		t.Fatalf("collector startup status = %#v", status)
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

func TestSetupPlanDoesNotRequireDurableExecutable(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	output, err := runQLog(t, t.TempDir(), "setup", "copilot", "--dry-run")
	if err != nil {
		t.Fatalf("setup plan: %v\n%s", err, output)
	}
}

func TestSetupContinuesAfterCollectorExternalPolicyDenial(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows Task Scheduler policy diagnostics")
	}
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &policyDeniedCollectorManager{}

	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), temporaryDurableExecutable(t), true, false, adapters.Default(), manager)
	if err != nil {
		t.Fatalf("bootstrapSupportedAdapters() error = %v", err)
	}
	if !result.Collector.Installed || !result.Collector.Started {
		t.Fatalf("collector = %#v, want installed and started user fallback", result.Collector)
	}
	for _, want := range []string{"Acceso denegado", "user fallback installed and started"} {
		if !strings.Contains(strings.Join(result.Collector.Actions, "\n"), want) {
			t.Fatalf("collector actions = %q, want %q", result.Collector.Actions, want)
		}
	}
}

func TestSetupFailsForGenericAccessDeniedCollectorError(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	manager := &genericAccessDeniedCollectorManager{}
	if _, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), temporaryDurableExecutable(t), true, false, adapters.Default(), manager); err == nil {
		t.Fatal("bootstrap succeeded for generic access denied error")
	}
}

func TestSetupAllFiltersUnavailableSetupAdapters(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", "")
	output, err := runQLog(t, t.TempDir(), "setup", "--all", "--dry-run", "--executable", temporaryDurableExecutable(t))
	if err != nil {
		t.Fatalf("setup --all --dry-run: %v\n%s", err, output)
	}
	if strings.Contains(output, " | ") {
		t.Fatalf("setup --all planned unavailable adapters:\n%s", output)
	}
}

func TestSetupPlansNoUnavailableAdapters(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", "")
	output, err := runQLog(t, t.TempDir(), "setup", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("setup dry-run: %v\n%s", err, output)
	}
	var result BootstrapResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode setup output = %q: %v", output, err)
	}
	if len(result.Adapters) != 0 {
		t.Fatalf("planned adapters = %#v, want none when all are unavailable", result.Adapters)
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
			if !test.wantErr && options.ExecutablePath != canonicalExecutablePath(t, test.path) {
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
	home := t.TempDir()
	setup := exec.Command(artifact, "--home", home, "setup", "claude-code", "--yes")
	setup.Env = append(os.Environ(), "QLOG_ADAPTER_CONFIG_HOME="+configHome)
	if output, err := setup.CombinedOutput(); err != nil {
		t.Fatalf("run built artifact setup: %v\n%s", err, output)
	}

	settings, err := os.ReadFile(filepath.Join(configHome, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read generated Claude Code settings: %v", err)
	}
	contents := string(settings)
	escapedArtifact := strings.ReplaceAll(canonicalExecutablePath(t, artifact), `\`, `\\`)
	if !strings.Contains(contents, escapedArtifact) {
		t.Fatalf("generated hook does not reference built artifact %q: %s", artifact, contents)
	}
	if strings.Contains(contents, "go-build") || strings.Contains(contents, ".test") {
		t.Fatalf("generated hook references transient Go executable: %s", contents)
	}

	copilotHome := filepath.Join(t.TempDir(), "copilot-hooks")
	directInstall := exec.Command(artifact, "--home", home, "adapter", "install", "copilot")
	directInstall.Env = append(os.Environ(), "COPILOT_HOME="+copilotHome, "QLOG_ADAPTER_CONFIG_HOME=")
	if output, err := directInstall.CombinedOutput(); err != nil {
		t.Fatalf("run built artifact adapter install: %v\n%s", err, output)
	}
	hooks, err := os.ReadFile(filepath.Join(copilotHome, "hooks", "qlog.json"))
	if err != nil {
		t.Fatalf("read generated Copilot hooks: %v", err)
	}
	escapedHome := strings.ReplaceAll(home, `\`, `\\`)
	if !strings.Contains(string(hooks), escapedArtifact) || !strings.Contains(string(hooks), escapedHome) {
		t.Fatalf("generated Copilot hook does not use artifact and selected home: %s", hooks)
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

func canonicalExecutablePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve executable path %q: %v", path, err)
	}
	return filepath.Clean(resolved)
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

type genericAccessDeniedCollectorManager struct{ fakeCollectorManager }

func (*policyDeniedCollectorManager) Install(_, _ string) (CollectorStatus, error) {
	return CollectorStatus{}, errors.New(`task scheduler operation /Create for task "QUANTUM_LOG Collector" failed: exit status 1: Error: Acceso denegado.`)
}

func (m *policyDeniedCollectorManager) InstallFallback(_, listen string) (CollectorStatus, error) {
	m.installed = true
	m.started = true
	return CollectorStatus{Installed: true, Running: true, Listen: listen, Message: "user fallback installed and started"}, nil
}

func (*genericAccessDeniedCollectorManager) Install(_, _ string) (CollectorStatus, error) {
	return CollectorStatus{}, errors.New("create collector directory: access denied")
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
