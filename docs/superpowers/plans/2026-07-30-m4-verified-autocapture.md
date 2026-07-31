# M4 Verified Autocapture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver truthful, privacy-first, replay-safe automatic capture for Codex, Claude Code, GitHub Copilot CLI/VS Code, and OpenCode, with release verification based on real-agent evidence.

**Architecture:** Keep agent-specific setup and evidence claims in `internal/adapters`; keep bootstrap orchestration in CLI; retain one loopback collector with per-request SQLite access. Add deterministic sanitized ingestion identity before ledger append, and let normalization run only for an accepted raw event. Verification queries durable raw-event/model-call linkage rather than setup status alone.

**Tech Stack:** Go, Cobra, `modernc.org/sqlite`, embedded SQL migrations, OTLP/HTTP, qlog JSON event receiver, Windows Task Scheduler, launchd, systemd user services, PowerShell, POSIX shell.

## Global Constraints

- M4 stable-adapter set is exactly `codex`, `claude-code`, `copilot-vscode`, and `opencode`.
- `pi`, `openclaw`, and `hermes` are unsupported: exclude them from default bootstrap, stable-adapter status, verification, docs, and release assurance. Preserve generic/manual JSONL import behavior without presenting it as M4 autocapture.
- Only persist allowlisted, sanitized metadata. Never hash, persist, export, or log prompt/response content, transcript references, tool data, environment values, cookies, authorization, API keys, tokens, secrets, passwords, or remote URLs carrying credentials.
- Resolve project ownership centrally. Never infer ownership from provider, model, or agent name; unresolved ownership remains `unattributed`.
- `agent_reported` and `otel_reported` require real documented source counters. `lifecycle_only` has no token counters. `estimated` is visibly non-measured and never substitutes for observed usage.
- Collector default is `127.0.0.1:4318`; non-loopback listeners require explicit `--allow-non-loopback`.
- Every official installer path must obtain clear installer-level consent before bootstrap and offer explicit opt-out. Re-running bootstrap must be idempotent and preserve recoverable user configuration.
- Do not claim M4 complete until clean-device real-agent evidence exists for every supported adapter/version/target OS. Synthetic tests never satisfy this gate.
- Run `go test -count=1 ./...` and `go vet ./...` before each commit. Do not stage databases, WAL/SHM files, locks, or existing user changes.

## Existing Changes Excluded From This Plan

- `CLAUDE.md` is modified by user and is out of scope.
- `QUANTUM_LOG_MASTER_PROMPT.md` is modified by user and is out of scope.
- `docs/superpowers/specs/2026-07-30-m4-verified-autocapture-design.md` is approved input, not an implementation target.
- Any other pre-existing untracked `docs/superpowers/` artifact is out of scope except files explicitly created by tasks below.

## Planned File Structure

- `internal/adapters/adapters.go`: stable adapter scope and truthful adapter status contract.
- `internal/adapters/{codex,claude_code,opencode,vscode_copilot}.go`: four supported configuration, detection, uninstall, and evidence metadata paths.
- `internal/adapters/adapters_test.go`: stable-scope and each adapter's setup/status contract tests.
- `internal/cli/setup.go`, `internal/cli/adapters.go`: consented bootstrap orchestration and failing-closed evidence verification.
- `internal/cli/collector.go`, `internal/cli/collector_windows.go`, `internal/cli/collector_darwin.go`, `internal/cli/collector_linux.go`: collector supervisor abstraction and persistent per-user OS lifecycle implementations.
- `internal/cli/{setup_test,capture_commands_test,collector_windows_test,collector_darwin_test,collector_linux_test}.go`: bootstrap, command, and lifecycle command tests.
- `internal/ingest/{qlogevent,otlp}/`: sanitized upstream identity propagation; documented Codex payload handling only.
- `internal/ingest/jsonl/importer.go`: normalize model calls only after store accepts an event.
- `internal/storage/sqlite/migrations/006_ingestion_identity.sql`: durable identity/suppression schema and uniqueness rule.
- `internal/storage/sqlite/store.go`: atomic append-or-suppress API, raw-event-to-model-call linkage, evidence and session-snapshot queries.
- `internal/storage/sqlite/{store_test,reporting_test}.go`: concurrent/replayed ingestion and grouped quality/confidence report coverage.
- `installers/{install.ps1,install.sh}` and `internal/distribution/installers_test.go`: consented automatic bootstrap in official qlog installation paths, with an explicit no-bootstrap opt-out.
- `docs-int/verification/m4-evidence.md`, `README.md`, `docs/DEVELOPER_GUIDE.md`, `docs-int/releases/distribution.md`: truthful support matrix, lifecycle operations, and real-device evidence record.

---

### Task 1: Establish Four-Adapter Truthfulness And Source-Evidence Gates

**Files:**
- Modify: `internal/adapters/adapters.go:34-39,121-135`
- Modify: `internal/adapters/setup.go:11-70`
- Modify: `internal/adapters/codex.go:11-59`
- Modify: `internal/adapters/claude_code.go:20-104`
- Modify: `internal/adapters/opencode.go:20-64`
- Modify: `internal/adapters/vscode_copilot.go:17-69`
- Modify: `internal/adapters/adapters_test.go`
- Modify: `internal/cli/setup.go:12-95`
- Modify: `internal/cli/adapters.go:31-212`

**Interfaces:**
- Produces `Descriptor{ID, Name, Version, Capabilities, Stable bool}` where `Stable` is true only for the four M4 adapters.
- Produces `SetupStatus{InstallationState SetupState, CollectorReachable bool, RecentEvidence bool, CaptureQuality CaptureQuality, Evidence string}`; do not overload `Installed` to mean verified.
- Produces `Registry.Stable() []Adapter`, sorted by ID, returning only the four M4 adapters.
- Consumes existing `Adapter` lifecycle methods without adding a fabricated agent runtime API.

