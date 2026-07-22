# Copilot Reality Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GitHub Copilot VS Code capture functional and verifiable from OTLP ingest through SQLite and `qlog usage project`.

**Architecture:** Keep the existing collector and JSONL normalization pipeline, but widen `/v1/traces` to OTLP JSON + protobuf. Harden the `copilot-vscode` adapter around VS Code settings lifecycle, add a staged `adapter verify` command, and document M4 as `IN_PROGRESS` until real Copilot evidence exists.

**Tech Stack:** Go, Cobra, modernc SQLite, OTLP protobuf types from OpenTelemetry, GitHub Copilot VS Code OTel settings, Windows user-session process management.

## Global Constraints

- Content capture must stay disabled: `github.copilot.chat.otel.captureContent=false`.
- Collector remains loopback-only by default.
- Unsupported OTLP content types must still return `415`.
- Prompt, response, tool arguments, tool results, secrets, and authorization fields must not be persisted.
- M4 remains `IN_PROGRESS` until `docs/verification/m4-evidence.md` proves real Copilot-originated capture.
- `v0.3.1` scope is Copilot-first; do not verify unrelated agents in this plan.
- Tag releases from `main`, not feature branches.

---

## File Map

- `internal/ingest/otlp/receiver.go`: HTTP content-type dispatch and OTLP JSON/protobuf decoding.
- `internal/ingest/otlp/receiver_test.go`: JSON/protobuf ingest tests and unsupported content-type test.
- `internal/adapters/vscode_settings.go`: new focused helper for VS Code settings path detection, JSONC parsing, atomic writes, managed-key metadata, and uninstall safety.
- `internal/adapters/vscode_copilot.go`: adapter descriptor, install, status, test, and uninstall behavior.
- `internal/adapters/adapters_test.go`: adapter setup/uninstall/status tests.
- `internal/cli/collector.go`: managed collector lifecycle commands.
- `internal/cli/collector_windows.go`: Windows user-session collector process helpers.
- `internal/cli/collector_other.go`: non-Windows clear unsupported response for managed lifecycle.
- `internal/cli/adapters.go`: `adapter verify` command.
- `internal/cli/capture_commands_test.go`: CLI tests for collector lifecycle command shape and adapter verify output.
- `internal/storage/sqlite/store.go`: query helper for recent Copilot reported model calls if existing report API is insufficient.
- `README.md` and `docs/DEVELOPER_GUIDE.md`: honest M4/Copilot state.
- `docs/verification/m4-evidence.md`: evidence checklist and current verification output.

---

### Task 1: Downgrade M4 Claims and Add Evidence Skeleton

**Files:**
- Modify: `README.md:13-94`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Create: `docs/verification/m4-evidence.md`

**Interfaces:**
- Consumes: approved spec `docs/superpowers/specs/2026-07-21-copilot-reality-check-design.md`.
- Produces: honest docs that later tasks update with verification evidence.

- [ ] **Step 1: Write the failing doc check**

Run:

```bash
rg "M4 \| `IMPLEMENTED`|functional with honest capture-quality labels|CAPTURE_VERIFIED" README.md docs/DEVELOPER_GUIDE.md docs/verification/m4-evidence.md
```

Expected before the change: at least one match in README or developer guide claiming M4 implemented/functional without evidence.

- [ ] **Step 2: Update README status table**

Change the M4 row to this exact text:

```markdown
| M4 | `IN_PROGRESS` | Copilot VS Code has setup and experimental OTel ingestion, but real Copilot-to-SQLite E2E evidence is not complete. OpenCode/Codex/Claude remain quality-labeled experimental or lifecycle-only paths. |
```

Change the setup table rows to:

```markdown
| `copilot-vscode` | VS Code GitHub Copilot OTel settings to local `/v1/traces`, content capture disabled. | `CAPTURE_EXPERIMENTAL`; promote only after `docs/verification/m4-evidence.md` records real Copilot-originated tokens in SQLite. |
| `opencode` | Global OpenCode plugin posts sanitized events to local `/v1/events`. | `agent_reported` when plugin payload includes usage; otherwise `lifecycle_only`. |
| `codex` | Codex app-server `rawResponse/completed` events can be forwarded to `/v1/events`. | `agent_reported` only when `usage` is non-null. |
| `claude-code` | `.claude/settings.json` lifecycle hooks call `qlog hook claude-code`. | `lifecycle_only`; no token capability is claimed. |
| `pi`, `openclaw`, `hermes` | Setup-capable fallback targets. | `lifecycle_only` or `unavailable` until verified token sources exist. |
```

