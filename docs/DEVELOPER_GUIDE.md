# QUANTUM_LOG — Developer Guide (Idiot-Proof)

This guide assumes you know nothing about the project. Follow it top to bottom and you will be able to build, test, run, and contribute.

## 0. What is QUANTUM_LOG?

A local-first CLI that records verifiable, privacy-aware usage evidence for AI coding agents (Claude, Gemini, Cursor, etc.). It tracks model calls, projects, and attribution so you can do FinOps and audit every event without relying on a SaaS, proxy, or prompt archive.

Think of it as a tamper-evident local SQLite ledger plus a CLI/TUI/MCP surface.

## 1. What version is this?

`0.3.0` — first setup-first M4 auto-capture surface. It remains compatible with v0.2.0 local homes and databases, then adds `qlog setup`, collector endpoints, agent hooks/plugins, and project-first usage reporting. M4 remains `IN_PROGRESS` until real Copilot-originated evidence is recorded in `docs/verification/m4-evidence.md`.

## 2. Prerequisites

Install Go 1.22 or later:
- <https://go.dev/dl/>
- Check with `go version`.

Optional but recommended:
- `git` for cloning and versioned builds.
- `make` if you want the shortcuts in the Makefile (not required).
- `goreleaser` only if you cut a release.

Demo: build and run one-liner with `go install` (after v0.3.0 tag is published):

```bash
go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.3.0
qlog --version   # -> qlog 0.3.0 ...
```

No `GOFLAGS`, no `CGO_ENABLED`, no clone. Just Go 1.22+ and one command.

If you already installed v0.2.0, run the same `go install ...@v0.3.0` command. It upgrades the binary and keeps your existing qlog home, database, project registrations, tasks, raw events, and anchors.

The project uses `modernc.org/sqlite`, so it compiles and runs without CGo with `CGO_ENABLED=0` if you ever need CGo-free builds.

## 3. Clone and Build

```bash
git clone https://github.com/janpereira-dev/quantum_log.git
cd quantum_log
go build -o qlog ./cmd/qlog
./qlog --version
```

You should see `qlog 0.3.0 (...)`.

## 4. Run the Tests

```bash
go test -count=1 ./...
```

All packages should print `ok`. If any package fails, stop. Do not add behavior on a red baseline.

Optional CGo-free repeat (matches CI):
```bash
CGO_ENABLED=0 go test -count=1 ./...
```

Optional vet:
```bash
go vet ./...
```

## 5. Initialize Your Local Ledger

Pick a home directory for QUANTUM_LOG data (or use the default). Then:

```bash
./qlog init
```

This creates `config.yaml` and `qlog.db` (with the quiescence and writer lock files `.quiescence.lock` and `.writer.lock`) under your `QLOG_HOME`.

To override the home directory:
```bash
export QLOG_HOME=/path/to/quantum-log-data
./qlog init
```

Defaults:
- Linux: `$XDG_DATA_HOME/quantum-log` or `~/.local/share/quantum-log`
- macOS: `~/Library/Application Support/quantum-log` (user config dir)
- Windows: `%LOCALAPPDATA%\QUANTUM_LOG`

Always run `init` before any other command. Commands that need a missing lock will tell you to run `qlog init`.

## 6. Register a Project

A `Project` is stable identity. A `ProjectLocation` is one checkout/worktree of that project.

Register the repo you are working in:
```bash
./qlog project register --path . --name MY_PROJECT
```

Confirm attribution:
```bash
./qlog project current --json
```

The JSON shows `project_slug`, `method` (explicit, environment, cwd, git_root, registered_path, adapter), `confidence`, and `evidence`.

## 7. Resolution Precedence (Read This Once)

1. Explicit `--project <slug>` on the command line.
2. `QLOG_PROJECT` environment variable.
3. Current working directory matches a registered path exactly.
4. Git root matches a registered path.
5. Longest registered path prefix of CWD or Git root.
6. Adapter-supplied project hint.
7. `unattributed` — QUANTUM_LOG never guesses from provider/model/agent name.

