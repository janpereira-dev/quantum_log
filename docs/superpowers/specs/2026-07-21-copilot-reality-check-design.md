# v0.3.1 Copilot Reality Check Design

`v0.3.1` closes the first real Copilot auto-capture loop: GitHub Copilot in VS Code must emit OTLP data that QUANTUM_LOG can receive, sanitize, persist, attribute to a project, and report without storing prompt or response content.

## Quick Path

1. Install `qlog` and register the project.
2. Run `qlog setup copilot-vscode --yes`.
3. Start the local collector with a managed background command.
4. Use Copilot Chat or Copilot Agent in VS Code.
5. Run `qlog adapter verify copilot-vscode` and `qlog usage project <slug>`.
6. Confirm model, tokens, project, and `capture_quality=otel_reported` appear without prompt, response, or tool-argument content.

## Decision

M4 remains `IN_PROGRESS` until this Copilot path has evidence from setup through persisted report. `v0.3.1` is not a broad multi-agent milestone. It focuses on Copilot first because Copilot is the product-critical blocker and has an official OTel surface.

## Scope

| Area | Decision |
|---|---|
| OTLP input | Support OTLP/HTTP JSON and protobuf on `/v1/traces`. |
| Collector lifecycle | Add a managed local collector flow so capture can continue without a foreground terminal. |
| VS Code setup | Use correct platform paths, parse JSONC, write atomically, and keep backups. |
| Uninstall | Remove only qlog-owned Copilot OTel settings and preserve user settings. |
| Verification | Add `qlog adapter verify copilot-vscode` that proves collector-to-SQLite-to-report behavior. |
| Evidence | Add `docs/verification/m4-evidence.md` and keep M4 status honest. |

## Out of Scope

- Full OpenCode, Codex, Claude Code, Pi, Hermes, or OpenClaw verification.
- Cloud-hosted telemetry ingestion.
- Prompt, response, tool argument, or tool result content capture.
- Provider billing formulas beyond preserving reported token fields.
- External package-manager publication.

## Architecture

```text
VS Code Copilot OTel
  -> OTLP/HTTP JSON or protobuf
  -> qlog collector /v1/traces
  -> OTLP receiver
  -> sanitized model.call payload
  -> SQLite raw_events + model_calls
  -> qlog usage project <slug>
  -> docs/verification/m4-evidence.md
```

The receiver remains local-first and loopback-only by default. Content capture stays disabled in the Copilot settings managed by qlog.

## Components

### OTLP Receiver

`internal/ingest/otlp` accepts:

- `application/json`
- `application/x-protobuf`
- `application/protobuf`

JSON remains supported for tests and synthetic fixtures. Protobuf uses official OTLP trace protobuf types instead of a hand-rolled parser. Unsupported content types still return `415`.

### Collector Lifecycle

`qlog collector serve` remains the foreground debug path.

`v0.3.1` adds a managed local lifecycle surface:

```bash
qlog collector install
qlog collector start
qlog collector stop
qlog collector restart
qlog collector logs
qlog collector uninstall
```

First implementation target is Windows user-session reliability, because the user is developing on Windows and Copilot VS Code validation can happen there first. macOS `launchd`, Linux `systemd --user`, and WSL are explicit follow-up targets unless they can be implemented safely inside this slice without broadening risk.

### VS Code Copilot Adapter

The adapter must:

- Resolve VS Code settings path per OS.
- Handle Windows native, macOS, Linux, and common VS Code variants where practical.
- Detect WSL and avoid silently writing Linux settings when the active VS Code host is Windows.
- Parse JSONC settings with comments and trailing commas.
- Preserve formatting as much as practical, but correctness and safety beat formatting preservation.
- Write via temp file + rename.
- Create backup before modifying existing settings.
- Mark qlog-managed settings so uninstall can distinguish qlog-owned keys from user-owned keys.

### Copilot Uninstall

`qlog adapter uninstall copilot-vscode` must remove only qlog-owned keys or revert qlog-owned values from `settings.json`.

It must not delete unrelated user settings, comments, or manually configured OTel settings that were not installed by qlog.

### Copilot Verification

`qlog adapter verify copilot-vscode` verifies local readiness and persisted capture evidence:

