# Simple local usage ledger

Quantum Log imports existing Codex session files on demand. It stores selected
usage evidence in a separate local ledger, then reports aggregate totals. There
is no background process to keep running between commands.

## Build and use

From the repository root, using the Go toolchain specified in `go.mod`:

```powershell
go build -o qlog.exe ./cmd/qlog
.\qlog.exe import
.\qlog.exe report
.\qlog.exe export > usage.csv
```

On Linux or macOS, build with `go build -o qlog ./cmd/qlog` and replace
`.\qlog.exe` with `./qlog`.

Running `qlog` is equivalent to `qlog report`. Before the first import it shows
an empty report; it does not require an initialization command. `qlog --help`
lists the supported commands.

## Choose what to import

The default source is `$CODEX_HOME/sessions` when `CODEX_HOME` is set, otherwise
`~/.codex/sessions`. The source files are read, never modified.

```powershell
.\qlog.exe import
.\qlog.exe import --days 30 --limit 100
.\qlog.exe import C:\path\to\session.jsonl
.\qlog.exe import --days 30 C:\path\to\sessions
```

Place import flags before the optional path. Directory discovery considers
files modified in the last seven days by default. `--days` accepts 1–36500.
`--limit` is a positive file count with a maximum of 100; the default is 100.
An explicit file is not filtered by the discovery window. The newest eligible
files are selected first. The limit bounds files imported, not directory
traversal or the size of an individual session file.

An import is a snapshot. Run it again after using Codex to include newer
evidence. Reimporting the same evidence is idempotent. A recent modification
time can include older events within the selected file; it is not the event
date filter used by `report --today`. `import --json` provides a machine-readable
import result.

## Read and export

```powershell
.\qlog.exe report --today
.\qlog.exe report --project PROJECT
.\qlog.exe report --json
.\qlog.exe export --today > today.csv
.\qlog.exe export --project PROJECT > project.csv
```

Use the project identifier printed by the report. Without a filter, reporting
and exporting cover all imported evidence grouped by project, agent, and model.
`--today` uses the current local calendar day. CSV exports go to standard
output and contain aggregate summaries, not raw content or source paths.

Tokens are based on agent-reported cumulative usage snapshots. They are not a
measurement of subscription charges or a guarantee that every model call is
present in the logs. Usage sample counts must not be interpreted as actual
model call counts. Costs are explicitly unavailable.

## Storage and privacy

The default data directory is a `simple` subdirectory of the normal Quantum
Log data directory, separate from the earlier ledger:

| Platform | Default directory when `QLOG_HOME` is unset |
| --- | --- |
| Windows | `%LOCALAPPDATA%\QUANTUM_LOG\simple` |
| Linux | `$XDG_DATA_HOME/quantum-log/simple`, or `~/.local/share/quantum-log/simple` |
| macOS | `~/Library/Application Support/QUANTUM_LOG/simple` |

When `QLOG_HOME` is set, the default is `$QLOG_HOME/simple`. The data directory
contains the existing storage's configuration, SQLite database, key, and locks.
`--home PATH` uses the chosen directory directly. Use a new empty directory for
an isolated trial. An explicit existing ledger is supported, so choose this
override carefully; it is not automatically isolated from that ledger.

The importer whitelists metadata and usage counters. It does not retain
prompts, responses, tool arguments, tool results, or source file content. Local
metadata still identifies sessions and projects; protect the data directory
as you would other local work records. Exports omit source paths.

The existing append-only storage, duplicate prevention, privacy handling, and
hash chain are reused. The historical schema and migrations remain in source;
this change reduces the active product surface rather than replacing storage.

## Limits and performance

The [local validation report](verification/simple-local-ledger.md) records the
tested build, real source import/replay, resource observations, and limitations.
Use `scripts/build-simple.ps1` to build a portable Windows package. Generated
packages and preservation backups are separate under `dist`.
The build script uses the exact toolchain pinned in `go.mod`, then restores the
calling process's setting. Plain `go build` can use a newer installed compiler.

- Only available local Codex session logs are supported. Their schema may
  change, and logs may be absent or incomplete.
- Importing is passive and manual. It does not capture activity in real time
  or configure Codex, other agents, hooks, services, or startup tasks.
- Existing installations are unchanged by building this executable. A running
  older collector must be managed using the earlier version's controls.
- Resource measurements must name the tested build, source size, and hardware.
  A successful import alone is not proof of low resource use or complete
  end-to-end agent coverage.

## Historical documentation

[Install](INSTALL.md), [Auto-capture](AUTOCAPTURE.md), [Verification](VERIFY.md),
and [Troubleshooting](TROUBLESHOOTING.md) describe the earlier product. They are
reference material, not the setup flow for the default source executable.
The [preserved README](legacy/README-v0.4.md) records that earlier entry point.