- [ ] **Step 3: Add evidence skeleton**

Create `docs/verification/m4-evidence.md` with:

```markdown
# M4 Evidence

M4 is `IN_PROGRESS` until this file records real Copilot-originated OTLP data flowing into SQLite and `qlog usage project`.

## Current State

| Adapter | State | Evidence |
|---|---|---|
| copilot-vscode | CAPTURE_EXPERIMENTAL | Setup exists. OTLP receiver tests are synthetic until a real Copilot run is recorded below. |
| opencode | CAPTURE_EXPERIMENTAL | Plugin path exists; real usage depends on payload usage fields. |
| codex | CAPTURE_EXPERIMENTAL | `rawResponse/completed` path exists; real usage depends on non-null usage. |
| claude-code | LIFECYCLE_ONLY | Lifecycle hooks exist; token capture is not claimed. |

## Required Copilot Verification

- [ ] `qlog setup copilot-vscode --yes` installs settings with content capture disabled.
- [ ] `qlog collector start` leaves a reachable loopback collector.
- [ ] Real Copilot VS Code emits an OTLP span or event to qlog.
- [ ] SQLite contains a Copilot-originated `model.call` with `capture_quality=otel_reported`.
- [ ] `qlog usage project <slug>` shows model and tokens for the target project, or explicitly records `unattributed` if Copilot did not provide reliable project context.
- [ ] No prompt, response, tool arguments, tool results, secrets, or authorization fields are persisted.

## Evidence Log

Pending real Copilot run.
```

- [ ] **Step 4: Verify doc check passes**

Run:

```bash
rg "M4 \| `IMPLEMENTED`|functional with honest capture-quality labels|CAPTURE_VERIFIED" README.md docs/DEVELOPER_GUIDE.md docs/verification/m4-evidence.md
```

Expected: no false claim that M4 is implemented or Copilot is verified. `CAPTURE_VERIFIED` may appear only as a future criterion.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/DEVELOPER_GUIDE.md docs/verification/m4-evidence.md
git commit -m "docs: mark copilot capture experimental"
```

---

### Task 2: Accept OTLP Protobuf on `/v1/traces`

**Files:**
- Modify: `go.mod`
- Modify: `internal/ingest/otlp/receiver.go`
- Modify: `internal/ingest/otlp/receiver_test.go`

**Interfaces:**
- Consumes: existing `Receiver.ServeHTTP` and `Receiver.ingest(ctx, exportTraceServiceRequest)`.
- Produces: `decodeRequest(*http.Request, http.ResponseWriter) (exportTraceServiceRequest, error)` and protobuf mapping into existing internal request structs.

- [ ] **Step 1: Add failing protobuf test**

Add this test to `internal/ingest/otlp/receiver_test.go`:

```go
func TestReceiverAcceptsOTLPProtobuf(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	service, err := app.Open(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	repo := t.TempDir()
	project, err := service.RegisterProject(ctx, repo, "Project", nil)
	if err != nil {
		t.Fatal(err)
	}

	payload := &collectortracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
			{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "copilot-chat"}}},
			{Key: "session.id", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "window-1"}}},
		}},
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{
			TraceId: []byte{1, 2, 3},
			StartTimeUnixNano: uint64(time.Date(2026, 7, 21, 1, 0, 0, 0, time.UTC).UnixNano()),
			Attributes: []*commonpb.KeyValue{
				{Key: "qlog.project", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: project.Slug}}},
				{Key: "gen_ai.operation.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "chat"}}},
				{Key: "gen_ai.provider.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "github"}}},
				{Key: "gen_ai.request.model", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "gpt-5"}}},
				{Key: "gen_ai.response.model", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "gpt-5-resolved"}}},
				{Key: "gen_ai.usage.input_tokens", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 11}}},
				{Key: "gen_ai.usage.output_tokens", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 13}}},
			},
		}}}},
	}}}
	body, err := proto.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/x-protobuf")
	response := httptest.NewRecorder()
	otlp.NewHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	report, err := service.Store.UsageReport(ctx, sqlite.UsageQuery{ProjectSlug: project.Slug})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rows) != 1 || report.Rows[0].AgentName != "copilot-chat" || report.Rows[0].TotalTokens != 24 || report.Rows[0].CaptureQuality != "otel_reported" {
		t.Fatalf("usage rows = %#v", report.Rows)
	}
}
```

Add imports if missing:

```go
import (
	"google.golang.org/protobuf/proto"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test -count=1 ./internal/ingest/otlp -run TestReceiverAcceptsOTLPProtobuf
```

Expected: fail with missing imports or `415` before implementation.

- [ ] **Step 3: Implement content-type dispatch**

In `receiver.go`, replace JSON-only check with:

```go
payload, err := decodeTraceRequest(request, writer)
if err != nil {
	http.Error(writer, err.Error(), statusForDecodeError(err))
	return
}
```

Add helpers:

```go
var errUnsupportedMediaType = errors.New("unsupported OTLP content type")

func decodeTraceRequest(request *http.Request, writer http.ResponseWriter) (exportTraceServiceRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
	defer func() { _ = request.Body.Close() }()
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]))
	switch contentType {
	case "application/json":
		var payload exportTraceServiceRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return exportTraceServiceRequest{}, fmt.Errorf("decode OTLP JSON: %w", err)
		}
		return payload, nil
	case "application/x-protobuf", "application/protobuf":
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return exportTraceServiceRequest{}, fmt.Errorf("read OTLP protobuf: %w", err)
		}
		var protobufPayload collectortracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &protobufPayload); err != nil {
			return exportTraceServiceRequest{}, fmt.Errorf("decode OTLP protobuf: %w", err)
		}
		return fromProto(&protobufPayload), nil
	default:
		return exportTraceServiceRequest{}, errUnsupportedMediaType
	}
}

