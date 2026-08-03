# M4 Review Corrections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply seven validated M4 review corrections: strict Codex proof, TOML-safe Codex ownership, consistent collector status, and durable platform lifecycle behavior.

**Architecture:** Preserve existing boundaries. Codex accepted-log normalization writes a narrow payload discriminator; SQLite evidence filtering consumes that persisted field without adding schema. Codex configuration becomes structured only within `[otel]`, retaining byte-identical text outside it and ownership state only for settings qlog actually changes. Collector CLI resolves final managed settings before deriving paths, while each platform manager keeps lifecycle semantics behind its existing manager boundary.

**Tech Stack:** Go 1.26.0, Cobra v1.10.2, modernc.org/sqlite v1.38.2, OpenTelemetry OTLP protobuf v1.10.0, per-platform systemd/Task Scheduler/launchctl integration.

## Global Constraints

- Implement exactly seven corrections in `docs/superpowers/specs/2026-08-02-m4-review-corrections-design.md`; do not add adapters, endpoints, telemetry schemas, migrations, reporting dimensions, CI changes, or release-status changes.
- Preserve privacy filtering, loopback policy, project resolution, and token-accounting behavior; do not persist prompt, response, tool, secret, or authorization data.
- Codex verification must fail closed: only accepted `service.name=codex`, `event.name=codex.sse_event`, `event.kind=response.completed` log normalization may write `codex_response_completed=true`.
- Never adopt matching preexisting Codex settings: no ownership state, `Installed=false`, and uninstall must leave matching user configuration unchanged.
- Preserve text outside `[otel]` byte-for-byte; canonicalization is allowed only inside `[otel]`.
- Explicit `--home` overrides persisted collector state; status `home`, `database`, state path, and log path must describe one installation.
- Linux uninstall remains fail-fast before collector state/log cleanup. Windows restart policy must be finite. Darwin may ignore only an absent loaded job during replacement.
- Tests use `t.TempDir()`, table-driven cases where variants exist, and direct behavior assertions. Synthetic tests are regression evidence, not clean-device real-agent acceptance.
- Run `gofmt` on modified Go files, then focused tests, `go test -count=1 ./...`, and `go vet ./...`. Run Linux, Windows, and Darwin platform tests in their respective CI jobs.
- Do not change product code, tests, README, configuration, PR threads, CI, or commit while creating this plan. Plan execution makes one Conventional Commit per task and never adds AI attribution.

---

## File Structure

- Modify: `internal/ingest/otlp/receiver.go` - add `codex_response_completed` only to accepted Codex `response.completed` normalized payloads.
- Modify: `internal/ingest/otlp/receiver_test.go` - prove discriminator persistence and refusal for non-Codex/malformed evidence.
- Modify: `internal/storage/sqlite/store.go` - add narrow `RequireCodexResponseCompleted` query option and JSON predicate.
- Modify: `internal/storage/sqlite/store_test.go` - prove accepted Codex evidence requires discriminator plus linked reported-token model call.
- Modify: `internal/cli/adapters.go` - request discriminator only for Codex evidence contract in both status and verification paths.
- Modify: `internal/cli/capture_commands_test.go` - verify Codex contract and command behavior rejects generic `agent_name=codex` OTLP evidence and accepts normalized evidence.
- Modify: `go.mod` and `go.sum` - add one TOML parser dependency used only by Codex configuration mutation.
- Modify: `internal/adapters/codex.go` - parse/rewrite `[otel]` exporter representations safely and retain ownership-safe original values.
- Modify: `internal/adapters/adapters_test.go` - cover inline, dotted, nested, idempotent, restoration, and unclaimed matching Codex configuration.
- Modify: `internal/cli/collector.go` and `internal/cli/collector_status_test.go` - resolve final managed home before calculating status database.
- Modify: `internal/cli/collector_linux.go` and `internal/cli/collector_linux_test.go` - remove systemd unit before daemon reload with injectable command/file seams.
- Modify: `README.md` - describe Codex `otel_reported` support and its remaining clean-device evidence gate.
- Modify: `internal/cli/collector_windows.go` and `internal/cli/collector_windows_test.go` - add finite Task Scheduler restart settings to XML.
- Modify: `internal/cli/collector_darwin.go` and `internal/cli/collector_darwin_test.go` - boot out a loaded LaunchAgent before bootstrap of rewritten plist.

