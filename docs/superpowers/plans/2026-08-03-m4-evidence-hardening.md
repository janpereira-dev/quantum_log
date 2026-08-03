# M4 Evidence Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden M4 adapter evidence verification, setup hook executable selection, and OpenCode lifecycle normalization without expanding collection or changing storage schema.

**Architecture:** Add an optional exact provider predicate to the existing sanitized raw-event evidence query and require `github` only for Copilot verification. Reuse the collector's transient-executable validator after setup resolves and cleans its executable path. Make the generated OpenCode plugin emit stable `agent.event` lifecycle records, which the existing JSONL importer preserves without creating model calls.

**Tech Stack:** Go, Cobra, modernc.org/sqlite, Go standard-library testing.

## Global Constraints

- No adapters, telemetry sources, storage migrations, public CLI flags, or model-call inference.
- Copilot verification must require normalized sanitized payload provider `github`; missing or non-`github` values fail closed.
- Provider identifies evidence only; project ownership remains centralized and must not derive from provider, model, or agent.
- Setup must validate the resolved absolute qlog executable with existing transient-binary rules before adapter files change.
- Reject resolved `.test`, `.test.exe`, and `/go-build` executable paths; retain durable installed binaries.
- OpenCode remains `lifecycle_only`; do not capture tokens, models, providers, prompts, responses, tool arguments/results, secrets, credentials, or authorization fields.
- Synthetic regression tests do not establish clean-device or real-agent M4 acceptance.

---

### Task 1: Require Copilot Raw-Event Provider Evidence

**Files:**
- Modify: `internal/storage/sqlite/store.go:111-120` (`AdapterEvidenceQuery`), `internal/storage/sqlite/store.go:1719-1775` (`(*Store).HasRecentAdapterEvidence`)
- Modify: `internal/cli/adapters.go:242-261` (`localAdapterStatusAccess.HasRecentEvidence`), `internal/cli/adapters.go:276-298` (`adapterEvidenceContract`, `evidenceContract`), `internal/cli/adapters.go:309-382` (`verifyAdapter`)
- Modify: `internal/storage/sqlite/store_test.go:554-609` (`TestAdapterEvidenceUsesModelCallAllocationForReportedTokens`)
- Modify: `internal/cli/capture_commands_test.go:216-271` (`TestAdapterVerifyCopilotAcceptsSanctionedOTLPEvidence`)
- Test: `internal/storage/sqlite/store_test.go`
- Test: `internal/cli/capture_commands_test.go`

**Interfaces:**
- Consumes: `sqlite.AdapterEvidenceQuery`, `(*sqlite.Store).HasRecentAdapterEvidence(ctx context.Context, query AdapterEvidenceQuery) (bool, error)`, and sanitized `raw_events.payload_json_sanitized`.
- Produces: `AdapterEvidenceQuery.RequiredProvider string`; `adapterEvidenceContract.RequiredProvider string`; Copilot verification requiring raw payload `provider == github` case-insensitively.

- [ ] **Step 1: Write failing storage tests for raw-payload provider matching**

Extend `TestAdapterEvidenceUsesModelCallAllocationForReportedTokens` into a table-driven test. Keep a linked reported-token `model_calls` row with `Provider: "github"`, but vary only raw-event payload provider and expected evidence result. This proves predicate reads sanitized raw event rather than model-call provider.

```go
for _, test := range []struct {
	name    string
	payload string
	want    bool
}{
	{name: "normalized github provider", payload: `{"agent_name":"GitHub Copilot Chat","provider":"github","capture_quality":"otel_reported"}`, want: true},
	{name: "provider omitted", payload: `{"agent_name":"GitHub Copilot Chat","capture_quality":"otel_reported"}`, want: false},
	{name: "other provider", payload: `{"agent_name":"GitHub Copilot Chat","provider":"openai","capture_quality":"otel_reported"}`, want: false},
} {
	t.Run(test.name, func(t *testing.T) {
		raw, err := store.AppendRawEvent(ctx, RawEventInput{Source: "otlp-http", SessionID: test.name, EventType: "model.call", OccurredAt: now, Payload: []byte(test.payload)})
		if err != nil || !raw.Accepted { t.Fatalf("AppendRawEvent() = %#v, %v", raw, err) }
		if _, err := store.RecordModelCall(ctx, ModelCallInput{RawEventID: raw.ID, ProjectID: project.ID, SessionID: test.name, AgentName: "GitHub Copilot Chat", Provider: "github", ModelID: "gpt-5", InputTokens: 1, CaptureQuality: "otel_reported", OccurredAt: now}); err != nil { t.Fatal(err) }
		found, err := store.HasRecentAdapterEvidence(ctx, AdapterEvidenceQuery{
			AdapterID: "copilot-vscode", AllowedAgentNames: []string{"GitHub Copilot Chat"},
			Source: "otlp-http", From: now.Add(-time.Minute), To: now.Add(time.Minute),
			ProjectSlug: project.Slug, RequiredQuality: "otel_reported", RequiredProvider: "github",
		})
		if err != nil || found != test.want {
			t.Fatalf("HasRecentAdapterEvidence() = %t, %v; want %t", found, err, test.want)
		}
	})
}
```

