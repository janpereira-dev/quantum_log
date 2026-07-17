# ADR-001: Keep domain independent of delivery and storage

Status: accepted

QUANTUM_LOG uses a dependency direction of CLI -> application -> domain/resolver -> infrastructure. Cobra and SQLite stay outside domain contracts, so future TUI, MCP, and adapters can reuse the same rules.
