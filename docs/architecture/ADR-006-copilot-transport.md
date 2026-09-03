# ADR-006: Select durable local Copilot capture transports from real evidence

Status: accepted

Date: 2026-09-03

## Context

Copilot CLI and Copilot Chat for VS Code are separate products and must not share
an evidence claim. Quantum Log needs reported model and token evidence without
capturing prompts, responses, tool inputs, tool outputs, credentials, or user
paths. A transport decision also has to reduce, rather than extend, the
persistent-collector lifecycle described in ADR-005.

The primary product sources are GitHub's
[Copilot CLI command reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference)
and [OpenTelemetry agent-monitoring overview](https://docs.github.com/en/copilot/concepts/agents/opentelemetry).
GitHub links to the official
[VS Code Copilot monitoring reference](https://code.visualstudio.com/docs/agents/guides/monitoring-agents)
for editor-specific settings.

## Weighted criteria frozen before observation

The finalization plan fixed these criteria before the 2026-09-03 runtime
observation. Scores are out of 100.

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

Privacy and clean removal are veto gates: a transport that exposes content by
default or cannot be removed without touching unrelated user state is rejected
regardless of its weighted score.

## Evidence

The sanitized spike used GitHub Copilot CLI 1.0.78 on Windows 11 x64. A minimal
authenticated prompt completed successfully with only process-scoped
`COPILOT_OTEL_FILE_EXPORTER_PATH` and content capture explicitly false. The
documented file exporter produced model, session, and reported token attribute
names in a local JSONL file. The evidence contains only signal counts, attribute
names, a sanitized-schema hash, and privacy-scan results; the raw JSONL was
deleted.

Visual Studio Code 1.136.0 was installed, but no GitHub Copilot extension was
installed. No authenticated editor-agent run was therefore possible. The
official settings describe OTLP and file export, but documentation alone is not
real source evidence for that installed product state.

Full sanitized evidence is recorded in
[`docs-int/verification/copilot-transport-spike.md`](../../docs-int/verification/copilot-transport-spike.md).

## Options compared

| Product / option | Score | Decision | Reason |
| --- | ---: | --- | --- |
| CLI: documented file exporter | 95 | Accept for implementation | Official, local, durable JSONL; observed model, conversation, and token schema; process-scoped ownership requires only an output file and cursor/checkpoint. |
| CLI: OTLP HTTP | 75 | Reject as default | Official and rich, but push delivery needs a live collector and introduces availability and lifecycle debt. CLI 1.0.78 also disables cleartext HTTP export rather than failing the agent. |
| CLI: documented lifecycle hooks | 40 | Reject for usage | Useful lifecycle evidence, but no official guarantee that hooks expose complete model and token counters. |
| CLI: explicit wrapper | 55 | Reject while file export works | Can own process environment and cleanup, but output parsing or process interception adds a second behavioral boundary without improving evidence. |
| VS Code: documented file exporter settings | 70 provisional | Unsupported for stable capture | Strong official candidate, but no installed extension or authenticated real-device evidence exists for this build. |
| VS Code: documented OTLP settings | 65 provisional | Unsupported for stable capture | Same missing runtime evidence, plus a persistent receiver requirement. |

Rejected alternatives include private APIs, undocumented databases, log scraping, UI interception or UI
automation, and background packet interception are rejected. They are unstable,
cannot provide a narrow ownership boundary, or inspect data outside the explicit
telemetry contract.

## Decision

- **CLI: documented file exporter.** Implement a process-scoped launcher that
  sets an output path owned by Quantum Log and keeps content capture false. Import
  the append-only JSONL incrementally through a durable offset plus file-identity
  checkpoint, sanitize before ledger persistence, fsync the checkpoint after a
  committed batch, and remove only the qlog-owned file and checkpoint on
  uninstall. This does not require a persistent collector. The decision does not
  claim that the current adapter already implements this boundary.
- **VS Code: unsupported for stable capture.** Keep its maturity below verified
  until an installed, authenticated Copilot extension on a pinned VS Code and
  extension version produces a sanitized real-device envelope. If that evidence
  passes the same gates, evaluate the documented file exporter first. Do not
  inherit the CLI result.

Task 8 may implement only the CLI decision. A VS Code implementation remains
blocked by external evidence rather than by assumed compatibility.

## Privacy impact

The CLI exporter is opt-in and process-scoped. Import must allowlist only the
identity, correlation, timing, model, and numeric usage fields needed by the
ledger. The observed `gen_ai.tool.definitions` attribute name is a specific
minimization warning: Quantum Log must not persist that value. It must likewise
discard prompt, response, message, tool argument/result, resource-path, and
credential values even if a future producer emits them. Raw JSONL is transient
ingestion material, never committed verification evidence.

## Consequences

- CLI capture can be durable and offline without keeping an OTLP receiver alive.
- Incremental import must handle partial final lines, rotation/truncation, crash
  replay, deduplication, and bounded file growth before it is called complete.
- The observed schema hash and pinned producer version become drift gates, not a
  promise that every attribute will always exist.
- VS Code remains an explicit evidence gap; this ADR does not convert official
  capability documentation into a successful E2E claim.

## Rollback

Before implementation, rollback is deletion of this ADR, its sanitized spike
record, the structural test, and the two source-contract edits. After a CLI
implementation exists, rollback disables the qlog-owned launcher/importer,
removes only its output file and checkpoint, and restores the prior adapter
maturity. It must not edit global profiles, VS Code settings, credentials, or
unrelated telemetry configuration.
