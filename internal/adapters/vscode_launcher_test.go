package adapters

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

func TestVSCodeLauncherHasNoStaticTokenAndPreservesEditedLauncher(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "user edited"}[edit], func(t *testing.T) {
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
			home := t.TempDir()
			a := newVSCodeCopilotAdapter()
			if _, err := a.Install(context.Background(), InstallOptions{Home: home}); err != nil {
				t.Fatal(err)
			}
			state, err := a.readLauncherState()
			if err != nil {
				t.Fatal(err)
			}
			paths, err := config.Resolve(home)
			if err != nil {
				t.Fatal(err)
			}
			token, err := config.CollectorToken(paths)
			if err != nil {
				t.Fatal(err)
			}
			settings := string(mustReadFile(t, a.settingsPath()))
			launcher := mustReadFile(t, state.Path)
			if strings.Contains(settings, token) || strings.Contains(string(launcher), token) {
				t.Fatal("reusable token embedded in settings or launcher")
			}
			if edit {
				launcher = append(launcher, []byte("\n# user customization\n")...)
				if err := os.WriteFile(state.Path, launcher, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := a.Uninstall(context.Background(), InstallOptions{Home: home}); err != nil {
				t.Fatal(err)
			}
			if edit {
				if got := mustReadFile(t, state.Path); string(got) != string(launcher) {
					t.Fatal("user-edited launcher overwritten")
				}
			} else if _, err := os.Stat(state.Path); !os.IsNotExist(err) {
				t.Fatalf("owned launcher retained: %v", err)
			}
			if got := mustReadFile(t, filepath.Join(home, "collector.token")); string(got) != token {
				t.Fatal("shared credential changed by individual uninstall")
			}
		})
	}
}

