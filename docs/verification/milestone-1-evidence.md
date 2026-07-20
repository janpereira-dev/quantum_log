# Milestone 1 Evidence Matrix

## Current State

`M1` is `BLOCKED`. Task 1 creates this baseline only; it executes no product
build, test, integration, migration, or fixture command. Therefore no acceptance
criterion is `PASS`.

`VERIFIED` requires every required row to be `PASS`. `FAIL`, `NOT_RUN`, and
`BLOCKED` prevent that transition. Source files, stubs, registrations, templates,
or isolated unit tests are not acceptance evidence by themselves.

| AC ID | Criterion | Test | Command | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M1-01 | `go build ./...` succeeds. | Full build | `go build ./...` | Not executed in Task 1. | None. | `NOT_RUN` |
| M1-02 | `go test ./...` succeeds. | Full test suite | `go test ./...` | Not executed in Task 1. | None. | `NOT_RUN` |
| M1-03 | Binary is named `qlog`. | Future test ID: `TestRootCommandName`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-04 | `qlog --version` shows version, commit, and build date. | Future test ID: `TestVersionOutputIncludesMetadata`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-05 | `qlog init` creates config and SQLite in OS-correct paths. | Future test ID: `TestInitCreatesPlatformPaths`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-06 | `qlog init` is idempotent. | Future test ID: `TestInitIsIdempotent`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-07 | `qlog doctor` does not modify the system. | Future test ID: `TestDoctorIsReadOnly`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-08 | `qlog doctor --json` returns valid JSON. | Future test ID: `TestDoctorJSONOutput`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-09 | `qlog verify` accepts an intact chain. | Future test ID: `TestVerifyAcceptsIntactLedger`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-10 | Manual event modification breaks verification. | Future test ID: `TestVerifyRejectsManualEventModification`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-11 | SQLite works with `CGO_ENABLED=0`. | CGo-free suite | `CGO_ENABLED=0 go test -count=1 ./...` | Not executed in Task 1. | None. | `NOT_RUN` |
| M1-12 | Migrations are embedded. | Future test ID: `TestEmbeddedMigrations`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-13 | Initial contracts/tables exist for required entities. | Future test ID: `TestInitialSchemaContracts`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-14 | `project register --path .` is idempotent. | Future test ID: `TestProjectRegisterIsIdempotent`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-15 | `project current` shows project, location, method, and confidence. | Future test ID: `TestProjectCurrentHumanOutput`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-16 | `project current --json` has stable JSON. | Future test ID: `TestProjectCurrentStableJSON`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-17 | Absolute path is not a permanent `Project` property. | Future test ID: `TestProjectDoesNotPersistAbsolutePath`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-18 | Git branch and commit belong to `WorkContext`. | Future test ID: `TestWorkContextOwnsGitMetadata`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-19 | A session can move A -> B -> A without mixing usage. | Future test ID: `TestSessionProjectTransitionAtoBtoA`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-20 | One identity can have multiple `ProjectLocation` values. | Future test ID: `TestProjectSupportsMultipleLocations`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-21 | `--project` overrides `QLOG_PROJECT`. | Future test ID: `TestExplicitProjectOverridesEnvironment`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-22 | `QLOG_PROJECT` overrides CWD and Git inference. | Future test ID: `TestEnvironmentOverridesCWDAndGit`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-23 | Impossible resolution remains `unattributed`. | Future test ID: `TestImpossibleResolutionIsUnattributed`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-24 | Events retain sanitized resolution method, confidence, and evidence. | Future test ID: `TestEventPersistsSanitizedResolutionEvidence`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-25 | Invalid allocation sums are rejected. | Future test ID: `TestInvalidAllocationSumRejected`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-26 | Prompt and response content are not stored. | Future test ID: `TestPromptAndResponseRedactedBeforePersistence`. | Not executed; implement and run the named test before recording evidence. | No evidence recorded. | None. | `NOT_RUN` |
| M1-27 | README, LICENSE, SECURITY, CONTRIBUTING, and CHANGELOG exist. | Required tracked files: expect exit 0 only when all five paths match. | `git ls-files --error-unmatch README.md LICENSE SECURITY.md CONTRIBUTING.md CHANGELOG.md` | Not executed in Task 1. Expected: exit 0 and all five paths listed. | None. | `NOT_RUN` |
| M1-28 | CI covers Linux, macOS, and Windows. | OS-matrix assertions: expect every platform query to exit 0. | `cmd /c "rg -q \"ubuntu\" .github\\workflows && rg -q \"macos\" .github\\workflows && rg -q \"windows\" .github\\workflows"` | Not executed in Task 1. Expected: exit 0 only when all three platform labels occur. | None. | `NOT_RUN` |
| M1-29 | GoReleaser produces a local snapshot. | Snapshot assertion: expect release success and `dist` output. | `cmd /c "goreleaser release --snapshot --clean && if exist dist (exit /b 0) else (exit /b 1)"` | Not executed in Task 1. Expected: exit 0 and generated `dist` output. | None. | `NOT_RUN` |
| M1-30 | No secrets, binaries, or databases are committed. | Negative repository-hygiene assertion: return success only for a no-match result. | `powershell -NoProfile -Command "$paths = @(git ls-files); if ($paths -match '\.db$' -or $paths -match '\.sqlite$' -or $paths -match '\.exe$' -or $paths -match '\.zip$' -or $paths -match '\.tar\.gz$' -or $paths -match '\.env$' -or $paths -match 'secret') { exit 1 }; exit 0"` | Not executed in Task 1. Expected: exit 0 only when no forbidden tracked path matches. | None. | `NOT_RUN` |
| M1-31 | Example data is marked DEMO or fixture. | Negative fixture-label assertion: return failure when any fixture lacks a marker. | `powershell -NoProfile -Command "$unlabeled = @(rg -L -i -e demo -e fixture fixtures); if ($unlabeled.Count -gt 0) { exit 1 }; exit 0"` | Not executed in Task 1. Expected: exit 0 only when every fixture file contains `DEMO` or `fixture`. | None. | `NOT_RUN` |
| M1-32 | Unexecuted build, test, or lint work is never reported as successful. | Public-claim audit | `rg -n "M0|M1|M2|M3|M4|M5|M6|verified capture|implemented" README.md` | Initial audit found unsupported availability claims in the pre-Task-1 README. | `.superpowers/sdd/task-1-report.md` records the command output. | `FAIL` |

## Evidence Update Rules

- Keep one row per acceptance criterion from `QUANTUM_LOG_MASTER_PROMPT.md`.
- Record command output, fixture hashes, and artifact paths before changing a row to `PASS`.
- Use `FAIL` for executed checks that violate a criterion, `NOT_RUN` when no check has run, and `BLOCKED` when a prerequisite prevents execution.
- For `NOT_RUN` rows, name the future test but do not add a command that can pass without executing that test.
- Do not change `M1` from `BLOCKED` to `VERIFIED` until every required row is `PASS` and an independent review confirms the evidence.
