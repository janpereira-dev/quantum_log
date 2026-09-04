# M6 Evidence Matrix — final candidate audit

**Candidate:** `3f09212eee520b63e424d7c3fb00f0758a5c1d8d` (2026-09-04). **Reviewer:** independent review not yet recorded.

| AC ID | Criterion | Exact command | Platform | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M6-01 | Versioned MCP tools enforce input/output, request-id, and idempotency contracts | `go test -count=1 ./internal/mcpserver -run 'Schema|Idempot|Request' -v` | Windows, macOS, Linux | Current allocation integration is still being repaired; not run | Build/acceptance receipt unavailable | `BLOCKED` |
| M6-02 | Task lifecycle correlates project, WorkContext, ingestion, completion, budgets, and verification | `go test -count=1 ./internal/mcpserver ./internal/cli -run 'Task|Lifecycle|Correlation' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M6-03 | Partial data and unavailable metrics remain explicit; no values are invented | `go test -count=1 ./internal/mcpserver ./internal/storage/sqlite -run 'Partial|Unavailable|Non.?Invention' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M6-04 | Allocation correction and revert are auditable through the MCP boundary | `go test -count=1 ./internal/mcpserver ./internal/storage/sqlite -run 'Allocation|Revision|Revert' -v` | Windows, macOS, Linux | Current integration is not independently verified | Allocation implementation evidence is incomplete | `BLOCKED` |

M6 remains `IMPLEMENTED`, not `VERIFIED`.
