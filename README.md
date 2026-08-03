# QUANTUM_LOG

Trace every agent. Trust every event.

QUANTUM_LOG is local-first observability and FinOps for AI coding agents. It records privacy-aware, tamper-evident usage evidence without requiring a SaaS, proxy, or prompt archive.

## Start here

- [Documentation](docs/README.md)
- [Security policy](SECURITY.md)
- [Architecture decision records](docs/architecture/)

## What it protects

Data stays local by default. Prompt and response content, tool arguments and results, secrets, and authorization fields are sanitized before hashing or import. Raw events are append-only and chained by source and session.

Capture quality is explicit. Provider-reported, agent-reported, lifecycle-only, unavailable, and other evidence are not treated as equivalent. QUANTUM_LOG never invents token counts.

## Current status

| Milestone | Current status | Evidence |
| --- | --- | --- |
| M0 | Historical work; reconfirmation required | Acceptance evidence was not preserved. |
| M1 | `BLOCKED` | [M1 evidence matrix](docs-int/verification/milestone-1-evidence.md) contains no passing acceptance criteria. |
| M2 | `IMPLEMENTED` | Milestone acceptance evidence required for `VERIFIED`. |
| M3 | `IMPLEMENTED` | Milestone acceptance evidence required for `VERIFIED`. |
| M4 | `IN_PROGRESS` | [M4 evidence](docs-int/verification/m4-evidence.md) |
| M5 | `IMPLEMENTED` | Milestone acceptance evidence required for `VERIFIED`. |
| M6 | `IMPLEMENTED` | Milestone acceptance evidence required for `VERIFIED`. |

M4 is `IN_PROGRESS`. Its stable auto-capture scope is exactly Claude Code, Codex, Copilot VS Code, and OpenCode. Generic JSONL import remains available, but it is not M4 auto-capture. A configured adapter is not verified capture: every release claim needs recorded source evidence and clean-device real-agent evidence in [M4 evidence](docs-int/verification/m4-evidence.md).

| Adapter | Current quality | M4 release status |
| --- | --- | --- |
| Claude Code | `lifecycle_only` | Awaiting clean-device lifecycle evidence. |
| Codex | `otel_reported` | Documented OTLP `response.completed` logs with source-reported tokens are supported; clean-device accepted `response.completed` evidence and normal verification remain required. |
| Copilot VS Code | `otel_reported` | Documented VS Code OTel configuration and sanctioned Copilot model/token evidence are supported; clean-device real-agent acceptance remains required. |
| OpenCode | `lifecycle_only` | Awaiting documented usage schema and clean-device evidence. |

`lifecycle_only` records sanitized lifecycle evidence with no token counters. `unavailable` means qlog does not claim automatic counters. `agent_reported`, `otel_reported`, and `provider_reported` require documented source counters; `estimated` is visibly non-measured. Reports keep capture qualities separate and label cost fields as estimated; they never manufacture tokens or provider spend.

## Capture bootstrap

`qlog setup --yes` is consented qlog bootstrap. It installs and starts a qlog-owned collector on `127.0.0.1:4318`, then configures only detected stable adapters. Re-running it preserves unchanged qlog-managed configuration and reports changed, unchanged, or skipped actions. Use `qlog setup --dry-run` to inspect its plan first.

The collector serves OTLP on `/v1/traces`, qlog JSON events on `/v1/events`, and health on `/healthz`. It refuses a non-loopback listener unless `--allow-non-loopback` is explicit. Manage its user-owned lifecycle with `qlog collector install`, `start`, `status --json`, `logs`, `stop`, `restart`, and `uninstall`. Adapter configuration updates retain a recoverable qlog backup when an existing managed file changes.

`qlog adapter verify <adapter> --project <slug> --json` writes its stages and exits non-zero until every required gate passes: setup, availability, collector reachability, quality, source evidence, and fresh durable evidence. Lifecycle adapters require a real raw event. Reported-token adapters additionally require exactly linked normalized model-call evidence with source-reported tokens. Use `qlog report usage --json`, `qlog usage project <slug> --json`, and `qlog session summary <session-id> --json` to inspect recorded quality-separated evidence.

Replayed events are suppressed by a durable sanitized ingestion identity before ledger append and normalization. A replay cannot increase raw-event, model-call, token, or estimated-cost totals. Current `/v1/events` responses expose accepted-event count only; use records and reports, not a response field, to prove suppression.

## Quick start

```bash
go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.3.0
qlog init
qlog project register --path . --name MY_PROJECT
qlog project current --json
qlog verify
qlog doctor --json
```

## License

MIT. See [LICENSE](LICENSE).
