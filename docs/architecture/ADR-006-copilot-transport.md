# ADR-006: Require real, privacy-safe evidence before selecting a Copilot transport

Status: accepted decision; no stable Copilot transport approved

Date: 2026-09-03

## Context

Copilot CLI and Copilot Chat for VS Code are independent producer boundaries.
Neither official capability documentation nor a successful agent command proves
that Quantum Log received safe model and token evidence. The primary sources are
GitHub's [Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference)
and [OpenTelemetry agent-monitoring overview](https://docs.github.com/en/copilot/concepts/agents/opentelemetry),
plus the official [VS Code Copilot monitoring reference](https://code.visualstudio.com/docs/agents/guides/monitoring-agents)
linked by GitHub for editor-specific settings.

## Weighted criteria frozen before observation

| Criterion | Weight |
| --- | ---: |
| Documented and stable source | 20 |
| Real token and model availability | 15 |
| Project and session correlation | 15 |
| Privacy by default and minimization | 20 |
| Offline/local transport | 10 |
| Exact install and uninstall ownership | 10 |
| Cross-platform support | 5 |
| Version-drift detection | 5 |

Privacy and clean removal are veto gates. A transport that can expose transient
content without a proven bound, or cannot be removed without touching unrelated
state, is rejected regardless of score.

## Observed evidence

Both bounded probes used GitHub Copilot CLI 1.0.78 on Windows 11 x64, a minimal
authenticated prompt, and content capture explicitly false. No global profile,
user environment, product configuration, or credential was changed.

- **OTLP probe: 0 raw events.** A qlog collector built from this checkout was
  healthy on an isolated loopback port with a temporary `QLOG_HOME`. The bound
  source commit was `19a70213309e9c245d8e31363b17b487af2845a3`; the executed
  qlog binary SHA-256 was
  `4b37a3fb2c17c1a4d3a171e19a2f017fcb0470c06720de222d84204765b0a708`.
  Copilot exited `0`, but qlog recorded zero raw events and zero model calls. One
  producer diagnostic record reported HTTP export network failure. This does not
  support accepting OTLP.
- **File probe: 10 records.** The process-scoped file exporter emitted two spans
  and eight metrics with model, conversation, and token attribute names. It also
  emitted the `gen_ai.tool.definitions` attribute name while content capture was
  false. Its value was intentionally not retained, so the contradiction with the
  official content-capture description cannot be resolved from this evidence.
- **VS Code:** Visual Studio Code 1.136.0 had no Copilot extension installed; no
  authenticated editor turn or schema evidence exists.

The canonical sanitized representations and reproducible hashes are in
[`docs-int/verification/copilot-transport-spike.md`](../../docs-int/verification/copilot-transport-spike.md).

## Options compared

| Product / option | Weighted result | Decision |
| --- | ---: | --- |
| CLI: OTLP HTTP | 75/100 | Unsupported: the real loopback probe delivered no evidence. |
| CLI: documented file exporter | 95/100 before veto | File exporter: diagnostic only; privacy veto remains because transient content and ownership bounds are unproved. |
| CLI: documented lifecycle hooks | 40/100 | Rejected for usage: no complete model/token contract. |
| CLI: explicit wrapper | 55/100 | Rejected: wrapping output or process behavior adds an unsupported interception boundary. |
| VS Code: documented OTLP/file settings | 70/100 provisional | Unsupported without a pinned extension and authenticated real-device evidence. |

Private APIs, undocumented databases, log scraping, UI interception or UI
automation, and background packet interception are rejected.

## Decision

- **CLI: unsupported for stable capture.** Keep the implemented OTLP HTTP path
  accurately described as experimental and unverified. No replacement is
  authorized. The file exporter may be used only for bounded diagnostics until
  its privacy and lifecycle gates are proven.
- **VS Code: unsupported for stable capture.** Do not inherit CLI evidence.

A future accepted transport must have its own implementation task before
real-agent acceptance. The acceptance framework must not replace or reconfigure
the producer transport.

## Required file-spool proof before reconsideration

Any future file proposal must prove all of the following as one contract:

1. Exact owned path: `$QLOG_HOME/spool/copilot-cli/<launch-id>.jsonl`, with one
   file per Copilot process and an unpredictable UUID launch ID.
2. The directory is owner-only (`0700` on POSIX; protected DACL for the current
   Windows user) and each file is owner-only (`0600` on POSIX; the same protected
   DACL on Windows).
3. Every path component and opened handle is checked against symlinks and Windows
   reparse points; creation is exclusive and the final handle identity is
   revalidated before read, checkpoint, and delete.
4. Each complete raw line gets a raw-line SHA-256. The ledger append and that
   digest's idempotency record commit atomically; the byte checkpoint advances
   only afterward. A crash between commit and checkpoint safely replays into the
   digest deduplication gate.
5. Partial final lines remain unread. Rotation starts a new file identity at byte
   zero; same-identity truncation is quarantined rather than skipped. These are
   explicit rotation and truncation tests.
6. A hard numeric growth cap is enforced: 16 MiB per process file and 64 MiB for
   the complete Copilot spool. The launcher must fail closed if it cannot enforce
   those bounds while the producer runs; post-exit cleanup alone is insufficient.
   Fail closed applies only to capture: qlog degrades capture, marks usage
   unavailable, and stops retaining new spool data. It must not alter the Copilot
   command result; the upstream agent result remains authoritative.
7. Successful import securely removes only the verified owned file and
   checkpoint. Orphan recovery applies the same identity, privacy, cap, and
   idempotency checks. Uninstall removes only verified qlog-owned spool artifacts
   and leaves the ledger and unrelated files untouched.

No implementation currently proves this complete contract. In particular, the
producer controls append growth during a session, so the bounded transient-content
policy remains unresolved.

## Privacy impact

The file probe did not retain values, which protected the spike but also means it
cannot prove the value behind `gen_ai.tool.definitions` was harmless. A bounded
scanner may record only record type/count, schema hash, and forbidden-marker
absence. It must never copy the disputed value into Git, logs, or review output.
Until the source behavior and hard transient-storage bounds are proven, the
privacy veto remains.

## Consequences

- Task 7 closes with an honest unsupported decision rather than a speculative
  implementation authorization.
- Current OTLP setup remains unchanged but cannot be promoted from a zero-event
  run.
- Any later transport implementation is a separate, reviewable work unit before
  acceptance collection.
- Capture privacy, cap, or export failures mark Copilot usage unavailable and
  must never alter the authoritative upstream command result.
- Version or schema drift requires a new sanitized probe and hash.

## Rollback

Rollback removes this decision/evidence unit and its plan routing only. It does
not change runtime configuration. A future transport rollback must be defined by
its own accepted implementation task and exact ownership proof.
