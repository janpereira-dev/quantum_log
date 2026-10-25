# Contributing

The current product imports existing local Codex sessions, reports token usage,
and exports CSV. Read the [current guide](docs/SIMPLE.md) and
[ADR-008](docs/architecture/ADR-008-on-demand-product.md) before changing its scope.

## Development

Use the Go toolchain specified in `go.mod`:

```bash
go build -o qlog ./cmd/qlog
./qlog --home .qlog/demo import fixtures/demo-session.jsonl
./qlog --home .qlog/demo report
```

On Windows, build `qlog.exe` and run `.\qlog.exe`. The fixture is synthetic;
keep personal sessions and usage out of tests, issues, and commits.

| Area | Package |
| --- | --- |
| Current executable | `cmd/qlog` |
| Current CLI and command tests | `internal/simplecli` |
| Codex session import | `internal/ingest/codexsession` |
| Application access and lifecycle | `internal/app` |
| Storage, migrations, privacy, and queries | `internal/storage/sqlite` |
| Earlier product | `cmd/qlog-legacy` and its existing packages |

Keep changes focused and reuse the existing boundaries. Preserve append-only
history, duplicate prevention, privacy filtering, and the lock protocol. Do not
add services, hooks, agent configuration, or MCP to the current product.

Add a focused regression test for a bug or non-trivial behavior, then run the
relevant packages and the required checks. Use `t.TempDir()` for isolated homes.
Schema changes require a numbered migration and storage tests. Keep the
[English](docs/SIMPLE.md) and [Spanish](docs/es/simple.md) guides aligned.

## Checks

Required local checks:

```bash
go test -count=1 ./...
go test -race ./...
go vet ./...
git diff --check
```

Format changed Go files with `gofmt`. Race tests require CGo and a supported C
compiler; record any check you cannot run. Do not commit databases, keys, logs,
generated binaries, or credentials. Preserve `unattributed` rather than guessing
project ownership. Keep usage samples distinct from model-call counts and leave
unavailable costs unavailable.

The [historical contributor guides](docs/legacy/README.md) cover earlier
packages and command contracts. Legacy `qlog` release archives and installers
must stay compatible; portable `qlog-simple` assets are a separate build.
Keep dated release evidence separate from current verification claims.

## Pull request review gate

Request an independent Codex review (`@codex review`) on the pull request,
address every material finding, and rerun the required checks. The review must
cover the final commit before merge. The pull-request template records that
receipt; do not mark it complete while any review thread remains unresolved.
