# QUANTUM_LOG

A small, local usage ledger for Codex sessions.

Import the session files already on your computer, view reported token usage
by project, and export an aggregate CSV. The default executable runs only when
you invoke it and exits when the command finishes.

## Start here

Build this working-tree version with the Go toolchain specified in `go.mod`:

```powershell
go build -o qlog.exe ./cmd/qlog
.\qlog.exe import
.\qlog.exe
.\qlog.exe export > usage.csv
```

On Linux or macOS, use `go build -o qlog ./cmd/qlog` and `./qlog`.

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

[Simple mode guide](docs/SIMPLE.md) explains storage, privacy, import limits,
and what the counters mean.

## What the numbers mean

Tokens come from cumulative counters reported in available Codex session
files, labeled `agent_reported`. Usage samples are not a count of model calls.
Missing counters remain unavailable. Costs and subscription spend are
unavailable; the tool does not invent prices or treat missing costs as zero.

The importer retains selected metadata and usage counters, not prompt,
response, tool argument, or tool result content. Files stay local, and the
default executable has no background service, startup registration, hooks,
agent configuration, or MCP server.

## Earlier version

This source change narrows the default product. It does not replace an existing
installed collector or change a published release. Existing release downloads
and installers belong to the earlier product until a new release is published.
Use the source build above for this version.

The previous ledger and implementation are preserved. The old entry point is
available in source as `cmd/qlog-legacy`; the old installation and capture
guides are historical reference. See the [preserved README](docs/legacy/README-v0.4.md)
and [architecture decision](docs/architecture/ADR-008-on-demand-product.md).

## License

MIT. See [LICENSE](LICENSE).
