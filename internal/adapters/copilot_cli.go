package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type copilotCLIUserEnvironmentStore interface {
	Get(string) (string, bool, error)
	Set(string, string) error
	Delete(string) error
}

var copilotCLIUserEnvironment copilotCLIUserEnvironmentStore = newCopilotCLIUserEnvironmentStore()
var copilotCLIUserEnvironmentChanged = notifyCopilotCLIUserEnvironment

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
	changes := make([]SetupChange, 0, 3)
	if runtime.GOOS == "windows" {
		profileChange, err := a.installWindowsPowerShellProfile(options.DryRun)
		if err != nil {
			return InstallResult{}, err
		}
		changes = append(changes, profileChange)
		legacyChange, err := a.cleanupLegacyWindowsUserEnvironment(options.DryRun)
		if err != nil {
			return InstallResult{}, err
		}
		changes = append(changes, legacyChange)
	} else {
		profileChange, err := a.installPosixProfile(options.DryRun)
		if err != nil {
			return InstallResult{}, err
		}
		changes = append(changes, profileChange)
	}
	change, err := applyManagedFile(a.hooksPath(), copilotCLIHooksConfig(options.Home, options.ExecutablePath), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	changes = append(changes, change)
	actions := make([]string, 0, len(changes))
	changed := false
	for _, item := range changes {
		actions = append(actions, formatChange(item))
		changed = changed || item.Action == "created" || item.Action == "updated"
	}
	changed = changed && !options.DryRun
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
	changes := []SetupChange{change}
	notes := []string{"installs prompt, lifecycle, tool, and subagent hooks plus persistent qlog-owned Copilot CLI OTel configuration"}
	if runtime.GOOS == "windows" {
		changes = append(changes, SetupChange{Path: "PowerShell CurrentUserCurrentHost profile", Action: "updated", Description: "adds a qlog-owned Copilot-only OTel launcher function"})
		notes[0] = "installs lifecycle hooks plus a qlog-owned Windows PowerShell Copilot-only OTel launcher"
	} else {
		changes = append(changes, SetupChange{Path: a.posixProfilePath(), Action: "updated", Description: "adds a qlog-owned Copilot-only OTel shell function"})
		notes[0] = "installs lifecycle hooks plus a qlog-owned shell Copilot-only OTel launcher"
	}
	notes = append(notes, "OTel content capture remains disabled; clean-device source evidence is still required")
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureOTELReported, Changes: changes, Notes: notes}, nil
}

func (a copilotCLIAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := fileContains(a.hooksPath(), "hook copilot-cli --event")
	if runtime.GOOS == "windows" {
		installed = installed && a.windowsPowerShellProfileInstalled()
	} else {
		installed = installed && a.posixProfileInstalled()
	}
	state := SetupUnavailable
	if detection.Available {
		state = SetupAvailable
	}
	if installed {
		state = SetupInstalled
	}
	quality := CaptureLifecycleOnly
	notes := []string{"Copilot CLI hooks retain lifecycle and CWD evidence; qlog-owned OTel configuration disables message content capture", "No source E2E evidence is claimed by setup"}
	if installed && runtime.GOOS == "windows" {
		quality = CaptureOTELReported
	}
	if installed && runtime.GOOS != "windows" {
		state = SetupPartial
		notes = append(notes, "POSIX shell profiles instrument interactive bash/zsh launches only; non-interactive launches remain lifecycle-only")
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: quality, Evidence: detection.Evidence, Notes: notes}, nil
}

func (a copilotCLIAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Available && status.Installed, CaptureQuality: CaptureLifecycleOnly, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a copilotCLIAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	changes := make([]SetupChange, 0, 4)
	for _, item := range []struct{ path, description string }{{a.hooksPath(), "Copilot CLI qlog hook config"}} {
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
	if runtime.GOOS == "windows" {
		profileChange, err := a.uninstallWindowsPowerShellProfile(options.DryRun)
		if err != nil {
			return InstallResult{}, err
		}
		changes = append(changes, profileChange)
		legacyChange, err := a.cleanupLegacyWindowsUserEnvironment(options.DryRun)
		if err != nil {
			return InstallResult{}, err
		}
		changes = append(changes, legacyChange)
	} else {
		profileChange, err := a.uninstallPosixProfile(options.DryRun)
		if err != nil {
			return InstallResult{}, err
		}
		changes = append(changes, profileChange)
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

func (a copilotCLIAdapter) windowsPowerShellProfileStatePath() string {
	return filepath.Join(filepath.Dir(a.hooksPath()), "qlog-copilot-otel-profile")
}

func (a copilotCLIAdapter) windowsUserEnvironmentStatePath() string {
	return filepath.Join(filepath.Dir(a.hooksPath()), "qlog-copilot-otel-user-env")
}

func copilotCLIHooksConfig(home, executablePath string) string {
	command := copilotCLIHookCommand(home, executablePath)
	powershell := copilotCLIPowerShellHookCommand(home, executablePath)
	return fmt.Sprintf(`{
  "version": 1,
  "hooks": {
    "sessionStart": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "sessionEnd": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "userPromptSubmitted": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "agentStop": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "errorOccurred": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "preToolUse": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "postToolUse": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "subagentStart": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}],
    "subagentStop": [{"type":"command","command":%q,"bash":%q,"powershell":%q,"timeoutSec":5}]
  }
}
`, copilotCLIGenericHookCommand(home, executablePath, "sessionStart"), command+" --event sessionStart", powershell+" --event sessionStart", copilotCLIGenericHookCommand(home, executablePath, "sessionEnd"), command+" --event sessionEnd", powershell+" --event sessionEnd", copilotCLIGenericHookCommand(home, executablePath, "userPromptSubmitted"), command+" --event userPromptSubmitted", powershell+" --event userPromptSubmitted", copilotCLIGenericHookCommand(home, executablePath, "agentStop"), command+" --event agentStop", powershell+" --event agentStop", copilotCLIGenericHookCommand(home, executablePath, "errorOccurred"), command+" --event errorOccurred", powershell+" --event errorOccurred", copilotCLIGenericHookCommand(home, executablePath, "preToolUse"), command+" --event preToolUse", powershell+" --event preToolUse", copilotCLIGenericHookCommand(home, executablePath, "postToolUse"), command+" --event postToolUse", powershell+" --event postToolUse", copilotCLIGenericHookCommand(home, executablePath, "subagentStart"), command+" --event subagentStart", powershell+" --event subagentStart", copilotCLIGenericHookCommand(home, executablePath, "subagentStop"), command+" --event subagentStop", powershell+" --event subagentStop")
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
