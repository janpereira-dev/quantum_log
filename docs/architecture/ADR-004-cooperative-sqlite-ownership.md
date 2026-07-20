# ADR-004: Cooperative SQLite ownership and quiescent verification

Status: accepted

`qlog.db` is private QUANTUM_LOG storage. Every official process that opens the
database participates in a cross-process quiescence lock. Normal readers and
writers take quiescence in shared mode. Mutating operations also take writer lock
exclusively. Locks are always acquired in this order: quiescence, then writer.

`qlog doctor` and `qlog verify` take exclusive quiescence. They validate a
quiescent database, not a live database. After exclusive quiescence is acquired,
they block on a non-empty WAL, warn without changing an isolated SHM file, and
open SQLite with `mode=ro&immutable=1`. They never create or remove files, run
migrations or checkpoints, modify SQLite sidecars, update anchors, or repair
state.

SQLite writers outside QUANTUM_LOG that bypass these locks are unsupported. Users
must use `qlog export`, `qlog snapshot`, or `qlog backup` for external readers.
Maintenance commands own checkpoint, recovery, and anchor rebuild explicitly.

This does not protect against a process that deliberately changes the database,
WAL, SHM, and anchor while bypassing all QUANTUM_LOG locks.
