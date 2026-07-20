# M4 Agent Auto-Capture Design

QUANTUM_LOG must make agent usage capture the main product path. A developer should install `qlog`, run one setup command, choose the agents they use, and get local usage evidence without hand-writing NDJSON or manually wiring every tool.

## Quick Path

1. Install qlog.
2. Run `qlog setup`.
3. Select installed agents from an interactive checklist or pass one explicitly, such as `qlog setup opencode`.
4. Let qlog write idempotent agent configuration, hooks, plugins, or local collector settings.
5. Run `qlog adapter status` to confirm which agents capture real token data, which capture lifecycle only, and which need manual setup.
6. Run normal agent work and inspect usage with `qlog usage today --group-by project,agent,provider,model`.

## Problem

M4 currently has the right foundations but not the right product experience. The repository has JSONL ingestion, OTLP ingestion, a process wrapper, an adapter interface, and detection-only command adapters. That proves the architecture can receive usage evidence, but it does not yet solve the user's primary problem: automatic capture from real coding agents.

The project must not claim universal metering unless the source exposes real usage. If an agent gives token counts, qlog records those counts as reported. If an agent only exposes process lifecycle, qlog records lifecycle evidence and labels it clearly. If qlog estimates prompt or response size locally, that estimate must be labeled as estimated and must never be merged with provider-reported tokens as if they are equivalent.

## Decision

Build M4 around a setup-first adapter system modeled after Engram's agent setup flow.

The primary command is `qlog setup`. It detects supported agents, presents a simple selection flow, writes the correct per-agent configuration, and verifies the capture path. Per-agent setup commands are also available for scripts and documentation.

```bash
qlog setup
qlog setup opencode
qlog setup claude-code
qlog setup codex
qlog setup pi
qlog setup copilot-vscode
qlog setup openclaw
qlog setup hermen
qlog adapter status
qlog adapter test opencode
```

The adapter contract remains honest. Adapters report capabilities, install status, capture quality, and the exact files or settings they manage. Install must be idempotent, support `--dry-run`, create backups before changing existing files, and avoid silent behavior changes.

## Recommended Approach

Implement three layers.

| Layer | Purpose | Why |
|---|---|---|
| Setup orchestrator | Detect agents, show checklist, run adapter installers, verify capture | Gives developers the Engram-like experience: install once, select agents, done. |
| Agent adapters | Own per-agent config, hooks, plugins, or log imports | Keeps agent-specific behavior out of core attribution and reporting logic. |
| Capture intake | Receive normalized events from hooks, plugins, OTLP, JSONL, or wrappers | Keeps storage, sanitizer, hash chain, and project resolver shared across agents. |

This is better than a proxy-only design because many coding agents cannot be forced through one proxy. It is also better than manual imports because manual ingestion does not become a habit for normal developers.

## Alternatives Considered

### Proxy-First Metering

Route every model call through qlog and count usage at the transport boundary.

Tradeoff: strongest capture when the model call passes through qlog, but unrealistic as the default because Copilot, Claude Code, Codex, OpenCode, Pi, and editor extensions have different execution models. It also risks becoming a mandatory proxy, which the project explicitly avoids.

### Manual Import First

Ask users to export logs or pipe JSONL into qlog.

Tradeoff: safest and easiest to maintain, but it fails the product goal. Developers will not consistently export logs after every session. Manual import remains a fallback, not the main path.

### Setup-First Native Adapters

Install the best available integration for each agent: plugin, hook, MCP config, local collector config, log watcher, or wrapper.

Tradeoff: more work per agent, but it matches the user goal and the Engram usability model. This is the recommended path.

## Agent Coverage Plan

| Agent | Initial target | Capture quality goal | Notes |
|---|---|---|---|
| OpenCode | Native plugin or config hook plus qlog collector/MCP registration | Reported tokens when exposed; otherwise lifecycle plus session metadata | First priority because the current project already runs inside OpenCode and Engram has a proven plugin pattern. |
| Claude Code | Plugin or hook setup, plus optional MCP registration | Reported tokens when exposed by hook/plugin data; otherwise lifecycle plus session metadata | Must support Windows carefully because hook shells can vary. |
| Codex | Config + instruction/hook/plugin path if available | Reported usage if Codex exposes it; otherwise lifecycle plus session metadata | Follow Engram's config-file pattern for `~/.codex/config.toml`. |
| Pi | Package/plugin path inspired by Engram Pi setup | Reported usage if Pi extension can observe it | Good candidate for early rich capture because Pi extension surface can emit events directly. |
| GitHub Copilot in VS Code | VS Code MCP/config first, VS Code extension later | Initially setup guidance plus lifecycle/context; extension needed for richer capture | Copilot token usage may not be directly exposed through native MCP. Do not promise full token capture until verified. |
| OpenClaw | Detect CLI/config/log surface, then adapter | Unknown until explored | Treat as supported only after a real data source is identified. |
| Hermen | Detect CLI/config/log surface, then adapter | Unknown until explored | Treat as supported only after a real data source is identified. |

## Capture Quality Model

Every model call or activity event must declare capture quality.

