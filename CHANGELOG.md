# Changelog

## 0.4.0-rc.7 - 2026-08-19

- Fixed managed PowerShell Copilot launchers to resolve one `copilot` application while preserving `PATH` precedence, preventing multiple candidate paths from being invoked as one command (#45).
- Added Dependabot coverage for GitHub Actions, npm, pip, and Go modules, and updated CI to Go 1.26.6 (#35).
- Updated GitHub Actions used by CI and release automation: checkout to v7, setup-go to v7, GoReleaser action to v7, and golangci-lint action to v9 (#36, #37, #39, #40).
- Updated Go dependencies: `google.golang.org/protobuf` to 1.36.12, `github.com/pelletier/go-toml/v2` to 2.4.3, and `go.opentelemetry.io/proto/otlp` to 1.11.0 (#41, #42, #44).
- The unpublished npm thin distributor remains intentionally pinned to `0.3.2-rc.3`; it is a legacy local packaging-validation path and is not evidence of an rc.7 npm publication.

## 0.4.0-rc.6 - 2026-08-13

- Reconciled OTLP aggregate spans durably across requests and excluded only exact parent/child usage duplicates from consumption totals.

## 0.4.0-rc.5 - 2026-08-13

- Materialized privacy-safe canonical interaction roots from Copilot, Claude Code, and Codex telemetry traces when a prompt hook is unavailable.
- Included unallocated model calls in usage rows as `unattributed` instead of hiding their reported consumption.

## 0.4.0-rc.4 - 2026-08-13

- Fixed Copilot OTLP capture to derive one canonical interaction per telemetry trace when hook or root spans are absent.
- Linked Copilot model calls to the trace interaction and derived hook identities from documented prompt payloads without event IDs.

## 0.4.0-rc.3 - 2026-08-13

- Fixed `qlog setup --yes` when a running managed collector owns the ledger writer lock.
- Preserved the active collector's configured home and listener during setup recovery, including Windows scheduler-backed collectors.

## 0.4.0-rc.2 - 2026-08-12

- Fixed Windows PowerShell profile backups to use the same OneDrive-compatible writer as the profile update.

## 0.4.0-rc.1 - 2026-08-11

- Added canonical prompt interactions with source/session/upstream deduplication and linked model calls.
- Added prompt-capture modes (`off`, `hash`, and redacted local `full`) and prompt-aware native hooks.
- Added five-agent setup, embedded OpenCode plugin, interaction log commands, and interaction-based reports.
- Added managed loopback collector setup and cross-platform build targets.

## 0.3.2-rc.3 - 2026-08-06

### Release candidate

- Feature freeze for five-agent capture and reporting, acceptance-package preparation, and collector readiness.
- Pending external end-to-end acceptance.

## 0.3.2-rc.2 - 2026-08-05

### Release candidate

- Advanced local release-candidate metadata for the next validation cycle.

## 0.3.2-rc.1 - 2026-08-04

### Release candidate

- Aligned local release-candidate version metadata across binary defaults and distribution artifacts.

## 0.2.0 - 2026-07-20

### M1 closed (integrity and attribution)

- Fixed project resolver precedence: explicit -> QLOG_PROJECT -> CWD -> Git root -> registered path -> adapter -> unattributed.
- Fixed SQLite store location selection to use normalized matching paths instead of first-by-slug.
- Added cross-platform cooperative lock protocol (ADR-004): shared quiescence + exclusive writer locks.
- Added read-only `doctor` and `verify` that take an exclusive quiescence lock, block on active WAL, and warn on isolated SHM without mutation.
- Added `qlog maintenance status` and `qlog maintenance checkpoint`.
- Sanitized raw-event evidence before hashing; expanded the sensitive key list (cookie, token, bearer, apikey, private_key, credentials).
- Added external ledger anchors (`qlog anchor export` / `qlog anchor check --file`) with mismatch and truncation detection.
- Honest milestone status contract and 0.2.0 quickstart in README.

### Documentation

- Added [docs/DEVELOPER_GUIDE.md] — step-by-step idiot-proof developer guide.
- Updated README to 0.2.0 functional status with honest `IMPLEMENTED` vs `VERIFIED` markers.
- Versioned QUANTUM_LOG_MASTER_PROMPT to 1.3 (0.2.0).

### Build

- Default `qlog --version` now reports `0.2.0`.
- All tests pass with `go test -count=1 ./...`; vet clean.

## 0.1.0 - 2026-07-17

- Added Milestone 0 foundation and Milestone 1 core ledger scaffold.
- Added Milestones 2 through 5 reporting, capture, and distribution source assets.

## Unreleased

- Added canonical prompt interactions with durable upstream deduplication and model-call linkage.
- Added versioned embedded OpenCode plugin source and interaction log commands.
- Preserved historical model and tool rows as legacy-unlinked during SQLite migration.
