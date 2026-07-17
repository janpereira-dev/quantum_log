package adapters

import (
	"context"
	"errors"
	"io"
	"os/exec"
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