- [ ] **Step 1: Write failing stable-scope tests**

```go
func TestStableAdaptersContainOnlyM4Contract(t *testing.T) {
	ids := make([]string, 0)
	for _, adapter := range Default().Stable() {
		ids = append(ids, adapter.Descriptor().ID)
		if !adapter.Descriptor().Stable {
			t.Fatalf("stable adapter %q lacks stable descriptor flag", adapter.Descriptor().ID)
		}
	}
	if diff := cmp.Diff([]string{"claude-code", "codex", "copilot-vscode", "opencode"}, ids); diff != "" {
		t.Fatalf("stable adapter ids (-want +got):\n%s", diff)
	}
}

func TestUnsupportedAdaptersAreNotSelectedByDefaultSetup(t *testing.T) {
	items, err := setupDefaultAdapters(context.Background(), adapters.Default().List())
	if err != nil { t.Fatal(err) }
	for _, adapter := range items {
		if adapter.Descriptor().ID == "pi" || adapter.Descriptor().ID == "openclaw" || adapter.Descriptor().ID == "hermes" {
			t.Fatalf("unsupported adapter selected: %s", adapter.Descriptor().ID)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/adapters ./internal/cli -run "TestStableAdaptersContainOnlyM4Contract|TestUnsupportedAdaptersAreNotSelectedByDefaultSetup"`

Expected: FAIL because `Descriptor.Stable` and `Registry.Stable` do not exist and default setup still evaluates seven adapters.

- [ ] **Step 3: Implement scope and truthful dimensions**

```go
type Descriptor struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Version string `json:"version"`
	Stable bool `json:"stable"`
	Capabilities Capabilities `json:"capabilities"`
}

func (r *Registry) Stable() []Adapter {
	stable := make([]Adapter, 0, 4)
	for _, adapter := range r.List() {
		if adapter.Descriptor().Stable { stable = append(stable, adapter) }
	}
	return stable
}
```

Set `Stable: true` only in four supported adapter descriptors. Change `setupDefaultAdapters` to iterate `registry.Stable()` supplied by `newSetupCommand`, not `registry.List()`. Keep `generic-jsonl` import command available but set `Stable: false`; retain Pi/OpenClaw/Hermes only if compatibility requires `adapter detect <id>`, never as defaults or verified capture adapters.

Set capabilities and `CaptureQuality` only to evidence current adapter can emit: Claude Code stays `lifecycle_only`; Copilot stays `unavailable` until a documented VS Code Copilot OTLP source/version is independently confirmed, then becomes `otel_reported`; Codex stays `unavailable` until Task 4 gate confirms a documented forwarding path; OpenCode stays `lifecycle_only` until documented source-backed plugin usage field evidence exists, then may become `agent_reported`. Do not label configuration as `agent_reported` or `experimental`.

- [ ] **Step 4: Add per-adapter contract tests**

```go
func TestClaudeCodeStatusIsLifecycleOnly(t *testing.T) {
	status, err := newClaudeCodeAdapter().Status(context.Background())
	if err != nil { t.Fatal(err) }
	if status.CaptureQuality != CaptureLifecycleOnly { t.Fatalf("quality = %q", status.CaptureQuality) }
}

func TestUnverifiedTokenAdaptersReportUnavailable(t *testing.T) {
	for _, adapter := range []Adapter{newCodexAdapter(), newVSCodeCopilotAdapter()} {
		status, err := adapter.Status(context.Background())
		if err != nil { t.Fatal(err) }
		if status.CaptureQuality != CaptureUnavailable { t.Fatalf("%s quality = %q", status.AdapterID, status.CaptureQuality) }
	}
}

func TestOpenCodeStatusIsLifecycleOnly(t *testing.T) {
	status, err := newOpenCodeAdapter().Status(context.Background())
	if err != nil { t.Fatal(err) }
	if status.CaptureQuality != CaptureLifecycleOnly { t.Fatalf("quality = %q", status.CaptureQuality) }
}
```

- [ ] **Step 5: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/adapters ./internal/cli -run "TestStableAdaptersContainOnlyM4Contract|TestUnsupportedAdaptersAreNotSelectedByDefaultSetup|TestClaudeCodeStatusIsLifecycleOnly|TestOpenCodeStatusIsLifecycleOnly|TestUnverifiedTokenAdaptersReportUnavailable"`

Expected: PASS.

- [ ] **Step 6: Record source-evidence gates before enabling token claims**

Create a dated evidence table in `docs-int/verification/m4-evidence.md` with columns `adapter`, `supported version`, `official source URL`, `documented configuration key/hook`, `emitted identity`, `token fields`, `privacy setting`, and `reviewer`. Require all cells before changing Codex or Copilot from `unavailable`, or OpenCode from `lifecycle_only`, to a reported quality.

Codex blocker: repository only contains an instruction file and `normalizeCodexRawResponse`; it has no current documented installation/configuration route that forwards `rawResponse/completed` to `/v1/events`. Do not implement forwarding until official source proves a supported route.

Copilot blocker: source configures VS Code only, not Copilot CLI. Do not state Copilot CLI is supported until official source proves a separate documented telemetry/configuration path and emitted identity.

- [ ] **Step 7: Commit**

```bash
git add internal/adapters internal/cli/setup.go internal/cli/adapters.go docs-int/verification/m4-evidence.md
git commit -m "feat: constrain M4 adapter scope"
```

### Task 2: Add Consented Bootstrap Semantics To Official qlog Installers

**Files:**
- Modify: `internal/cli/setup.go:12-118`
- Modify: `internal/cli/setup_test.go`
- Modify: `installers/install.ps1:17-166`
- Modify: `installers/install.sh:13-177`
- Modify: `internal/distribution/installers_test.go`
- Modify: `docs-int/verification/m4-evidence.md`

**Interfaces:**
- Produces `BootstrapResult{Consent bool, Collector CollectorBootstrapStatus, Adapters []adapters.SetupPlan}` serialized by `qlog setup --json`.
- Produces `bootstrapSupportedAdapters(ctx context.Context, home string, yes, dryRun bool, registry *adapters.Registry, manager collectorManager) (BootstrapResult, error)`.
- Consumes `Registry.Stable`, `Adapter.PlanInstall`, `Adapter.Install`, and collector `Install`/`Start`.
- Produces installer flags `--bootstrap` and `--no-bootstrap`; after clear installer-level consent, official qlog installers invoke installed qlog as `qlog setup --yes` unless `--no-bootstrap` is selected.

- [ ] **Step 1: Write failing bootstrap tests**

```go
func TestSetupYesBootstrapsCollectorBeforeAdapterFiles(t *testing.T) {
	home := t.TempDir()
	manager := &fakeCollectorManager{}
	result, err := bootstrapSupportedAdapters(context.Background(), home, true, false, adapters.Default(), manager)
	if err != nil { t.Fatal(err) }
	if !result.Consent || !manager.installed || !manager.started { t.Fatalf("bootstrap = %#v", result) }
	if got := adapterIDs(result.Adapters); !slices.Equal(got, []string{"claude-code", "codex", "copilot-vscode", "opencode"}) { t.Fatalf("adapters = %v", got) }
}

