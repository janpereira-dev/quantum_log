package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type vscodeCopilotAdapter struct {
	commandAdapter
}

func newVSCodeCopilotAdapter() vscodeCopilotAdapter {
	return vscodeCopilotAdapter{commandAdapter: newCommandAdapter("copilot-vscode", "GitHub Copilot for VS Code", "code", "Code/User/prompts/qlog.instructions.md")}
}

func (a vscodeCopilotAdapter) Descriptor() Descriptor {
	return Descriptor{ID: a.id, Name: a.name, Version: "otel", Capabilities: Capabilities{ModelIdentity: true, InputTokens: true, OutputTokens: true, ReasoningTokens: true, CacheTokens: true, ToolCalls: true, MCPCalls: true, SessionLifecycle: true, ProjectIdentity: true, VCSContext: true, StructuredEvents: true}}
}

func (a vscodeCopilotAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyJSONSettings(a.settingsPath(), copilotOTelSettings(), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: change.Action == "created" || change.Action == "updated", Actions: []string{formatChange(change)}}, nil
}

func (a vscodeCopilotAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyJSONSettings(a.settingsPath(), copilotOTelSettings(), true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureOTELReported, Changes: []SetupChange{change}, Notes: []string{"configures VS Code Copilot native OpenTelemetry to qlog localhost collector with content capture disabled"}}, nil
}

func (a vscodeCopilotAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.commandAdapter.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := jsonSettingsContain(a.settingsPath(), copilotOTelSettings())
	state := SetupUnavailable
	if detection.Available {
		state = SetupAvailable
	}
	if installed {
		state = SetupInstalled
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, CaptureQuality: CaptureOTELReported, Evidence: detection.Evidence, Notes: []string{"VS Code Copilot emits GenAI OTel spans with token, cache, reasoning, tool, MCP, repo, branch, and commit metadata when enabled"}}, nil
}

func (a vscodeCopilotAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Installed, CaptureQuality: CaptureOTELReported, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a vscodeCopilotAdapter) settingsPath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "Code", "User", "settings.json")
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "Code", "User", "settings.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "Code", "User", "settings.json")
	}
	return filepath.Join("Code", "User", "settings.json")
}

func copilotOTelSettings() map[string]any {
	return map[string]any{
		"github.copilot.chat.otel.enabled":        true,
		"github.copilot.chat.otel.exporterType":   "otlp-http",
		"github.copilot.chat.otel.otlpEndpoint":   "http://127.0.0.1:4318",
		"github.copilot.chat.otel.captureContent": false,
	}
}

func applyJSONSettings(path string, desired map[string]any, dryRun bool) (SetupChange, error) {
	currentBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, fmt.Errorf("read %s: %w", path, err)
	}
	settings := map[string]any{}
	if err == nil && len(currentBytes) > 0 {
		if err := json.Unmarshal(currentBytes, &settings); err != nil {
			return SetupChange{}, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	changed := false
	for key, value := range desired {
		if settings[key] != value {
			settings[key] = value
			changed = true
		}
	}
	if !changed {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog settings already up to date"}, nil
	}
	action := "created"
	if err == nil {
		action = "updated"
	}
	if dryRun {
		return SetupChange{Path: path, Action: action, Description: "dry run: qlog settings would be written"}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, fmt.Errorf("create parent directory: %w", err)
	}
	change := SetupChange{Path: path, Action: action, Description: "qlog settings written"}
	if err == nil {
		backupPath := fmt.Sprintf("%s.qlog-backup-%s", path, time.Now().UTC().Format("20060102150405"))
		if err := os.WriteFile(backupPath, currentBytes, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("write backup: %w", err)
		}
		change.BackupPath = backupPath
	}
	next, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return SetupChange{}, err
	}
	next = append(next, '\n')
	if err := os.WriteFile(path, next, 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write %s: %w", path, err)
	}
	return change, nil
}

func jsonSettingsContain(path string, desired map[string]any) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	settings := map[string]any{}
	if err := json.Unmarshal(contents, &settings); err != nil {
		return false
	}
	for key, value := range desired {
		if settings[key] != value {
			return false
		}
	}
	return true
}
