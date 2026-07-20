# QUANTUM_LOG

Trace every agent. Trust every event.

Local-first observability and FinOps for AI coding agents. QUANTUM_LOG records verifiable, privacy-aware usage evidence without making a SaaS, proxy, or prompt archive mandatory.

## Status

Lifecycle state records the audited delivery stage. Public availability and verification
claims derive from acceptance evidence, not from source presence, stubs, templates,
registrations, or unexecuted commands.

| Milestone | Lifecycle state | Evidence status |
|---|---|---|
| M0 | `IMPLEMENTED` | Matrix absent; implementation is audited source inventory, not verification. |
| M1 | `BLOCKED` | [M1 evidence matrix](docs/verification/milestone-1-evidence.md) has required `FAIL` and `NOT_RUN` rows. |
| M2 | `IMPLEMENTED` | Matrix absent; implementation is audited source inventory, not verification. |
| M3 | `IN_PROGRESS` | Matrix absent; no availability or verification claim. |
| M4 | `IN_PROGRESS` | Matrix absent; capture maturity is `DETECTION_ONLY`, not verified capture. |
| M5 | `IMPLEMENTED` | Matrix absent; implementation is audited source inventory, not verification. |
| M6 | `IMPLEMENTED` | Matrix absent; implementation is audited source inventory, not verification. |

Milestone lifecycle states are `NOT_STARTED`, `IN_PROGRESS`, `IMPLEMENTED`,
`VERIFIED`, `BLOCKED`, and `DEFERRED`. `DETECTION_ONLY` is M4 capture maturity,
not a seventh lifecycle state. A milestone is `VERIFIED` only after every required
acceptance criterion is `PASS` in `docs/verification/milestone-<n>-evidence.md`.
Any `FAIL`, `NOT_RUN`, or `BLOCKED` row prevents verification.

While M1 is `BLOCKED`, every command, path, and behavior description below is
unverified source inventory unless it links to recorded acceptance evidence.

## Concepts

- A `Project` is stable logical identity.
- A `ProjectLocation` is one physical checkout or worktree.
- A `WorkContext` is temporal state: CWD, Git metadata, session, and resolved project.
- A session may move A -> B -> A. Each event keeps its own resolved project, method, evidence, and confidence.
- Unknown ownership remains `unattributed`; QUANTUM_LOG never guesses from provider, model, or agent name.

Resolution precedence is explicit `--project`, `QLOG_PROJECT`, current working directory, Git root, registered path, adapter hint, then `unattributed`.

## Privacy

Data remains local by default. Prompt content, response content, tool arguments, tool results, secrets, and authorization fields are removed from imported payloads. Absolute paths are retained locally only to resolve project identity and are intended to be redacted or hashed in future exports.

Capture quality is explicit. Provider-reported, observed, inferred, manual, and unavailable data must not be presented as equivalent.

## Unverified Command Examples

M1 is `BLOCKED`. These commands are source-inventory examples, not supported
current behavior or acceptance evidence.

```bash
go run ./cmd/qlog init
go run ./cmd/qlog project register --path . --name QUANTUM_LOG
go run ./cmd/qlog project current --json
go run ./cmd/qlog ingest file fixtures/session-a-b-a.ndjson
go run ./cmd/qlog verify
```

## Unaudited Source Inventory

The repository contains source inventory for a TUI, stdio MCP, tasks, project
operations, allocations, pricing, reports, exports, unattributed repair, and
budgets. These M2-M6 interfaces are neither verified nor supported capability
claims; consult their acceptance evidence before relying on any command or behavior.

## Local Paths

`QLOG_HOME` overrides all paths. Windows defaults under `%LOCALAPPDATA%\\QUANTUM_LOG`; Linux respects `$XDG_DATA_HOME` or `~/.local/share/quantum-log`; macOS uses the user configuration directory. `qlog init` creates `config.yaml` and `qlog.db` idempotently.

## Installation

Use Go installation today:

```bash
GOFLAGS=-buildvcs=true go install github.com/janpereira-dev/quantum_log/cmd/qlog@latest
```

Native installers and package-manager metadata are present, but their release endpoints and external package registrations must exist before they can install a release. They never bypass checksum verification and default to user-owned directories. See [distribution release process](docs/releases/distribution.md) before using or publishing them.

## Architecture

```text
CLI -> application service -> domain/resolver -> SQLite
```

The domain has no dependency on Cobra or SQLite. Migrations are embedded. SQLite uses `modernc.org/sqlite`, so normal builds run with `CGO_ENABLED=0`. Raw events are append-only and chained with SHA-256 per source/session.

## Security

Run `qlog doctor --json` for local database health and `qlog verify` for ledger integrity. See [SECURITY.md](SECURITY.md). No example usage data is represented as real activity; fixtures are explicitly test data.

## Recovery Sequence

This sequence is delivery planning, not evidence of availability.

1. M1: integrity and project attribution.
2. M2: reporting, allocations, pricing, and export correctness.
3. M4: technical capture beyond detection-only maturity.
4. M3: TUI backed by shared query services.
5. M5: distribution and clean-runner installation.
6. M6: MCP and agent integration.

## License

MIT. See [LICENSE](LICENSE).
