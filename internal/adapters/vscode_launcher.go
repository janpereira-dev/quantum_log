package adapters

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

const vscodeLauncherMarker = "# Managed by qlog: VS Code TLS launcher"

type vscodeLauncherState struct {
	Path string `json:"path"`
	Hash string `json:"sha256"`
}

func (a vscodeCopilotAdapter) launcherStatePath() string {
	return a.settingsPath() + ".qlog-launcher.json"
}

func vscodeLauncherEnvironment(paths config.Paths) map[string]string {
	env := map[string]string{
		"NODE_TLS_REJECT_UNAUTHORIZED": "1",
		"COPILOT_OTEL_ENABLED":         "true", "COPILOT_OTEL_EXPORTER_TYPE": "otlp-http", "COPILOT_OTEL_ENDPOINT": "https://127.0.0.1:4318",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "https://127.0.0.1:4318", "OTEL_EXPORTER_OTLP_PROTOCOL": "http/json",
		"OTEL_EXPORTER_OTLP_CERTIFICATE":                     config.CollectorCertificatePath(paths),
		"OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT": "false",
	}
	for _, signal := range []string{"TRACES", "LOGS", "METRICS"} {
		env["OTEL_EXPORTER_OTLP_"+signal+"_PROTOCOL"] = "http/json"
		env["OTEL_EXPORTER_OTLP_"+signal+"_CERTIFICATE"] = config.CollectorCertificatePath(paths)
		env["OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT"] = "https://127.0.0.1:4318/v1/" + strings.ToLower(signal)
	}
	return env
}

