# M5 Evidence Matrix — final candidate audit

**Candidate:** `3f09212eee520b63e424d7c3fb00f0758a5c1d8d` (2026-09-04). **Reviewer:** independent review not yet recorded.

| AC ID | Criterion | Exact command | Platform | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M5-01 | Archives, version metadata, checksums, SBOM, and provenance are produced | `goreleaser check; goreleaser release --snapshot --clean` | Release host; Windows, macOS, Linux targets | Not run against this candidate | RC10 evidence is stale | `NOT_RUN` |
| M5-02 | Install, version, init, doctor, update policy, uninstall, and data preservation work on clean environments | `go test -count=1 ./internal/distribution -run 'Lifecycle|Install|Uninstall' -v` plus clean-device runs | Windows, macOS, Linux, WSL | Not run against this candidate | No current-candidate clean-device receipt | `NOT_RUN` |
| M5-03 | Published artifacts are independently authenticated and immutable | Release authenticity verifier against published candidate | Release host | No current stable publication | Immutable-tag ruleset exists, but does not prove this candidate | `BLOCKED` |

M5 remains `IMPLEMENTED`, not `VERIFIED`; no stable-release GO is recorded.
