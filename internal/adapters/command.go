package adapters

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

// commandAdapter deliberately remains detection-only until a supported hook/plugin
// can be installed without guessing metrics or changing user configuration silently.
type commandAdapter struct {
	id         string
	name       string
	executable string
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
	action := "no files changed: minimal adapter is detection-only"
	if options.DryRun {
		action = "dry run: " + action
	}
	return InstallResult{Actions: []string{action}}, nil
}

func (a commandAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	action := "no files changed: minimal adapter is detection-only"
	if options.DryRun {
		action = "dry run: " + action
	}
	return SetupPlan{AdapterID: a.id, State: SetupPartial, CaptureQuality: CaptureLifecycleOnly, Changes: []SetupChange{{Action: "unchanged", Description: action}}, Notes: []string{"token capture is not verified for this adapter yet"}}, nil
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
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: false, State: state, CaptureQuality: CaptureLifecycleOnly, Evidence: detection.Evidence, Notes: []string{"detection-only; no token capture installed"}}, nil
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
	action := "no files changed: minimal adapter owns no hook or plugin"
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
