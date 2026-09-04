# M6 Evidence Matrix — final candidate audit

**Frozen source candidate:** `f651232104596a0188a1a61bc92434b7c5139cb3` (2026-09-04). This reconciliation is documentation-only and does not alter source bytes. **Reviewer:** independent review not yet recorded.

| AC ID | Criterion | Exact command | Platform | Result | Evidence | State |
|---|---|---|---|---|---|---|
| M6-01 | Versioned MCP tools enforce input/output, request-id, and idempotency contracts | `go test -count=1 ./internal/mcpserver -run 'Schema|Idempot|Request' -v` | Windows, macOS, Linux | Focused current-host run not recorded for all platforms | No cross-platform current-candidate receipt | `NOT_RUN` |
| M6-02 | Task lifecycle correlates project, WorkContext, ingestion, completion, budgets, and verification | `go test -count=1 ./internal/mcpserver ./internal/cli -run 'Task|Lifecycle|Correlation' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M6-03 | Partial data and unavailable metrics remain explicit; no values are invented | `go test -count=1 ./internal/mcpserver ./internal/storage/sqlite -run 'Partial|Unavailable|Non.?Invention' -v` | Windows, macOS, Linux | Not run against this candidate | No current-candidate receipt | `NOT_RUN` |
| M6-04 | Allocation correction and revert are auditable through the MCP boundary | `go test -count=1 ./internal/mcpserver ./internal/storage/sqlite -run 'Allocation|Revision|Revert' -v` | Current host (Windows) | **PASS**: MCP allocation guard and append-only revision, revert, idempotency, projection rebuild, grouping, and validation tests passed (3.2s). | Test output recorded against the frozen source candidate; independent cross-platform review still required | `PASS` |

M6 remains `IMPLEMENTED`, not `VERIFIED`.
