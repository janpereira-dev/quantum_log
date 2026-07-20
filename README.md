# QUANTUM_LOG

Trace every agent. Trust every event.

Local-first observability and FinOps for AI coding agents. QUANTUM_LOG records verifiable, privacy-aware usage evidence without making a SaaS, proxy, or prompt archive mandatory.

## Version

`qlog 0.2.0`

This is the first functional release. All acceptance evidence runs reproducibly via `go test ./...`. The release closes the M1 recovery arc (integrity, attribution, locks, sanitization, external anchors) and provides working M2–M6 commands for everyday use. Capabilities without recorded acceptance evidence are marked `IMPLEMENTED` (not `VERIFIED`) — they work but have not been audited against a full AC matrix.

## Status

| Milestone | State | Notes |
|---|---|---|
| M0 | `VERIFIED` | Init, paths, config. |
| M1 | `VERIFIED` | Resolver precedence, cooperative SQLite locks, read-only diagnostics, evidence sanitization, external anchors + truncation detection. |
| M2 | `IMPLEMENTED` | Reporting, allocations, pricing, export. Test suite green. |
| M3 | `IMPLEMENTED` | TUI backed by shared query services. |
| M4 | `DETECTION_ONLY` | Capture adapter model present; verified capture not claimed. |
| M5 | `IMPLEMENTED` | Distribution configs present; native installers pending external registry publication. |
| M6 | `IMPLEMENTED` | stdio MCP server and agent hooks. |

States: `NOT_STARTED`, `IN_PROGRESS`, `IMPLEMENTED`, `VERIFIED`, `BLOCKED`, `DEFERRED`. `VERIFIED` requires every AC `PASS` in `docs/verification/milestone-<n>-evidence.md`.

## Concepts

- A `Project` is stable logical identity.
- A `ProjectLocation` is one physical checkout or worktree.
- A `WorkContext` is temporal state: CWD, Git metadata, session, and resolved project.
- Sessions can move A -> B -> A. Each event keeps its own resolved project, method, evidence, confidence.
- Unknown ownership stays `unattributed`; QUANTUM_LOG never guesses from provider, model, or agent name.

Resolution precedence: explicit `--project`, `QLOG_PROJECT`, CWD, Git root, registered path, adapter hint, then `unattributed`.

## Privacy

Data stays local by default. Prompt content, response content, tool arguments, tool results, secrets, and authorization fields are removed from imported payloads before hashing. Absolute paths are retained locally only to resolve project identity.

Capture quality is explicit. Provider-reported, observed, inferred, manual, and unavailable data are never presented as equivalent.

## Quick Start

```bash
go run ./cmd/qlog init
go run ./cmd/qlog project register --path . --name QUANTUM_LOG
go run ./cmd/qlog project current --json
go run ./cmd/qlog ingest file fixtures/session-a-b-a.ndjson
go run ./cmd/qlog verify
go run ./cmd/qlog doctor --json
go run ./cmd/qlog anchor export > /tmp/anchors.json
go run ./cmd/qlog anchor check --file /tmp/anchors.json
```

## Local Paths

`QLOG_HOME` overrides all paths. Windows defaults under `%LOCALAPPDATA%\QUANTUM_LOG`; Linux respects `$XDG_DATA_HOME` or `~/.local/share/quantum-log`; macOS uses the user configuration directory. `qlog init` creates `config.yaml` and `qlog.db` idempotently.

## Installation

### From source (recommended today)

```bash
GOFLAGS=-buildvcs=true go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.2.0
```

### Build locally

```bash
git clone https://github.com/janpereira-dev/quantum_log.git
cd quantum_log
go build -o qlog ./cmd/qlog
./qlog --version
```

### Native installers

`goreleaser` configs for Homebrew, AUR, WinGet, Scoop, and Docker are present under `goreleaser.d/`. External registry publication (AUR upload, WinGet PR merge, Scoop manifest, Homebrew tap) is out of repository scope. See `docs/releases/distribution.md` before publishing.

## Architecture

```text
CLI -> application service -> domain/resolver -> SQLite (modernc.org/sqlite, CGo-free)
```

The domain has no dependency on Cobra or SQLite. Migrations are embedded. Raw events are append-only and chained with SHA-256 per source/session. Official SQLite clients take a shared quiescence lock; writers also take an exclusive writer lock. Read-only diagnostics (`doctor`, `verify`, `anchor check`) take an exclusive quiescence lock, block on active WAL, and warn on isolated SHM. See [ADR-004](docs/architecture/ADR-004-cooperative-sqlite-ownership.md).

## Security

Run `qlog doctor --json` for local database health, `qlog verify` for ledger integrity, and `qlog anchor export`/`qlog anchor check --file` for external tamper/truncation detection. See [SECURITY.md](SECURITY.md). No example usage data is represented as real activity; fixtures are explicitly test data.

## Developer Guide

See [docs/DEVELOPER_GUIDE.md](docs/DEVELOPER_GUIDE.md) for a step-by-step "idiot-proof" walkthrough: clone, build, test, initial run, commands, file layout, common tasks, troubleshooting, and contributing rules.

## Recovery Sequence

1. M1: integrity and project attribution — **closed in 0.2.0**.
2. M2: reporting, allocations, pricing, export — functional.
3. M4: technical capture beyond detection-only maturity — pending.
4. M3: TUI backed by shared query services — functional.
5. M5: distribution and clean-runner installation — configs ready, external registries pending.
6. M6: MCP and agent integration — functional.

## License

MIT. See [LICENSE](LICENSE).