# Developer Guide

## M4 Capture Operations

M4 stable auto-capture scope contains only `claude-code`, `codex`, `copilot-vscode`, and `opencode`. Do not treat generic JSONL import or a configured adapter as verified auto-capture.

Use the consented bootstrap path:

```text
qlog setup --dry-run
qlog setup --yes
qlog collector status --json
```

Bootstrap creates or reuses qlog-owned, per-user collector lifecycle state and configures detected stable adapters. It starts on `127.0.0.1:4318`. Configuration updates are idempotent; qlog preserves a timestamped backup when it changes an existing managed adapter file. The collector rejects non-loopback addresses unless `collector serve --allow-non-loopback` is specified.

Lifecycle commands are `qlog collector install`, `start`, `status`, `logs`, `stop`, `restart`, and `uninstall`. `status --json` separates installed, running, and reachable state. Windows uses a limited user Task Scheduler task; macOS uses a per-user LaunchAgent; Linux uses a systemd user service. `uninstall` removes qlog-owned lifecycle state, not the ledger or user data.

Capture quality is evidence, not a marketing label. `lifecycle_only` may have raw-event evidence but zero model calls and zero tokens. `unavailable` is adapter status, not usage. Reported qualities require source-backed counters; `estimated` remains separate. `qlog report usage --json`, `qlog usage project <slug> --json`, `qlog task summary <task-id> --json`, and `qlog session summary <session-id> --json` expose quality-separated measurements and estimated-cost fields.

`qlog adapter verify <adapter> --project <slug> --json` is fail-closed. It writes stage results but exits non-zero unless required setup, availability, collector, source-evidence, quality, and durable raw-event gates pass. For reported-token capture, raw evidence must link to one normalized model call with source-reported tokens. Replays use a durable sanitized identity and are suppressed before ledger append and normalization; confirm suppression through recorded counts rather than assuming a retry succeeded.

See [M4 evidence](../docs-int/verification/m4-evidence.md) for source-evidence gates and clean-device acceptance requirements.

## Contributor Guides

This legacy entry point also links to public contributor guides:

- [English contributing guide](en/contributing.md)
- [Guía de contribución en español](es/contribucion.md)

This file remains a compatibility link for existing references.
