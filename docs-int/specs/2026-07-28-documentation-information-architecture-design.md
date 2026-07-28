# Documentation Information Architecture Design

## Goal

Make QUANTUM_LOG documentation complete, navigable, and trustworthy for both
users and maintainers. Public documentation must be bilingual. Milestone and
verification material must live outside the public learning path.

## Decisions

- Keep public documentation in `docs/`.
- Add mirrored English and Spanish trees under `docs/en/` and `docs/es/`.
- Provide one complete, task-oriented guide per language plus focused reference
  pages for the CLI, architecture, operations, privacy, and contribution.
- Retain ADRs as language-neutral normative records under `docs/architecture/`.
- Move milestones, verification evidence, release process, historical specs, and
  implementation plans into `docs-int/`.
- Add reciprocal language links and a bilingual root index.
- Document only capabilities supported by current code and verification evidence.

## Public Structure

```text
docs/
  README.md
  en/
    README.md
    guide.md
    cli-reference.md
    architecture.md
    operations.md
    privacy-security.md
    contributing.md
  es/
    README.md
    guia.md
    referencia-cli.md
    arquitectura.md
    operaciones.md
    privacidad-seguridad.md
    contribucion.md
  architecture/
    ADR-001-clean-core.md
    ADR-002-project-first-attribution.md
    ADR-003-local-ledger.md
    ADR-004-cooperative-sqlite-ownership.md
```

`guide.md` and `guia.md` are complete linear onboarding paths. Focused pages
are authoritative deep references and are linked from the guides rather than
duplicating every detail.

## Internal Structure

```text
docs-int/
  README.md
  milestones/
  verification/
  releases/
  specs/
  plans/
```

Internal documents retain evidence, historical decisions, and milestone state.
They are not used as public product documentation.

## CLI Reference Contract

Each CLI command group documents purpose, syntax, flags, examples, output or
failure behavior, and operational safety. The reference covers current Cobra
command groups, including ledger initialization, projects, ingest, reports,
pricing, tasks, anchors, maintenance, capture setup, collectors, adapters,
hooks, TUI, MCP, and process wrapper execution.

The first implementation is curated Markdown validated against command tests
and current Cobra definitions. A future generator may be introduced only after
the command descriptions and examples have stable source metadata.

## Safety and Maintenance

- Public pages must state capture quality and verification status explicitly.
- No page may claim real Copilot token capture before M4 evidence proves it.
- Docs must identify diagnostics that need a quiescent ledger.
- Language mirrors use equivalent paths and reciprocal links to reduce
  translation drift.
- Root `README.md` links into documentation instead of duplicating long guides.

## Validation

- Check every Markdown link locally.
- Compare public command examples with `qlog --help` and command tests.
- Confirm English and Spanish trees have equivalent page coverage.
- Preserve all ADRs and verification evidence during relocation.
- Run `git diff --check` after edits.
