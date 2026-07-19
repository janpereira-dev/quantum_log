# QUANTUM_LOG

> **Trace every agent. Trust every event.**

**QUANTUM_LOG** is a local-first observability and FinOps tool for AI coding work. It records evidence about agent activity, project attribution, model usage, tasks, estimated cost, and ledger integrity without requiring a SaaS, a mandatory proxy, or a prompt archive.

It is built for people who need a clear answer to questions such as:

- Which AI agent worked on this project?
- Which provider and model were reported?
- How many tokens and how much estimated cost were recorded?
- Which task was active when the work happened?
- Did one session move between projects?
- Is the local evidence ledger still intact?
- Which events still need project attribution?

## Start Here

```text
AI agent / CLI / OTLP / NDJSON
              |
              v
       Capture and sanitize
              |
              v
    Resolve active project context
              |
              v
     Local SQLite evidence ledger
              |
              v
     Tokens, tasks, cost, reports, TUI
```

### Five-minute path

1. Install `qlog` with one of the methods below.
2. Initialize your local data directory.
3. Register the repository you are working in.
4. Start a task and ingest real telemetry or structured events.
5. Inspect usage, attribution, and ledger integrity.

```bash
qlog init
qlog project register --path . --name "My Project"
qlog task start --project my-project --type build --title "Initial AI-assisted work"
qlog project current --json
qlog usage month --group-by project,provider,model --json
qlog verify
```

Expected result: a local database is created, the current repository is registered as a logical project, and unknown usage remains visible as `unattributed` rather than being guessed.

## Installation

