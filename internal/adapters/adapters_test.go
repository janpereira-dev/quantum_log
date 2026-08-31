package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestMain(m *testing.M) {
	originalEnvironment := copilotCLIUserEnvironment
	originalEnvironmentChanged := copilotCLIUserEnvironmentChanged
	originalProfileDiscovery := copilotCLIPowerShellProfileDiscovery
	var temporaryProfileDirectory string
	if runtime.GOOS == "windows" {
		copilotCLIUserEnvironment = fakeCopilotCLIUserEnvironment{}
		copilotCLIUserEnvironmentChanged = func() error { return nil }
		var err error
		temporaryProfileDirectory, err = os.MkdirTemp("", "qlog-copilot-profile-")
		if err != nil {
			panic(err)
		}
		copilotCLIPowerShellProfileDiscovery = func() (string, error) {
			return filepath.Join(temporaryProfileDirectory, "Microsoft.PowerShell_profile.ps1"), nil
		}
	}
	exitCode := m.Run()
	if temporaryProfileDirectory != "" {
		_ = os.RemoveAll(temporaryProfileDirectory)
	}
	copilotCLIUserEnvironment = originalEnvironment
	copilotCLIUserEnvironmentChanged = originalEnvironmentChanged
	copilotCLIPowerShellProfileDiscovery = originalProfileDiscovery
	os.Exit(exitCode)
}

func TestDefaultRegistryDeclaresOnlyVerifiedCapabilities(t *testing.T) {
	registry := Default()
	items := registry.List()
	if len(items) != 9 {
		t.Fatalf("List() returned %d adapters, want 9", len(items))
	}
	generic, found := registry.Get("generic-jsonl")
	if !found || !generic.Descriptor().Capabilities.StructuredEvents {
		t.Fatal("generic JSONL adapter must declare structured event support")
	}
	if generic.Descriptor().Capabilities.Costs || generic.Descriptor().Capabilities.InputTokens {
		t.Fatal("generic JSONL must not claim metrics supplied only by callers")
	}
	copilot, found := registry.Get("copilot-vscode")
	if !found || !copilot.Descriptor().Capabilities.ModelIdentity || !copilot.Descriptor().Capabilities.InputTokens || !copilot.Descriptor().Capabilities.OutputTokens || !copilot.Descriptor().Capabilities.ReasoningTokens || !copilot.Descriptor().Capabilities.CacheTokens || !copilot.Descriptor().Capabilities.StructuredEvents {
		t.Fatalf("copilot-vscode must declare documented OTel model and token capabilities")
	}
	copilotCLI, found := registry.Get("copilot")
	if !found || !copilotCLI.Descriptor().Capabilities.SessionLifecycle || !copilotCLI.Descriptor().Capabilities.ProjectIdentity || !copilotCLI.Descriptor().Capabilities.WorkingDirectory || !copilotCLI.Descriptor().Capabilities.ToolCalls || !copilotCLI.Descriptor().Capabilities.StructuredEvents || !copilotCLI.Descriptor().Capabilities.InputTokens || !copilotCLI.Descriptor().Capabilities.OutputTokens {
		t.Fatalf("copilot must declare documented OTel token and hook lifecycle capabilities")
	}
	opencode, found := registry.Get("opencode")
	if !found || !opencode.Descriptor().Capabilities.ModelIdentity || !opencode.Descriptor().Capabilities.InputTokens || !opencode.Descriptor().Capabilities.OutputTokens || !opencode.Descriptor().Capabilities.ReasoningTokens || !opencode.Descriptor().Capabilities.CacheTokens || !opencode.Descriptor().Capabilities.Costs || !opencode.Descriptor().Capabilities.ToolCalls || !opencode.Descriptor().Capabilities.SessionLifecycle || !opencode.Descriptor().Capabilities.StructuredEvents {
		t.Fatalf("opencode must declare audited plugin usage and lifecycle capabilities")
	}
	codex, found := registry.Get("codex")
	if !found || !codex.Descriptor().Capabilities.ModelIdentity || !codex.Descriptor().Capabilities.InputTokens || !codex.Descriptor().Capabilities.OutputTokens || !codex.Descriptor().Capabilities.CacheTokens || !codex.Descriptor().Capabilities.ReasoningTokens || !codex.Descriptor().Capabilities.StructuredEvents {
		t.Fatalf("codex must declare documented OTLP response.completed capabilities")
	}
	claude, found := registry.Get("claude-code")
	if !found || !claude.Descriptor().Capabilities.SessionLifecycle || !claude.Descriptor().Capabilities.StructuredEvents || !claude.Descriptor().Capabilities.InputTokens || !claude.Descriptor().Capabilities.OutputTokens {
		t.Fatalf("claude-code must declare documented OTel token and lifecycle capabilities")
	}
	for _, id := range []string{"pi", "openclaw", "hermes"} {
		adapter, found := registry.Get(id)
		if !found {
			t.Fatalf("missing %s adapter", id)
		}
		if adapter.Descriptor().Capabilities != (Capabilities{}) {
			t.Fatalf("%s minimal adapter claimed unsupported capture capability", id)
		}
	}
}

func TestStableAdaptersContainOnlyM4Contract(t *testing.T) {
	ids := make([]string, 0)
	for _, adapter := range Default().Stable() {
		ids = append(ids, adapter.Descriptor().ID)
		if !adapter.Descriptor().Stable {
			t.Fatalf("stable adapter %q lacks stable descriptor flag", adapter.Descriptor().ID)
		}
	}
	if want := []string{"claude-code", "codex", "copilot", "copilot-vscode", "opencode"}; !slices.Equal(ids, want) {
		t.Fatalf("stable adapter ids = %v, want %v", ids, want)
	}
}