func TestSetupWithoutConsentOnlyPrintsPlan(t *testing.T) {
	manager := &fakeCollectorManager{}
	result, err := bootstrapSupportedAdapters(context.Background(), t.TempDir(), false, false, adapters.Default(), manager)
	if err != nil { t.Fatal(err) }
	if result.Consent || manager.installed || manager.started { t.Fatalf("mutated without consent: %#v", result) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/cli -run "TestSetupYesBootstrapsCollectorBeforeAdapterFiles|TestSetupWithoutConsentOnlyPrintsPlan"`

Expected: FAIL because `bootstrapSupportedAdapters`, `BootstrapResult`, and injectable collector manager do not exist.

- [ ] **Step 3: Implement idempotent bootstrap**

Implement `bootstrapSupportedAdapters` in `internal/cli/setup.go`. Resolve qlog home first, build plans for `registry.Stable()`, and return plans without mutation unless `yes` is true. With consent, call `manager.Install(home, "127.0.0.1:4318")`, `manager.Start(home, "127.0.0.1:4318")`, then apply only adapters whose detection reports available. Preserve current adapter-specific backup behavior and return every changed/unchanged/skipped action. Treat an already healthy collector and unchanged qlog-managed configuration as success.

Make `newSetupCommand` call this function. `qlog setup` remains manual fallback; it must not imply that it was run by an agent installer.

- [ ] **Step 4: Test consented automatic bootstrap in official qlog installers**

Add assertions that qlog installers expose clear bootstrap consent text, `--bootstrap`, and `--no-bootstrap`. Interactive installation defaults to invoking `qlog setup --yes` after the user accepts installer-level consent; `--bootstrap` selects the same behavior for non-interactive installation and `--no-bootstrap` suppresses it. Invoke bootstrap only after binary verification.

```go
func TestOfficialQlogInstallersExposeConsentedBootstrapAndOptOut(t *testing.T) {
	for _, name := range []string{"installers/install.ps1", "installers/install.sh"} {
		contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
		if err != nil { t.Fatal(err) }
		for _, want := range []string{"--bootstrap", "--no-bootstrap", "qlog setup --yes", "consent"} {
			if !strings.Contains(string(contents), want) { t.Fatalf("%s missing %q", name, want) }
		}
	}
}
```

- [ ] **Step 5: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/cli ./internal/distribution -run "TestSetupYesBootstrapsCollectorBeforeAdapterFiles|TestSetupWithoutConsentOnlyPrintsPlan|TestOfficialQlogInstallersExposeConsentedBootstrapAndOptOut"`

Expected: PASS.

- [ ] **Step 6: Record qlog-installer bootstrap evidence without blocking on third-party installers**

In `m4-evidence.md`, record clean-device evidence for each official qlog installer path: installer-level consent or explicit `--bootstrap`, bootstrap invocation, collector health, adapter setup state, opt-out behavior, and backup path. Do not require or claim hooks in third-party agent installers. Preserve Task 1 and Task 4 source-evidence gates for agent-specific telemetry integration, event identity, and token-quality claims. `qlog setup` remains a documented manual-install fallback.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/setup.go internal/cli/setup_test.go installers/install.ps1 installers/install.sh internal/distribution/installers_test.go docs-int/verification/m4-evidence.md
git commit -m "feat: add consented capture bootstrap"
```

### Task 3: Implement Persistent Cross-Platform Collector Lifecycle

**Files:**
- Modify: `internal/cli/collector.go:149-166`
- Modify: `internal/cli/collector_windows.go:13-109`
- Delete: `internal/cli/collector_other.go`
- Create: `internal/cli/collector_darwin.go`
- Create: `internal/cli/collector_linux.go`
- Create: `internal/cli/collector_windows_test.go`
- Create: `internal/cli/collector_darwin_test.go`
- Create: `internal/cli/collector_linux_test.go`
- Modify: `internal/cli/capture_commands_test.go:229-282`

**Interfaces:**
- Replaces string-only lifecycle response with `CollectorStatus{Installed, Running, Reachable bool; Listen, ServiceID, StatePath, LogPath, Message string}`.
- Replaces `collectorManager` methods with `Install(home, listen string) (CollectorStatus, error)`, `Start(home, listen string) (CollectorStatus, error)`, `Stop() (CollectorStatus, error)`, `Restart(home, listen string) (CollectorStatus, error)`, `Status(ctx context.Context, listen string) (CollectorStatus, error)`, `Logs() (string, error)`, `Uninstall() (CollectorStatus, error)`.
- OS targets: Windows Task Scheduler task `QUANTUM_LOG Collector`; macOS LaunchAgent label `dev.quantum-log.collector`; Linux user service `quantum-log-collector.service`.

- [ ] **Step 1: Write failing lifecycle command tests**

```go
func TestCollectorStatusReportsPersistentLifecycleFields(t *testing.T) {
	status := CollectorStatus{Installed: true, Running: true, Reachable: true, Listen: "127.0.0.1:4318", ServiceID: "dev.quantum-log.collector"}
	encoded, err := json.Marshal(status)
	if err != nil { t.Fatal(err) }
	for _, want := range []string{`"installed":true`, `"running":true`, `"reachable":true`, `"service_id":"dev.quantum-log.collector"`} {
		if !bytes.Contains(encoded, []byte(want)) { t.Fatalf("status = %s", encoded) }
	}
}
```

Add build-tagged tests that assert generated Windows XML contains `LogonTrigger`, a non-admin executable path, `collector serve --listen 127.0.0.1:4318`, and qlog home; macOS plist contains `RunAtLoad`, `KeepAlive`, same arguments, and per-user log paths; Linux unit contains `[Install]`, `WantedBy=default.target`, `Restart=on-failure`, and same arguments.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/cli -run "TestCollectorStatusReportsPersistentLifecycleFields|TestWindowsCollectorServiceDefinition|TestDarwinCollectorServiceDefinition|TestLinuxCollectorServiceDefinition"`

Expected: FAIL because `CollectorStatus` and platform service definitions do not exist.

- [ ] **Step 3: Implement lifecycle manager contract and OS definitions**

Change lifecycle commands to report structured JSON/text from `CollectorStatus`. `collector install` creates only user-owned service definition and state directory; `start` enables/loads then starts; `stop` stops without deleting configuration; `uninstall` disables/unloads then removes only qlog-owned service/state files. Every start/status result must probe `/healthz` with `probeCollectorHealth` and report `Reachable` separately from installed/running.

On Windows, replace PID-file process spawning with a Task Scheduler task created by `schtasks.exe /Create /SC ONLOGON /RL LIMITED`; query with `schtasks.exe /Query`; run with `/Run`; stop with `/End`; delete with `/Delete /F`. On macOS, write qlog-owned plist under `~/Library/LaunchAgents` and use `launchctl bootstrap/bootout/kickstart`. On Linux, write qlog-owned unit under `~/.config/systemd/user` and use `systemctl --user daemon-reload/enable/start/stop/disable`. Return platform command errors unchanged with service identifier/path so recovery is actionable.

- [ ] **Step 4: Preserve loopback refusal and test management idempotence**

```go
func TestCollectorInstallIsIdempotent(t *testing.T) {
	manager := newTestCollectorManager(t)
	first, err := manager.Install(t.TempDir(), "127.0.0.1:4318")
	if err != nil { t.Fatal(err) }
	second, err := manager.Install(t.TempDir(), "127.0.0.1:4318")
	if err != nil { t.Fatal(err) }
	if !first.Installed || !second.Installed || first.ServiceID != second.ServiceID { t.Fatalf("%#v %#v", first, second) }
}
```

- [ ] **Step 5: Run focused and cross-compile tests to verify pass**

Run: `go test -count=1 ./internal/cli -run "TestCollector|TestWindowsCollectorServiceDefinition|TestDarwinCollectorServiceDefinition|TestLinuxCollectorServiceDefinition"`

Expected: PASS on host.

Run: `set GOOS=windows&& set GOARCH=amd64&& go test ./internal/cli`

Expected: PASS on Windows cross-build test compilation.

Run: `set GOOS=darwin&& set GOARCH=arm64&& go test ./internal/cli`

Expected: PASS on macOS cross-build test compilation.

Run: `set GOOS=linux&& set GOARCH=amd64&& go test ./internal/cli`

Expected: PASS on Linux cross-build test compilation.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/collector.go internal/cli/collector_windows.go internal/cli/collector_darwin.go internal/cli/collector_linux.go internal/cli/collector_windows_test.go internal/cli/collector_darwin_test.go internal/cli/collector_linux_test.go internal/cli/capture_commands_test.go
git rm internal/cli/collector_other.go
git commit -m "feat: persist collector across supported platforms"
```

### Task 4: Make Collector-First Adapter Ingestion Truthful, With Codex Feasibility Gate

**Files:**
- Modify: `internal/ingest/qlogevent/handler.go:55-161`
- Modify: `internal/ingest/qlogevent/handler_test.go`
- Modify: `internal/ingest/otlp/receiver.go:90-166`
- Modify: `internal/ingest/otlp/receiver_test.go`
- Modify: `internal/cli/hook.go`
- Modify: `internal/cli/capture_commands_test.go:284-347`
- Modify: `internal/adapters/{claude_code,opencode,vscode_copilot,codex}.go`
- Modify: `docs-int/verification/m4-evidence.md`

**Interfaces:**
- Adds optional `UpstreamEventID string \`json:"upstream_event_id"\`` to `qlogevent.Event`.
- Adds `IngestionIdentity string` to JSONL internal event and `sqlite.RawEventInput` in Task 5.
- `qlogevent.Ingest` and OTLP receiver return `(accepted int, duplicates int, err error)` internally; HTTP response exposes `{"accepted":n,"duplicates":n}`.
- Claude hook must post to collector first; direct SQLite ingestion remains only fallback when no collector endpoint is configured and must report `lifecycle_only`.

- [ ] **Step 1: Write failing collector-first tests**

```go
func TestHookClaudeCodePostsToConfiguredCollectorWithoutDirectWrite(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/events" { t.Fatalf("path = %s", r.URL.Path) }
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accepted":1,"duplicates":0}`))
	}))
	t.Setenv("QLOG_COLLECTOR_URL", server.URL+"/v1/events")
	if _, err := runQLogWithInput(t, t.TempDir(), strings.NewReader(`{"hook_event_name":"Stop"}`), "hook", "claude-code"); err != nil { t.Fatal(err) }
	if calls != 1 { t.Fatalf("collector calls = %d", calls) }
}