- [ ] **Step 2: Run storage test to verify it fails**

Run: `go test -count=1 ./internal/storage/sqlite -run TestAdapterEvidenceUsesModelCallAllocationForReportedTokens`

Expected: FAIL because `AdapterEvidenceQuery` has no `RequiredProvider` field and query has no provider predicate.

- [ ] **Step 3: Write failing CLI verification tests for omitted and mismatched Copilot providers**

Extract sanctioned Copilot setup and trace submission into focused test helpers in `internal/cli/capture_commands_test.go`. Retain `TestAdapterVerifyCopilotAcceptsSanctionedOTLPEvidence` as accepted `github` evidence. Add sibling cases with provider attribute omitted and `openai`; each must return verification output containing `"ready":false` and a non-nil command error.

```go
func TestAdapterVerifyCopilotRejectsMissingOrNonGitHubProvider(t *testing.T) {
	for _, test := range []struct {
		name              string
		providerAttribute string
	}{
		{name: "provider omitted", providerAttribute: ""},
		{name: "provider openai", providerAttribute: `,{"key":"gen_ai.provider.name","value":{"stringValue":"openai"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			home, serverURL := setupCopilotVerification(t)
			postCopilotOTLPTrace(t, serverURL, test.providerAttribute)
			output, err := runQLog(t, home, "adapter", "verify", "copilot-vscode", "--project", "project", "--json")
			if err == nil || !strings.Contains(output, `"ready":false`) {
				t.Fatalf("Copilot evidence incorrectly verified: output=%s err=%v", output, err)
			}
		})
	}
}
```

- [ ] **Step 4: Run CLI tests to verify they fail**

Run: `go test -count=1 ./internal/cli -run 'TestAdapterVerifyCopilot(AcceptsSanctionedOTLPEvidence|RejectsMissingOrNonGitHubProvider)'`

Expected: FAIL because omitted and `openai` raw providers currently satisfy verification when linked token usage exists.

- [ ] **Step 5: Add provider contract and sanitized raw-event predicate**

Add optional exact fields, pass them through both status and verification query construction, set only Copilot contract provider to `github`, and append provider SQL only when a required provider is supplied. Normalize both sides with `lower(?)` exactly like existing quality and agent comparisons.

```go
// internal/storage/sqlite/store.go: add this field to AdapterEvidenceQuery.
RequiredProvider string

if strings.TrimSpace(query.RequiredProvider) != "" {
	where += ` AND lower(COALESCE(json_extract(r.payload_json_sanitized, '$.provider'), '')) = lower(?)`
	args = append(args, query.RequiredProvider)
}

// internal/cli/adapters.go: add this field to adapterEvidenceContract.
RequiredProvider string

case "copilot-vscode":
	return adapterEvidenceContract{
		Source: "otlp-http", Quality: adapters.CaptureOTELReported,
		AllowedAgentNames: []string{"GitHub Copilot Chat"}, RequiredProvider: "github",
		SourceEvidence: true, SourceEvidenceMessage: "VS Code documents Copilot OTel trace/span identity and gen_ai usage fields",
	}