func TestCopilotCLIInstallCreatesIsolatedLifecycleHookConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	home := filepath.Join(t.TempDir(), "qlog home")
	executable := filepath.Join(t.TempDir(), "qlog.exe")
	adapter := newCopilotCLIAdapter()

	result, err := adapter.Install(context.Background(), InstallOptions{Home: home, ExecutablePath: executable})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !result.Changed {
		t.Fatalf("install result = %#v", result)
	}
	contents := string(mustReadFile(t, adapter.hooksPath()))
	for _, want := range []string{"\"version\": 1", "sessionStart", "sessionEnd", "userPromptSubmitted", "hook copilot-cli --event", "\"timeoutSec\":1"} {
		if !strings.Contains(contents, want) {
			t.Fatalf("hooks config missing %q:\n%s", want, contents)
		}
	}
	for _, forbidden := range []string{"agentStop", "errorOccurred", "preToolUse", "postToolUse", "subagentStart", "subagentStop"} {
		if strings.Contains(contents, forbidden) {
			t.Fatalf("hooks config contains high-frequency event %q:\n%s", forbidden, contents)
		}
	}
	for _, forbidden := range []string{"toolArgs", "toolResult", "authorization"} {
		if strings.Contains(contents, forbidden) {
			t.Fatalf("hooks config contains forbidden %q:\n%s", forbidden, contents)
		}
	}
	status, err := adapter.Status(context.Background())
	if err != nil || !status.Installed {
		t.Fatalf("status = %#v, %v", status, err)
	}
	if runtime.GOOS == "windows" && status.CaptureQuality != CaptureOTELReported {
		t.Fatalf("Windows capture quality = %q, want %q", status.CaptureQuality, CaptureOTELReported)
	}
	if runtime.GOOS != "windows" && (status.State != SetupPartial || status.CaptureQuality != CaptureLifecycleOnly) {
		t.Fatalf("POSIX status = %#v, want partial lifecycle-only without a non-interactive launcher", status)
	}
}

func TestCopilotCLIPowerShellProfileDoesNotSpawnCMD(t *testing.T) {
	profile := copilotCLIProfileBlock()
	if strings.Contains(profile, "ComSpec") {
		t.Fatalf("qlog Copilot profile must not invoke cmd.exe:\n%s", profile)
	}
	if !strings.Contains(profile, "QLOG_COLLECTOR_URL") {
		t.Fatalf("qlog Copilot profile must forward hooks to the loopback collector:\n%s", profile)
	}
}

func TestCopilotCLIOTELLauncherUsesOfficialEnvironmentWithoutGlobalMutation(t *testing.T) {
	config := copilotCLIPosixBlock()
	for _, want := range []string{
		"COPILOT_OTEL_ENABLED=true", "COPILOT_OTEL_EXPORTER_TYPE=otlp-http", "OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318", "OTEL_EXPORTER_OTLP_PROTOCOL=http/json", "OTEL_METRICS_EXPORTER=none", "OTEL_LOGS_EXPORTER=none", "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false", "command copilot",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("Copilot CLI OTel config missing %q:\n%s", want, config)
		}
	}
}

func TestCopilotCLIPowerShellLauncherSelectsOneApplication(t *testing.T) {
	config := copilotCLIProfileBlock()
	for _, want := range []string{
		"@(Get-Command copilot -CommandType Application -ErrorAction Stop)[0].Path",
		"& $qlogCopilotExecutable @args",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("Copilot CLI PowerShell launcher missing %q:\n%s", want, config)
		}
	}
	for _, forbidden := range []string{
		"(Get-Command copilot -CommandType Application -ErrorAction Stop).Source",
		`Microsoft\WinGet\Links\copilot.exe`,
	} {
		if strings.Contains(config, forbidden) {
			t.Fatalf("Copilot CLI PowerShell launcher contains non-portable or ambiguous resolution %q:\n%s", forbidden, config)
		}
	}
}

func TestCopilotCLIPowerShellLauncherInvokesOnlyFirstApplication(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows PowerShell")
	}
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	first := filepath.Join(dir, "copilot-first.cmd")
	second := filepath.Join(dir, "copilot-second.cmd")
	if err := os.WriteFile(first, []byte("@echo off\r\necho first:%*\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("@echo off\r\necho second:%*\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `function global:Get-Command {
  [CmdletBinding()]
  param(
    [Parameter(Position=0)][string[]]$Name,
    [System.Management.Automation.CommandTypes]$CommandType
  )
  if ($Name -eq 'copilot' -and $CommandType -eq [System.Management.Automation.CommandTypes]::Function) { return }
  if ($Name -eq 'copilot' -and $CommandType -eq [System.Management.Automation.CommandTypes]::Application) {
    [pscustomobject]@{ Path = $env:QLOG_TEST_FIRST_COPILOT; Source = $env:QLOG_TEST_FIRST_COPILOT }
    [pscustomobject]@{ Path = $env:QLOG_TEST_SECOND_COPILOT; Source = $env:QLOG_TEST_SECOND_COPILOT }
    return
  }
  Microsoft.PowerShell.Core\Get-Command @PSBoundParameters
}
` + copilotCLIProfileBlock() + "copilot marker\n"
	scriptPath := filepath.Join(dir, "profile-test.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-File", scriptPath)
	command.Env = append(os.Environ(), "QLOG_TEST_FIRST_COPILOT="+first, "QLOG_TEST_SECOND_COPILOT="+second)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("PowerShell launcher failed: %v\n%s", err, output)
	}
	got := string(output)
	if !strings.Contains(got, "first:marker") || strings.Contains(got, "second:marker") {
		t.Fatalf("PowerShell launcher output = %q, want only first application", got)
	}
}

func TestCopilotCLIInstallConfiguresDiscoveredOneDrivePowerShellProfile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PowerShell profile behavior")
	}
	configHome := t.TempDir()
	redirectedProfile := filepath.Join(t.TempDir(), "OneDrive", "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	if err := os.MkdirAll(filepath.Dir(redirectedProfile), 0o700); err != nil {
		t.Fatalf("create redirected profile directory: %v", err)
	}
	if err := os.WriteFile(redirectedProfile, []byte("$env:USER_SETTING = 'preserve'\n"), 0o600); err != nil {
		t.Fatalf("write redirected profile: %v", err)
	}
	originalDiscovery := copilotCLIPowerShellProfileDiscovery
	copilotCLIPowerShellProfileDiscovery = func() (string, error) { return redirectedProfile, nil }
	t.Cleanup(func() { copilotCLIPowerShellProfileDiscovery = originalDiscovery })
	environment := fakeCopilotCLIUserEnvironment{}
	originalEnvironment := copilotCLIUserEnvironment
	copilotCLIUserEnvironment = environment
	t.Cleanup(func() { copilotCLIUserEnvironment = originalEnvironment })
	adapter := newCopilotCLIAdapter()
	first, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	second, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("repeat install: %v", err)
	}
	if !first.Changed || second.Changed {
		t.Fatalf("install idempotence = %#v, %#v", first, second)
	}
	if _, err := os.Stat(filepath.Join(configHome, "Microsoft.PowerShell_profile.ps1")); !os.IsNotExist(err) {
		t.Fatalf("install mutated assumed PowerShell profile: %v", err)
	}
	profile := string(mustReadFile(t, redirectedProfile))
	if !strings.Contains(profile, "$env:USER_SETTING = 'preserve'") {
		t.Fatalf("install did not preserve profile contents:\n%s", profile)
	}
	if !strings.Contains(profile, copilotCLIProfileBlockStart) || !strings.Contains(profile, "function global:copilot") {
		t.Fatalf("install did not configure discovered profile:\n%s", profile)
	}
	if !adapter.windowsPowerShellProfileInstalled() {
		t.Fatal("normal PowerShell profile state is not installed")
	}
	if _, err := adapter.Uninstall(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	profile = string(mustReadFile(t, redirectedProfile))
	if strings.Contains(profile, copilotCLIProfileBlockStart) || !strings.Contains(profile, "$env:USER_SETTING = 'preserve'") {
		t.Fatalf("uninstall did not remove only qlog profile block:\n%s", profile)
	}
}

