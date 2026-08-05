# Verify In Under 10 Minutes

Run local setup and evidence checks in under ten minutes. Real capture PASS is **not** claimed: P0 recorded external blockers for Codex, Copilot CLI, and Copilot VS Code.

## Fast path

1. Install local RC and isolate data. Use [Install](INSTALL.md) first; then set binary path.

```powershell
$qlog = 'C:\qlog-install\node_modules\.bin\qlog.cmd'
$env:QLOG_HOME = "$env:LOCALAPPDATA\qlog-verify"
& $qlog init
& $qlog project register --path . --name QLOG_VERIFY
```

2. Setup and status.

```powershell
& $qlog setup --yes
& $qlog collector status --json
& $qlog adapter status --json
```

3. If Scheduler policy blocks managed collector, start documented fallback in terminal one.

```powershell
& $qlog collector serve --home $env:QLOG_HOME --log-file "$env:QLOG_HOME\collector.log"
```

In terminal two, confirm `reachable=true`:

```powershell
& $qlog collector status --json
```

4. Make exactly one normal real agent action in registered project, then query evidence.

```powershell
& $qlog adapter verify <adapter> --project qlog-verify --since 10m --json
& $qlog usage project qlog-verify --json
& $qlog export --format json --from 2026-08-04 --to 2026-08-05
```

Use current date range for `export`; dates above mirror P0 command shape. `adapter verify` may fail until all gates have real evidence.

5. Restart, then make second agent action and query again.

Managed collector only:

```powershell
& $qlog collector restart --json
& $qlog collector status --json
```

Foreground fallback: stop terminal-one collector with `Ctrl+C`, rerun `collector serve`, then recheck status. Make a second real agent action and rerun step 4 commands.

6. Check ledger health.

```powershell
& $qlog verify
& $qlog doctor --json
```

## Agent actions

| Agent | Real action | Current P0 result |
| --- | --- | --- |
| Codex | Run one authenticated `codex exec` action in registered project while collector is healthy. | **BLOCKED_EXTERNAL**: action completed but no OTLP request reached foreground collector. |
| Copilot CLI | Run one authenticated `copilot -p` action after `qlog setup copilot --yes`. | **BLOCKED_EXTERNAL**: hook installed but no hook event reached qlog. |
| Copilot VS Code | Open registered project in VS Code; send one Copilot Chat/Agent message after `qlog setup copilot-vscode --yes`. | **BLOCKED_EXTERNAL**: this host has no GitHub Copilot extension/login surface. |
| Claude Code / OpenCode | Run one normal agent action after setup. | Guided validation only; source-backed clean-device event remains pending. |

## Pass criteria

Do not call capture PASS merely because setup or collector health passed. A valid agent run needs:

- `adapter verify <adapter> ... --json` returns `ready=true`.
- Query shows source-originated evidence for target project.
- `lifecycle_only` records zero token counters; `otel_reported` requires source-reported tokens.
- After restart and second action, query shows durable additional evidence.
- `qlog verify` prints `ledger: verified`; `qlog doctor --json` has `status=ok`.

## Blocked paths

- **Codex:** reproduce only; do not inject synthetic OTLP to manufacture PASS.
- **Copilot CLI:** reproduce only; do not fabricate hook payloads.
- **Copilot VS Code:** install/authenticate extension and use healthy collector on supported host before attempting real action.
- **Windows Scheduler:** use foreground fallback when `/Create` returns access denied. This does not verify managed service restart.

If event never appears, retain `BLOCKED_EXTERNAL` or guided-validation status and see [Troubleshooting](TROUBLESHOOTING.md).
