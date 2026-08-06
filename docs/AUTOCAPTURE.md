# Auto-capture

`qlog setup --yes` configures only detected stable integrations and attempts a qlog-owned loopback collector. Configuration is not capture proof.

## Start

```bash
qlog adapter detect --json
qlog setup --yes
qlog collector status --json
qlog adapter status --json
```

Inspect configured result after first real agent action:

```bash
qlog adapter verify <adapter> --since 10m --json
qlog usage project <project-slug> --json
```

`adapter verify` exits non-zero until setup, agent availability, collector reachability, documented quality/source evidence, and fresh durable evidence all pass.

## Scheduler policy fallback

**Observed Windows P0 condition:** Task Scheduler returned `Error: Acceso denegado.` when qlog attempted to create `QUANTUM_LOG Collector`. This is an external policy block, not successful managed collector installation.

Run a foreground collector in one terminal instead:

```powershell
qlog collector serve --home $env:QLOG_HOME --log-file "$env:QLOG_HOME\collector.log"
```

Keep this process running while agent acts. In another terminal, check health:

```bash
qlog collector status --json
```

Foreground service is not managed. Stop it with `Ctrl+C`; start it again after any restart. Do not claim managed restart PASS from this fallback.

## Support matrix

| Adapter | Setup mechanism | Evidence quality | P0 status |
| --- | --- | --- | --- |
| Claude Code | qlog-managed lifecycle hooks | `lifecycle_only` | Guided validation only; clean-device real event pending. |
| Codex | user-level OTLP log exporter with `log_user_prompt = false` | `otel_reported` | **BLOCKED_EXTERNAL**: authenticated real action sent no request to healthy foreground collector. |
| Copilot CLI | qlog-owned user hook file | `lifecycle_only` | **BLOCKED_EXTERNAL**: authenticated real action produced no delivered hook event. |
| Copilot VS Code | qlog-managed OTel settings with `captureContent=false` | `otel_reported` | **BLOCKED_EXTERNAL**: host lacks GitHub Copilot extension/login; no real action attempted. |
| OpenCode | qlog-managed plugin usage, lifecycle, and tool events | `agent_reported` for allowlisted assistant counters; `lifecycle_only` otherwise | Guided validation only; clean-device real event pending. |

`lifecycle_only` means sanitized lifecycle evidence, not token counters. `otel_reported` and `agent_reported` mean qlog accepts documented source counters only when source event reaches collector. qlog never invents token counts.

## Privacy

Collector defaults to loopback and serves `/v1/traces`, `/v1/logs`, `/v1/events`, and `/healthz`. Non-loopback listener requires explicit `--allow-non-loopback`.

P0 configuration evidence:

- Codex disables user-prompt logging with `log_user_prompt = false`.
- Copilot VS Code writes `github.copilot.chat.otel.captureContent=false`.
- Copilot CLI hook parser keeps only session ID and CWD; it discards prompt, response, tool, secret, authorization, and token data.
- QLog sanitizes prompt/response content, tool arguments/results, secrets, and authorization fields before hash or persistence.

No P0 run persisted a real external agent event. Persisted-payload privacy inspection remains guided validation, not PASS.

## Cleanup

```bash
qlog adapter uninstall <adapter> --json
qlog collector uninstall --json
```

These remove qlog-owned configuration and managed collector state. They do not erase local ledger automatically.
