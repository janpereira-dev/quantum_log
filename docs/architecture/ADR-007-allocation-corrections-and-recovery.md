# ADR-007: Append-only allocation corrections and recovery

Status: accepted
Date: 2026-09-04

## Decision

Quantum Log uses design A: append-only allocation revision rows with
`usage_allocations` retained as a rebuildable current-state projection. A
correction never edits or deletes a historical revision. A revert is a new
revision whose entries restore the selected revision's parent state.

## Contract and invariants

- A revision has an immutable `revision_id`, monotonically increasing
  `revision_number`, optional `parent_revision_id`, idempotency key, author,
  source, reason, and UTC creation time.
- Revision entries are immutable and basis points are in `[0,10000]`; a
  non-empty allocation revision totals exactly 10000 basis points.
- Repeating an idempotency key for the same subject returns the existing
  revision and does not add rows. Trace and span identifiers are metadata only.
- The current `usage_allocations` table is a projection, never an authority.
  It may be replaced transactionally from the latest revision, but revision
  history is not modified. `RebuildAllocationProjection` deterministically
  rebuilds it from revision rows.
- Crash recovery is transaction-bound: a revision and its projection update
  commit together or neither is visible. A failed transaction leaves the
  previous projection and history unchanged.
- Anchor/checkpoint operations remain outside the allocation transaction. A
  database/anchor mismatch is reported as recovery-required; it is never
  repaired by changing historical allocations.
- Migration 014 backfills existing projection rows as a `migration` revision.
  It does not rewrite or remove the legacy rows. Rollback after publication is
  a forward migration; migration 014 is never deleted from an initialized DB.

## Bounded design evaluation

| Criterion | A: revision rows + projection | B: raw ledger events + rebuilt projection |
|---|---|---|
| Transactionality | Strong, one SQLite transaction | Depends on event/projection coupling |
| Auditability | Explicit typed history and parent links | Requires decoding every event |
| Query cost | Current reports remain indexed | Every report rebuilds or caches events |
| Recovery determinism | Revision ordering is explicit | Event ordering and schema versions add ambiguity |
| Migration risk | Additive table and backfill | Reinterpret existing raw-event payloads |
| Compatibility | Existing projection readers continue to work | All readers require a new event interpreter |

Design A is accepted because it preserves existing reporting compatibility
while making corrections and recovery auditable. Design B remains rejected for
this release because it increases migration and recovery surface without a
measured benefit.

## Acceptance scenarios

The executable coverage is in
`internal/storage/sqlite/allocation_history_test.go`: assign then correct and
inspect history; correct then revert; duplicate idempotency; projection
rebuild; legacy migration; and failed transaction preservation. Concurrent
writers use the existing writer lock and SQLite transaction boundary; the
concurrent writers scenario is therefore deterministic and idempotent.

## Recovery and rollback

Recovery first verifies the revision chain and then rebuilds the projection.
No recovery command may hard-delete a revision. Before migration 014 is
applied, the normal database backup is the rollback boundary. After it is
applied, compatibility is maintained by forward migrations only.
