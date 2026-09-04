# M3 Evidence Matrix — final candidate audit

**Candidate:** `3f09212eee520b63e424d7c3fb00f0758a5c1d8d` (2026-09-04). **Reviewer:** independent review not yet recorded.

| AC ID | Criterion | Exact command | Platform | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M3-01 | TUI and CLI consume the same query contract and totals | `go test -count=1 ./internal/cli ./internal/tui -run 'Parity|Query|Total' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M3-02 | Terminal-size, compact, color, and keyboard behavior are covered by goldens/interactions | `go test -count=1 ./internal/tui -run 'Golden|Terminal|Interaction' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M3-03 | Quality labels and unavailable measurements are not invented in views | `go test -count=1 ./internal/tui ./internal/cli -run 'Quality|Unavailable' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |

M3 remains `IMPLEMENTED`, not `VERIFIED`.
