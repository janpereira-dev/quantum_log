# GitHub Copilot for VS Code Source Contract

Source: VS Code Copilot telemetry settings and OpenTelemetry GenAI semantic conventions.

## Supported Configuration

`qlog adapter install copilot-vscode` manages only these user settings:

- `github.copilot.chat.otel.enabled = true`
- `github.copilot.chat.otel.exporterType = "otlp-http"`
- `github.copilot.chat.otel.otlpEndpoint = "http://127.0.0.1:4318"`
- `github.copilot.chat.otel.captureContent = false`

Existing user settings are preserved, repeated installation is byte-identical when settings already match, and uninstall restores values qlog changed. qlog does not configure headers, prompt capture, response capture, tool arguments, tool results, credentials, or authorization fields.

## Ingest Contract

qlog accepts a Copilot span only with trace and span IDs, a recognized Copilot service identity, a model, and at least one explicitly emitted non-negative token metric. It prefers `gen_ai.conversation.id` as session identity and retains `session.id` only as sanitized window-session metadata when different.

Project attribution follows explicit project, local CWD, then a unique registered Git root plus normalized remote. A remote URL is never treated as a local directory and is not persisted. Ambiguous or unmatched Git context remains unattributed.

Accepted token metrics retain their emitted GenAI raw key with `otel` source and `reported` confidence. Missing attributes remain not emitted; emitted `0` remains reported zero. Real VS Code session evidence remains required for full E2E acceptance.
