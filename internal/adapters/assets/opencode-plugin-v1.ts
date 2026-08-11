// QUANTUM_LOG OpenCode passive capture
// Managed by qlog. Prompt bodies, responses, tool inputs, and tool outputs never leave OpenCode.

const endpoint = process.env.QLOG_COLLECTOR_URL || "http://127.0.0.1:4318/v1/events"

async function post(event: unknown) {
  try {
    await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(event) })
  } catch {
    // Capture must not affect OpenCode execution.
  }
}

function text(value: unknown): string | undefined { return typeof value === "string" ? value : undefined }
function numberValue(value: unknown): number | undefined { return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : undefined }

async function promptHash(value: unknown): Promise<string> {
  if (typeof value !== "string" || !value) return ""
  const bytes = new TextEncoder().encode(value)
  const digest = await crypto.subtle.digest("SHA-256", bytes)
  return Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, "0")).join("")
}

function envelope(type: string, ctx: any, event: any, payload: Record<string, unknown>, upstreamEventID: string) {
  const properties = event?.properties || {}
  const context = event?.context || {}
  return {
    source: "opencode-plugin",
    session_id: properties.sessionID || properties.info?.sessionID || properties.part?.sessionID || "",
    event_type: type,
    occurred_at: new Date(event?.time || Date.now()).toISOString(),
    upstream_event_id: upstreamEventID,
    project_hint: { cwd: ctx.directory || ctx.worktree || context.directory || context.worktree || "" },
    payload,
  }
}

export const QuantumLogPlugin = async (ctx: any) => ({
  event: async ({ event }: any) => {
    const info = event?.properties?.info
    if (event?.type === "message.updated" && info?.role === "user" && text(info.id)) {
      // Only hash user text locally. Hash identity permits dedup without transcript retention.
      const hash = await promptHash(info.text || info.content)
      await post(envelope("interaction.prompt", ctx, event, { agent_name: "opencode", capture_quality: "lifecycle_only", prompt_hash: hash }, `message:${info.id}`))
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
      return
    }
	if (event?.type === "message.part.updated" && event?.properties?.part?.type === "step-finish") {
	  const part = event.properties.part
	  const sessionID = part.sessionID
	  await post(envelope("agent.event", ctx, event, { agent_name: "opencode", capture_quality: "lifecycle_only" }, `part:${event.properties.part.id}`))
	  return
	}
    if (["session.created", "session.idle", "session.error"].includes(event?.type)) await post(envelope("agent.event", ctx, event, { agent_name: "opencode", capture_quality: "lifecycle_only" }, event?.id || event?.type))
  },
  "tool.execute.before": async (input: any) => post(envelope("tool.execute.before", ctx, input, { agent_name: "opencode", capture_quality: "lifecycle_only", interaction_upstream_id: input?.sessionID ? `active:${input.sessionID}` : "" }, input?.id || "tool.before")),
  "tool.execute.after": async (input: any) => post(envelope("tool.execute.after", ctx, input, { agent_name: "opencode", capture_quality: "lifecycle_only", interaction_upstream_id: input?.sessionID ? `active:${input.sessionID}` : "" }, input?.id || "tool.after")),
})

function metricObservations(tokens: any, cache: any) {
  const observations = []
  for (const [name, rawKey, value] of [["input_tokens", "tokens.input", tokens.input], ["output_tokens", "tokens.output", tokens.output], ["reasoning_tokens", "tokens.reasoning", tokens.reasoning], ["cached_input_tokens", "cache.read", cache.read], ["cache_write_tokens", "cache.write", cache.write]]) {
    const next = numberValue(value)
    if (next !== undefined) observations.push({ name, value: next, source: "opencode", raw_key: rawKey, confidence: "reported" })
  }
  return observations
}
