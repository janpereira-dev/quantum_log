package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// copilotCLIAdapter installs an isolated user hook file. It never edits
// user settings, repository hooks, prompts, or tool payloads.
type copilotCLIAdapter struct{ commandAdapter }

func newCopilotCLIAdapter() copilotCLIAdapter {
	return copilotCLIAdapter{commandAdapter: newCommandAdapter("copilot", "GitHub Copilot CLI", "copilot", ".copilot/hooks/qlog.json")}
}

func (a copilotCLIAdapter) Descriptor() Descriptor {
	return Descriptor{ID: a.id, Name: a.name, Version: "hooks-otel-v1", Stable: true, Capabilities: Capabilities{ModelIdentity: true, InputTokens: true, OutputTokens: true, CacheTokens: true, ToolCalls: true, SessionLifecycle: true, ProjectIdentity: true, WorkingDirectory: true, StructuredEvents: true}}
}

func (a copilotCLIAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyManagedFile(a.hooksPath(), copilotCLIHooksConfig(options.Home, options.ExecutablePath), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	otelChange, err := applyManagedFile(a.otelPath(), copilotCLIOTELConfig("http://127.0.0.1:4318"), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	changes := []SetupChange{change, otelChange}
	actions := []string{formatChange(change), formatChange(otelChange)}
	changed := !options.DryRun && (change.Action == "created" || change.Action == "updated" || otelChange.Action == "created" || otelChange.Action == "updated")
	return InstallResult{Changed: changed, Actions: actions, Changes: changes}, nil
}

func (a copilotCLIAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyManagedFile(a.hooksPath(), copilotCLIHooksConfig(options.Home, ""), true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	otelChange, err := applyManagedFile(a.otelPath(), copilotCLIOTELConfig("http://127.0.0.1:4318"), true)
	if err != nil {
		return SetupPlan{}, err
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureOTELReported, Changes: []SetupChange{change, otelChange}, Notes: []string{"installs lifecycle hooks plus a qlog-owned Copilot CLI OTel environment file; source it before launching copilot", "OTel content capture remains disabled; clean-device source evidence is still required"}}, nil
}

func (a copilotCLIAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := fileContains(a.hooksPath(), "hook copilot-cli --event")
	state := SetupUnavailable
	if detection.Available {
		state = SetupAvailable
	}
	if installed {
		state = SetupInstalled
	}
	quality := CaptureLifecycleOnly
	if fileContains(a.otelPath(), "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false") {
		quality = CaptureOTELReported
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: quality, Evidence: detection.Evidence, Notes: []string{"Copilot CLI hooks retain lifecycle and CWD evidence; qlog-owned OTel configuration disables message content capture", "No source E2E evidence is claimed by setup"}}, nil
}

func (a copilotCLIAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Available && status.Installed, CaptureQuality: CaptureLifecycleOnly, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a copilotCLIAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	changes := make([]SetupChange, 0, 2)
	for _, item := range []struct{ path, description string }{
		{a.hooksPath(), "Copilot CLI qlog hook config"},
		{a.otelPath(), "Copilot CLI qlog OTel environment"},
	} {
		change := SetupChange{Path: item.path, Action: "unchanged", Description: item.description + " already absent"}
		if _, err := os.Stat(item.path); err == nil {
			change.Action = "removed"
			change.Description = "removed qlog-owned " + item.description
			if !options.DryRun {
				if err := os.Remove(item.path); err != nil {
					return InstallResult{}, fmt.Errorf("remove %s: %w", item.description, err)
				}
			}
		} else if !os.IsNotExist(err) {
			return InstallResult{}, fmt.Errorf("stat %s: %w", item.description, err)
		}
		if options.DryRun && change.Action == "removed" {
			change.Description = "dry run: " + change.Description
		}
		changes = append(changes, change)
	}
	actions := make([]string, 0, len(changes))
	changed := false
	for _, change := range changes {
		actions = append(actions, formatChange(change))
		changed = changed || change.Action == "removed"
	}
	return InstallResult{Changed: changed && !options.DryRun, Actions: actions, Changes: changes}, nil
}

func (a copilotCLIAdapter) Ingest(context.Context, io.Reader) ([]RawRecord, error) {
	return nil, errors.New("copilot CLI hooks post directly to qlog /v1/events")
}

func (a copilotCLIAdapter) Normalize(record RawRecord) (RawRecord, error) { return record, nil }

func (a copilotCLIAdapter) ExtractProjectSignals(RawRecord) ProjectSignals { return ProjectSignals{} }

func (a copilotCLIAdapter) hooksPath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, ".copilot", "hooks", "qlog.json")
	}
	if home := os.Getenv("COPILOT_HOME"); home != "" {
		return filepath.Join(home, "hooks", "qlog.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".copilot", "hooks", "qlog.json")
	}
	return filepath.Join(".copilot", "hooks", "qlog.json")
}

func (a copilotCLIAdapter) otelPath() string {
	return filepath.Join(filepath.Dir(a.hooksPath()), "qlog-otel.env")
}

func copilotCLIOTELConfig(endpoint string) string {
	return "COPILOT_OTEL_ENABLED=true\n" +
		"COPILOT_OTEL_EXPORTER_TYPE=otlp-http\n" +
		"OTEL_EXPORTER_OTLP_ENDPOINT=" + endpoint + "\n" +
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/json\n" +
		"OTEL_METRICS_EXPORTER=none\n" +
		"OTEL_LOGS_EXPORTER=none\n" +
		"OTEL_SERVICE_NAME=github-copilot\n" +
		"OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false\n"
}

func copilotCLIHooksConfig(home, executablePath string) string {
	command := copilotCLIHookCommand(home, executablePath)
	powershell := copilotCLIPowerShellHookCommand(home, executablePath)
	return fmt.Sprintf(`{
  "version": 1,
  "hooks": {
    "sessionStart": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "sessionEnd": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "agentStop": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "postToolUse": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}]
  }
}
`, copilotCLIGenericHookCommand(home, executablePath, "sessionStart"), command+" --event sessionStart", powershell+" --event sessionStart", copilotCLIGenericHookCommand(home, executablePath, "sessionEnd"), command+" --event sessionEnd", powershell+" --event sessionEnd", copilotCLIGenericHookCommand(home, executablePath, "agentStop"), command+" --event agentStop", powershell+" --event agentStop", copilotCLIGenericHookCommand(home, executablePath, "postToolUse"), command+" --event postToolUse", powershell+" --event postToolUse")
}

func copilotCLIHookCommand(home, executablePath string) string {
	command := "qlog"
	if strings.TrimSpace(executablePath) != "" {
		command = shellQuote(executablePath)
	}
	if strings.TrimSpace(home) != "" {
		command += " --home " + shellQuote(home)
	}
	return command + " hook copilot-cli"
}

func copilotCLIPowerShellHookCommand(home, executablePath string) string {
	command := "qlog"
	if strings.TrimSpace(executablePath) != "" {
		command = "& " + powerShellQuote(executablePath)
	}
	if strings.TrimSpace(home) != "" {
		command += " --home " + powerShellQuote(home)
	}
	return "$input | " + command + " hook copilot-cli"
}

func copilotCLIGenericHookCommand(home, executablePath, event string) string {
	return "powershell.exe -NoProfile -NonInteractive -Command " + windowsCommandQuote(copilotCLIPowerShellHookCommand(home, executablePath)+" --event "+event)
}

func windowsCommandQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func powerShellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
