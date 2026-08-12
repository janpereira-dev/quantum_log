# Auto-capture

`qlog setup --yes` configures only detected stable integrations and attempts a qlog-owned loopback collector. Configuration is not capture proof.

Prompt capture defaults to locally stored hashes. Prompt, response, tool input, and tool output content remain excluded unless an explicit local capture policy permits redacted prompt storage.

## Start

```bash
qlog adapter detect --json
qlog setup --yes
qlog collector status --json
qlog adapter status --json
```

Inspect the configured result after normal agent use:

```bash
qlog adapter verify <adapter> --since 10m --json
qlog adapter verify copilot --since 10m --json
qlog usage project <project-slug> --json
```

`adapter verify` exits non-zero until setup, agent availability, collector reachability, documented quality/source evidence, and fresh durable evidence all pass.

## Collector recovery

The collector listens on loopback and is installed as a user-level service. On Windows, qlog falls back to a user process when Task Scheduler policy denies registration. Hooks and plugins are best-effort and never fail the agent; the next event retries normal ingestion.

## Support matrix

| Adapter | Interaction | Prompt | Tokens | Cache | Cost | Duration | Tools |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Claude Code | captured | configurable | reported | reported | not_emitted_by_source | reported | captured |
| Codex | captured | configurable | reported | reported | not_emitted_by_source | reported | captured |
| Copilot CLI | captured | configurable | reported | reported | not_emitted_by_source | reported | captured |
| Copilot VS Code | captured | configurable | reported | reported | reported | reported | captured |
| OpenCode | captured | configurable | reported | reported | reported | reported | captured |

`lifecycle_only` means sanitized lifecycle evidence, not token counters. `otel_reported` and `agent_reported` mean qlog accepts documented source counters only when source event reaches collector. qlog never invents token counts.

## Privacy

Collector defaults to loopback and serves `/v1/traces`, `/v1/logs`, `/v1/events`, and `/healthz`. Non-loopback listener requires explicit `--allow-non-loopback`.

P0 configuration evidence:

- Codex disables user-prompt logging with `log_user_prompt = false`.
- Copilot VS Code writes `github.copilot.chat.otel.captureContent=false`.
- Copilot CLI hook parser keeps lifecycle metadata and applies the configured prompt policy (`off`, `hash`, or redacted `full`) before persistence; it always discards responses, tool content, authorization data, and token data from hook payloads.
- QLog sanitizes prompt/response content, tool arguments/results, secrets, and authorization fields before hash or persistence.

No P0 run persisted a real external agent event. Persisted-payload privacy inspection remains guided validation, not PASS.

## Cleanup

```bash
qlog adapter uninstall <adapter> --json
qlog collector uninstall --json
```

These remove qlog-owned configuration and managed collector state. They do not erase local ledger automatically.
