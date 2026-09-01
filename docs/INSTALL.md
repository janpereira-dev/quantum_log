# Install QUANTUM_LOG

Install `qlog`, then initialize a local ledger. The source CLI currently declares
`v0.4.0-rc10`. The npm distribution package still declares `v0.3.2-rc.3`; it has
not yet been aligned with the current CLI candidate. Existing P0 evidence covers
prior candidates only, not a public release acceptance.

## Verified published release installer

Use the release installer, not `go install` or the legacy npm package. It
downloads the matching GitHub Release archive and verifies its SHA-256 entry in
`checksums.txt` before replacing the binary.

After `v0.4.0-rc10` is published, install that exact release on macOS or Linux:

```sh
curl --fail --location --remote-name https://raw.githubusercontent.com/janpereira-dev/quantum_log/v0.4.0-rc10/installers/install.sh
sh install.sh --version v0.4.0-rc10
```

On Windows PowerShell:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/janpereira-dev/quantum_log/v0.4.0-rc10/installers/install.ps1 -OutFile .\install.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 --version v0.4.0-rc10
```

Use `--channel latest` only to evaluate the newest published candidate; it can
select prereleases. The default `stable` channel deliberately excludes alpha,
beta, and RC releases.

```sh
sh install.sh --channel latest
```

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 --channel latest
```

The operator must first create and push the release tag (for example,
`git tag -a v0.4.0-rc10 -m "release: v0.4.0-rc10"` followed by `git push origin
v0.4.0-rc10`). The pushed `v*` tag triggers the release workflow, which then
publishes the artifacts and GitHub Release consumed by these installers. Until
that completes, no public RC artifact exists.

## Legacy local packaging validation (not an RC install)

Requires Node.js 18 or later.

```powershell
cmd /c "set QLOG_INSTALL_LOCAL_ARTIFACT_DIR=C:\path\to\dist&& npm install --prefix C:\qlog-install .\packaging\npm"
```

Run from repository root only when validating the legacy package. This path still expects generated
`checksums.txt` and the exact host `v0.3.2-rc.3` archive. The installer selects the
matching platform/architecture artifact, verifies SHA-256, and extracts only
`qlog` or `qlog.exe`. It uses no telemetry. It is not a supported way to install
`v0.4.0-rc10`; do not substitute `go install` for a published release artifact.

Confirm installed binary:

```powershell
C:\qlog-install\node_modules\.bin\qlog.cmd --version
```

P0 observed `qlog 0.3.2-rc.1`; one P0-11 extracted artifact also embedded commit `dba6ca4040b93b889ead41ec90d4b2ffd19226c1`. Do not substitute an older tag or archive.

**No public RC artifact exists yet.** Do not attempt `go install ...@v0.4.0-rc10`.
Create and push the tag first; the release workflow publishes its artifacts
afterward. Install an RC by its explicit verified release tag, while the default
`stable` channel continues to reject prerelease tags.

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

On Windows, if Task Scheduler denies managed collector creation with `Acceso denegado`, setup still configures detected integrations but does not create a detached process or a Startup-app fallback. Start `qlog collector serve --home <home>` in the active session before using an OTLP-only source. See [collector recovery](AUTOCAPTURE.md#collector-recovery) and [ADR-005](architecture/ADR-005-collector-lifecycle.md).

## Cleanup

Remove every qlog-owned adapter configuration and collector before deleting the
binary:

```powershell
& $qlog uninstall --json
```

`uninstall` removes only qlog-owned setup, including a legacy Windows Startup
entry created by older candidates. Local ledger data is retained by default. Use
`& $qlog uninstall --purge-data` only when you explicitly want to erase it.
