# Task 1 Report: Milestone Status and Evidence Scaffolding

## Scope

Completed only Task 1 from `docs/superpowers/plans/2026-07-19-m1-ledger-recovery.md`.
No production Go source was edited. No commit was created.

## Final Lifecycle and Evidence State

- M0: `IMPLEMENTED`; acceptance matrix absent.
- M1: `BLOCKED`; 31 rows are `NOT_RUN` and M1-32 is `FAIL`.
- M2: `IMPLEMENTED`; acceptance matrix absent.
- M3: `IN_PROGRESS`; acceptance matrix absent.
- M4: `IN_PROGRESS`; acceptance matrix absent and capture maturity is `DETECTION_ONLY`.
- M5: `IMPLEMENTED`; acceptance matrix absent.
- M6: `IMPLEMENTED`; acceptance matrix absent.

`IMPLEMENTED` records audited delivery stage, not verification. `VERIFIED` remains
prohibited until a milestone has complete passing acceptance evidence.

## Preserved Initial Audit

Task 1 initially found a README claim that M0-M6 were implemented with verified
capture/distribution but no recorded acceptance evidence. M1-32 preserves that
failure as audit history even though the public claim has since been removed.

## Changes

- Created `docs/verification/milestone-1-evidence.md` with one executable-evidence
  row for each M1 criterion in `QUANTUM_LOG_MASTER_PROMPT.md`.
- Replaced README's blanket M0-M6 implementation claim with audited states,
  lifecycle definitions, matrix rules, and a recovery sequence that is not a
  capability claim.
- Added the same state and evidence contract to `QUANTUM_LOG_MASTER_PROMPT.md`.
- Replaced inherited `ledger VERIFIED` and capture-exact display examples with
  `PENDING EVIDENCE` / `EXAMPLE ONLY` language.
- Marked the retained historical prompt snapshot as non-verifiable requirements,
  not implementation or verification evidence.

## Command Evidence

| Command | Result |
|---|---|
| `rg -n "M0|M1|M2|M3|M4|M5|M6|verified capture|implemented" README.md` before edits | Found unsupported claims at `README.md:9` (M0-M6 implemented, verified distribution) and `README.md:70` (verified capture adapters). |
| `rg -c "^\\| M1-[0-9][0-9] \\|" docs/verification/milestone-1-evidence.md` | `32` rows. |
| `rg -n "VERIFIED|CAPTURE_VERIFIED|Milestone" README.md QUANTUM_LOG_MASTER_PROMPT.md docs/verification/milestone-1-evidence.md` after edits | Remaining `VERIFIED` references are lifecycle/evidence rules or M0's explicit non-verified state; no M1-M6 capability is declared verified. |
| `rg -n -i "ledger verified|verified capture|capture_verified" README.md QUANTUM_LOG_MASTER_PROMPT.md docs/verification/milestone-1-evidence.md` | No stale public capability claim remains. The only match is the literal audit search expression in M1-32. |
| `git diff --check` | Exit 0; no whitespace errors. |

## Not Run

No Go build, test, CGo-free test, migration, CLI, fixture, CI, or release command
was run. Task 1 establishes the matrix that later tasks must populate with actual
command output and artifacts.

## Open Concerns

- M1 cannot move from `BLOCKED` to `VERIFIED` while M1-32 is `FAIL` and the other
  required rows are `NOT_RUN`.
- Existing untracked `.superpowers/` and `docs/superpowers/` content predated this
  task. This task adds only `.superpowers/sdd/task-1-report.md` and
  `docs/verification/milestone-1-evidence.md` within those untracked trees.

## Historical Review Fixes

The lifecycle wording previously recorded here is obsolete. The authoritative Task 1
state is `Final Lifecycle and Evidence State` and `Final Review-Finding Fixes` below;
they retain `IMPLEMENTED`, `IN_PROGRESS`, and `BLOCKED` lifecycle values separately
from matrix-absence evidence status.