func TestPluginPayloadAllowlistDropsRemoteURLAndTranscript(t *testing.T) {
	got := sanitizePluginPayload(json.RawMessage(`{"provider":"x","model":"y","remote_url":"https://u:p@example.test","transcript_path":"x","input_tokens":1}`))
	if bytes.Contains(got, []byte("remote_url")) || bytes.Contains(got, []byte("transcript_path")) { t.Fatalf("payload = %s", got) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/cli ./internal/ingest/qlogevent -run "TestHookClaudeCodePostsToConfiguredCollectorWithoutDirectWrite|TestPluginPayloadAllowlistDropsRemoteURLAndTranscript"`

Expected: FAIL because sanitizer is deny-list based and the hook path can ingest directly.

- [ ] **Step 3: Replace plugin sanitization with metadata allowlist**

Make `sanitizePluginPayload` decode object and construct a new map containing only `provider`, `model`, `model_id`, `agent_name`, `input_tokens`, `output_tokens`, `reasoning_tokens`, `cached_input_tokens`, `cache_write_tokens`, `capture_quality`, `task_id`, and `turn_id`. Validate integer counter values as non-negative. Do not copy unknown fields. Keep `ProjectHint` separate and pass only `project`/`cwd` into the central resolver.

In OTLP, construct the raw payload from documented allowlisted GenAI, workspace, and trace fields only. Add `trace_id` as upstream identity only when OTLP source provides it; never consume arbitrary remote URL attributes.

- [ ] **Step 4: Add Codex source-evidence decision gate**

Before changing `codexAdapter.Install`, add one explicit release-gate test/document record requiring all of:

```text
official Codex source URL
supported Codex version range
documented opt-in configuration syntax
documented localhost event-forwarding transport
documented stable event ID or equivalent replay identity
documented rawResponse/completed usage fields
```

If any evidence is absent, implement no Codex forwarding configuration. Make `codexAdapter.PlanInstall` return `CaptureUnavailable` with note `no documented collector forwarding integration recorded`; make `adapter verify codex` fail evidence stage. Existing `normalizeCodexRawResponse` remains parser coverage for externally received event data, not proof of a supported adapter integration.

- [ ] **Step 5: Add OpenCode and Copilot evidence-conditioned tests**

```go
func TestOpenCodeUsageQualityRequiresNonNegativeDocumentedUsage(t *testing.T) {
	payload := pluginPayloadWithUsage(-1, 2)
	if payload.CaptureQuality != "lifecycle_only" { t.Fatalf("quality = %q", payload.CaptureQuality) }
}

func TestOTLPUsesTraceIDAsUpstreamIdentity(t *testing.T) {
	event, err := receiver.event(context.Background(), resource, spanAttrs, span{TraceID: "trace-1"})
	if err != nil { t.Fatal(err) }
	if event["upstream_event_id"] != "trace-1" { t.Fatalf("event = %#v", event) }
}
```

Only change OpenCode or Copilot reported quality after Task 1 evidence table proves their exact event/payload source. Copilot CLI remains unavailable unless separate evidence exists; do not silently treat VS Code evidence as CLI evidence.

- [ ] **Step 6: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/cli ./internal/ingest/qlogevent ./internal/ingest/otlp -run "TestHookClaudeCodePostsToConfiguredCollectorWithoutDirectWrite|TestPluginPayloadAllowlistDropsRemoteURLAndTranscript|TestOpenCodeUsageQualityRequiresNonNegativeDocumentedUsage|TestOTLPUsesTraceIDAsUpstreamIdentity"`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/hook.go internal/cli/capture_commands_test.go internal/ingest/qlogevent internal/ingest/otlp internal/adapters docs-int/verification/m4-evidence.md
git commit -m "feat: route adapter evidence through collector"
```

### Task 5: Add Replay-Safe Durable Ingestion Deduplication

**Files:**
- Create: `internal/storage/sqlite/migrations/006_ingestion_identity.sql`
- Modify: `internal/storage/sqlite/store.go:55-67,478-525,944-975`
- Modify: `internal/storage/sqlite/store_test.go`
- Modify: `internal/ingest/jsonl/importer.go:16-133`
- Modify: `internal/ingest/jsonl/importer_test.go`
- Modify: `internal/ingest/qlogevent/handler.go`
- Modify: `internal/ingest/otlp/receiver.go`

**Interfaces:**
- Adds `RawEventInput.IngestionIdentity string` and `RawEventAppendResult{ID string; Accepted bool; SuppressionReason string}`.
- Replaces `AppendRawEvent(ctx, input) (string, error)` with `AppendRawEvent(ctx, input) (RawEventAppendResult, error)`.
- Adds `CanonicalIngestionIdentity(input RawEventInput, sanitizedPayload []byte) (string, error)`.
- Adds `raw_event_dedup` table keyed by `ingestion_identity`, containing first raw-event ID, source, suppression count, first/last received timestamps, and no sensitive content.

- [ ] **Step 1: Write failing store and importer tests**

```go
func TestAppendRawEventSuppressesReplayWithoutChangingLedger(t *testing.T) {
	store := openTestStore(t)
	input := RawEventInput{Source: "opencode-plugin", SessionID: "s", EventType: "model.call", OccurredAt: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC), Payload: []byte(`{"provider":"x","model":"y","input_tokens":2}`)}
	first, err := store.AppendRawEvent(context.Background(), input)
	if err != nil || !first.Accepted { t.Fatalf("first = %#v, %v", first, err) }
	second, err := store.AppendRawEvent(context.Background(), input)
	if err != nil || second.Accepted || second.ID != first.ID || second.SuppressionReason != "duplicate_ingestion_identity" { t.Fatalf("second = %#v, %v", second, err) }
	if err := store.VerifyLedger(context.Background(), "s"); err != nil { t.Fatal(err) }
	assertCount(t, store, "raw_events", 1)
	assertCount(t, store, "raw_event_dedup", 1)
}

func TestDistinctEventsWithSharedSessionAndTimeAreAccepted(t *testing.T) {
	base := RawEventInput{Source: "opencode-plugin", SessionID: "s", EventType: "model.call", OccurredAt: fixedTime, Payload: []byte(`{"provider":"x","model":"y","turn_id":"a"}`)}
	if got, err := store.AppendRawEvent(ctx, base); err != nil || !got.Accepted { t.Fatal(got, err) }
	base.Payload = []byte(`{"provider":"x","model":"y","turn_id":"b"}`)
	if got, err := store.AppendRawEvent(ctx, base); err != nil || !got.Accepted { t.Fatal(got, err) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/storage/sqlite ./internal/ingest/jsonl -run "TestAppendRawEventSuppressesReplayWithoutChangingLedger|TestDistinctEventsWithSharedSessionAndTimeAreAccepted"`

Expected: FAIL because duplicate append creates two raw events and `RawEventAppendResult` does not exist.

- [ ] **Step 3: Add migration and canonical identity implementation**

Create migration:

```sql
CREATE TABLE raw_event_dedup (
    ingestion_identity TEXT PRIMARY KEY,
    raw_event_id TEXT NOT NULL REFERENCES raw_events(id),
    source TEXT NOT NULL,
    first_received_at TEXT NOT NULL,
    last_received_at TEXT NOT NULL,
    suppression_count INTEGER NOT NULL DEFAULT 0 CHECK (suppression_count >= 0)
);
CREATE INDEX idx_raw_event_dedup_raw_event_id ON raw_event_dedup(raw_event_id);
```

Compute identity after `sanitizePayload` and `sanitizeEvidence`. Prefer a non-empty `RawEventInput.IngestionIdentity` supplied from documented upstream event/span/snapshot ID. Otherwise SHA-256 a canonical JSON object containing only source, session ID, event type, UTC occurrence timestamp, project/location/work-context IDs, resolution method/confidence, sanitized evidence, and sanitized payload. Never include receive time or forbidden content.

Within one transaction, attempt insert into `raw_event_dedup` before creating chain event. On uniqueness conflict, update only `last_received_at` and `suppression_count`, return original raw event ID with `Accepted:false`, and do not append raw event or normalize model call. On acceptance, append raw event, then insert dedup record referencing it. Preserve ledger chain unchanged for first accepted event.

- [ ] **Step 4: Make normalization conditional on accepted raw event**

```go
appendResult, err := store.AppendRawEvent(ctx, rawInput)
if err != nil { return count, err }
if !appendResult.Accepted {
	duplicates++
	continue
}
if err := normalizeModelCall(ctx, store, parsed, appendResult.ID); err != nil { return count, err }
accepted++
```

Add `RawEventID string` to `ModelCallInput`, persist it with unique constraint in `model_calls`, and normalize against the accepted raw event only. This durable link prevents duplicate model calls even if process crashes between raw append and later replay.

- [ ] **Step 5: Add crash/replay and concurrent-ingest tests**

```go
func TestConcurrentReplayCreatesOneRawEventAndOneModelCall(t *testing.T) {
	// Start two goroutines importing identical one-line model.call input.
	// Assert accepted total is 1, duplicate total is 1, raw_events is 1,
	// model_calls is 1, token sum equals one payload, and VerifyLedger passes.
}
```

Run: `go test -count=1 ./internal/storage/sqlite ./internal/ingest/jsonl ./internal/ingest/qlogevent ./internal/ingest/otlp -run "TestAppendRawEventSuppressesReplayWithoutChangingLedger|TestDistinctEventsWithSharedSessionAndTimeAreAccepted|TestConcurrentReplayCreatesOneRawEventAndOneModelCall"`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/sqlite/migrations/006_ingestion_identity.sql internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/ingest/jsonl internal/ingest/qlogevent internal/ingest/otlp
git commit -m "feat: deduplicate replayed capture events"
```

### Task 6: Make Adapter Verification Fail Closed With Process Exit Semantics

**Files:**
- Modify: `internal/cli/adapters.go:18-29,157-283`
- Modify: `internal/cli/capture_commands_test.go:42-227`
- Modify: `internal/storage/sqlite/store.go:1315-1358`
- Modify: `internal/storage/sqlite/store_test.go`

**Interfaces:**
- Extends `adapterVerifyStage` with `Required bool`.
- Adds `AdapterEvidenceQuery{AdapterID, Source string, From, To time.Time, ProjectSlug string, RequiredQuality string}`.
- Adds `Store.HasRecentAdapterEvidence(ctx context.Context, query AdapterEvidenceQuery) (bool, error)`.
- `adapter verify <adapter>` returns a non-zero Cobra error when any required stage fails, while still writing JSON result to stdout when `--json` is requested.

- [ ] **Step 1: Write failing verification tests**

```go
func TestAdapterVerifyReturnsNonZeroForMissingRequiredEvidence(t *testing.T) {
	home := t.TempDir()
	_, err := runQLog(t, home, "init")
	if err != nil { t.Fatal(err) }
	output, err := runQLog(t, home, "adapter", "verify", "opencode", "--json")
	if err == nil { t.Fatalf("verify succeeded: %s", output) }
	if !strings.Contains(output, `"ready":false`) { t.Fatalf("output = %s", output) }
}

func TestAdapterVerifyRequiresRawAndNormalizedLinkage(t *testing.T) {
	// Seed a matching raw event without its model call, then a model call with another raw_event_id.
	// Assert evidence stage fails.
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/cli ./internal/storage/sqlite -run "TestAdapterVerifyReturnsNonZeroForMissingRequiredEvidence|TestAdapterVerifyRequiresRawAndNormalizedLinkage"`

Expected: FAIL because current command returns nil when `Ready` is false and only Copilot performs evidence lookup.

- [ ] **Step 3: Implement common evidence stages**

For each stable adapter, require and report: setup ownership, adapter availability, collector reachability, fresh raw event from expected source, expected capture quality, and for reported-token adapters, exactly one linked normalized model call with non-zero source-reported tokens. Claude Code requires real lifecycle raw evidence but no model call/tokens. Codex/OpenCode/Copilot reported-token requirements activate only after their Task 1 evidence gate passes; before then verification fails with explicit `source_evidence` stage.

Implement `HasRecentAdapterEvidence` with SQL join from `raw_events` to `model_calls` through `model_calls.raw_event_id`; filter raw source, adapter name from sanitized payload, time window, optional project allocation, and required quality. It must not use arbitrary JSONL source, adapter name alone, setup configuration, or synthetic process health as evidence.

- [ ] **Step 4: Return execution error after writing result**

```go
if !result.Ready {
	if verifyJSON { _ = writeJSON(command.Root().OutOrStdout(), result) }
	return fmt.Errorf("adapter %s is not verified", result.AdapterID)
}
```

Refactor output so JSON writes exactly once; use a sentinel typed error if tests need to identify failed verification without matching error text.

- [ ] **Step 5: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/cli ./internal/storage/sqlite -run "TestAdapterVerify"`

Expected: PASS, including existing rejection of generic/spoofed Copilot data and new non-zero result on every missing mandatory stage.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/adapters.go internal/cli/capture_commands_test.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go
git commit -m "fix: fail closed adapter verification"
```

### Task 7: Consolidate Reporting, Session Snapshots, And Confidence Semantics

**Files:**
- Modify: `internal/storage/sqlite/store.go:108-128,163-177,779-825,1258-1313`
- Modify: `internal/storage/sqlite/reporting_test.go`
- Modify: `internal/cli/root.go:400-444,740-808`
- Modify: `internal/cli/root_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`

**Interfaces:**
- Adds `MeasurementSummary{Quality string; ModelCallCount, InputTokens, OutputTokens, ReasoningTokens, CachedInputTokens, CacheWriteTokens, TotalTokens, EstimatedCostUSDMicros int64}`.
- Extends `TaskSummary`, `ProjectReport`, and `UsageReport` with `Measurements []MeasurementSummary`; preserve existing totals only as aggregate values and label costs `estimated_cost_*`.
- Adds `SessionSnapshot{SessionID, AgentName string; StartedAt, LastEventAt time.Time; RawEventCount, ModelCallCount int64; Measurements []MeasurementSummary; ResolutionMethod, ResolutionConfidence string}`.
- Adds `Store.SessionSnapshot(ctx context.Context, sessionID string) (SessionSnapshot, error)`.

- [ ] **Step 1: Write failing reporting tests**

```go
func TestUsageSeparatesReportedLifecycleAndEstimatedMeasurements(t *testing.T) {
	// Insert one agent_reported call, one estimated call, and one Claude lifecycle raw event.
	report, err := store.Usage(ctx, UsageQuery{GroupBy: []string{"project", "agent", "provider", "model", "capture_quality"}})
	if err != nil { t.Fatal(err) }
	if got := measurement(report.Measurements, "agent_reported").TotalTokens; got != 12 { t.Fatalf("reported tokens = %d", got) }
	if got := measurement(report.Measurements, "estimated").TotalTokens; got != 9 { t.Fatalf("estimated tokens = %d", got) }
	if measurement(report.Measurements, "lifecycle_only").TotalTokens != 0 { t.Fatal("lifecycle-only fabricated tokens") }
}

func TestSessionSnapshotPreservesResolutionConfidence(t *testing.T) {
	snapshot, err := store.SessionSnapshot(ctx, "session-1")
	if err != nil { t.Fatal(err) }
	if snapshot.ResolutionConfidence != "exact" { t.Fatalf("confidence = %q", snapshot.ResolutionConfidence) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -count=1 ./internal/storage/sqlite ./internal/cli ./internal/mcpserver -run "TestUsageSeparatesReportedLifecycleAndEstimatedMeasurements|TestSessionSnapshotPreservesResolutionConfidence"`

Expected: FAIL because reports expose only grouped rows and no session snapshot API.

- [ ] **Step 3: Implement quality-preserving aggregation**

Create measurement summaries by `capture_quality`; no query may merge rows across quality. Include `lifecycle_only` in snapshots when raw evidence exists even though it has zero model calls/tokens. Keep `unavailable` as adapter status, not fabricated usage. Preserve direct/unattributed allocation behavior and do not promote unknown project to a provider/model-derived project.

Extend CLI JSON output for `report usage`, `usage project`, and `task summary`; add `qlog session summary <session-id> --json` backed by `SessionSnapshot`. Update MCP task/project result structures only to expose recorded quality-separated data, not inferred metrics.

- [ ] **Step 4: Add CLI assertions for labels**

```go
func TestUsageJSONLabelsEstimatedCostsAndCaptureQuality(t *testing.T) {
	output, err := runQLog(t, home, "report", "usage", "--json")
	if err != nil { t.Fatal(err) }
	for _, want := range []string{`"capture_quality":"agent_reported"`, `"estimated_cost_usd_micros"`, `"measurements"`} {
		if !strings.Contains(output, want) { t.Fatalf("output missing %s: %s", want, output) }
	}
}
```

- [ ] **Step 5: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/storage/sqlite ./internal/cli ./internal/mcpserver -run "TestUsageSeparatesReportedLifecycleAndEstimatedMeasurements|TestSessionSnapshotPreservesResolutionConfidence|TestUsageJSONLabelsEstimatedCostsAndCaptureQuality"`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/sqlite/store.go internal/storage/sqlite/reporting_test.go internal/cli/root.go internal/cli/root_test.go internal/mcpserver/server.go internal/mcpserver/server_test.go
git commit -m "feat: expose capture quality in session reports"
```

### Task 8: Document Support Boundaries And Execute Real-Device Release Acceptance

**Files:**
- Modify: `README.md`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Modify: `docs-int/releases/distribution.md`
- Modify: `docs-int/verification/m4-evidence.md`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Documents exact stable support matrix: adapter, target OS, documented source version, setup mechanism, collector endpoint, available quality, and release evidence status.
- Defines evidence record fields: device OS/version/architecture, qlog version/hash, agent version, official source URL, consent transcript, collector status JSON, sanitized raw-event ID, model-call ID when applicable, `adapter verify --json` output/exit code, report output, replay result, and privacy inspection result.

- [ ] **Step 1: Write failing documentation-contract test**

```go
func TestM4EvidenceListsOnlyStableAdaptersAndNoCompletionClaim(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "docs-int", "verification", "m4-evidence.md"))
	if err != nil { t.Fatal(err) }
	for _, want := range []string{"codex", "claude-code", "copilot-vscode", "opencode", "IN_PROGRESS", "real-agent"} {
		if !strings.Contains(string(contents), want) { t.Fatalf("evidence missing %q", want) }
	}
	for _, forbidden := range []string{"Pi", "OpenClaw", "Hermes", "M4 is VERIFIED"} {
		if strings.Contains(string(contents), forbidden) { t.Fatalf("evidence contains %q", forbidden) }
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test -count=1 ./internal/distribution -run TestM4EvidenceListsOnlyStableAdaptersAndNoCompletionClaim`

Expected: FAIL until evidence document contains full four-adapter matrix and explicit real-agent requirement.

- [ ] **Step 3: Write operational documentation**

Document `qlog setup --yes`, `qlog collector install/start/status/logs/uninstall`, loopback behavior, backup/recovery behavior, adapter quality terms, `adapter verify` non-zero behavior, and dedup responses. State that no unavailable token counters are auto-captured. State Codex and Copilot CLI status from source-evidence gate, not optimistic capability descriptors.

- [ ] **Step 4: Execute clean-device acceptance per supported adapter**

For each matrix row whose source-evidence gate is complete, perform this sequence on every supported OS/device combination:

```text
qlog init
qlog project register --path <clean-project> --name <name>
<consented supported bootstrap path>
qlog collector status --json
<one real agent action>
qlog adapter verify <adapter> --project <slug> --json
qlog usage project <slug> --json
<repeat same upstream event or documented snapshot replay>
qlog adapter verify <adapter> --project <slug> --json
qlog verify
qlog doctor --json
```

Expected: collector is installed/running/reachable on loopback; first real event produces one sanitized raw event; reported-token adapters produce one linked model call and one quality-labelled report row; Claude Code produces lifecycle evidence with zero tokens; replay produces duplicate suppression and no raw/model/token/cost increase; `adapter verify` exits zero only when all required stages pass; privacy inspection finds no forbidden field in raw payload, evidence, hashes, exports, or logs.

If any adapter lacks source evidence or clean-device E2E evidence, leave its row `IN_PROGRESS` and leave M4 unreleased. Do not convert synthetic tests into real-device evidence.

- [ ] **Step 5: Run final automated validation**

Run: `go test -count=1 ./...`

Expected: PASS.

Run: `go vet ./...`

Expected: PASS with no output.

Run: `git diff --check`

Expected: PASS with no output.

- [ ] **Step 6: Commit documentation and evidence only after factual validation**

```bash
git add README.md docs/DEVELOPER_GUIDE.md docs-int/releases/distribution.md docs-int/verification/m4-evidence.md internal/distribution/installers_test.go
git commit -m "docs: record M4 capture acceptance evidence"
```

## Plan Self-Review

- Spec coverage: Task 1 covers exactly four truthful adapters, including OpenCode `lifecycle_only` evidence until source-backed usage fields are documented; Task 2 covers consented automatic bootstrap in official qlog installers, explicit opt-out, idempotence, and manual `qlog setup` fallback; Task 3 covers persistent loopback collector across target OSes; Task 4 covers collector-first behavior and Codex/Copilot source-evidence blockers; Task 5 covers durable deduplication; Task 6 covers closed verification semantics; Task 7 covers reporting/session snapshot confidence; Task 8 covers docs and real-device acceptance.
- Completeness scan: every implementation step names files, symbols, commands, expected outcomes, and evidence gates. Official qlog installation paths automatically bootstrap only after installer-level consent and provide an explicit opt-out; third-party agent installer hooks are neither claimed nor release blockers. Agent-specific telemetry integrations remain blocked until their documented source evidence is complete.
- Type consistency: raw-event ingestion identity flows from receiver to `RawEventInput`, append result controls normalization, model call links back to raw event, and verification queries that durable linkage.
