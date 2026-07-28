# CLI Reference

This reference documents current Cobra command definitions. Run `qlog <command> --help` for executable help in your installed version. All commands accept root flag `--home <path>` to override the local data directory.

[Referencia CLI en español](../es/referencia-cli.md)

## Safety conventions

- Read-only diagnostics: `doctor`, `verify`, and `anchor check` require exclusive diagnostic access. They do not modify the ledger, and may fail when an official client holds the cooperative lock or an active WAL exists.
- Mutating commands use the normal application lifecycle. Do not bypass its lock protocol with external SQLite tools.
- Capture quality is evidence, not decoration. Reports must retain labels such as `otel_reported`, `agent_reported`, or `lifecycle_only`; token totals are not invented.
- Setup, adapter install, and collector lifecycle commands can change local configuration or services. Use their dry-run/status paths first when provided.

## `init`

**Syntax:** `qlog init`
**Purpose:** Initialize local configuration and ledger.
**Safety:** Creates local state; use `--home` for an isolated test ledger.
**Example:** `qlog init`
**Result:** `initialized QUANTUM_LOG at <home>`. Failures usually identify an unusable home or initialization problem.

## `doctor`

**Syntax:** `qlog doctor [--json]`
**Purpose:** Check ledger health without modifying it.
**Safety:** Read-only, exclusive diagnostic operation.
**Example:** `qlog doctor --json`
**Result:** text `doctor: ok` or JSON with `status`, `database`, and possible warning. It fails before initialization, for pending migrations, an active WAL, or a held quiescence lock.

## `verify`

**Syntax:** `qlog verify [--session <id>]`
**Purpose:** Verify append-only ledger hash chains, optionally for one session.
**Safety:** Read-only, exclusive diagnostic operation.
**Example:** `qlog verify --session session-123`
**Result:** `ledger: verified`; failure reports an integrity or diagnostic-access problem.

## `maintenance`

**Syntax:** `qlog maintenance <status|checkpoint|recover|rebuild-anchor>`
**Purpose:** Manage controlled local ledger maintenance.
**Safety:** Quiesce official clients before `checkpoint`; recovery functions are deliberately not available.
**Examples:**

```bash
qlog maintenance status
qlog maintenance checkpoint
```

**Result:** `checkpoint` reports `maintenance checkpoint: WAL cleared`. `recover` and `rebuild-anchor` fail with a not-implemented error.

## `project`

**Syntax:**

```text
qlog project register [--path <path>] --name <name> [--slug <slug>]
qlog project current|detect [--project <slug>] [--json]
qlog project list [--json]
qlog project show <slug> [--json]
qlog project tag <key=value> --project <slug>
qlog project tag list --project <slug> [--json]
```

**Purpose:** Manage logical projects, physical locations, and normalized tags.
**Safety:** Registration and tagging write ledger metadata. Do not use provider/model names as attribution substitutes.
**Example:**

```bash
qlog project register --path . --name MY_PROJECT
qlog project tag environment=work --project my-project
qlog project current --json
```

**Result:** registration reports `registered <slug> at <path>`; current output includes method and confidence. `tag` rejects input not shaped as `key=value`; commands requiring an unknown project fail clearly.

## `ingest`

**Syntax:** `qlog ingest file <path>` or `qlog ingest stdin`
**Purpose:** Import normalized NDJSON raw events.
**Safety:** Input is sanitized before import and hash chaining. Import only data you are permitted to process.
**Example:** `qlog ingest file events.ndjson`
**Result:** `imported N event(s)`. An unreadable file, invalid NDJSON, or reserved-source spoofing fails the import.

## `usage`

**Syntax:**

```text
qlog usage today|week|month [--group-by <dimensions>] [--json]
qlog usage project <slug> [--json]
```

**Purpose:** Show observed token usage.
**Safety:** Read totals alongside `capture_quality`; lifecycle-only events are not token evidence.
**Example:** `qlog usage today --group-by project,agent,provider,model,capture_quality`
**Result:** rows use `project | agent | provider/model | capture_quality | N tokens`, followed by `TOTAL | N tokens`; `--json` returns the report structure.

## `report`

**Syntax:**