func TestCopilotCLIInstallCreatesDiscoveredPowerShellProfileParent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PowerShell profile behavior")
	}
	configHome := t.TempDir()
	redirectedProfile := filepath.Join(t.TempDir(), "OneDrive", "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	originalDiscovery := copilotCLIPowerShellProfileDiscovery
	copilotCLIPowerShellProfileDiscovery = func() (string, error) { return redirectedProfile, nil }
	t.Cleanup(func() { copilotCLIPowerShellProfileDiscovery = originalDiscovery })

	if _, err := newCopilotCLIAdapter().Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install with missing discovered profile parent: %v", err)
	}
	if _, err := os.Stat(redirectedProfile); err != nil {
		t.Fatalf("created discovered profile: %v", err)
	}
}

func TestCopilotCLIInstallFallsBackToPowerShellForMissingProfileWrite(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PowerShell profile behavior")
	}
	configHome := t.TempDir()
	profile := filepath.Join(t.TempDir(), "OneDrive", "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	if err := os.MkdirAll(filepath.Dir(profile), 0o700); err != nil {
		t.Fatalf("create profile parent: %v", err)
	}

	originalDiscovery := copilotCLIPowerShellProfileDiscovery
	copilotCLIPowerShellProfileDiscovery = func() (string, error) { return profile, nil }
	t.Cleanup(func() { copilotCLIPowerShellProfileDiscovery = originalDiscovery })
	originalWrite := copilotCLIPowerShellProfileWriteFile
	copilotCLIPowerShellProfileWriteFile = func(string, []byte, os.FileMode) error {
		return &os.PathError{Op: "open", Path: profile, Err: syscall.ENOENT}
	}
	t.Cleanup(func() { copilotCLIPowerShellProfileWriteFile = originalWrite })
	originalCommand := copilotCLIPowerShellProfileWriteCommand
	copilotCLIPowerShellProfileWriteCommand = func(name string, args ...string) (string, error) {
		if name != "powershell.exe" {
			t.Fatalf("command = %q, want powershell.exe", name)
		}
		if len(args) != 7 || args[0] != "-NoProfile" || args[1] != "-NonInteractive" || args[2] != "-Command" || args[4] != profile || args[6] != "" {
			t.Fatalf("command args = %#v", args)
		}
		if args[3] != copilotCLIPowerShellProfileWriteScript || strings.Contains(args[3], "-Force") || strings.Contains(args[3], "ToHexString") || !strings.Contains(args[3], "New-Item -ItemType File -Path $target") || !strings.Contains(args[3], "[IO.File]::WriteAllBytes($target, $bytes)") || !strings.Contains(args[3], "[BitConverter]::ToString") {
			t.Fatalf("unsafe PowerShell write script: %q", args[3])
		}
		payload, err := base64.StdEncoding.DecodeString(args[5])
		if err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if err := os.WriteFile(profile, payload, 0o600); err != nil {
			t.Fatalf("simulate PowerShell write: %v", err)
		}
		return "", nil
	}
	t.Cleanup(func() { copilotCLIPowerShellProfileWriteCommand = originalCommand })

	result, err := newCopilotCLIAdapter().Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !result.Changed {
		t.Fatalf("install result = %#v", result)
	}
	profileContents := string(mustReadFile(t, profile))
	if !strings.Contains(profileContents, copilotCLIProfileBlockStart) {
		t.Fatalf("profile missing qlog block:\n%s", profileContents)
	}
}

func TestCopilotCLIProfileWriteFallbackPreservesExistingProfileForUninstall(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only PowerShell profile behavior")
	}
	profile := filepath.Join(t.TempDir(), "OneDrive", "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(profile), 0o700); err != nil {
		t.Fatalf("create profile parent: %v", err)
	}
	const userContents = "$env:USER_SETTING = 'preserve'\n"
	if err := os.WriteFile(profile, []byte(userContents), 0o600); err != nil {
		t.Fatalf("write user profile: %v", err)
	}
	originalDiscovery := copilotCLIPowerShellProfileDiscovery
	copilotCLIPowerShellProfileDiscovery = func() (string, error) { return profile, nil }
	t.Cleanup(func() { copilotCLIPowerShellProfileDiscovery = originalDiscovery })
	originalWrite := copilotCLIPowerShellProfileWriteFile
	copilotCLIPowerShellProfileWriteFile = func(string, []byte, os.FileMode) error {
		return &os.PathError{Op: "open", Path: profile, Err: syscall.ENOENT}
	}
	t.Cleanup(func() { copilotCLIPowerShellProfileWriteFile = originalWrite })
	originalCommand := copilotCLIPowerShellProfileWriteCommand
	copilotCLIPowerShellProfileWriteCommand = func(_ string, args ...string) (string, error) {
		if len(args) != 7 {
			t.Fatalf("command args = %#v", args)
		}
		target := args[4]
		if target != profile && !strings.HasPrefix(target, profile+".qlog-backup-") {
			t.Fatalf("unexpected write target %q", target)
		}
		if target == profile {
			before := mustReadFile(t, profile)
			hash := sha256.Sum256(before)
			if args[6] != fmt.Sprintf("%x", hash) {
				t.Fatalf("expected profile hash = %q, want %x", args[6], hash)
			}
		} else if args[6] != "" {
			t.Fatalf("backup expected empty hash, got %q", args[6])
		}
		payload, err := base64.StdEncoding.DecodeString(args[5])
		if err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if err := os.WriteFile(target, payload, 0o600); err != nil {
			t.Fatalf("simulate PowerShell write: %v", err)
		}
		return "", nil
	}
	t.Cleanup(func() { copilotCLIPowerShellProfileWriteCommand = originalCommand })

	adapter := newCopilotCLIAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := adapter.Uninstall(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if got := string(mustReadFile(t, profile)); got != userContents {
		t.Fatalf("profile after uninstall = %q, want preserved user contents", got)
	}
}