func statusForDecodeError(err error) int {
	if errors.Is(err, errUnsupportedMediaType) {
		return http.StatusUnsupportedMediaType
	}
	return http.StatusBadRequest
}
```

Add imports:

```go
"errors"
"io"
collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
commonpb "go.opentelemetry.io/proto/otlp/common/v1"
"google.golang.org/protobuf/proto"
```

- [ ] **Step 4: Implement protobuf mapping**

Add:

```go
func fromProto(input *collectortracepb.ExportTraceServiceRequest) exportTraceServiceRequest {
	output := exportTraceServiceRequest{ResourceSpans: make([]resourceSpans, 0, len(input.GetResourceSpans()))}
	for _, resourceSpan := range input.GetResourceSpans() {
		mappedResource := resourceSpans{Resource: resource{Attributes: fromProtoAttributes(resourceSpan.GetResource().GetAttributes())}}
		for _, scopeSpan := range resourceSpan.GetScopeSpans() {
			mappedScope := scopeSpans{Spans: make([]span, 0, len(scopeSpan.GetSpans()))}
			for _, protoSpan := range scopeSpan.GetSpans() {
				mappedScope.Spans = append(mappedScope.Spans, span{
					TraceID:           fmt.Sprintf("%x", protoSpan.GetTraceId()),
					StartTimeUnixNano: strconv.FormatUint(protoSpan.GetStartTimeUnixNano(), 10),
					Attributes:        fromProtoAttributes(protoSpan.GetAttributes()),
				})
			}
			mappedResource.ScopeSpans = append(mappedResource.ScopeSpans, mappedScope)
		}
		output.ResourceSpans = append(output.ResourceSpans, mappedResource)
	}
	return output
}

func fromProtoAttributes(values []*commonpb.KeyValue) []keyValue {
	result := make([]keyValue, 0, len(values))
	for _, value := range values {
		result = append(result, keyValue{Key: value.GetKey(), Value: fromProtoValue(value.GetValue())})
	}
	return result
}

