# Copilot CLI Source Contract

Primary source: [GitHub Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference),
OpenTelemetry monitoring section.

## Current status

Copilot CLI is unsupported for stable capture. The implemented transport remains
OTLP HTTP and is experimental/unverified: an isolated real Copilot CLI 1.0.78
probe reached a healthy qlog collector boundary but persisted zero raw events and
zero model calls. A successful Copilot command is not source evidence.

No replacement is authorized by ADR-006. The documented file exporter is
diagnostic only because its real output included the `gen_ai.tool.definitions`
attribute name with content capture false, while the value was deliberately not
retained. That leaves the privacy veto unresolved.

## Implemented configuration

Current Windows install writes qlog-owned PowerShell profile blocks. The managed
`copilot` function sets these values only in the child process environment and
restores the previous process values after Copilot exits:

- `COPILOT_OTEL_ENABLED=true`
- `COPILOT_OTEL_EXPORTER_TYPE=otlp-http`
- `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318`
- `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`
- `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false`

That path requires a reachable collector. Install/uninstall owns and removes only
the delimited PowerShell profile blocks and its profile ownership state while
preserving unrelated profile content. Registry/HKCU environment behavior is
legacy cleanup only: current install does not create persistent current-user
environment values, while uninstall may remove values proven to be owned by an
older qlog installation. These statements describe existing setup behavior, not
verified telemetry delivery.

## Evidence gate

Promotion requires a pinned producer version to deliver real model and
non-negative token evidence with session/project correlation through an accepted
transport. The privacy scan must prove that prompt, response, message, tool
definition/argument/result, credential, environment-value, and path values do not
reach the ledger or an unbounded transient store. Missing values remain missing;
zero is reported only when explicitly emitted.

Any future capture path must fail closed on a privacy or growth-cap violation by
discarding/quarantining capture and marking usage unavailable. It must not alter
the Copilot command result; the upstream agent result remains authoritative.

See [`docs-int/verification/copilot-transport-spike.md`](../../../docs-int/verification/copilot-transport-spike.md).
