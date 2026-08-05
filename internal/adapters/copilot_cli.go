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
	return Descriptor{ID: a.id, Name: a.name, Version: "hooks-v1", Stable: true, Capabilities: Capabilities{ToolCalls: true, SessionLifecycle: true, ProjectIdentity: true, WorkingDirectory: true, StructuredEvents: true}}
}

func (a copilotCLIAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyManagedFile(a.hooksPath(), copilotCLIHooksConfig(options.Home, options.ExecutablePath), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a copilotCLIAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyManagedFile(a.hooksPath(), copilotCLIHooksConfig(options.Home, ""), true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureLifecycleOnly, Changes: []SetupChange{change}, Notes: []string{"installs an isolated Copilot CLI user hook config for sanitized local lifecycle events; token usage is unavailable"}}, nil
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
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: CaptureLifecycleOnly, Evidence: detection.Evidence, Notes: []string{"Copilot CLI hook capture records lifecycle evidence only; prompts, responses, tool arguments, results, secrets, authorization, and tokens are excluded"}}, nil
}

func (a copilotCLIAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Available && status.Installed, CaptureQuality: CaptureLifecycleOnly, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a copilotCLIAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	path := a.hooksPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		change := SetupChange{Path: path, Action: "unchanged", Description: "Copilot CLI qlog hook config already absent"}
		return InstallResult{Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
	} else if err != nil {
		return InstallResult{}, fmt.Errorf("stat Copilot CLI hook config: %w", err)
	}
	change := SetupChange{Path: path, Action: "removed", Description: "removed qlog-owned Copilot CLI hook config"}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
		return InstallResult{Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
	}
	if err := os.Remove(path); err != nil {
		return InstallResult{}, fmt.Errorf("remove Copilot CLI hook config: %w", err)
	}
	return InstallResult{Changed: true, Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
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
