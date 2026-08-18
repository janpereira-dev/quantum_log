# ADR-005: The collector is transport infrastructure, not the ledger core

Status: current-state record; replacement direction proposed

## Product boundary

QUANTUM_LOG is a local ledger of AI-agent activity and measured usage. Its core
responsibility is to answer which interaction occurred, in which project, when,
and with how many reported input, cached, and output tokens, tool calls, duration,
outcome, and estimated cost. It is not a memory engine, context-injection system,
RAG service, or permanently running agent platform.

The product rule is **ledger first**. Collectors, hooks, plugins, wrappers, and MCP
servers are ingestion transports. None of them is the conceptual center of the
product.

## Current architecture

Some supported sources push OpenTelemetry data to a loopback HTTP receiver:

```text
agent -> OTLP HTTP -> persistent qlog collector -> SQLite ledger
```

The default endpoint is `http://127.0.0.1:4318`. A process must be listening when
the source emits an event. If it is not:

- the agent continues because telemetry is best-effort;
- qlog does not receive the event;
- the missing prompt, model call, or token counters cannot be reconstructed
  reliably later.

Therefore a persistent collector is technically required for sources whose only
supported transport is push-based OTLP. It is not inherently required for a hook,
plugin, wrapper, or MCP integration that can start qlog on demand.

## Why `qlog.exe` appears in Windows startup applications

`qlog setup --yes` currently attempts to install the loopback collector as a
per-user scheduled task named `QUANTUM_LOG Collector`. When Windows denies task
creation specifically with an access-denied result, qlog installs a per-user
fallback at:

```text
HKCU\Software\Microsoft\Windows\CurrentVersion\Run
QUANTUM_LOG Collector = "...\qlog.exe" collector serve ...
```

That registry value is the `qlog.exe` entry shown by Windows under Startup apps.
The fallback also starts the collector immediately and persists qlog-owned state
so status, restart, and uninstall can identify the managed process.

This behavior entered in [PR #22](https://github.com/janpereira-dev/quantum_log/pull/22),
commit [`9b69f8a`](https://github.com/janpereira-dev/quantum_log/commit/9b69f8a3125308245050aad2020f05e2862a5fa6),
to keep collection running when Task Scheduler policy blocked registration.
[PR #28](https://github.com/janpereira-dev/quantum_log/pull/28) subsequently had
to harden setup and recovery around an already running collector.

## Operational cost and architectural debt

Making the collector persistent and implicit expanded `setup` beyond adapter
configuration. It now participates in:

- service installation and fallback selection;
- scheduled-task and registry ownership;
- detached-process identity and PID state;
- bounded startup and `/healthz` polling;
- stop, restart, recovery, and reinstall behavior;
- coordination with SQLite ownership and setup mutations;
- platform-specific cleanup and tests.

This is coherent with the current push-based transport, but it is
disproportionate to the ledger's main responsibility. Infrastructure was added to
keep the receiver alive, followed by more infrastructure to control the lifecycle
and locking consequences of that receiver. The complexity is real product debt,
not a core ledger capability.

There is also a consent problem. `qlog setup --yes` currently initializes the
ledger, configures adapters, installs a background process, creates a scheduled
task or registry fallback, starts a process, and verifies health. Those side
effects must remain explicit in documentation while this behavior exists. A user
must not discover persistence only through Windows Startup apps.

## Which integrations need a persistent process

| Integration model | Persistent collector required? | Reason |
| --- | --- | --- |
| Direct qlog hook or plugin invocation | No | The source starts qlog for the event. |
| MCP over stdio | No | The parent agent owns the MCP process lifecycle. |
| OTLP HTTP push | Yes, with the current design | Events are emitted only to a listening receiver. |

Engram's usual MCP-over-stdio operation is a useful lifecycle comparison: the
agent starts the MCP server for its session and terminates it afterward. That does
not make Engram permanently active. Engram's HTTP or autosync modes likewise need
a supervised process when permanent availability is requested; see its
[documentation](https://github.com/Gentleman-Programming/engram/blob/main/DOCS.md)
and [architecture](https://github.com/Gentleman-Programming/engram/blob/main/docs/ARCHITECTURE.md).
QUANTUM_LOG must copy neither Engram's memory model nor its product scope; the
relevant lesson is only that the parent process can own an on-demand stdio
lifecycle.

## Current decision

The current collector cannot simply be removed. Doing so before replacing the
transport would silently lose OTLP evidence from sources such as Codex or Copilot
VS Code while leaving those agents apparently healthy.

Until a replacement is implemented:

1. The collector remains the receiver for integrations that require OTLP HTTP.
2. Startup persistence and its cleanup must be documented as visible setup side
   effects.
3. Task Scheduler remains preferred on Windows. The `Run` key remains only the
   access-denied fallback, not a second independently selected service mechanism.
4. Capture failures remain best-effort and never break the upstream agent.
5. Collector health proves only receiver availability, not real-agent token
   capture.

## Proposed simplification direction

This direction is a product proposal, not behavior implemented by this ADR:

```text
hook or plugin   -> qlog hook/ingest -> short SQLite write
MCP client       -> qlog mcp over stdio -> SQLite ledger
OTLP-only source -> explicitly enabled collector -> SQLite ledger
```

The intended operating contract is:

- prefer direct per-event ingestion for Claude Code, Copilot CLI, and OpenCode
  when their supported hooks or plugins expose enough evidence;
- prefer agent-managed `qlog mcp` over stdio where MCP is the supported lifecycle;
- retain the collector only for sources that genuinely require OTLP HTTP;
- make durable collector installation an explicit opt-in, for example a future
  `qlog collector enable` or `qlog setup --with-collector` contract;
- make default setup disclose planned persistence and avoid installing startup
  entries unless that consent was explicit;
- keep one managed mechanism per installation and never add an automatic fallback
  for errors other than the narrowly defined Task Scheduler access denial;
- keep SQLite opens short for direct writes and preserve cooperative locking, WAL,
  and bounded busy-timeout behavior.

The exact CLI flags require a separate implementation decision. Documentation
must not claim that the collector is optional by default until the code actually
enforces that contract.

## Removal and diagnostics

Users can inspect and remove qlog-owned collector persistence without deleting the
ledger:

```powershell
qlog collector status --json
qlog collector logs
qlog collector uninstall --json
```

On Windows, uninstall removes the qlog-owned scheduled task or only the
`QUANTUM_LOG Collector` registry value and its managed state. It must not remove
unrelated startup entries. Removing the collector while an OTLP-only adapter
remains configured causes silent gaps in future capture; disable or replace that
adapter transport first.

## Consequences

- The repository states honestly why the startup entry exists.
- Current runtime behavior remains unchanged by this ADR.
- The permanent collector is treated as transport debt, not as a product feature
  that every adapter must inherit.
- Future simplification has a boundary: preserve measured evidence before removing
  lifecycle infrastructure.
- No release may describe setup as lightweight or non-persistent while it still
  installs a scheduled task or Windows `Run` entry.
