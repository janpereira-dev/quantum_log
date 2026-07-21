package adapters

import "context"

type codexAdapter struct{ commandAdapter }

func newCodexAdapter() codexAdapter {
	return codexAdapter{commandAdapter: newCommandAdapter("codex", "Codex", "codex", ".codex/qlog-instructions.md")}
}

func (a codexAdapter) Descriptor() Descriptor {
	return Descriptor{
		ID:      "codex",
		Name:    "Codex",
		Version: "app-server-raw-events",
		Capabilities: Capabilities{
			ModelIdentity:    true,
			InputTokens:      true,
			OutputTokens:     true,
			ReasoningTokens:  true,
			CacheTokens:      true,
			SessionLifecycle: true,
			ProjectIdentity:  true,
			WorkingDirectory: true,
			StructuredEvents: true,
		},
	}
}

func (a codexAdapter) PlanInstall(ctx context.Context, options SetupOptions) (SetupPlan, error) {
	plan, err := a.commandAdapter.PlanInstall(ctx, options)
	if err != nil {
		return SetupPlan{}, err
	}
	plan.CaptureQuality = CaptureAgentReported
	plan.Notes = []string{"Codex app-server rawResponse/completed events can provide exact upstream usage when experimentalRawEvents is enabled; usage may be null and must not be estimated"}
	return plan, nil
}

func (a codexAdapter) Status(ctx context.Context) (SetupStatus, error) {
	status, err := a.commandAdapter.Status(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	status.CaptureQuality = CaptureAgentReported
	status.Notes = []string{"requires Codex app-server rawResponse/completed forwarding to qlog /v1/events with non-null usage"}
	return status, nil
}

func (a codexAdapter) Test(ctx context.Context) (TestResult, error) {
	result, err := a.commandAdapter.Test(ctx)
	if err != nil {
		return TestResult{}, err
	}
	result.CaptureQuality = CaptureAgentReported
	if result.Passed {
		result.Message += "; token capture requires rawResponse/completed usage forwarding"
	}
	return result, nil
}
