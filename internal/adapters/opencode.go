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
	return Descriptor{ID: a.id, Name: a.name, Version: "plugin", Stable: true, Capabilities: Capabilities{ModelIdentity: true, InputTokens: true, OutputTokens: true, ReasoningTokens: true, CacheTokens: true, Costs: true, ToolCalls: true, SessionLifecycle: true, ProjectIdentity: true, WorkingDirectory: true, VCSContext: true, WorkspaceContext: true, StructuredEvents: true}}
}

func (a openCodeAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyManagedFile(a.pluginPath(), openCodePluginSource(), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a openCodeAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyManagedFile(a.pluginPath(), openCodePluginSource(), true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureAgentReported, Changes: []SetupChange{change}, Notes: []string{"installs a global OpenCode TypeScript plugin that posts allowlisted assistant usage plus sanitized lifecycle/tool events to qlog localhost collector"}}, nil
}

func (a openCodeAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
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
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: CaptureAgentReported, Evidence: detection.Evidence, Notes: []string{"OpenCode 1.18.x plugin captures allowlisted assistant-reported usage; step-finish events corroborate completion without creating a second model call"}}, nil
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
// Managed by qlog setup opencode. Do not store prompts, responses, reasoning, tool args, or tool results.

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

function envelope(type, ctx, event, payload, upstreamEventID) {
	const body = event || {}
	const properties = body.properties || {}
	const context = body.context || {}
	const info = properties.info || {}
	const part = properties.part || {}
	return {
	  source: "opencode-plugin",
	  session_id: properties.sessionID || info.sessionID || part.sessionID || "",
    event_type: type,
    occurred_at: new Date(body.time || Date.now()).toISOString(),
    upstream_event_id: upstreamEventID || body.id || "",
    project_hint: {
      project: "",
      cwd: ctx.directory || ctx.worktree || context.directory || context.worktree || "",
    },
    payload,
  }
}

function stringValue(value) {
  return typeof value === "string" ? value : undefined
}

function numberValue(value) {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : undefined
}

function setString(target, key, value) {
  const next = stringValue(value)
  if (next !== undefined) target[key] = next
}

function setNumber(target, key, value) {
  const next = numberValue(value)
  if (next !== undefined) target[key] = next
}

function metricObservations(tokens, cache) {
  const observations = []
  for (const [name, rawKey, value] of [
    ["input_tokens", "tokens.input", tokens.input],
    ["output_tokens", "tokens.output", tokens.output],
    ["reasoning_tokens", "tokens.reasoning", tokens.reasoning],
    ["cached_input_tokens", "tokens.cache.read", cache.read],
    ["cache_write_tokens", "tokens.cache.write", cache.write],
  ]) {
    const number = numberValue(value)
    if (number !== undefined) observations.push({ name, value: number, source: "opencode", raw_key: rawKey, confidence: "reported" })
  }
  return observations
}

function lifecyclePayload() {
  return { agent_name: "opencode", capture_quality: "lifecycle_only" }
}

function assistantUsage(ctx, event) {
  const properties = (event || {}).properties || {}
  const info = properties.info
  if (!info || info.role !== "assistant") return
  if (numberValue(info.time && info.time.completed) === undefined) return

  const tokens = info.tokens || {}
  const cache = tokens.cache || {}
  const payload = { agent_name: "opencode", capture_quality: "agent_reported" }
	setString(payload, "session_id", properties.sessionID || info.sessionID)
  setString(payload, "message_id", info.id)
  setString(payload, "parent_message_id", info.parentID)
  setString(payload, "provider", info.providerID)
  setString(payload, "model", info.modelID)
  setString(payload, "finish", info.finish)
  setNumber(payload, "estimated_cost_usd_micros", numberValue(info.cost) === undefined ? undefined : Math.round(info.cost * 1000000))
  setNumber(payload, "input_tokens", tokens.input)
  setNumber(payload, "output_tokens", tokens.output)
  setNumber(payload, "reasoning_tokens", tokens.reasoning)
  setNumber(payload, "cached_input_tokens", cache.read)
  setNumber(payload, "cache_write_tokens", cache.write)
  setNumber(payload, "created_at", info.time && info.time.created)
  setNumber(payload, "completed_at", info.time && info.time.completed)
  payload.metric_observations = metricObservations(tokens, cache)

  const messageID = stringValue(info.id)
  if (!messageID) return
  return envelope("model.call", ctx, event, payload, "message:" + messageID)
}

function stepFinish(ctx, event) {
  const properties = (event || {}).properties || {}
  const part = properties.part
  if (!part || part.type !== "step-finish") return

  const payload = { agent_name: "opencode", capture_quality: "lifecycle_only" }
  setString(payload, "session_id", properties.sessionID || part.sessionID)
  setString(payload, "message_id", part.messageID)
  setString(payload, "part_id", part.id)
  setString(payload, "finish", part.reason)
  const partID = stringValue(part.id)
  if (!partID) return
  return envelope("agent.event", ctx, event, payload, "part:" + partID)
}

export const QuantumLogPlugin = async (ctx) => ({
  event: async ({ event }) => {
    if (event.type === "message.updated") {
      const usage = assistantUsage(ctx, event)
      if (usage) await post(usage)
      return
    }
    if (event.type === "message.part.updated") {
      const completion = stepFinish(ctx, event)
      if (completion) await post(completion)
      return
    }
    if (["session.created", "session.idle", "session.error"].includes(event.type)) {
      await post(envelope("agent.event", ctx, event, lifecyclePayload()))
    }
  },
  "tool.execute.before": async (input) => {
    await post(envelope("tool.execute.before", ctx, input, lifecyclePayload()))
  },
  "tool.execute.after": async (input) => {
    await post(envelope("tool.execute.after", ctx, input, lifecyclePayload()))
  },
})
`
}