### Task 1: Require Persisted Codex Completion Evidence

**Files:**
- Modify: `internal/ingest/otlp/receiver.go:368-416`
- Modify: `internal/ingest/otlp/receiver_test.go:88-177,230-260`
- Modify: `internal/storage/sqlite/store.go:111-119,1718-1770`
- Modify: `internal/storage/sqlite/store_test.go:499-609`
- Modify: `internal/cli/adapters.go:242-260,275-295,307-345`
- Modify: `internal/cli/capture_commands_test.go:372-377` and add focused Codex verification cases

**Interfaces:**
- Consumes: `Receiver.codexLogEvent(ctx, resource, record, input) (map[string]any, bool, error)` and `sqlite.AdapterEvidenceQuery`.
- Produces: `AdapterEvidenceQuery.RequireCodexResponseCompleted bool`; `adapterEvidenceContract.RequireCodexResponseCompleted bool`; accepted Codex raw-event payloads with `codex_response_completed: true`.

- [ ] **Step 1: Write failing OTLP and storage tests**

Add direct assertions to accepted Codex normalization and a table-driven storage test. Keep private-field checks alongside the existing allowlist test.

```go
func TestCodexLogEventMarksOnlyAcceptedResponseCompletedEvidence(t *testing.T) {
	service, err := app.Initialize(context.Background(), t.TempDir())
	if err != nil { t.Fatalf("initialize service: %v", err) }
	t.Cleanup(func() { _ = service.Close() })

	line, accepted, err := (Receiver{service: service}).codexLogEvent(context.Background(),
		map[string]string{"service.name": "codex"},
		map[string]string{"event.name": "codex.sse_event", "event.kind": "response.completed", "model": "gpt-5", "input_tokens": "1", "output_tokens": "2"},
		logRecord{TraceID: "trace", SpanID: "span"},
	)
	if err != nil || !accepted { t.Fatalf("accepted=%t err=%v", accepted, err) }
	payload := line["payload"].(map[string]any)
	if payload["codex_response_completed"] != true { t.Fatalf("payload = %#v", payload) }
}

func TestAdapterEvidenceRequiresCodexResponseCompleted(t *testing.T) {
	// Append two otherwise identical otlp-http Codex raw events and linked calls.
	// Only payload containing codex_response_completed:true may satisfy query.
	// Use t.TempDir(), now +/- time.Minute, project allocation, and otel_reported tokens.
}
```

Add command-level cases using the existing `newCollectorMux(home)` pattern: one generic OTLP payload that writes `agent_name=codex` without the discriminator must make `adapter verify codex --project project --json` return non-zero; one accepted `/v1/logs` Codex payload must make it succeed once settings and collector reachability are satisfied.

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test -count=1 ./internal/ingest/otlp ./internal/storage/sqlite ./internal/cli -run 'TestCodexLogEventMarksOnlyAcceptedResponseCompletedEvidence|TestAdapterEvidenceRequiresCodexResponseCompleted|TestAdapterVerifyCodex'`

Expected: FAIL because normalized payload lacks `codex_response_completed` and evidence query/contract have no discriminator field.

- [ ] **Step 3: Add narrow discriminator interfaces and implementation**

Extend only the evidence query/contract and accepted normalized payload. Do not add a migration; raw-event sanitized JSON already persists normalized payload fields.

```go
// internal/storage/sqlite/store.go
type AdapterEvidenceQuery struct {
	AdapterID                     string
	AllowedAgentNames             []string
	Source                        string
	From                          time.Time
	To                            time.Time
	ProjectSlug                   string
	RequiredQuality               string
	RequireCodexResponseCompleted bool
}

// Inside HasRecentAdapterEvidence, after capture-quality predicate.
if query.RequireCodexResponseCompleted {
	where += ` AND json_extract(r.payload_json_sanitized, '$.codex_response_completed') = 1`
}
```

```go
// internal/ingest/otlp/receiver.go, accepted payload only.
payload := map[string]any{
	"provider":                 "openai",
	"model":                    model,
	"agent_name":               "codex",
	"capture_quality":          "otel_reported",
	"codex_response_completed": true,
	"input_tokens":             inputTokens,
	"output_tokens":            outputTokens,
}
```

```go
// internal/cli/adapters.go
type adapterEvidenceContract struct {
	Source                        string
	Quality                       adapters.CaptureQuality
	AllowedAgentNames             []string
	RequireCodexResponseCompleted bool
	SourceEvidence                bool
	SourceEvidenceMessage         string
}

