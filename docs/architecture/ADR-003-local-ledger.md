# ADR-003: Use an append-only local SQLite ledger

Status: accepted

Raw events are stored locally in SQLite with embedded migrations and a SHA-256 chain per source/session. This proves local tampering detection; it is not a blockchain or distributed consensus mechanism.