func fromProtoValue(value *commonpb.AnyValue) attributeValue {
	switch typed := value.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return attributeValue{StringValue: typed.StringValue}
	case *commonpb.AnyValue_IntValue:
		return attributeValue{IntValue: json.Number(strconv.FormatInt(typed.IntValue, 10))}
	case *commonpb.AnyValue_DoubleValue:
		return attributeValue{StringValue: strconv.FormatFloat(typed.DoubleValue, 'f', -1, 64)}
	case *commonpb.AnyValue_BoolValue:
		return attributeValue{StringValue: strconv.FormatBool(typed.BoolValue)}
	default:
		return attributeValue{}
	}
}
```

- [ ] **Step 5: Run receiver tests**

```bash
go test -count=1 ./internal/ingest/otlp
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/ingest/otlp/receiver.go internal/ingest/otlp/receiver_test.go
git commit -m "feat(otlp): accept protobuf traces"
```

---

### Task 3: Harden VS Code Settings Lifecycle

**Files:**
- Create: `internal/adapters/vscode_settings.go`
- Modify: `internal/adapters/vscode_copilot.go`
- Modify: `internal/adapters/adapters_test.go`

**Interfaces:**
- Produces: `applyVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error)`.
- Produces: `removeVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error)`.
- Produces: `vscodeSettingsPath(variant string) string`.

- [ ] **Step 1: Add failing JSONC install test**

Add to `adapters_test.go`:

```go
func TestVSCodeCopilotInstallHandlesJSONCAndPreservesSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	before := `{
  // keep this user setting
  "editor.fontSize": 14,
}
`
	if err := os.WriteFile(settingsPath, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}

	adapter := newVSCodeCopilotAdapter()
	result, err := adapter.Install(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Changes[0].BackupPath == "" {
		t.Fatalf("install result = %#v", result)
	}
	contents, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "editor.fontSize") || !strings.Contains(text, "github.copilot.chat.otel.enabled") || strings.Contains(text, "github.copilot.chat.otel.captureContent\": true") {
		t.Fatalf("settings after install = %s", text)
	}
}
```

- [ ] **Step 2: Add failing uninstall test**

Add:

```go
func TestVSCodeCopilotUninstallRemovesOnlyManagedSettings(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	adapter := newVSCodeCopilotAdapter()
	if _, err := adapter.Install(context.Background(), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configHome, "Code", "User", "settings.json")
	settings := readSettingsMap(t, settingsPath)
	settings["editor.fontSize"] = float64(14)
	settings["github.copilot.chat.otel.outfile"] = "C:/tmp/copilot.jsonl"
	writeSettingsMap(t, settingsPath, settings)

	result, err := adapter.Uninstall(context.Background(), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatalf("uninstall result = %#v", result)
	}
	after := readSettingsMap(t, settingsPath)
	if _, found := after["github.copilot.chat.otel.enabled"]; found {
		t.Fatalf("managed setting remained: %#v", after)
	}
	if after["editor.fontSize"] != float64(14) || after["github.copilot.chat.otel.outfile"] != "C:/tmp/copilot.jsonl" {
		t.Fatalf("unrelated settings not preserved: %#v", after)
	}
}
```

Add helper functions:

```go
func readSettingsMap(t *testing.T, path string) map[string]any {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal(contents, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func writeSettingsMap(t *testing.T, path string, settings map[string]any) {
	t.Helper()
	contents, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(contents, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Run tests to verify failure**

```bash
go test -count=1 ./internal/adapters -run 'TestVSCodeCopilot(InstallHandlesJSONC|UninstallRemovesOnlyManagedSettings)'
```

Expected: JSON parse failure or uninstall leaving settings behind.

- [ ] **Step 4: Create `vscode_settings.go`**

Implement JSONC sanitization and atomic write:

```go
package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const qlogVSCodeManagedKey = "qlog.managed.github.copilot.chat.otel"

func vscodeSettingsPath(variant string) string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "Code", "User", "settings.json")
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "Code", "User", "settings.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("Code", "User", "settings.json")
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
	default:
		return filepath.Join(home, ".config", "Code", "User", "settings.json")
	}
}

func applyVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error) {
	settings, original, existed, err := readVSCodeSettings(path)
	if err != nil {
		return SetupChange{}, err
	}
	changed := false
	managed := map[string]any{}
	for key, value := range desired {
		if settings[key] != value {
			settings[key] = value
			changed = true
		}
		managed[key] = value
	}
	if settings[qlogVSCodeManagedKey] == nil {
		settings[qlogVSCodeManagedKey] = map[string]any{owner: managed}
		changed = true
	} else {
		settings[qlogVSCodeManagedKey] = map[string]any{owner: managed}
		changed = true
	}
	if !changed {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog settings already up to date"}, nil
	}
	action := "created"
	if existed {
		action = "updated"
	}
	if dryRun {
		change := SetupChange{Path: path, Action: action, Description: "dry run: qlog settings would be written"}
		if existed {
			change.BackupPath = fmt.Sprintf("%s.qlog-backup-%s", path, "planned")
		}
		return change, nil
	}
	return writeVSCodeSettings(path, settings, original, existed, action)
}

func removeVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error) {
	settings, original, existed, err := readVSCodeSettings(path)
	if err != nil {
		return SetupChange{}, err
	}
	if !existed {
		return SetupChange{Path: path, Action: "unchanged", Description: "settings file does not exist"}, nil
	}
	changed := false
	for key := range desired {
		if _, found := settings[key]; found {
			delete(settings, key)
			changed = true
		}
	}
	if _, found := settings[qlogVSCodeManagedKey]; found {
		delete(settings, qlogVSCodeManagedKey)
		changed = true
	}
	if !changed {
		return SetupChange{Path: path, Action: "unchanged", Description: "no qlog-managed settings found"}, nil
	}
	if dryRun {
		return SetupChange{Path: path, Action: "removed", Description: "dry run: qlog settings would be removed", BackupPath: fmt.Sprintf("%s.qlog-backup-%s", path, "planned")}, nil
	}
	return writeVSCodeSettings(path, settings, original, true, "removed")
}

func readVSCodeSettings(path string) (map[string]any, []byte, bool, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	settings := map[string]any{}
	cleaned := stripJSONC(contents)
	if len(bytes.TrimSpace(cleaned)) > 0 {
		if err := json.Unmarshal(cleaned, &settings); err != nil {
			return nil, nil, false, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	return settings, contents, true, nil
}

func writeVSCodeSettings(path string, settings map[string]any, original []byte, existed bool, action string) (SetupChange, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, fmt.Errorf("create parent directory: %w", err)
	}
	change := SetupChange{Path: path, Action: action, Description: "qlog settings written"}
	if existed {
		backupPath := fmt.Sprintf("%s.qlog-backup-%s", path, time.Now().UTC().Format("20060102150405"))
		if err := os.WriteFile(backupPath, original, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("write backup: %w", err)
		}
		change.BackupPath = backupPath
	}
	next, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return SetupChange{}, err
	}
	tmp := path + ".qlog-tmp"
	if err := os.WriteFile(tmp, append(next, '\n'), 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write temp settings: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return SetupChange{}, fmt.Errorf("replace %s: %w", path, err)
	}
	return change, nil
}

func stripJSONC(input []byte) []byte {
	var output strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			output.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			output.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			output.WriteByte('\n')
			continue
		}
		if ch == '/' && i+1 < len(input) && input[i+1] == '*' {
			i += 2
			for i+1 < len(input) && !(input[i] == '*' && input[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		output.WriteByte(ch)
	}
	return removeTrailingCommas([]byte(output.String()))
}

func removeTrailingCommas(input []byte) []byte {
	output := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == ',' {
			j := i + 1
			for j < len(input) && (input[j] == ' ' || input[j] == '\n' || input[j] == '\r' || input[j] == '\t') {
				j++
			}
			if j < len(input) && (input[j] == '}' || input[j] == ']') {
				continue
			}
		}
		output = append(output, input[i])
	}
	return output
}
```

- [ ] **Step 5: Wire Copilot adapter to helper**

In `vscode_copilot.go`:

```go
func (a vscodeCopilotAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a vscodeCopilotAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := removeVSCodeSettings(a.settingsPath(), copilotOTelSettings(), a.id, options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && change.Action == "removed", Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a vscodeCopilotAdapter) settingsPath() string {
	return vscodeSettingsPath("Code")
}
```

Make `jsonSettingsContain` use `readVSCodeSettings`.

- [ ] **Step 6: Run adapter tests**

```bash
go test -count=1 ./internal/adapters
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add internal/adapters/vscode_settings.go internal/adapters/vscode_copilot.go internal/adapters/adapters_test.go
git commit -m "fix(copilot): harden vscode settings lifecycle"
```

---

### Task 4: Add Managed Collector Lifecycle Commands

**Files:**
- Modify: `internal/cli/collector.go`
- Create: `internal/cli/collector_windows.go`
- Create: `internal/cli/collector_other.go`
- Modify: `internal/cli/capture_commands_test.go`

**Interfaces:**
- Produces: `collectorManager` interface with `Install`, `Start`, `Stop`, `Restart`, `Logs`, `Uninstall`.
- Produces: CLI subcommands under `qlog collector`.

- [ ] **Step 1: Add command-shape test**

In `capture_commands_test.go`, add:

```go
func TestCollectorLifecycleCommandsExist(t *testing.T) {
	command := New(Version{})
	collector, _, err := command.Find([]string{"collector"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"install", "start", "stop", "restart", "logs", "uninstall"} {
		found := false
		for _, child := range collector.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("collector command %q not found", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify failure**

```bash
go test -count=1 ./internal/cli -run TestCollectorLifecycleCommandsExist
```

Expected: fail because commands do not exist.

- [ ] **Step 3: Add manager interface and commands**

In `collector.go`, add:

```go
type collectorManager interface {
	Install(home, listen string) (string, error)
	Start(home, listen string) (string, error)
	Stop() (string, error)
	Restart(home, listen string) (string, error)
	Logs() (string, error)
	Uninstall() (string, error)
}
```

Add command factory:

```go
func collectorLifecycleCommand(name, short string, run func(collectorManager, string, string) (string, error), home *string, listen *string) *cobra.Command {
	return &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		message, err := run(newCollectorManager(), *home, *listen)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.Root().OutOrStdout(), message)
		return err
	}}
}
```

Register:

```go
collector.AddCommand(
	status,
	serve,
	collectorLifecycleCommand("install", "Install managed collector", func(m collectorManager, home, listen string) (string, error) { return m.Install(home, listen) }, home, &listen),
	collectorLifecycleCommand("start", "Start managed collector", func(m collectorManager, home, listen string) (string, error) { return m.Start(home, listen) }, home, &listen),
	collectorLifecycleCommand("stop", "Stop managed collector", func(m collectorManager, _, _ string) (string, error) { return m.Stop() }, home, &listen),
	collectorLifecycleCommand("restart", "Restart managed collector", func(m collectorManager, home, listen string) (string, error) { return m.Restart(home, listen) }, home, &listen),
	collectorLifecycleCommand("logs", "Show managed collector logs", func(m collectorManager, _, _ string) (string, error) { return m.Logs() }, home, &listen),
	collectorLifecycleCommand("uninstall", "Uninstall managed collector", func(m collectorManager, _, _ string) (string, error) { return m.Uninstall() }, home, &listen),
)
```

- [ ] **Step 4: Implement Windows manager**

Create `collector_windows.go`:

```go
//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type windowsCollectorManager struct{}

func newCollectorManager() collectorManager { return windowsCollectorManager{} }

func collectorStateDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "QUANTUM_LOG", "collector")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "QUANTUM_LOG", "collector")
}

