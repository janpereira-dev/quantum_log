# M1 Ledger Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make project attribution, local ledger integrity, privacy boundaries, and diagnostic behavior satisfy Milestone 1 acceptance criteria with reproducible evidence.

**Architecture:** Keep resolution policy in `internal/attribution/resolver`, application orchestration in `internal/app`, and persistence in `internal/storage/sqlite`. Introduce read-only database access separately from migration-capable access. Persist sanitized ledger records in SQLite, then atomically update an external ledger anchor located beside user configuration.

**Tech Stack:** Go 1.26.5, Cobra, modernc.org/sqlite, embedded SQL migrations, standard-library JSON and filesystem APIs.

## Global Constraints

- Use exact resolution precedence: explicit, `QLOG_PROJECT`, CWD, Git root, registered path, adapter hint, `unattributed`.
- `qlog doctor` must not create config, SQLite, WAL files, migrations, or timestamps.
- Every official SQLite client takes shared quiescence; writers additionally take exclusive writer lock after quiescence.
- `qlog doctor` and `qlog verify` take exclusive quiescence, block on non-empty WAL, warn on isolated SHM, and use immutable access only after quiescence.
- Sanitization happens before hashing and persistence; prompts, responses, credentials, headers, cookies, and unsafe evidence do not reach SQLite.
- Hash verification detects edits, reordered events, duplicate sequences, intermediate deletion, tail truncation, and anchor/database divergence.
- Keep `Project` logical, `ProjectLocation` path-specific, and `WorkContext` temporal.
- Preserve CGo-free builds: `CGO_ENABLED=0 go test -count=1 ./...` must pass.
- Do not mark M1 `VERIFIED` with any `FAIL`, `NOT_RUN`, or `BLOCKED` evidence row.

---

### Task 1: Add milestone status and evidence scaffolding

**Files:**
- Create: `docs/verification/milestone-1-evidence.md`
- Modify: `README.md`
- Modify: `QUANTUM_LOG_MASTER_PROMPT.md`
- Test: manual documentation consistency review

**Interfaces:**
- Consumes: acceptance criteria in `QUANTUM_LOG_MASTER_PROMPT.md`.
- Produces: a factual M1 state and a matrix whose rows map every M1 criterion to executable evidence.

- [ ] **Step 1: Write failing documentation assertions**

Add an evidence row for every M1 criterion with initial state `FAIL` or
`NOT_RUN`. Add a README status section declaring M0 `IMPLEMENTED`, M1 `BLOCKED`,
M2 `IMPLEMENTED` but unverified, M3 `IN_PROGRESS`, M4 `DETECTION_ONLY`, M5
`IMPLEMENTED` but unverified, and M6 `IMPLEMENTED` but unverified.

- [ ] **Step 2: Verify existing public claims are unsupported**

Run: `rg -n "M0|M1|M2|M3|M4|M5|M6|verified capture|implemented" README.md`

Expected: current claims overstate availability and require replacement.

- [ ] **Step 3: Add state and evidence contracts**

Add the six-state milestone contract and evidence-matrix rules from
`docs/superpowers/specs/2026-07-19-m1-m6-recovery-design.md`. Replace public
claims with only current evidence-backed states.

- [ ] **Step 4: Verify documentation consistency**

Run: `rg -n "VERIFIED|CAPTURE_VERIFIED|Milestone" README.md QUANTUM_LOG_MASTER_PROMPT.md docs/verification/milestone-1-evidence.md`

Expected: no M1-M6 feature is publicly described as verified before evidence.

### Task 2: Correct project-resolution precedence and location selection

**Files:**
- Modify: `internal/attribution/resolver/resolver.go`
- Modify: `internal/attribution/resolver/resolver_test.go`
- Modify: `internal/app/service.go`
- Modify: `internal/storage/sqlite/store.go`
- Modify: `internal/storage/sqlite/store_test.go`
- Modify: `internal/cli/root_test.go`

**Interfaces:**
- Consumes: `resolver.Input`, `resolver.ProjectResolution`, and registered path mappings.
- Produces: `Resolve(input, locations)` selecting exact precedence and a resolved `ProjectLocation` matching CWD/Git evidence.

- [ ] **Step 1: Write failing precedence tests**

Replace adapter-over-environment coverage with tests for:

