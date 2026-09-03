# Copilot CLI Source Contract

Primary source: [GitHub Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference),
OpenTelemetry monitoring section.

## Decision and current state

ADR-006 selects the documented file exporter for the next Copilot CLI adapter
implementation. This decision is evidence-bound to GitHub Copilot CLI 1.0.78 on
Windows 11 x64. It does not claim that the current qlog adapter already uses this
transport: the existing setup path still configures OTLP HTTP and therefore
retains its collector dependency until Task 8 replaces it.

## Supported configuration boundary

The selected launcher will set only process-scoped values for the child Copilot
CLI process:

- `COPILOT_OTEL_FILE_EXPORTER_PATH=<qlog-owned external path>`
- `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false`

Setting the file path automatically enables the documented file exporter. The
launcher must not modify a shell profile, user or machine environment, Copilot
configuration, credentials, or VS Code settings; content capture remains false.
Uninstall removes only the qlog-owned telemetry file, durable import checkpoint,
and launcher registration created by this adapter.

## Ingest contract

Import is incremental and durable. The importer tracks file identity and byte
offset, accepts only complete JSONL records, commits a sanitized batch before
advancing its checkpoint, and safely replays after interruption. Truncation or
rotation starts a separately identified stream; it never silently skips bytes.
The implementation must bound retained source-file growth.

Only allowlisted identity, correlation, timing, model, and non-negative numeric
usage fields may reach the ledger. Accepted token values retain their emitted raw
key and `otel_reported` provenance. Missing attributes remain missing and emitted
zero remains reported zero. Prompt, response, message, tool definition,
tool-argument/result, credential, environment-value, and path values are dropped
before persistence.

Because Copilot writes locally and qlog imports later, this transport does not require a persistent collector.
Export or import failure remains best-effort and
must not change the Copilot command result.

## Evidence and drift gate

The 2026-09-03 spike observed 2 spans, 8 metrics, model/session/token attribute
names, and sanitized schema SHA-256
`1f891d72e02a6d1765098848a1d1a517ab37b2cc134663ff44fdd4c61c8bde8c`.
See [`docs-int/verification/copilot-transport-spike.md`](../../../docs-int/verification/copilot-transport-spike.md).

Any producer-version or schema change fails the evidence gate until a new
privacy-safe fixture proves required fields and forbidden content handling.
