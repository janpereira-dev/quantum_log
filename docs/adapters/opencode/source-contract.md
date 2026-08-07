# OpenCode Source Contract

qlog installs a global TypeScript plugin using documented OpenCode plugin events. It posts allowlisted assistant usage plus sanitized local lifecycle and tool event envelopes to qlog.

Source: [OpenCode plugins documentation](https://opencode.ai/docs/plugins) and the OpenCode V1 session event schema. Local inspection used OpenCode CLI `1.18.12`.

## Accepted Fields

`message.updated` is accepted only when `properties.info.role` is `assistant`. qlog allowlists the assistant message ID, parent ID, session ID, provider ID, model ID, cost, token breakdown, created/completed timestamps, finish reason, and safe CWD. `message.part.updated` is accepted only when `properties.part.type` is `step-finish`; it records session/message/part IDs and finish reason as raw corroboration only. Prompts, responses, message parts, reasoning text, tool arguments, tool results, environment values, authorization data, and secrets are excluded.

## Metrics

Assistant messages report `cost` and `tokens.input`, `tokens.output`, `tokens.reasoning`, `tokens.cache.read`, and `tokens.cache.write`; OpenCode capture is `agent_reported`. A message ID is the primary ingestion identity, so repeat `message.updated` callbacks deduplicate. Step-finish never emits a model call and cannot double-count usage. Lifecycle and tool events remain `lifecycle_only`.

The audited 1.18.12 assistant schema has no source-reported `tokens.total` field. qlog derives stored total from reported token components and does not invent or forward a separate total value. Missing counters remain absent; source-reported zero remains zero.

Real OpenCode plugin E2E evidence remains required.