1. Copilot VS Code settings are installed.
2. Collector endpoint is reachable.
3. Receiver accepts OTLP JSON and protobuf probe payloads.
4. SQLite contains at least one Copilot `otel_reported` model call for the target project inside the verification window, or the command reports the exact missing stage.

Synthetic probes can verify qlog's collector and persistence path. They do not by themselves mark Copilot real capture verified. Real Copilot capture requires evidence from a Copilot-originated resource or span, such as `service.name=copilot-chat` or `service.name=github-copilot` with token attributes.

## Data Contract

The Copilot path records these fields when present:

| Field | Status |
|---|---|
| Provider | Verified from OTel GenAI attributes. |
| Requested model | Verified from `gen_ai.request.model`. |
| Resolved model | Verified from `gen_ai.response.model`. |
| Input tokens | Verified from `gen_ai.usage.input_tokens`. |
| Output tokens | Verified from `gen_ai.usage.output_tokens`. |
| Session | Verified from `session.id` or `gen_ai.conversation.id`. |
| Tool calls | Partial until tool spans/events are persisted as structured tool calls. |
| MCP calls | Partial until extension tool type is mapped and tested. |
| Cache tokens | Unverified unless Copilot emits stable cache attributes in real traces. |
| Reasoning tokens | Unverified unless Copilot emits stable reasoning attributes in real traces. |
| Project context | Experimental unless correlated with reliable workspace context. |

Token reporting must eventually separate reported totals from computed bucket sums, but that pricing/reporting model change is outside `v0.3.1`. For this release, the priority is not to overstate bucket semantics.

## Project Attribution

The receiver can still use OTel attributes when present, but real Copilot traces do not guarantee repository path attributes. To avoid false confidence:

- If qlog cannot resolve a project from reliable context, the event remains `unattributed`.
- The evidence file must show whether real Copilot capture resolved the project or fell back to unattributed.
- A future VS Code workspace heartbeat or extension can provide stronger attribution if Copilot OTel alone is insufficient.

## Documentation Changes

Documentation must say:

- M4 is `IN_PROGRESS` until E2E evidence passes.
- `copilot-vscode` is `CAPTURE_EXPERIMENTAL` before real evidence.
- `CAPTURE_VERIFIED` requires real Copilot-originated events in SQLite and usage reports.
- `qlog collector serve` is debug mode; managed collector lifecycle is the product path.

## Acceptance Criteria

- [ ] `/v1/traces` accepts OTLP JSON and protobuf payloads.
- [ ] Unsupported content types still return `415`.
- [ ] Copilot setup writes correct VS Code settings on Windows and does not enable content capture.
- [ ] Copilot setup handles JSONC settings with comments or trailing commas.
- [ ] Copilot setup writes atomically and creates backup for existing files.
- [ ] Copilot uninstall removes qlog-owned settings and preserves unrelated settings.
- [ ] Collector lifecycle commands exist and work on Windows user session.
- [ ] `adapter verify copilot-vscode` reports staged readiness and persisted capture evidence.
- [ ] `qlog usage project <slug>` shows real Copilot model/tokens with `capture_quality=otel_reported` after a real Copilot run.
- [ ] Prompt, response, tool arguments, tool results, secrets, and authorization fields are not persisted.
- [ ] `docs/verification/m4-evidence.md` records commands, outputs, platform, Copilot/VS Code evidence, and remaining gaps.
- [ ] README and developer guide downgrade M4 claims until evidence supports verification.

## Validation Plan

Run before PR:

```bash
go test -count=1 ./...
go vet ./...
golangci-lint run
git diff --check
```

Manual Windows E2E:

```bash
qlog init
qlog project register --path . --name QUANTUM_LOG
qlog setup copilot-vscode --yes
qlog collector install
qlog collector start
qlog adapter verify copilot-vscode
# Use Copilot Chat or Agent in VS Code inside this repo.
qlog adapter verify copilot-vscode
qlog usage project quantum-log
qlog collector logs
```

## Release Rule

Do not tag `v0.3.1` until:

- CI passes.
- `docs/verification/m4-evidence.md` exists.
- The evidence file clearly states whether Copilot is `CAPTURE_VERIFIED` or still `CAPTURE_EXPERIMENTAL`.
- The tag is created from `main`, not a feature branch.
