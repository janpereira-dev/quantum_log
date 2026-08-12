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

## What it protects

Data stays local by default. Prompt and response content, tool arguments and results, secrets, and authorization fields are sanitized before hashing or import. Raw events are append-only and chained by source and session.

Capture quality is explicit. Provider-reported, agent-reported, lifecycle-only, unavailable, and other evidence are not treated as equivalent. QUANTUM_LOG never invents token counts.

## Auto-capture ledger

Run `qlog setup --yes`, restart configured agents, and use them normally. The collector is managed as a user-level loopback service and hooks/plugins use the installed absolute qlog binary. Every native prompt creates one canonical interaction; model and tool calls are linked children.

| Agent | Interaction | Prompt | Tokens | Cache | Cost | Duration | Tools |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Copilot CLI | captured | configurable | reported | reported | not_emitted_by_source | reported | captured |
| Copilot VS Code | captured | configurable | reported | reported | reported | reported | captured |
| Codex | captured | configurable | reported | reported | not_emitted_by_source | reported | captured |
| Claude Code | captured | configurable | reported | reported | not_emitted_by_source | reported | captured |
| OpenCode | captured | configurable | reported | reported | reported | reported | captured |

`lifecycle_only` records sanitized lifecycle evidence with no token counters. `unavailable` means qlog does not claim automatic counters. `agent_reported`, `otel_reported`, and `provider_reported` require documented source counters; `estimated` is visibly non-measured. Reports keep capture qualities separate and label cost fields as estimated; they never manufacture tokens or provider spend.

## Quick start

```powershell
cmd /c "set QLOG_INSTALL_LOCAL_ARTIFACT_DIR=C:\path\to\dist&& npm install --prefix C:\qlog-install .\packaging\npm"
```

This one-command path installs the local `v0.4.0-rc.1` release-candidate package from its generated artifact directory. For consented setup, follow [Install](docs/INSTALL.md). For auto-capture and reports, use [Auto-capture](docs/AUTOCAPTURE.md).

## License

MIT. See [LICENSE](LICENSE).
