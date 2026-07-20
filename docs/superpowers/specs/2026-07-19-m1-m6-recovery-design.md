# QUANTUM_LOG Milestone Recovery Design

## Goal

Deliver Milestones 1 through 6 without overstating completion. A milestone is
`VERIFIED` only when every acceptance criterion in `QUANTUM_LOG_MASTER_PROMPT.md`

## Scope

This recovery includes missing implementation and verification for Milestones 1
through 6. Milestone 7 remains out of scope. Public documentation must report the
actual state instead of planned or detection-only capability.

## Milestone State Contract

Each milestone has one state: `NOT_STARTED`, `IN_PROGRESS`, `IMPLEMENTED`,
`VERIFIED`, `BLOCKED`, or `DEFERRED`.

`VERIFIED` requires every acceptance criterion to be `PASS` in an evidence
matrix. `FAIL`, `NOT_RUN`, or `BLOCKED` prevents verification. Files, stubs,
templates, unit tests, and registrations alone do not prove capability.

Each milestone records its matrix at
`docs/verification/milestone-<n>-evidence.md`. The matrix columns are AC ID,
criterion, test, command, result, evidence, and state.

## Delivery Sequence

1. M1 integrity and project attribution.
2. M2 reporting, allocations, pricing, and export correctness.
3. M4 verified technical capture.
4. M3 TUI backed by shared query services.
5. M5 release distribution and clean-runner installation.
6. M6 MCP and agent integration.

Later work may not advertise itself as supported before earlier dependency
milestones are verified. Exploratory work stays isolated and cannot change a
milestone state.

## M1: Integrity and Attribution

Project resolution order is exact: explicit project, `QLOG_PROJECT`, exact CWD
location, Git root, normalized Git remote, longest registered path, adapter hint,
then `unattributed`. Adapter metadata cannot override an explicit or environment
selection.

`qlog doctor` is strictly read-only. It must not create configuration or SQLite
files, apply migrations, modify WAL files, update timestamps, repair data, or
download dependencies. Only `qlog init`, `qlog migrate`, and
`qlog repair --confirm` may mutate schema.

All official QUANTUM_LOG SQLite clients acquire a shared quiescence lock. Writers
also acquire an exclusive writer lock, in that order. `doctor` and `verify`
acquire exclusive quiescence and inspect only a quiescent database. A non-empty
WAL blocks them; an isolated SHM file produces a warning without modification.
Only after those checks may they use `mode=ro&immutable=1`. Checkpoint, recovery,
and anchor rebuild remain explicit maintenance operations.

All source data follows this sequence before persistence: parse, classify,
redact, validate, canonicalize, hash, then persist. Sanitization covers payloads,
resolution evidence, errors, tool metadata, adapter metadata, authorization
headers, cookies, common API keys, credentials in URLs, and environment secrets.

The ledger uses a monotonic sequence and a durable external anchor containing the
head sequence, event count, and final event hash. Verification checks both the
hash chain and anchor. It must detect edits, reordering, duplicate sequences,
intermediate deletion, tail truncation, and database/anchor divergence. The
documentation states that a local attacker able to rewrite both stores can defeat
the local anchor.

## M2: Reporting and FinOps

The query service owns filtering and grouping. CLI, JSON, export, and TUI consume
the same service. Each valid `--group-by` combination changes query grouping and
preserves global totals.

Model calls retain observed tokens and observed cost. Allocations store allocated
tokens, allocated cost, and basis points. Allocation totals cannot exceed 10,000
basis points. Rounding remainder remains unattributed. Grouped allocation reports
must never duplicate observed tokens or cost.

Allocation changes append auditable corrections and support history and revert;
they never delete the prior allocation state. Pricing rules are versioned,
temporal, validated, source-attributed, and non-overlapping. Cost snapshots retain
catalog version, formula, FX evidence, and calculation time. Money uses scaled
integers with checked arithmetic.

Exports provide JSON, NDJSON, and CSV. They sanitize paths and unsafe spreadsheet
cells by default.

## M4: Verified Capture

Capture maturity is declared as `DETECTION_ONLY`, `CAPTURE_EXPERIMENTAL`,
`CAPTURE_PARTIAL`, or `CAPTURE_VERIFIED`. Detection does not imply capture.

OTLP accepts bounded, cancellable loopback-only ingestion by default, preserves
source and receipt timestamps, writes sanitized append-only raw events first, and
deduplicates by adapter and source event identifier. It exposes controlled
lifecycle and health status.

The generic wrapper records only observable process facts. It preserves child
signals, explicit project precedence, and unavailable metrics without inventing
token, model, or cost values.

Two official adapters must complete documented real-capture fixtures, idempotent
install/uninstall/status/test flows, privacy checks, and shared contract tests
before they become `CAPTURE_VERIFIED`.

## M3: Terminal UI

The TUI uses shared application query interfaces and never opens SQLite directly.
It adds required views, keyboard navigation, compact/no-color modes, explicit
quality labels, terminal-size handling, golden coverage, and interaction tests.
Every displayed total must equal the corresponding CLI/JSON query result.

## M5: Distribution

Release configuration must produce tested platform archives, version metadata,
checksums, SBOMs, and provenance evidence. Installation verification runs on clean
Linux, macOS, Windows, and WSL environments: install, version, init, doctor,
update when supported, uninstall, and preservation of user data. Package manager
claims require validated publication and installation evidence. `self-update`
remains unavailable until implemented and tested.

## M6: Agent Integration

The local MCP server exposes versioned `qlog.*` tools with strict input, output,
and error schemas, request IDs, and idempotency keys. It reuses application
services and remains independent of baseline capture.

Task lifecycle tests cover task start, explicit project selection, ingestion,
WorkContext change, additional ingestion, completion, summary, budgets, and
ledger verification. Corrections are auditable and reversible. Multi-agent and
partial-data scenarios prove totals, correlation, and non-invention of metrics.

## Documentation Rules

README, CHANGELOG, release notes, package documentation, and `qlog status` derive
their capability statements from the evidence matrix. They cannot call a feature
implemented or verified based on stubs, templates, detection, isolated unit tests,
or unexecuted commands.

## Verification Gates

Every milestone runs build, tests, vet, CGo-free tests, focused integration tests,
and its acceptance-criterion evidence matrix. Distribution and adapter work add
platform or fixture checks. An independent review verifies evidence before public
state changes from `IMPLEMENTED` to `VERIFIED`.
