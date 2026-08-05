# M4 Evidence

M4 is `IN_PROGRESS`. Synthetic tests and local setup state do not prove real-agent capture.

## Stable Scope And Source-Evidence Gates

M4 stable auto-capture scope is exactly `claude-code`, `codex`, `copilot`, `copilot-vscode`, and `opencode`. Generic JSONL import is retained for manual import, not presented as M4 auto-capture. Do not change an adapter to a reported capture quality until every source-evidence cell is independently reviewed and clean-device real-agent acceptance is recorded.

| Adapter | Target OS | Supported source version | Official source URL | Documented configuration key/hook | Emitted identity | Token fields | Privacy setting | Available quality | Release evidence |
|---|---|---|---|---|---|---|---|---|---|
| claude-code | Windows, macOS, Linux | 2.1.218 | https://docs.anthropic.com/en/docs/claude-code/hooks | documented `hooks` settings configuration and lifecycle JSON command-hook input | lifecycle event | no source-reported usage or token counters | qlog hook sanitization | `lifecycle_only` | IN_PROGRESS: clean-device real-agent lifecycle evidence required |
| codex | Windows, macOS, Linux | 0.145.0 | https://learn.chatgpt.com/docs/config-file/config-advanced | user-level `~/.codex/config.toml` `[otel]` with `otlp-http` exporter to qlog `/v1/logs` | OTLP log trace/span identity, with conversation ID when present | `input_tokens`, `output_tokens`, `cached_input_tokens`, `reasoning_output_tokens` on `codex.sse_event` `response.completed` | `log_user_prompt = false` | `otel_reported` | IN_PROGRESS: clean-device real Codex response.completed evidence required |
| copilot | Windows, macOS, Linux | Copilot CLI 1.0.73 | https://docs.github.com/en/copilot/reference/hooks-reference | qlog-owned user hook file `~/.copilot/hooks/qlog.json`, using `sessionStart`, `sessionEnd`, `agentStop`, and `postToolUse` command hooks | hook `sessionId` and `cwd`, resolved centrally by qlog | no documented source-reported token counters are captured | hook parser discards prompts, responses, tool args/results, secrets, authorization, and tokens | `lifecycle_only` | `BLOCKED_EXTERNAL`: any Copilot CLI PASS is retracted until a real interactive Copilot session proves native hook-origin lifecycle persistence; manual `qlog hook` is invalid evidence |
| copilot-vscode | Windows | VS Code 1.131.0; `code --list-extensions --show-versions` shows no GitHub Copilot extension | https://code.visualstudio.com/docs/agents/guides/monitoring-agents | `github.copilot.chat.otel.enabled=true`, `exporterType="otlp-http"`, `otlpEndpoint="http://127.0.0.1:4318"`, `captureContent=false` | sanctioned `copilot-chat` service with `GitHub Copilot Chat` agent plus OTLP trace/span identity | documented `gen_ai.usage.*` token attributes | prompt, response, and tool content are excluded by default unless explicitly enabled | `otel_reported` | P0-07 `BLOCKED_EXTERNAL`: qlog-managed settings with `captureContent=false` were written from compiled RC in isolated home, but no GitHub Copilot extension/login surface exists and Task Scheduler denied collector creation. Real extension acceptance, persistence/query, replay, and payload privacy inspection remain required. |
| opencode | Windows, macOS, Linux | 1.18.10 | https://github.com/anomalyco/opencode/blob/dev/packages/plugin/src/index.ts | qlog-managed plugin lifecycle events | session lifecycle events and `chat.message` carry `sessionID`; `chat.message` optionally identifies agent and model provider/model IDs | no source-reported usage or token-counter fields exposed by hook interface | plugin allowlist excludes prompt, response, and tool content | `lifecycle_only` | IN_PROGRESS: source evidence supports lifecycle metadata only; no documented usage schema or clean-device real-agent evidence |

OpenCode token capture remains unavailable. Copilot VS Code reported-token handling is limited to its sanctioned OTel identity and documented token attributes.

## Current State

| Adapter | State | Evidence |
|---|---|---|
| copilot-vscode | OTEL_REPORTED | Documented VS Code Copilot OTel configuration and receiver support are ready; GitHub Copilot extension acceptance remains required on another device. |
| opencode | LIFECYCLE_ONLY | Plugin records lifecycle/tool events only; token field names have not been source-verified. |
| codex | OTEL_REPORTED | qlog manages documented user-level Codex OTLP logs; real-agent clean-device acceptance remains required. |
| copilot | LIFECYCLE_ONLY | Official Copilot CLI lifecycle hooks are configured in a qlog-owned user hook file; real CLI action emitted no observable hook event in this environment. |
| claude-code | LIFECYCLE_ONLY | Lifecycle hooks exist; token capture is not claimed. |

