# M4 Agent Auto-Capture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the M4 setup-first adapter surface so developers can run `qlog setup`, install safe qlog-owned agent integrations, verify capture readiness, and see honest capture quality.

**Architecture:** Add a focused setup package that plans and applies idempotent file/config changes through qlog-owned files, settings, or marker blocks. Keep agent-specific setup inside adapters; keep project resolution, storage, sanitizer, and reporting shared. First PR finishes safe setup/status/test capability and supports OpenCode, Claude Code, Codex, Pi, VS Code Copilot, OpenClaw, and Hermes with honest capability labels.

**Tech Stack:** Go 1.26, Cobra, standard library file I/O, existing `internal/adapters`, existing CLI test helpers.

## Global Constraints

- This work targets `v0.3.0`; users with `v0.2.0` must upgrade with `go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.3.0` and keep their existing qlog home/database/config usable.
- Do not break existing v0.2.0 commands: `init`, `project register/current`, `doctor`, `verify`, `anchor export/check`, `ingest`, `usage`, `report`, `mcp`, and `tui`.
- Any schema/config change must be additive and migration-safe for existing v0.2.0 databases.
- Do not persist prompts, responses, tool arguments, tool results, environment variables, API keys, cookies, tokens, or authorization headers.
- Do not claim token capture unless the adapter has a verified token source.
- Setup writes must be idempotent and support `--dry-run` and `--json`.
- Existing files must be backed up before mutation.
- Uninstall removes only qlog-owned marker blocks or qlog-owned files.
- No new mandatory network proxy.
- Keep SQLite lock protocol untouched.

---

## File Structure

- Create `internal/adapters/setup.go`: setup status types, plan types, capture quality constants, marker-block helpers, backup/write helpers.
- Modify `internal/adapters/adapters.go`: extend adapter interface with setup lifecycle and register target agents.
- Modify `internal/adapters/command.go`: replace detection-only command adapter with file-backed setup-capable adapter for known agent config surfaces.
- Modify `internal/adapters/generic.go`: implement setup lifecycle as no-op installed built-in importer.
- Modify `internal/cli/adapters.go`: add `adapter status`, `adapter test`, and uninstall command surfaces.
- Create `internal/cli/setup.go`: add `qlog setup` top-level command.
- Modify `internal/cli/root.go`: register `newSetupCommand()`.
- Modify `internal/adapters/adapters_test.go`: unit tests for setup plans, marker blocks, status, and supported adapters.
- Modify `internal/cli/capture_commands_test.go`: CLI tests for `qlog setup`, `adapter status`, and `adapter test`.
- Modify `README.md` and `docs/DEVELOPER_GUIDE.md`: document M4 setup path and honest limitations.
- Modify `cmd/qlog/main.go`: default development version becomes `0.3.0`.

---

### Task 1: Adapter Setup Contract

**Files:**
- Create: `internal/adapters/setup.go`
- Modify: `internal/adapters/adapters.go`
- Modify: `internal/adapters/generic.go`
- Test: `internal/adapters/adapters_test.go`

**Interfaces:**
- Produces: `CaptureQuality`, `SetupStatus`, `SetupPlan`, `SetupChange`, `TestResult`, `SetupOptions`, and lifecycle methods on `Adapter`.
- Consumes: existing adapter registry and descriptor types.

- [ ] **Step 1: Write failing adapter contract tests**

Add tests that expect each default adapter to expose setup status and test result:

```go
func TestDefaultRegistryAdaptersExposeSetupLifecycle(t *testing.T) {
    t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
    for _, adapter := range Default().List() {
        status, err := adapter.Status(context.Background())
        if err != nil {
            t.Fatalf("%s status: %v", adapter.Descriptor().ID, err)
        }
        if status.AdapterID != adapter.Descriptor().ID || status.State == "" || status.CaptureQuality == "" {
            t.Fatalf("%s status = %#v", adapter.Descriptor().ID, status)
        }
        result, err := adapter.Test(context.Background())
        if err != nil {
            t.Fatalf("%s test: %v", adapter.Descriptor().ID, err)
        }
        if result.AdapterID != adapter.Descriptor().ID || result.CaptureQuality == "" {
            t.Fatalf("%s test = %#v", adapter.Descriptor().ID, result)
        }
    }
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/adapters -run TestDefaultRegistryAdaptersExposeSetupLifecycle -count=1`

Expected: FAIL because `Adapter` has no `Status` or `Test` methods.

- [ ] **Step 3: Implement setup contract**

Add `internal/adapters/setup.go` with these types and constants:

