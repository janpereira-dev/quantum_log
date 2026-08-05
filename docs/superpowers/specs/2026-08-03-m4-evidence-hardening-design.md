# M4 Evidence Hardening Design

Apply three approved PR #20 corrections. Each narrows verification or normalization without adding adapters, telemetry sources, storage schema, or data collection.

## Quick Review Path

1. Confirm Copilot evidence requires sanitized provider `github`.
2. Confirm setup rejects transient hook executables through existing durable-path rules.
3. Confirm OpenCode lifecycle events cannot normalize into model calls.

## Scope

| Correction | Decision | Boundary |
| --- | --- | --- |
| Copilot verification | Require sanitized normalized payload provider `github` in addition to current source, quality, agent, freshness, project, and linked-token checks. Missing or any other provider fails closed. | `internal/cli/adapters.go`, `internal/storage/sqlite/` |
| Setup hook executable | Apply existing transient-binary rejection to `durableExecutablePath` after absolute-path and symlink resolution. Reject `.test`, `.test.exe`, and paths containing `/go-build`; retain valid installed binaries. | `internal/cli/setup.go` |
| OpenCode lifecycle | Plugin sends lifecycle events as `agent.event`, never `message.updated`; importer therefore retains raw lifecycle evidence but creates no `model_calls` record. | `internal/adapters/opencode.go`, `internal/ingest/qlogevent/` |

## Non-Goals

- No new model-call inference, provider aliases, fallback provider matching, or relaxed verification.
- No changes to valid installed-binary resolution, collector lifecycle, adapter setup ownership, migrations, or public CLI flags.
- No OpenCode token capture, model/provider capture expansion, or change to its `lifecycle_only` contract.
- No real-device acceptance claim from synthetic regression tests.

## Design

### Copilot Provider Gate

Extend `sqlite.AdapterEvidenceQuery` with an exact required provider field. Copilot contract supplies `github`; other adapters leave it empty. `HasRecentAdapterEvidence` filters `$.provider` case-insensitively only when required. It evaluates sanitized raw-event payload, not unsanitized input and not model-call provider alone.

Result: correctly shaped Copilot OTLP evidence with provider missing, `openai`, or another value cannot verify. Existing sanctioned Copilot OTel evidence with normalized `github` remains valid.

### Durable Setup Executable

`durableExecutablePath` remains one resolver: select explicit path or `os.Executable`, require absolute path, resolve symlinks, clean path, then call existing `validateCollectorExecutable` semantics. This prevents generated Claude hooks from embedding test or Go temporary executables while preserving a resolved installed `qlog`/`qlog.exe` path.

### OpenCode Lifecycle Classification

OpenCode plugin retains selected lifecycle callbacks and sanitized `lifecycle_only` payload. Its emitted `event_type` becomes stable `agent.event` rather than upstream event names. JSONL normalization only creates model calls for `model.call`, so lifecycle evidence remains append-only raw evidence with zero model calls and no usage row.

## Ownership And Privacy

| Area | Invariant |
| --- | --- |
| Verification | Provider is evidence identity, not project ownership. Project resolution remains centralized and never derives ownership from provider, model, or agent. |
| Executable path | Setup owns only qlog-generated hook references. It must reference a durable local executable or fail before adapter files change. |
| OpenCode events | Preserve lifecycle metadata only. Do not persist prompts, responses, tool arguments/results, secrets, credentials, or authorization fields. |
| Measurement | `lifecycle_only` must not create zero-token model calls or appear as observed model usage. |

## Exact Test Evidence

| Test | Required assertion |
| --- | --- |
| `internal/cli/capture_commands_test.go:TestAdapterVerifyCopilotAcceptsSanctionedOTLPEvidence` | Existing accepted Copilot OTLP payload with provider `github` still verifies. |
| New sibling Copilot verification tests | Otherwise identical sanctioned OTLP evidence fails verification when normalized provider is absent and when it is non-`github`. |
| `internal/storage/sqlite/store_test.go:TestAdapterEvidenceUsesModelCallAllocationForReportedTokens` | Extend query coverage to prove provider predicate reads sanitized raw payload and rejects mismatch despite linked reported-token model call. |
| `internal/cli/setup_test.go:TestSetupInstallOptionsDeriveDurableExecutableForManualSetup` | Existing resolved executable remains absolute and usable. |
| New `setupInstallOptions` table test | Explicit resolved paths ending `.test`/`.test.exe` or containing `/go-build` fail; a durable installed binary succeeds. |
| `internal/ingest/qlogevent/handler_test.go:TestHandlerKeepsOpenCodePluginEventsLifecycleOnly` | Update to assert a lifecycle event creates one raw event and zero `model_calls`; no usage row is emitted. |
| `internal/adapters/adapters_test.go:TestOpenCodeInstallWritesGlobalPluginPostingLocalEvents` | Assert generated plugin emits `agent.event`, not `message.updated` as event type. |
| Regression | `gofmt -d` modified Go files; `go test -count=1 ./...`; `go vet ./...`. |

Synthetic evidence proves only regression behavior. It does not satisfy M4 clean-device, real-agent acceptance.

## Rollback

| Risk | Control | Rollback |
| --- | --- | --- |
| Legitimate Copilot producer uses unrecognized provider | Fail closed prevents false verification. | Revert provider predicate only after documented producer evidence establishes approved normalized provider value; add fixture first. |
| Setup rejects developer test binary | Intentional: generated hook would break after test exits. | Run installed durable `qlog`; do not bypass validation. |
| OpenCode lifecycle event no longer yields model-call row | Intentional correction for unavailable usage. | Revert event-type mapping only if a documented OpenCode payload supplies actual reported usage, then add separate source-backed normalization. |

## Self-Review

- [x] Exactly three approved corrections; no unrelated PR #20 work.
- [x] Failure semantics explicit for missing and non-`github` provider.
- [x] Existing durable validation reused after path resolution; valid installed executable preserved.
- [x] Lifecycle downgrade explains importer behavior and zero-model-call result.
- [x] Scope, privacy, ownership, test evidence, and rollback contain no placeholders or conflicting claims.

## Next Recommended

Implement as one focused correction set, then run listed regression suite before updating PR #20 evidence.