case "codex":
	return adapterEvidenceContract{
		Source: "otlp-http", Quality: adapters.CaptureOTELReported,
		RequireCodexResponseCompleted: true, SourceEvidence: true,
		SourceEvidenceMessage: "Codex 0.145.0 documents OTLP response.completed logs with source-reported tokens",
	}
```

Pass `RequireCodexResponseCompleted: contract.RequireCodexResponseCompleted` in both `localAdapterStatusAccess.HasRecentEvidence` and `verifyAdapter` query literals.

- [ ] **Step 4: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/ingest/otlp ./internal/storage/sqlite ./internal/cli -run 'TestReceiverImportsCodexResponseCompletedLogsAndDeduplicatesReplay|TestReceiverRejectsGenericOrSpoofedLogs|TestCodexLogEventMarksOnlyAcceptedResponseCompletedEvidence|TestAdapterEvidenceRequiresCodexResponseCompleted|TestAdapterVerifyCodex|TestCodexEvidenceContractUsesDocumentedOTLPLogs'`

Expected: PASS; generic or agent-name-only OTLP evidence cannot verify Codex, accepted normalized log evidence can.

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/otlp/receiver.go internal/ingest/otlp/receiver_test.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/adapters.go internal/cli/capture_commands_test.go
git commit -m "fix(codex): require persisted completion evidence"
```

### Task 2: Rewrite Codex Exporter TOML Safely

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/adapters/codex.go:15-22,101-105,108-282`
- Modify: `internal/adapters/adapters_test.go:244-290`

**Interfaces:**
- Consumes: `applyCodexOTelConfig(configPath, statePath string, dryRun bool) (SetupChange, error)` and `removeCodexOTelConfig(configPath, statePath string, dryRun bool) (SetupChange, error)`.
- Produces: `updateCodexOTel(contents string, desired map[string]string) (updated string, original codexManagedState, changed bool, err error)` and `restoreCodexOTel(contents string, state codexManagedState, desired map[string]string) (string, bool, error)`.

- [ ] **Step 1: Write failing table-driven adapter tests**

Replace the single Codex config test with table cases for `inline exporter`, `dotted exporter keys`, `nested exporter table`, and `already desired user-owned exporter`. Each case writes a config with unrelated root and `[otel]` fields, then asserts parseability, exactly one logical exporter, unchanged non-`[otel]` prefix/suffix, ownership state behavior, idempotency, and uninstall result.

```go
func TestCodexInstallReplacesEquivalentExporterForms(t *testing.T) {
	tests := []struct {
		name          string
		before        string
		wantChanged   bool
		wantStateFile bool
		wantAfter     string
	}{
		{"inline", "model = \"gpt-5\"\n\n[otel]\nexporter = \"none\"\n", true, true, "exporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"binary\" } }"},
		{"dotted", "model = \"gpt-5\"\n\n[otel]\nexporter.otlp-http.endpoint = \"http://old\"\n", true, true, "exporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"binary\" } }"},
		{"nested", "model = \"gpt-5\"\n\n[otel.exporter.otlp-http]\nendpoint = \"http://old\"\n", true, true, "exporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"binary\" } }"},
		{"matching user-owned", "model = \"gpt-5\"\n\n[otel]\nexporter = { otlp-http = { endpoint = \"http://127.0.0.1:4318/v1/logs\", protocol = \"binary\" } }\nlog_user_prompt = false\n", false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Install twice; decode resulting TOML; then uninstall.
			// Assert matching user-owned input remains byte-identical after uninstall.
		})
	}
}
```

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test -count=1 ./internal/adapters -run 'TestCodexInstall(ReplacesEquivalentExporterForms|PreservesConfigAndUninstallRestoresOnlyQlogSettings)'`

Expected: FAIL because current line-oriented `tomlKey` does not remove dotted/nested child declarations and treats matching user-owned values as installed ownership.

- [ ] **Step 3: Add parser-backed `[otel]` mutation and exact ownership state**

Add `github.com/pelletier/go-toml/v2` as the single direct TOML dependency. Parse the complete document to validate it, then isolate the `[otel]` logical region (including `[otel.exporter...]` descendants), remove every exporter representation, and write the canonical qlog exporter plus `log_user_prompt = false`. Keep all bytes before and after that logical region unchanged.

```go
type codexManagedState struct {
	OriginalExporter codexManagedValue `json:"original_exporter"`
	OriginalLogPrompt codexManagedValue `json:"original_log_user_prompt"`
}

