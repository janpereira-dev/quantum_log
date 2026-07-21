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

### 2026-07-21 Task 6 Verification Attempt

M4 remains `IN_PROGRESS`. Automated verification passed, setup installed Copilot OTel settings with content capture disabled, and the collector accepted only supported OTLP content types. A real VS Code Copilot Chat/Agent message was not generated from this CLI-only environment, so no Copilot-originated model call was recorded in SQLite.

#### Automated Verification

| Command | Result |
|---|---|
| `go test -count=1 ./...` | PASS. All packages completed successfully. |
| `go vet ./...` | PASS. No output. |
| `golangci-lint run` | PASS. No output. |
| `git diff --check` | PASS. No output. |

#### Copilot Setup and Collector Checks

Commands were run from `C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-full-recovery` with `QLOG_HOME=C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-task6-qlog-home` to isolate verification data from the user's default ledger.

| Command | Result |
|---|---|
| `go run ./cmd/qlog init` | PASS. Initialized isolated ledger at `C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-task6-qlog-home`. |
| `go run ./cmd/qlog project register --path . --name QUANTUM_LOG` | PASS. Registered `quantum-log` at this worktree path. |
| `go run ./cmd/qlog setup copilot-vscode --yes` | PASS. Updated `C:\Users\cowbo\AppData\Roaming\Code\User\settings.json`. |
| `go run ./cmd/qlog collector install` | PASS. Installed user-session collector state under `C:\Users\cowbo\AppData\Local\QUANTUM_LOG\collector`. |
| `go run ./cmd/qlog collector start` | PASS. Started loopback collector with pid `43052`. |
| `go run ./cmd/qlog adapter verify copilot-vscode --json` while collector was running | FAIL. `ready=false`; database stage failed with `quiescence lock is held by an active qlog client; retry after it exits`. |
| `curl.exe -s -o NUL -w "%{http_code}" -H "Content-Type: text/plain" --data "unsupported" http://127.0.0.1:4318/v1/traces` | PASS. Returned `415`, preserving unsupported OTLP content-type rejection. |
| `go run ./cmd/qlog usage project quantum-log --json` while collector was running | FAIL. Command returned `writer lock is held by an active qlog process; retry after it exits`. |
| `go run ./cmd/qlog collector logs` | PASS. Logged `qlog collector listening on http://127.0.0.1:4318 (/v1/traces OTLP JSON, /v1/events qlog JSON)`. |
| `go run ./cmd/qlog collector stop` | PASS. Stopped collector to release locks after the CLI-only attempt. |
| `go run ./cmd/qlog adapter verify copilot-vscode --json` after stopping collector | FAIL as expected without a real Copilot event. `ready=false`; `copilot_model_call` failed with `requires recent Copilot-originated otel_reported model call with tokens in local storage`. |
| `go run ./cmd/qlog usage project quantum-log --json` after stopping collector | PASS. Returned no rows and `total_tokens=0`. |

#### Privacy and Safety Checks

| Check | Result |
|---|---|
| VS Code setting `github.copilot.chat.otel.captureContent` | PASS. Confirmed `false`. |
| VS Code setting `github.copilot.chat.otel.otlpEndpoint` | PASS. Confirmed `http://127.0.0.1:4318`. |
| Collector default bind address | PASS. Code and tests keep default `127.0.0.1:4318`. |
| Unsupported OTLP content type | PASS. Live collector returned `415`. |
| Sensitive fields | PASS by existing tests and code inspection: prompt, response, tool arguments, tool results, authorization, token, and secret-family fields are stripped before persistence. No real Copilot content was persisted in this attempt. |

#### Pending Manual E2E

Manual VS Code Copilot E2E is still required before M4 can move out of `IN_PROGRESS`:

1. Start from this worktree on Windows.
2. Run `go run ./cmd/qlog init` with the intended `QLOG_HOME`.
3. Run `go run ./cmd/qlog project register --path . --name QUANTUM_LOG`.
4. Run `go run ./cmd/qlog setup copilot-vscode --yes`.
5. Run `go run ./cmd/qlog collector install`.
6. Run `go run ./cmd/qlog collector start`.
7. Open VS Code in this repository.
8. Send one real GitHub Copilot Chat/Agent message.
9. Run `go run ./cmd/qlog adapter verify copilot-vscode --json`.
10. Run `go run ./cmd/qlog usage project quantum-log --json`.
11. Run `go run ./cmd/qlog collector logs`.

Expected close criteria remain unchanged: `adapter verify` must return `ready=true`, and usage rows must include `capture_quality=otel_reported`, `agent_name` containing `copilot`, `provider=github`, a model, and non-zero tokens.

Project attribution result: pending. No real Copilot-originated OTLP model call was captured in this CLI-only attempt, so attribution could not be proven as `quantum-log` or `unattributed`.
