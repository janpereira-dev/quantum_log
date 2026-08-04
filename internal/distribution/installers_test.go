package distribution

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerContracts(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := map[string][]string{
		"installers/install.sh":    {"--dry-run", "checksums.txt", "SHA-256", "--no-modify-path", "QLOG_RELEASE_BASE"},
		"installers/uninstall.sh":  {"--dry-run", "--no-modify-path", "data is preserved"},
		"installers/install.ps1":   {"--dry-run", "checksums.txt", "Get-FileHash", "--no-modify-path", "QLOG_RELEASE_BASE"},
		"installers/uninstall.ps1": {"--dry-run", "--no-modify-path", "data is preserved"},
		"installers/install.cmd":   {"install.ps1", "ExecutionPolicy Bypass"},
		"installers/uninstall.cmd": {"uninstall.ps1", "ExecutionPolicy Bypass"},
	}
	for name, required := range cases {
		t.Run(name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			for _, text := range required {
				if !strings.Contains(string(contents), text) {
					t.Errorf("%s does not document or implement %q", name, text)
				}
			}
		})
	}
}

func TestOfficialQlogInstallersExposeConsentedBootstrapAndOptOut(t *testing.T) {
	for _, name := range []string{"installers/install.ps1", "installers/install.sh"} {
		contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"--bootstrap", "--no-bootstrap", "qlog setup --yes", "consent"} {
			if !strings.Contains(string(contents), want) {
				t.Fatalf("%s missing %q", name, want)
			}
		}
	}
}

func TestShellInstallerBootstrapPassesVerifiedExecutablePath(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "installers", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"$INSTALL_DIR/qlog" setup --yes --executable "$INSTALL_DIR/qlog"`) {
		t.Fatal("install.sh bootstrap must pass the verified installed qlog path to setup")
	}
	if !strings.Contains(string(contents), `*) INSTALL_DIR="$(pwd -P)/$INSTALL_DIR" ;;`) {
		t.Fatal("install.sh must normalize relative install directories before passing hook executable paths")
	}
}

func TestInstallersBootstrapWithDurableExecutableAndHealthCheck(t *testing.T) {
	cases := map[string][]string{
		"installers/install.sh": {
			`"$INSTALL_DIR/qlog" setup --yes --executable "$INSTALL_DIR/qlog"`,
			`"$INSTALL_DIR/qlog" doctor`,
			`qlog health check failed`,
		},
		"installers/install.ps1": {
			`$installDir = [System.IO.Path]::GetFullPath($installDir)`,
			`& $target setup --yes --executable $target`,
			`& $target doctor`,
			`qlog health check failed`,
		},
	}
	for name, required := range cases {
		t.Run(name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range required {
				if !strings.Contains(string(contents), want) {
					t.Fatalf("%s missing %q", name, want)
				}
			}
		})
	}
}

func TestPowerShellInstallerSkipsBootstrapPromptWhenNoninteractive(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "installers", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "$bootstrap -eq $null -and -not $dryRun -and -not [Console]::IsInputRedirected") {
		t.Fatal("install.ps1 must prompt only when an interactive PowerShell host is available")
	}
}

func TestM4EvidenceDocumentsStableScopeAndCleanDeviceGate(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "docs-int", "verification", "m4-evidence.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"codex",
		"claude-code",
		"copilot-vscode",
		"opencode",
		"IN_PROGRESS",
		"real-agent",
		"## Clean-Device Acceptance Protocol",
		"Device OS/version/architecture",
		"adapter verify --json",
		"Replay result",
		"Privacy inspection result",
	} {
		if !strings.Contains(string(contents), want) {
			t.Fatalf("M4 evidence missing %q", want)
		}
	}
	for _, forbidden := range []string{"M4 is VERIFIED", "Pi", "OpenClaw", "Hermes"} {
		if strings.Contains(string(contents), forbidden) {
			t.Fatalf("M4 evidence contains unsupported completion or adapter claim %q", forbidden)
		}
	}
}

func TestShellInstallDryRunDoesNotWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell smoke test runs on Unix CI jobs")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is unavailable")
	}
	installDir := filepath.Join(t.TempDir(), "bin")
	command := exec.Command("sh", filepath.Join("..", "..", "installers", "install.sh"), "--dry-run", "--version", "v0.0.0", "--install-dir", installDir, "--no-modify-path")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created install directory: %v", err)
	}
	if !strings.Contains(string(output), "dry-run: no files downloaded or changed") {
		t.Fatalf("dry-run output = %q", output)
	}
}

func TestPowerShellInstallDryRunPinsRequestedCandidateWithoutWrites(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell installer smoke test runs on Windows CI jobs")
	}
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh is unavailable")
	}
	installDir := filepath.Join(t.TempDir(), "bin")
	command := exec.Command(pwsh, "-NoProfile", "-File", filepath.Join("..", "..", "installers", "install.ps1"), "--dry-run", "--version", "v0.3.2-rc.1", "--install-dir", installDir, "--no-modify-path", "--no-bootstrap")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created install directory: %v", err)
	}
	for _, want := range []string{
		"release: v0.3.2-rc.1",
		"qlog_0.3.2-rc.1_windows_",
		"dry-run: no files downloaded or changed",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}
