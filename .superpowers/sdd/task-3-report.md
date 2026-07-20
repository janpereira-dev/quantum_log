# Task 3: Read-only diagnostics and ledger verification

## Scope

- Implemented only Task 3 from `docs/superpowers/plans/2026-07-19-m1-ledger-recovery.md`.
- Added a read-only SQLite open path for `qlog doctor` and `qlog verify`.
- No commit created. Tasks 4-7 were not changed.

## RED

Initial initialized-home snapshot test passed because the initialized database was already in WAL mode and the legacy writable open did not make an observable change.

The test was strengthened before production changes by removing the latest migration record, then snapshotting the initialized home. This proves diagnostics must not repair pending schema.

Command:

```text
go test ./internal/cli -run "Test(Doctor|Verify)IsReadOnly" -count=1
```

Result:

```text
--- FAIL: TestDoctorIsReadOnly
    root_test.go:93: qlog doctor pending schema error = <nil>
--- FAIL: TestVerifyIsReadOnly
    root_test.go:119: qlog verify pending schema error = <nil>
FAIL
```

This showed both commands used `app.Open`, which ran migrations and silently repaired the pending schema.

## GREEN

Added:

- `sqlite.OpenReadOnly`: checks database existence, uses `mode=ro`, `immutable=1`, and `query_only(1)`, skips parent-directory creation and migrations, and validates every embedded migration is already applied.
- `app.OpenReadOnly`: resolves paths without `config.Ensure` and requires an existing database.
- Read-only command wiring for `doctor` and `verify`.
- CLI snapshots of relative paths, file modes, and SHA-256 contents for absent homes, initialized homes, and pending-schema homes.
- Storage test ensuring a missing database does not create its parent directory.

Focused commands:

```text
go test ./internal/cli -run "Test(Doctor|Verify)IsReadOnly" -count=1
ok github.com/janpereira-dev/quantum_log/internal/cli

go test ./internal/storage/sqlite -run TestOpenReadOnlyDoesNotCreateMissingDatabase -count=1
ok github.com/janpereira-dev/quantum_log/internal/storage/sqlite
```

The first implementation with `mode=ro` alone still created `qlog.db-wal` and `qlog.db-shm`. Adding `immutable=1` made initialized-home snapshots pass without sidecar creation.

## Full Verification

```text
go test ./internal/attribution/resolver ./internal/storage/sqlite ./internal/cli -count=1
ok github.com/janpereira-dev/quantum_log/internal/attribution/resolver
ok github.com/janpereira-dev/quantum_log/internal/storage/sqlite
ok github.com/janpereira-dev/quantum_log/internal/cli

go test ./... -count=1
PASS: all packages

git diff --check -- internal/storage/sqlite/store.go internal/app/service.go internal/cli/root.go internal/cli/root_test.go
PASS: no whitespace errors
```

## Error Behavior

- Missing or uninitialized database: returns guidance to run `qlog init` without creating configuration, database, WAL, SHM, or parent directories.
- Pending schema: returns `database schema has pending migration ...; run qlog init` without applying it.
- Failed integrity check: existing `Store.Doctor` behavior remains unchanged and reports the SQLite integrity-check failure.
- Valid initialized database: `doctor --json` and `verify` retain existing success behavior.

## Concern

`immutable=1` is required to meet the no-WAL/no-SHM write contract. It intentionally avoids SQLite locking and WAL visibility, so a diagnostic run concurrent with a writer may validate the main database snapshot rather than uncheckpointed WAL content. This is the required no-write tradeoff; concurrent-writer semantics need a separate explicit design if required later.

## Review Follow-up

### RED

Added two focused regression tests before production changes:

- `TestDiagnosticsRejectActiveWAL` keeps a writer open with a non-empty `qlog.db-wal`, then proves both `doctor --json` and `verify` must reject it with close-and-retry guidance.
- `TestSnapshotTreeRecordsModificationTimes` changes only a file modification timestamp and proves the snapshot must change.

Command:

```text
go test ./internal/cli -run "Test(DiagnosticsRejectActiveWAL|SnapshotTreeRecordsModificationTimes)" -count=1
```

Result:

```text
--- FAIL: TestDiagnosticsRejectActiveWAL
    root_test.go:145: qlog doctor --json active WAL error = <nil>
--- FAIL: TestSnapshotTreeRecordsModificationTimes
    root_test.go:165: snapshot did not record fixture modification time
FAIL
```

### GREEN

- `OpenReadOnly` now checks `<database>-wal` before opening SQLite. A non-empty WAL returns `database has an active WAL; close active qlog writers and retry`.
- Snapshots now include root, directory, and file modification times along with path, content hash, and mode. Existing doctor/verify snapshots therefore assert unchanged timestamps for absent, initialized, and pending-schema homes.