// Add RequiredProvider: contract.RequiredProvider to HasRecentEvidence and verifyAdapter query literals.
```

- [ ] **Step 6: Run focused tests to verify they pass**

Run: `go test -count=1 ./internal/storage/sqlite -run TestAdapterEvidenceUsesModelCallAllocationForReportedTokens`

Expected: PASS; `github` raw payload passes and missing/non-`github` raw providers fail despite linked reported-token calls.

Run: `go test -count=1 ./internal/cli -run 'TestAdapterVerifyCopilot(AcceptsSanctionedOTLPEvidence|RejectsMissingOrNonGitHubProvider)'`

Expected: PASS; sanctioned `github` verifies, while omitted and `openai` providers fail closed.

- [ ] **Step 7: Commit provider gate**

```bash
git add internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/adapters.go internal/cli/capture_commands_test.go
git commit -m "fix(capture): require Copilot provider evidence"
```

### Task 2: Reject Transient Setup Hook Executables

**Files:**
- Modify: `internal/cli/setup.go:187-211` (`setupInstallOptions`, `durableExecutablePath`)
- Modify: `internal/cli/setup_test.go:70-81` (`TestSetupInstallOptionsDeriveDurableExecutableForManualSetup`)
- Test: `internal/cli/setup_test.go`

**Interfaces:**
- Consumes: `validateCollectorExecutable(executable string) error` in `internal/cli/collector.go:293-299` and `filepath.EvalSymlinks` resolution in `durableExecutablePath(executable string) (string, error)`.
- Produces: `setupInstallOptions(home, executable) (adapters.InstallOptions, error)` rejects transient resolved hook executables before returning `ExecutablePath`.

- [ ] **Step 1: Write failing table-driven setup option tests**

Keep `TestSetupInstallOptionsDeriveDurableExecutableForManualSetup` for `os.Executable()` behavior. Add a sibling table test that creates executable-named files under `t.TempDir()` and calls `setupInstallOptions(t.TempDir(), path)`. Use forward-slash path segments through `filepath.Join` so Windows behavior exercises the validator's normalization.

```go
func TestSetupInstallOptionsRejectsTransientExecutablePaths(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "go test binary", path: filepath.Join(root, "qlog.test"), wantErr: true},
		{name: "windows go test binary", path: filepath.Join(root, "qlog.test.exe"), wantErr: true},
		{name: "go build cache binary", path: filepath.Join(root, "go-build123", "qlog"), wantErr: true},
		{name: "installed binary", path: filepath.Join(root, "bin", "qlog"), wantErr: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Dir(test.path), 0o700); err != nil { t.Fatal(err) }
			if err := os.WriteFile(test.path, nil, 0o700); err != nil { t.Fatal(err) }
			options, err := setupInstallOptions(t.TempDir(), test.path)
			if (err != nil) != test.wantErr { t.Fatalf("setupInstallOptions() error = %v, wantErr %t", err, test.wantErr) }
			if !test.wantErr && options.ExecutablePath != filepath.Clean(test.path) { t.Fatalf("ExecutablePath = %q", options.ExecutablePath) }
		})
	}
}
```

- [ ] **Step 2: Run setup tests to verify they fail**

Run: `go test -count=1 ./internal/cli -run 'TestSetupInstallOptions(DeriveDurableExecutableForManualSetup|RejectsTransientExecutablePaths)'`

Expected: FAIL because `durableExecutablePath` resolves and cleans transient paths without calling `validateCollectorExecutable`.

- [ ] **Step 3: Reuse durable collector validation after resolution**

Preserve path source selection, absolute-path rejection, symlink resolution, and cleaning. Validate only resolved clean path so a symlink cannot hide a transient target.

```go
func durableExecutablePath(executable string) (string, error) {
	if executable == "" {
		path, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolve qlog executable: %w", err)
		}
		executable = path
	}
	if !filepath.IsAbs(executable) {
		return "", fmt.Errorf("qlog executable path must be absolute: %q", executable)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve qlog executable path %q: %w", executable, err)
	}
	resolved = filepath.Clean(resolved)
	if err := validateCollectorExecutable(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}
