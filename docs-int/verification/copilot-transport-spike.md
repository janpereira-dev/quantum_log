# Copilot transport decision spike

Date: 2026-09-03

Only sanitized counts, schema names/hashes, privacy results, versions, transport,
and command results are retained. Prompt/response text, attribute values, tool
data, credentials, user paths, raw JSONL, the temporary ledger, and diagnostic
logs were deleted.

## Environment

| Field | Observation |
| --- | --- |
| OS | Windows 11 x64 |
| Copilot CLI | GitHub Copilot CLI 1.0.78 |
| Editor | Visual Studio Code 1.136.0, commit `520fb30b2d3d324b4cb2342f6e88e2cd93751de1`, x64 |
| Local documentation | `copilot help monitoring` inspected |
| Content capture | `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT` was explicitly `false` in each child process |
| Persistent mutation | None; no global profile, user environment, product setting, credential, or extension changed |

## Probe A: process-scoped file exporter

`COPILOT_OTEL_FILE_EXPORTER_PATH` pointed to an external temporary file for one
minimal authenticated command. The command returned exit code `0`. The exporter
produced 10 JSONL records: 2 spans and 8 metrics. Raw JSONL was deleted after the
bounded in-memory scan.

Privacy result: prompt literal: absent; response literal: absent; credential
markers: absent; tool argument/result keys: absent. The attribute name
`gen_ai.tool.definitions` was present. Its type/value/count were not retained, so
the official-documentation contradiction remains unresolved and this is not an
acceptance artifact.

### Canonical sanitized representation

The checked-in canonical JSON below is UTF-8, one line, no BOM, with object keys
lexicographically sorted, arrays in the shown order, no insignificant whitespace,
and a final newline excluded from hashing. Re-running SHA-256 over exactly the
bytes inside the code block reproduces
`9b5022f32568bc1382e8463bcfed0a62e6c55026de080ce5454debc71a8ac131`.

```json
{"attribute_names":["enduser.pseudo.id","gen_ai.agent.id","gen_ai.agent.version","gen_ai.conversation.id","gen_ai.operation.name","gen_ai.provider.name","gen_ai.request.model","gen_ai.request.stream","gen_ai.response.finish_reasons","gen_ai.response.id","gen_ai.response.model","gen_ai.response.time_to_first_chunk","gen_ai.token.type","gen_ai.tool.definitions","gen_ai.usage.cache_creation.input_tokens","gen_ai.usage.input_tokens","gen_ai.usage.output_tokens","gen_ai.usage.reasoning.output_tokens","github.copilot.agent.type","github.copilot.context.custom_agent_names","github.copilot.context.skills","github.copilot.cost","github.copilot.current_tokens","github.copilot.initiator","github.copilot.interaction_id","github.copilot.messages_length","github.copilot.nano_aiu","github.copilot.server_duration","github.copilot.service_request_id","github.copilot.token_limit","github.copilot.turn_count","github.copilot.turn_id","github.copilot.user.message.interaction_id","github.copilot.user.message.source","service.name","service.version"],"record_name_counts":{"metric::gen_ai.client.operation.duration":1,"metric::gen_ai.client.operation.time_per_output_chunk":1,"metric::gen_ai.client.operation.time_to_first_chunk":1,"metric::gen_ai.client.token.usage":1,"metric::gen_ai.invoke_agent.duration":1,"metric::gen_ai.invoke_agent.inference_calls":1,"metric::gen_ai.invoke_agent.tool_calls":1,"metric::github.copilot.agent.turn.count":1,"span::chat auto":1,"span::invoke_agent":1},"signal_counts":{"metric":8,"span":2}}
```

## Probe B: process-scoped OTLP HTTP to qlog

A `qlog.exe` built from this checkout initialized a temporary `QLOG_HOME` and ran
its collector on an isolated loopback port. Health returned ready before Copilot
started. Only the Copilot child process received the OTLP endpoint/exporter and
content-capture variables. No file exporter was set.

The probe bound both source and executable identity before launch:

- qlog source commit: `19a70213309e9c245d8e31363b17b487af2845a3`
- qlog binary SHA-256: `4b37a3fb2c17c1a4d3a171e19a2f017fcb0470c06720de222d84204765b0a708`
- Copilot version: `1.0.78`
- content capture: `false`

Result:

- collector ready: true
- Copilot exit code: `0`
- qlog raw event count: `0`
- qlog model-call count: `0`
- producer diagnostic HTTP export network-error count: `1` matching diagnostic
  record (the count is by diagnostic line, not repeated marker text within it)
- persisted prompt literal: absent
- persisted credential markers: absent
- persisted `gen_ai.tool.definitions` marker count: `0` (no event arrived)

The temporary qlog database, binary, logs, and home were deleted. Zero received
events means the privacy scan is vacuous and cannot authorize OTLP.

### Canonical OTLP result

The same canonical JSON rules produce SHA-256
`5d4d516174a3b4b4886c6b1b269430184199ecbad3b5630baab7617f405f3064`:

```json
{"collector_ready":true,"content_capture":false,"copilot_exit_code":0,"copilot_version":"1.0.78","model_call_count":0,"network_error_count":1,"qlog_binary_sha256":"4b37a3fb2c17c1a4d3a171e19a2f017fcb0470c06720de222d84204765b0a708","qlog_commit":"19a70213309e9c245d8e31363b17b487af2845a3","raw_event_count":0,"transport":"otlp-http"}
```

## VS Code result

No Copilot extension was installed. No VS Code agent turn was run. Therefore no
editor signal counts, schema, privacy result, or transport acceptance exists.

## Conclusion

- OTLP HTTP failed to deliver evidence to the isolated healthy qlog collector.
- File export proves schema availability but fails the privacy veto because the
  disputed tool-definition value and a hard transient-growth boundary were not
  proven.
- Copilot CLI and VS Code remain unsupported for stable capture.

Sources:

- [GitHub Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference)
- [GitHub OpenTelemetry for agent monitoring](https://docs.github.com/en/copilot/concepts/agents/opentelemetry)
- [VS Code: Monitor agent usage with OpenTelemetry](https://code.visualstudio.com/docs/agents/guides/monitoring-agents)
