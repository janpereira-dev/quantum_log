# Milestone Status

This index tracks delivery status. Acceptance evidence belongs in [`../verification/`](../verification/); plans, source code, and passing isolated tests do not independently make a milestone `VERIFIED`.

| Milestone | Current status | Evidence owner |
| --- | --- | --- |
| M0 | Historical work; reconfirmation required | Acceptance evidence was not preserved |
| M1 | `BLOCKED` | [M1 evidence matrix](../verification/milestone-1-evidence.md) |
| M2 | `IMPLEMENTED` | Milestone acceptance evidence |
| M3 | `IMPLEMENTED` | Milestone acceptance evidence |
| M4 | `IN_PROGRESS` | [M4 evidence](../verification/m4-evidence.md) |
| M5 | `IMPLEMENTED` | Milestone acceptance evidence |
| M6 | `IMPLEMENTED` | Milestone acceptance evidence |

## Verification Rule

Only full passing acceptance evidence can change a milestone to `VERIFIED`. Every required acceptance criterion must be `PASS`; any `FAIL`, `NOT_RUN`, or `BLOCKED` result prevents that transition. Record command output and required artifacts in the milestone's verification document before changing status.

## Evidence Ownership

- Maintainers update the relevant verification matrix when they run acceptance checks.
- Reviewers confirm the matrix contains complete passing evidence before approving a `VERIFIED` claim.
- Public documentation links to verified facts but remains separate from delivery evidence.
