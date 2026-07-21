package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type claudeCodeAdapter struct{}

func newClaudeCodeAdapter() claudeCodeAdapter { return claudeCodeAdapter{} }

func (claudeCodeAdapter) Descriptor() Descriptor {
	return Descriptor{ID: "claude-code", Name: "Claude Code", Version: "hooks", Capabilities: Capabilities{SessionLifecycle: true, ProjectIdentity: true, WorkingDirectory: true, StructuredEvents: true}}
}

func (claudeCodeAdapter) Detect(context.Context) (Detection, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return Detection{Evidence: "claude not found on PATH"}, nil
	}
	return Detection{Available: true, Evidence: path}, nil
}

func (a claudeCodeAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := a.applySettings(options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a claudeCodeAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := a.applySettings(true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: "claude-code", State: SetupPartial, CaptureQuality: CaptureLifecycleOnly, Changes: []SetupChange{change}, Notes: []string{"installs Claude Code lifecycle hooks that call qlog hook claude-code; hooks do not expose exact token usage"}}, nil
}

func (a claudeCodeAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := claudeSettingsHasQlog(a.settingsPath())
	state := SetupUnavailable
	if detection.Available || installed {
		state = SetupPartial
	}
	return SetupStatus{AdapterID: "claude-code", Available: detection.Available, Installed: installed, State: state, CaptureQuality: CaptureLifecycleOnly, Evidence: detection.Evidence, Notes: []string{"Claude Code hook capture is lifecycle-only until official token usage is available"}}, nil
}

func (a claudeCodeAdapter) Test(ctx context.Context) (TestResult, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return TestResult{}, err
	}
	message := detection.Evidence
	if !detection.Available {
		message = "adapter unavailable: " + detection.Evidence
	}
	return TestResult{AdapterID: "claude-code", Passed: detection.Available, CaptureQuality: CaptureLifecycleOnly, Message: message, TestedAt: time.Now().UTC()}, nil
}

func (claudeCodeAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	action := "no files changed: Claude Code hook removal is not implemented yet"
	if options.DryRun {
		action = "dry run: " + action
	}
	return InstallResult{Actions: []string{action}}, nil
}

func (a claudeCodeAdapter) HealthCheck(ctx context.Context) error {
	detection, err := a.Detect(ctx)
	if err != nil {
		return err
	}
	if !detection.Available {
		return errors.New(detection.Evidence)
	}
	return nil
}

func (claudeCodeAdapter) Ingest(context.Context, io.Reader) ([]RawRecord, error) {
	return nil, errors.New("claude code hooks post directly to qlog /v1/events")
}

func (claudeCodeAdapter) Normalize(record RawRecord) (RawRecord, error) { return record, nil }

func (claudeCodeAdapter) ExtractProjectSignals(RawRecord) ProjectSignals { return ProjectSignals{} }

func (a claudeCodeAdapter) applySettings(dryRun bool) (SetupChange, error) {
	path := a.settingsPath()
	current, _ := os.ReadFile(path)
	next, err := claudeSettingsWithQlogHooks(current)
	if err != nil {
		return SetupChange{}, err
	}
	action := "created"
	if len(current) > 0 {
		action = "updated"
	}
	if string(current) == string(next) {
		action = "unchanged"
	}
	change := SetupChange{Path: path, Action: action, Description: "Claude Code lifecycle hooks call qlog hook claude-code"}
	if dryRun || action == "unchanged" {
		return change, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, err
	}
	if len(current) > 0 {
		backup := path + ".qlog-backup-" + time.Now().UTC().Format("20060102150405")
		if err := os.WriteFile(backup, current, 0o600); err != nil {
			return SetupChange{}, err
		}
		change.BackupPath = backup
	}
	return change, os.WriteFile(path, next, 0o600)
}

func (claudeCodeAdapter) settingsPath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, ".claude", "settings.json")
	}
	if dir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(dir, ".claude", "settings.json")
	}
	return filepath.Join(".claude", "settings.json")
}

func claudeSettingsHasQlog(path string) bool {
	contents, err := os.ReadFile(path)
	return err == nil && bytesContains(contents, []byte("qlog hook claude-code"))
}

func claudeSettingsWithQlogHooks(current []byte) ([]byte, error) {
	settings := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &settings); err != nil {
			return nil, err
		}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	entry := []map[string]any{{"hooks": []map[string]any{{"type": "command", "command": "qlog hook claude-code"}}}}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SubagentStop"} {
		hooks[event] = entry
	}
	settings["hooks"] = hooks
	return json.MarshalIndent(settings, "", "  ")
}

func bytesContains(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
