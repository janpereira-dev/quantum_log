# Contributing

Contribute changes through focused Go packages, explicit privacy boundaries, and evidence-backed verification. The executable path is deliberately thin:

```text
cmd/qlog -> internal/cli -> internal/app -> domain services/resolver -> internal/storage/sqlite
```

[Contribución en español](../es/contribucion.md)

## Local development

```bash
go build -o qlog ./cmd/qlog
go test -count=1 ./...
go vet ./...
```

Optional focused commands:

```bash
go test -count=1 ./internal/cli
go test -count=1 ./internal/cli -run TestName
make build
make test
make race
make vet
make fmt
```

The project uses `modernc.org/sqlite`, so CGo-free validation is supported:

```bash
CGO_ENABLED=0 go test -count=1 ./...
```

## Change boundaries

| Area | Put changes here |
| --- | --- |
| Executable startup | `cmd/qlog/` only. |
| Cobra command definition and command tests | `internal/cli/`. |
| App context lifecycle | `internal/app/`. |
| Pure project attribution | `internal/attribution/resolver/`. |
| Persistence, migrations, reporting, sanitization | `internal/storage/sqlite/`. |
| Hash-chain/anchor verification | `internal/audit/`. |
| JSONL, OTLP, qlog event normalization | `internal/ingest/`. |
| Agent integration and passive capture | `internal/adapters/`, `internal/capture/wrapper/`. |
| Terminal and MCP views | `internal/tui/`, `internal/mcpserver/`. |

Do not make CLI commands reach into SQLite internals when an application or store boundary already owns the behavior.

## CLI changes

Add a command with `newXxxCommand(home *string) *cobra.Command` under `internal/cli/`, register it from `internal/cli/root.go`, and add focused tests using `--home <tmpdir>`. Keep current command behavior accurate in [CLI reference](cli-reference.md).

Mutating commands use `app.Open`. Read-only commands use `app.OpenReadOnly`. Diagnostics require exclusive quiescence access; never bypass this protocol with an external SQLite client.

## Storage and migrations

New persistent behavior belongs on `*Store` in `internal/storage/sqlite/`. Schema changes need a numbered migration under `internal/storage/sqlite/migrations/` and storage tests. Migrations run in lexical order.

Use `t.TempDir()` for isolated test homes/databases. Never stage generated `qlog.db`, WAL/SHM files, or lock files.

## Privacy and capture rules

Sensitive content must be sanitized before hashing or import. Maintain sanitizer coverage when an integration introduces another sensitive-key family. Do not persist prompts, responses, tool arguments/results, secrets, tokens, authorization values, API keys, cookies, environment values, or process output.

Every capture path must declare truthful capture quality. Do not synthesize token counts. Lifecycle-only evidence must remain lifecycle-only through reports, exports, tests, and documentation.

M4 remains `IN_PROGRESS`. Copilot VS Code capture remains experimental until a real trace persists Copilot-originated tokens in SQLite and the relevant verification evidence supports the claim.

## Validation and documentation

Before requesting review, run at minimum:

```bash
go test -count=1 ./...
go vet ./...
git diff --check
```

For capture changes, run relevant adapter/collector tests and validate the explicit capture-quality contract. For CLI changes, verify `--help` output and update docs without inventing flags or unsupported recovery behavior.

## Release handoff

Maintain release evidence separately from public claims. Do not mark a milestone `VERIFIED` without complete passing acceptance evidence in [`docs-int/verification/`](../../docs-int/verification/). Keep ADRs under `docs/architecture/` as normative records.

Maintainers can run a release dry run:

```bash
goreleaser snapshot --clean
```

Commit messages use Conventional Commits. Do not add `Co-Authored-By` or AI attribution.
