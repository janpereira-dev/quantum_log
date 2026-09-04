# Milestone Status

This index tracks delivery status. Acceptance evidence belongs in [`../verification/`](../verification/); plans, source code, and passing isolated tests do not independently make a milestone `VERIFIED`.

| Milestone | Current status | Evidence owner |
| --- | --- | --- |
| M0 | `NOT_VERIFIED` | [M0 evidence matrix](../verification/milestone-0-evidence.md) |
| M1 | `BLOCKED` | [M1 evidence matrix](../verification/milestone-1-evidence.md) |
| M2 | `IMPLEMENTED` | [M2 evidence matrix](../verification/milestone-2-evidence.md) |
| M3 | `IMPLEMENTED` | [M3 evidence matrix](../verification/milestone-3-evidence.md) |
| M4 | `IN_PROGRESS` | [M4 evidence](../verification/m4-evidence.md) |
| M5 | `IMPLEMENTED` | [M5 evidence matrix](../verification/milestone-5-evidence.md) |
| M6 | `IMPLEMENTED` | [M6 evidence matrix](../verification/milestone-6-evidence.md) |

## Status Vocabulary

- `IMPLEMENTED` means code exists; it is not runtime or release evidence.
- `READY_FOR_EXTERNAL_E2E` means the implementation can be exercised under the external acceptance protocol; it does not mean that protocol ran.
- `PASS` means matching local evidence exists for one criterion, command, candidate, and platform.
- `VERIFIED` requires a committed acceptance matrix and independent review, with every required criterion at `PASS` for the same candidate.

M1 remains `BLOCKED`, and M4 remains `IN_PROGRESS`. Neither the published RC10 artifacts nor passing repository tests promote either milestone by themselves.

## Verification Rule

Only full passing acceptance evidence can change a milestone to `VERIFIED`. Every required acceptance criterion must be `PASS`; any `FAIL`, `NOT_RUN`, or `BLOCKED` result prevents that transition. Record command output and required artifacts in the milestone's verification document before changing status.

## Evidence Ownership

- Maintainers update the relevant verification matrix when they run acceptance checks.
- Reviewers confirm the matrix contains complete passing evidence before approving a `VERIFIED` claim.
- Public documentation links to verified facts but remains separate from delivery evidence.


The final candidate audit at commit 3f09212eee520b63e424d7c3fb00f0758a5c1d8d is recorded in the M0, M2, M3, M5, and M6 matrices. It intentionally retains NOT_RUN and BLOCKED rows where current-candidate or independent evidence is missing. There is no stable-release GO.