func TestCopilotCLIPowerShellProfileWriterAddsUTF8BOMButKeepsBackupsRaw(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "Microsoft.PowerShell_profile.ps1")
	backup := profile + ".qlog-backup-test"
	if err := writeCopilotCLIPowerShellProfile(profile, []byte{0xc3, 0xa9}, 0o600, nil, false); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	contents := mustReadFile(t, profile)
	if !hasUTF8BOM(contents) || !slices.Equal(contents[3:], []byte{0xc3, 0xa9}) {
		t.Fatalf("profile bytes = %x", contents)
	}
	raw := []byte{0xff, 0x00, 0x81}
	if err := writeCopilotCLIPowerShellProfile(backup, raw, 0o600, nil, false); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if contents := mustReadFile(t, backup); !slices.Equal(contents, raw) {
		t.Fatalf("backup bytes = %x, want %x", contents, raw)
	}
}

func TestCopilotCLIPowerShellProfileDiscoveryUsesFixedNoProfileCommand(t *testing.T) {
	originalRun := copilotCLIPowerShellProfileCommand
	var gotName string
	var gotArgs []string
	copilotCLIPowerShellProfileCommand = func(name string, args ...string) (string, error) {
		gotName, gotArgs = name, args
		return `C:\Users\alice\OneDrive\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1`, nil
	}
	t.Cleanup(func() { copilotCLIPowerShellProfileCommand = originalRun })

	profile, err := discoverCopilotCLIPowerShellProfile()
	if err != nil {
		t.Fatalf("discover profile: %v", err)
	}
	if profile != `C:\Users\alice\OneDrive\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1` {
		t.Fatalf("profile = %q", profile)
	}
	if gotName != "powershell.exe" || !slices.Equal(gotArgs, []string{"-NoProfile", "-NonInteractive", "-Command", "$PROFILE.CurrentUserCurrentHost"}) {
		t.Fatalf("discovery command = %q %q", gotName, gotArgs)
	}
}

type fakeCopilotCLIUserEnvironment map[string]string

func (f fakeCopilotCLIUserEnvironment) Get(name string) (string, bool, error) {
	value, found := f[name]
	return value, found, nil
}

func (f fakeCopilotCLIUserEnvironment) Set(name, value string) error {
	f[name] = value
	return nil
}

func (f fakeCopilotCLIUserEnvironment) Delete(name string) error {
	delete(f, name)
	return nil
}

func TestCopilotCLIHooksExposePowerShellGenericCommand(t *testing.T) {
	config := struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}{}
	if err := json.Unmarshal([]byte(copilotCLIHooksConfig(`C:\Users\alice\AppData\Local\QUANTUM_LOG`, `C:\Program Files\QUANTUM_LOG\qlog.exe`)), &config); err != nil {
		t.Fatal(err)
	}
	for event, entries := range config.Hooks {
		if len(entries) != 1 {
			t.Fatalf("%s entries = %#v", event, entries)
		}
		command, _ := entries[0]["command"].(string)
		for _, want := range []string{"powershell.exe -NoProfile -NonInteractive -Command", "$input |", "hook copilot-cli --event " + event} {
			if !strings.Contains(command, want) {
				t.Fatalf("%s generic command missing %q: %s", event, want, command)
			}
		}
	}
}

func TestCopilotCLIPowerShellHookScriptParsesOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows PowerShell")
	}
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Fatal(err)
	}
	script := copilotCLIPowerShellHookCommand(`C:\Users\alice\AppData\Local\QUANTUM_LOG`, `C:\Program Files\QUANTUM_LOG\qlog.exe`) + " --event sessionStart"
	command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", "[scriptblock]::Create($env:QLOG_TEST_POWERSHELL_HOOK) | Out-Null")
	command.Env = append(os.Environ(), "QLOG_TEST_POWERSHELL_HOOK="+script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell parser failed: %v\n%s", err, output)
	}
}

func TestCopilotCLIHooksPreferCOPILOT_HOMEAfterTestOverride(t *testing.T) {
	override := t.TempDir()
	copilotHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", override)
	t.Setenv("COPILOT_HOME", copilotHome)
	adapter := newCopilotCLIAdapter()
	if got, want := adapter.hooksPath(), filepath.Join(override, ".copilot", "hooks", "qlog.json"); got != want {
		t.Fatalf("test override hooks path = %q, want %q", got, want)
	}
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", "")
	if got, want := adapter.hooksPath(), filepath.Join(copilotHome, "hooks", "qlog.json"); got != want {
		t.Fatalf("COPILOT_HOME hooks path = %q, want %q", got, want)
	}
}

func TestCopilotCLIUninstallRemovesOnlyQlogOwnedHookConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newCopilotCLIAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	userHook := filepath.Join(configHome, ".copilot", "hooks", "user.json")
	if err := os.WriteFile(userHook, []byte(`{"version":1,"hooks":{}}`), 0o600); err != nil {
		t.Fatalf("write user hook: %v", err)
	}
	result, err := adapter.Uninstall(context.Background(), InstallOptions{})
	if err != nil || !result.Changed {
		t.Fatalf("uninstall = %#v, %v", result, err)
	}
	if _, err := os.Stat(adapter.hooksPath()); !os.IsNotExist(err) {
		t.Fatalf("qlog hook config remains: %v", err)
	}
	if _, err := os.Stat(userHook); err != nil {
		t.Fatalf("user hook removed: %v", err)
	}
}

func TestClaudeCodeStatusDefaultsToLifecycleOnly(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	status, err := newClaudeCodeAdapter().Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.CaptureQuality != CaptureLifecycleOnly {
		t.Fatalf("quality = %q", status.CaptureQuality)
	}
	if status.InstallationState == "" {
		t.Fatal("installation state is required")
	}
	if status.CollectorReachable || status.RecentEvidence {
		t.Fatalf("unverified status = %#v", status)
	}
}

