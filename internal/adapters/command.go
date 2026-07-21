package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// commandAdapter deliberately remains detection-only until a supported hook/plugin
// can be installed without guessing metrics or changing user configuration silently.
type commandAdapter struct {
	id                 string
	name               string
	executable         string
	configRelativePath string
	marker             string
	setupText          string
}

func newCommandAdapter(id, name, executable, configRelativePath string) commandAdapter {
	return commandAdapter{id: id, name: name, executable: executable, configRelativePath: configRelativePath, marker: "agent-auto-capture", setupText: setupInstructions(id, name)}
}

func (a commandAdapter) Descriptor() Descriptor {
	return Descriptor{ID: a.id, Name: a.name, Version: "minimal", Capabilities: Capabilities{}}
}

func (a commandAdapter) Detect(context.Context) (Detection, error) {
	path, err := exec.LookPath(a.executable)
	if err != nil {
		return Detection{Evidence: a.executable + " not found on PATH"}, nil
	}
	return Detection{Available: true, Evidence: path}, nil
}

func (a commandAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := ApplyMarkerBlock(a.configPath(), a.marker, a.setupText, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}}, nil
}

func (a commandAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := ApplyMarkerBlock(a.configPath(), a.marker, a.setupText, true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupPartial, CaptureQuality: CaptureLifecycleOnly, Changes: []SetupChange{change}, Notes: []string{"setup installs qlog-owned instructions; token capture remains lifecycle-only until the agent exposes reported usage"}}, nil
}

func (a commandAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	state := SetupUnavailable
	if detection.Available {
		state = SetupPartial
	}
	installed := HasMarkerBlock(a.configPath(), a.marker)
	if installed {
		state = SetupPartial
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, CaptureQuality: CaptureLifecycleOnly, Evidence: detection.Evidence, Notes: []string{"setup instructions installed; token capture depends on agent-reported data source"}}, nil
}

func (a commandAdapter) Test(ctx context.Context) (TestResult, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return TestResult{}, err
	}
	message := detection.Evidence
	if !detection.Available {
		message = "adapter unavailable: " + detection.Evidence
	}
	return TestResult{AdapterID: a.id, Passed: detection.Available, CaptureQuality: CaptureLifecycleOnly, Message: message, TestedAt: time.Now().UTC()}, nil
}

func (a commandAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	action := "no files changed: setup marker removal is not implemented yet"
	if options.DryRun {
		action = "dry run: " + action
	}
	return InstallResult{Actions: []string{action}}, nil
}

func (a commandAdapter) HealthCheck(ctx context.Context) error {
	detection, err := a.Detect(ctx)
	if err != nil {
		return err
	}
	if !detection.Available {
		return errors.New(detection.Evidence)
	}
	return nil
}

func (commandAdapter) Ingest(context.Context, io.Reader) ([]RawRecord, error) {
	return nil, errors.New("minimal adapter does not ingest events")
}

func (commandAdapter) Normalize(record RawRecord) (RawRecord, error) { return record, nil }

func (commandAdapter) ExtractProjectSignals(RawRecord) ProjectSignals { return ProjectSignals{} }

func (a commandAdapter) configPath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, filepath.FromSlash(a.configRelativePath))
	}
	if dir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(dir, filepath.FromSlash(a.configRelativePath))
	}
	return filepath.FromSlash(a.configRelativePath)
}

func setupInstructions(id, name string) string {
	return fmt.Sprintf(`## QUANTUM_LOG Agent Auto-Capture

This %s setup is managed by qlog for adapter %q.

- Prefer provider-reported or agent-reported token usage when this agent exposes it.
- If real token data is unavailable, record lifecycle/session evidence only and label it capture_quality=lifecycle_only.
- Never persist prompts, responses, tool arguments, tool results, environment variables, API keys, cookies, tokens, or authorization headers.
- Use qlog's central project resolver; do not infer project ownership from provider, model, or agent name.
`, name, id)
}

func formatChange(change SetupChange) string {
	prefix := ""
	if change.Description != "" {
		prefix = change.Description + ": "
	}
	if change.BackupPath == "" {
		return fmt.Sprintf("%s%s: %s", prefix, change.Action, change.Path)
	}
	return fmt.Sprintf("%s%s: %s (backup: %s)", prefix, change.Action, change.Path, change.BackupPath)
}
