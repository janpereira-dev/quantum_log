# Canonical Interaction Ledger

An interaction is one upstream user prompt. It is canonical root evidence, separate from zero or more model calls, tool calls, or MCP calls produced while handling it.

`interactions` has a stable source/session/upstream identity. Re-delivery returns existing root. `model_calls.interaction_id` and `tool_calls.interaction_id` link descendants without turning descendants into prompts. Historical model and tool rows remain valid legacy-unlinked rows after migration.

Raw events remain append-only and hash chained. Prompt bodies, response bodies, tool inputs, and tool outputs are excluded. Prompt capture defaults to `hash`; `off` stores neither value nor hash, while `full` is reserved for explicit locally redacted configuration.

Project resolution creates a local project only for an unambiguous Git root. No Git root means unattributed; provider, model, and agent identity never imply ownership.

## Operational architecture

The ledger is the product core. Hooks, plugins, wrappers, MCP servers, and the
loopback collector are ingestion transports. Sources that push OTLP HTTP currently
require a listening collector; direct hooks and MCP-over-stdio integrations do not
inherently require a permanent process.

See [ADR-005](architecture/ADR-005-collector-lifecycle.md) for the Windows Task
Scheduler policy boundary, cleanup of legacy Startup entries, and the direction
toward explicit collector opt-in.