func TestCodexAndCopilotReportTheirDocumentedQuality(t *testing.T) {
	for _, want := range []struct {
		adapter Adapter
		quality CaptureQuality
	}{
		{newCodexAdapter(), CaptureOTELReported},
		{newVSCodeCopilotAdapter(), CaptureOTELReported},
	} {
		status, err := want.adapter.Status(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if status.CaptureQuality != want.quality {
			t.Fatalf("%s quality = %q, want %q", status.AdapterID, status.CaptureQuality, want.quality)
		}
	}
}

func TestCodexInstallReportsCreatedConfigOnFreshHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newCodexAdapter()

	result, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || len(result.Changes) != 1 || result.Changes[0].Action != "created" {
		t.Fatalf("fresh Codex install = %#v", result)
	}
}

func TestOpenCodeStatusIsAgentReported(t *testing.T) {
	status, err := newOpenCodeAdapter().Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.CaptureQuality != CaptureAgentReported {
		t.Fatalf("quality = %q", status.CaptureQuality)
	}
}

func TestCopilotVSCodeInstallConfiguresNativeOTelWithoutContentCapture(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter, found := Default().Get("copilot-vscode")
	if !found {
		t.Fatal("missing copilot-vscode adapter")
	}
	result, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Install() changed = false, actions = %#v", result.Actions)
	}
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	contents, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(contents, &settings); err != nil {
		t.Fatalf("settings JSON invalid: %v\n%s", err, contents)
	}
	assertSetting(t, settings, "github.copilot.chat.otel.enabled", true)
	assertSetting(t, settings, "github.copilot.chat.otel.exporterType", "otlp-http")
	assertSetting(t, settings, "github.copilot.chat.otel.otlpEndpoint", "http://127.0.0.1:4318")
	assertSetting(t, settings, "github.copilot.chat.otel.captureContent", false)

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Installed || status.CaptureQuality != CaptureOTELReported {
		t.Fatalf("status = %#v", status)
	}
}

func TestVSCodeCopilotEqualInstallIsByteIdenticalAndReportsExactDrift(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newVSCodeCopilotAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	first := mustReadFile(t, settingsPath)
	second, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil || second.Changed || len(second.Changes) != 1 || second.Changes[0].Action != "unchanged" {
		t.Fatalf("equal install = %#v, %v", second, err)
	}
	if after := mustReadFile(t, settingsPath); string(after) != string(first) {
		t.Fatalf("equal install changed settings:\n%s\nwant:\n%s", after, first)
	}

	settings := readSettingsMap(t, settingsPath)
	settings["github.copilot.chat.otel.captureContent"] = true
	writeSettingsMap(t, settingsPath, settings)
	drifted, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil || !drifted.Changed || len(drifted.Changes) != 1 {
		t.Fatalf("drifted install = %#v, %v", drifted, err)
	}
	if got := drifted.Changes[0].Description; got != "qlog managed settings drifted: github.copilot.chat.otel.captureContent" {
		t.Fatalf("drift description = %q", got)
	}
}

func TestVSCodeCopilotInstallHandlesJSONCAndPreservesSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	before := `{
  // keep this user setting
  "editor.fontSize": 14,
  "editor.snippetSuggestions": "inline, }",
}
`
	if err := os.WriteFile(settingsPath, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}

	adapter := newVSCodeCopilotAdapter()
	result, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Changes[0].BackupPath == "" {
		t.Fatalf("install result = %#v", result)
	}
	contents, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "// keep this user setting") || !strings.Contains(text, "editor.fontSize") || !strings.Contains(text, "inline, }") || !strings.Contains(text, "github.copilot.chat.otel.enabled") || strings.Contains(text, "github.copilot.chat.otel.captureContent\": true") {
		t.Fatalf("settings after install = %s", text)
	}
}

func TestVSCodeCopilotUninstallRestoresPreexistingManagedSetting(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	before := `{
  // user-owned Copilot setting
  "github.copilot.chat.otel.enabled": false
}
`
	if err := os.WriteFile(settingsPath, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}

	adapter := newVSCodeCopilotAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Uninstall(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	after := string(mustReadFile(t, settingsPath))
	if !strings.Contains(after, `"github.copilot.chat.otel.enabled": false`) {
		t.Fatalf("preexisting setting not restored: %s", after)
	}
	if strings.Contains(after, qlogVSCodeManagedKey) || strings.Contains(after, "github.copilot.chat.otel.exporterType") {
		t.Fatalf("qlog-managed settings remained: %s", after)
	}
}

func TestVSCodeCopilotUninstallRemovesOnlyManagedSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newVSCodeCopilotAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	settings := readSettingsMap(t, settingsPath)
	settings["editor.fontSize"] = float64(14)
	settings["github.copilot.chat.otel.outfile"] = "C:/tmp/copilot.jsonl"
	writeSettingsMap(t, settingsPath, settings)

	result, err := adapter.Uninstall(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatalf("uninstall result = %#v", result)
	}
	after := readSettingsMap(t, settingsPath)
	if _, found := after["github.copilot.chat.otel.enabled"]; found {
		t.Fatalf("managed setting remained: %#v", after)
	}
	if after["editor.fontSize"] != float64(14) || after["github.copilot.chat.otel.outfile"] != "C:/tmp/copilot.jsonl" {
		t.Fatalf("unrelated settings not preserved: %#v", after)
	}
}

