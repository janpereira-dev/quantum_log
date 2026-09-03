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

func TestInstallersResolveChannelsUnlessVersionIsExplicit(t *testing.T) {
	cases := map[string][]string{
		"installers/install.sh":  {"QLOG_RELEASE_VERSION:-", "releases/latest", "releases?per_page=100", "GitHub returned prerelease", "resolve_release"},
		"installers/install.ps1": {"QLOG_RELEASE_VERSION", "releases/latest", "releases?per_page=100", "GitHub returned prerelease", "Resolve-Release"},
	}
	for name, required := range cases {
		t.Run(name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range required {
				if !strings.Contains(string(contents), want) {
					t.Fatalf("%s missing channel resolution %q", name, want)
				}
			}
		})
	}
}

func TestPowerShellLatestChannelEnumeratesReleaseArraysBeforeFiltering(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "installers", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(contents), "\r\n", "\n")
	response := strings.Index(script, "Invoke-RestMethod -Uri \"https://api.github.com/repos/$repository/releases?per_page=100\"")
	enumerate := strings.Index(script, "ForEach-Object { $_ }")
	filter := strings.Index(script, "Where-Object { -not $_.draft }")
	if response < 0 || enumerate < response || filter < enumerate {
		t.Fatalf("install.ps1 must enumerate the multi-release REST array before filtering it:\n%s", script)
	}
}

func TestReleaseConfigKeepsPrereleaseTagsOutOfLatest(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	config := string(contents)
	for _, want := range []string{`contains "-alpha" .Tag`, `contains "-beta" .Tag`, `contains "-rc" .Tag`, `}}false{{ else }}true{{`} {
		if !strings.Contains(config, want) {
			t.Fatalf("release config missing %q:\n%s", want, config)
		}
	}
	for _, reversed := range []string{`contains .Tag "-alpha"`, `contains .Tag "-beta"`, `contains .Tag "-rc"`} {
		if strings.Contains(config, reversed) {
			t.Fatalf("release config reverses Sprig contains arguments %q:\n%s", reversed, config)
		}
	}
}

func TestInstallGuideDocumentsPublishedRCInstallerLifecycle(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "docs", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	guide := string(contents)
	for _, want := range []string{"installers/install.sh", "installers/install.ps1", "--version v0.4.0-rc10", "--channel latest", "`v0.4.0-rc10` is a published prerelease", "The pushed `v*` tag triggers the release workflow"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("install guide missing %q", want)
		}
	}
}

