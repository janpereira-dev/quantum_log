# Operations

Operate QUANTUM_LOG through its CLI lifecycle and cooperative locks. Keep the ledger local, verify it before relying on reports, and treat unavailable maintenance actions as boundaries rather than invitations to improvise.

[Operaciones en español](../es/operaciones.md)

## Routine checks

```bash
qlog doctor --json
qlog verify
qlog collector status --json
qlog adapter status --json
```

`doctor` checks SQLite and local ledger health without mutation. `verify` checks append-only chain integrity. `collector status` reports endpoints and reachability. `adapter status` reports installation state and capture quality, not verified usage claims.

Run diagnostics when normal writers are inactive. Diagnostics acquire exclusive quiescence access, so a held lock or active WAL can make them fail. Stop or finish the relevant qlog activity first; do not force SQLite open externally.

## Collector operation

Default collector endpoint is `http://127.0.0.1:4318`:

```bash
qlog collector serve
qlog collector status
qlog collector logs
```

Endpoints are:

| Endpoint | Input |
| --- | --- |
| `/v1/traces` | OTLP/HTTP JSON or protobuf traces. |
| `/v1/events` | qlog JSON events. |
| `/healthz` | `GET` or `HEAD` health check. |

For managed service lifecycle use `install`, `start`, `stop`, `restart`, `logs`, and `uninstall`. A listener outside loopback is rejected unless `--allow-non-loopback` is explicit. If you opt in, you own network exposure, firewall, authentication, and host-hardening consequences.

## Adapter operation

Start with discovery and a dry run:

```bash
qlog adapter detect
qlog setup --dry-run
qlog adapter install opencode --dry-run
```

Then apply and test a specific adapter:

```bash
qlog setup opencode --yes
qlog adapter test opencode
qlog adapter status opencode
```

Use `qlog adapter uninstall <adapter> --dry-run` before removal. Adapter setup should only touch qlog-owned configuration, but review planned changes and backups before applying them.

### Copilot verification

M4 is `IN_PROGRESS`. Copilot VS Code remains experimental until a real trace persists Copilot-originated tokens. Use:

```bash
qlog adapter verify copilot-vscode --project my-project --since 1h --json
```

This verifies settings, collector reachability, a valid duration, local database access, and a recent qualified Copilot OTLP model call. A configured extension without persisted token evidence is not a verified capture path.

## Backup and recovery boundaries

Before copying a ledger, quiesce qlog clients and run:

```bash
qlog maintenance checkpoint
qlog verify
qlog anchor export > anchors.json
```

Store the resulting backup and `anchors.json` separately. External anchors are valuable because they let a later check detect mismatch or truncation:

```bash
qlog anchor check --file anchors.json
```

`maintenance recover` and `maintenance rebuild-anchor` are intentionally blocked pending anchor work. Do not claim a supported recovery procedure exists, and do not rebuild or edit ledger state with an external SQLite tool. Escalate a damaged-ledger incident with the original files, command output, and any externally stored anchors intact.

## Lock and WAL failures

| Symptom | Meaning | Safe response |
| --- | --- | --- |
| `quiescence lock is held` | Another official client is active. | Finish or stop that client, then retry diagnostic. |
| Active WAL failure | Stable read-only diagnostic cannot safely proceed. | Let writer close, or use the supported checkpoint path after quiescence. |
| Pending migration | Current ledger schema does not match application. | Run `qlog init` with current qlog version after backup policy is satisfied. |
| Isolated SHM warning | SQLite sidecar needs operator awareness. | Preserve evidence; inspect using supported qlog lifecycle. |

## Evidence operations

Use reports and exports with provenance:

```bash
qlog usage today --group-by project,agent,provider,model,capture_quality
qlog report --json
qlog export --format csv --redact-paths > usage.csv
```

Before sharing, use `--redact-paths` where location paths are not needed. Do not aggregate away `capture_quality` when comparing integrations or producing an audit statement.

## Incident handoff

Capture these facts before escalating:

1. qlog version and operating system.
2. Exact command, flags, exit status, stdout, and stderr.
3. Whether collector or adapter process was running.
4. `qlog doctor --json` and `qlog verify` output, if lock state allows.
5. Whether an external anchor exists and result of `qlog anchor check --file ...`.

Do not attach raw prompts, responses, tool payloads, API keys, or copied database contents to an incident by default. See [Privacy and security](privacy-security.md).