```

- [ ] **Step 4: Run setup tests to verify they pass**

Run: `go test -count=1 ./internal/cli -run 'TestSetupInstallOptions(DeriveDurableExecutableForManualSetup|RejectsTransientExecutablePaths)'`

Expected: PASS; `os.Executable()` remains absolute and stat-able, transient targets are rejected, and durable installed path is returned.

- [ ] **Step 5: Commit durable executable validation**

```bash
git add internal/cli/setup.go internal/cli/setup_test.go
git commit -m "fix(setup): reject transient hook executables"
```

### Task 3: Classify OpenCode Plugin Callbacks As Lifecycle Events

**Files:**
- Modify: `internal/adapters/opencode.go:114-164` (`openCodePluginSource`)
- Modify: `internal/adapters/adapters_test.go:325-357` (`TestOpenCodeInstallWritesGlobalPluginPostingLocalEvents`)
- Modify: `internal/ingest/qlogevent/handler_test.go:20-51` (`TestHandlerKeepsOpenCodePluginEventsLifecycleOnly`)
- Test: `internal/adapters/adapters_test.go`
- Test: `internal/ingest/qlogevent/handler_test.go`

**Interfaces:**
- Consumes: generated plugin `base(type, ctx, event)` and `jsonl.normalizeModelCall(ctx context.Context, store *sqlite.Store, parsed event, rawEventID string) (bool, error)`, which creates calls only for normalized `model.call` event types.
- Produces: OpenCode selected `event` callbacks emit `event_type: "agent.event"`; raw lifecycle evidence persists with zero model-call and usage rows.

- [ ] **Step 1: Write failing generated-plugin contract assertion**

Update `TestOpenCodeInstallWritesGlobalPluginPostingLocalEvents` to require retained upstream callback selection (`message.updated` remains an observed upstream callback) and stable emitted type `agent.event`, while explicitly rejecting `base(event.type, ctx, event)`.

```go
for _, want := range []string{
	`["session.created", "message.updated", "session.idle", "session.error"]`,
	`base("agent.event", ctx, event)`,
	"tool.execute.before", "tool.execute.after", "capture_quality",
} {
	if !strings.Contains(text, want) {
		t.Fatalf("plugin missing %q:\n%s", want, text)
	}
}
if strings.Contains(text, "base(event.type, ctx, event)") {
	t.Fatalf("plugin emits upstream lifecycle type: %s", text)
}
```

- [ ] **Step 2: Write failing lifecycle ingestion assertions**

Change `TestHandlerKeepsOpenCodePluginEventsLifecycleOnly` input event type from `model.call` to `agent.event`; retain payload's provider/model/token fields to prove sanitization and lifecycle handling do not create usage. Query raw events and model calls directly, then confirm `Usage` has no rows.

```go
var rawEvents, modelCalls int
reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(service.Paths.Database)+"?mode=ro")
if err != nil { t.Fatal(err) }
t.Cleanup(func() { _ = reader.Close() })
if err := reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM raw_events WHERE source = 'opencode-plugin'`).Scan(&rawEvents); err != nil { t.Fatal(err) }
if err := reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_calls`).Scan(&modelCalls); err != nil { t.Fatal(err) }
if rawEvents != 1 || modelCalls != 0 {
	t.Fatalf("raw_events=%d model_calls=%d, want 1 and 0", rawEvents, modelCalls)
}
if len(report.Rows) != 0 || report.TotalTokens != 0 {
	t.Fatalf("OpenCode lifecycle event created usage: %#v", report)
}
```

- [ ] **Step 3: Run focused tests to verify they fail**

Run: `go test -count=1 ./internal/adapters -run TestOpenCodeInstallWritesGlobalPluginPostingLocalEvents`

Expected: FAIL because generated plugin currently calls `base(event.type, ctx, event)`.

Run: `go test -count=1 ./internal/ingest/qlogevent -run TestHandlerKeepsOpenCodePluginEventsLifecycleOnly`

Expected: FAIL because current assertion contract expects a lifecycle usage row, not zero model calls and zero usage rows.

- [ ] **Step 4: Emit stable OpenCode lifecycle event type**

Change only selected generic `event` callback emission. Keep upstream callback filter, tool callback behavior, endpoint, and payload allowlist unchanged.

```ts
event: async ({ event }) => {
  if (["session.created", "message.updated", "session.idle", "session.error"].includes(event.type)) {
    await post(base("agent.event", ctx, event))
  }
},
```

`agent.event` reaches existing `normalizeModelCall`, which returns without writing `model_calls` because it only accepts `model.call`.

- [ ] **Step 5: Run focused tests to verify they pass**

Run: `go test -count=1 ./internal/adapters -run TestOpenCodeInstallWritesGlobalPluginPostingLocalEvents`

Expected: PASS; plugin retains upstream callback selection but emits stable `agent.event`.

Run: `go test -count=1 ./internal/ingest/qlogevent -run TestHandlerKeepsOpenCodePluginEventsLifecycleOnly`

Expected: PASS; exactly one raw event persists, zero `model_calls` exist, and no usage row is emitted.

- [ ] **Step 6: Commit OpenCode lifecycle classification**

```bash
git add internal/adapters/opencode.go internal/adapters/adapters_test.go internal/ingest/qlogevent/handler_test.go
git commit -m "fix(opencode): preserve lifecycle-only events"
```

## Final Regression Checks

- [ ] Run formatting check for every modified Go file.

Run: `gofmt -d internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/adapters.go internal/cli/capture_commands_test.go internal/cli/setup.go internal/cli/setup_test.go internal/adapters/opencode.go internal/adapters/adapters_test.go internal/ingest/qlogevent/handler_test.go`

Expected: no output.

- [ ] Run full test suite.

Run: `go test -count=1 ./...`

Expected: PASS for all packages.

- [ ] Run static analysis.

Run: `go vet ./...`

Expected: no findings and exit status 0.

- [ ] Inspect final diff before opening or updating any review.

Run: `git diff --check && git status --short`

Expected: no whitespace errors; only intentional implementation, test, and plan changes are present. Do not claim real-device M4 acceptance from these synthetic checks.
