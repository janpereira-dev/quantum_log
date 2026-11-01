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
	return Descriptor{ID: a.id, Name: a.name, Version: "otel", Stable: true, Capabilities: Capabilities{ModelIdentity: true, InputTokens: true, OutputTokens: true, ReasoningTokens: true, CacheTokens: true, StructuredEvents: true}}
}

func (a vscodeCopilotAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	if !options.DryRun {
		if err := ensurePrivateAdapterPaths(a.settingsPath(), a.launcherStatePath()); err != nil {
			return InstallResult{}, err
		}
	}
	paths, token, err := collectorCredentials(options.Home, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	if _, err := writeCollectorTokenFile(paths.Home, token, options.DryRun); err != nil {
		return InstallResult{}, err
	}
	if _, err := a.installLauncher(paths, true); err != nil {
		return InstallResult{}, err
	}
	change, err := applyVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	launcher, err := a.installLauncher(paths, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated" || launcher.Action == "created" || launcher.Action == "updated"), Actions: []string{formatChange(change), formatChange(launcher)}, Changes: []SetupChange{change, launcher}}, nil
}

func (a vscodeCopilotAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	paths, _, err := collectorCredentials(options.Home, true)
	if err != nil {
		return SetupPlan{}, err
	}
	change, err := applyVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	launcher, err := a.installLauncher(paths, true)
	if err != nil {
		return SetupPlan{}, err
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureOTELReported, Changes: []SetupChange{change, launcher}, Notes: []string{"close all VS Code windows and relaunch through the generated qlog-code launcher; direct launch has no collector credential", "clean-device real-agent acceptance remains required"}}, nil
}

func (a vscodeCopilotAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := removeVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	launcher, err := a.uninstallLauncher(options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "removed" || launcher.Action == "removed"), Actions: []string{formatChange(change), formatChange(launcher)}, Changes: []SetupChange{change, launcher}}, nil
}

func (a vscodeCopilotAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := jsonSettingsContain(a.settingsPath(), copilotOTelSettings()) && a.launcherInstalled()
	state := SetupUnavailable
	if detection.Available {
		state = SetupAvailable
	}
	if installed {
		state = SetupInstalled
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: CaptureOTELReported, Evidence: detection.Evidence, Notes: []string{"close all VS Code windows and relaunch through the generated qlog-code TLS launcher; direct launch has no collector credential", "clean-device real-agent acceptance remains required"}}, nil
}

func (a vscodeCopilotAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Installed, CaptureQuality: CaptureOTELReported, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a vscodeCopilotAdapter) settingsPath() string {
	return vscodeSettingsPath("Code")
}

func copilotOTelSettings() map[string]any {
	settings := map[string]any{
		"github.copilot.chat.otel.enabled":        true,
		"github.copilot.chat.otel.exporterType":   "otlp-http",
		"github.copilot.chat.otel.otlpEndpoint":   "https://127.0.0.1:4318",
		"github.copilot.chat.otel.captureContent": false,
	}
	settings["github.copilot.chat.otel.headers"] = map[string]any{}
	return settings
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
