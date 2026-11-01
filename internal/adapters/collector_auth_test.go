package adapters

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLegacyProducerInstallsAuthenticateCustomHome(t *testing.T) {
	for _, id := range []string{"codex", "claude-code", "copilot-vscode", "copilot", "opencode"} {
		t.Run(id, func(t *testing.T) {
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
			home := t.TempDir()
			key := []byte(strings.Repeat("custom-home-key-", 3))
			if err := os.WriteFile(filepath.Join(home, "prompt-hash.key"), key, 0600); err != nil {
				t.Fatal(err)
			}
			mac := hmac.New(sha256.New, key)
			mac.Write([]byte("qlog-collector-auth-v2"))
			token := hex.EncodeToString(mac.Sum(nil))
			adapter, _ := Default().Get(id)
			if _, err := adapter.Install(context.Background(), InstallOptions{Home: home}); err != nil {
				t.Fatal(err)
			}
			var path string
			switch id {
			case "codex":
				path = newCodexAdapter().configPath()
			case "claude-code":
				path = newClaudeCodeAdapter().settingsPath()
			case "copilot-vscode":
				path = newVSCodeCopilotAdapter().settingsPath()
			case "copilot":
				path = copilotCLIPowerShellProfilePath()
				if runtime.GOOS != "windows" {
					path = newCopilotCLIAdapter().posixProfilePath()
				}
			case "opencode":
				path = newOpenCodeAdapter().pluginPath()
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			source := string(data)
			if id == "opencode" {
				if !strings.Contains(source, "X-Qlog-Token") || !strings.Contains(source, "qlog-collector-auth-v2") || !strings.Contains(source, strings.ReplaceAll(filepath.Join(home, "prompt-hash.key"), `\`, `\\`)) {
					t.Fatalf("plugin missing runtime auth for custom home")
				}
				if strings.Contains(source, token) || strings.Contains(source, string(key)) {
					t.Fatal("plugin embeds secret")
				}
			} else if id == "claude-code" {
				if strings.Contains(source, token) {
					t.Fatal("Claude lifecycle setup must not install a collector token")
				}
			} else if id == "copilot-vscode" {
				if strings.Contains(source, token) || strings.Contains(source, "X-Qlog-Token") {
					t.Fatal("VS settings must not contain a reusable collector token")
				}
				state, err := newVSCodeCopilotAdapter().readLauncherState()
				if err != nil {
					t.Fatal(err)
				}
				launcher := string(mustReadFile(t, state.Path))
				if !strings.Contains(launcher, "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE") || !strings.Contains(launcher, home) {
					t.Fatal("VS launcher missing custom-home TLS configuration")
				}
			} else if id == "copilot" {
				if !strings.Contains(source, "OTEL_EXPORTER_OTLP_HEADERS") || !strings.Contains(source, home) {
					t.Fatal("wrapper missing custom-home authentication")
				}
			} else if !strings.Contains(strings.ToLower(source), "x-qlog-token") || !strings.Contains(source, token) {
				t.Fatalf("%s missing authenticated exporter", id)
			}
		})
	}
}

func TestAuthenticatedProducerUninstallRestoresOriginalSettingsAfterReinstall(t *testing.T) {
	for _, id := range []string{"codex", "claude-code", "copilot-vscode"} {
		t.Run(id, func(t *testing.T) {
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
			var path string
			var original []byte
			switch id {
			case "codex":
				path = newCodexAdapter().configPath()
				original = []byte("model = \"preserve\"\n[otel]\nexporter = \"none\"\nlog_user_prompt = true\n")
			case "claude-code":
				path = newClaudeCodeAdapter().settingsPath()
				original = []byte(`{"env":{"OTEL_EXPORTER_OTLP_HEADERS":"Authorization=original","OTEL_LOGS_EXPORTER":"console","USER_SETTING":"keep"}}`)
			case "copilot-vscode":
				path = newVSCodeCopilotAdapter().settingsPath()
				original = []byte(`{"editor.fontSize":19,"github.copilot.chat.otel.headers":{"Authorization":"original"},"github.copilot.chat.otel.enabled":false}`)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			adapter, _ := Default().Get(id)
			// A second home rotates the managed credential without replacing the user's restore state.
			for i := 0; i < 2; i++ {
				if _, err := adapter.Install(context.Background(), InstallOptions{Home: t.TempDir()}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := adapter.Uninstall(context.Background(), InstallOptions{}); err != nil {
				t.Fatal(err)
			}
			restored := mustReadFile(t, path)
			if id == "codex" {
				if string(restored) != string(original) {
					t.Fatalf("original TOML not restored: %s", restored)
				}
				return
			}
			var before, after any
			if err := json.Unmarshal(original, &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(restored, &after); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("original settings not restored: %s", restored)
			}
		})
	}
}

func TestCodexAuthenticationUpgradeKeepsOldRestoreState(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	a := newCodexAdapter()
	original := "[otel]\nexporter = " + codexOTelSettings()["exporter"] + "\nlog_user_prompt = true\n"
	if err := os.MkdirAll(filepath.Dir(a.configPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.configPath(), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := applyCodexOTelConfig(a.configPath(), a.statePath(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Install(context.Background(), InstallOptions{Home: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Uninstall(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if restored := string(mustReadFile(t, a.configPath())); restored != original {
		t.Fatalf("legacy upgrade lost original exporter: %s", restored)
	}
}

func TestProducerDryRunDoesNotCreateCredentials(t *testing.T) {
	for _, id := range []string{"codex", "claude-code", "copilot-vscode", "copilot", "opencode"} {
		t.Run(id, func(t *testing.T) {
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
			home := filepath.Join(t.TempDir(), "not-created")
			adapter, _ := Default().Get(id)
			if _, err := adapter.Install(context.Background(), InstallOptions{Home: home, DryRun: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.PlanInstall(context.Background(), SetupOptions{Home: home}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Fatalf("dry run created home: %v", err)
			}
		})
	}
}

func TestPowerShellAuthenticatedWrapperRestoresParentEnvironment(t *testing.T) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("PowerShell required")
	}
	tokenPath := filepath.Join(t.TempDir(), "collector's.token")
	if err := os.WriteFile(tokenPath, []byte(strings.Repeat("a", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(t.TempDir(), "collector's.pem")
	if err := os.WriteFile(caPath, []byte("synthetic public CA"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `$env:OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE='original-ca'
$env:NODE_TLS_REJECT_UNAUTHORIZED='0'
$env:OTEL_EXPORTER_OTLP_HEADERS = 'Authorization=original'
foreach ($signal in @('LOGS','METRICS')) {
 Set-Item ('Env:OTEL_EXPORTER_OTLP_'+$signal+'_CERTIFICATE') ('hostile-'+$signal+'-ca.pem')
 Set-Item ('Env:OTEL_EXPORTER_OTLP_'+$signal+'_HEADERS') ('Authorization=hostile-'+$signal)
 Set-Item ('Env:OTEL_EXPORTER_OTLP_'+$signal+'_ENDPOINT') ('https://127.0.0.1:9999/v1/'+$signal.ToLower())
}
function global:copilot { foreach ($signal in @('LOGS','METRICS')) { if ([Environment]::GetEnvironmentVariable('OTEL_EXPORTER_OTLP_'+$signal+'_CERTIFICATE','Process') -ne $env:QLOG_TEST_CA -or [Environment]::GetEnvironmentVariable('OTEL_EXPORTER_OTLP_'+$signal+'_HEADERS','Process') -ne ('x-qlog-token='+('a'*64)) -or [Environment]::GetEnvironmentVariable('OTEL_EXPORTER_OTLP_'+$signal+'_ENDPOINT','Process') -ne ('https://127.0.0.1:4318/v1/'+$signal.ToLower())) {throw 'ambient per-signal trust/credential/destination escaped'} }; if ($env:OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE -ne $env:QLOG_TEST_CA -or $env:NODE_TLS_REJECT_UNAUTHORIZED -ne '1') {throw 'child missing TLS pin'}; if ($env:OTEL_EXPORTER_OTLP_HEADERS -ne ('x-qlog-token=' + ('a' * 64))) { throw 'child missing authentication' }; throw 'synthetic failure' }
` + copilotCLIProfileBlock(tokenPath, caPath) + `
try { copilot; throw 'expected synthetic failure' } catch { if ($_.Exception.Message -ne 'synthetic failure') { throw } }
if ($env:OTEL_EXPORTER_OTLP_HEADERS -ne 'Authorization=original' -or $env:OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE -ne 'original-ca' -or $env:NODE_TLS_REJECT_UNAUTHORIZED -ne '0') { throw 'parent headers or TLS were changed' }
foreach ($signal in @('LOGS','METRICS')) {
 if ([Environment]::GetEnvironmentVariable('OTEL_EXPORTER_OTLP_'+$signal+'_CERTIFICATE','Process') -ne ('hostile-'+$signal+'-ca.pem') -or [Environment]::GetEnvironmentVariable('OTEL_EXPORTER_OTLP_'+$signal+'_HEADERS','Process') -ne ('Authorization=hostile-'+$signal) -or [Environment]::GetEnvironmentVariable('OTEL_EXPORTER_OTLP_'+$signal+'_ENDPOINT','Process') -ne ('https://127.0.0.1:9999/v1/'+$signal.ToLower())) {throw 'parent per-signal environment not restored'}
}
`
	path := filepath.Join(t.TempDir(), "wrapper-test.ps1")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	cmd.Env = append(os.Environ(), "QLOG_TEST_CA="+caPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("wrapper runtime: %v\n%s", err, output)
	}
}

func TestLegacyProducersRequirePinnedHTTPS(t *testing.T) {
	for _, id := range []string{"codex", "copilot", "opencode"} {
		t.Run(id, func(t *testing.T) {
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
			home := t.TempDir()
			adapter, _ := Default().Get(id)
			if _, err := adapter.Install(context.Background(), InstallOptions{Home: home}); err != nil {
				t.Fatal(err)
			}
			var source string
			switch id {
			case "codex":
				source = string(mustReadFile(t, newCodexAdapter().configPath()))
			case "claude-code":
				source = string(mustReadFile(t, newClaudeCodeAdapter().settingsPath()))
			case "copilot":
				path := copilotCLIPowerShellProfilePath()
				if runtime.GOOS != "windows" {
					path = newCopilotCLIAdapter().posixProfilePath()
				}
				source = string(mustReadFile(t, path))
			case "opencode":
				source = string(mustReadFile(t, newOpenCodeAdapter().pluginPath()))
			}
			if strings.Contains(source, "http://127.0.0.1:4318") || !strings.Contains(source, "https://127.0.0.1:4318") {
				t.Fatal("producer does not exclusively use HTTPS")
			}
			switch id {
			case "codex":
				if !strings.Contains(source, "ca-certificate") {
					t.Fatal("Codex lacks exporter CA pin")
				}
			case "claude-code", "copilot":
				for _, key := range []string{"OTEL_EXPORTER_OTLP_CERTIFICATE", "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE"} {
					if !strings.Contains(source, key) {
						t.Fatalf("producer lacks %s", key)
					}
				}
			case "opencode":
				if !strings.Contains(source, "rejectUnauthorized: true") || strings.Contains(source, "fetch(") {
					t.Fatal("plugin lacks pinned HTTPS transport")
				}
			}
		})
	}
}

func TestClaudeLifecycleSetupNeverInstallsUnsupportedNativeCredentials(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	a := newClaudeCodeAdapter()
	if _, err := a.Install(context.Background(), InstallOptions{Home: home}); err != nil {
		t.Fatal(err)
	}
	settings := readSettingsMap(t, a.settingsPath())
	env := settings["env"].(map[string]any)
	for _, key := range []string{"OTEL_TRACES_EXPORTER", "OTEL_LOGS_EXPORTER", "OTEL_METRICS_EXPORTER"} {
		if env[key] != "none" {
			t.Fatalf("unsupported native exporter remains enabled: %s", key)
		}
	}
	for key := range env {
		if strings.Contains(key, "HEADERS") || strings.Contains(key, "CERTIFICATE") {
			t.Fatalf("unsupported native credential/trust installed: %s", key)
		}
	}
	status, err := a.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.CaptureQuality != CaptureLifecycleOnly || status.State != SetupPartial {
		t.Fatalf("unsupported TLS transport overclaimed: %#v", status)
	}
	if a.Descriptor().Capabilities.InputTokens || a.Descriptor().Capabilities.OutputTokens || a.Descriptor().Capabilities.ModelIdentity {
		t.Fatal("lifecycle-only adapter claims native model usage")
	}
}

func TestClaudeLifecycleMigrationRemovesPriorNativeTokenAndRestoresOriginalEnvironment(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	a := newClaudeCodeAdapter()
	original := []byte(`{"env":{"OTEL_EXPORTER_OTLP_HEADERS":"Authorization=original","OTEL_TRACES_EXPORTER":"console","USER_SETTING":"keep"}}`)
	oldEnv := map[string]string{"CLAUDE_CODE_ENABLE_TELEMETRY": "1", "OTEL_TRACES_EXPORTER": "otlp", "OTEL_METRICS_EXPORTER": "none", "OTEL_LOGS_EXPORTER": "none", "OTEL_EXPORTER_OTLP_HEADERS": "x-qlog-token=" + strings.Repeat("b", 64), "OTEL_EXPORTER_OTLP_TRACES_HEADERS": "x-qlog-token=" + strings.Repeat("b", 64), "OTEL_EXPORTER_OTLP_CERTIFICATE": "old-ca.pem", "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE": "old-ca.pem", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "https://127.0.0.1:4318/v1/traces"}
	oldSettings, err := claudeSettingsWithQlogHooks(original, claudeCodeHookCommand(home, ""), oldEnv)
	if err != nil {
		t.Fatal(err)
	}
	state := vscodeSettingsMarker{Managed: map[string]any{}, Previous: map[string]any{"OTEL_EXPORTER_OTLP_HEADERS": "Authorization=original", "OTEL_TRACES_EXPORTER": "console"}, PreviousPresent: map[string]bool{}}
	for key, value := range oldEnv {
		state.Managed[key] = value
		_, state.PreviousPresent[key] = state.Previous[key]
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(a.settingsPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.settingsPath(), oldSettings, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.settingsPath()+".qlog-otel-state.json", encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Install(context.Background(), InstallOptions{Home: home}); err != nil {
		t.Fatal(err)
	}
	settings := readSettingsMap(t, a.settingsPath())
	env := settings["env"].(map[string]any)
	if env["OTEL_EXPORTER_OTLP_HEADERS"] != "Authorization=original" {
		t.Fatal("original user header not restored during safe migration")
	}
	for _, key := range []string{"OTEL_EXPORTER_OTLP_TRACES_HEADERS", "OTEL_EXPORTER_OTLP_CERTIFICATE", "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"} {
		if _, found := env[key]; found {
			t.Fatalf("prior unsupported native setting retained: %s", key)
		}
	}
	if _, err := a.Uninstall(context.Background(), InstallOptions{Home: home}); err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(original, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mustReadFile(t, a.settingsPath()), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatal("migration/uninstall lost original user environment")
	}
}
