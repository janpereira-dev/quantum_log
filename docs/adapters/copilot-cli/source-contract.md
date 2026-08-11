# Copilot CLI Source Contract

Source: GitHub Copilot CLI command reference, OpenTelemetry monitoring section: https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference

## Supported configuration

`qlog adapter install copilot` creates a qlog-owned `qlog-otel.env` beside its qlog-owned hook configuration. On Windows, it discovers the active Windows PowerShell `CurrentUserCurrentHost` profile with `powershell.exe -NoProfile -NonInteractive -Command $PROFILE.CurrentUserCurrentHost`, then adds a delimited qlog-owned OTel block to that exact profile. This supports redirected Documents locations, including OneDrive. It also writes matching qlog-owned current-user environment values as a complementary setting. Existing terminals retain their inherited environment; close them and launch a new PowerShell after setup. Uninstall removes only the qlog-owned profile block and environment values, preserving unrelated profile content and user-modified values. qlog backs up a preexisting profile before changing it.

Copilot CLI documents hooks and OTel environment variables, but not arbitrary persistent OTLP settings. On non-Windows shells, source `qlog-otel.env` before starting `copilot`.

- `COPILOT_OTEL_ENABLED=true`
- `COPILOT_OTEL_EXPORTER_TYPE=otlp-http`
- `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318`
- `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`
- `OTEL_METRICS_EXPORTER=none`
- `OTEL_LOGS_EXPORTER=none`
- `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false`

Only traces are enabled because this local receiver currently accepts traces and logs, not OTLP metrics. Hooks remain installed for lifecycle and CWD evidence. qlog does not configure headers, file export, prompt capture, response capture, tool arguments, tool results, credentials, or authorization fields.

## Ingest contract

qlog accepts only local OTLP spans with trace and span IDs, a Copilot service identity, a model identity, and explicitly emitted non-negative token attributes. Every accepted token value retains its emitted raw key and `reported` provenance. An absent attribute remains absent; emitted `0` remains reported zero.

No source E2E evidence is claimed here. Validate with `qlog adapter verify copilot` after a real local Copilot CLI session.
