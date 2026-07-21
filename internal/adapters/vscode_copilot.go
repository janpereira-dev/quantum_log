package adapters

import (
	"context"
	"reflect"
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
	change, err := applyVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a vscodeCopilotAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureExperimental, Changes: []SetupChange{change}, Notes: []string{"configures VS Code Copilot native OpenTelemetry to qlog localhost collector with content capture disabled; run adapter verify after a real Copilot event to prove capture"}}, nil
}

func (a vscodeCopilotAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := removeVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && change.Action == "removed", Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a vscodeCopilotAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
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
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, CaptureQuality: CaptureExperimental, Evidence: detection.Evidence, Notes: []string{"VS Code Copilot OTel setup is experimental until adapter verify finds a recent Copilot-originated otel_reported model call"}}, nil
}

func (a vscodeCopilotAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Installed, CaptureQuality: CaptureExperimental, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a vscodeCopilotAdapter) settingsPath() string {
	return vscodeSettingsPath("Code")
}

func copilotOTelSettings() map[string]any {
	return map[string]any{
		"github.copilot.chat.otel.enabled":        true,
		"github.copilot.chat.otel.exporterType":   "otlp-http",
		"github.copilot.chat.otel.otlpEndpoint":   "http://127.0.0.1:4318",
		"github.copilot.chat.otel.captureContent": false,
	}
}

func jsonSettingsContain(path string, desired map[string]any) bool {
	settings, _, _, err := readVSCodeSettings(path)
	if err != nil {
		return false
	}
	for key, value := range desired {
		if !reflect.DeepEqual(settings[key], value) {
			return false
		}
	}
	return true
}