type codexManagedValue struct {
	Exists bool   `json:"exists"`
	Value  string `json:"value,omitempty"` // Exact exporter node(s) or log key text.
}

type codexOTelRegion struct {
	start int
	end   int
	text  string
}

type parsedCodexOTel struct {
	exporter      codexManagedValue
	logUserPrompt codexManagedValue
	otherLines    []string
}

func locateOTelRegion(contents string) codexOTelRegion
func parseOTelRegion(contents string) (parsedCodexOTel, error)
func exporterMatches(value codexManagedValue, want string) bool
func renderOTelRegion(otherLines []string, desired map[string]string) string
func replaceOTelRegion(contents string, region codexOTelRegion, replacement string) string

func updateCodexOTel(contents string, desired map[string]string) (string, codexManagedState, bool, error) {
	var document map[string]any
	if err := toml.Unmarshal([]byte(contents), &document); err != nil {
		return "", codexManagedState{}, false, fmt.Errorf("parse Codex TOML: %w", err)
	}
	region := locateOTelRegion(contents)
	current, err := parseOTelRegion(region.text)
	if err != nil { return "", codexManagedState{}, false, err }
	if exporterMatches(current, desired["exporter"]) && current.logUserPrompt == desired["log_user_prompt"] {
		return contents, codexManagedState{}, false, nil
	}
	state := codexManagedState{OriginalExporter: current.exporter, OriginalLogPrompt: current.logUserPrompt}
	return replaceOTelRegion(contents, region, renderOTelRegion(current.otherLines, desired)), state, true, nil
}
```

Persist `codexManagedState` only when `changed` is true. On uninstall, restore only state-recorded exporter and `log_user_prompt` values, remove qlog's canonical declarations when originals did not exist, then delete state. Keep `Status` ownership-based: `installed` requires both desired config and qlog state. Remove `tomlSectionRange` and `tomlKey` after their callers are gone.

- [ ] **Step 4: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/adapters -run 'TestCodexInstall(ReplacesEquivalentExporterForms|PreservesConfigAndUninstallRestoresOnlyQlogSettings)'`

Expected: PASS; all three nonmatching exporter forms become one parseable canonical exporter, restore exactly their prior representation/value, and matching user-owned config creates no state or ownership.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/adapters/codex.go internal/adapters/adapters_test.go
git commit -m "fix(codex): rewrite exporter TOML safely"
```

### Task 3: Resolve Collector Status From Final Managed Home

**Files:**
- Modify: `internal/cli/collector.go:31-51`
- Modify: `internal/cli/collector_status_test.go:13-55`

**Interfaces:**
- Consumes: `config.Resolve(home string) (config.Paths, error)` and `resolveManagedCollectorSettings(manager, home, listen, homeExplicit, listenExplicit) (string, string)`.
- Produces: status JSON where `CollectorStatus.Home` and `CollectorStatus.Database` are both resolved from final managed home unless `--home` is explicit.

- [ ] **Step 1: Write failing status command tests**

Introduce a fake manager implementing both `collectorManager` and `managedCollectorSettingsResolver`; use a temporary managed home distinct from CLI default. Assert output paths derive from managed home and explicit home wins.

```go
func TestCollectorStatusResolvesDatabaseFromManagedHome(t *testing.T) {
	cliHome := t.TempDir()
	managedHome := t.TempDir()
	manager := managedStatusCollectorManager{home: managedHome, listen: "127.0.0.1:4319"}
	status, err := collectorStatus(context.Background(), cliHome, defaultCollectorListen, false, false, manager)
	if err != nil { t.Fatalf("collectorStatus() error = %v", err) }
	if status.Home != managedHome || status.Database != filepath.Join(managedHome, "qlog.db") {
		t.Fatalf("status = %#v", status)
	}
}

func TestCollectorStatusExplicitHomeOverridesManagedHome(t *testing.T) {
	// Pass homeExplicit=true and assert home/database use explicit temporary home.
}
```

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test -count=1 ./internal/cli -run 'TestCollectorStatus(ResolvesDatabaseFromManagedHome|ExplicitHomeOverridesManagedHome)'`

