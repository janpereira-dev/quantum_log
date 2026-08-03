# M4 Verified Autocapture Design

## Decision

M4 delivers verified, privacy-first automatic capture for exactly four stable adapters: Codex, Claude Code, GitHub Copilot CLI/VS Code, and OpenCode. It replaces setup-only or instruction-only success claims with a complete, observable chain:

```text
official agent installation
  -> installer-run, consented qlog per-user bootstrap
  -> persistent loopback collector
  -> adapter-emitted sanitized event
  -> replay-safe ingestion
  -> SQLite raw event and normalized model call
  -> adapter verification and usage report
```

Pi, OpenClaw, and Hermes are unsupported in M4. They must not be included in default bootstrap, stable-adapter status, or M4 release assurance. Existing generic/manual import behavior is outside this scope.

## Current Evidence and Gap

Current repository evidence proves ingestion paths and synthetic tests, not full M4 completion. `internal/adapters/adapters.go` registers seven agent adapters, but only OpenCode installs a plugin. Codex currently writes instructions while declaring that `rawResponse/completed` forwarding is required. Claude Code installs hooks and intentionally records `lifecycle_only`. Copilot configures local OTLP with content capture disabled, but `docs-int/verification/m4-evidence.md` records no real Copilot-originated model call. `qlog setup` requires `--yes` and does not install or start the collector. `AppendRawEvent` appends each submission, so retry and snapshot replay can duplicate raw events and normalized model calls.

Therefore M4 begins at official, already-installed supported agents and makes qlog bootstrap configure only documented integration points. It must not invent undocumented agent hooks, telemetry fields, token sources, or installation APIs.

## Approved Scope

| Area | M4 requirement |
|---|---|
| Adapter set | Codex, Claude Code, Copilot CLI/VS Code, and OpenCode only. Each has a maintained adapter contract, version evidence, installation detection, setup, health check, uninstall, and verification path. |
| Bootstrap | Official supported-agent installers automatically invoke per-user bootstrap as part of installation, after installer-level consent with a clear opt-out. Bootstrap discovers supported installations, shows planned changes, writes only qlog-managed content, backs up modified user files, installs/starts the collector, and reports each adapter's actual state. Re-running is idempotent. `qlog setup` remains available for non-official or manual installations, but is not required after an official install channel completes. |
| Collector | Local persistent collector binds loopback by default, exposes only required ingest and health endpoints, and is installed as a per-user background process using target-OS-supported lifecycle mechanisms. Non-loopback use remains explicit opt-in. |
| Evidence | Verification proves adapter -> collector -> SQLite for each stable adapter, then checks report output. Configuration, process health, or synthetic payloads alone cannot mark an adapter verified. |
| Deduplication | Retries and repeated snapshots do not create duplicate raw events, model calls, token totals, or costs. |
| Privacy | Events are minimized and sanitized before persistence, hashing, normalization, and reporting. |

## Adapter Contracts

Each adapter may emit lifecycle evidence even when measured usage is unavailable. Codex and OpenCode may record `agent_reported` tokens only from actual documented agent payload fields. Copilot CLI/VS Code may record `otel_reported` tokens only from real supported OTLP telemetry. Claude Code remains `lifecycle_only` unless a documented official source provides token usage; M4 must not infer or estimate it.

Adapter status has separate dimensions: installation state, collector reachability, recent real evidence, and measurement quality. A successful setup never implies measured token capture. A stable adapter is release-verified only after its own real-agent E2E record exists for supported versions and target operating systems.

## Ingestion and Deduplication

Ingress sanitizes and normalizes only allowlisted metadata: adapter/source, upstream event identity when available, session/trace identity, event type, occurrence time, project-resolution evidence, provider/model identity, reported usage counters, and capture quality. Deduplication runs after sanitization and before raw-event append and model-call normalization.

The implementation will persist a deterministic ingestion identity with a unique constraint. Prefer an upstream event, span, or snapshot identifier when a documented source supplies one. Otherwise derive a canonical fingerprint from sanitized stable fields; never include prompt content, response content, tool data, secrets, authorization, or a receive timestamp. Duplicate submissions return a successful idempotent result and do not create another ledger row or model call. Distinct events sharing a session or timestamp must remain distinct. The first accepted event remains chained in the append-only ledger; deduplication metadata records why later submissions were suppressed without altering that event.

## Privacy and Measurement Semantics

Raw event rules are deny-by-default for content and credentials. Prompts, responses, transcript references, tool arguments/results, environment values, cookies, authorization headers, API keys, tokens, secrets, passwords, and remote URLs with credentials are dropped before any SQLite write or hash. Project ownership remains resolved centrally; provider, model, and agent name cannot determine ownership. Unknown ownership remains `unattributed`.

Reports group measurement quality with agent, provider, model, and project. `otel_reported` and `agent_reported` mean counters were emitted by that source. `lifecycle_only` means no token measurement. `unavailable` means no usable event source. `estimated` remains visibly non-measured and is not a substitute for reported tokens. Costs derived from reported tokens remain estimates unless a real provider cost source exists. No report, status, or release text may collapse these categories or fabricate zero/estimated values as observed usage.

## Release Acceptance

1. Fresh supported-agent installations on each clean target OS device complete consented per-user bootstrap, leave user configuration recoverable, and start a reachable loopback collector.
2. For each of Codex, Claude Code, Copilot CLI/VS Code, and OpenCode, an actual agent run creates expected sanitized SQLite evidence; token-bearing adapters also create exactly one normalized model call and report row with source-backed quality.
3. Intentional retries and replayed snapshots create no additional raw-event, model-call, token, or cost totals; distinct events are retained.
4. Privacy inspection and automated regression tests prove forbidden fields never reach raw-event payloads, evidence, hashes, exports, or logs.
5. `adapter verify` fails closed when configuration, collector reachability, real evidence, freshness, measurement quality, or SQLite/report linkage is missing.
6. Synthetic unit, integration, and collector tests support regression coverage but do not satisfy real-agent acceptance.

`100%` release assurance requires actual runs on supported agent versions and clean target OS devices, not only synthetic tests. Until all four adapters meet that evidence bar, M4 remains incomplete and each adapter retains its truthful quality and verification state.

## Validation Plan

| Layer | Evidence |
|---|---|
| Unit | Adapter configuration ownership, sanitization, canonical identity, duplicate classification, and measurement labels. |
| Integration | Bootstrap idempotence, collector lifecycle, loopback refusal, ingest-to-SQLite linkage, unique deduplication, reports, and failed verification states. |
| Real-agent E2E | Official installations on clean target devices; one normal use, retry/replay case where available, SQLite inspection, `adapter verify`, and usage report per stable adapter. |

## Out of Scope

No source, test, installer, or user configuration change is made by this document. No claim is made that current M4 is verified. No support commitment is made for Pi, OpenClaw, Hermes, arbitrary JSONL producers, provider billing reconciliation, remote collectors, or capture of unavailable metrics.