The README now labels its TUI, MCP, task, allocation, pricing, reporting, export,
repair, and budget text as unaudited source inventory rather than supported
behavior. M1-27 through M1-31 now include exit-status assertions and explicit
expected outcomes. M1-30 uses a PowerShell no-match assertion: no forbidden path
returns exit 0; any match returns exit 1. This avoids CMD parsing of regex
alternation.

### Review-Fix Command Evidence

| Command | Result |
|---|---|
| `rg -n "qlog tui|qlog mcp|qlog task|qlog pricing|qlog export|qlog budget" README.md` | Re-run after the edit; no unqualified feature-description line remains. |
| `rg -n "IMPLEMENTED|Inventario de fuente|Unaudited source inventory" README.md QUANTUM_LOG_MASTER_PROMPT.md` | Re-run after the edit; M0, M2, M5, and M6 are inventory labels, not implementation claims. |
| `rg -n "M1-2[7-9]|M1-3[0-1]" docs/verification/milestone-1-evidence.md` | Re-run after the edit; each row contains an assertion command and an explicit expected outcome. |
| `git diff --check` | Re-run after the edit; result recorded after final verification. |

### Actual Review-Fix Outputs

- `rg -n "qlog tui|qlog mcp|qlog task|qlog pricing|qlog export|qlog budget" README.md` returned no matches.
- The inventory-state search returned `Unaudited source inventory` for M0, M2,
  M5, and M6 in the README and `Inventario de fuente sin auditar` for the same
  milestones in the master prompt. The only remaining `IMPLEMENTED` occurrences
  describe permitted lifecycle vocabulary.
- The M1-27 through M1-31 search returned all five rows with assertion commands
  and expected exit outcomes.
- The M1-30 PowerShell path-name assertion exited 0 with no output. It proves only
  that its forbidden tracked-path patterns did not match; M1-30 remains `NOT_RUN`
  until a complete repository hygiene audit is recorded.
- `git diff --check` exited 0 with no output.

## Remaining Review-Finding Fixes

The milestone tables now separate exact lifecycle states from evidence status:
M0, M2, M5, and M6 are `IMPLEMENTED` audited delivery stages with matrices absent;
M3 and M4 are `IN_PROGRESS`; M1 is `BLOCKED`. M4 retains `DETECTION_ONLY` only as
capture maturity. This does not claim verification for any matrix-absent milestone.

The README marks command examples and all subsequent behavior descriptions as
unverified source inventory while M1 remains `BLOCKED`, and its resolution order now
places adapter hint before `unattributed`.

M1-03 through M1-10 and M1-12 through M1-26 now name future test identifiers,
state that they are not executed, and contain no `go test -run` command that could
pass without matching a test. Final command outputs are recorded after verification.

## Final Review-Finding Fixes

This section supersedes the earlier inventory-only lifecycle wording. Both milestone
tables now use only exact lifecycle values: M0, M2, M5, and M6 are `IMPLEMENTED`;
M1 is `BLOCKED`; M3 and M4 are `IN_PROGRESS`. A separate evidence-status column
records matrix absence. M4's `DETECTION_ONLY` remains capture maturity only.

The README qualifies its command examples and every later behavior description as
unverified source inventory while M1 is `BLOCKED`. Its resolution order now places
adapter hint before `unattributed`.

### Final Review-Fix Command Outputs

- Lifecycle-state search returned only `IMPLEMENTED`, `IN_PROGRESS`, and `BLOCKED`
  values for M0-M6 in both tables; matrix absence is in the separate evidence column.
- README search found the unverified-inventory qualification, adapter-hint precedence,
  and unverified command-example heading.
- Search for `go test -run` in M1-03..10 and M1-12..26 returned no matches.
- `rg -c "Future test ID" docs/verification/milestone-1-evidence.md` returned `23`.
- `git diff --check` exited 0 with no output.

## Task Closure Validation

- After reconciling final lifecycle state and marking Task 1 complete,
  `git diff --check` exited 0 with no output.