Expected: FAIL because command resolves `paths.Database` before `ResolveManagedCollectorSettings` changes home.

- [ ] **Step 3: Extract status orchestration and resolve config twice**

Move command body into a testable helper. Resolve initial CLI paths only to obtain initial home, apply managed settings, then resolve paths again with final home before assigning database.

```go
func collectorStatus(ctx context.Context, home, listen string, homeExplicit, listenExplicit bool, manager collectorManager) (CollectorStatus, error) {
	paths, err := config.Resolve(home)
	if err != nil { return CollectorStatus{}, err }
	resolvedHome, resolvedListen := resolveManagedCollectorSettings(manager, paths.Home, listen, homeExplicit, listenExplicit)
	finalPaths, err := config.Resolve(resolvedHome)
	if err != nil { return CollectorStatus{}, err }
	output, err := manager.Status(ctx, resolvedListen)
	if err != nil { return CollectorStatus{}, err }
	output.Home = finalPaths.Home
	output.Database = finalPaths.Database
	output.Endpoints = []string{"/v1/traces", "/v1/logs", "/v1/events", "/healthz"}
	output.Scope = "loopback-only by default"
	output.Health = output.Message
	return output, nil
}
```

Have `newCollectorCommand` call this helper with `command.Flags().Changed("home")` and `command.Flags().Changed("listen")`.

- [ ] **Step 4: Run focused tests to verify pass**

Run: `go test -count=1 ./internal/cli -run 'TestCollectorStatus(ReportsPersistentLifecycleFields|ResolvesDatabaseFromManagedHome|ExplicitHomeOverridesManagedHome|ShowsLocalEndpoints)'`

Expected: PASS; omitted home reports persisted managed home/database; explicit home remains authoritative.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/collector.go internal/cli/collector_status_test.go
git commit -m "fix(collector): resolve status from managed home"
```

### Task 4: Remove Linux Unit Before Reloading systemd

**Files:**
- Modify: `internal/cli/collector_linux.go:129-150`
- Modify: `internal/cli/collector_linux_test.go:12-67`

**Interfaces:**
- Consumes: `linuxCollectorManager.Uninstall() (CollectorStatus, error)`.
- Produces: ordered lifecycle calls: `stop`, `disable`, `remove unit`, `daemon-reload`, then collector logs/state cleanup.

- [ ] **Step 1: Write failing ordering and fail-fast tests**

Add package-level test seams for command execution and file removal; restore them with `t.Cleanup`. Record call order and assert state/log cleanup is absent when `daemon-reload` fails.

```go
func TestLinuxCollectorUninstallRemovesUnitBeforeDaemonReload(t *testing.T) {
	var calls []string
	// Stub stop, systemctl, remove, state read, and RemoveAll to append identifiers.
	if _, err := (linuxCollectorManager{}).Uninstall(); err != nil { t.Fatal(err) }
	want := []string{"stop", "disable", "remove-unit", "daemon-reload", "remove-logs", "remove-state"}
	if !slices.Equal(calls, want) { t.Fatalf("uninstall calls = %q, want %q", calls, want) }
}

func TestLinuxCollectorUninstallKeepsStateWhenReloadFails(t *testing.T) {
	// Make daemon-reload return errors.New("reload failed"); assert no RemoveAll/state remove call.
}
```

Use standard-library slice comparison instead of adding `cmp` if the repository does not already depend on it.

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test -count=1 ./internal/cli -run 'TestLinuxCollectorUninstall(RemovesUnitBeforeDaemonReload|KeepsStateWhenReloadFails)'`

Expected: FAIL because current implementation runs `daemon-reload` before `os.Remove(linuxCollectorUnitPath())`.

- [ ] **Step 3: Introduce local lifecycle seams and correct order**

Keep production behavior unchanged by default. Use tiny package variables only for `systemctl`, unit removal, and collector-directory cleanup required to assert sequence.

