# M4 Review Corrections Design

## Decision

Apply seven validated PR #20 corrections as a single, minimal follow-up. The change preserves M4's privacy, ownership, and local-first contracts while making Codex verification evidence strict through a persisted, queryable response-completed discriminator, configuration mutation TOML-safe, collector status internally consistent, and platform lifecycle behavior durable.

## Quick Review Path

1. Review the seven corrections table for exact intended behavior.
2. Review ownership and lifecycle constraints before implementation.
3. Use the test evidence matrix to verify each correction without expanding M4 scope.

## Scope

| Correction | Intended change | Primary boundary |
| --- | --- | --- |
| Strict Codex log evidence | Require persisted `codex_response_completed` evidence emitted only by accepted Codex OTLP log normalization before `adapter verify codex` can pass. | `internal/cli/adapters.go`, OTLP normalization, SQLite evidence query tests |
| TOML exporter mutation | Safely replace existing inline, dotted-key, or nested-table Codex exporter forms without duplicate declarations. | `internal/adapters/codex.go` |
| Custom-home collector status | Resolve both reported home and database from persisted managed home when no explicit `--home` overrides it. | `internal/cli/collector.go` |
| systemd uninstall order | Remove unit before `systemctl --user daemon-reload`. | `internal/cli/collector_linux.go` |
| Codex README row | Report implemented Codex OTLP capability and remaining evidence gate consistently with executable behavior. | `README.md` |
| Windows restart policy | Add bounded Task Scheduler restart-on-failure settings. | `internal/cli/collector_windows.go` |
| Darwin replacement | Boot out loaded LaunchAgent before bootstrapping rewritten plist. | `internal/cli/collector_darwin.go` |

## Non-Goals

- No new adapters, collector endpoints, telemetry schemas, migrations, or reporting dimensions.
- No change to privacy filtering, loopback policy, project resolution, or token-accounting semantics.
- No adoption of preexisting matching Codex configuration as qlog-owned state.
- No changes to installer behavior, global agent configuration, PR threads, CI configuration, or release/milestone status.
- No attempt to obtain or claim clean-device real-agent acceptance evidence.

## Correction Details

### 1. Strict Codex Log Evidence

`adapter verify codex` must distinguish accepted Codex `response.completed` evidence from arbitrary `otlp-http` records and from records that merely claim `agent_name=codex`. Accepted Codex OTLP log normalization must persist a queryable `codex_response_completed` discriminator only after validating the Codex log shape. The verification query must require this discriminator in addition to normalized Codex agent identity (`codex`), existing source, quality, project, freshness, and linked model-call requirements.

The accepted evidence path remains narrow:

```text
Codex OTLP /v1/logs record
  -> service.name=codex, codex.sse_event, response.completed
  -> accepted Codex log normalization
  -> sanitized normalized event with agent_name=codex and codex_response_completed=true
  -> linked model call with otel_reported tokens
  -> Codex-only verification query requiring codex_response_completed=true
```

Malformed, unsupported, Copilot, generic OTLP, stale, wrong-project, tokenless, or agent-name-only evidence must not satisfy Codex verification. The discriminator is emitted only by accepted `service.name=codex`, `codex.sse_event`, `response.completed` log normalization; receiver acceptance rules and token values otherwise remain unchanged.

### 2. TOML-Safe Codex Exporter Mutation

Codex configuration mutation must treat these as equivalent representations of exporter configuration within `[otel]`:

- Inline `exporter = { ... }`.
- Dotted keys such as `exporter.otlp-http.endpoint = ...`.
- Nested tables such as `[otel.exporter.otlp-http]`.

Before writing qlog's desired exporter, implementation must parse TOML into a structured representation, replace the exporter as one logical value, and write valid TOML without retaining conflicting child keys or tables. It must preserve every setting outside `[otel]` byte-for-byte and may canonicalize only `[otel]`; tests must prove unrelated settings survive that rewrite.

Qlog state records originals only when qlog changes a setting. A preexisting configuration that already equals qlog's desired values remains unclaimed: no state file is created, `Installed` remains false, and uninstall leaves it untouched. When qlog owns a changed exporter, uninstall restores the exact prior exporter representation/value and `log_user_prompt` state captured at first mutation.

### 3. Custom-Home Collector Status

`qlog collector status --json` first resolves CLI configuration, then lets a platform manager restore persisted managed settings when `--home` or `--listen` was omitted. After managed-home resolution, it must resolve configuration again from that final home before assigning `database`.

Result invariant: `home`, `database`, state path, log path, and managed service definition describe one collector installation. An explicit `--home` continues to override persisted state.

### 4. Linux systemd Uninstall Order

Linux uninstall ordering must be:

```text
stop service -> disable service -> remove unit file -> daemon-reload -> remove qlog collector state and logs
```

`daemon-reload` is required after deletion so the user manager discards cached unit configuration. Failure behavior remains fail-fast before state/log removal, avoiding a status that claims uninstall completed while the system manager still has an actionable definition.

### 5. README Codex Support Row

The M4 support matrix must describe Codex as `otel_reported`, matching `codexAdapter.Status` and `evidenceContract`. Its release status must still state that clean-device `response.completed` log evidence and normal verification are required. Configuration alone is not verification.

### 6. Windows Scheduled-Task Restart Policy

Generated task XML must retain current-user, least-privilege, logon-trigger, and loopback-only behavior while adding Task Scheduler restart-on-failure settings. Restart count and interval must be finite and explicit, preventing uncontrolled retry loops while recovering from transient collector startup or runtime failures.

