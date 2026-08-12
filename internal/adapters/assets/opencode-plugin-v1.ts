// QUANTUM_LOG OpenCode passive capture
// Managed by qlog. Responses, tool inputs, and tool outputs never leave OpenCode.

const endpoint = process.env.QLOG_COLLECTOR_URL || "http://127.0.0.1:4318/v1/events"
const localCollector = /^https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\])(?::\d+)?(?:\/|$)/i.test(endpoint)

async function post(event: unknown) {
  try {
    await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(event) })
  } catch {
    // Capture must not affect OpenCode execution.
  }
}

function text(value: unknown): string | undefined { return typeof value === "string" ? value : undefined }
function numberValue(value: unknown): number | undefined { return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : undefined }

function envelope(type: string, ctx: any, event: any, payload: Record<string, unknown>, upstreamEventID: string) {
  const properties = event?.properties || {}
  const context = event?.context || {}
  return {
    source: "opencode-plugin",
    session_id: properties.sessionID || properties.info?.sessionID || properties.part?.sessionID || event?.sessionID || "",
    event_type: type,
    occurred_at: new Date(event?.time || Date.now()).toISOString(),
    upstream_event_id: upstreamEventID,
    project_hint: { cwd: ctx.directory || ctx.worktree || context.directory || context.worktree || "" },
    payload,
  }
}

export const QuantumLogPlugin = async (ctx: any) => {
  // OpenCode's tool callbacks identify their session, not their parent message.
  // Keep the latest user-message identity per session only in plugin memory.
  // It is sent as metadata and never persisted as prompt text by qlog.
  const activeInteractions = new Map<string, string>()
  const toolInteractions = new Map<string, string>()
  const toolSession = (input: any) => text(input?.sessionID || input?.session?.id) || ""
  const toolPayload = (input: any, interactionID: string) => ({ agent_name: "opencode", capture_quality: "lifecycle_only", interaction_upstream_id: interactionID, tool_name: text(input?.tool) || text(input?.name) || text(input?.toolName) || "unknown" })
  return {
  event: async ({ event }: any) => {
    const info = event?.properties?.info
    if (event?.type === "message.updated" && info?.role === "user" && text(info.id)) {
      const interactionID = `message:${info.id}`
      const prompt = text(info.text || info.content) || ""
      // qlog enforces prompt-capture at its persistence boundary and derives
      // the installation-local HMAC. This local payload is never persisted raw.
      const payload: Record<string, unknown> = { agent_name: "opencode", capture_quality: "lifecycle_only", prompt_available: localCollector }
      // Raw text may only cross the plugin boundary to qlog's loopback collector.
      // A custom URL can be remote, so it receives no prompt body in any mode.
      if (localCollector) payload.prompt = prompt
      const sessionID = text(info.sessionID)
      if (sessionID) activeInteractions.set(sessionID, interactionID)
      await post(envelope("interaction.prompt", ctx, event, payload, interactionID))
      return
    }
    if (event?.type === "message.updated" && info?.role === "assistant" && text(info.id)) {
      if (numberValue(info.time && info.time.completed) === undefined) return
      const tokens = info.tokens || {}
      const cache = tokens.cache || {}
	  const sessionID = info.sessionID
      const payload: Record<string, unknown> = { agent_name: "opencode", capture_quality: "agent_reported", interaction_upstream_id: info.parentID ? `message:${info.parentID}` : "" }
      for (const [name, value] of [["provider", info.providerID], ["model", info.modelID], ["message_id", info.id], ["finish", info.finish]]) {
        const next = text(value); if (next) payload[name] = next
      }
      for (const [name, value] of [["input_tokens", tokens.input], ["output_tokens", tokens.output], ["reasoning_tokens", tokens.reasoning], ["cached_input_tokens", cache.read], ["cache_write_tokens", cache.write]]) {
        const next = numberValue(value); if (next !== undefined) payload[name] = next
      }
	  const cost = numberValue(info.cost); if (cost !== undefined) payload.estimated_cost_usd_micros = Math.round(cost * 1000000)
	  const created = numberValue(info.time && info.time.created); if (created !== undefined) payload.created_at = created
	  const completed = numberValue(info.time && info.time.completed); if (completed !== undefined) payload.completed_at = completed
	  payload.metric_observations = metricObservations(tokens, cache)
      await post(envelope("model.call", ctx, event, payload, `message:${info.id}`))
	  if (sessionID && activeInteractions.get(sessionID) === payload.interaction_upstream_id) activeInteractions.delete(sessionID)
      return
    }
	if (event?.type === "message.part.updated" && event?.properties?.part?.type === "step-finish") {
	  const part = event.properties.part
	  const sessionID = part.sessionID
	  await post(envelope("agent.event", ctx, event, { agent_name: "opencode", capture_quality: "lifecycle_only" }, `part:${part.sessionID || "unknown"}:${part.id}`))
	  return
	}
	if (["session.created", "session.idle", "session.error"].includes(event?.type)) {
	  const sessionID = event?.properties?.info?.id || event?.properties?.sessionID || event?.properties?.info?.sessionID || "unknown"
	  await post(envelope("agent.event", ctx, event, { agent_name: "opencode", capture_quality: "lifecycle_only" }, `session:${sessionID}:${event.type}`))
	}
  },
  "tool.execute.before": async (input: any) => {
    const callID = text(input?.callID || input?.id) || "unknown"
    const interactionID = activeInteractions.get(toolSession(input)) || ""
    toolInteractions.set(callID, interactionID)
    await post(envelope("tool.execute.before", ctx, input, toolPayload(input, interactionID), `tool.before:${callID}`))
  },
  "tool.execute.after": async (input: any) => {
    const callID = text(input?.callID || input?.id) || "unknown"
    const interactionID = toolInteractions.get(callID) || activeInteractions.get(toolSession(input)) || ""
    try { await post(envelope("tool.execute.after", ctx, input, toolPayload(input, interactionID), `tool.after:${callID}`)) } finally { toolInteractions.delete(callID) }
  },
  }
}

function metricObservations(tokens: any, cache: any) {
  const observations = []
  for (const [name, rawKey, value] of [["input_tokens", "tokens.input", tokens.input], ["output_tokens", "tokens.output", tokens.output], ["reasoning_tokens", "tokens.reasoning", tokens.reasoning], ["cached_input_tokens", "cache.read", cache.read], ["cache_write_tokens", "cache.write", cache.write]]) {
    const next = numberValue(value)
    if (next !== undefined) observations.push({ name, value: next, source: "opencode", raw_key: rawKey, confidence: "reported" })
  }
  return observations
}
