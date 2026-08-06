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

M4 is `IN_PROGRESS`. Its stable auto-capture scope is exactly Claude Code, Codex, Copilot CLI, Copilot VS Code, and OpenCode. Generic JSONL import remains available, but it is not M4 auto-capture. A configured adapter is not verified capture: every release claim needs recorded source evidence and clean-device real-agent evidence in [M4 evidence](docs-int/verification/m4-evidence.md).

| Adapter | Current quality | M4 release status |
| --- | --- | --- |
| Claude Code | `lifecycle_only` | Awaiting clean-device lifecycle evidence. |
| Codex | `otel_reported` | `BLOCKED_EXTERNAL`: a real authenticated action completed, but no OTLP request reached a healthy foreground collector. |
| Copilot CLI | `lifecycle_only` | `BLOCKED_EXTERNAL`: official qlog-owned hooks were installed and a real action completed, but no hook event reached qlog. |
| Copilot VS Code | `otel_reported` | `BLOCKED_EXTERNAL`: qlog wrote content-disabled OTel settings, but this host lacks the GitHub Copilot extension/login surface. |
| OpenCode | `lifecycle_only` | Awaiting documented usage schema and clean-device evidence. |

`lifecycle_only` records sanitized lifecycle evidence with no token counters. `unavailable` means qlog does not claim automatic counters. `agent_reported`, `otel_reported`, and `provider_reported` require documented source counters; `estimated` is visibly non-measured. Reports keep capture qualities separate and label cost fields as estimated; they never manufacture tokens or provider spend.

## Quick start

```powershell
cmd /c "set QLOG_INSTALL_LOCAL_ARTIFACT_DIR=C:\path\to\dist&& npm install --prefix C:\qlog-install .\packaging\npm"
```

This one-command path installs the local `v0.3.2-rc.3` release-candidate package from its generated artifact directory. A signed HTTPS RC installer is still blocked externally. For consented setup, follow [Install](docs/INSTALL.md). For observed versus unverified agent capture, use [Auto-capture](docs/AUTOCAPTURE.md) and [10-minute verification](docs/VERIFY.md).

## License

MIT. See [LICENSE](LICENSE).