If attribution is wrong, check precedence. The CLI reports the method and evidence used, so you can see why a project was picked.

## 8. Ingest Events

Import NDJSON agent events from a fixture or your own adapter:
```bash
./qlog ingest file fixtures/session-a-b-a.ndjson
```

You can also pipe JSONL on stdin:
```bash
cat my-events.ndjson | ./qlog ingest file --stdin
```

Events are append-only. Every event has a previous hash, current hash, sanitized payload, and sanitized evidence, all in SQLite.

## 9. Verify the Ledger

```bash
./qlog verify
./qlog verify --session <session-id>
```

This recomputes the SHA-256 chain for all events (or one session) and errors if any previous-hash link or content hash mismatches. Read-only — uses the cooperative quiescence lock and does not mutate the database.

## 10. Check Database Health

```bash
./qlog doctor
./qlog doctor --json
```

Reports:
- Quiescence lock presence.
- Active WAL (blocks if non-empty — close running qlog writers first).
- Isolated SHM warning (no mutation).
- Other metadata.

Read-only. Safe to run anytime except while a writer is actively writing.

## 11. External Anchors (Tamper/Truncation Detection)

Export the current head hash and event count of each session:
```bash
./qlog anchor export > /tmp/anchors.json
```

Store that file somewhere safe (commit it, copy it to another host, archive it). Later, verify the current ledger matches:
```bash
./qlog anchor check --file /tmp/anchors.json
```

If the head hash changed → mismatch (possible tampering). If the session is gone or has fewer events than recorded → truncation. Exit non-zero on any mismatch.

## 12. Reports, Usage, Allocations

```bash
./qlog usage today
./qlog usage project MY_PROJECT
./qlog report --from 2026-07-01 --to 2026-07-31
./qlog allocation list
./qlog allocation set --project MY_PROJECT --basis-points 7000
./qlog allocation set --project OTHER --basis-points 3000
```

Allocations must sum to 10000 basis points (100%). The CLI rejects invalid splits.

## 13. Setup Agent Capture

Run setup after installation to configure qlog-owned capture integrations for supported agents:

```bash
./qlog setup --dry-run
./qlog setup opencode --yes
./qlog collector status --json
./qlog collector serve
./qlog adapter status --json
./qlog adapter test opencode
```

Setup targets: `opencode`, `claude-code`, `codex`, `pi`, `copilot-vscode`, `openclaw`, and `hermes`.

Current adapter evidence:

| Adapter | Capture path | Capture quality |
|---|---|---|
| `copilot-vscode` | VS Code GitHub Copilot OTel settings to local `/v1/traces`, content capture disabled. | `CAPTURE_EXPERIMENTAL`; promote only after `docs/verification/m4-evidence.md` records real Copilot-originated tokens in SQLite. |
| `opencode` | Global OpenCode plugin posts sanitized events to local `/v1/events`. | `agent_reported` when plugin payload includes usage; otherwise `lifecycle_only`. |
| `codex` | Codex app-server `rawResponse/completed` events can be forwarded to `/v1/events`. | `agent_reported` only when `usage` is non-null. |
| `claude-code` | `.claude/settings.json` lifecycle hooks call `qlog hook claude-code`. | `lifecycle_only`; no token capability is claimed. |
| `pi`, `openclaw`, `hermes` | Setup-capable fallback targets. | `lifecycle_only` or `unavailable` until verified token sources exist. |

Rules:
- `--dry-run` shows planned file changes without writing.
- `--yes` applies setup changes without an interactive prompt.
- Existing files are backed up before qlog writes qlog-owned files, settings, or marker blocks.
- Re-running setup is idempotent.
- Token capture is only reported when a provider, agent, OTLP event, or structured event exposes real token data.
- If no real token source exists, qlog labels the activity as `lifecycle_only` or another explicit `capture_quality`; it does not invent token counts.

