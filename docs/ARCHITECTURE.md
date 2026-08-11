# Canonical Interaction Ledger

An interaction is one upstream user prompt. It is canonical root evidence, separate from zero or more model calls, tool calls, or MCP calls produced while handling it.

`interactions` has a stable source/session/upstream identity. Re-delivery returns existing root. `model_calls.interaction_id` and `tool_calls.interaction_id` link descendants without turning descendants into prompts. Historical model and tool rows remain valid legacy-unlinked rows after migration.

Raw events remain append-only and hash chained. Prompt bodies, response bodies, tool inputs, and tool outputs are excluded. Prompt capture defaults to `hash`; `off` stores neither value nor hash, while `full` is reserved for explicit locally redacted configuration.

Project resolution creates a local project only for an unambiguous Git root. No Git root means unattributed; provider, model, and agent identity never imply ownership.
