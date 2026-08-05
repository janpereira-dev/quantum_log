# Troubleshooting

Use compiled `qlog` artifact for diagnosis. Do not manufacture telemetry or claim real capture from setup files, unit tests, or synthetic events.

## Collector is not reachable

```bash
qlog collector status --json
qlog doctor --json
```

P0 Windows observation: managed collector creation can fail with Task Scheduler `Acceso denegado`. Start foreground loopback collector instead:

```powershell
qlog collector serve --home $env:QLOG_HOME --log-file "$env:QLOG_HOME\collector.log"
```

Keep it running, then rerun `qlog collector status --json`. Foreground service has no managed lifecycle; stop with `Ctrl+C`.

## Setup changed nothing

Check detected adapter before consented setup:

```bash
qlog adapter detect --json
qlog setup --dry-run --json
qlog adapter status --json
```

`setup --yes` configures only detected stable adapters. Unavailable adapter is skipped. Re-run only one adapter when needed:

```bash
qlog setup <adapter> --yes --json
```

## Adapter verify fails

```bash
qlog adapter verify <adapter> --since 10m --json
qlog usage project <project-slug> --json
qlog export --format json --from 2026-08-04 --to 2026-08-05
```

Expected before first real source event: verification exits non-zero. Read its stage output; it distinguishes setup, availability, collector, quality/source evidence, and raw durable evidence.

Known P0 external blocks:

- Codex real action completed but no OTLP request arrived at healthy foreground collector.
- Copilot CLI real action completed but official qlog hook did not deliver an event.
- Copilot VS Code extension/login was absent on P0 host.

Do not add synthetic events to clear these stages.

## Install cannot fetch RC

`v0.3.2-rc.2` is a local candidate artifact, not a public end-to-end installer acceptance. Use local artifact directory only for RC validation:

```powershell
cmd /c "set QLOG_INSTALL_LOCAL_ARTIFACT_DIR=C:\path\to\dist&& npm install --prefix C:\qlog-install .\packaging\npm"
```

Official installer full download/bootstrap remains blocked until signed HTTPS RC artifact exists. Do not downgrade silently to older release.

## Remove qlog-owned configuration

```bash
qlog adapter uninstall <adapter> --json
qlog collector uninstall --json
```

Then manually remove intended `QLOG_HOME` only if you want to erase local ledger data.