func vscodeLauncherSource(paths config.Paths, windows bool) string {
	env := vscodeLauncherEnvironment(paths)
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	caPath, tokenPath := config.CollectorCertificatePath(paths), filepath.Join(paths.Home, "collector.token")
	if !windows {
		source := "#!/bin/sh\n" + vscodeLauncherMarker + "\nset -eu\n[ -r " + shellQuote(caPath) + " ] && [ -s " + shellQuote(caPath) + " ] || { echo 'qlog-code: collector CA is missing' >&2; exit 1; }\nqlog_token=$(cat " + shellQuote(tokenPath) + ")\ncase \"$qlog_token\" in *[!0-9a-f]*) echo 'qlog-code: collector token is invalid' >&2; exit 1;; esac\n[ \"${#qlog_token}\" -eq 64 ] || { echo 'qlog-code: collector token is invalid' >&2; exit 1; }\n"
		for _, key := range keys {
			source += "export " + key + "=" + shellQuote(env[key]) + "\n"
		}
		for _, key := range []string{"OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS", "OTEL_EXPORTER_OTLP_LOGS_HEADERS", "OTEL_EXPORTER_OTLP_METRICS_HEADERS"} {
			source += "export " + key + "=\"x-qlog-token=$qlog_token\"\n"
		}
		return source + "exec code \"$@\"\n"
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	source := vscodeLauncherMarker + "\n$ErrorActionPreference = 'Stop'\n[void][IO.File]::ReadAllBytes(" + quote(caPath) + ")\n$qlogToken = [IO.File]::ReadAllText(" + quote(tokenPath) + ").Trim()\nif ($qlogToken -notmatch '^[0-9a-f]{64}$') { throw 'qlog-code: collector token is invalid' }\n$qlogPrevious = @{}\n$qlogEnvironment = @{\n"
	for _, key := range keys {
		source += "  " + quote(key) + " = " + quote(env[key]) + "\n"
	}
	for _, key := range []string{"OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS", "OTEL_EXPORTER_OTLP_LOGS_HEADERS", "OTEL_EXPORTER_OTLP_METRICS_HEADERS"} {
		source += "  " + quote(key) + " = ('x-qlog-token=' + $qlogToken)\n"
	}
	return source + "}\ntry {\n  foreach ($key in $qlogEnvironment.Keys) { $qlogPrevious[$key] = [Environment]::GetEnvironmentVariable($key, 'Process'); Set-Item -Path ('Env:' + $key) -Value $qlogEnvironment[$key] }\n  $qlogCode = @(Get-Command code -CommandType Application -ErrorAction Stop)[0].Path\n  & $qlogCode @args\n  $qlogExitCode = $LASTEXITCODE\n} finally {\n  foreach ($key in $qlogPrevious.Keys) { if ($null -eq $qlogPrevious[$key]) { Remove-Item -Path ('Env:' + $key) -ErrorAction SilentlyContinue } else { Set-Item -Path ('Env:' + $key) -Value $qlogPrevious[$key] } }\n}\nif ($null -ne $qlogExitCode -and $qlogExitCode -ne 0) { throw ('code exited with code ' + $qlogExitCode) }\n"
}

func (a vscodeCopilotAdapter) readLauncherState() (vscodeLauncherState, error) {
	if err := config.CheckPrivateFile(a.launcherStatePath()); err != nil {
		return vscodeLauncherState{}, err
	}
	data, err := os.ReadFile(a.launcherStatePath())
	if err != nil {
		return vscodeLauncherState{}, err
	}
	var state vscodeLauncherState
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func launcherHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func (a vscodeCopilotAdapter) installLauncher(paths config.Paths, dryRun bool) (SetupChange, error) {
	name := "qlog-code"
	if runtime.GOOS == "windows" {
		name += ".ps1"
	}
	path := filepath.Join(paths.Home, name)
	source := vscodeLauncherSource(paths, runtime.GOOS == "windows")
	if runtime.GOOS == "windows" && hasNonASCIIBody([]byte(source)) {
		source = string([]byte{0xEF, 0xBB, 0xBF}) + source
	}
	previous, stateErr := a.readLauncherState()
	if stateErr != nil && !os.IsNotExist(stateErr) {
		return SetupChange{}, stateErr
	}
	if data, err := os.ReadFile(path); err == nil {
		if !strings.Contains(string(data), vscodeLauncherMarker) || (string(data) != source && (previous.Path != path || launcherHash(data) != previous.Hash)) {
			return SetupChange{}, fmt.Errorf("preserve user-edited launcher %s", path)
		}
	} else if !os.IsNotExist(err) {
		return SetupChange{}, err
	}
	change, err := applyManagedFileWithWrite(path, source, dryRun, func(path string, data []byte, _ os.FileMode, _ []byte, _ bool) error {
		if err := writePrivateAdapterFile(path, data); err != nil {
			return err
		}
		if runtime.GOOS != "windows" {
			return os.Chmod(path, 0700)
		}
		return nil
	})
	if err != nil || dryRun {
		return change, err
	}
	state := vscodeLauncherState{Path: path, Hash: launcherHash([]byte(source))}
	data, err := json.Marshal(state)
	if err != nil {
		return SetupChange{}, err
	}
	if err := writePrivateAdapterFile(a.launcherStatePath(), data); err != nil {
		return SetupChange{}, err
	}
	if previous.Path != "" && previous.Path != path {
		if data, err := os.ReadFile(previous.Path); err == nil && launcherHash(data) == previous.Hash {
			if err := os.Remove(previous.Path); err != nil {
				return SetupChange{}, err
			}
		}
	}
	change.Description = "close all VS Code windows and relaunch through this qlog-code TLS launcher"
	return change, nil
}

func (a vscodeCopilotAdapter) uninstallLauncher(dryRun bool) (SetupChange, error) {
	state, err := a.readLauncherState()
	if os.IsNotExist(err) {
		return SetupChange{Path: a.launcherStatePath(), Action: "unchanged", Description: "qlog-code launcher already absent"}, nil
	}
	if err != nil {
		return SetupChange{}, err
	}
	change := SetupChange{Path: state.Path, Action: "unchanged", Description: "preserved user-edited qlog-code launcher"}
	data, err := os.ReadFile(state.Path)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, err
	}
	if err == nil && launcherHash(data) == state.Hash {
		change.Action = "removed"
		change.Description = "removed qlog-owned VS Code TLS launcher"
		if !dryRun {
			if err := os.Remove(state.Path); err != nil {
				return SetupChange{}, err
			}
		}
	}
	if !dryRun {
		if err := os.Remove(a.launcherStatePath()); err != nil {
			return SetupChange{}, err
		}
	}
	return change, nil
}

func (a vscodeCopilotAdapter) launcherInstalled() bool {
	state, err := a.readLauncherState()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(state.Path)
	return err == nil && launcherHash(data) == state.Hash
}
