# M0 Evidence Matrix — final candidate audit

**Candidate:** repository `HEAD` `3f09212eee520b63e424d7c3fb00f0758a5c1d8d` (2026-09-04).  **Reviewer:** independent review not yet recorded.

M0 has no current-candidate acceptance run. Historical notes are not promoted.

| AC ID | Criterion | Exact command | Platform | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M0-01 | Baseline repository and build contracts are reproducible | `go mod tidy; git diff --exit-code go.mod go.sum; go build ./...` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M0-02 | Privacy and repository hygiene hold before release | `go test -count=1 ./internal/distribution -run 'Test(RepositoryHygiene|ReleaseDocumentation)' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M0-03 | Release gate has independent evidence | `goreleaser check` plus independent review of candidate artifacts | Release host | No independent review recorded | Prior RC evidence is stale for this candidate | `BLOCKED` |

M0 is not `VERIFIED`; any `NOT_RUN` or `BLOCKED` row prevents that status.
