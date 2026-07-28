# Architecture

QUANTUM_LOG records local, privacy-aware evidence about AI coding-agent activity. Its core flow is:

```text
cmd/qlog -> internal/cli -> internal/app -> domain services/resolver -> internal/storage/sqlite
```

[Arquitectura en español](../es/arquitectura.md)

## Layers

| Layer | Responsibility |
| --- | --- |
| `cmd/qlog` | Executable entrypoint only. |
| `internal/cli` | Cobra command construction and command-level behavior. |
| `internal/app` | Opens services and centralizes read/write lifecycle, locking, and checkpoints. |
| `internal/attribution/resolver` | Pure project-resolution policy. |
| `internal/ingest/*` | Normalizes JSONL, OTLP, and qlog event inputs. |
| `internal/adapters`, `internal/capture/wrapper` | Passive or lifecycle-oriented integrations. |
| `internal/storage/sqlite` | Migrations, persistence, reporting, sanitization, and SQLite queries. |
| `internal/audit` | Verifies append-only SHA-256 chains and external anchors. |
| `internal/tui`, `internal/mcpserver` | Terminal and MCP views over the same query services. |

Normative design decisions live in [architecture decision records](../architecture/), especially [ADR-002](../architecture/ADR-002-project-first-attribution.md), [ADR-003](../architecture/ADR-003-local-ledger.md), and [ADR-004](../architecture/ADR-004-cooperative-sqlite-ownership.md).

## Project resolution

Project ownership is resolved from evidence, in this order:

1. Explicit `--project`.
2. `QLOG_PROJECT`.
3. Registered current directory.
4. Registered Git root.
5. Longest matching registered path.
6. Adapter project signal.
7. Unresolved/unattributed.

The resolver returns a method, confidence, and evidence value. Provider, model, and agent name never establish project ownership. This prevents convenient but unsupported attribution.

## Event lifecycle

1. A JSONL import, OTLP receiver, plugin, hook, or wrapper receives activity.
2. Ingestion resolves project evidence and strips sensitive content.
3. Normalized raw events are appended to SQLite.
4. Events are chained by source and session with SHA-256 hashes.
5. Model-call evidence can feed usage, cost, allocation, task, export, and anchor queries.
6. `verify` checks chain integrity; external anchors detect divergence or truncation outside the local ledger.

Events are not all usage records. A Claude Code hook or `qlog run` creates lifecycle evidence and must remain labeled accordingly. Reports only show observed token totals when upstream evidence supplied them.

## Capture-quality contract

`capture_quality` states what an event can support. Examples include `otel_reported`, `agent_reported`, `lifecycle_only`, and `unavailable`. It is preserved in reports and exports because equivalent-looking totals can have different provenance.

- `otel_reported`: token fields arrived through accepted OTLP evidence.
- `agent_reported`: an agent integration supplied token fields.
- `lifecycle_only`: a process or session happened, but no token count is asserted.
- `unavailable`: no supported usage evidence exists.

New integrations must select a truthful label. They must not infer or estimate token counts merely to fill reporting columns.

## Locking and diagnostics

Official SQLite clients use a cooperative cross-platform protocol:

- Every client takes shared quiescence access.
- Writers also take exclusive writer access.
- Read-only diagnostics (`doctor`, `verify`, `anchor check`) take exclusive quiescence access and block while an active WAL or client activity makes a stable check unsafe.

The implication is operational: use qlog commands, not external SQLite editors or immutable opens, for normal maintenance. A diagnostic refusal due to a lock is intentional protection against inconsistent observations.

## Local boundaries

Data stays local by default under `QLOG_HOME` or platform defaults. The project uses `modernc.org/sqlite`, supporting CGo-free builds. SQLite migrations are embedded and applied in lexical order.

The collector is local-first too. Its default listener is loopback, and non-loopback exposure requires a deliberate flag. MCP runs over stdio. Neither design replaces network security requirements if an operator explicitly exposes a service beyond the local machine.

## Current delivery boundary

M4 remains `IN_PROGRESS`. Copilot VS Code capture is experimental until real Copilot-originated usage with tokens persists in SQLite and verification evidence supports the claim. Installation, a dry run, or generic imported usage do not meet that standard.
