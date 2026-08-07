# Codex Source Contract

qlog configures Codex's documented `[otel]` log exporter at `http://127.0.0.1:4318/v1/logs` with binary OTLP/HTTP and `log_user_prompt = false`.

Source: [Codex configuration reference](https://developers.openai.com/codex/config-reference) and [advanced configuration](https://developers.openai.com/codex/config-file/config-advanced).

## Accepted Evidence

qlog accepts a Codex OTLP log record only when all of these are present:

- Resource `service.name = codex`.
- `event.name = codex.sse_event`.
- `event.kind = response.completed`.
- Trace ID and span ID.
- `model`, `input_tokens`, and `output_tokens` with non-negative values.

When present, `cached_input_tokens` and `reasoning_output_tokens` are stored with their raw key, `otel` source, and `reported` confidence. Missing fields remain not emitted. These accepted keys are qlog's current constrained contract, not a claim that every Codex release emits every key.

qlog does not enable prompt logging, persist exporter headers, or collect tool inputs/results. Real-agent source E2E evidence remains required.
