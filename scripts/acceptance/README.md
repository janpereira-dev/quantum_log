# Release acceptance harnesses

`release-lifecycle.sh` and `release-lifecycle.ps1` exercise one immutable release upgrade:

1. install `QLOG_FROM_VERSION` into an isolated directory;
2. initialize an isolated `QLOG_HOME` and ingest one deterministic, sanitized event through `qlog ingest file`;
3. install `QLOG_TO_VERSION`, run `doctor --json` and `verify`, and compare the ledger SHA-256;
4. uninstall without data purge, prove `qlog.db` and its hash remain unchanged, reinstall the target version, and verify again.

The harness never resolves `latest`. Set all three release inputs explicitly:

```text
QLOG_RELEASE_BASE=https://github.com/janpereira-dev/quantum_log/releases/download
QLOG_FROM_VERSION=v0.4.0-rc9
QLOG_TO_VERSION=v0.4.0-rc10
```

Optionally set `QLOG_EVIDENCE_DIR` to retain evidence at a chosen location. Otherwise a new platform temporary evidence directory is retained and printed. Evidence is bounded to versions, command exit codes, sanitized doctor/verify output, and ledger SHA-256 values; it excludes the fixture, raw ledger rows, payloads, environment values, and release URLs.

Use contract-only mode to validate isolation and explicit-version wiring without network access, release downloads, or persistent writes:

```sh
sh scripts/acceptance/release-lifecycle.sh --contract-only
```

```powershell
pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1 -ContractOnly
```

Contract-only mode creates and removes only a temporary directory. Full mode removes its isolated home and install directory in all outcomes. It never reads or writes the current user's normal Quantum Log home.
