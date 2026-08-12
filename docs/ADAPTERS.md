# Adapter Boundaries

Adapters emit only evidence their upstream source supports. Claude Code and Codex hooks provide prompt lifecycle roots plus non-invasive OTel enrichment. Copilot CLI maps `userPromptSubmitted`; Copilot VS Code maps supported root `invoke_agent` events and children where emitted. OpenCode installs an embedded, versioned TypeScript plugin: user messages create roots, completed assistant messages link model calls, and tool callbacks carry lifecycle evidence.

No adapter fabricates tokens, costs, model identity, tool data, or unsupported correlation fields. Missing source values remain `not_emitted_by_source`.
