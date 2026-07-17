# QUANTUM_LOG

Trace every agent. Trust every event.

Local-first observability and FinOps for AI coding agents. QUANTUM_LOG records verifiable, privacy-aware usage evidence without making a SaaS, proxy, or prompt archive mandatory.

## Status

Milestones 0 through 5 are implemented in source. Current commands provide local initialization, SQLite ledger integrity checks, project registration/tags/resolution, NDJSON model-call normalization, task lifecycle, allocation repair/splits, versioned pricing persistence and recalculation, usage/report summaries, JSON/CSV export, accessible terminal dashboard, OTLP capture, and verified distribution scripts. No public release host or external package-manager package is claimed as published.

## Concepts

- A `Project` is stable logical identity.
- A `ProjectLocation` is one physical checkout or worktree.
- A `WorkContext` is temporal state: CWD, Git metadata, session, and resolved project.
- A session may move A -> B -> A. Each event keeps its own resolved project, method, evidence, and confidence.
- Unknown ownership remains `unattributed`; QUANTUM_LOG never guesses from provider, model, or agent name.

Resolution precedence is explicit `--project`, `QLOG_PROJECT`, current working directory, Git root, registered path, then `unattributed`.

## Privacy

Data remains local by default. Prompt content, response content, tool arguments, tool results, secrets, and authorization fields are removed from imported payloads. Absolute paths are retained locally only to resolve project identity and are intended to be redacted or hashed in future exports.

Capture quality is explicit. Provider-reported, observed, inferred, manual, and unavailable data must not be presented as equivalent.

## Quick Start

```bash
go run ./cmd/qlog init
go run ./cmd/qlog project register --path . --name QUANTUM_LOG
go run ./cmd/qlog project current --json
go run ./cmd/qlog ingest file fixtures/session-a-b-a.ndjson
go run ./cmd/qlog verify
```

Run `qlog tui` for the dashboard. With no arguments, `qlog` opens it only when stdout is an interactive terminal; piped or redirected output keeps command help. Navigation supports Left/Right, Tab, and 1-4; `Esc` returns to Overview, `?` toggles keyboard help, and `q` or `Ctrl+C` quits. Set `NO_COLOR=1` for plain text. Available Milestone 2 commands include `qlog task start|finish|list`, `qlog project list|show|tag list`, `qlog allocation show|split|repair`, `qlog pricing add|list|show|recalculate`, `qlog report summary`, `qlog export --format json|csv`, and `qlog usage month --group-by project,provider,model --json`.

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

## Roadmap

1. Milestone 1: complete project/core ledger APIs and multi-project fixtures.
2. Milestone 2: usage reports, allocations, tags, pricing, and JSON output.
3. Milestone 3: accessible TUI.
4. Milestone 4: verified capture adapters and OTLP.
5. Milestone 5: signed releases and installers.

## License

MIT. See [LICENSE](LICENSE).
