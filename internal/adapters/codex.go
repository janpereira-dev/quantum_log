package adapters

import "context"

type codexAdapter struct{ commandAdapter }

func newCodexAdapter() codexAdapter {
	return codexAdapter{commandAdapter: newCommandAdapter("codex", "Codex", "codex", ".codex/AGENTS.md")}
}

func (a codexAdapter) Descriptor() Descriptor {
	return Descriptor{
		ID:      "codex",
		Name:    "Codex",
		Version: "app-server-raw-events",
		Stable:  true,
	}
}

func (a codexAdapter) PlanInstall(ctx context.Context, options SetupOptions) (SetupPlan, error) {
	plan, err := a.commandAdapter.PlanInstall(ctx, options)
	if err != nil {
		return SetupPlan{}, err
	}
	plan.CaptureQuality = CaptureUnavailable
	plan.Notes = []string{"no documented collector forwarding integration recorded"}
	return plan, nil
}

func (a codexAdapter) Status(ctx context.Context) (SetupStatus, error) {
	status, err := a.commandAdapter.Status(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	status.CaptureQuality = CaptureUnavailable
	status.Notes = []string{"no documented collector forwarding integration recorded"}
	return status, nil
}

func (a codexAdapter) Test(ctx context.Context) (TestResult, error) {
	result, err := a.commandAdapter.Test(ctx)
	if err != nil {
		return TestResult{}, err
	}
	result.CaptureQuality = CaptureUnavailable
	result.Message += "; no documented collector forwarding integration recorded"
	return result, nil
}
