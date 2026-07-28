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

M4 is `IN_PROGRESS`. Copilot VS Code setup and OTel ingestion are experimental until real Copilot-originated usage is recorded in SQLite and documented in [M4 evidence](docs-int/verification/m4-evidence.md). OpenCode, Codex, Claude Code, Pi, OpenClaw, and Hermes capture paths retain their reported quality labels and limits.

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
