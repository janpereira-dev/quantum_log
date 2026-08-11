# Capability-Aware Reports

`qlog report` shows evidence quality and metric coverage without fabricating unavailable data.

`qlog log today`, `qlog log list`, `qlog log tail`, and `qlog log show <id>` operate on canonical interactions. Interaction and prompt counts are roots, not model/tool call counts. Token reports retain model-call semantics and do not add parent and child totals together.

## Quick Path

```text
qlog report today
qlog report project <project>
qlog report agent <agent>
qlog report session <session>
```

All forms accept `--from`, `--to`, `--json`, and `--csv`. `today` is a rolling 24-hour window. `qlog report usage` and `qlog report summary` remain available for legacy usage output.

## Human Output

Human output is compact monospace evidence:

```text
MODEL CALLS 4 | TOKENS 120 | LIFECYCLE 8 | TOOL 2 | MCP 0 | ERRORS 0 | UNATTRIBUTED 1/20
SOURCE otlp-http        QUALITY otel_reported    VERSION —            CALLS 4
METRIC input_tokens         80 | 4/4 emitted | zero=0
METRIC reasoning_tokens     — | 0/4 emitted | zero=0
```

| Marker | Meaning |
|---|---|
| `0` | Source explicitly reported zero. |
| `—` | Metric was not emitted. |
| `?` | Partial or unreconciled metric coverage. |

Lifecycle-only evidence appears in summary counts even when no model calls exist. It does not create empty model rows or synthetic tokens.

## Machine Output

JSON retains nullable metric `value`, `state`, coverage counts, and each emitted metric's source, raw key, and confidence. CSV emits one row per metric/provenance record with same fields.

`total_tokens` is retained as source-reported provenance when emitted. It is not added to component-token totals, preventing a second total from being counted.

## Limits

Source/version values appear only when persisted. `—` means unavailable, not inferred. Tool, MCP, and error counts only reflect recognized stored event types. Reports do not claim agent E2E evidence or a source capability that has not been recorded.