```go
package adapters

import "time"

type CaptureQuality string

const (
    CaptureProviderReported CaptureQuality = "provider_reported"
    CaptureAgentReported    CaptureQuality = "agent_reported"
    CaptureOTELReported     CaptureQuality = "otel_reported"
    CaptureLifecycleOnly    CaptureQuality = "lifecycle_only"
    CaptureEstimated        CaptureQuality = "estimated"
    CaptureManualImport     CaptureQuality = "manual_import"
    CaptureUnavailable      CaptureQuality = "unavailable"
)

type SetupState string

const (
    SetupAvailable   SetupState = "available"
    SetupInstalled   SetupState = "installed"
    SetupPartial     SetupState = "partial"
    SetupDrifted     SetupState = "drifted"
    SetupUnavailable SetupState = "unavailable"
)

type SetupOptions struct {
    DryRun bool
    Yes    bool
}

type SetupChange struct {
    Path        string `json:"path"`
    Action      string `json:"action"`
    BackupPath  string `json:"backup_path,omitempty"`
    Description string `json:"description"`
}

type SetupPlan struct {
    AdapterID      string         `json:"adapter_id"`
    State          SetupState     `json:"state"`
    CaptureQuality CaptureQuality `json:"capture_quality"`
    Changes        []SetupChange  `json:"changes"`
    Notes          []string       `json:"notes"`
}

type SetupStatus struct {
    AdapterID      string         `json:"adapter_id"`
    Available      bool           `json:"available"`
    Installed      bool           `json:"installed"`
    State          SetupState     `json:"state"`
    CaptureQuality CaptureQuality `json:"capture_quality"`
    Evidence       string         `json:"evidence"`
    Notes          []string       `json:"notes,omitempty"`
}

type TestResult struct {
    AdapterID      string         `json:"adapter_id"`
    Passed         bool           `json:"passed"`
    CaptureQuality CaptureQuality `json:"capture_quality"`
    Message        string         `json:"message"`
    TestedAt       time.Time      `json:"tested_at"`
}
```

Extend `Adapter` in `internal/adapters/adapters.go`:

```go
PlanInstall(context.Context, SetupOptions) (SetupPlan, error)
Status(context.Context) (SetupStatus, error)
Test(context.Context) (TestResult, error)
```

Implement no-op lifecycle for `GenericJSONL`:

```go
func (GenericJSONL) PlanInstall(context.Context, SetupOptions) (SetupPlan, error) { ... }
func (GenericJSONL) Status(context.Context) (SetupStatus, error) { ... }
func (GenericJSONL) Test(context.Context) (TestResult, error) { ... }
```

- [ ] **Step 4: Run adapter tests**

Run: `go test ./internal/adapters -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/setup.go internal/adapters/adapters.go internal/adapters/generic.go internal/adapters/adapters_test.go
git commit -m "feat(adapters): add setup lifecycle contract"
```

---

### Task 2: Marker Blocks and Safe File Writes

**Files:**
- Modify: `internal/adapters/setup.go`
- Test: `internal/adapters/adapters_test.go`

**Interfaces:**
- Produces: `ApplyMarkerBlock(path, marker, content string, dryRun bool) (SetupChange, error)` and `HasMarkerBlock(path, marker string) bool`.
- Consumes: `SetupChange` from Task 1.

- [ ] **Step 1: Write failing marker tests**

Add tests for create, update, backup, and idempotent second apply.

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/adapters -run TestApplyMarkerBlock -count=1`

Expected: FAIL because helper is missing.

- [ ] **Step 3: Implement marker helpers**

Implement marker replacement with exact markers:

```text
<!-- qlog:begin <marker> -->
...
<!-- qlog:end <marker> -->
```

Rules:
- If file is missing, create parent directories and file.
- If marker exists, replace only marked content.
- If file exists and mutation is needed, copy to `<path>.qlog-backup-<yyyymmddhhmmss>` before writing.
- If content is unchanged, return `Action: "unchanged"` and no backup.
- If dry-run, do not create parent dirs, backups, or target file.

- [ ] **Step 4: Run marker tests**

Run: `go test ./internal/adapters -run TestApplyMarkerBlock -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/setup.go internal/adapters/adapters_test.go
git commit -m "feat(adapters): add safe marker writes"
```

---

### Task 3: Setup-Capable Agent Adapters

**Files:**
- Modify: `internal/adapters/adapters.go`
- Modify: `internal/adapters/command.go`
- Test: `internal/adapters/adapters_test.go`

**Interfaces:**
- Produces: default adapters for `opencode`, `claude-code`, `codex`, `pi`, `copilot-vscode`, `openclaw`, `hermes`.
- Consumes: marker helpers from Task 2.

- [ ] **Step 1: Write failing registry tests**

Update registry test to expect 8 adapters: `generic-jsonl`, `opencode`, `claude-code`, `codex`, `pi`, `copilot-vscode`, `openclaw`, `hermes`.

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/adapters -run TestDefaultRegistryDeclaresOnlyVerifiedCapabilities -count=1`

Expected: FAIL because only 3 adapters are registered.

- [ ] **Step 3: Implement setup-capable command adapter**

Add fields to `commandAdapter`: `configRelativePath`, `marker`, `setupText`, `captureQuality`, `notes`.

Resolve config home with `QLOG_ADAPTER_CONFIG_HOME` for tests, otherwise use `os.UserConfigDir()` and user-home based paths. Use conservative config files:
- OpenCode: `.config/opencode/AGENTS.md`
- Claude Code: `.claude/QUANTUM_LOG.md`
- Codex: `.codex/qlog-instructions.md`
- Pi: `.config/pi/qlog.md`
- VS Code Copilot: `Code/User/prompts/qlog.instructions.md`
- OpenClaw: `.config/openclaw/qlog.md`
- Hermes: `.config/hermes/qlog.md`

