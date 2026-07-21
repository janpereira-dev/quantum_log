package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type openCodeAdapter struct {
	commandAdapter
}

func newOpenCodeAdapter() openCodeAdapter {
	return openCodeAdapter{commandAdapter: newCommandAdapter("opencode", "OpenCode", "opencode", ".config/opencode/plugins/quantum-log.ts")}
}

func (a openCodeAdapter) Descriptor() Descriptor {
	return Descriptor{ID: a.id, Name: a.name, Version: "plugin", Capabilities: Capabilities{ModelIdentity: true, InputTokens: true, OutputTokens: true, ToolCalls: true, SessionLifecycle: true, ProjectIdentity: true, WorkingDirectory: true, VCSContext: true, WorkspaceContext: true, StructuredEvents: true}}
}

func (a openCodeAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyManagedFile(a.pluginPath(), openCodePluginSource(), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}}, nil
}

func (a openCodeAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyManagedFile(a.pluginPath(), openCodePluginSource(), true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureAgentReported, Changes: []SetupChange{change}, Notes: []string{"installs a global OpenCode TypeScript plugin that posts sanitized session/message/tool events to qlog localhost collector"}}, nil
}

func (a openCodeAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.commandAdapter.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := fileContains(a.pluginPath(), "QUANTUM_LOG OpenCode passive capture")
	state := SetupUnavailable
	if detection.Available {
		state = SetupAvailable
	}
	if installed {
		state = SetupInstalled
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, CaptureQuality: CaptureAgentReported, Evidence: detection.Evidence, Notes: []string{"OpenCode plugin captures lifecycle/tool events; exact token fields are forwarded only when present in official event payloads"}}, nil
}

func (a openCodeAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Installed, CaptureQuality: CaptureAgentReported, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a openCodeAdapter) pluginPath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, ".config", "opencode", "plugins", "quantum-log.ts")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "opencode", "plugins", "quantum-log.ts")
	}
	return filepath.Join(".config", "opencode", "plugins", "quantum-log.ts")
}

func applyManagedFile(path, content string, dryRun bool) (SetupChange, error) {
	currentBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err == nil && string(currentBytes) == content {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog managed file already up to date"}, nil
	}
	action := "created"
	if err == nil {
		action = "updated"
	}
	if dryRun {
		return SetupChange{Path: path, Action: action, Description: "dry run: qlog managed file would be written"}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, fmt.Errorf("create parent directory: %w", err)
	}
	change := SetupChange{Path: path, Action: action, Description: "qlog managed file written"}
	if err == nil {
		backupPath := fmt.Sprintf("%s.qlog-backup-%s", path, time.Now().UTC().Format("20060102150405"))
		if err := os.WriteFile(backupPath, currentBytes, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("write backup: %w", err)
		}
		change.BackupPath = backupPath
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write %s: %w", path, err)
	}
	return change, nil
}

func fileContains(path, needle string) bool {
	contents, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(contents), needle)
}

func openCodePluginSource() string {
	return `// QUANTUM_LOG OpenCode passive capture
// Managed by qlog setup opencode. Do not store prompts, responses, tool args, or tool results.

const endpoint = process.env.QLOG_COLLECTOR_URL || "http://127.0.0.1:4318/v1/events"

async function post(event) {
  try {
    await fetch(endpoint, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(event),
    })
  } catch {
    // qlog must never break the agent workflow.
  }
}

function base(type, ctx, event) {
  const body = event || {}
  const session = body.session || body.properties || body
  return {
    source: "opencode-plugin",
    session_id: session.id || body.sessionID || body.sessionId || "",
    event_type: type,
    occurred_at: new Date().toISOString(),
    project_hint: {
      project: ctx.project?.name || "",
      cwd: ctx.directory || ctx.worktree || "",
    },
    payload: {
      agent_name: "opencode",
      provider: body.provider || body.model?.provider || "",
      model: body.model?.id || body.model || "",
      input_tokens: body.usage?.input_tokens || body.usage?.inputTokens || 0,
      output_tokens: body.usage?.output_tokens || body.usage?.outputTokens || 0,
      reasoning_tokens: body.usage?.reasoning_tokens || body.usage?.reasoningTokens || 0,
      cached_input_tokens: body.usage?.cached_input_tokens || body.usage?.cachedInputTokens || 0,
      cache_write_tokens: body.usage?.cache_write_tokens || body.usage?.cacheWriteTokens || 0,
      capture_quality: body.usage ? "agent_reported" : "lifecycle_only",
    },
  }
}

export const QuantumLogPlugin = async (ctx) => ({
  event: async ({ event }) => {
    if (["session.created", "message.updated", "session.idle", "session.error"].includes(event.type)) {
      await post(base(event.type, ctx, event))
    }
  },
  "tool.execute.before": async (input) => {
    await post(base("tool.execute.before", ctx, input))
  },
  "tool.execute.after": async (input) => {
    await post(base("tool.execute.after", ctx, input))
  },
})
`
}