```go
var runLinuxSystemctl = func(args ...string) error {
	return exec.Command("systemctl", args...).Run()
}
var removeLinuxCollectorUnit = os.Remove
var removeLinuxCollectorTree = os.RemoveAll

func (manager linuxCollectorManager) Uninstall() (CollectorStatus, error) {
	if _, err := manager.Stop(); err != nil { return CollectorStatus{}, err }
	if err := runLinuxSystemctl("--user", "disable", linuxCollectorUnitName); err != nil { return CollectorStatus{}, err }
	if err := removeLinuxCollectorUnit(linuxCollectorUnitPath()); err != nil && !os.IsNotExist(err) { return CollectorStatus{}, err }
	if err := runLinuxSystemctl("--user", "daemon-reload"); err != nil { return CollectorStatus{}, err }
	home := readLinuxCollectorState(linuxCollectorStatePath()).Home
	if home != "" {
		if err := removeLinuxCollectorTree(filepath.Join(home, "collector")); err != nil { return CollectorStatus{}, err }
	}
	if err := os.Remove(linuxCollectorStatePath()); err != nil && !os.IsNotExist(err) { return CollectorStatus{}, err }
	return CollectorStatus{ServiceID: linuxCollectorUnitName, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log"), Message: "collector user service uninstalled"}, nil
}
```

- [ ] **Step 4: Run focused test to verify pass**

Run: `go test -count=1 ./internal/cli -run 'TestLinuxCollector(ServiceDefinition|UninstallRemovesUnitBeforeDaemonReload|UninstallKeepsStateWhenReloadFails)'`

Expected: PASS; unit removal precedes reload and reload failure leaves qlog state/logs intact for diagnosis.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/collector_linux.go internal/cli/collector_linux_test.go
git commit -m "fix(collector): remove Linux unit before reload"
```

### Task 5: Align README Codex Support Statement

**Files:**
- Modify: `README.md:33-40`

**Interfaces:**
- Consumes: `codexAdapter.Status` (`CaptureOTELReported`) and `evidenceContract("codex")` strict completion requirement.
- Produces: support matrix text consistent with executable Codex capability and verification gate.

- [ ] **Step 1: Write the documentation acceptance check**

Before editing, record exact required row content in plan execution notes and verify it against code:

```text
| Codex | `otel_reported` | Documented OTLP `response.completed` logs with source-reported tokens are supported; clean-device accepted `response.completed` evidence and normal verification remain required. |
```

- [ ] **Step 2: Verify current wording fails acceptance**

Run: `rg -n '^\| Codex \|' README.md`

Expected: output contains `unavailable`, which conflicts with `codexAdapter.Status` and `evidenceContract("codex")`.

- [ ] **Step 3: Replace only Codex row**

```markdown
| Codex | `otel_reported` | Documented OTLP `response.completed` logs with source-reported tokens are supported; clean-device accepted `response.completed` evidence and normal verification remain required. |
```

Do not alter milestone status, other adapter rows, configuration instructions, or acceptance claims.

- [ ] **Step 4: Verify wording and executable contract align**

Run: `rg -n '^\| Codex \|' README.md && go test -count=1 ./internal/adapters ./internal/cli -run 'TestCodexAndCopilotReportTheirDocumentedQuality|TestCodexEvidenceContractUsesDocumentedOTLPLogs'`

Expected: README says `otel_reported` and retains clean-device/normal-verification gate; both focused tests PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs(readme): align Codex support statement"
```

### Task 6: Add Bounded Windows Task Restart Policy

**Files:**
- Modify: `internal/cli/collector_windows.go:129-133`
- Modify: `internal/cli/collector_windows_test.go:15-48`

**Interfaces:**
- Consumes: `windowsCollectorTaskDefinition(executable, home, listen, userID, logPath string) string`.
- Produces: Task Scheduler XML with finite `<RestartOnFailure>` interval/count plus existing InteractiveToken, LeastPrivilege, logon trigger, and loopback command arguments.

- [ ] **Step 1: Write failing XML contract test**

Add one focused test to existing XML tests; assert explicit finite scheduler values and existing security/lifecycle values.