Adapter verification is fail-closed. `qlog adapter verify <adapter> --project <slug> --json` emits its stage result and exits non-zero while any required setup, availability, collector, capture-quality, source-evidence, or fresh durable-evidence stage fails. Reported-token quality additionally requires a linked normalized model call with source-reported tokens. Clean-device real-agent evidence keeps Codex unverified; source-evidence gates keep Copilot VS Code and OpenCode unverified.

Reports retain measurement quality. `lifecycle_only` evidence has no model-call or token counters; `unavailable` is not fabricated usage; reported qualities require source counters; and estimated cost remains labelled `estimated_cost_*`. Use `qlog report usage --json`, `qlog usage project <slug> --json`, and `qlog session summary <session-id> --json` to inspect persisted evidence.

Replays are suppressed by durable sanitized ingestion identity before raw-event append and model-call normalization. Acceptance requires no increase to raw-event, model-call, token, or estimated-cost totals. Current qlog JSON event responses expose accepted and duplicates counts, so record the report outputs and durable IDs when proving a replay result.

## Clean-Device Acceptance Protocol

Do not mark M4 released or VERIFIED without a completed record for every source-evidence-complete matrix row on its supported OS/device combination. Synthetic tests, setup files, collector health, and manually injected events never satisfy this gate.

Record each acceptance run with:

- Device OS/version/architecture
- qlog version/hash
- Agent version and official source URL
- Installer consent transcript or manual bootstrap record
- `qlog collector status --json` output
- Sanitized raw-event ID and model-call ID when applicable
- `qlog adapter verify --json` output and exit code before and after replay
- `qlog report usage --json` or `qlog usage project <slug> --json` output
- Replay result
- Privacy inspection result

Run this sequence in a clean project and isolated qlog home:

```text
qlog init
qlog project register --path <clean-project> --name <name>
<consented installer bootstrap or qlog setup --yes>
qlog collector status --json
<one real-agent action>
qlog adapter verify <adapter> --project <slug> --json
qlog usage project <slug> --json
<repeat same upstream event or documented snapshot replay>
qlog adapter verify <adapter> --project <slug> --json
qlog verify
qlog doctor --json
```

For lifecycle-only adapters, first event must create sanitized raw evidence with zero tokens. For a source-evidence-complete reported-token adapter, it must create exactly one linked normalized model call and one quality-labelled report row with source-reported tokens. Replay must be suppressed. Privacy inspection must find no prompt, response, tool data, secret, authorization, or credential-bearing remote URL in persisted payload, evidence, hashes, exports, or logs. If any requirement is absent, retain `IN_PROGRESS`.

## Required Copilot Verification

- [ ] `qlog setup copilot-vscode --yes` installs settings with content capture disabled.
- [ ] `qlog collector start` leaves a reachable loopback collector.
- [ ] Real Copilot VS Code emits an OTLP span or event to qlog.
- [ ] SQLite contains a Copilot-originated `model.call` with `capture_quality=otel_reported`.
- [ ] `qlog usage project <slug>` shows model and tokens for the target project, or explicitly records `unattributed` if Copilot did not provide reliable project context.
- [ ] No prompt, response, tool arguments, tool results, secrets, or authorization fields are persisted.

## Evidence Log

### 2026-08-05 Real Copilot Observations And Provenance Gate

Observed from real-user validation, privacy-safe facts only: one VS Code Copilot OTel token event occurred; provider was `github`; model identity is intentionally omitted; no trusted workspace context was available, so project ownership remains `unattributed`. Do not infer a project from agent, provider, model, branch, commit, or remote URL. A local operator may inspect `qlog unattributed list` and repair a single call only with `qlog unattributed repair <model-call-id> --project <registered-project-slug>` after making an explicit local ownership decision.

Copilot CLI PASS is retracted. Generated hooks now contain Unix `bash`, PowerShell, and generic PowerShell-wrapper commands, but parser/configuration validation does not demonstrate that Copilot invoked them. A valid future acceptance requires an interactive authenticated Copilot session after installation, upstream session evidence, and a matching persisted `copilot-cli-hook` record. Manual `qlog hook copilot-cli` invocation, injected JSON, or replay cannot satisfy this gate.

### 2026-08-01 Codex CLI 0.145.0 OTel Source Evidence

Installed Codex version: `codex-cli 0.145.0`.

Official source: https://learn.chatgpt.com/docs/config-file/config-advanced

qlog-managed user-level configuration in `~/.codex/config.toml`:

```toml
[otel]
exporter = { otlp-http = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "binary" } }
log_user_prompt = false
```

The official documentation states that telemetry is configured at user level, that project-local `otel` configuration is ignored, and that exporters flush on shutdown. It documents `codex.sse_event` log events with token counts on `response.completed`. qlog accepts only a Codex resource plus that event/kind pair, then normalizes only model and documented token fields. Prompt, response, tool, authorization, and secret-bearing fields are not persisted.