| Quality | Meaning |
|---|---|
| `provider_reported` | Provider reported tokens/cost directly. |
| `agent_reported` | Agent reported normalized usage from its own logs or hooks. |
| `otel_reported` | OpenTelemetry GenAI attributes supplied usage fields. |
| `lifecycle_only` | qlog observed process/session/tool lifecycle but not tokens. |
| `estimated` | qlog estimated local size or cost; not provider truth. |
| `manual_import` | User imported a structured event file. |
| `unavailable` | Source cannot expose the requested metric. |

Reports must keep these categories visible. A monthly total may sum observed token values, but UI and exports must preserve `capture_quality` so users do not confuse estimates, lifecycle evidence, and provider-reported usage.

## Setup UX

`qlog setup` should work in both interactive and non-interactive modes.

Interactive flow:

```text
qlog setup

Detected agents:
[x] OpenCode       available  capture: setup available
[x] Claude Code    available  capture: setup available
[ ] Codex          not found   capture: install skipped
[ ] Pi             not found   capture: install skipped
[ ] VS Code        available  capture: partial, extension recommended later

Install selected adapters? y/N
```

Non-interactive flow:

```bash
qlog setup opencode --yes
qlog setup claude-code --dry-run --json
qlog setup --all --yes
```

Setup output must state exactly what changed, which files were backed up, and what remains manual.

## Adapter Lifecycle

Each adapter implements this lifecycle:

1. `Detect`: identify installed agent and relevant config locations without writing.
2. `PlanInstall`: compute changes and backup paths.
3. `Install`: write idempotent config/hooks/plugins after backup.
4. `Status`: report installed, partial, drifted, or unavailable state.
5. `Test`: emit or import a harmless synthetic usage event and verify qlog can see it.
6. `Uninstall`: remove only qlog-owned marker blocks or files, never unrelated user settings.

Marker blocks are required when editing existing instruction/config files. Example:

```text
<!-- qlog:begin agent-auto-capture -->
qlog-managed instructions or configuration
<!-- qlog:end agent-auto-capture -->
```

## Data Flow

```text
Agent hook/plugin/log/wrapper
  -> adapter-specific event
  -> sanitizer
  -> normalized qlog event
  -> central project resolver
  -> SQLite raw_events + model_calls
  -> usage/report/TUI/MCP/export
```

Adapters provide signals. They do not decide final project ownership. The application service continues to resolve project identity with the established precedence: explicit project, `QLOG_PROJECT`, CWD, Git root, registered path, adapter hint, then `unattributed`.

## Privacy and Security

M4 auto-capture must preserve the existing privacy policy.

- Do not persist prompts, responses, tool arguments, tool results, environment variables, API keys, cookies, tokens, or authorization headers.
- Do not enable network listeners outside loopback unless explicitly requested.
- Do not install global hooks without showing the affected file path.
- Back up edited config files before writing.
- Keep setup idempotent.
- Keep uninstall scoped to qlog-owned markers and files.

## Implementation Slices

### Slice 1: Setup Framework

Add `qlog setup`, adapter status/test surfaces, install planning, backups, marker-block utilities, JSON output, and tests. This slice can still use detection-only adapters while proving the UX and safety rules.

### Slice 2: OpenCode Adapter

Implement first real setup adapter. It should configure qlog-owned OpenCode plugin or hook files, register qlog MCP/collector where appropriate, and verify a synthetic event reaches qlog.

### Slice 3: Claude Code Adapter

Implement Claude Code setup with Windows-safe handling, backup, marker blocks, and a synthetic capture test.

### Slice 4: Codex and Pi Adapters

Follow Engram-style config for Codex and package/plugin integration for Pi. Preserve honest capability reporting when tokens are not exposed.

### Slice 5: VS Code Copilot Track

Start with MCP/config/instructions setup for VS Code and document that full Copilot token capture likely requires a VS Code extension. Then design the extension only after verifying available VS Code/Copilot APIs.

### Slice 6: OpenClaw and Hermen Exploration

Add detection and setup only after identifying stable config, hook, log, plugin, or telemetry surfaces. Do not add fake support.

## Acceptance Criteria

- `qlog setup --dry-run --json` lists detected agents, planned changes, backup paths, and capture-quality expectations.
- `qlog setup opencode --yes` is idempotent and creates backups before modifying existing files.
- `qlog adapter status --json` distinguishes `available`, `installed`, `partial`, `drifted`, and `unavailable`.
- `qlog adapter test <agent>` proves a harmless event can be captured or returns a precise unsupported reason.
- Reports preserve `capture_quality` and do not hide lifecycle-only or estimated data.
- No setup path stores prompt or response content.
- Existing `doctor`, `verify`, and anchor checks still pass after setup and synthetic capture.
- Documentation explains the easy path first and labels unsupported token capture honestly.

## Out of Scope

- Mandatory model proxying for all users.
- Cloud sync or SaaS ingestion.
- Billing-provider reconciliation.
- Full GitHub Copilot token capture without verified extension/API evidence.
- Fake adapters that only detect a binary and claim support.

## Next Step

After this design is approved, create an implementation plan starting with Slice 1. The first implementation should make `qlog setup` safe, reviewable, and testable before adding deeper per-agent capture.