func TestCodexInstallReplacesEquivalentExporterForms(t *testing.T) {
	const prefix = "model = \"gpt-5\"\n\n"
	const suffix = "[features]\nexperimental = true\n"
	desiredExporter := `exporter = { otlp-http = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "binary" } }`
	tests := []struct {
		name          string
		otel          string
		wantChanged   bool
		wantStateFile bool
	}{
		{name: "inline exporter", otel: "[otel]\nenvironment = \"production\"\nexporter = \"none\"\n\n", wantChanged: true, wantStateFile: true},
		{name: "dotted exporter keys", otel: "[otel]\nenvironment = \"production\"\nexporter.otlp-http.endpoint = \"http://old\"\n\n", wantChanged: true, wantStateFile: true},
		{name: "nested exporter table", otel: "[otel.exporter.otlp-http]\nendpoint = \"http://old\"\n\n", wantChanged: true, wantStateFile: true},
		{name: "matching user-owned exporter", otel: "[otel]\n" + desiredExporter + "\nlog_user_prompt = false\n\n", wantChanged: false, wantStateFile: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configHome := t.TempDir()
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
			configPath := filepath.Join(configHome, ".codex", "config.toml")
			if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
				t.Fatal(err)
			}
			before := prefix + test.otel + suffix
			if err := os.WriteFile(configPath, []byte(before), 0o600); err != nil {
				t.Fatal(err)
			}

			adapter := newCodexAdapter()
			first, err := adapter.Install(context.Background(), InstallOptions{})
			if err != nil {
				t.Fatalf("install: %v", err)
			}
			if first.Changed != test.wantChanged {
				t.Fatalf("first install changed = %t, want %t: %#v", first.Changed, test.wantChanged, first)
			}
			installed := string(mustReadFile(t, configPath))
			if !strings.HasPrefix(installed, prefix) || !strings.HasSuffix(installed, suffix) {
				t.Fatalf("non-otel text changed:\n%s", installed)
			}
			var document map[string]any
			if err := toml.Unmarshal([]byte(installed), &document); err != nil {
				t.Fatalf("parse installed TOML: %v\n%s", err, installed)
			}
			otel, ok := document["otel"].(map[string]any)
			if !ok {
				t.Fatalf("otel = %#v", document["otel"])
			}
			exporter, ok := otel["exporter"].(map[string]any)
			if !ok || len(exporter) != 1 {
				t.Fatalf("exporter = %#v", otel["exporter"])
			}
			otlpHTTP, ok := exporter["otlp-http"].(map[string]any)
			if !ok || otlpHTTP["endpoint"] != "http://127.0.0.1:4318/v1/logs" || otlpHTTP["protocol"] != "binary" {
				t.Fatalf("otlp-http = %#v", exporter["otlp-http"])
			}

			_, stateErr := os.Stat(adapter.statePath())
			if (stateErr == nil) != test.wantStateFile {
				t.Fatalf("state exists = %t, want %t: %v", stateErr == nil, test.wantStateFile, stateErr)
			}
			second, err := adapter.Install(context.Background(), InstallOptions{})
			if err != nil || second.Changed {
				t.Fatalf("second install = %#v, %v", second, err)
			}
			uninstall, err := adapter.Uninstall(context.Background(), InstallOptions{})
			if err != nil {
				t.Fatalf("uninstall: %v", err)
			}
			if uninstall.Changed != test.wantChanged {
				t.Fatalf("uninstall changed = %t, want %t: %#v", uninstall.Changed, test.wantChanged, uninstall)
			}
			if after := string(mustReadFile(t, configPath)); after != before {
				t.Fatalf("config after uninstall = %q, want %q", after, before)
			}
		})
	}
}

func TestOpenCodeInstallWritesGlobalPluginPostingLocalEvents(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter, found := Default().Get("opencode")
	if !found {
		t.Fatal("missing opencode adapter")
	}
	result, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Install() changed = false, actions = %#v", result.Actions)
	}
	pluginPath := filepath.Join(configHome, ".config", "opencode", "plugins", "quantum-log.ts")
	contents, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	text := string(contents)
	for _, want := range []string{"/v1/events", "session.created", "message.updated", "message.part.updated", "tool.execute.before", "tool.execute.after", "properties.info", "properties.part", "step-finish", "capture_quality", "input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "cache_write_tokens", "tool_name", "callID", "activeInteractions.get(toolSession(input))", "toolInteractions.get(callID)", "prompt_available: false", "prompt_source: \"not_emitted\"", "event?.sessionID"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plugin missing %q:\n%s", want, text)
		}
	}
	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Installed || status.CaptureQuality != CaptureAgentReported {
		t.Fatalf("status = %#v", status)
	}
}

func TestOpenCodePluginUsesAuditedUsageFieldsWithoutRawContent(t *testing.T) {
	source := openCodePluginSource()
	for _, want := range []string{"properties.sessionID", "properties.info", "properties.part", "info.sessionID", "part.sessionID", "context.directory", "info.providerID", "info.modelID", "info.cost", "tokens.input", "tokens.output", "tokens.reasoning", "cache.read", "cache.write", "info.time.created", "info.time.completed", "info.finish", "capture_quality: \"agent_reported\""} {
		if !strings.Contains(source, want) {
			t.Fatalf("plugin missing %q:\n%s", want, source)
		}
	}
	for _, forbidden := range []string{"payload: info", "payload: part", "...info", "...part", `setString(payload, "response"`, `setString(payload, "tool_args"`, `setString(payload, "tool_results"`, `setString(payload, "authorization"`, `setString(payload, "secret"`, "total_tokens"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("plugin forwards forbidden raw content %q:\n%s", forbidden, source)
		}
	}
}

func TestOpenCodePluginUsesPluginWorkspaceAndEmitsFinalMetricObservations(t *testing.T) {
	source := openCodePluginSource()
	for _, want := range []string{
		"ctx.directory || ctx.worktree || context.directory",
		"metric_observations",
		"raw_key: rawKey",
		"source: \"opencode\"",
		"confidence: \"reported\"",
		"if (numberValue(info.time && info.time.completed) === undefined) return",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("plugin missing %q:\n%s", want, source)
		}
	}
	for _, eventType := range []string{"session.created", "session.idle", "session.error"} {
		if !strings.Contains(source, eventType) {
			t.Fatalf("plugin dropped allowlisted lifecycle type %q:\n%s", eventType, source)
		}
	}
}

func TestClaudeCodeInstallPreservesExistingHooksAndAddsHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{
		"hooks": map[string]any{
			"Stop":         []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "user-stop-hook"}}}},
			"SessionStart": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "qlog hook claude-code"}}}},
		},
	}
	writeSettingsMap(t, settingsPath, existing)
	adapter, found := Default().Get("claude-code")
	if !found {
		t.Fatal("claude-code adapter missing")
	}
	qlogHome := filepath.Join(t.TempDir(), "qlog-home")
	absHome, err := filepath.Abs(qlogHome)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Install(context.Background(), InstallOptions{Home: absHome})
	if err != nil {
		t.Fatalf("install claude-code: %v", err)
	}
	if !result.Changed {
		t.Fatalf("install result = %#v", result)
	}
	settings := readSettingsMap(t, settingsPath)
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("settings hooks missing: %#v", settings)
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SubagentStop"} {
		if _, ok := hooks[event]; !ok {
			t.Fatalf("settings missing hook event %q: %#v", event, hooks)
		}
	}
	commands := collectHookCommands(settings)
	wantCommand := "qlog --home " + strconv.Quote(absHome) + " hook claude-code"
	for _, want := range []string{"user-stop-hook", wantCommand} {
		if !containsAdapterString(commands, want) {
			t.Fatalf("settings commands missing %q: %#v", want, commands)
		}
	}
	if containsAdapterString(commands, "qlog hook claude-code") {
		t.Fatalf("old qlog hook command was not updated: %#v", commands)
	}
}

