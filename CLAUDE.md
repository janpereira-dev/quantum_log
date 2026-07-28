# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- Build CLI: `go build -o qlog ./cmd/qlog`
- Build package target: `make build` (`go build ./cmd/qlog`)
- Run all tests: `go test -count=1 ./...`
- Run Makefile test shortcut: `make test` (`go test ./...`)
- Run one package test: `go test -count=1 ./internal/cli`
- Run one test: `go test -count=1 ./internal/cli -run TestName`
- Race test suite: `make race` (`go test -race ./...`)
- Vet: `go vet ./...` or `make vet`
- Format Go files: `gofmt -w .` or `make fmt`
- Install current release from source: `go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.3.0`
- Local smoke path after build:
  - `./qlog init`
  - `./qlog project register --path . --name QUANTUM_LOG`
  - `./qlog project current --json`
  - `./qlog verify`
  - `./qlog doctor --json`
- Agent capture smoke path:
  - `./qlog setup --dry-run`
  - `./qlog collector status --json`
  - `./qlog adapter status --json`
  - `./qlog adapter test opencode`
- MCP server: `./qlog mcp stdio`
- Release dry run: `goreleaser snapshot --clean` (maintainers only; see `docs/releases/distribution.md`)

## Architecture

QUANTUM_LOG is a local-first Go CLI/TUI/MCP application for privacy-aware, tamper-evident usage evidence from AI coding agents. The primary flow is:

```text
cmd/qlog -> internal/cli -> internal/app -> domain services/resolver -> internal/storage/sqlite
```

Key boundaries:

- `cmd/qlog/` is only the executable entrypoint.
- `internal/cli/` owns Cobra command construction and command-level tests.
- `internal/app/` opens application contexts and centralizes read/write lifecycle (`Open`, `OpenReadOnly`, `Close`, `Checkpoint`).
- `internal/attribution/resolver/` contains pure project resolution logic. Resolution precedence is explicit `--project`, `QLOG_PROJECT`, CWD, Git root, registered path, adapter hint, then `unattributed`; qlog must not guess ownership from provider, model, or agent name.
- `internal/storage/sqlite/` owns persistence, migrations, reporting queries, sanitization, and SQLite tests. Migrations are embedded and run in lexical order.
- `internal/storage/lock/` implements the cooperative cross-platform quiescence/writer lock protocol.
- `internal/audit/` verifies append-only SHA-256 event chains and external anchors.
- `internal/ingest/jsonl/`, `internal/ingest/otlp/`, and `internal/ingest/qlogevent/` normalize supported event inputs into sanitized ledger events.
- `internal/adapters/`, `internal/capture/wrapper/`, and setup commands support passive capture integrations for agents.
- `internal/tui/` is the Bubble Tea terminal UI and uses the same query services as CLI reports.
- `internal/mcpserver/` exposes stdio MCP tools for agent integration.
- `fixtures/` are explicit test data only, not real activity.
- `docs/verification/` contains milestone acceptance evidence; do not mark a milestone `VERIFIED` without full PASS evidence there.

## Storage and privacy constraints

- Data stays local by default under `QLOG_HOME`; platform defaults are documented in `README.md` and `docs/DEVELOPER_GUIDE.md`.
- The project uses `modernc.org/sqlite`, so builds can be CGo-free (`CGO_ENABLED=0 go test -count=1 ./...`).
- Official SQLite clients must follow ADR-004's cooperative locking protocol:
  - every client takes a shared quiescence lock;
  - writers also take an exclusive writer lock;
  - read-only diagnostics (`doctor`, `verify`, `anchor check`) take exclusive quiescence and block on active WAL;
  - do not bypass the protocol with external SQLite editors or unsafe immutable opens.
- Raw events are append-only and chained by source/session hashes.
- Prompt content, response content, tool arguments, tool results, secrets, and authorization fields are sanitized before hashing/import. Keep sanitizer coverage updated when adding sensitive-key families.
- Capture quality must remain explicit (`otel_reported`, `agent_reported`, `lifecycle_only`, `unavailable`, etc.); never invent token counts when an agent does not expose real usage.

## Change patterns

- Add a CLI command by creating `newXxxCommand(home *string) *cobra.Command` in `internal/cli/`, registering it in `internal/cli/root.go`, and testing it with `--home <tmpdir>` in `internal/cli/root_test.go` or a focused CLI test file.
- Mutating commands should use `app.Open`; read-only commands should use `app.OpenReadOnly`.
- Add store behavior on `*Store` in `internal/storage/sqlite/`; schema changes require a numbered migration under `internal/storage/sqlite/migrations/` plus storage tests.
- Tests that open storage should use `t.TempDir()` for isolated homes/databases. Prefer table-driven tests where there are many variants.
- Before committing, run `go test -count=1 ./...` and `go vet ./...`; do not stage `qlog.db`, WAL/SHM files, or lock files.
- Commit messages use Conventional Commits. Do not add `Co-Authored-By` or AI attribution.
