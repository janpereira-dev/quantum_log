# Copilot CLI Source Contract

> Historical integration contract for `cmd/qlog-legacy`. The current product
> supports local Codex session files; see the [current guide](../../SIMPLE.md).

Primary source: [GitHub Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference),
OpenTelemetry monitoring section.

## Current status

Copilot CLI remains unsupported for stable capture. The implemented transport
remains OTLP HTTP over verified HTTPS. On 2026-11-01, the official
portable [CLI 1.0.105](https://github.com/github/copilot-cli/releases/tag/v1.0.105)
on Windows completed an isolated, offline synthetic task using the documented
[local provider and offline mode](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-llm-models).
Its native HTTPS OTLP exporter delivered two
raw events and one model call with 10 input tokens, 5 output tokens and 15 total
tokens to the collector built from `f2e2d77`. A foreign self-signed CA and a
plaintext port squatter received zero HTTP exporter requests. No account or real
provider credentials were used. This verifies the native transport and reported
usage from a synthetic response, not authenticated provider execution, project
attribution, or the full stable-capture evidence gate. The earlier CLI 1.0.78
probe that persisted zero events remains historical evidence for that version.

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
- `OTEL_EXPORTER_OTLP_ENDPOINT=https://127.0.0.1:4318`
- `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`
- `OTEL_EXPORTER_OTLP_CERTIFICATE=<legacy-home>/collector-tls/cert.pem`
- `OTEL_EXPORTER_OTLP_HEADERS` read from the protected collector token file
- `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false`

The launcher also overrides endpoint, protocol, certificate and headers for
each signal so ambient trace, log or metric settings cannot redirect the
managed token. The token is passed through the child environment, not arguments.

CLI 1.0.105's `copilot help monitoring` documents that its native Rust exporter
adds this CA to OS trust. A controlled child-only `SSL_CERT_FILE` /
`SSL_CERT_DIR` experiment also allowed an explicitly selected foreign CA. No OS
trust stores were changed. This is TLS verification of the literal loopback
endpoint, not exclusive certificate pinning: a port-only attacker with an
untrusted certificate is rejected, while control of client trust, configuration,
or an already trusted CA remains outside this boundary. No documented
OTLP-specific option to disable the other roots was found. Global root overrides
are not installed because they also affect the client's provider connections.
The native metric exporter receives `404` from the unsupported `/v1/metrics`
route; this produces a shutdown warning but did not prevent trace or usage
persistence or change the successful command result.

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
