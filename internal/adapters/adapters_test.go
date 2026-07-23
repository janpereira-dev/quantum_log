package adapters

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultRegistryDeclaresOnlyVerifiedCapabilities(t *testing.T) {
	registry := Default()
	items := registry.List()
	if len(items) != 8 {
		t.Fatalf("List() returned %d adapters, want 8", len(items))
	}
	generic, found := registry.Get("generic-jsonl")
	if !found || !generic.Descriptor().Capabilities.StructuredEvents {
		t.Fatal("generic JSONL adapter must declare structured event support")
	}
	if generic.Descriptor().Capabilities.Costs || generic.Descriptor().Capabilities.InputTokens {
		t.Fatal("generic JSONL must not claim metrics supplied only by callers")
	}
	copilot, found := registry.Get("copilot-vscode")
	if !found || !copilot.Descriptor().Capabilities.InputTokens || !copilot.Descriptor().Capabilities.CacheTokens || !copilot.Descriptor().Capabilities.MCPCalls {
		t.Fatalf("copilot-vscode must declare verified OTel token/cache/MCP capabilities")
	}
	opencode, found := registry.Get("opencode")
	if !found || !opencode.Descriptor().Capabilities.ToolCalls || !opencode.Descriptor().Capabilities.SessionLifecycle || !opencode.Descriptor().Capabilities.StructuredEvents {
		t.Fatalf("opencode must declare plugin lifecycle/tool capture capabilities")
	}
	codex, found := registry.Get("codex")
	if !found || !codex.Descriptor().Capabilities.InputTokens || !codex.Descriptor().Capabilities.OutputTokens || !codex.Descriptor().Capabilities.CacheTokens || !codex.Descriptor().Capabilities.ReasoningTokens || !codex.Descriptor().Capabilities.StructuredEvents {
		t.Fatalf("codex must declare app-server rawResponse usage capabilities")
	}
	claude, found := registry.Get("claude-code")
	if !found || !claude.Descriptor().Capabilities.SessionLifecycle || !claude.Descriptor().Capabilities.StructuredEvents || claude.Descriptor().Capabilities.InputTokens {
		t.Fatalf("claude-code must declare lifecycle hooks without token capability")
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
	if !status.Installed || status.CaptureQuality != CaptureExperimental {
		t.Fatalf("status = %#v", status)
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
	for _, want := range []string{"/v1/events", "session.created", "message.updated", "tool.execute.before", "tool.execute.after", "capture_quality", "prompt", "response"} {
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
