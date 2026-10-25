# QUANTUM_LOG documentation

Quantum Log imports existing local Codex session files, reports token usage,
and exports aggregate CSV. Each command finishes and exits.

## Choose a language

- [English](en/README.md)
- [Español](es/README.md)

## Current product

- [English guide](SIMPLE.md): build, import, report, export, and privacy.
- [Guía en español](es/simple.md): uso, almacenamiento y límites.
- [Contributing](../CONTRIBUTING.md): development and checks.
- [ADR-008](architecture/ADR-008-on-demand-product.md): why the product runs on demand.

The current source executable is `cmd/qlog`. Costs are unavailable, and usage
sample counts are not model-call counts.

## Technical and historical reference

- [Historical product documentation](legacy/README.md): earlier installers,
  collectors, adapters, and command set under `cmd/qlog-legacy`.
- [Architecture decision records](architecture/).
- [Dated local validation](verification/simple-local-ledger.md).
- [Internal delivery evidence, milestones, releases, and plans](../docs-int/README.md).

Historical integration tests and milestone status describe the builds named in
those records. They do not establish that a new build or release is verified.