## 14. Pricing and Budgets

```bash
./qlog pricing list
./qlog budget set --project MY_PROJECT --monthly-usd 50
./qlog budget alerts
```

Pricing tables drive estimated cost micros per model call. Budget alerts compare monthly usage to the configured cap per project or tag.

## 15. Tasks

```bash
./qlog task create --title "M2 report drill-down"
./qlog task list
```

Tasks group model calls under a work unit for reporting.

## 16. Export

```bash
./qlog export --format csv > usage.csv
./qlog export --format json > usage.json
```

Columns cover tokens, cost, allocation, project slug, and capture quality.

## 17. TUI

```bash
./qlog tui
```

Or just run `./qlog` in a terminal (it launches the TUI by default).

The TUI shows projects, sessions, usage, and tasks using the same query services as the CLI.

## 18. MCP Server (for agent integration)

```bash
./qlog mcp stdio
```

Run this inside an agent harness or a standalone wrapper that pipes MCP JSON-RPC over stdio. The MCP server exposes tools for project attribution, model-call reporting, usage queries, and ledger verification.

## 19. Maintenance Commands

Mutating explicit operations (not read-only):
```bash
./qlog maintenance status
./qlog maintenance checkpoint
```

`checkpoint` flushes the WAL back into the main database after quiescence is confirmed. Use it only when no qlog writer is running.

`recover` and `rebuild-anchor` are reserved commands that currently return blocked. Do not rely on them in 0.2.0.

## 20. File Layout (What Lives Where)

```
cmd/qlog/                      CLI entrypoint (main.go)
internal/cli/                  Cobra commands
internal/app/                  Application service (Open, OpenReadOnly, Close, Checkpoint)
internal/attribution/resolver/ Pure project resolution logic
internal/storage/sqlite/        SQLite store (locks, migrations, queries)
internal/storage/lock/          Cross-platform quiescence/writer lock files
internal/audit/                 SHA-256 chain record + verification
internal/pricing/              Pricing tables
internal/capture/wrapper/       Adapter capture wrapper
internal/distribution/         Release distribution metadata
internal/ingest/jsonl/         NDJSON ingestion
internal/mcpserver/            MCP stdio server
internal/tui/                  Terminal UI
docs/                          Guides, ADRs, evidence matrices
docs/verification/             Per-milestone acceptance evidence
docs/architecture/             ADRs
docs/releases/                Release/distribution process
migrations/*.sql               Embedded schema migrations
fixtures/                      Test fixtures (explicitly test data)
```

## 21. SQLite Lock Protocol (If You Touch the Database)

QUANTUM_LOG owns the SQLite database via a cooperative lock protocol (ADR-004). Do not open it with external SQLite editors — they bypass the protocol.

- Every official client acquires a shared **quiescence** lock.
- Writers additionally acquire an exclusive **writer** lock.
- Read-only diagnostics (`doctor`, `verify`, `anchor check`) acquire an **exclusive** quiescence lock and block if any qlog client is active.
- A non-empty WAL blocks diagnostics (close active qlog writers first).
- An isolated `qlog.db-shm` triggers a warning but is never mutated by diagnostics.
- `immutable=1` is unsafe without quiescence and is never used by the official path.

If a command errors with "quiescence lock is held by ...", exit the other qlog process and retry.

## 22. Adding a New CLI Command

1. Add a `func newXxxCommand(home *string) *cobra.Command` in `internal/cli/`.
2. Register it in the `root.AddCommand(...)` line in `internal/cli/root.go`.
3. Add tests in `internal/cli/root_test.go` using `--home <tmpdir>` and verifying stdout/stderr/exit code.
4. Run `go test ./internal/cli -count=1`.
5. Run `go vet ./...`.