The policy applies to task-managed execution only. Manual `collector start`, task registration errors, and persistent configuration errors keep existing explicit error behavior.

### 7. Darwin LaunchAgent Replacement

When `collector start` installs a rewritten plist, it must replace an already loaded job rather than only `kickstart` the stale loaded definition:

```text
write plist and managed state -> bootout existing job if loaded -> bootstrap replacement plist -> kickstart replacement
```

If no job is loaded, bootstrap proceeds directly. If `bootout` reports that the job is absent, continue with bootstrap; return every other lifecycle error. The new job must receive requested executable, home, listener, and log-path arguments.

## Data Flow And Ownership Constraints

| Area | Required invariant |
| --- | --- |
| Codex evidence | Only normalized, accepted Codex `response.completed` OTLP log events that persist `codex_response_completed=true` can prove Codex verification. `agent_name=codex` alone is insufficient. |
| Settings ownership | Qlog writes state only for values it changes. Matching user-owned values are never adopted. |
| Uninstall | Restore qlog-owned values from recorded originals; never delete or rewrite unclaimed matching configuration. |
| Collector status | Resolve final managed settings before deriving dependent paths, especially database. |
| Lifecycle state | Platform manager owns service/task/plist lifecycle; CLI owns JSON/status presentation. |
| Privacy | No prompt, response, tool, secret, authorization, or raw telemetry content enters state, logs, identities, or tests. |

## Platform Lifecycle Semantics

| Platform | Install/start semantics | Uninstall/replacement semantics |
| --- | --- | --- |
| Linux | Write per-user systemd unit, reload, enable, start. | Stop, disable, remove unit, then reload manager before qlog state cleanup. |
| Windows | Register per-user InteractiveToken task with bounded restart on failure. | Existing stop/delete behavior remains; restart policy activates only after task failure. |
| Darwin | Create user LaunchAgent plist and bootstrap job. | A changed plist replaces any loaded job through bootout then bootstrap before kickstart. |

All platforms retain durable executable validation, loopback default, qlog-owned logs, and explicit command failures. Health reachability remains an independent status signal, not proof of adapter evidence.

## Test Evidence

| Layer | Required evidence |
| --- | --- |
| Codex adapter unit tests | Dotted exporter keys and nested exporter tables produce parseable, non-duplicated TOML; install is idempotent; qlog-owned values restore; matching preexisting values remain unclaimed and survive uninstall. |
| OTLP normalization tests | Accepted Codex `service.name=codex`, `codex.sse_event`, `response.completed` logs persist `codex_response_completed=true`; malformed, unsupported, Copilot, generic OTLP, and agent-name-only paths cannot emit it. |
| Verification/storage tests | Codex verification accepts fresh evidence with `agent_name=codex`, `codex_response_completed=true`, and a linked `otel_reported` model call. It rejects otherwise matching records missing the discriminator, including generic OTLP records that set `agent_name=codex`. |
| Collector command tests | Persisted custom home causes JSON `home` and `database` to resolve from same home; explicit home still wins. |
| Linux tests | Assert uninstall command/file ordering: disable, remove unit, daemon-reload. |
| Windows tests | Assert task XML includes bounded restart interval and count while retaining current-user and least-privilege settings. |
| Darwin tests | Stub lifecycle commands to assert loaded replacement uses bootout, bootstrap, then kickstart; unloaded path omits bootout. |
| Documentation check | README Codex row matches `otel_reported` contract and names clean-device evidence gate. |
| Regression suite | `go test -count=1 ./...`, `go vet ./...`, and `gofmt -d` on modified Go files. Run Linux, Windows, and Darwin platform unit tests in their respective CI matrix jobs. |

Synthetic tests establish regression behavior only. They do not satisfy M4 clean-device real-agent acceptance.

## Risks And Rollback

| Risk | Control | Rollback |
| --- | --- | --- |
| TOML parser/writer rewrites unrelated formatting | Limit mutation to `[otel]`; cover inline, dotted, and nested forms with fixtures. | Revert adapter mutation change; manually restore backed-up qlog-owned values through state. |
| Stricter Codex discriminator blocks ambiguous prior evidence | This is intentional fail-closed verification. | Revert discriminator filter only if accepted Codex `response.completed` logs cannot persist a stable discriminator; add normalization evidence first. |
| systemd reload fails after file removal | Return error and retain remaining qlog state for diagnosis. | Re-run uninstall or `systemctl --user daemon-reload`; no unit recreation. |
| Windows retry settings cause unexpected recovery cadence | Use documented finite values and XML assertions. | Replace task with prior definition via normal collector install after reverting change. |
| Darwin replacement interrupts active collector briefly | Replacement is required to apply changed arguments. | Re-run start with prior managed settings after reverting plist change. |

## Acceptance Checklist

- [x] Scope contains exactly seven validated corrections.
- [x] Codex verification requires `codex_response_completed=true`; `agent_name=codex` alone cannot pass.
- [x] Codex matching user-owned configuration remains unclaimed.
- [x] Platform lifecycle ordering and failure semantics are explicit.
- [x] Tests distinguish regression coverage from real-agent acceptance.
- [x] No placeholders, ambiguity, or unrelated work remains.

## Next Step

Implement this design in a separate code-change task. Keep one focused correction set, add only listed regression tests, and re-run the validation matrix before updating PR #20.
