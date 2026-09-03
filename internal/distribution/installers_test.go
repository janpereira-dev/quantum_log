package distribution

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func normalizeDocumentationText(contents []byte) string {
	plain := strings.NewReplacer("`", "", "*", "", "_", " ").Replace(strings.ToLower(string(contents)))
	return strings.Join(strings.Fields(plain), " ")
}

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
		"docs/TROUBLESHOOTING.md",
		"packaging/npm/README.md",
		"README.md",
		"CHANGELOG.md",
	}
	forbidden := []string{
		"m4 is verified",
		"pass means external verification",
		"stable release is available",
		"v0.4.0 is a stable public release",
		"pass means automated contract coverage or recorded external acceptance",
		"pass for recorded lifecycle acceptance",
		"no public rc artifact exists yet",
		"after v0.4.0-rc10 is published",
	}

	for _, name := range []string{"README.md", "docs/verification/five-agent-external-evidence.md"} {
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
		text := normalizeDocumentationText(contents)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Errorf("%s contains unsupported current claim %q", name, phrase)
			}
		}
	}

	troubleshooting, err := os.ReadFile(filepath.Join(root, "docs", "TROUBLESHOOTING.md"))
	if err != nil {
		t.Fatal(err)
	}
	troubleshootingText := normalizeDocumentationText(troubleshooting)
	if strings.Contains(troubleshootingText, "blocked until signed https rc artifact exists") {
		t.Error("docs/TROUBLESHOOTING.md says the signed RC is unavailable although RC10 is published")
	}
	if strings.Contains(troubleshootingText, "qlog install local artifact dir") && strings.Contains(troubleshootingText, "npm install") {
		t.Error("docs/TROUBLESHOOTING.md promotes the legacy npm artifact route for current RC validation")
	}

	npmGuide, err := os.ReadFile(filepath.Join(root, "packaging", "npm", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	npmText := normalizeDocumentationText(npmGuide)
	if strings.Contains(npmText, "distributor for verified quantum_log") ||
		strings.Contains(npmText, "npm install -g @janpereira.dev/quantum-log") ||
		(strings.Contains(npmText, "downloads matching") && strings.Contains(npmText, "v0.3.2-rc.3")) {
		t.Error("packaging/npm/README.md presents the stale npm package as the verified current installation path")
	}

	for _, name := range []string{"README.md", "docs/INSTALL.md"} {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		text := normalizeDocumentationText(contents)
		if strings.Contains(text, "there is no stable public release yet") ||
			!strings.Contains(text, "no stable v0.4.0 release has been published") ||
			!strings.Contains(text, "older stable release line is not the supported evaluation path") {
			t.Errorf("%s does not distinguish the absent stable v0.4.0 release from the published RC10 and older stable line", name)
		}
	}

	fiveAgent, err := os.ReadFile(filepath.Join(root, "docs", "verification", "five-agent-evidence.md"))
	if err != nil {
		t.Fatal(err)
	}
	fiveAgentText := normalizeDocumentationText(fiveAgent)
	if strings.Contains(fiveAgentText, "partial:") || strings.Contains(fiveAgentText, "blocked external") ||
		!strings.Contains(fiveAgentText, "ready for external e2e") || !strings.Contains(fiveAgentText, "2026-08-05") || !strings.Contains(fiveAgentText, "v0.3.2-rc.1") {
		t.Error("docs/verification/five-agent-evidence.md uses PARTIAL outside the four-state vocabulary or leaves historical evidence unbound")
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

func TestDocumentationNormalizationMakesLegacyEnvironmentTokenSearchable(t *testing.T) {
	normalized := normalizeDocumentationText([]byte("Use QLOG_INSTALL_LOCAL_ARTIFACT_DIR with npm install for RC validation"))
	legacyRouteDetected := strings.Contains(normalized, "qlog install local artifact dir") && strings.Contains(normalized, "npm install")
	if !legacyRouteDetected {
		t.Fatalf("normalized legacy environment token = %q", normalized)
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

type lifecycleEngine struct {
	name       string
	executable string
	script     string
	contract   string
	powerShell bool
}

func lifecycleEngines(t *testing.T) []lifecycleEngine {
	t.Helper()
	root := filepath.Join("..", "..", "scripts", "acceptance")
	engines := []lifecycleEngine{{name: "POSIX sh", executable: "sh", script: filepath.Join(root, "release-lifecycle.sh"), contract: "--contract-only"}}
	if runtime.GOOS == "windows" {
		engines = append(engines,
			lifecycleEngine{name: "PowerShell 5.1", executable: "powershell.exe", script: filepath.Join(root, "release-lifecycle.ps1"), contract: "-ContractOnly", powerShell: true},
			lifecycleEngine{name: "pwsh", executable: "pwsh", script: filepath.Join(root, "release-lifecycle.ps1"), contract: "-ContractOnly", powerShell: true},
		)
	}
	return engines
}

func lifecycleCommand(t *testing.T, engine lifecycleEngine, arguments ...string) *exec.Cmd {
	t.Helper()
	executable, err := exec.LookPath(engine.executable)
	if err != nil {
		t.Skipf("%s is unavailable", engine.executable)
	}
	if engine.powerShell {
		prefix := []string{"-NoProfile"}
		if engine.executable == "powershell.exe" {
			prefix = append(prefix, "-ExecutionPolicy", "Bypass")
		}
		arguments = append(append(prefix, "-File", engine.script), arguments...)
	} else {
		arguments = append([]string{engine.script}, arguments...)
	}
	return exec.Command(executable, arguments...)
}

func lifecycleEnvironment(overrides ...string) []string {
	blocked := []string{"QLOG_FROM_VERSION=", "QLOG_TO_VERSION=", "QLOG_RELEASE_BASE=", "QLOG_EVIDENCE_DIR=", "QLOG_INSTALLER_SH=", "QLOG_UNINSTALLER_SH=", "QLOG_INSTALLER_PS1=", "QLOG_UNINSTALLER_PS1=", "QLOG_FAKE_"}
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		upper := strings.ToUpper(entry)
		skip := false
		for _, prefix := range blocked {
			if strings.HasPrefix(upper, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			environment = append(environment, entry)
		}
	}
	return append(environment, overrides...)
}

func buildLifecycleFakeQlog(t *testing.T, directory string) string {
	t.Helper()
	source := filepath.Join(directory, "fake-qlog.go")
	program := `package main
import ("fmt"; "os"; "path/filepath"; "strings")
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(2) }
func main() {
 args := os.Args[1:]
 calls := os.Getenv("QLOG_FAKE_CALLS")
 if calls != "" { f, _ := os.OpenFile(calls, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); if f != nil { fmt.Fprintln(f, "qlog|"+strings.Join(args, "|")); _ = f.Close() } }
 executable, _ := os.Executable(); versionBytes, _ := os.ReadFile(filepath.Join(filepath.Dir(executable), "version.txt")); version := strings.TrimSpace(string(versionBytes))
 if len(args) == 1 && args[0] == "--version" { fmt.Printf("qlog %s (commit mock, built test)\n", strings.TrimPrefix(version, "v")); return }
 if len(args) < 3 || args[0] != "--home" { if len(args) == 2 && args[0] == "uninstall" && args[1] == "--json" { fmt.Println("{}"); return }; fail("missing --home") }
 home, command := args[1], args[2]; ledger := filepath.Join(home, "qlog.db")
 switch command {
 case "init": if len(args) != 3 { fail("bad init argv") }; _ = os.MkdirAll(home, 0700); if err := os.WriteFile(ledger, []byte("ledger-v1\n"), 0600); err != nil { fail(err.Error()) }
 case "ingest": if len(args) != 5 || args[3] != "file" { fail("bad ingest argv") }; fixture, err := os.ReadFile(args[4]); if err != nil || !strings.Contains(string(fixture), "qlog-release-lifecycle-v1") { fail("bad sentinel") }; f, err := os.OpenFile(ledger, os.O_APPEND|os.O_WRONLY, 0600); if err != nil { fail(err.Error()) }; _, _ = f.Write([]byte("sentinel-v1\n")); _ = f.Close()
 case "doctor": if len(args) != 4 || args[3] != "--json" { fail("bad doctor argv") }; fmt.Printf("{\"database\":%q,\"status\":\"ok\"}\n", ledger)
 case "verify": if len(args) != 3 { fail("bad verify argv") }; fmt.Printf("verified %s\n", home)
 default: fail("unknown command")
 }
}`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "fake-qlog")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-o", binary, source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fake qlog: %v\n%s", err, output)
	}
	return binary
}

func writeLifecycleMocks(t *testing.T, directory string, powerShell bool) (string, string) {
	t.Helper()
	if powerShell {
		installer := filepath.Join(directory, "install.ps1")
		uninstaller := filepath.Join(directory, "uninstall.ps1")
		installScript := `param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$Arguments -join '|' | Add-Content -LiteralPath $env:QLOG_FAKE_CALLS -Encoding ASCII
if ($Arguments.Count -ne 6 -or $Arguments[0] -ne '--version' -or $Arguments[2] -ne '--install-dir' -or $Arguments[4] -ne '--no-modify-path' -or $Arguments[5] -ne '--no-bootstrap') { throw "bad installer argv: $($Arguments -join '|')" }
$version = $Arguments[1]; $installDir = $Arguments[3]
if (($env:QLOG_FAKE_ADVERSARIAL_VERSION -eq 'source' -and $version -eq 'v1') -or ($env:QLOG_FAKE_ADVERSARIAL_VERSION -eq 'target' -and $version -eq 'v2')) { $version = $version + '0' }
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
Copy-Item -LiteralPath $env:QLOG_FAKE_BINARY -Destination (Join-Path $installDir 'qlog.exe') -Force
Set-Content -LiteralPath (Join-Path $installDir 'version.txt') -Value $version -Encoding ASCII
`
		uninstallScript := `param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
('uninstaller|' + ($Arguments -join '|')) | Add-Content -LiteralPath $env:QLOG_FAKE_CALLS -Encoding ASCII
if ($Arguments.Count -ne 3 -or $Arguments[0] -ne '--install-dir' -or $Arguments[2] -ne '--no-modify-path') { throw "bad uninstaller argv: $($Arguments -join '|')" }
$target = Join-Path $Arguments[1] 'qlog.exe'; & $target 'uninstall' '--json' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'cleanup failed' }; Remove-Item -LiteralPath $target -Force
`
		if err := os.WriteFile(installer, []byte(installScript), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(uninstaller, []byte(uninstallScript), 0o600); err != nil {
			t.Fatal(err)
		}
		return installer, uninstaller
	}
	installer := filepath.Join(directory, "install.sh")
	uninstaller := filepath.Join(directory, "uninstall.sh")
	installScript := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$QLOG_FAKE_CALLS"
[ "$#" -eq 6 ] && [ "$1" = --version ] && [ "$3" = --install-dir ] && [ "$5" = --no-modify-path ] && [ "$6" = --no-bootstrap ] || exit 22
version=$2
case "${QLOG_FAKE_ADVERSARIAL_VERSION:-}:$version" in source:v1|target:v2) version="${version}0" ;; esac
mkdir -p "$4"; cp "$QLOG_FAKE_BINARY" "$4/qlog"; chmod +x "$4/qlog"; printf '%s\n' "$version" > "$4/version.txt"
`
	uninstallScript := `#!/bin/sh
set -eu
printf 'uninstaller|%s\n' "$*" >> "$QLOG_FAKE_CALLS"
[ "$#" -eq 3 ] && [ "$1" = --install-dir ] && [ "$3" = --no-modify-path ] || exit 22
"$2/qlog" uninstall --json >/dev/null; rm -f "$2/qlog"
`
	if err := os.WriteFile(installer, []byte(installScript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(uninstaller, []byte(uninstallScript), 0o700); err != nil {
		t.Fatal(err)
	}
	return installer, uninstaller
}

func runAdversarialLifecycleVersion(t *testing.T, engine lifecycleEngine, mode string) ([]byte, error) {
	t.Helper()
	sandbox := t.TempDir()
	tempRoot := filepath.Join(sandbox, "temp")
	evidence := filepath.Join(sandbox, "evidence")
	mocks := filepath.Join(sandbox, "mocks")
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mocks, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := buildLifecycleFakeQlog(t, mocks)
	installer, uninstaller := writeLifecycleMocks(t, mocks, engine.powerShell)
	calls := filepath.Join(sandbox, "calls.txt")
	overrides := []string{"HOME=" + sandbox, "TMPDIR=" + tempRoot, "TEMP=" + tempRoot, "TMP=" + tempRoot, "QLOG_FROM_VERSION=v1", "QLOG_TO_VERSION=v2", "QLOG_RELEASE_BASE=https://invalid.example/releases", "QLOG_EVIDENCE_DIR=" + evidence, "QLOG_FAKE_BINARY=" + fake, "QLOG_FAKE_CALLS=" + calls, "QLOG_FAKE_ADVERSARIAL_VERSION=" + mode}
	if engine.powerShell {
		overrides = append(overrides, "QLOG_INSTALLER_PS1="+installer, "QLOG_UNINSTALLER_PS1="+uninstaller)
	} else {
		overrides = append(overrides, "QLOG_INSTALLER_SH="+installer, "QLOG_UNINSTALLER_SH="+uninstaller)
	}
	command := lifecycleCommand(t, engine)
	command.Env = lifecycleEnvironment(overrides...)
	return command.CombinedOutput()
}

func TestReleaseLifecycleHarnessContracts(t *testing.T) {
	for _, engine := range lifecycleEngines(t) {
		engine := engine
		t.Run(engine.name+" contract validation", func(t *testing.T) {
			sandbox := t.TempDir()
			command := lifecycleCommand(t, engine, engine.contract)
			command.Env = lifecycleEnvironment("HOME="+sandbox, "TMPDIR="+sandbox, "TEMP="+sandbox, "TMP="+sandbox)
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "QLOG_FROM_VERSION is required") {
				t.Fatalf("contract-only accepted missing explicit inputs: err=%v output=%q", err, output)
			}
			command = lifecycleCommand(t, engine, engine.contract)
			command.Env = lifecycleEnvironment("HOME="+sandbox, "TMPDIR="+sandbox, "TEMP="+sandbox, "TMP="+sandbox,
				"QLOG_FROM_VERSION=v1", "QLOG_TO_VERSION=v2", "QLOG_RELEASE_BASE=https://invalid.example/releases")
			output, err = command.CombinedOutput()
			if err != nil || !strings.Contains(string(output), "PASS contract: explicit versions and isolated home") {
				t.Fatalf("contract-only failed: %v\n%s", err, output)
			}
			entries, readErr := os.ReadDir(sandbox)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("contract-only retained writes: %v", entries)
			}
		})

		t.Run(engine.name+" full lifecycle", func(t *testing.T) {
			sandbox := t.TempDir()
			tempRoot := filepath.Join(sandbox, "temp")
			if runtime.GOOS != "windows" {
				tempRoot = filepath.Join(sandbox, "temp|delimiter")
			}
			if err := os.MkdirAll(tempRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			evidence := filepath.Join(sandbox, "evidence")
			mocks := filepath.Join(sandbox, "mocks")
			if err := os.MkdirAll(mocks, 0o700); err != nil {
				t.Fatal(err)
			}
			fake := buildLifecycleFakeQlog(t, mocks)
			installer, uninstaller := writeLifecycleMocks(t, mocks, engine.powerShell)
			calls := filepath.Join(sandbox, "calls.txt")
			overrides := []string{"HOME=" + sandbox, "TMPDIR=" + tempRoot, "TEMP=" + tempRoot, "TMP=" + tempRoot,
				"QLOG_FROM_VERSION=v1", "QLOG_TO_VERSION=v2", "QLOG_RELEASE_BASE=https://invalid.example/releases",
				"QLOG_EVIDENCE_DIR=" + evidence, "QLOG_FAKE_BINARY=" + fake, "QLOG_FAKE_CALLS=" + calls}
			if engine.powerShell {
				overrides = append(overrides, "QLOG_INSTALLER_PS1="+installer, "QLOG_UNINSTALLER_PS1="+uninstaller)
			} else {
				overrides = append(overrides, "QLOG_INSTALLER_SH="+installer, "QLOG_UNINSTALLER_SH="+uninstaller)
			}
			command := lifecycleCommand(t, engine)
			command.Env = lifecycleEnvironment(overrides...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("full lifecycle failed: %v\n%s", err, output)
			}
			if !strings.Contains(string(output), "PASS lifecycle: v1 -> v2") {
				t.Fatalf("output=%q", output)
			}
			callBytes, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			callsText := strings.ReplaceAll(string(callBytes), "\r\n", "\n")
			normalizedCalls := strings.ReplaceAll(callsText, " ", "|")
			for _, want := range []string{"--version|v1|--install-dir|", "--version|v2|--install-dir|", "qlog|--home|", "|ingest|file|", "qlog|uninstall|--json", "uninstaller|--install-dir|"} {
				if !strings.Contains(normalizedCalls, want) {
					t.Errorf("argv log missing %q:\n%s", want, callsText)
				}
			}
			if got := strings.Count(normalizedCalls, "|doctor|--json"); got != 1 {
				t.Errorf("doctor execution count = %d, want 1:\n%s", got, callsText)
			}
			if got := strings.Count(normalizedCalls, "|verify\n"); got != 2 {
				t.Errorf("verify execution count = %d, want 2:\n%s", got, callsText)
			}
			before, err := os.ReadFile(filepath.Join(evidence, "ledger-before.sha256"))
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"ledger-after-upgrade.sha256", "ledger-after-uninstall.sha256", "ledger-after-reinstall.sha256"} {
				after, readErr := os.ReadFile(filepath.Join(evidence, name))
				if readErr != nil {
					t.Fatal(readErr)
				}
				if strings.TrimSpace(string(before)) != strings.TrimSpace(string(after)) {
					t.Errorf("ledger hash changed in %s", name)
				}
			}
			for _, name := range []string{"doctor.json", "verify.txt", "verify-reinstall.txt"} {
				contents, readErr := os.ReadFile(filepath.Join(evidence, name))
				if readErr != nil {
					t.Fatal(readErr)
				}
				if strings.Contains(string(contents), tempRoot) || !strings.Contains(string(contents), "<TEMP>") {
					t.Errorf("%s was not sanitized: %q", name, contents)
				}
			}
			if _, err := os.Stat(filepath.Join(evidence, "sentinel.ndjson")); !os.IsNotExist(err) {
				t.Errorf("raw sentinel leaked into evidence: %v", err)
			}
			remaining, err := os.ReadDir(tempRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(remaining) != 0 {
				t.Errorf("temporary lifecycle state was not cleaned: %v", remaining)
			}
		})

		t.Run(engine.name+" exact version rejection", func(t *testing.T) {
			for _, stage := range []string{"source", "target"} {
				output, err := runAdversarialLifecycleVersion(t, engine, stage)
				if err == nil || !strings.Contains(string(output), "installed "+stage+" version does not match") {
					t.Fatalf("accepted adversarial %s version: err=%v output=%q", stage, err, output)
				}
			}
		})
	}
}

func workflowStepRun(workflow, stepName string) (string, bool) {
	inStep := false
	for _, line := range strings.Split(strings.ReplaceAll(workflow, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "- name: "+stepName {
			inStep = true
			continue
		}
		if inStep && strings.HasPrefix(trimmed, "- name: ") {
			return "", false
		}
		if inStep && strings.HasPrefix(trimmed, "run: ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "run: ")), true
		}
	}
	return "", false
}

func TestHostedArtifactLifecycleWorkflowContract(t *testing.T) {
	root := filepath.Join("..", "..")
	contents, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "artifact-lifecycle.yml"))
	if err != nil {
		t.Fatalf("read hosted artifact lifecycle workflow: %v", err)
	}
	workflow := strings.ReplaceAll(string(contents), "\r\n", "\n")
	required := []string{
		"workflow_dispatch:",
		"workflow_call:",
		"from_version:",
		"to_version:",
		"release_base:",
		"permissions:\n  contents: read",
		"timeout-minutes:",
		"os: [ubuntu-latest, macos-latest, windows-latest]",
		"actions/checkout@v7",
		"actions/upload-artifact@v6",
		"if: always()",
		"QLOG_FROM_VERSION:",
		"QLOG_TO_VERSION:",
		"QLOG_RELEASE_BASE:",
		`default: ''`,
		`[ "$supplied" -eq 0 ]`,
		"echo 'live=false'",
		`[ "$supplied" -ne 3 ]`,
		"needs.validate-inputs.outputs.live != 'true'",
		"needs.validate-inputs.outputs.live == 'true'",
	}
	for _, want := range required {
		if !strings.Contains(workflow, want) {
			t.Errorf("artifact lifecycle workflow missing %q", want)
		}
	}
	for _, forbidden := range []string{"version: latest", "releases/latest", "QLOG_FROM_VERSION: latest", "QLOG_TO_VERSION: latest", "secrets."} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("artifact lifecycle workflow contains unsafe or mutable selector %q", forbidden)
		}
	}
	stepCommands := map[string]string{
		"Validate POSIX lifecycle contract":      "sh scripts/acceptance/release-lifecycle.sh --contract-only",
		"Validate PowerShell lifecycle contract": "pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1 -ContractOnly",
		"Run POSIX artifact lifecycle":           "sh scripts/acceptance/release-lifecycle.sh",
		"Run PowerShell artifact lifecycle":      "pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1",
	}
	for name, want := range stepCommands {
		got, found := workflowStepRun(workflow, name)
		if !found {
			t.Errorf("artifact lifecycle workflow is missing named step %q with a run command", name)
			continue
		}
		if got != want {
			t.Errorf("artifact lifecycle workflow step %q runs %q, want exactly %q", name, got, want)
		}
	}
}

func TestCIUsesHostedLifecycleOnlyAsContractValidation(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := strings.ReplaceAll(string(contents), "\r\n", "\n")
	for _, want := range []string{"artifact-lifecycle-contract:", "uses: ./.github/workflows/artifact-lifecycle.yml"} {
		if !strings.Contains(workflow, want) {
			t.Errorf("CI does not wire hosted lifecycle contract validation: missing %q", want)
		}
	}
	jobStart := strings.Index(workflow, "artifact-lifecycle-contract:")
	if jobStart < 0 {
		return
	}
	job := workflow[jobStart:]
	for _, forbidden := range []string{"from_version:", "to_version:", "release_base:"} {
		if strings.Contains(job, forbidden) {
			t.Errorf("ordinary CI supplies live artifact input %q", forbidden)
		}
	}
}

func TestWorkflowStepRunDistinguishesLiveFromContractOnly(t *testing.T) {
	workflow := `steps:
  - name: Validate POSIX lifecycle contract
    run: sh scripts/acceptance/release-lifecycle.sh --contract-only
  - name: Run POSIX artifact lifecycle
    run: sh scripts/acceptance/release-lifecycle.sh --contract-only
  - name: Validate PowerShell lifecycle contract
    run: pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1 -ContractOnly
  - name: Run PowerShell artifact lifecycle
    run: pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1 -ContractOnly
`
	for name, commands := range map[string][2]string{
		"Run POSIX artifact lifecycle": {
			"sh scripts/acceptance/release-lifecycle.sh --contract-only",
			"sh scripts/acceptance/release-lifecycle.sh",
		},
		"Run PowerShell artifact lifecycle": {
			"pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1 -ContractOnly",
			"pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1",
		},
	} {
		command, found := workflowStepRun(workflow, name)
		if !found {
			t.Fatalf("synthetic workflow step %q was not found", name)
		}
		if command != commands[0] {
			t.Errorf("synthetic workflow step %q parsed as %q, want %q", name, command, commands[0])
		}
		if command == commands[1] {
			t.Errorf("contract-only command for %q was mistaken for live command %q", name, commands[1])
		}
	}
}
