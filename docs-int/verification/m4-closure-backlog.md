# M4 Closure Backlog

P0-01 audit completed on 2026-08-04. Release candidate is `v0.3.2-rc.1`. This is the only candidate supported by current repository evidence: the default binary version is `0.3.2`, while no local `v0.3.2*` tag exists. M4 remains `IN_PROGRESS`; this ledger does not claim runtime proof.

## Release Baseline

| Topic | Observed evidence |
|---|---|
| RC source of truth | `cmd/qlog/main.go` sets default version to `0.3.2`; `.goreleaser.yaml` injects the tag version with `-X main.version={{ .Version }}`. Candidate: `v0.3.2-rc.1`. |
| Branch and base | Current branch: `feat/m4-auto-capture` at `520acbf7ddb5e2b56fa8779be02bc3d2b613c305`. `origin/main` is an ancestor at `6ffdbaccdf5f381645168b56462cb9189c0ff89a`; `origin/main...HEAD` reports `0 45`. Current branch tracks `origin/feat/m4-auto-capture` and is ahead by 3 commits. |
| Tags | Local tags: `v0.1.0`, `v0.2.0`, `v0.3.0`, `v0.3.1`. No local `v0.3.2*` tag. |
| Release workflow | `.github/workflows/release.yml` releases on pushed `v*` tags through GoReleaser and uploads a Cosign checksum bundle. No release action was run. |
| Public metadata drift | `CHANGELOG.md` latest released section is `0.2.0`; `README.md` quick start installs `v0.3.0`; npm package and installer hard-code `0.1.0`. Homebrew, Scoop, AUR, and Winget templates use `{{VERSION}}`. |
| Working tree at audit start | Uncommitted modifications existed in `internal/cli/setup.go` and `internal/cli/setup_test.go`. They are outside P0-01 and remain unstaged. |

## Status Vocabulary

Allowed values: `NOT_STARTED`, `IN_PROGRESS`, `BLOCKED`, `COMPLETE`.

## P0 Ledger

| ID | Status | Change made | Validation command | Result | Evidence | Remaining blockers |
|---|---|---|---|---|---|---|
| P0-01 | COMPLETE | Created this audit ledger only. No product, installer, package, tag, remote, or workflow change. | `git status --short`; `git branch -vv`; `git log --oneline origin/main..HEAD`; `git tag --sort=-version:refname`; `git diff --check`; repository reads of release metadata. | Audit completed. Working tree had two pre-existing modified setup files; no local `v0.3.2*` tag; candidate resolves to `v0.3.2-rc.1`. | `cmd/qlog/main.go:11`; `.goreleaser.yaml:16-17`; `CHANGELOG.md:3`; `README.md:55`; `packaging/npm/package.json:3`; `packaging/npm/scripts/install.js:9-10`; `.github/workflows/release.yml:3-33`; `docs-int/verification/m4-evidence.md:7`; `internal/adapters/adapters_test.go:64`. | Runtime proof for Codex CLI and GitHub Copilot VS Code is not recorded. GitHub Copilot CLI is explicitly unavailable without separate official telemetry evidence and is absent from current stable adapter set. Release metadata versions remain inconsistent. |
| P0-02 | BLOCKED | None. No P0-02 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-02 scope and acceptance evidence. |
| P0-03 | BLOCKED | None. No P0-03 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-03 scope and acceptance evidence. |
| P0-04 | BLOCKED | None. No P0-04 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-04 scope and acceptance evidence. |
| P0-05 | BLOCKED | None. No P0-05 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-05 scope and acceptance evidence. |
| P0-06 | BLOCKED | None. No P0-06 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-06 scope and acceptance evidence. |
| P0-07 | BLOCKED | None. No P0-07 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-07 scope and acceptance evidence. |
| P0-08 | BLOCKED | None. No P0-08 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-08 scope and acceptance evidence. |
| P0-09 | BLOCKED | None. No P0-09 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-09 scope and acceptance evidence. |
| P0-10 | BLOCKED | None. No P0-10 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-10 scope and acceptance evidence. |
| P0-11 | BLOCKED | None. No P0-11 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-11 scope and acceptance evidence. |
| P0-12 | BLOCKED | None. No P0-12 task definition found in repository evidence audited. | `git log --all -S "P0-01"`; repository-wide search for `P0-01` through `P0-12`. | No P0 task definitions found. | Searches returned no P0 task rows or task scope. | Define P0-12 scope and acceptance evidence. |