Each setup text must instruct agent to emit qlog-compatible local events when supported and use `qlog run`/OTLP/JSONL fallback otherwise. Do not claim prompt capture.

- [ ] **Step 4: Run adapter tests**

Run: `go test ./internal/adapters -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/adapters.go internal/adapters/command.go internal/adapters/adapters_test.go
git commit -m "feat(adapters): add setup-capable agent targets"
```

---

### Task 4: Adapter Status, Test, Uninstall CLI

**Files:**
- Modify: `internal/cli/adapters.go`
- Test: `internal/cli/capture_commands_test.go`

**Interfaces:**
- Produces CLI commands: `qlog adapter status [adapter]`, `qlog adapter test <adapter>`, `qlog adapter uninstall <adapter>`.
- Consumes adapter lifecycle from Tasks 1-3.

- [ ] **Step 1: Write failing CLI tests**

Add tests asserting:
- `qlog adapter status --json` returns JSON array.
- `qlog adapter status opencode --json` returns one JSON object.
- `qlog adapter test opencode --json` returns `passed` and `capture_quality`.
- `qlog adapter uninstall opencode --dry-run --json` returns JSON and no changed state.

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/cli -run 'TestAdapter(Status|Test|Uninstall)' -count=1`

Expected: FAIL because commands do not exist.

- [ ] **Step 3: Implement CLI commands**

Follow existing `adapter list/detect/install` style. Use root stdout for JSON/human output. Add `--json` where applicable and `--dry-run` for uninstall.

- [ ] **Step 4: Run CLI capture tests**

Run: `go test ./internal/cli -run 'TestAdapter|TestCollector' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/adapters.go internal/cli/capture_commands_test.go
git commit -m "feat(cli): expose adapter status and tests"
```

---

### Task 5: Top-Level qlog setup Command

**Files:**
- Create: `internal/cli/setup.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/capture_commands_test.go`

**Interfaces:**
- Produces CLI command: `qlog setup [adapter] [--all] [--yes] [--dry-run] [--json]`.
- Consumes `Adapter.PlanInstall` and `Adapter.Install`.

- [ ] **Step 1: Write failing setup command tests**

Add tests for:
- `qlog setup --dry-run --json` returns detected setup plans.
- `qlog setup opencode --yes --json` writes a marked config file under `QLOG_ADAPTER_CONFIG_HOME`.
- Re-running same command returns unchanged/idempotent result.

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/cli -run TestSetupCommand -count=1`

Expected: FAIL because `setup` command does not exist.

- [ ] **Step 3: Implement setup command**

Implement `newSetupCommand()` with non-interactive behavior by default for tests. If no adapter is specified, plan all known non-generic adapters. Require `--yes` to mutate unless `--dry-run` is set. For human output, print one line per plan/action.

- [ ] **Step 4: Run setup command tests**

Run: `go test ./internal/cli -run TestSetupCommand -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/setup.go internal/cli/root.go internal/cli/capture_commands_test.go
git commit -m "feat(cli): add qlog setup"
```

---

### Task 6: Documentation and Validation

**Files:**
- Modify: `README.md`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Modify: `cmd/qlog/main.go`
- Modify: `docs/superpowers/specs/2026-07-20-agent-auto-capture-design.md` if implementation terms changed.

**Interfaces:**
- Consumes all previous tasks.
- Produces developer-facing M4 setup docs.

- [ ] **Step 1: Update docs**

Add a `Setup Agent Capture` section with commands:

```bash
go install github.com/janpereira-dev/quantum_log/cmd/qlog@v0.3.0
qlog setup --dry-run
qlog setup opencode --yes
qlog adapter status --json
qlog adapter test opencode
```

State plainly: full token capture is only available when the agent source exposes reported usage; otherwise qlog records lifecycle/setup evidence with `capture_quality`.

- [ ] **Step 2: Bump default development version**

In `cmd/qlog/main.go`, set default version to `0.3.0` so local builds report the upcoming release line. Do not change release linker flags.

- [ ] **Step 3: Run focused tests**

Run: `go test ./internal/adapters ./internal/cli -count=1`

Expected: PASS.

- [ ] **Step 4: Run full validation**

Run: `go test -count=1 ./...`

Expected: PASS.

- [ ] **Step 5: Run vet**

Run: `go vet ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add README.md docs/DEVELOPER_GUIDE.md cmd/qlog/main.go docs/superpowers/specs/2026-07-20-agent-auto-capture-design.md
git commit -m "docs: document M4 agent setup"
```

---

## Self-Review Result

- Spec coverage: setup-first UX, adapter lifecycle, capture quality, privacy, and phased coverage map to Tasks 1-6.
- Placeholder scan: no TBD/TODO/placeholder instructions are present.
- Type consistency: all task interfaces use the same setup lifecycle names and JSON field names.

## Execution Choice

The user requested inline execution through PR creation. Use `executing-plans` next, then implement tasks in order.
