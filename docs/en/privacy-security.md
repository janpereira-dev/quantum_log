# Privacy and Security

QUANTUM_LOG is designed to retain local usage evidence without retaining sensitive agent content. This is a system boundary, not permission to ingest arbitrary private data or expose local services carelessly.

[Privacidad y seguridad en español](../es/privacidad-seguridad.md)

## Storage policy

Data stays local by default. Raw events are append-only and chained per source/session. Before hashing or import, sanitization removes prompt and response content, tool arguments and results, secrets, authorization values, API keys, tokens, and related sensitive-key families.

The system intentionally does not store sensitive content. In particular:

- `qlog run` never persists command arguments, environment values, or process output.
- Claude Code hook handling reduces payloads to lifecycle-safe metadata; transcript and prompt fields are not retained.
- Plugin and OTLP ingestion strip sensitive payload and attribute values before storage.

Sanitization reduces risk; it does not make every external input safe to share. Limit collector access and inspect exports before distributing them.

## Capture quality

Capture quality is explicit because data provenance affects what users can conclude.

| Label | Meaning | Do not infer |
| --- | --- | --- |
| `otel_reported` | Accepted OTLP supplied usage fields. | That every provider emitted complete usage. |
| `agent_reported` | Agent integration supplied usage fields. | Provider-billed totals or full transcript evidence. |
| `lifecycle_only` | A lifecycle/process event was captured. | Any token count or cost. |
| `unavailable` | No supported usage evidence is available. | Zero actual usage. |

Reports and exports preserve these labels. QUANTUM_LOG never invents token counts for missing evidence.

## Copilot status

M4 is `IN_PROGRESS`. Copilot VS Code capture is experimental until real end-to-end evidence records Copilot-originated usage in SQLite. A successful configuration install, a collector health check, or generic imported data is not enough.

`qlog adapter verify copilot-vscode` requires a recent local OTLP Copilot model call with `otel_reported` tokens. Until that stage passes against real evidence, describe the integration as experimental and unverified.

## Threat model

QUANTUM_LOG addresses these local risks:

| Risk | Control |
| --- | --- |
| Sensitive content reaches ingestion | Sanitization before import and hashing; narrow plugin/hook payloads. |
| Ledger event is altered | Source/session SHA-256 chain verification. |
| Ledger history is truncated or diverges | Exported external anchors and `anchor check`. |
| Concurrent SQLite access produces unsafe diagnostics | Cooperative quiescence/writer locking and WAL checks. |
| Local collector is exposed accidentally | Loopback-only default; explicit non-loopback opt-in. |
| Attribution is guessed from weak metadata | Explicit project-resolution policy and unattributed state. |

It does not eliminate risks from a compromised host, an authorized local user, secrets already present in metadata that a sanitizer does not recognize, insecure backup storage, or deliberately exposing a collector to a network. Apply operating-system access controls and protect backups and anchor files.

## Safe export and sharing

Use:

```bash
qlog export --format csv --redact-paths > usage.csv
```

Review exported fields, capture-quality labels, project names, timestamps, provider/model metadata, and allocation data before sharing. Redacting paths does not redact every business-sensitive field.

Do not share raw database files, WAL/SHM sidecars, unreviewed anchor files, adapter configuration, or diagnostics containing environment-specific paths unless recipients are authorized and the content has been reviewed.

## Security response

For a security issue, follow [SECURITY.md](../../SECURITY.md). Preserve evidence in place, avoid destructive recovery actions, and do not include secrets or sensitive agent content in public reports.
