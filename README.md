# QUANTUM_LOG

Trace every agent. Trust every event.

QUANTUM_LOG is local-first observability and FinOps for AI coding agents. It records privacy-aware, tamper-evident usage evidence without requiring a SaaS, proxy, or prompt archive.

## Start here

- [Install](docs/INSTALL.md)
- [Auto-capture](docs/AUTOCAPTURE.md)
- [10-minute verification](docs/VERIFY.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
- [Security policy](SECURITY.md)
- [Architecture decision records](docs/architecture/)
- [Collector lifecycle and Windows startup behavior](docs/architecture/ADR-005-collector-lifecycle.md)

## What it protects

Data stays local by default. Prompt and response content, tool arguments and results, secrets, and authorization fields are sanitized before hashing or import. Raw events are append-only and chained by source and session.

Capture quality is explicit. Provider-reported, agent-reported, lifecycle-only, unavailable, and other evidence are not treated as equivalent. QUANTUM_LOG never invents token counts.

## Auto-capture ledger

Run `qlog setup --yes`, restart configured agents, and use them normally. The collector is managed as a user-level loopback service when the platform manager permits it; hooks/plugins use the installed absolute qlog binary. On Windows, a Task Scheduler policy denial does not create a Startup-app fallback: use an explicit foreground collector for OTLP capture. Every native prompt creates one canonical interaction; model and tool calls are linked children.

To remove qlog-owned startup, collector, and adapter configuration in one step,
run `qlog uninstall`. It always retains the local ledger in RC.10: automatic
`--purge-data` is temporarily unavailable and fails closed.

| Agent | Implemented capture path | Current evidence state |
| --- | --- | --- |
| Copilot CLI | Sanitized lifecycle hooks | `READY_FOR_EXTERNAL_E2E`; native hook-origin RC10 evidence is not recorded |
| Copilot VS Code | Privacy-disabled-content OTLP | `READY_FOR_EXTERNAL_E2E`; two-machine RC10 evidence is not recorded |
| Codex | Privacy-disabled-prompt OTLP logs | `READY_FOR_EXTERNAL_E2E`; clean-device RC10 evidence is not recorded |
| Claude Code | Sanitized lifecycle hooks | `READY_FOR_EXTERNAL_E2E`; real-source RC10 evidence is not recorded |
| OpenCode | Sanitized plugin lifecycle events | `READY_FOR_EXTERNAL_E2E`; real-source RC10 evidence is not recorded |

`lifecycle_only` records sanitized lifecycle evidence with no token counters. `unavailable` means qlog does not claim automatic counters. `agent_reported`, `otel_reported`, and `provider_reported` require documented source counters; `estimated` is visibly non-measured. Reports keep capture qualities separate and label cost fields as estimated; they never manufacture tokens or provider spend.

## Quick start

No stable `v0.4.0` release has been published. Published prerelease `v0.4.0-rc10`
(`35ae43bd0031b3aca2621c52ede74731ae136357`) is available for evaluation, but
does not have complete two-machine or five-agent external E2E evidence. Do not
use `go install` or the legacy npm package: both bypass the published archive and
checksum lifecycle. The older stable release line is not the supported
evaluation path. Install the RC explicitly with
`install.sh --version v0.4.0-rc10` or `install.ps1 --version v0.4.0-rc10` as
described in [Install](docs/INSTALL.md).

## License

MIT. See [LICENSE](LICENSE).
