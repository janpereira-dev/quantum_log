# QUANTUM_LOG

A small, local usage ledger for Codex sessions.

Import the session files already on your computer, view reported token usage
by project, and export an aggregate CSV. The default executable runs only when
you invoke it and exits when the command finishes.

[English guide](docs/SIMPLE.md) · [Guía en español](docs/es/simple.md)

## Portable preview — no Go installation

Open the [v0.4.0-rc11 release](https://github.com/janpereira-dev/quantum_log/releases/tag/v0.4.0-rc11).
Once its release checks complete and assets are available, choose the
`qlog-simple_0.4.0-rc11_OS_ARCH` archive for your operating system and
architecture, and verify its SHA-256 against that release's `checksums.txt`.
Extract it into a new folder and open a terminal there. The included
[bilingual README](packaging/simple/README.md) walks through the portable version.

Try the synthetic demo on Windows in PowerShell:

```powershell
.\qlog-simple.exe --home .qlog/demo import fixtures/demo-session.jsonl
.\qlog-simple.exe --home .qlog/demo report
```

On Linux or macOS:

```sh
./qlog-simple --home .qlog/demo import fixtures/demo-session.jsonl
./qlog-simple --home .qlog/demo report
```

Expect two usage samples and **150 synthetic tokens**, with project
`unattributed` and costs unavailable. The demo uses its own ledger and does
not read your personal Codex sessions. To import your sessions afterwards,
run `qlog-simple import`, then `qlog-simple report`; `qlog-simple export`
writes CSV. Use `.\qlog-simple.exe` or `./qlog-simple` for your platform.

## Optional: build from source

On Windows, use the existing build script from the cloned repository:

```powershell
git clone https://github.com/janpereira-dev/quantum_log.git
cd quantum_log
.\scripts\build-simple.ps1
```

The script pins the toolchain declared in `go.mod`, restores the previous
setting, and handles checkout paths containing brackets. Its executable is
`.\dist\simple\qlog.exe`. On Linux or macOS, use a patched Go toolchain matching
`go.mod`, build with `go build -o qlog ./cmd/qlog`, and invoke `./qlog`.
Plain `go build` can select a newer installed compiler; that compiler must
also be patched. See the [Go security advisory](https://pkg.go.dev/vuln/GO-2026-6809).

Try the included synthetic example in a separate demo ledger:

```powershell
.\dist\simple\qlog.exe --home .qlog/demo import fixtures/demo-session.jsonl
.\dist\simple\qlog.exe --home .qlog/demo report
```

The fixture contains two cumulative usage samples totaling **150 tokens**,
using `demo-model`. It has no real project directory, so its project is
`unattributed`. The numbers are invented demo data, not a real user's activity.
Importing it again leaves the total at 150.

To import your own local Codex sessions:

```powershell
.\dist\simple\qlog.exe import
.\dist\simple\qlog.exe
.\dist\simple\qlog.exe export > usage.csv
```

Import scans the local Codex sessions directory for files modified within the
last seven days, with at most 100 eligible files per run. It initializes a
separate ledger automatically. Repeating an import does not duplicate the same
usage evidence. No `init` or agent setup is needed.

Command references use `qlog` for the source-built executable; use
`qlog-simple` for the portable preview.

| Command | Result |
| --- | --- |
| `qlog` or `qlog report` | Summarize imported usage by project, agent, and model |
| `qlog import` | Import recent local Codex session files |
| `qlog import path/to/session.jsonl` | Import one explicit session file |
| `qlog report --today` | Report the current local day's usage |
| `qlog report --project PROJECT --json` | Read one project's summary as JSON |
| `qlog export > usage.csv` | Export aggregate usage without source paths |

The [English guide](docs/SIMPLE.md) and [guía en español](docs/es/simple.md)
explain storage, privacy, import limits, and what the counters mean.

## What the numbers mean

Tokens come from cumulative counters reported in available Codex session
files, labeled `agent_reported`. Usage samples are not a count of model calls.
Inherited parent snapshots in spawned or forked session files are excluded;
ambiguous ownership is skipped rather than counted as new child usage.
Missing counters remain unavailable. Costs and subscription spend are
unavailable; the tool does not invent prices or treat missing costs as zero.

The importer retains selected metadata and usage counters, not prompt,
response, tool argument, or tool result content. Files stay local, and the
default executable has no background service, startup registration, hooks,
agent configuration, or MCP server.

## Earlier versions

Existing `qlog_VERSION_OS_ARCH` archives and installers use the earlier
`cmd/qlog-legacy` product. Building or running the simple executable does not
replace an installed collector or remove its configuration.

The previous ledger and implementation are preserved. The old entry point is
available in source as `cmd/qlog-legacy`. See the [historical documentation](docs/legacy/README.md)
and [architecture decision](docs/architecture/ADR-008-on-demand-product.md).

For development and security reports, see [Contributing](CONTRIBUTING.md) and
the [security policy](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).