Focused verification:

```text
go test ./internal/cli -run "Test(DiagnosticsRejectActiveWAL|SnapshotTreeRecordsModificationTimes)" -count=1
ok github.com/janpereira-dev/quantum_log/internal/cli

go test ./internal/cli -run "Test(Doctor|Verify)IsReadOnly|DiagnosticsRejectActiveWAL|SnapshotTreeRecordsModificationTimes" -count=1
ok github.com/janpereira-dev/quantum_log/internal/cli

go test ./internal/storage/sqlite -count=1
ok github.com/janpereira-dev/quantum_log/internal/storage/sqlite
```

### CGo-free Verification

```text
CGO_ENABLED=0 go test ./... -count=1
PASS: all packages
```

The prior concurrent-WAL concern is now guarded: diagnostics do not report ledger health when a non-empty WAL could make the immutable main-database view stale.

## TOCTOU Investigation

### Context7 Evidence

Context7 retrieved the `modernc.org/sqlite` `Driver.Open` documentation from <https://pkg.go.dev/modernc.org/sqlite>. The driver accepts SQLite URI names with query parameters, and each `_pragma` parameter is executed as a `PRAGMA ...` statement. This supports the tested driver syntax `file:...?...&_pragma=query_only(1)`. The documentation does not establish no-mutation or WAL-consistency semantics, so those were tested directly.

### RED

The hypothesis test created a committed raw event, corrupted only its hash in an open writer's non-empty WAL, and ran `qlog verify`. A correct WAL-aware verifier must return `ledger event hash does not match`; an immutable connection would ignore the committed WAL state.

With the current immutable-plus-WAL-rejection safeguard, the test failed as expected:

```text
go test ./internal/cli -run TestVerifyReadsCommittedWALWithoutMutatingFiles -count=1
--- FAIL: TestVerifyReadsCommittedWALWithoutMutatingFiles
    root_test.go:147: qlog verify active WAL error = database has an active WAL; close active qlog writers and retry
FAIL
```

### Mode=ro Experiment

Temporarily removed `immutable=1` and the pre-open WAL rejection while retaining `mode=ro` and `_pragma=query_only(1)`. The verifier read the corrupted committed WAL event, but then mutated `qlog.db-shm`:

```text
--- FAIL: TestVerifyReadsCommittedWALWithoutMutatingFiles
    root_test.go:149: filesystem snapshot "qlog.db-shm" = "file:-rw-rw-rw-:b2d503592651e2c486b4dc7e3694c3378dbaaee989c9dc2628dbc52147c41f1d:2026-07-19T20:34:16.8327321Z", want "file:-rw-rw-rw-:d37e0b5351ebff24d223f10b8cb66888dd1d70e089a07723536670bf6b237d77:2026-07-19T20:34:16.8327321Z"
FAIL
```

This proves the non-immutable read-only connection can observe committed WAL state but violates the no-file-mutation contract by changing SHM contents. Per review instruction, no workaround was added. The experiment was reverted to immutable mode with active-WAL rejection.

### Viable Lock Design

A future design needs writer cooperation, not a different SQLite URI. A coordinator should hold an exclusive cross-process lock, block new writers, cause the writer-owned connection to checkpoint and close the WAL, then let diagnostics acquire a shared lock and use the immutable snapshot path. Writers must hold the same lock before reopening or writing. This prevents the checkpoint-to-diagnostic-open race without letting diagnostics mutate SQLite sidecars.

## Cooperative Lock Correction

### Context7 Rationale

Context7 retrieved `golang.org/x/sys` lock APIs: Unix `Flock` accepts shared, exclusive, and unlock modes; Windows `LockFileEx` supports `LOCKFILE_FAIL_IMMEDIATELY` and `LOCKFILE_EXCLUSIVE_LOCK`, with matching `UnlockFileEx`. The focused `internal/storage/lock` package wraps these APIs behind shared and exclusive handles. No dependency change was required because `golang.org/x/sys v0.47.0` already existed in `go.mod`.

### RED

Before the lock implementation, focused tests failed because writer opens did not create a sibling lock and readers accepted a missing lock:

```text
go test ./internal/storage/sqlite -run "Test(OpenCreatesExclusiveLockFile|OpenReadOnlyDoesNotCreateMissingLock|OpenReadOnlyBlockedWhileWriterHoldsLock|CloseCheckpointsWALBeforeReadOnlyVerification|OpenReadOnlyRejectsStaleWAL)" -count=1
--- FAIL: TestOpenCreatesExclusiveLockFile
    locking_test.go:23: stat lock file: ... qlog.db.lock: The system cannot find the file specified.
--- FAIL: TestOpenReadOnlyDoesNotCreateMissingLock
    locking_test.go:46: OpenReadOnly accepted a missing lock
FAIL

go test ./internal/cli -run TestDiagnosticsRejectActiveWriter -count=1
--- FAIL: TestDiagnosticsRejectActiveWriter
    root_test.go:145: qlog doctor --json active writer error = database has an active WAL; close active qlog writers and retry
FAIL
```