```text
qlog report [--from <RFC3339|YYYY-MM-DD>] [--to <RFC3339|YYYY-MM-DD>] [--group-by <dimensions>] [--json]
qlog report summary [same flags]
```

**Purpose:** Summarize observed usage and allocated cost. `summary` is the explicit subcommand; the top-level `report` command also has `summary` as an alias.
**Safety:** Cost reflects persisted pricing and allocations, not a billing invoice.
**Example:** `qlog report --from 2026-07-01 --to 2026-08-01 --json`
**Result:** rows include tokens and USD micros, then a total. Invalid date values fail with accepted format guidance.

## `allocation`

**Syntax:**

```text
qlog allocation split <model-call-id> <project=basis-points>...
qlog allocation show <model-call-id> [--json]
qlog allocation repair <model-call-id> --project <slug>
```

**Purpose:** Manage model-call cost allocations.
**Safety:** `split` replaces allocations; repair only when an explicit, correct owner is known.
**Example:** `qlog allocation split call-1 alpha=5000 beta=5000`
**Result:** writes report `allocation: updated` or `allocation: repaired`. Invalid allocation syntax or unknown projects fail.

## `pricing`

**Syntax:**

```text
qlog pricing validate <file>
qlog pricing add <file>
qlog pricing list [--json]
qlog pricing show <provider/model> [--json]
qlog pricing recalculate [--from <RFC3339|YYYY-MM-DD>] [--to <RFC3339|YYYY-MM-DD>]
```

**Purpose:** Manage versioned pricing registries and calculated model-call costs.
**Safety:** Validate rules before persisting; recalculation updates computed cost data using persisted rules.
**Example:** `qlog pricing validate pricing.json`
**Result:** validation prints `pricing: valid`; add prints a rule ID; recalculate reports `recalculated N model call(s)`. Bad JSON/rules, dates, or missing identities fail.

## `task`

**Syntax:**

```text
qlog task start --project <slug> --title <title> [--type <type>]
qlog task finish <task-id> [--result <result>]
qlog task list [--project <slug>] [--json]
qlog task summary <task-id> [--json]
```

**Purpose:** Associate recorded usage with project tasks.
**Safety:** Task records organize evidence; they do not manufacture model calls or tokens.
**Example:**

```bash
qlog task start --project my-project --title "Implement import" --type build
qlog task finish <task-id> --result success
```

**Result:** start prints an ID; finish prints model-call, token, and allocated-cost summary. Required project/title flags and unknown IDs fail.

## `export`

**Syntax:** `qlog export [--format json|csv] [--from <RFC3339|YYYY-MM-DD>] [--to <RFC3339|YYYY-MM-DD>] [--redact-paths]`
**Purpose:** Export normalized model calls as JSON or CSV.
**Safety:** Prefer `--redact-paths` before sharing exports. Exported records retain capture quality and allocation context.
**Example:** `qlog export --format csv --redact-paths > qlog-calls.csv`
**Result:** JSON array or CSV headed by model-call and allocation fields. Unsupported formats and invalid dates fail.

## `anchor`

**Syntax:** `qlog anchor export` or `qlog anchor check --file <path>`
**Purpose:** Export and verify external ledger anchors for tamper and truncation detection.
**Safety:** Store exported anchor JSON outside the ledger and protect it from modification.
**Example:**

```bash
qlog anchor export > anchors.json
qlog anchor check --file anchors.json
```

**Result:** check prints `anchors: ok`, or prints mismatches/truncations and fails. Omitting `--file` fails before verification.

## `setup`

**Syntax:** `qlog setup [adapter] [--all] [--yes] [--dry-run] [--json]`
**Purpose:** Plan or apply auto-capture integration setup.
**Safety:** Start with `--dry-run`; only `--yes` applies changes. Without adapter or `--all`, setup selects available or installed setup-capable adapters.
**Example:** `qlog setup opencode --dry-run --json`
**Result:** each plan reports adapter ID, state, capture quality, and changes. Unknown adapters fail.

## `collector`

**Syntax:**

```text
qlog collector status [--listen <address>] [--json]
qlog collector serve [--listen <address>] [--allow-non-loopback]
qlog collector install|start|stop|restart|logs|uninstall
```

