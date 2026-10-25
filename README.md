# QUANTUM_LOG

A small, local usage ledger for Codex sessions.

Import the session files already on your computer, view reported token usage
by project, and export an aggregate CSV. The default executable runs only when
you invoke it and exits when the command finishes.

[English guide](docs/SIMPLE.md) · [Guía en español](docs/es/simple.md)

## Try it from source

Clone the repository, then build with the Go toolchain specified in `go.mod`:

```powershell
git clone https://github.com/janpereira-dev/quantum_log.git
cd quantum_log
go build -o qlog.exe ./cmd/qlog
```

On Linux or macOS, use `go build -o qlog ./cmd/qlog` and `./qlog`.

Try the included synthetic example in a separate demo ledger:

```powershell
.\qlog.exe --home .qlog/demo import fixtures/demo-session.jsonl
.\qlog.exe --home .qlog/demo report
```

The fixture contains two cumulative usage samples totaling **150 tokens**,
using `demo-model`. It has no real project directory, so its project is
`unattributed`. The numbers are invented demo data, not a real user's activity.
Importing it again leaves the total at 150.

To import your own local Codex sessions:

```powershell
.\qlog.exe import
.\qlog.exe
.\qlog.exe export > usage.csv
```

Import scans the local Codex sessions directory for files modified within the
last seven days, with at most 100 eligible files per run. It initializes a
separate ledger automatically. Repeating an import does not duplicate the same
usage evidence. No `init` or agent setup is needed.

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

## Downloads and earlier versions

Use the source build above for the current product. Release packaging also
defines portable `qlog-simple_VERSION_OS_ARCH` archives with a `qlog-simple`
executable (`qlog-simple.exe` on Windows). When those assets are available on
a [release](https://github.com/janpereira-dev/quantum_log/releases), choose the
archive for your operating system and architecture, verify its checksum, and
substitute that executable name in the commands above. This documentation
does not imply that a new release has been published.

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