func collectorPIDPath() string { return filepath.Join(collectorStateDir(), "collector.pid") }
func collectorLogPath() string { return filepath.Join(collectorStateDir(), "collector.log") }

func (windowsCollectorManager) Install(home, listen string) (string, error) {
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return "", err
	}
	return fmt.Sprintf("collector installed for user session at %s", collectorStateDir()), nil
}

func (m windowsCollectorManager) Start(home, listen string) (string, error) {
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	logFile, err := os.OpenFile(collectorLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer func() { _ = logFile.Close() }()
	args := []string{"collector", "serve", "--listen", listen}
	if home != "" {
		args = append([]string{"--home", home}, args...)
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if err := os.WriteFile(collectorPIDPath(), []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o600); err != nil {
		return "", err
	}
	return fmt.Sprintf("collector started with pid %d", cmd.Process.Pid), nil
}

func (windowsCollectorManager) Stop() (string, error) {
	pidBytes, err := os.ReadFile(collectorPIDPath())
	if err != nil {
		return "collector is not running", nil
	}
	cmd := exec.Command("taskkill", "/PID", string(pidBytes), "/T", "/F")
	_ = cmd.Run()
	_ = os.Remove(collectorPIDPath())
	return "collector stopped", nil
}

func (m windowsCollectorManager) Restart(home, listen string) (string, error) {
	_, _ = m.Stop()
	return m.Start(home, listen)
}

func (windowsCollectorManager) Logs() (string, error) {
	contents, err := os.ReadFile(collectorLogPath())
	if err != nil {
		return "collector log is empty", nil
	}
	return string(contents), nil
}

func (m windowsCollectorManager) Uninstall() (string, error) {
	_, _ = m.Stop()
	if err := os.RemoveAll(collectorStateDir()); err != nil {
		return "", err
	}
	return "collector uninstalled", nil
}
```

- [ ] **Step 5: Implement non-Windows manager**

Create `collector_other.go`:

```go
//go:build !windows

package cli

import "errors"

type unsupportedCollectorManager struct{}

func newCollectorManager() collectorManager { return unsupportedCollectorManager{} }

func (unsupportedCollectorManager) Install(_, _ string) (string, error) { return "", errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve") }
func (unsupportedCollectorManager) Start(_, _ string) (string, error) { return "", errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve") }
func (unsupportedCollectorManager) Stop() (string, error) { return "", errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve") }
func (unsupportedCollectorManager) Restart(_, _ string) (string, error) { return "", errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve") }
func (unsupportedCollectorManager) Logs() (string, error) { return "", errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve") }
func (unsupportedCollectorManager) Uninstall() (string, error) { return "", errors.New("managed collector lifecycle is currently implemented for Windows only; use qlog collector serve") }
```

- [ ] **Step 6: Run CLI tests**

```bash
go test -count=1 ./internal/cli -run TestCollectorLifecycleCommandsExist
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/collector.go internal/cli/collector_windows.go internal/cli/collector_other.go internal/cli/capture_commands_test.go
git commit -m "feat(collector): add managed lifecycle commands"
```

---

### Task 5: Add `adapter verify copilot-vscode`

**Files:**
- Modify: `internal/cli/adapters.go`
- Modify: `internal/cli/capture_commands_test.go`
- Modify: `internal/storage/sqlite/store.go` if a direct query helper is needed.

**Interfaces:**
- Produces CLI: `qlog adapter verify copilot-vscode --project <slug> --since 1h --json`.
- Produces JSON output with `adapter_id`, `ready`, `stages`, `message`.

- [ ] **Step 1: Add failing command test**

In `capture_commands_test.go` add:

```go
func TestAdapterVerifyCopilotReportsMissingEvidence(t *testing.T) {
	home := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", configHome)
	command := New(Version{})
	output := new(bytes.Buffer)
	command.SetArgs([]string{"--home", home, "adapter", "verify", "copilot-vscode", "--json"})
	setOutput(command, output)
	if err := command.Execute(); err != nil {
		t.Fatalf("adapter verify: %v", err)
	}
	var result struct {
		AdapterID string `json:"adapter_id"`
		Ready     bool   `json:"ready"`
		Stages    []struct {
			Name   string `json:"name"`
			Passed bool   `json:"passed"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("output = %s: %v", output.String(), err)
	}
	if result.AdapterID != "copilot-vscode" || result.Ready || len(result.Stages) == 0 {
		t.Fatalf("verify result = %#v", result)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

```bash
go test -count=1 ./internal/cli -run TestAdapterVerifyCopilotReportsMissingEvidence
```

Expected: fail because `verify` command does not exist.

- [ ] **Step 3: Add verify command types**

In `adapters.go`:

```go
type adapterVerifyStage struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type adapterVerifyResult struct {
	AdapterID string               `json:"adapter_id"`
	Ready     bool                 `json:"ready"`
	Stages    []adapterVerifyStage `json:"stages"`
	Message   string               `json:"message"`
}
```

- [ ] **Step 4: Implement verify command**

Add under `newAdapterCommand`:

```go
var verifyJSON bool
verify := &cobra.Command{Use: "verify <adapter>", Short: "Verify adapter capture readiness and evidence", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
	adapter, ok := registry.Get(args[0])
	if !ok {
		return fmt.Errorf("unknown adapter %q", args[0])
	}
	result := verifyAdapter(command.Context(), *home, adapter)
	if verifyJSON {
		return writeJSON(command.Root().OutOrStdout(), result)
	}
	for _, stage := range result.Stages {
		state := "FAIL"
		if stage.Passed {
			state = "PASS"
		}
		_, _ = fmt.Fprintf(command.Root().OutOrStdout(), "%s %s: %s\n", state, stage.Name, stage.Message)
	}
	if !result.Ready {
		return nil
	}
	_, err := fmt.Fprintln(command.Root().OutOrStdout(), result.Message)
	return err
}}
verify.Flags().BoolVar(&verifyJSON, "json", false, "output JSON")
command.AddCommand(verify)
```

Add:

```go
func verifyAdapter(ctx context.Context, home string, adapter adapters.Adapter) adapterVerifyResult {
	status, err := adapter.Status(ctx)
	stages := []adapterVerifyStage{}
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "status", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Stages: stages, Message: "adapter status failed"}
	}
	stages = append(stages, adapterVerifyStage{Name: "settings", Passed: status.Installed, Message: status.State})
	if adapter.Descriptor().ID != "copilot-vscode" {
		ready := status.Installed
		return adapterVerifyResult{AdapterID: adapter.Descriptor().ID, Ready: ready, Stages: stages, Message: "generic adapter verification complete"}
	}
	service, err := app.Open(ctx, home)
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "database", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: "copilot-vscode", Stages: stages, Message: "database unavailable"}
	}
	defer func() { _ = service.Close() }()
	report, err := service.Store.UsageReport(ctx, sqlite.UsageQuery{})
	if err != nil {
		stages = append(stages, adapterVerifyStage{Name: "usage", Passed: false, Message: err.Error()})
		return adapterVerifyResult{AdapterID: "copilot-vscode", Stages: stages, Message: "usage query failed"}
	}
	foundCopilot := false
	for _, row := range report.Rows {
		if row.CaptureQuality == "otel_reported" && strings.Contains(strings.ToLower(row.AgentName), "copilot") && row.TotalTokens > 0 {
			foundCopilot = true
		}
	}
	stages = append(stages, adapterVerifyStage{Name: "copilot_model_call", Passed: foundCopilot, Message: "requires real Copilot-originated otel_reported model call with tokens"})
	ready := status.Installed && foundCopilot
	message := "Copilot capture is not verified yet"
	if ready {
		message = "Copilot capture verified"
	}
	return adapterVerifyResult{AdapterID: "copilot-vscode", Ready: ready, Stages: stages, Message: message}
}
```

Add imports: `strings`, app/sqlite packages if missing.

- [ ] **Step 5: Run verify test**

```bash
go test -count=1 ./internal/cli -run TestAdapterVerifyCopilotReportsMissingEvidence
```

Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/adapters.go internal/cli/capture_commands_test.go internal/storage/sqlite/store.go
git commit -m "feat(adapters): verify copilot capture evidence"
```

---

### Task 6: Update Evidence and Run Full Verification

**Files:**
- Modify: `docs/verification/m4-evidence.md`
- Modify: `docs/DEVELOPER_GUIDE.md`
- Modify: `README.md` if command list changed.

**Interfaces:**
- Consumes: all previous tasks.
- Produces: final evidence and release readiness notes.

- [ ] **Step 1: Update command docs**

Add to README setup section:

```bash
qlog collector install
qlog collector start
qlog adapter verify copilot-vscode
qlog usage project QUANTUM_LOG
qlog collector logs
```

Add note:

```markdown
`qlog collector serve` remains the foreground debug path. Use `qlog collector start` for managed background capture on supported platforms.
```

- [ ] **Step 2: Run full automated verification**

```bash
go test -count=1 ./...
go vet ./...
golangci-lint run
git diff --check
```

Expected: all pass with no output from `go vet`, `golangci-lint`, or `git diff --check`.

- [ ] **Step 3: Perform Windows Copilot E2E**

Run:

```bash
go run ./cmd/qlog init
go run ./cmd/qlog project register --path . --name QUANTUM_LOG
go run ./cmd/qlog setup copilot-vscode --yes
go run ./cmd/qlog collector install
go run ./cmd/qlog collector start
go run ./cmd/qlog adapter verify copilot-vscode --json
```

Then open VS Code in this repo and send one Copilot Chat/Agent message. Run:

```bash
go run ./cmd/qlog adapter verify copilot-vscode --json
go run ./cmd/qlog usage project quantum-log --json
go run ./cmd/qlog collector logs
```

Expected for close: `adapter verify` returns `ready=true` and usage rows include `capture_quality=otel_reported`, `agent_name` containing `copilot`, `provider=github`, a model, and non-zero tokens.

- [ ] **Step 4: Update evidence file**

Record exact commands and results. If real Copilot does not emit project context, write this explicitly:

```markdown
Project attribution result: `unattributed` because real Copilot OTel did not include a stable workspace or repository path attribute. Token capture is verified; project attribution remains experimental.
```

If real Copilot emits project context and qlog resolves it, write:

```markdown
Project attribution result: `quantum-log` via <method> with <confidence> confidence.
```

- [ ] **Step 5: Commit evidence/docs**

```bash
git add README.md docs/DEVELOPER_GUIDE.md docs/verification/m4-evidence.md
git commit -m "docs: record copilot capture evidence"
```

---

### Task 7: Final PR Gate

**Files:**
- No required file edits unless verification exposes gaps.

**Interfaces:**
- Consumes: completed implementation and evidence.
- Produces: pushed branch and PR ready for review.

- [ ] **Step 1: Final local verification**

Run:

```bash
go test -count=1 ./...
go vet ./...
golangci-lint run
git diff --check
git status --short
```

Expected: tests pass, static checks pass, `git status --short` clean.

- [ ] **Step 2: Push branch**

```bash
git push -u origin feat/v0.3.1-copilot-reality-check
```

- [ ] **Step 3: Create PR**

Create PR body:

```markdown
## Summary
- hardens Copilot VS Code setup/uninstall and marks capture as experimental until evidence passes
- accepts OTLP HTTP JSON and protobuf on `/v1/traces`
- adds managed collector lifecycle commands and Copilot adapter verification
- records M4 evidence for Copilot reality check

## Verification
- [ ] go test -count=1 ./...
- [ ] go vet ./...
- [ ] golangci-lint run
- [ ] git diff --check
- [ ] Windows Copilot E2E recorded in docs/verification/m4-evidence.md

Closes #12
```

- [ ] **Step 4: Watch CI**

```bash
gh pr checks --watch --interval 10
```

Expected: all required checks pass.
