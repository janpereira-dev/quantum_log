# Claude Code Source Contract

Source: Claude Code monitoring documentation: https://code.claude.com/docs/en/monitoring-usage

## Supported configuration

`qlog adapter install claude-code` keeps qlog lifecycle hooks and adds trace-only OTel environment settings to Claude Code settings:

- `CLAUDE_CODE_ENABLE_TELEMETRY=1`
- `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`
- `OTEL_TRACES_EXPORTER=otlp`
- `OTEL_METRICS_EXPORTER=none`
- `OTEL_LOGS_EXPORTER=none`
- `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://127.0.0.1:4318/v1/traces`
- `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false`

Logs are disabled because Claude Code documents tool and prompt-related events there. qlog does not enable `OTEL_LOG_TOOL_DETAILS`.

## Ingest contract

qlog accepts only Claude Code trace spans with trace and span IDs, a model, and explicit non-negative documented token attributes. Supported raw keys are `input_tokens`, `output_tokens`, `cache_read_input_tokens`, and `cache_creation_input_tokens`, with compatible GenAI aliases. Every stored value records source, raw key, and `reported` confidence; absent attributes are not represented as zero.

No source E2E evidence is claimed here. Trace export is documented beta functionality. Validate after a real local Claude Code session with `qlog adapter verify claude-code`.