QUANTUM_LOG `v0.1.0` is published on [GitHub Releases](https://github.com/janpereira-dev/quantum_log/releases/tag/v0.1.0). Release archives include SHA-256 checksums, SBOMs, and a keyless Sigstore bundle for `checksums.txt`.

### Choose Your Platform

| Platform | Recommended command | Result |
|---|---|---|
| macOS or Linux | Native installer | Installs verified `qlog` to a user-owned directory |
| Windows PowerShell | Native installer | Installs verified `qlog.exe` without administrator rights |
| Any Go environment | `go install` | Builds from the tagged module source |
| npm | `npm install -g` | Downloads the verified platform binary after install |
| macOS/Linux Homebrew | `brew install` | Uses the public `janpereira-dev/tap` formula |
| Windows Scoop | `scoop install` | Uses the public `quantum-log` bucket |

### macOS and Linux

Review the script first if you use a pipeline. The installer downloads `checksums.txt`, verifies the selected archive SHA-256, stages the binary, then installs it.

```bash
curl -fsSLO https://raw.githubusercontent.com/janpereira-dev/quantum_log/main/installers/install.sh
sh install.sh --version v0.1.0
qlog --version
```

For a preview with no download or file changes:

```bash
sh install.sh --dry-run --version v0.1.0
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/janpereira-dev/quantum_log/main/installers/install.ps1 -OutFile install.ps1
./install.ps1 --version v0.1.0
qlog --version
```

Preview only:

```powershell
./install.ps1 --dry-run --version v0.1.0
```

### Go

```bash
GOFLAGS=-buildvcs=true go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.1.0
qlog --version
```

### npm

The npm package is a thin binary distributor, not a JavaScript rewrite of the core. It downloads the matching GitHub Release archive and verifies its SHA-256 entry before extracting `qlog`.

```bash
npm install -g @janpereira.dev/quantum-log
qlog --version
```

### Homebrew

```bash
brew install janpereira-dev/tap/quantum-log
qlog --version
```

### Scoop

```powershell
scoop bucket add quantum-log https://github.com/janpereira-dev/scoop-bucket
scoop install quantum-log
qlog --version
```

### Verify a Release Manually

For high-assurance environments, download an archive and `checksums.txt` from the release page, then compare hashes before extraction.

```bash
sha256sum qlog_0.1.0_linux_amd64.tar.gz
grep 'qlog_0.1.0_linux_amd64.tar.gz' checksums.txt
```

To verify the keyless checksum signature, install [Cosign](https://docs.sigstore.dev/cosign/system_config/installation/) and run:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/heads/main" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
```

## Understand the Data Model

QUANTUM_LOG intentionally separates three concepts that are often mixed together:

| Concept | Meaning | Example |
|---|---|---|
| `Project` | Stable logical identity | `quantum-log` |
| `ProjectLocation` | One physical checkout/worktree | `C:\Code\quantum_log` |
| `WorkContext` | Temporary state at an instant | CWD, Git branch, session, task |

One agent session can work in project A, then project B, then project A again. Each event keeps its own resolved project, method, evidence, and confidence.

```text
09:00  project-a  -> model call A
09:41  project-b  -> model call B
10:21  project-a  -> model call C
```

Project resolution follows evidence, not model or agent names:

```text
--project
  -> QLOG_PROJECT
  -> current working directory
  -> Git root
  -> registered location
  -> unattributed
```

When evidence is insufficient, the result is `unattributed`. That is a feature: QUANTUM_LOG does not silently invent ownership.

## Everyday Workflow

### 1. Initialize once

```bash
qlog init
qlog doctor --json
```

`init` is idempotent. It creates `config.yaml` and `qlog.db` in your local QUANTUM_LOG home. `doctor` checks SQLite integrity without changing data.

Set `QLOG_HOME` to place all local state somewhere else.

```bash
QLOG_HOME=/secure/qlog qlog init
```

Default locations:

| OS | Default data location |
|---|---|
| Windows | `%LOCALAPPDATA%` when set; otherwise user configuration directory under `QUANTUM_LOG` |
| Linux | `$XDG_DATA_HOME/quantum-log` or `~/.local/share/quantum-log` |
| macOS | User configuration directory for `QUANTUM_LOG` |

### 2. Register projects and tags

```bash
qlog project register --path . --name "Customer Portal" --slug customer-portal
qlog project tag --project customer-portal environment=work
qlog project tag --project customer-portal cost-center=research
qlog project current --json
qlog project list --json
```

### 3. Track meaningful work

```bash
qlog task start \
  --project customer-portal \
  --type build \
  --title "Implement account export"

qlog task list --project customer-portal --json
qlog task finish TASK_ID --result success
```

### 4. Capture evidence

Choose the strongest source your agent supports.

#### NDJSON import

```bash
qlog ingest file events.ndjson
qlog ingest stdin < events.ndjson
```

For `model.call` events, QUANTUM_LOG can normalize provider, model, agent, input/output/reasoning/cache tokens, estimated cost, session, task, and capture quality when those fields exist.

#### OTLP/HTTP

Start a loopback-only receiver:

```bash
qlog collector serve
```

Send OTLP JSON traces to:

```text
POST http://127.0.0.1:4318/v1/traces
```

Recognized GenAI signals include `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `session.id`, and `service.name`. The receiver also accepts project-context signals such as CWD, Git root, branch, workspace, and declared project.

#### Process wrapper

```bash
qlog run --agent codex -- codex
qlog run --agent claude -- claude
```

The wrapper records process lifecycle, executable name, PID, exit code, duration, project context, and Git context. It **does not** record command arguments, environment variables, terminal output, prompts, or responses.

### 5. Inspect usage and reports

```bash
qlog usage today --group-by project,provider,model
qlog usage week --group-by provider,model,project --json
qlog usage month --group-by project,provider,model --json
qlog report summary --json
qlog export --format csv --redact-paths
```

Money is stored as integer micros, never floating point. Reported token usage remains separate from allocated cost.

### 6. Handle shared or unknown ownership

Split one model call by basis points:

```bash
qlog allocation split CALL_ID customer-portal=6000 internal-tools=4000
```

This means 60% / 40%. The original observed token count is never rewritten to fake a split.

Repair one missing allocation with explicit ownership:

```bash
qlog allocation repair CALL_ID --project customer-portal
```

### 7. Verify integrity

```bash
qlog verify
qlog verify --session SESSION_ID
```

Raw events are append-only and chained by SHA-256 per source/session. Changing stored evidence outside QUANTUM_LOG should break verification.

## Cost and Pricing

Pricing rules are versioned and time-bound. They support input, output, reasoning, cached-input, and cache-write token prices.

```bash
qlog pricing validate pricing.json
qlog pricing add pricing.json
qlog pricing list --json
qlog pricing show PROVIDER/MODEL_PATTERN --json
qlog pricing recalculate
```

## Terminal Dashboard

```bash
qlog tui
```

The TUI has Overview, Projects, Tasks, and Integrity views. It remains readable in compact terminals and uses clear text labels in addition to color.

| Key | Action |
|---|---|
| `Tab`, `Left`, `Right`, `1`-`4` | Navigate views |
| `Esc` | Return to Overview |
| `?` | Toggle keyboard help |
| `q`, `Ctrl+C` | Quit |

Set `NO_COLOR=1` for plain-text output. Running `qlog` without arguments opens the TUI only on an interactive terminal; redirected output shows help instead.

## Privacy and Security

By default, QUANTUM_LOG is local-first.

- Imported prompt content, response content, tool arguments/results, authorization values, API keys, tokens, secrets, and passwords are removed from raw payloads.
- No outbound connection occurs during normal ledger use.
- Release download, explicit price update, and explicit adapter update are the only intended network categories.
- Absolute paths are retained locally only for project resolution. Use `qlog export --redact-paths` before sharing an export.
- `qlog doctor` validates SQLite health; `qlog verify` validates the hash chain.

Read [SECURITY.md](SECURITY.md) before using QUANTUM_LOG with sensitive repositories.

## What QUANTUM_LOG Does Not Do Yet

Be precise about current capabilities:

- OpenCode and Claude Code adapters are detection-only. They do not install hooks or claim token, cost, tool, or context capture.
- The generic wrapper proves process activity; it cannot see token usage inside an agent/provider protocol.
- Automatic agent metering requires OTLP, structured logs, hooks, plugins, or a verified adapter that exposes real data.
- There is no native EUR pricing catalog or historical FX engine yet. EUR import fields may exist, but pricing rules currently calculate USD token cost.
- `--redact-paths` is explicit for exports. Do not share unredacted exports outside your trusted environment.
- GitHub Copilot, Codex, Cursor, Gemini CLI, Cline, Roo Code, Continue, Amazon Q, and Windsurf do not yet have dedicated full-capture adapters.

This is a functional local observability and FinOps core, not a plug-and-play universal meter for every AI agent.

## Architecture

```text
CLI / TUI / adapters / MCP
             |
             v
    application services
             |
             v
domain + project resolver
             |
             v
SQLite / pricing / external integrations
```

The domain does not depend on Cobra, Bubble Tea, or SQLite. SQLite uses `modernc.org/sqlite`, keeps `CGO_ENABLED=0` for normal builds, embeds SQL migrations, and stores raw events append-only.

## Troubleshooting

| Symptom | What to do |
|---|---|
| `qlog` is not found | Open a new terminal, or install with `--no-modify-path` disabled. |
| Project is `unattributed` | Run `qlog project current --json`, register the location, or pass `--project`. |
| No tokens after `qlog run` | Expected without real telemetry. Use OTLP, NDJSON, hooks, or a verified capture adapter. |
| `verify` fails | Preserve the database, export diagnostic context, and investigate external SQLite modification. |
| Installer rejects archive | Do not bypass it. Re-download `checksums.txt` and archive from the matching release. |
| TUI has unreadable color | Set `NO_COLOR=1`, or use a terminal with standard ANSI support. |

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md). Use real tests and evidence; do not commit databases, binaries, credentials, or fabricated usage data.

## License

MIT. See [LICENSE](LICENSE).
