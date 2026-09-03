# Copilot transport decision spike

Date: 2026-09-03

This record contains sanitized decision evidence only. It deliberately excludes
the prompt, response, tool inputs and outputs, raw telemetry values, credentials,
user paths, logs, and raw JSONL.

## Environment and method

| Field | Observation |
| --- | --- |
| OS | Windows 11 x64 |
| Copilot CLI | GitHub Copilot CLI 1.0.78 |
| Editor | Visual Studio Code 1.136.0, commit `520fb30b2d3d324b4cb2342f6e88e2cd93751de1`, x64 |
| CLI documentation check | `copilot help monitoring` matched the official file-exporter and content-capture contract |
| CLI activation | Only process-scoped `COPILOT_OTEL_FILE_EXPORTER_PATH`; `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false` |
| CLI action | Minimal authenticated real prompt; no repository context or tool action was requested |
| Command result | exit code `0`; response output existed but was not retained or recorded |
| Cleanup | Raw JSONL was deleted after in-memory sanitization; the external temporary directory was removed |

No global profile, user environment, Copilot configuration, VS Code setting, or
repository configuration was changed. No extension was installed.

## Sanitized CLI result

The file exporter created 10 JSONL records: 2 spans and 8 metrics.

Record names and counts:

| Signal | Name | Count |
| --- | --- | ---: |
| span | `invoke_agent` | 1 |
| span | `chat auto` | 1 |
| metric | `gen_ai.invoke_agent.tool_calls` | 1 |
| metric | `gen_ai.invoke_agent.inference_calls` | 1 |
| metric | `gen_ai.client.token.usage` | 1 |
| metric | `gen_ai.client.operation.duration` | 1 |
| metric | `gen_ai.client.operation.time_per_output_chunk` | 1 |
| metric | `gen_ai.client.operation.time_to_first_chunk` | 1 |
| metric | `gen_ai.invoke_agent.duration` | 1 |
| metric | `github.copilot.agent.turn.count` | 1 |

Observed attribute names, sorted (values were neither recorded nor retained):

```text
enduser.pseudo.id
gen_ai.agent.id
gen_ai.agent.version
gen_ai.conversation.id
gen_ai.operation.name
gen_ai.provider.name
gen_ai.request.model
gen_ai.request.stream
gen_ai.response.finish_reasons
gen_ai.response.id
gen_ai.response.model
gen_ai.response.time_to_first_chunk
gen_ai.token.type
gen_ai.tool.definitions
gen_ai.usage.cache_creation.input_tokens
gen_ai.usage.input_tokens
gen_ai.usage.output_tokens
gen_ai.usage.reasoning.output_tokens
github.copilot.agent.type
github.copilot.context.custom_agent_names
github.copilot.context.skills
github.copilot.cost
github.copilot.current_tokens
github.copilot.initiator
github.copilot.interaction_id
github.copilot.messages_length
github.copilot.nano_aiu
github.copilot.server_duration
github.copilot.service_request_id
github.copilot.token_limit
github.copilot.turn_count
github.copilot.turn_id
github.copilot.user.message.interaction_id
github.copilot.user.message.source
service.name
service.version
```

The SHA-256 of the sanitized schema comprising sorted signal types, record names,
and attribute names is
`1f891d72e02a6d1765098848a1d1a517ab37b2cc134663ff44fdd4c61c8bde8c`.

Privacy scan outcome:

- prompt literal: absent
- response literal: absent
- credential markers: absent
- tool argument/result keys: absent
- content-capture setting: `false`
- JSONL parse failures: zero

The presence of the `gen_ai.tool.definitions` attribute name means the importer
must discard its value rather than treating capture-content false as a complete
downstream allowlist.

## VS Code result

No Copilot extension was installed in Visual Studio Code 1.136.0. No VS Code agent turn was run,
because installing or authenticating an extension was outside
this privacy-safe spike. Consequently there is no event count, attribute-name
set, or schema hash for the editor product. The documented file exporter remains
a candidate, not observed evidence, and VS Code stays unsupported for stable
capture until a pinned extension/device run exists.

## Sources and conclusion

- [GitHub Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference)
- [GitHub OpenTelemetry for agent monitoring](https://docs.github.com/en/copilot/concepts/agents/opentelemetry)
- [VS Code: Monitor agent usage with OpenTelemetry](https://code.visualstudio.com/docs/agents/guides/monitoring-agents)

The real CLI result supports the documented file exporter as the preferred
durable local boundary. The missing editor extension evidence blocks a truthful
VS Code decision.