```go
func TestResolvePrecedence(t *testing.T) {
    registered := map[string]string{"C:/repos/a": "project-a"}
    tests := []struct{ name string; input Input; want Method }{
        {"explicit beats environment", Input{ExplicitProject: "a", EnvironmentProject: "b"}, Explicit},
        {"environment beats cwd", Input{EnvironmentProject: "a", CWD: "C:/repos/a"}, Environment},
        {"cwd beats git", Input{CWD: "C:/repos/a", GitRoot: "C:/repos/b"}, CWD},
        {"git beats registered fallback", Input{GitRoot: "C:/repos/a", AdapterProject: "b"}, GitRoot},
        {"registered path beats adapter hint", Input{CWD: "C:/repos/a", AdapterProject: "b"}, CWD},
        {"unknown is unattributed", Input{}, Unresolved},
    }
    for _, test := range tests {
        got := Resolve(test.input, registered)
        if got.Method != test.want {
            t.Fatalf("%s: method = %q, want %q", test.name, got.Method, test.want)
        }
    }
}
```

Add a store test registering two locations for one slug and asserting CWD returns
the matching location, not the first row returned for that project.

- [ ] **Step 2: Run targeted failing tests**

Run: `go test ./internal/attribution/resolver ./internal/storage/sqlite -run "TestResolvePrecedence|TestProjectLocation" -count=1`

Expected: FAIL because adapter currently precedes `QLOG_PROJECT` and location
selection is slug-first.

- [ ] **Step 3: Implement deterministic source ordering**

Change `Resolve` so it evaluates `ExplicitProject`, `EnvironmentProject`, exact
CWD match, Git-root match, longest registered path, `AdapterProject`, then
unresolved. Add a distinct `Adapter` method only for the final hint. Keep
evidence to a non-secret path alias or normalized path.

Change service/store lookup to select a `ProjectLocation` by normalized matched
path. Do not call `ProjectBySlug` when a specific resolution path is available.

- [ ] **Step 4: Add CLI contract tests**

Add tests for `qlog project current --json` that assert matching location ID,
location path, method, confidence, and evidence. Add `qlog project register
--path . --name <name>` idempotency coverage.

- [ ] **Step 5: Run targeted passing tests**

Run: `go test ./internal/attribution/resolver ./internal/storage/sqlite ./internal/cli -count=1`

Expected: PASS.

### Task 3: Make diagnostics and ledger verification read-only

**Files:**
- Modify: `internal/storage/sqlite/store.go`
- Modify: `internal/app/service.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`
- Create: `internal/storage/sqlite/readonly_test.go`

**Interfaces:**
- Consumes: database path from `config.Paths`.
- Produces: `sqlite.OpenReadOnly(ctx, databasePath) (*Store, error)` and `app.OpenReadOnly(ctx, home) (*Service, error)`.

- [ ] **Step 1: Write read-only behavior tests**

Create tests that snapshot absent and initialized homes before and after `qlog
doctor`:

```go
func TestDoctorIsReadOnly(t *testing.T) {
    home := t.TempDir()
    before := snapshotTree(t, home)
    _, err := runQLog(t, home, "doctor", "--json")
    if err == nil { t.Fatal("doctor accepted an uninitialized home") }
    assertTreeEqual(t, before, snapshotTree(t, home))

    runQLog(t, home, "init")
    before = snapshotTree(t, home)
    runQLog(t, home, "doctor", "--json")
    assertTreeEqual(t, before, snapshotTree(t, home))
}
```

The snapshot records relative paths, content hashes, and file modes. It includes
`qlog.db`, `qlog.db-wal`, and `qlog.db-shm` when present.

- [ ] **Step 2: Run the failing diagnostic test**

Run: `go test ./internal/cli -run TestDoctorIsReadOnly -count=1`

Expected: FAIL because `app.Open` calls migration-capable `sqlite.Open`.

- [ ] **Step 3: Add quiescent read-only open path**

Add cross-platform `quiescence.lock` and `writer.lock` support. Every official
SQLite read takes shared quiescence; every writer then takes writer exclusive.
`OpenReadOnly` takes exclusive quiescence, requires existing lock files, blocks
on non-empty WAL, warns on isolated SHM, then uses SQLite URI
`mode=ro&immutable=1`. It must not call `ensureParent` or `migrate`.
`app.OpenReadOnly` resolves paths, requires an existing database, and does not
call `config.Ensure`.

