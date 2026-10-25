# QUANTUM_LOG documentation

Quantum Log reads the Codex session files already on your computer and shows
their reported token usage by project. You import when you want to refresh
the report; no background process is required.

[Leer en español](../es/README.md)

## Start here

- [Current guide](../SIMPLE.md): build, try synthetic data, import, report, and export.
- [Project overview](../../README.md).
- [Security policy](../../SECURITY.md).

The three everyday commands are `qlog import`, `qlog report`, and `qlog export`.
Running `qlog` shows the report. Costs are unavailable, and usage samples are
not model-call counts.

## Contributors

- [Contributing](../../CONTRIBUTING.md): current packages, local checks, and review.
- [Current product decision](../architecture/ADR-008-on-demand-product.md).
- [Architecture decision records](../architecture/): normative technical decisions.

## Earlier product

The earlier capture-oriented implementation is preserved in `cmd/qlog-legacy`.
Its [guide](guide.md), [CLI reference](cli-reference.md),
[architecture](architecture.md), [operations](operations.md),
[privacy and security](privacy-security.md), and [contributor notes](contributing.md)
are historical reference. See the [historical index](../legacy/README.md) for
the complete set. These pages are not the current setup flow.
