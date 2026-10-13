# Current product architecture

The default source executable is an on-demand local Codex usage ledger.
See [ADR-008](../../docs/architecture/ADR-008-on-demand-product.md).

```text
Existing local Codex sessions
    -> passive file importer (metadata and usage whitelist)
    -> canonical usage evidence and privacy policy
    -> separate simple ledger (existing SQLite storage)
    -> project summaries / aggregate CSV
```

`cmd/qlog` calls `internal/simplecli`. Running without arguments reports usage;
import creates its own ledger automatically. Commands exit after completion.
No collector, startup registration, hooks, agent configuration, or MCP server
belongs to this default runtime. Import does not modify source session files.

## Invariants

- Local-first and usable offline, independent of telemetry availability.
- Append-only evidence with existing duplicate prevention and hash chain.
- Prompts, responses, tool arguments/results, and file content are not retained.
- Usage counters retain their source quality; missing cost is unavailable.
- Usage sample count is not actual model-call count.
- Default storage is separate from the earlier ledger; no automatic migration.
- An explicit `--home` can deliberately target an existing ledger.
- Aggregate exports omit source paths and raw conversation content.

Directory import defaults to files modified in the last seven days, with a
maximum of 100 eligible files imported per invocation. This bounds import
selection, not directory traversal or the size of each file. Explicit files
are not filtered by that window.

## Parked historical components

`cmd/qlog-legacy` preserves the earlier entry point. Its receiver, integration
setup, account allocation, telemetry, diagnostics, and service management
remain in source as historical work. They are not required by the new default
CLI. The historical domain and telemetry models in this directory describe
that broader design and must not expand the current product implicitly.

Published releases and existing installations stay on their current behavior
until a new release is validated and installed. Runtime measurements require
an identified build, source size, and hardware; architectural narrowing alone
does not establish low resource consumption.
