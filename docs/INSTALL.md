# Install QUANTUM_LOG

`v0.4.0-rc10` is a published prerelease. It points to commit
`35ae43bd0031b3aca2621c52ede74731ae136357` and has GitHub Release archives,
`checksums.txt`, a Sigstore bundle for that checksum file, and per-archive SBOMs.
This proves artifact availability, not stable-release readiness or external E2E.

There is no stable public release yet.

## Published RC installer

Use the versioned release installer. It downloads the matching GitHub Release
archive and verifies its SHA-256 entry in `checksums.txt` before replacing the
binary.

On macOS or Linux:

```sh
curl --fail --location --remote-name https://raw.githubusercontent.com/janpereira-dev/quantum_log/v0.4.0-rc10/installers/install.sh
sh install.sh --version v0.4.0-rc10
```

On Windows PowerShell:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/janpereira-dev/quantum_log/v0.4.0-rc10/installers/install.ps1 -OutFile .\install.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 --version v0.4.0-rc10
```

Use `--channel latest` only to evaluate the newest published candidate; it may
select a prerelease. The default `stable` channel deliberately excludes alpha,
beta, and RC releases.

```sh
sh install.sh --channel latest
```

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 --channel latest
```

The pushed `v*` tag triggers the release workflow. That workflow publishes the
archives consumed by `installers/install.sh` and `installers/install.ps1`.
RC10 is already published; do not recreate, move, or overwrite its tag.

## Legacy historical packaging evidence

The unpublished npm thin distributor remains pinned to `v0.3.2-rc.3`. Earlier
P0 evidence also exercised a locally generated `v0.3.2-rc.1` artifact at commit
`dba6ca4040b93b889ead41ec90d4b2ffd19226c1`. Those dated checks are preserved as
historical evidence only. The npm route is not supported for installing or
verifying RC10, and it must not be used as current release evidence.

Do not use `go install` to evaluate RC10: it bypasses the published archive and
checksum lifecycle.

## Optional local setup

Choose a dedicated local home if you want isolation, then use the installed
`qlog` executable:

```powershell
$env:QLOG_HOME = "$env:LOCALAPPDATA\QUANTUM_LOG"
qlog init
qlog project register --path . --name MY_PROJECT
qlog setup --yes
qlog doctor --json
```

`setup --yes` is consented. It initializes the ledger, attempts a qlog-owned
loopback collector on `127.0.0.1:4318`, and configures only detected stable
adapters. Inspect the plan without writes:

```powershell
qlog setup --dry-run --json
```

On Windows, if Task Scheduler denies managed collector creation, setup still
configures detected integrations but does not create a detached process or a
Startup-app fallback. Start `qlog collector serve --home <home>` in the active
session before using an OTLP-only source. See
[collector recovery](AUTOCAPTURE.md#collector-recovery) and
[ADR-005](architecture/ADR-005-collector-lifecycle.md).

## Cleanup

Remove every qlog-owned adapter configuration and collector before deleting the
binary:

```powershell
qlog uninstall --json
```

RC10 retains local ledger data in every mode. Automatic `--purge-data` is
temporarily unavailable and fails closed with `data_purged: false`.

To remove data manually, first run `qlog uninstall --json`, back up the ledger,
stop every `qlog collector serve` process, and inspect the resolved ledger
directory before deleting it. Never delete an unverified home path.
