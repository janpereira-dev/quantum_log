# Task 2 Report: Project Resolution and Location Selection

## Scope

Implemented only Task 2 from `docs/superpowers/plans/2026-07-19-m1-ledger-recovery.md`, lines 60-123.

## RED

Command:

```text
go test ./internal/attribution/resolver ./internal/storage/sqlite ./internal/cli -run "TestResolvePrecedence|TestProjectLocation|TestProjectCurrentSelectsLocation" -count=1
```

Output:

```text
--- FAIL: TestResolvePrecedence (0.00s)
    --- FAIL: TestResolvePrecedence/git_root_beats_registered_path_fallback (0.00s)
        resolver_test.go:78: Resolve() = resolver.ProjectResolution{ProjectSlug:"project-adapter", Method:"adapter", Confidence:"high", Evidence:"adapter project signal"}, want slug="project-a" method="git_root" confidence="high" evidence="c:/repos/a"
    --- FAIL: TestResolvePrecedence/registered_path_beats_adapter_hint (0.00s)
        resolver_test.go:78: Resolve() = resolver.ProjectResolution{ProjectSlug:"project-adapter", Method:"adapter", Confidence:"high", Evidence:"adapter project signal"}, want slug="project-a" method="registered_path" confidence="high" evidence="c:/repos/a"
FAIL
FAIL	github.com/janpereira-dev/quantum_log/internal/attribution/resolver	1.515s
--- FAIL: TestProjectLocationMatchesNormalizedResolvedPath (0.04s)
    store_test.go:64: projectByLocation("c:/users/cowbo/appdata/local/temp/testprojectlocationmatchesnormalizedresolvedpath3501792853/003/second") = project=domain.Project{ID:"", Slug:"", Name:"", CanonicalKey:"", CreatedAt:time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)} location=domain.ProjectLocation{ID:"", ProjectID:"", AbsolutePath:"", PathHash:"", CreatedAt:time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)} found=false, want project="f3187eb59c5cd702f54cf5d21e8c3fd0" location="5a217446711e6907ba58aed8661addd5"
FAIL
FAIL	github.com/janpereira-dev/quantum_log/internal/storage/sqlite	1.605s
--- FAIL: TestProjectCurrentSelectsLocationMatchingWorkingDirectory (0.27s)
    root_test.go:118: project current = struct { ProjectSlug string "json:\"project_slug\""; LocationID string "json:\"location_id\""; LocationPath string "json:\"location_path\""; Method string "json:\"method\""; Confidence string "json:\"confidence\""; Evidence string "json:\"evidence\"" }{ProjectSlug:"project", LocationID:"", LocationPath:"C:\\Users\\cowbo\\AppData\\Local\\Temp\\TestProjectCurrentSelectsLocationMatchingWorkingDirectory32106571\\002", Method:"cwd", Confidence:"high", Evidence:"c:/users/cowbo/appdata/local/temp/testprojectcurrentselectslocationmatchingworkingdirectory32106571/003"}
FAIL
FAIL	github.com/janpereira-dev/quantum_log/internal/cli	3.294s
FAIL
```

## GREEN

Target command:

```text
go test ./internal/attribution/resolver ./internal/storage/sqlite ./internal/cli -run "TestResolvePrecedence|TestProjectLocation|TestProjectCurrentSelectsLocation|TestProjectRegisterIsIdempotentForCurrentDirectory" -count=1
```

Output:

```text
ok  	github.com/janpereira-dev/quantum_log/internal/attribution/resolver	0.564s
ok  	github.com/janpereira-dev/quantum_log/internal/storage/sqlite	1.003s
ok  	github.com/janpereira-dev/quantum_log/internal/cli	3.310s
```

CGo-free repository command (Windows `cmd.exe` environment syntax):

```text
set "CGO_ENABLED=0" && go test -count=1 ./... && git diff --check
```

Output:

```text
?   	github.com/janpereira-dev/quantum_log/cmd/qlog	[no test files]
ok  	github.com/janpereira-dev/quantum_log/internal/adapters	0.866s
?   	github.com/janpereira-dev/quantum_log/internal/app	[no test files]
ok  	github.com/janpereira-dev/quantum_log/internal/attribution/resolver	0.893s
ok  	github.com/janpereira-dev/quantum_log/internal/audit	0.831s
ok  	github.com/janpereira-dev/quantum_log/internal/capture/wrapper	2.249s
ok  	github.com/janpereira-dev/quantum_log/internal/cli	4.546s
?   	github.com/janpereira-dev/quantum_log/internal/config	[no test files]
ok  	github.com/janpereira-dev/quantum_log/internal/distribution	0.808s
?   	github.com/janpereira-dev/quantum_log/internal/domain	[no test files]
ok  	github.com/janpereira-dev/quantum_log/internal/ingest/jsonl	2.151s
ok  	github.com/janpereira-dev/quantum_log/internal/ingest/otlp	2.751s
ok  	github.com/janpereira-dev/quantum_log/internal/mcpserver	3.033s
ok  	github.com/janpereira-dev/quantum_log/internal/pricing	0.844s
ok  	github.com/janpereira-dev/quantum_log/internal/storage/sqlite	1.940s
ok  	github.com/janpereira-dev/quantum_log/internal/tui	0.931s
```

`git diff --check` completed with exit code 0 and no output.

## Changed Files

- `internal/attribution/resolver/resolver.go`: applies explicit, environment, exact CWD, exact Git root, longest registered path, adapter, unattributed ordering.
- `internal/attribution/resolver/resolver_test.go`: verifies every resolution source, method, confidence, and evidence.
- `internal/storage/sqlite/store.go`: normalizes location matching and exposes path-specific project lookup.
- `internal/storage/sqlite/store_test.go`: verifies a normalized resolution path returns the second location registered for the same project.
- `internal/app/service.go`: uses path-specific lookup for CWD, Git-root, and registered-path resolutions; returns selected location path.
- `internal/cli/root.go`: uses the service's resolved location instead of re-querying by slug.
- `internal/cli/root_test.go`: verifies `project current --json` emits matching location ID/path and register idempotency for `--path .`.
- `.superpowers/sdd/task-2-report.md`: this report.

## Concerns

- No Git remote support added. It requires a separate persisted registration contract, outside Task 2.
- Existing Task 1 documentation modifications and untracked recovery-plan artifacts were present before this work and were not changed.
- The portable CGo-free command uses Windows `cmd.exe` syntax; equivalent POSIX form remains `CGO_ENABLED=0 go test -count=1 ./...`.
