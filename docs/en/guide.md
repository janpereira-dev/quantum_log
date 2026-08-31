# Guide

This guide takes you from an empty local ledger to verified, explicitly qualified usage evidence. QUANTUM_LOG is local-first: it does not require a SaaS account, proxy, or prompt archive.

[Guía en español](../es/guia.md)

## Quick path

```bash
# Install a published binary following ../INSTALL.md first.
qlog init
qlog project register --path . --name MY_PROJECT
qlog project current --json
qlog verify
qlog doctor --json
```

`qlog init` creates local configuration and the ledger. `qlog project register` records a logical project and its location. `qlog verify` checks append-only event chains. `qlog doctor` checks local ledger health without modifying it.

## Install

Install a verified published binary following [Install](../INSTALL.md). Do not
use `go install` as an end-user release channel: it bypasses the release archive
and its checksum verification.

For development, build from this checkout:

```bash
go build -o qlog ./cmd/qlog
./qlog --help
```

Every command accepts `--home <path>` to override the local QUANTUM_LOG data directory. Use an explicit home for isolated testing or automation.

## Initialize and register

```bash
qlog init
qlog project register --path . --name MY_PROJECT
qlog project current --json
```

Expected output includes `initialized QUANTUM_LOG at ...`, then `registered my-project at ...`. The JSON from `project current` reports a project slug, resolution method, confidence, and location when one resolves.

Project attribution is evidence-based. Resolution checks an explicit `--project`, `QLOG_PROJECT`, current directory, Git root, registered path, adapter signal, then leaves data unattributed. It never guesses ownership from a provider, model, or agent name. See [Architecture](architecture.md).

## Ingest normalized events

Import newline-delimited JSON from a file or standard input:

```bash
qlog ingest file events.ndjson
cat events.ndjson | qlog ingest stdin
```

Successful imports report `imported N event(s)`. Input is normalized and sanitized before storage and hashing. Do not treat imported token fields as authoritative unless their `capture_quality` supports that claim.

## Capture setup

Inspect installed integrations first:

```bash
qlog adapter list
qlog adapter detect
qlog adapter status
qlog setup --dry-run
```

Apply a specific integration only after reviewing its dry-run plan:

```bash
qlog setup opencode --dry-run
qlog setup opencode --yes
qlog adapter test opencode
```

Use `qlog collector serve` when an adapter sends OTLP or qlog event payloads to the local collector. The collector listens on loopback by default. Binding a non-loopback address requires the explicit `--allow-non-loopback` opt-in.

For Claude Code hooks, configure the host to pipe hook JSON to:

```bash
qlog hook claude-code
```

Hook and wrapper capture can be lifecycle-only. Lifecycle evidence records that a process or session existed; it is not token usage.

### Copilot evidence boundary

M4 is `IN_PROGRESS`. Copilot VS Code setup and OTLP ingestion remain experimental until a real Copilot-originated trace persists token usage in SQLite. Installed settings alone do not verify capture. Use:

```bash
qlog adapter verify copilot-vscode --project my-project --since 1h --json
```

`qlog adapter verify copilot-vscode` is ready only when settings are installed, the collector is reachable, and recent Copilot-originated `otel_reported` model-call evidence with tokens exists. No single internal stage alone proves capture. Do not claim Copilot token capture from a dry run, installation result, or generic imported event.

## Read usage and cost

```bash
qlog usage today
qlog usage project my-project --json
qlog report --from 2026-07-01 --to 2026-08-01
```

Usage output includes capture quality. This is mandatory context: `otel_reported`, `agent_reported`, `lifecycle_only`, `unavailable`, and other labels are not equivalent. QUANTUM_LOG does not invent token counts.

Add versioned pricing rules before expecting calculated cost:

```bash
qlog pricing validate pricing.json
qlog pricing add pricing.json
qlog pricing recalculate --from 2026-07-01 --to 2026-08-01
```

Then inspect or repair explicit cost allocations only when you know the correct project:

```bash
qlog allocation show <model-call-id>
qlog allocation repair <model-call-id> --project my-project
```

## Verify and troubleshoot

Run these commands when no writer is active:

```bash
qlog verify
qlog doctor --json
qlog maintenance status
qlog collector status
```

`verify` checks source/session hash chains. `doctor` performs a read-only health check. Both take an exclusive diagnostic lock and can fail while another official client holds the cooperative quiescence lock or when an active WAL exists. This is a protection, not a reason to open the database with an external SQLite editor.

Use `qlog maintenance checkpoint` only after quiescing official clients. `maintenance recover` and `maintenance rebuild-anchor` are intentionally unavailable and return a not-implemented error.

## Next references

- [CLI reference](cli-reference.md): syntax, flags, examples, and failure states.
- [Architecture](architecture.md): layers, resolution, event lifecycle, and locking.
- [Operations](operations.md): diagnostics, backup, recovery boundaries, and anchors.
- [Privacy and security](privacy-security.md): sanitization, capture-quality policy, and threat model.
- [Contributing](contributing.md): local development and release handoff.