### GREEN

- Writer `Store.Open` creates and exclusively locks `<database>.lock` before opening or migrating SQLite, then retains it through `Store.Close`.
- Read-only `Store.OpenReadOnly` opens an existing lock file only and acquires a nonblocking shared lock. Missing locks return init guidance; writer contention returns an actionable retry error.
- Writer `Store.Close` executes `PRAGMA wal_checkpoint(TRUNCATE)` before closing SQLite and releasing its exclusive lock.
- Read-only diagnostics retain immutable SQLite access and reject any remaining non-empty WAL.
- Snapshot tests continue to compare paths, hashes, modes, and modification timestamps.

Verification:

```text
go test ./internal/storage/lock ./internal/storage/sqlite ./internal/cli -count=1
PASS

go test ./... -count=1
PASS

CGO_ENABLED=0 go test ./... -count=1
PASS

GOOS=linux CGO_ENABLED=0 go test -c -o NUL ./internal/storage/sqlite
GOOS=windows CGO_ENABLED=0 go test -c -o NUL ./internal/storage/sqlite
PASS
```

## Residual Verification Closure

### Verify Isolated-SHM Coverage

Added direct `qlog verify` coverage matching doctor behavior: an isolated `qlog.db-shm` emits an isolated-SHM warning, still reports `ledger: verified`, and leaves the full path/hash/mode/timestamp snapshot unchanged.

The first run passed because warning propagation was already implemented for verify alongside doctor; no production behavior change was needed:

```text
go test ./internal/cli -run "Test(Doctor|Verify)WarnsForIsolatedSHMWithoutMutation" -count=1
PASS
```

### Close-Error Testability

No focused CLI close-error test was added. `newDoctorCommand` and `newVerifyCommand` construct concrete `app.Service` values through `app.OpenReadOnly`, and `Service.Close` delegates to concrete SQLite/lock handles. Read-only close has no controllable real failure condition in the command contract. Adding a production-only injection seam or a test-only error hook would expand the runtime API solely for testing, so it was intentionally not added. The commands already join their concrete `Close` errors into the returned `RunE` result.

### Windows Command Equivalents

The report's environment-variable examples use POSIX assignment syntax. Run these equivalents on Windows.

`cmd.exe`:

```bat
set "CGO_ENABLED=0" && go test ./... -count=1
set "GOOS=linux" && set "CGO_ENABLED=0" && go test -c -o NUL ./internal/storage/sqlite
set "GOOS=windows" && set "CGO_ENABLED=0" && go test -c -o NUL ./internal/storage/sqlite
```

PowerShell:

```powershell
$env:CGO_ENABLED = '0'; go test ./... -count=1
$env:GOOS = 'linux'; $env:CGO_ENABLED = '0'; go test -c -o NUL ./internal/storage/sqlite
$env:GOOS = 'windows'; $env:CGO_ENABLED = '0'; go test -c -o NUL ./internal/storage/sqlite
```

## ADR-004 Quiescence and Maintenance Correction

### RED

Single-lock behavior could not satisfy accepted ADR-004. It neither created the required two lock files nor gave diagnostics exclusive quiescence:

```text
go test ./internal/storage/sqlite -run "Test(OpenCreatesExclusiveLockFile|OpenReadOnlyDoesNotCreateMissingLock|OpenReadOnlyBlockedWhileWriterHoldsLock|OpenReadOnlyBlocksWhileReaderHoldsQuiescence)" -count=1
--- FAIL: TestOpenCreatesExclusiveLockFile
    locking_test.go:26: stat lock file ... qlog.db.quiescence.lock: The system cannot find the file specified.
--- FAIL: TestOpenReadOnlyDoesNotCreateMissingLock
    locking_test.go:52: OpenReadOnly accepted a missing lock
--- FAIL: TestOpenReadOnlyBlockedWhileWriterHoldsLock
    locking_test.go:81: OpenReadOnly writer lock error = database lock is held by an active qlog writer; retry after it exits
--- FAIL: TestOpenReadOnlyBlocksWhileReaderHoldsQuiescence
    locking_test.go:100: OpenReadOnly acquired exclusive quiescence while a reader held it
FAIL

go test ./internal/cli -run "Test(DiagnosticsRejectActiveWriter|DiagnosticsBlockWhileReadOnlyClientHoldsQuiescence|DoctorBlocksPendingWALWithoutMutation|DoctorWarnsForIsolatedSHMWithoutMutation|MaintenanceCommandSurface)" -count=1
--- FAIL: TestDoctorWarnsForIsolatedSHMWithoutMutation
    root_test.go:201: qlog doctor isolated SHM output = "{...\\\"status\\\":\\\"ok\\\"}"
--- FAIL: TestMaintenanceCommandSurface
    root_test.go:212: qlog maintenance status: unknown command "maintenance" for "qlog"
FAIL
```