func TestReleaseDocumentationDoesNotClaimUnsupportedEvidence(t *testing.T) {
	root := filepath.Join("..", "..")
	files := []string{
		"docs-int/milestones/README.md",
		"docs-int/verification/milestone-1-evidence.md",
		"docs-int/verification/m4-evidence.md",
		"docs-int/verification/m4-closure-backlog.md",
		"docs/verification/five-agent-evidence.md",
		"docs/verification/five-agent-external-evidence.md",
		"docs/INSTALL.md",
		"README.md",
		"CHANGELOG.md",
	}
	forbidden := []string{
		"M4 is VERIFIED",
		"PASS means external verification",
		"stable release is available",
		"v0.4.0 is a stable public release",
		"`PASS` means automated contract coverage or recorded external acceptance",
		"`PASS` for recorded lifecycle acceptance",
		"No public RC artifact exists yet",
		"After `v0.4.0-rc10` is published",
	}

	for _, name := range []string{"README.md", "docs/verification/five-agent-evidence.md", "docs/verification/five-agent-external-evidence.md"} {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(contents), "v0.3.2-rc.1") {
			t.Errorf("%s presents obsolete v0.3.2-rc.1 outside a dated history document", name)
		}
	}
	for _, name := range files {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, phrase := range forbidden {
			if strings.Contains(string(contents), phrase) {
				t.Errorf("%s contains unsupported current claim %q", name, phrase)
			}
		}
	}

	install, err := os.ReadFile(filepath.Join(root, "docs", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"`v0.4.0-rc10` is a published prerelease",
		"35ae43bd0031b3aca2621c52ede74731ae136357",
		"Legacy historical packaging evidence",
	} {
		if !strings.Contains(string(install), want) {
			t.Errorf("docs/INSTALL.md missing current release fact %q", want)
		}
	}

	vocabulary, err := os.ReadFile(filepath.Join(root, "docs-int", "milestones", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"`IMPLEMENTED` means code exists",
		"`READY_FOR_EXTERNAL_E2E` means the implementation can be exercised",
		"`PASS` means matching local evidence exists",
		"`VERIFIED` requires a committed acceptance matrix and independent review",
	} {
		if !strings.Contains(string(vocabulary), want) {
			t.Errorf("milestone vocabulary missing %q", want)
		}
	}
}

func TestWindowsSmokeGuideStartsItsOwnForegroundCollector(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "scripts", "smoke-v0.3.2-rc.1-windows.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"foreground collector started by this harness is terminated before continuation", "collector serve --log-file"} {
		if !strings.Contains(string(contents), want) {
			t.Fatalf("Windows smoke guide missing %q", want)
		}
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

func TestPowerShellUninstallerRejectsPurgeBeforeInvokingInstalledBinary(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "installers", "uninstall.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(contents), "\r\n", "\n")
	reject := strings.Index(script, "if ($purgeData) {\n    Fail '--purge-data is temporarily unavailable")
	invoke := strings.Index(script, "& $target @cleanupArguments")
	if reject < 0 || invoke < 0 || reject > invoke {
		t.Fatalf("uninstall.ps1 must reject --purge-data before it can invoke an installed qlog binary:\n%s", script)
	}
	if strings.Contains(script, "$cleanupArguments += '--purge-data'") {
		t.Fatalf("uninstall.ps1 forwarded --purge-data to an installed binary:\n%s", script)
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

func TestShellUninstallerRunsOwnedCleanupBeforeRemovingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell test")
	}
	root := filepath.Join("..", "..")
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(dir, "calls.txt")
	qlog := filepath.Join(bin, "qlog")
	fake := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$QLOG_TEST_CALLS\"\n"
	if err := os.WriteFile(qlog, []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(root, "installers", "uninstall.sh"), "--install-dir", bin, "--no-modify-path")
	cmd.Env = append(os.Environ(), "QLOG_TEST_CALLS="+calls)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("uninstall: %v\n%s", err, output)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "uninstall --json\n" {
		t.Fatalf("cleanup call = %q", got)
	}
	if _, err := os.Stat(qlog); !os.IsNotExist(err) {
		t.Fatalf("binary remains: %v", err)
	}
}

func TestShellUninstallerRetainsBinaryWhenOwnedCleanupFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell test")
	}
	root := filepath.Join("..", "..")
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(dir, "calls.txt")
	qlog := filepath.Join(bin, "qlog")
	fake := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$QLOG_TEST_CALLS\"\nexit 23\n"
	if err := os.WriteFile(qlog, []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(root, "installers", "uninstall.sh"), "--install-dir", bin, "--no-modify-path")
	cmd.Env = append(os.Environ(), "QLOG_TEST_CALLS="+calls)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("uninstall succeeded after cleanup failure:\n%s", output)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "uninstall --json\n" {
		t.Fatalf("cleanup call = %q", got)
	}
	if _, err := os.Stat(qlog); err != nil {
		t.Fatalf("binary was not retained: %v", err)
	}
}

func TestShellUninstallerRejectsEmptyInstallDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell test")
	}
	root := filepath.Join("..", "..")
	for name, args := range map[string][]string{
		"equals form":   {"--install-dir=", "--no-modify-path"},
		"separate form": {"--install-dir", "", "--no-modify-path"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command("sh", append([]string{filepath.Join(root, "installers", "uninstall.sh")}, args...)...)
			cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("uninstall accepted an empty install directory:\n%s", output)
			}
			if strings.Contains(string(output), "binary: /qlog") {
				t.Fatalf("uninstaller targeted /qlog before rejecting the empty directory:\n%s", output)
			}
		})
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
	command := exec.Command(pwsh, "-NoProfile", "-File", filepath.Join("..", "..", "installers", "install.ps1"), "--dry-run", "--version", "v0.3.2-rc.3", "--install-dir", installDir, "--no-modify-path", "--no-bootstrap")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created install directory: %v", err)
	}
	for _, want := range []string{
		"release: v0.3.2-rc.3",
		"qlog_0.3.2-rc.3_windows_",
		"dry-run: no files downloaded or changed",
	} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}
