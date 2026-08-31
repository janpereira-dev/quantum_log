# Troubleshooting

## Windows recovery from RC7 or an interrupted uninstall

Candidates before `0.4.0-rc.8` could leave a qlog-owned Startup value and a
Copilot hook configuration behind. This cleanup removes only qlog-owned
configuration and retains the ledger. Run it in one PowerShell window, then
restart Windows:

```powershell
$qlog = (Get-Command qlog -All -ErrorAction Stop | Select-Object -First 1).Source

foreach ($adapter in 'copilot','codex','claude-code','copilot-vscode','opencode','pi','openclaw','hermes') {
  & $qlog adapter uninstall $adapter --json
}
& $qlog collector uninstall --json

# Idempotent removal of the exact legacy persistence identifiers.
schtasks.exe /End /TN 'QUANTUM_LOG Collector' 2>$null
schtasks.exe /Delete /TN 'QUANTUM_LOG Collector' /F 2>$null
Remove-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' `
  -Name 'QUANTUM_LOG Collector' -ErrorAction SilentlyContinue

# Stop only qlog collectors; do not terminate arbitrary cmd.exe processes.
Get-CimInstance Win32_Process -Filter "Name = 'qlog.exe'" |
  Where-Object { $_.CommandLine -match '(?i)collector\s+serve' } |
  ForEach-Object { Stop-Process -Id $_.ProcessId -Force }
```

`0.4.0-rc9` replaces this with a single `qlog uninstall --json` command. Add
`--purge-data` only if you also want to delete the local ledger.

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

`v0.3.2-rc.3` is a local candidate artifact, not a public end-to-end installer acceptance. Use local artifact directory only for RC validation:

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
