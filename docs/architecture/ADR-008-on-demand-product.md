# ADR-008: Make the default product an on-demand local ledger

Status: accepted for the source implementation; release validation pending
Date: 2026-10-13

## Context

The earlier product combined multiple agent capture transports, service
lifecycle management, agent setup, accounting, reporting, and diagnostics.
This exceeded the intended small local utility. The user authorized radical
scope reduction while preserving useful work and existing data.

## Decision

`cmd/qlog` becomes the entry point for an on-demand CLI with three operations:
import local Codex session files, report imported usage, and export aggregate
CSV. Running it without arguments reports usage. Import initializes only its
own data directory; reports do not require a separate setup step.

The importer reads existing local session files. Directory discovery defaults
to the last seven modified days and at most 100 eligible files per invocation.
It retains whitelisted metadata and reported usage, not conversation content.
Cumulative usage snapshots provide token evidence; a usage sample is not an
actual model-call count. Cost and subscription spend remain unavailable.

The default executable does not start a collector or server, register startup,
install hooks, edit agent configuration, or expose MCP. It exits after each
command. No additional agent integrations are part of this product slice.

## Preservation and compatibility

The earlier entry point is retained as `cmd/qlog-legacy`. Its implementation,
tests, architecture records, and documentation remain source reference. The
old README is preserved verbatim under `docs/legacy/README-v0.4.md`.

The new default data directory is a separate `simple` directory. There is no
automatic migration, deletion, or reinterpretation of the existing ledger.
An explicit `--home` can target an existing ledger; default isolation is not
a restriction on that deliberate override. The CLI reuses existing append-only
storage, privacy controls, idempotency, and hash-chain mechanisms. Existing
migrations remain; this decision narrows the runtime, not historical source.

The command surface changes for source builds. Existing published releases,
installers, and installed services do not change merely because this source
entry point changes. Documentation directs users to build from source until a
release of the new product passes its release gates.

## Consequences

Less continuous capture and fewer supported agents reduce setup and maintenance
work. Users must import again to refresh their reports. Evidence is bounded by
the local files that exist, and schema changes require explicit importer
maintenance. An explicit file may be large even with bounded discovery.

Legacy components remain parked rather than deleted. Their presence in the
repository is not a promise that they belong to the default executable, nor
does it prove that the new runtime has met a resource budget.

## Validation and release gates

- Verify repeated import does not duplicate usage and cumulative snapshots do
  not double count tokens.
- Verify malformed input fails safely and content fields are not persisted.
- Verify report, local-day/project filters, and CSV/JSON output contracts.
- Verify default storage stays separate from the older ledger.
- Validate a real available Codex session file and record the exact build,
  fixture size, elapsed time, and memory measurements before claiming resource
  improvement.
- Distinguish unit/fixture coverage from real agent end-to-end evidence.

The earlier release's five-agent validation matrix is historical. It is not
the acceptance matrix for this narrowed product or proof of its runtime.