Change `doctor` and `verify` to use `OpenReadOnly`. Return actionable errors for
uninitialized database, active official client, pending WAL, lock inconsistency,
pending schema, or failed integrity check. Do not write WAL mode pragmas on this
path. Add explicit mutating maintenance commands for checkpoint, recovery, and
anchor rebuild; they take exclusive quiescence then exclusive writer lock.

- [ ] **Step 4: Run read-only tests**

Run: `go test ./internal/cli ./internal/storage/sqlite -run "TestDoctorIsReadOnly|TestOpenReadOnly" -count=1`

Expected: PASS.

### Task 4: Sanitize every persisted raw-event field before hashing

**Files:**
- Modify: `internal/storage/sqlite/store.go`
- Modify: `internal/ingest/jsonl/importer.go`
- Modify: `internal/ingest/jsonl/importer_test.go`
- Modify: `internal/storage/sqlite/store_test.go`

**Interfaces:**
- Consumes: `RawEventInput{Payload, EvidenceJSON}`.
- Produces: `SanitizeJSON(raw []byte) ([]byte, error)` and a sanitized,
canonical `RawEventInput` passed to hashing and SQLite insertion.

- [ ] **Step 1: Write failing sanitization tests**

Cover payload and evidence using API keys, bearer tokens, cookies,
Authorization headers, URL credentials, prompt content, response content,
tool arguments, and benign text:

```go
func TestAppendRawEventSanitizesBeforePersistence(t *testing.T) {
    id, err := store.AppendRawEvent(ctx, RawEventInput{
        Source: "fixture", EventType: "model.call",
        Payload: []byte(`{"prompt":"secret","authorization":"Bearer token","safe":"ok"}`),
        EvidenceJSON: `{"url":"https://user:pass@example.test","api_key":"key"}`,
    })
    requireNoError(t, err)
    event := readRawEvent(t, store, id)
    assertNotContains(t, event.Payload, "secret", "token")
    assertNotContains(t, event.Evidence, "user:pass", "key")
    assertContains(t, event.Payload, "ok", "[REDACTED]")
}
```

- [ ] **Step 2: Run the failing sanitizer tests**

Run: `go test ./internal/storage/sqlite ./internal/ingest/jsonl -run Sanitize -count=1`

Expected: FAIL because evidence currently bypasses `sanitizePayload`.

- [ ] **Step 3: Implement canonical redaction boundary**

Use one recursive JSON sanitizer for payload and evidence. It removes known
content-bearing keys and replaces secret-bearing values with `[REDACTED]`.
Validate JSON before hashing; invalid evidence fails ingestion instead of falling
back to raw text. Make `AppendRawEvent` sanitize both fields before
`canonicalEvent` and SQL insertion. Ensure importer passes only structured
evidence into this boundary.

- [ ] **Step 4: Run sanitization regression tests**

Run: `go test ./internal/storage/sqlite ./internal/ingest/jsonl -count=1`

Expected: PASS.

### Task 5: Add external ledger anchors and truncation detection

**Files:**
- Create: `internal/audit/anchor.go`
- Create: `internal/audit/anchor_test.go`
- Modify: `internal/config/paths.go`
- Modify: `internal/app/service.go`
- Modify: `internal/storage/sqlite/store.go`
- Modify: `internal/storage/sqlite/store_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`

**Interfaces:**
- Produces:

```go
type Anchor struct {
    LedgerID string `json:"ledger_id"`
    LastSequence int64 `json:"last_sequence"`
    LastEventHash string `json:"last_event_hash"`
    EventCount int64 `json:"event_count"`
    AnchoredAt time.Time `json:"anchored_at"`
    AnchorVersion int `json:"anchor_version"`
}
func LoadAnchor(path string) (Anchor, error)
func WriteAnchor(path string, anchor Anchor) error
```

- [ ] **Step 1: Write failing integrity tests**

Add tests that append two events, verify successfully, then independently:
update payload, delete first event, delete last event, delete both tail events,
duplicate sequence, modify anchor, and replace SQLite with an older copy. Every
mutation must make `VerifyLedger` fail.

- [ ] **Step 2: Run the failing truncation tests**

Run: `go test ./internal/storage/sqlite ./internal/audit -run "TestLedger.*(Truncation|Anchor|Tamper)" -count=1`

