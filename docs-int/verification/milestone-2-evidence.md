# M2 Evidence Matrix — final candidate audit

**Candidate:** `3f09212eee520b63e424d7c3fb00f0758a5c1d8d` (2026-09-04). **Reviewer:** independent review not yet recorded.

| AC ID | Criterion | Exact command | Platform | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M2-01 | Grouped usage totals preserve observed totals without duplication | `go test -count=1 ./internal/storage/sqlite ./internal/cli -run 'Usage|Report' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M2-02 | Allocation corrections are append-only, bounded, historical, and reversible | `go test -count=1 ./internal/storage/sqlite -run 'Allocation|Revision|Revert' -v` | Windows, macOS, Linux | Current implementation changed; full acceptance not run | Allocation work is not independently reviewed | `BLOCKED` |
| M2-03 | Pricing and scaled-cost arithmetic are temporal and deterministic | `go test -count=1 ./internal/storage/sqlite -run 'Pricing|Cost' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M2-04 | JSON, NDJSON, and CSV exports sanitize paths and spreadsheet cells | `go test -count=1 ./internal/storage/sqlite ./internal/cli -run 'Export|Sanit' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |

M2 remains `IMPLEMENTED`, not `VERIFIED`.