func TestVSCodePowerShellLauncherScopesTrustAndFailsClosedWithoutCA(t *testing.T) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("PowerShell required")
	}
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	home := filepath.Join(t.TempDir(), "collector's-á")
	a := newVSCodeCopilotAdapter()
	if _, err := a.Install(context.Background(), InstallOptions{Home: home}); err != nil {
		t.Fatal(err)
	}
	state, err := a.readLauncherState()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	token, err := config.CollectorToken(paths)
	if err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "child-env")
	fakeCode := filepath.Join(t.TempDir(), "code.cmd")
	fake := "@echo off\r\nset OTEL_EXPORTER_OTLP>\"%QLOG_TEST_CAPTURE%\"\r\necho NODE_TLS_REJECT_UNAUTHORIZED=%NODE_TLS_REJECT_UNAUTHORIZED% >>\"%QLOG_TEST_CAPTURE%\"\r\nexit /b 0\r\n"
	if err := os.WriteFile(fakeCode, []byte(fake), 0600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "verify.ps1")
	script := `$ErrorActionPreference='Stop'
$env:OTEL_EXPORTER_OTLP_HEADERS='Authorization=original'
$env:OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE='original-ca'
$env:NODE_TLS_REJECT_UNAUTHORIZED='0'
function Get-Command { param($Name,$CommandType,$ErrorAction); if ($Name -eq 'code') {return [pscustomobject]@{Path=$env:QLOG_TEST_CODE}}; throw 'unexpected command' }
& $env:QLOG_TEST_LAUNCHER 'synthetic argument'
if ($env:OTEL_EXPORTER_OTLP_HEADERS -ne 'Authorization=original' -or $env:OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE -ne 'original-ca' -or $env:NODE_TLS_REJECT_UNAUTHORIZED -ne '0') {throw 'parent environment changed'}
`
	if err := os.WriteFile(wrapper, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	run := func() ([]byte, error) {
		command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", wrapper)
		command.Env = append(os.Environ(), "QLOG_TEST_CODE="+fakeCode, "QLOG_TEST_CAPTURE="+capture, "QLOG_TEST_LAUNCHER="+state.Path)
		return command.CombinedOutput()
	}
	if output, err := run(); err != nil {
		t.Fatalf("launcher: %v %s", err, output)
	}
	captured := string(mustReadFile(t, capture))
	for _, want := range []string{"OTEL_EXPORTER_OTLP_HEADERS=x-qlog-token=" + token, "OTEL_EXPORTER_OTLP_TRACES_HEADERS=x-qlog-token=" + token, "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE=" + config.CollectorCertificatePath(paths), "OTEL_EXPORTER_OTLP_ENDPOINT=https://127.0.0.1:4318", "NODE_TLS_REJECT_UNAUTHORIZED=1"} {
		if !strings.Contains(captured, want) {
			t.Fatalf("child missing expected scoped value %q", want)
		}
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	ca := config.CollectorCertificatePath(paths)
	if err := os.Rename(ca, ca+".saved"); err != nil {
		t.Fatal(err)
	}
	if output, err := run(); err == nil {
		t.Fatalf("missing CA launched Code: %s", output)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("Code child launched without trust file")
	}
}

func adapterShellPath(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && len(path) > 2 && path[1] == ':' {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

func TestPOSIXProducerLaunchersScopeTrustAndFailClosedWithoutCA(t *testing.T) {
	for _, producer := range []string{"code", "copilot"} {
		t.Run(producer, func(t *testing.T) {
			shell := "sh"
			if runtime.GOOS == "windows" {
				shell = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
			}
			if _, err := exec.LookPath(shell); err != nil {
				t.Skip("POSIX shell unavailable")
			}
			t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
			home := t.TempDir()
			paths, token, err := collectorCredentials(home, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writeCollectorTokenFile(home, token, false); err != nil {
				t.Fatal(err)
			}
			source := vscodeLauncherSource(paths, false)
			if producer == "copilot" {
				source = "#!/bin/sh\n" + copilotCLIPosixBlock(filepath.Join(home, "collector.token"), config.CollectorCertificatePath(paths)) + "copilot \"$@\"\n"
			}
			ca := config.CollectorCertificatePath(paths)
			tokenPath := filepath.Join(home, "collector.token")
			source = strings.NewReplacer(ca, adapterShellPath(ca), tokenPath, adapterShellPath(tokenPath)).Replace(source)
			launcher := filepath.Join(t.TempDir(), "qlog-code")
			if err := os.WriteFile(launcher, []byte(source), 0700); err != nil {
				t.Fatal(err)
			}
			tools := t.TempDir()
			capture := filepath.Join(t.TempDir(), "child-env")
			fakeCode := filepath.Join(tools, producer)
			argvCapture := filepath.Join(t.TempDir(), "external-env-argv")
			spyEnv := `#!/bin/sh
printf '%s\n' "$@" > "$QLOG_TEST_ARGV_CAPTURE"
exec /usr/bin/env "$@"
`
			if err := os.WriteFile(filepath.Join(tools, "env"), []byte(spyEnv), 0700); err != nil {
				t.Fatal(err)
			}
			fake := `#!/bin/sh
set -eu
for key in OTEL_EXPORTER_OTLP_HEADERS OTEL_EXPORTER_OTLP_TRACES_HEADERS OTEL_EXPORTER_OTLP_CERTIFICATE OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE OTEL_EXPORTER_OTLP_ENDPOINT NODE_TLS_REJECT_UNAUTHORIZED OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE OTEL_EXPORTER_OTLP_METRICS_HEADERS OTEL_EXPORTER_OTLP_LOGS_HEADERS OTEL_EXPORTER_OTLP_METRICS_ENDPOINT OTEL_EXPORTER_OTLP_LOGS_ENDPOINT; do printf '%s=' "$key"; printenv "$key"; done > "$QLOG_TEST_CAPTURE"
`
			if err := os.WriteFile(fakeCode, []byte(fake), 0700); err != nil {
				t.Fatal(err)
			}
			run := func() ([]byte, error) {
				command := exec.Command(shell, "-c", `export PATH="$QLOG_TEST_TOOLS:/usr/bin:/bin:$PATH"; export OTEL_EXPORTER_OTLP_HEADERS='Authorization=original'; export OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE='original-ca'; export NODE_TLS_REJECT_UNAUTHORIZED=0; export OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE=hostile-metrics-ca.pem; export OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE=hostile-logs-ca.pem; export OTEL_EXPORTER_OTLP_METRICS_HEADERS=Authorization=hostile-metrics; export OTEL_EXPORTER_OTLP_LOGS_HEADERS=Authorization=hostile-logs; export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=https://127.0.0.1:9999/v1/metrics; export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=https://127.0.0.1:9999/v1/logs; sh "$QLOG_TEST_LAUNCHER" 'synthetic argument' || exit $?; [ "$OTEL_EXPORTER_OTLP_HEADERS" = 'Authorization=original' ] && [ "$OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE" = 'original-ca' ] && [ "$NODE_TLS_REJECT_UNAUTHORIZED" = 0 ] && [ "$OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE" = hostile-metrics-ca.pem ] && [ "$OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE" = hostile-logs-ca.pem ] && [ "$OTEL_EXPORTER_OTLP_METRICS_HEADERS" = Authorization=hostile-metrics ] && [ "$OTEL_EXPORTER_OTLP_LOGS_HEADERS" = Authorization=hostile-logs ] && [ "$OTEL_EXPORTER_OTLP_METRICS_ENDPOINT" = https://127.0.0.1:9999/v1/metrics ] && [ "$OTEL_EXPORTER_OTLP_LOGS_ENDPOINT" = https://127.0.0.1:9999/v1/logs ]`)
				command.Env = append(os.Environ(), "QLOG_TEST_TOOLS="+adapterShellPath(tools), "QLOG_TEST_CAPTURE="+adapterShellPath(capture), "QLOG_TEST_LAUNCHER="+adapterShellPath(launcher), "QLOG_TEST_ARGV_CAPTURE="+adapterShellPath(argvCapture))
				return command.CombinedOutput()
			}
			if output, err := run(); err != nil {
				t.Fatalf("POSIX launcher: %v %s", err, output)
			}
			if _, err := os.Stat(argvCapture); !os.IsNotExist(err) {
				t.Fatal("launcher passed reusable credentials through external env argv")
			}
			captured := string(mustReadFile(t, capture))
			for _, want := range []string{"OTEL_EXPORTER_OTLP_HEADERS=x-qlog-token=" + token, "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE=" + adapterShellPath(ca), "OTEL_EXPORTER_OTLP_ENDPOINT=https://127.0.0.1:4318", "NODE_TLS_REJECT_UNAUTHORIZED=1", "OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE=" + adapterShellPath(ca), "OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE=" + adapterShellPath(ca), "OTEL_EXPORTER_OTLP_METRICS_HEADERS=x-qlog-token=" + token, "OTEL_EXPORTER_OTLP_LOGS_HEADERS=x-qlog-token=" + token, "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=https://127.0.0.1:4318/v1/metrics", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=https://127.0.0.1:4318/v1/logs"} {
				if !strings.Contains(captured, want) {
					t.Fatalf("POSIX child missing scoped value %q", want)
				}
			}
			if err := os.Remove(capture); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(ca, ca+".saved"); err != nil {
				t.Fatal(err)
			}
			if output, err := run(); err == nil {
				t.Fatalf("missing CA launched POSIX Code: %s", output)
			}
			if _, err := os.Stat(capture); !os.IsNotExist(err) {
				t.Fatal("Code child launched without CA")
			}

			if err := os.Rename(ca+".saved", ca); err != nil {
				t.Fatal(err)
			}
			for _, tokenCase := range []string{"missing token", "invalid 64-byte token"} {
				if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if tokenCase == "invalid 64-byte token" {
					if err := os.WriteFile(tokenPath, []byte(strings.Repeat("z", 64)), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if output, err := run(); err == nil {
					t.Fatalf("POSIX producer did not reject %s: %s", tokenCase, output)
				}
				if _, err := os.Stat(capture); !os.IsNotExist(err) {
					t.Fatalf("POSIX producer launched child with %s", tokenCase)
				}
			}
		})
	}
}

func TestCopilotPowerShellLauncherRequiresTrustAndHexTokenBeforeChild(t *testing.T) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("PowerShell required")
	}
	for _, scenario := range []string{"missing CA", "missing token", "invalid 64-byte token"} {
		t.Run(scenario, func(t *testing.T) {
			tokenPath := filepath.Join(t.TempDir(), "collector.token")
			caPath := filepath.Join(t.TempDir(), "collector.pem")
			if scenario != "missing CA" {
				if err := os.WriteFile(caPath, []byte("synthetic public CA"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "missing token" {
				token := strings.Repeat("a", 64)
				if scenario == "invalid 64-byte token" {
					token = strings.Repeat("z", 64)
				}
				if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
					t.Fatal(err)
				}
			}
			marker := filepath.Join(t.TempDir(), "child-launched")
			source := `$ErrorActionPreference='Continue'
function global:copilot {[IO.File]::WriteAllText($env:QLOG_TEST_CHILD,'launched')}
` + copilotCLIProfileBlock(tokenPath, caPath) + "copilot\n"
			path := filepath.Join(t.TempDir(), "invalid-credentials.ps1")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
			command.Env = append(os.Environ(), "QLOG_TEST_CHILD="+marker)
			output, runErr := command.CombinedOutput()
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("unwrapped launcher reached child without valid trust/token: %s", output)
			}
			if runErr == nil {
				t.Fatalf("unwrapped launcher did not reject %s", scenario)
			}
		})
	}
}