Do not add commands that mutate the database without first acquiring the writer lock via `app.Open`. Read-only commands should use `app.OpenReadOnly`.

## 23. Adding a New Store Method

1. Declare the method on `*Store` in `internal/storage/sqlite/store.go`.
2. If it mutates, it must run inside an `Open` context (writer lock acquired). If it is read-only, prefer `OpenReadOnly` callers and no `Exec`.
3. Add a migration file `internal/storage/sqlite/migrations/NNN_description.sql` if you need schema changes. Migrations are embedded and run in lexical order on `Open`.
4. Add tests in `internal/storage/sqlite/`.

## 24. Adding a Test

We follow standard Go testing plus table-driven cases when there are many variants. Every test that opens a store uses `t.TempDir()` for isolation. Never depend on a shared database file.

```go
func TestMyFeature(t *testing.T) {
    t.Parallel()
    store, err := Open(context.Background(), filepath.Join(t.TempDir(), "qlog.db"))
    if err != nil { t.Fatalf("open: %v", err) }
    defer store.Close()
    // ... assertions
}
```

## 25. Troubleshooting

| Symptom | Fix |
|---|---|
| `quiescence lock is missing; run qlog init to restore it` | Run `qlog init`. |
| `database has an active WAL; close active qlog writers and retry` | Stop any running qlog writer or MCP server, then retry. |
| `writer lock is held by an active qlog process` | Another qlog writer is running. Quit or wait. |
| `quiescence lock is held by an active qlog client` | Another reader or writer is running. Retry after it exits. |
| `anchor verification failed` | The ledger changed after the anchor was exported. Investigate what changed. |
| `parse raw event JSON: ...` | The event payload is not valid JSON. Fix the producer. |
| `project <slug> not found` | Register it first with `qlog project register`. |
| `allocation basis points total X, want 10000` | Adjust allocations so they sum to 10000. |

## 26. Contributing

1. Branch from `main`.
2. Write tests first (TDD when adding new behavior). Run `go test ./...` and confirm red, then implement, then green.
3. Keep `go vet ./...` clean.
4. Do not commit `qlog.db`, `qlog.db-wal`, `qlog.db-shm`, or lock files. They are local artifacts.
5. Do not change the cooperative lock protocol without an updated ADR. See [ADR-004](architecture/ADR-004-cooperative-sqlite-ownership.md).
6. Keep sanitizer coverage updated when you add new sensitive-key families (look at `sensitiveKey` in `internal/storage/sqlite/store.go`).
7. Commit messages use Conventional Commits (`feat(scope): ...`, `fix(scope): ...`, `docs: ...`). No "Co-Authored-By" or AI attribution.
8. Never claim `VERIFIED` for a milestone without the evidence matrix fully `PASS` in `docs/verification/milestone-<n>-evidence.md`.

## 27. Cutting a Release (Maintainers Only)

1. Confirm `go test -count=1 ./...` and `go vet ./...` are green.
2. Confirm `README.md` and `docs/DEVELOPER_GUIDE.md` reflect the new version.
3. Tag the commit `vX.Y.Z`.
4. Run `goreleaser release` (or `goreleaser snapshot --clean` for a dry run).
5. Follow `docs/releases/distribution.md` for the full release process, including native installers.

Do not publish native installers before their target registry endpoints exist.

## 28. Where to Ask for Help

Open an issue at <https://github.com/janpereira-dev/quantum_log/issues> with: qlog version (`qlog --version`), OS, `qlog doctor --json` output (redact paths if needed), and the exact command you ran.

## 29. Final Checklist Before You Commit

- [ ] `go test -count=1 ./...` green
- [ ] `go vet ./...` clean
- [ ] No DB/WAL/SHM/lock artifacts staged
- [ ] Conventional Commits message
- [ ] No unverifiable milestone claims added
- [ ] README and DEVELOPER_GUIDE updated if behavior changed

You are now ready to use and extend QUANTUM_LOG 0.2.0.