```go
func TestWindowsCollectorTaskDefinitionBoundsRestartOnFailure(t *testing.T) {
	definition := windowsCollectorTaskDefinition(
		`C:\Program Files\QUANTUM_LOG\qlog.exe`, `C:\Users\alice\AppData\Local\QUANTUM_LOG`,
		"127.0.0.1:4318", `CONTOSO\alice`, `C:\Users\alice\AppData\Local\QUANTUM_LOG\collector\collector.log`,
	)
	for _, want := range []string{
		"<RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure>",
		"<LogonType>InteractiveToken</LogonType>", "<RunLevel>LeastPrivilege</RunLevel>",
		"collector serve --listen 127.0.0.1:4318",
	} {
		if !strings.Contains(definition, want) { t.Fatalf("task definition missing %q: %s", want, definition) }
	}
}
```

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test -count=1 ./internal/cli -run 'TestWindowsCollector(TaskDefinitionBoundsRestartOnFailure|ServiceDefinition|TaskDefinitionUsesInteractiveCurrentUser)'`

Expected: FAIL because `<Settings>` has no `<RestartOnFailure>` policy.

- [ ] **Step 3: Add finite policy within current task settings**

Keep existing task constraints. Add restart policy immediately after `StartWhenAvailable`.

```go
return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task"><Triggers><LogonTrigger><Enabled>true</Enabled></LogonTrigger></Triggers><Principals><Principal id="Author"><UserId>%s</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals><Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><StartWhenAvailable>true</StartWhenAvailable><RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure></Settings><Actions Context="Author"><Exec><Command>%s</Command><Arguments>%s</Arguments></Exec></Actions></Task>`, xmlEscape(userID), xmlEscape(executable), xmlEscape(arguments))
```

- [ ] **Step 4: Run focused test to verify pass**

Run: `go test -count=1 ./internal/cli -run 'TestWindowsCollector(TaskDefinitionBoundsRestartOnFailure|ServiceDefinition|TaskDefinitionUsesInteractiveCurrentUser|WriteWindowsCollectorTaskDefinitionUsesUTF16LE)'`

Expected: PASS; UTF-16 generation still works and XML keeps current-user, least-privilege, logon, loopback behavior with three one-minute failure retries.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/collector_windows.go internal/cli/collector_windows_test.go
git commit -m "fix(collector): bound Windows task restarts"
```

### Task 7: Replace Loaded Darwin LaunchAgent Definitions

**Files:**
- Modify: `internal/cli/collector_darwin.go:56-75`
- Modify: `internal/cli/collector_darwin_test.go:10-35`

**Interfaces:**
- Consumes: `darwinCollectorManager.Start(home, listen string) (CollectorStatus, error)`.
- Produces: loaded replacement order `Install -> launchctl print -> launchctl bootout -> launchctl bootstrap -> launchctl kickstart`; unloaded path omits bootout and continues to bootstrap.

- [ ] **Step 1: Write failing lifecycle tests using a command seam**

Add a package-level launchctl runner seam and use temp paths/state plus a safe executable seam if necessary. Test loaded, unloaded, and unexpected bootout failure separately.

```go
func TestDarwinCollectorStartReplacesLoadedLaunchAgent(t *testing.T) {
	var calls [][]string
	runDarwinLaunchctl = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		switch args[0] { case "print": return nil; case "bootout", "bootstrap", "kickstart": return nil }
		return fmt.Errorf("unexpected launchctl command %q", args)
	}
	// Stub install/executable/path functions to temp-safe values, then call Start.
	// Assert launchctl calls equal print, bootout, bootstrap, kickstart in order.
}

func TestDarwinCollectorStartBootstrapsWhenJobIsNotLoaded(t *testing.T) {
	// print returns an error; assert bootstrap then kickstart, without bootout.
}

func TestDarwinCollectorStartReturnsUnexpectedBootoutFailure(t *testing.T) {
	// loaded print succeeds, bootout returns an error; assert no bootstrap/kickstart.
}
```

- [ ] **Step 2: Run focused test to verify failure**

Run: `go test -count=1 ./internal/cli -run 'TestDarwinCollectorStart(ReplacesLoadedLaunchAgent|BootstrapsWhenJobIsNotLoaded|ReturnsUnexpectedBootoutFailure)'`

Expected: FAIL because current loaded path calls only `kickstart` and does not boot out/bootstrap the rewritten plist.

- [ ] **Step 3: Add explicit lifecycle runner and replacement sequence**

Use one local runner seam. Preserve absence handling only where an unloaded job is detected by `print`; a loaded job's failed `bootout` must return error.

