# Documentation Information Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fragmented documentation with bilingual public guides and a separate internal documentation area.

**Architecture:** Public pages live in mirrored `docs/en/` and `docs/es/` trees. Each tree provides one linear guide and focused reference pages. Architecture decisions remain language-neutral. Milestone, evidence, release, specification, and planning material moves under `docs-int/`.

**Tech Stack:** Markdown, repository-relative links, Cobra CLI source and tests as command-reference evidence.

## Global Constraints

- Keep English and Spanish page trees structurally equivalent.
- Preserve explicit capture-quality and milestone-verification claims.
- Do not claim real Copilot capture without evidence in `docs-int/verification/m4-evidence.md`.
- Keep ADRs in `docs/architecture/` as normative records.
- Do not add a documentation generator in this change; curate the first reference from current Cobra definitions.
- Validate links and run `git diff --check` before completion.

---

### Task 1: Create public navigation and document boundaries

**Files:**
- Create: `docs/README.md`
- Create: `docs/en/README.md`
- Create: `docs/es/README.md`
- Modify: `README.md`
- Modify: `docs/DEVELOPER_GUIDE.md`

**Interfaces:**
- Consumes: Existing root README and developer guide.
- Produces: Stable public entry points used by every public documentation page.

- [ ] **Step 1: Define public entry points**

Create a bilingual root index linking to English and Spanish trees, ADRs, and the internal documentation boundary. Create language indexes with reciprocal language links and audience routes for users and contributors.

- [ ] **Step 2: Replace duplicate navigation in legacy files**

Make root `README.md` a concise product overview that links to `docs/README.md`. Replace `docs/DEVELOPER_GUIDE.md` with a compatibility redirect to `docs/en/contributing.md` until legacy links can be retired.

- [ ] **Step 3: Defer full link validation until all public pages exist**

Do not validate local Markdown targets yet: the language indexes deliberately
link to pages created by Tasks 2 and 3. Task 5 validates the completed tree.

### Task 2: Write complete English public documentation

**Files:**
- Create: `docs/en/guide.md`
- Create: `docs/en/cli-reference.md`
- Create: `docs/en/architecture.md`
- Create: `docs/en/operations.md`
- Create: `docs/en/privacy-security.md`
- Create: `docs/en/contributing.md`

**Interfaces:**
- Consumes: Cobra command definitions in `internal/cli/`, ADRs, and internal verification evidence.
- Produces: Complete English user and maintainer documentation.

- [ ] **Step 1: Write linear guide**

Document installation, ledger initialization, project registration, ingestion, verification, capture setup, usage reports, troubleshooting, and next references. Link deeper details instead of repeating safety rules.

- [ ] **Step 2: Write full CLI reference**

Cover command groups and subcommands: `init`, `doctor`, `verify`, `maintenance`, `project`, `ingest`, `usage`, `report`, `allocation`, `pricing`, `task`, `export`, `anchor`, `setup`, `collector`, `adapter`, `hook`, `run`, `tui`, and `mcp`. Include syntax, purpose, safety notes, examples, expected output or failure states, and capture-quality limits.

- [ ] **Step 3: Write operations and governance pages**

Explain layers, project resolution, event lifecycle, locking, diagnostics, backup and recovery boundaries, sanitization, external anchors, threat model, contributor commands, release handoff, and evidence policy.

- [ ] **Step 4: Verify command claims**

Run:

```powershell
go run ./cmd/qlog --help
go run ./cmd/qlog adapter --help
go run ./cmd/qlog collector --help
```

Expected: documented command groups are present and descriptions match source behavior.

### Task 3: Mirror public content in Spanish

**Files:**
- Create: `docs/es/guia.md`
- Create: `docs/es/referencia-cli.md`
- Create: `docs/es/arquitectura.md`
- Create: `docs/es/operaciones.md`
- Create: `docs/es/privacidad-seguridad.md`
- Create: `docs/es/contribucion.md`

**Interfaces:**
- Consumes: English public page structure and verified facts.
- Produces: Equivalent Spanish navigation and content with reciprocal page links.

- [ ] **Step 1: Mirror page hierarchy**

Create exactly one Spanish counterpart for each English public page. Link every paired page to its counterpart.

- [ ] **Step 2: Translate technical guidance accurately**

Translate explanatory prose while retaining command names, flags, paths, capture-quality labels, and code blocks unchanged.

- [ ] **Step 3: Compare structural coverage**

Run:

```powershell
(Get-ChildItem docs/en -Filter *.md).Count
(Get-ChildItem docs/es -Filter *.md).Count
```

Expected: matching counts after excluding only intentionally language-specific filenames.

### Task 4: Move internal material to `docs-int/`

**Files:**
- Create: `docs-int/README.md`
- Move: verification materials to `docs-int/verification/`
- Move: release materials to `docs-int/releases/`
- Move: historical specifications to `docs-int/specs/`
- Move: implementation plans to `docs-int/plans/`
- Create: `docs-int/milestones/README.md`

**Interfaces:**
- Consumes: Existing evidence, release procedure, specs, and plans.
- Produces: Internal-only route for delivery history and proof.

- [ ] **Step 1: Relocate internal documents without content loss**

Move existing internal directories. Update all repository-relative links pointing to old locations.

- [ ] **Step 2: Add milestone index**

Create `docs-int/milestones/README.md` describing M0-M6 status source, ownership of evidence, and the rule that only passing acceptance evidence can change a milestone to `VERIFIED`.

- [ ] **Step 3: Check internal links**

Run:

```powershell
Get-ChildItem docs-int -Recurse -Filter *.md | Select-String -Pattern 'docs/(verification|releases|superpowers)'
```

Expected: no stale public-path references remain.

### Task 5: Validate documentation integrity

**Files:**
- Modify: all affected Markdown links only when validation reveals an invalid target.

**Interfaces:**
- Consumes: public and internal documentation trees.
- Produces: a clean, link-valid documentation change.

- [ ] **Step 1: Validate all local Markdown targets**

Run a PowerShell link-validation script that scans `docs/`, `docs-int/`, and `README.md`, ignores external URLs and anchors, and fails for missing Markdown targets.

- [ ] **Step 2: Check formatting and tree state**

Run:

```powershell
git diff --check
```

Expected: no whitespace errors; changed paths limited to documentation structure and corrected links.

- [ ] **Step 3: Review public claims against evidence**

Compare capture, privacy, and milestone language in both guides against `docs-int/verification/m4-evidence.md` and ADR-004. Expected: M4 remains `IN_PROGRESS`; Copilot remains experimental without real E2E evidence.