### GREEN

- Writer `Store.Open` acquires `<database>.quiescence.lock` shared before `<database>.writer.lock` exclusive. Both are created with the existing lock creation contract.
- `OpenReadOnly` acquires existing quiescence exclusively, requires the writer lock to exist without creating it, blocks active official writers/readers, rejects non-empty WAL, and reports isolated SHM as a non-mutating warning.
- Doctor and verify now join `Close` failures into their command result.
- `maintenance status`, `checkpoint`, `recover`, and `rebuild-anchor` exist. Checkpoint validates ledger, obtains exclusive quiescence then writer locks, checkpoints WAL, and confirms no non-empty WAL remains. Recover and rebuild-anchor return explicit blocked/not-implemented errors pending Task 5.
- Context7 confirms modernc accepts SQLite URI names and `_pragma` configuration; the implementation uses its existing immutable read-only URI only after quiescence checks.

Focused verification:

```text
go test ./internal/cli -run "Test(DiagnosticsRejectActiveWriter|DiagnosticsBlockWhileReadOnlyClientHoldsQuiescence|DoctorBlocksPendingWALWithoutMutation|DoctorWarnsForIsolatedSHMWithoutMutation|MaintenanceCommandSurface)" -count=1
PASS

go test ./internal/storage/sqlite -run "Test(OpenCreatesExclusiveLockFile|OpenReadOnlyDoesNotCreateMissingLock|OpenReadOnlyBlockedWhileWriterHoldsLock|OpenReadOnlyBlocksWhileReaderHoldsQuiescence|CheckpointClearsWAL)" -count=1
PASS
```

Full verification:

```text
go test ./internal/storage/lock ./internal/storage/sqlite ./internal/cli -count=1
PASS

go test ./... -count=1
PASS

CGO_ENABLED=0 go test ./... -count=1
PASS

GOOS=linux CGO_ENABLED=0 go test -c -o NUL ./internal/storage/sqlite
GOOS=windows CGO_ENABLED=0 go test -c -o NUL ./internal/storage/sqlite
PASS
```

The Unix lock-file mode assertion confirms `0600`; Windows keeps the `0600` creation request but cannot expose equivalent Unix permission bits through `os.FileMode`.

## Checkpoint Result and Reader Exclusion Follow-up

### RED

`Store.Close` used `ExecContext` for `PRAGMA wal_checkpoint(TRUNCATE)`, so it discarded the result row that reports checkpoint busy state. A non-cooperative raw reader was held across a second WAL write; the old close path returned nil:

```text
go test ./internal/storage/sqlite -run "Test(OpenBlocksWhileReadOnlyStoresHoldSharedLock|CloseReturnsBusyCheckpointErrorAndReleasesLock)" -count=1
--- FAIL: TestCloseReturnsBusyCheckpointErrorAndReleasesLock
    locking_test.go:144: Close() busy checkpoint error = <nil>
FAIL
```

### GREEN

- `Store.checkpointWAL` now uses `QueryRowContext(...).Scan(&busy, &logFrames, &checkpointedFrames)`.
- A nonzero `busy` result returns `WAL checkpoint busy` with all three result values, while `Store.Close` still closes SQLite and releases its advisory lock.
- `qlog init` now joins the writer `Service.Close` error into its `RunE` result instead of silently discarding it.
- `TestOpenBlocksWhileReadOnlyStoresHoldSharedLock` holds two shared read-only stores, verifies writer open fails, closes both readers, then verifies writer open succeeds.

Focused verification:

```text
go test ./internal/storage/sqlite -run "Test(OpenBlocksWhileReadOnlyStoresHoldSharedLock|CloseReturnsBusyCheckpointErrorAndReleasesLock|CloseCheckpointsWALBeforeReadOnlyVerification)" -count=1
PASS

go test ./internal/cli -run "TestCoreCommandsInitializeAndReportProject|TestDiagnosticsRejectActiveWriter" -count=1
PASS
```

Full verification:

```text
go test ./internal/storage/lock ./internal/storage/sqlite ./internal/cli -count=1
PASS

go test ./... -count=1
PASS

CGO_ENABLED=0 go test ./... -count=1
PASS

GOOS=linux CGO_ENABLED=0 go test -c -o NUL ./internal/storage/sqlite
GOOS=windows CGO_ENABLED=0 go test -c -o NUL ./internal/storage/sqlite
PASS
```