```go
var runDarwinLaunchctl = func(args ...string) error {
	return exec.Command("launchctl", args...).Run()
}

func (manager darwinCollectorManager) Start(home, listen string) (CollectorStatus, error) {
	if _, err := manager.Install(home, listen); err != nil { return CollectorStatus{}, err }
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	if err := runDarwinLaunchctl("print", service); err == nil {
		if err := runDarwinLaunchctl("bootout", service); err != nil { return CollectorStatus{}, err }
	}
	if err := runDarwinLaunchctl("bootstrap", darwinCollectorDomain(), darwinCollectorPlistPath()); err != nil {
		return CollectorStatus{}, err
	}
	if err := runDarwinLaunchctl("kickstart", "-k", service); err != nil { return CollectorStatus{}, err }
	status, err := manager.Status(context.Background(), listen)
	if err != nil { return CollectorStatus{}, err }
	status.Message = "collector LaunchAgent start requested; health=" + status.Message
	return status, nil
}
```

- [ ] **Step 4: Run focused test to verify pass**

Run: `go test -count=1 ./internal/cli -run 'TestDarwinCollector(ServiceDefinition|StartReplacesLoadedLaunchAgent|StartBootstrapsWhenJobIsNotLoaded|StartReturnsUnexpectedBootoutFailure)'`

Expected: PASS; loaded jobs are replaced before bootstrap, absent jobs bootstrap directly, and bootout errors stop replacement.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/collector_darwin.go internal/cli/collector_darwin_test.go
git commit -m "fix(collector): replace loaded Darwin agent"
```

## Final Verification

- [ ] **Step 1: Format all modified Go files**

Run: `gofmt -w internal/ingest/otlp/receiver.go internal/ingest/otlp/receiver_test.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/adapters.go internal/cli/capture_commands_test.go internal/adapters/codex.go internal/adapters/adapters_test.go internal/cli/collector.go internal/cli/collector_status_test.go internal/cli/collector_linux.go internal/cli/collector_linux_test.go internal/cli/collector_windows.go internal/cli/collector_windows_test.go internal/cli/collector_darwin.go internal/cli/collector_darwin_test.go`

Expected: command succeeds; `gofmt -d` later emits no output.

- [ ] **Step 2: Run full regression and static analysis**

Run: `go test -count=1 ./...`

Expected: PASS for host platform packages.

Run: `go vet ./...`

Expected: PASS with no diagnostics.

Run: `gofmt -d internal/ingest/otlp/receiver.go internal/ingest/otlp/receiver_test.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/adapters.go internal/cli/capture_commands_test.go internal/adapters/codex.go internal/adapters/adapters_test.go internal/cli/collector.go internal/cli/collector_status_test.go internal/cli/collector_linux.go internal/cli/collector_linux_test.go internal/cli/collector_windows.go internal/cli/collector_windows_test.go internal/cli/collector_darwin.go internal/cli/collector_darwin_test.go`

Expected: no output and exit code 0.

- [ ] **Step 3: Run platform matrix unit tests in CI**

Run on Linux CI: `go test -count=1 ./internal/cli -run 'TestLinuxCollector'`

Expected: PASS.

Run on Windows CI: `go test -count=1 ./internal/cli -run 'TestWindowsCollector'`

Expected: PASS.

Run on Darwin CI: `go test -count=1 ./internal/cli -run 'TestDarwinCollector'`

Expected: PASS.

- [ ] **Step 4: Inspect scope and commit state**

Run: `git diff --check && git status --short`

Expected: no whitespace errors; only seven correction files/tests, dependency lockfiles, and README appear. No product changes beyond the seven corrections, no PR/CI/release artifacts, and no test can be presented as clean-device real-agent acceptance.

## Plan Self-Review

- Spec coverage: Task 1 covers strict Codex completion proof; Task 2 TOML-safe exporter ownership; Task 3 custom-home status; Task 4 systemd order; Task 5 README accuracy; Task 6 bounded Windows retries; Task 7 Darwin replacement.
- Placeholder scan: no deferred implementation marker or unnamed test work remains.
- Type consistency: `RequireCodexResponseCompleted` is declared in `AdapterEvidenceQuery` and `adapterEvidenceContract`, passed in both status and verification query calls, and read only by `HasRecentAdapterEvidence`.
- Scope: no migration, telemetry endpoint, adapter, report dimension, CI, release, PR-thread, or real-device acceptance work is introduced.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-08-02-m4-review-corrections.md`. Two execution options:

1. **Subagent-Driven (recommended)** - Dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** - Execute tasks in this session using `executing-plans`, batch execution with checkpoints.