func TestClaudeCodeInstallUsesShellSafeExecutablePath(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, ".claude", "settings.json")
	home := filepath.Join(t.TempDir(), "qlog home")
	executable := filepath.Join(t.TempDir(), "qlog $stable")

	if _, err := newClaudeCodeAdapter().Install(context.Background(), InstallOptions{Home: home, ExecutablePath: executable}); err != nil {
		t.Fatalf("install claude-code: %v", err)
	}

	commands := collectHookCommands(readSettingsMap(t, settingsPath))
	want := "'" + executable + "' --home '" + home + "' hook claude-code"
	if !containsAdapterString(commands, want) {
		t.Fatalf("hook commands missing %q: %#v", want, commands)
	}
	if containsAdapterString(commands, "qlog hook claude-code") {
		t.Fatalf("hook command must not require qlog on PATH: %#v", commands)
	}
}

func TestClaudeCodeInstallConfiguresTraceOnlyOTelWithoutContentCapture(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	if _, err := newClaudeCodeAdapter().Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install Claude Code: %v", err)
	}
	settings := readSettingsMap(t, filepath.Join(configHome, ".claude", "settings.json"))
	env, ok := settings["env"].(map[string]any)
	if !ok {
		t.Fatalf("settings env = %#v", settings["env"])
	}
	for key, want := range map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":                       "1",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA":                "1",
		"OTEL_TRACES_EXPORTER":                               "otlp",
		"OTEL_METRICS_EXPORTER":                              "none",
		"OTEL_LOGS_EXPORTER":                                 "none",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":                 "http://127.0.0.1:4318/v1/traces",
		"OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT": "false",
	} {
		if env[key] != want {
			t.Fatalf("env[%q] = %#v, want %q", key, env[key], want)
		}
	}
}

func TestClaudeCodeStatusRequiresExactManagedOTELEnvironment(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSettingsMap(t, settingsPath, map[string]any{"env": map[string]any{"CLAUDE_CODE_ENABLE_TELEMETRY": "1", "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT": "true"}})
	status, err := newClaudeCodeAdapter().Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.CaptureQuality != CaptureLifecycleOnly {
		t.Fatalf("status accepted partial or unsafe OTel configuration: %#v", status)
	}
}

func TestClaudeCodeUninstallRemovesOnlyQlogHooks(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"Stop": []any{map[string]any{"hooks": []any{
				map[string]any{"type": "command", "command": "user-stop-hook"},
				map[string]any{"type": "command", "command": "user-qlog hook claude-code-notify"},
				map[string]any{"type": "command", "command": "qlog hook claude-code"},
			}}},
			"SessionStart": []any{map[string]any{"hooks": []any{
				map[string]any{"type": "command", "command": "qlog --home \"C:/qlog\" hook claude-code"},
			}}},
		},
	}
	writeSettingsMap(t, settingsPath, settings)

	result, err := newClaudeCodeAdapter().Uninstall(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("uninstall result = %#v", result)
	}
	after := readSettingsMap(t, settingsPath)
	if after["theme"] != "dark" {
		t.Fatalf("user setting not preserved: %#v", after)
	}
	commands := collectHookCommands(after)
	if !containsAdapterString(commands, "user-stop-hook") || !containsAdapterString(commands, "user-qlog hook claude-code-notify") || containsAdapterString(commands, "qlog hook claude-code") || containsAdapterString(commands, "qlog --home \"C:/qlog\" hook claude-code") {
		t.Fatalf("hooks after uninstall = %#v", commands)
	}
}

func TestClaudeCodeUninstallRemovesOnlyQlogOwnedOTELEnvironment(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{"env": map[string]any{
		"CLAUDE_CODE_ENABLE_TELEMETRY":                       "1",
		"OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT": "false",
		"OTEL_RESOURCE_ATTRIBUTES":                           "service.name=user-owned",
		"USER_SETTING":                                       "preserve",
	}}
	writeSettingsMap(t, settingsPath, settings)

	result, err := newClaudeCodeAdapter().Uninstall(context.Background(), InstallOptions{})
	if err != nil || !result.Changed {
		t.Fatalf("uninstall = %#v, %v", result, err)
	}
	after := readSettingsMap(t, settingsPath)
	env := after["env"].(map[string]any)
	if _, found := env["CLAUDE_CODE_ENABLE_TELEMETRY"]; found {
		t.Fatalf("qlog telemetry setting remains: %#v", env)
	}
	if _, found := env["OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT"]; found {
		t.Fatalf("qlog content setting remains: %#v", env)
	}
	if env["OTEL_RESOURCE_ATTRIBUTES"] != "service.name=user-owned" || env["USER_SETTING"] != "preserve" {
		t.Fatalf("unrelated environment changed: %#v", env)
	}
}

func TestCopilotCLIUninstallRemovesLauncherWhenHookIsMissingAndHonorsDryRun(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newCopilotCLIAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := os.Remove(adapter.hooksPath()); err != nil {
		t.Fatalf("remove hook: %v", err)
	}

	dryRun, err := adapter.Uninstall(context.Background(), InstallOptions{DryRun: true})
	if err != nil || dryRun.Changed {
		t.Fatalf("dry-run uninstall = %#v, %v", dryRun, err)
	}
	launcherInstalled := adapter.posixProfileInstalled()
	if runtime.GOOS == "windows" {
		launcherInstalled = adapter.windowsPowerShellProfileInstalled()
	}
	if !launcherInstalled {
		t.Fatal("dry run removed Copilot launcher")
	}

	result, err := adapter.Uninstall(context.Background(), InstallOptions{})
	if err != nil || !result.Changed {
		t.Fatalf("uninstall = %#v, %v", result, err)
	}
	launcherInstalled = adapter.posixProfileInstalled()
	if runtime.GOOS == "windows" {
		launcherInstalled = adapter.windowsPowerShellProfileInstalled()
	}
	if launcherInstalled {
		t.Fatal("qlog Copilot launcher remains")
	}
}

func collectHookCommands(value any) []string {
	commands := []string{}
	appendHookCommands(value, &commands)
	return commands
}

