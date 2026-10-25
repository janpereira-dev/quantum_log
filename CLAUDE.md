# CLAUDE.md

Guidance for Claude Code and other coding agents working in this repository.
Read `.agents/project.yaml`, `.agents/knowledge/architecture.md`, and
[ADR-008](docs/architecture/ADR-008-on-demand-product.md) before changing scope.

## Current product

The default executable imports existing local Codex session files on demand,
reports usage, and exports aggregate CSV. Running without arguments reports
usage. It exits after each operation.

```text
cmd/qlog -> internal/simplecli -> internal/ingest/codexsession
                              -> internal/app -> internal/storage/sqlite
```

- `cmd/qlog/` is the current executable entry point.
- `internal/simplecli/` owns the current commands, flags, and CLI tests.
- `internal/ingest/codexsession/` reads existing session files and retains only
  selected metadata and usage counters. Source files must remain unchanged.
- `internal/app/` manages initialization, read/write access, locks, and closing.
- `internal/storage/sqlite/` owns persistence, embedded migrations, queries,
  sanitization, duplicate prevention, and the append-only hash chain.
- `cmd/qlog-legacy/` preserves the earlier executable. `internal/cli/`, adapters,
  collector, TUI, accounting, and MCP packages belong to that historical product;
  their presence does not add them to the default runtime.

The current product has no services, startup registration, hooks, agent
configuration, HTTP receiver, or MCP server. Do not restore these features
implicitly. Keep historical work and existing user data intact.

## Commands

Use the Go toolchain declared in `go.mod`:

```bash
go build -o qlog ./cmd/qlog
./qlog --help
./qlog --home .qlog/demo import fixtures/demo-session.jsonl
./qlog --home .qlog/demo report
./qlog --home .qlog/demo export
```

On Windows, build `qlog.exe` and invoke `.\qlog.exe`.
The demo is synthetic; it does not read personal Codex sessions.

```bash
go test -count=1 ./internal/simplecli ./internal/ingest/codexsession
go test -count=1 ./...
go vet ./...
go test -race ./...
```

Race tests require CGo and a supported C compiler. Report any skipped checks.
Format changed Go files with `gofmt`. `make build`, `make test`, and `make vet`
are existing shortcuts. To inspect historical commands, build a distinct binary
from `./cmd/qlog-legacy`; do not replace a user's installed executable.

## Storage and privacy

- Default data lives under the platform Quantum Log directory in a separate
  `simple` subdirectory. With `QLOG_HOME` set, it is `$QLOG_HOME/simple`.
  An explicit `--home` selects that exact directory, including an older ledger.
- Import initializes its own ledger when files are found. Report/export use
  `app.OpenSnapshotReadOnly`; they never migrate implicitly. `migrate` is an
  explicit existing-ledger maintenance operation using `app.Open`.
- Retain append-only history, idempotency, hash-chain behavior, and the
  cooperative lock protocol in ADR-004. Do not bypass locks with external
  SQLite editors or unsafe immutable opens.
- Never retain prompts, responses, tool arguments/results, secrets, or source
  file content. Local session/project metadata is still private information.
  Aggregate exports must omit source paths.
- Tokens retain their source quality. Usage samples are not actual model-call
  counts. Costs and subscription spend are unavailable; never replace missing
  values with zero or invented prices.
- Keep default storage separate from the earlier ledger. Do not automatically
  migrate, delete, or reinterpret an existing installation.

## Changes and verification

- Keep changes focused. Current CLI work belongs in `internal/simplecli`, not
  Cobra command construction in the legacy `internal/cli` package.
- Trace callers and output contracts before changing behavior. Add a focused
  regression test for a bug or non-trivial logic, using isolated `t.TempDir()`
  homes. Never use personal sessions as committed fixtures.
- New persistent behavior belongs on `*Store`; schema changes require a numbered
  migration under `internal/storage/sqlite/migrations/` and storage tests.
- Run relevant checks before review, then the required suite in
  [CONTRIBUTING.md](CONTRIBUTING.md). Never claim a check passed without output.
- Keep the English and Spanish current guides aligned. Historical docs live
  behind the [historical index](docs/legacy/README.md).
- Legacy `qlog` archives/installers and portable `qlog-simple` archives are
  different products. Packaging definitions do not prove publication or runtime
  verification. Release evidence belongs in `docs-int/verification/`.
- Never stage local ledgers, WAL/SHM files, keys, locks, binaries, or private logs.
  Commit messages use Conventional Commits; do not add AI attribution.
