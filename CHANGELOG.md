# Changelog

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