**Purpose:** Receive and manage local telemetry. Collector endpoints are `/v1/traces` for OTLP JSON/protobuf, `/v1/events` for qlog JSON, and `/healthz`.
**Safety:** Default listener is `127.0.0.1:4318`; non-loopback binding requires explicit opt-in.
**Example:** `qlog collector serve`
**Result:** serve announces listener and endpoints. `status` reports reachability and health. A public address without `--allow-non-loopback` fails.

## `adapter`

**Syntax:**

```text
qlog adapter list [--json]
qlog adapter detect [adapter] [--json]
qlog adapter install <adapter> [--dry-run] [--json]
qlog adapter status [adapter] [--json]
qlog adapter test <adapter> [--json]
qlog adapter verify <adapter> [--project <slug>] [--since <duration>] [--json]
qlog adapter uninstall <adapter> [--dry-run] [--json]
```

**Purpose:** Inspect, install, test, verify, and remove capture adapters.
**Safety:** `install` and `uninstall` can change qlog-owned setup; dry-run first. Verification distinguishes configuration from evidence.
**Example:** `qlog adapter verify copilot-vscode --project my-project --since 1h --json`
**Result:** status/test output includes capture quality. Copilot remains unverified until recent local OTLP `model.call` evidence with `otel_reported` tokens exists; settings alone are insufficient.

## `hook`

**Syntax:** `qlog hook claude-code`
**Purpose:** Receive Claude Code lifecycle hook payloads on standard input.
**Safety:** Hook input is reduced to privacy-safe lifecycle metadata. Prompt, transcript, and similar content are not persisted.
**Example:** `qlog hook claude-code < hook-event.json`
**Result:** `hook: ingested N` when directly stored, or `hook: forwarded` when `QLOG_COLLECTOR_URL` is set. Non-JSON input or rejected collector responses fail.

## `run`

**Syntax:** `qlog run [--project <slug>] [--agent <name>] -- <command> [arguments...]`
**Purpose:** Run a command and record privacy-safe process lifecycle metadata.
**Safety:** Command arguments, environment, and process output are intentionally not persisted. This is lifecycle evidence, not usage capture.
**Example:** `qlog run --project my-project --agent codex -- codex`
**Result:** `recorded process session <id> (exit N)`. A wrapped non-zero exit returns an error after lifecycle recording.

## `tui`

**Syntax:** `qlog tui`
**Purpose:** Open accessible terminal dashboard.
**Safety:** Dashboard uses the same local query services; it does not replace `verify` for evidence checks.
**Example:** `qlog tui`
**Result:** starts interactive dashboard. Running bare `qlog` opens it only when output is a terminal; non-terminal output shows help.

## `mcp`

**Syntax:** `qlog mcp serve`
**Purpose:** Serve local QUANTUM_LOG MCP integration over standard input/output.
**Safety:** Keep MCP stdio isolated from human-oriented shell output; it is for agent integration.
**Example:** `qlog mcp serve`
**Result:** MCP server runs until its stdio session closes; initialization errors propagate to the caller.

## `completion`

**Syntax:** `qlog completion <bash|fish|powershell|zsh>`
**Purpose:** Generate a shell-completion script for `qlog`.
**Example:** `qlog completion powershell`
**Result:** Writes the requested completion script to standard output. Running without a shell shows completion help; an unknown shell fails as an unknown command.

## `help`

**Syntax:** `qlog help [command] [flags]`
**Purpose:** Show help for `qlog` or a command path.
**Example:** `qlog help project register`
**Result:** Prints command usage, available commands, and flags. An unknown command path fails with an unknown-command error.

## Additional root groups

Current CLI also exposes `unattributed` and `budget`.

| Group | Syntax | Purpose and safety | Example and output |
| --- | --- | --- | --- |
| `unattributed` | `list [--json]`; `repair <model-call-id> --project <slug>` | Inspect calls without allocations; repair only with explicit ownership evidence. | `qlog unattributed list`; repair reports `unattributed usage: assigned`. |
| `budget` | `set-project <slug> --monthly-usd-micros <n> [--alert-percent <n>]`; `set-tag <key=value> ...`; `status [--json]` | Configure monthly allocated-cost alerts. Budgets do not block usage. | `qlog budget status --json`; invalid tag syntax or unknown projects fail. |
