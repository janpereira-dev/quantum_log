# Install QUANTUM_LOG

Install `qlog`, then initialize a local ledger. The source CLI currently declares
`v0.4.0-rc.6`. The npm distribution package still declares `v0.3.2-rc.3`; it has
not yet been aligned with the current CLI candidate. Existing P0 evidence covers
prior candidates only, not a public release acceptance.

## One-command RC install

Requires Node.js 18 or later.

```powershell
cmd /c "set QLOG_INSTALL_LOCAL_ARTIFACT_DIR=C:\path\to\dist&& npm install --prefix C:\qlog-install .\packaging\npm"
```

Run from repository root. This legacy package path still expects generated
`checksums.txt` and the exact host `v0.3.2-rc.3` archive. The installer selects the
matching platform/architecture artifact, verifies SHA-256, and extracts only
`qlog` or `qlog.exe`. It uses no telemetry. Do not use this path as evidence that
`v0.4.0-rc.6` was installed until npm packaging, tests, and artifacts are aligned.

Confirm installed binary:

```powershell
C:\qlog-install\node_modules\.bin\qlog.cmd --version
```

P0 observed `qlog 0.3.2-rc.1`; one P0-11 extracted artifact also embedded commit `dba6ca4040b93b889ead41ec90d4b2ffd19226c1`. Do not substitute an older tag or archive.

**BLOCKED_EXTERNAL:** P0 did not run a public HTTPS bootstrap because this RC is intentionally non-public and no signed HTTPS RC artifact exists.

## Optional local setup

Choose a dedicated local home if you want isolation:

```powershell
$qlog = 'C:\qlog-install\node_modules\.bin\qlog.cmd'
$env:QLOG_HOME = "$env:LOCALAPPDATA\QUANTUM_LOG"
& $qlog init
& $qlog project register --path . --name MY_PROJECT
& $qlog setup --yes
& $qlog doctor --json
```

`setup --yes` is consented. It initializes ledger, attempts qlog-owned loopback collector on `127.0.0.1:4318`, then configures only detected stable adapters. Inspect plan without writes:

```powershell
& $qlog setup --dry-run --json
```

On P0 Windows host, Task Scheduler denied managed collector creation with `Acceso denegado`. Setup still configured detected integrations and recorded collector health. See [collector recovery](AUTOCAPTURE.md#collector-recovery) and [ADR-005](architecture/ADR-005-collector-lifecycle.md) before changing persistent collector state.

## Cleanup

Remove qlog-owned adapter configuration before deleting data:

```powershell
& $qlog adapter uninstall codex --json
& $qlog collector uninstall --json
```

Choose adapter ID from `qlog adapter list --json`. `adapter uninstall` removes only qlog-owned setup. Local ledger cleanup is manual: stop/uninstall collector first, then remove chosen `QLOG_HOME` directory.