func appendHookCommands(value any, commands *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if hookType, _ := typed["type"].(string); hookType == "command" {
			if command, _ := typed["command"].(string); command != "" {
				*commands = append(*commands, command)
			}
		}
		for _, child := range typed {
			appendHookCommands(child, commands)
		}
	case []any:
		for _, child := range typed {
			appendHookCommands(child, commands)
		}
	}
}

func containsAdapterString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func assertSetting(t *testing.T, settings map[string]any, key string, want any) {
	t.Helper()
	if got := settings[key]; got != want {
		t.Fatalf("%s = %#v, want %#v", key, got, want)
	}
}

func readSettingsMap(t *testing.T, path string) map[string]any {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal(contents, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func writeSettingsMap(t *testing.T, path string, settings map[string]any) {
	t.Helper()
	contents, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestDefaultRegistryAdaptersExposeSetupLifecycle(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	for _, adapter := range Default().List() {
		status, err := adapter.Status(context.Background())
		if err != nil {
			t.Fatalf("%s status: %v", adapter.Descriptor().ID, err)
		}
		if status.AdapterID != adapter.Descriptor().ID || status.State == "" || status.CaptureQuality == "" {
			t.Fatalf("%s status = %#v", adapter.Descriptor().ID, status)
		}
		result, err := adapter.Test(context.Background())
		if err != nil {
			t.Fatalf("%s test: %v", adapter.Descriptor().ID, err)
		}
		if result.AdapterID != adapter.Descriptor().ID || result.CaptureQuality == "" {
			t.Fatalf("%s test = %#v", adapter.Descriptor().ID, result)
		}
	}
}

func TestMinimalAdapterDryRunIsIdempotentAndDoesNotWrite(t *testing.T) {
	adapter, _ := Default().Get("claude-code")
	first, err := adapter.Install(context.Background(), InstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("first dry run: %v", err)
	}
	second, err := adapter.Install(context.Background(), InstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("second dry run: %v", err)
	}
	if first.Changed || second.Changed || len(first.Actions) != 1 || first.Actions[0] != second.Actions[0] || !strings.Contains(first.Actions[0], "dry run") {
		t.Fatalf("dry-run install = %#v then %#v", first, second)
	}
}

func TestApplyMarkerBlockCreatesUpdatesBacksUpAndStaysIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent", "config.md")
	change, err := ApplyMarkerBlock(path, "agent-auto-capture", "first", false)
	if err != nil {
		t.Fatalf("create marker block: %v", err)
	}
	if change.Action != "created" || change.BackupPath != "" {
		t.Fatalf("create change = %#v", change)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !strings.Contains(string(contents), "<!-- qlog:begin agent-auto-capture -->") || !strings.Contains(string(contents), "first") {
		t.Fatalf("created marker content = %q", contents)
	}
	if !HasMarkerBlock(path, "agent-auto-capture") {
		t.Fatal("marker block not detected")
	}

	change, err = ApplyMarkerBlock(path, "agent-auto-capture", "second", false)
	if err != nil {
		t.Fatalf("update marker block: %v", err)
	}
	if change.Action != "updated" || change.BackupPath == "" {
		t.Fatalf("update change = %#v", change)
	}
	backup, err := os.ReadFile(change.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !strings.Contains(string(backup), "first") {
		t.Fatalf("backup = %q", backup)
	}

	change, err = ApplyMarkerBlock(path, "agent-auto-capture", "second", false)
	if err != nil {
		t.Fatalf("idempotent marker block: %v", err)
	}
	if change.Action != "unchanged" || change.BackupPath != "" {
		t.Fatalf("idempotent change = %#v", change)
	}
}

func TestManagedFileUsesFallbackWriterForBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "OneDrive", "profile.ps1")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}
	var writes []string
	write := func(target string, contents []byte, perm os.FileMode, _ []byte, _ bool) error {
		writes = append(writes, target)
		return os.WriteFile(target, contents, perm)
	}
	change, err := applyManagedFileWithWrite(path, "updated", false, write)
	if err != nil {
		t.Fatalf("apply managed file: %v", err)
	}
	if change.BackupPath == "" || len(writes) != 2 || writes[0] != change.BackupPath || writes[1] != path {
		t.Fatalf("writer calls = %v, backup = %q", writes, change.BackupPath)
	}
	backup, err := os.ReadFile(change.BackupPath)
	if err != nil || string(backup) != "original" {
		t.Fatalf("backup = %q, %v", backup, err)
	}
}

func TestApplyMarkerBlockDryRunDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.md")
	change, err := ApplyMarkerBlock(path, "agent-auto-capture", "content", true)
	if err != nil {
		t.Fatalf("dry-run marker block: %v", err)
	}
	if change.Action != "create" {
		t.Fatalf("dry-run change = %#v", change)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote file: %v", err)
	}
}

func TestApplyMarkerBlockDryRunIncludesPlannedBackupPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent", "config.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	change, err := ApplyMarkerBlock(path, "agent-auto-capture", "content", true)
	if err != nil {
		t.Fatalf("dry-run marker block: %v", err)
	}
	if change.Action != "update" || change.BackupPath == "" || !strings.Contains(change.BackupPath, path+".qlog-backup-") {
		t.Fatalf("dry-run change = %#v", change)
	}
}

func TestCommandAdapterUninstallRemovesOnlyOwnedMarker(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newCommandAdapter("sample", "Sample", "go", ".sample/config.md")
	path := filepath.Join(configHome, ".sample", "config.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("before\n"), 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	result, err := adapter.Uninstall(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !result.Changed {
		t.Fatalf("uninstall result = %#v", result)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after uninstall: %v", err)
	}
	if !strings.Contains(string(contents), "before") || strings.Contains(string(contents), "qlog:begin agent-auto-capture") {
		t.Fatalf("contents after uninstall = %q", contents)
	}
}

func TestCommandAdapterStatusInstalledAndTestRequiresInstall(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newCommandAdapter("sample", "Sample", "go", ".sample/config.md")
	result, err := adapter.Test(context.Background())
	if err != nil {
		t.Fatalf("test before install: %v", err)
	}
	if result.Passed {
		t.Fatalf("test passed before setup install: %#v", result)
	}
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Installed || status.State != SetupInstalled {
		t.Fatalf("status = %#v", status)
	}
	result, err = adapter.Test(context.Background())
	if err != nil {
		t.Fatalf("test after install: %v", err)
	}
	if !result.Passed {
		t.Fatalf("test after install = %#v", result)
	}
}