Expected: FAIL because raw events have no monotonic sequence or external anchor.

- [ ] **Step 3: Add migration and atomic anchor update**

Add a new embedded migration rebuilding `raw_events` with
`ledger_sequence INTEGER NOT NULL UNIQUE`; SQLite cannot add this non-null unique
column in place. Copy existing rows in deterministic creation order, assigning
contiguous sequences, recreate indexes, then atomically replace the old table.
Add a sequence allocator for future events. Extend `config.Paths` with
`LedgerAnchor` at `<home>/state/ledger-anchor.json`.

During `AppendRawEvent`, compute the canonical sanitized event, append it with
the next sequence, commit SQLite, then write the anchor through a same-directory
temporary file followed by rename. If anchor update fails, return an error and
surface divergence through `qlog verify`; never silently claim verification.

`VerifyLedger` checks contiguous sequence, event count, chain head, and anchor
head. It reports whether SQLite is ahead of, behind, or inconsistent with the
anchor.

- [ ] **Step 4: Run ledger tests**

Run: `go test ./internal/audit ./internal/storage/sqlite -count=1`

Expected: PASS.

### Task 6: Validate multi-project attribution and explicit unattributed behavior

**Files:**
- Modify: `fixtures/session-a-b-a.ndjson`
- Modify: `fixtures/README.md`
- Modify: `internal/ingest/jsonl/importer.go`
- Modify: `internal/ingest/jsonl/importer_test.go`
- Modify: `internal/storage/sqlite/store_test.go`
- Modify: `internal/cli/root_test.go`

**Interfaces:**
- Consumes: fixture events with explicit project/location/work-context identity.
- Produces: raw events and normalized calls assigned to A, B, then A without
cross-project leakage; unresolved events remain `unattributed`.

- [ ] **Step 1: Write failing fixture tests**

Create a test importing the A-B-A fixture and assert three distinct work
contexts, correct project IDs on each raw event/model call, and project totals
that match only each project’s calls. Add an unresolved fixture event and assert
it remains visible in unattributed reporting.

- [ ] **Step 2: Run the failing fixture test**

Run: `go test ./internal/ingest/jsonl ./internal/storage/sqlite -run "Test.*(SessionABA|Unattributed)" -count=1`

Expected: FAIL because fixture hints are not resolved into persisted project and
work-context identities.

- [ ] **Step 3: Implement normalized fixture ingestion**

Extend NDJSON event parsing with resolution inputs only when they are safe and
explicitly documented. Resolve project through the application service, create a
new WorkContext whenever project/location evidence changes, and attach returned
IDs to raw and normalized events. Preserve unresolved events without inventing a
project.

- [ ] **Step 4: Run multi-project tests**

Run: `go test ./internal/ingest/jsonl ./internal/storage/sqlite ./internal/cli -count=1`

Expected: PASS.

### Task 7: Run M1 acceptance verification and publish evidence

**Files:**
- Modify: `docs/verification/milestone-1-evidence.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: all M1 command outputs and generated anchor/database fixtures.
- Produces: a truthful evidence matrix and public state matching actual results.

- [ ] **Step 1: Execute M1 required verification**

Run these commands from clean worktree state:

```text
go test -count=1 ./...
go build ./...
go vet ./...
CGO_ENABLED=0 go test -count=1 ./...
goreleaser release --snapshot --clean
```

Expected: all commands pass. Record exact output and environment limitations in
the evidence matrix.

- [ ] **Step 2: Execute CLI behavior fixtures**

Run isolated-home commands covering `qlog --version`, idempotent `qlog init`,
`qlog doctor --json`, `qlog verify`, `qlog project register`, `qlog project
detect`, and `qlog project current --json`. Preserve sanitized fixture output
under `docs/verification/artifacts/m1/`.

- [ ] **Step 3: Complete Evidence Matrix**

Set each M1 row to `PASS`, `FAIL`, `NOT_RUN`, `NOT_APPLICABLE`, or `BLOCKED`.
Link test names, command output, and generated hashes. Do not set M1 to
`VERIFIED` until every required row is `PASS`.

- [ ] **Step 4: Final documentation consistency check**

Run: `rg -n "VERIFIED|IMPLEMENTED|CAPTURE_VERIFIED|M1" README.md CHANGELOG.md docs/verification/milestone-1-evidence.md`

Expected: public claims exactly match evidence state.