M4 remains `IN_PROGRESS`. A clean-device gate is still mandatory: configure the user-level file through qlog, run a real Codex action, capture one source-originated `response.completed` log, verify source-reported usage and replay suppression using trace/span identity, and inspect persisted data for privacy compliance. Synthetic OTLP tests do not satisfy this gate.

### 2026-08-01 OpenCode 1.18.10 Plugin API Source Evidence

Installed OpenCode version: `1.18.10`.

Official source: https://github.com/anomalyco/opencode/blob/dev/packages/plugin/src/index.ts

Verified plugin hook contract:

- Plugin lifecycle event schema is exposed through `event?: (input: { event: Event }) => Promise<void>`; session lifecycle events carry `sessionID`.
- `chat.message` identifies `sessionID`, optional `agent`, and optional `model` with `providerID` and `modelID`.
- The provided hook interface exposes no source-reported usage or token-counter fields.

This source evidence supports lifecycle metadata only. It does not authorize `agent_reported` or any token claim. OpenCode remains `lifecycle_only`, and M4 remains `IN_PROGRESS` pending clean-device real-agent acceptance.

### 2026-08-01 Claude Code 2.1.218 Hooks Source Evidence

Installed Claude Code version: `2.1.218`.

Official source: https://docs.anthropic.com/en/docs/claude-code/hooks

The official documentation describes `hooks` configuration in Claude Code settings and JSON input supplied to command hooks for lifecycle events. This evidence supports qlog-managed lifecycle hooks only. The documented hook input exposes no source-reported usage or token counters, so Claude Code remains `lifecycle_only`.

M4 remains `IN_PROGRESS` pending clean-device real-agent lifecycle evidence. No reported-token quality or usage claim is authorized by this source evidence.

### 2026-08-01 VS Code 1.131.0 Copilot OTel Source Evidence

Installed VS Code version: `1.131.0`.

Official source: https://code.visualstudio.com/docs/agents/guides/monitoring-agents

The official VS Code documentation supports these qlog-managed settings:

```json
{
  "github.copilot.chat.otel.enabled": true,
  "github.copilot.chat.otel.exporterType": "otlp-http",
  "github.copilot.chat.otel.otlpEndpoint": "http://127.0.0.1:4318",
  "github.copilot.chat.otel.captureContent": false
}
```

It documents OTLP trace/span identity and `gen_ai.usage.*` token attributes. Prompt, response, and tool content are excluded by default unless content capture is explicitly enabled.

GitHub Copilot extension is not installed or available on this host. Therefore no supported extension version range, capture-quality promotion, or release evidence is claimed. `copilot-vscode` remains `unavailable`, and M4 remains `IN_PROGRESS` pending extension availability plus clean-device real-agent evidence.

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

### 2026-07-21 Post-Review Readiness Fix Verification

M4 remains `IN_PROGRESS`. This pass verifies the final review fixes without claiming real Copilot E2E.

| Command | Result |
|---|---|
| `go test -count=1 ./...` | PASS. |
| `go vet ./...` | PASS. |
| `golangci-lint run` | PASS. |
| `git diff --check` | PASS. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify init` | PASS. Initialized isolated ledger. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify project register --path . --name QUANTUM_LOG` | PASS. Registered `quantum-log` at this worktree path. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify setup copilot-vscode --yes` | PASS. Historical output used `capture=experimental`; Task 1 supersedes that label with `capture=unavailable` pending source evidence. The content-capture setting remained `false`. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify collector install` | PASS. Installed managed collector. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify collector start` | PASS. Started collector with pid `42736`. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify adapter verify copilot-vscode --json` while collector was running | PASS. Completed without SQLite lock contention. Returned `ready=false` because no real Copilot event exists yet. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify usage project quantum-log --json` while collector was running | PASS. Completed without SQLite lock contention. Returned no rows and `total_tokens=0`. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify collector logs` | PASS. Current collector logs include `/v1/traces OTLP JSON/protobuf`. |
| `go run ./cmd/qlog --home C:\Users\cowbo\AppData\Local\Temp\opencode\quantum-log-v031-verify collector stop` | PASS. Stopped collector. |

Additional automated coverage now verifies:

- `adapter verify copilot-vscode` does not pass from generic ingested fake Copilot usage.
- `adapter verify copilot-vscode` requires raw `otlp-http` Copilot `model.call` evidence with `otel_reported` tokens.
- OTLP receiver does not use `github.copilot.git.repository` or `copilot_chat.repo.remote_url` as `working_directory`, avoiding credential leakage from remote URLs.
- Collector opens the ledger per request, so the collector process no longer holds the writer lock between requests.
