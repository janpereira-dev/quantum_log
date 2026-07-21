# M4 Evidence

M4 is `IN_PROGRESS` until this file records real Copilot-originated OTLP data flowing into SQLite and `qlog usage project`.

## Current State

| Adapter | State | Evidence |
|---|---|---|
| copilot-vscode | CAPTURE_EXPERIMENTAL | Setup exists. OTLP receiver tests are synthetic until a real Copilot run is recorded below. |
| opencode | CAPTURE_EXPERIMENTAL | Plugin path exists; real usage depends on payload usage fields. |
| codex | CAPTURE_EXPERIMENTAL | `rawResponse/completed` path exists; real usage depends on non-null usage. |
| claude-code | LIFECYCLE_ONLY | Lifecycle hooks exist; token capture is not claimed. |

## Required Copilot Verification

- [ ] `qlog setup copilot-vscode --yes` installs settings with content capture disabled.
- [ ] `qlog collector start` leaves a reachable loopback collector.
- [ ] Real Copilot VS Code emits an OTLP span or event to qlog.
- [ ] SQLite contains a Copilot-originated `model.call` with `capture_quality=otel_reported`.
- [ ] `qlog usage project <slug>` shows model and tokens for the target project, or explicitly records `unattributed` if Copilot did not provide reliable project context.
- [ ] No prompt, response, tool arguments, tool results, secrets, or authorization fields are persisted.

## Evidence Log

Pending real Copilot run.
