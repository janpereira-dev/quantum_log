# Quantum Log Product Finalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish Quantum Log as a stable, privacy-safe, cross-platform Go product whose install, upgrade, real-agent capture, ledger, uninstall, and release claims are all backed by reproducible end-to-end evidence.

**Architecture:** Keep the Go ledger and CLI as the product core and treat collectors, hooks, plugins, installers, and agent integrations as replaceable boundary adapters. Work proceeds in independent, reviewable phases: first make cleanup and evidence trustworthy, then prove artifact lifecycle on all supported operating systems, then close real-agent and ledger-contract gaps, and only then publish a stable release. TokenTab is a competitor/reference point only: its behavior is not equivalent to Quantum Log's append-only, local-first contract, its installer/runtime assumptions are not accepted as safe, and it MUST NOT become a dependency or justify a rewrite.

**Tech Stack:** Go 1.26.6, SQLite (CGO-free), Cobra, POSIX `sh`, PowerShell 7/Windows PowerShell 5.1, GitHub Actions, GoReleaser v2, Syft, Cosign/Sigstore.

## Global Constraints

- Go remains the core language; a rewrite is out of scope unless a measured, documented blocker proves Go cannot meet a stable-release exit criterion.
- The ledger is authoritative, append-only, local-first, offline-capable, privacy-by-default, and usable with telemetry disabled.
- Prompt bodies, response bodies, file contents, tool arguments/results, credentials, authorization values, and environment values remain excluded by default.
- Cleanup is fail-closed: never delete `.qlog/qlog.db`, its sidecars, anchors, backups, or an unverified home path.
- Every installer test uses an explicit version; no test may silently resolve `latest` or a moving branch.
- Real-agent evidence must come from a real authenticated agent invocation; synthetic OTLP proves only the Quantum Log transport.
- `VERIFIED` means every applicable acceptance criterion is `PASS`; `FAIL`, `NOT_RUN`, `BLOCKED`, or missing evidence prevents that status.
- Source-mutating normalizers run before review. Tests, runtime evidence, and documentation for a behavior stay in the same work-unit commit.
- Conventional Commits only; never add `Co-Authored-By` or AI attribution.
- Keep authored additions plus deletions below 400 lines per PR when practical; otherwise split at the phase boundaries below.

---

## Phase and Dependency Map

Each phase is independently reviewable and leaves the repository in a truthful state. Do not start a dependent phase until its predecessor's exit criteria pass.

1. **Phase A — Safety baseline:** Tasks 1–3. Fix POSIX uninstall, add guarded repository cleanup, reconcile claims with existing evidence.
2. **Phase B — Published-artifact lifecycle:** Tasks 4–6. Build the version-parameterized lifecycle harness, run it in CI on Windows/Linux/macOS, and gate signed releases.
3. **Phase C — Capture truth:** Tasks 7–9. Decide Copilot transports first, then implement only approved adapters and collect two-machine Windows evidence.
4. **Phase D — Ledger completion:** Tasks 10–11. Decide the append-only allocation/recovery contract before writing a migration, then implement the selected contract as one vertical slice.
5. **Phase E — Product closure:** Tasks 12–13. Re-audit M0–M6, run the stable release candidate, and publish only on GO.
6. **Post-stable:** Task 14. Passive import, dashboard, and Gemini remain deferred unless the M0–M6 audit identifies one as a stable-release blocker.

## Stable-Release Exit Criteria

The release is **NO-GO** until every item below is backed by committed evidence:

- [ ] POSIX and PowerShell uninstall invoke `qlog uninstall --json` before removing the executable and preserve the ledger on success and failure.
- [ ] Repository cleanup removes only an explicit generated-artifact allowlist and a black-box test proves `.qlog/qlog.db` survives.
- [ ] `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `goreleaser check`, and formatting checks pass on the final candidate.
- [ ] Exact-version install → initialize sentinel ledger → upgrade → doctor/verify → uninstall → reinstall is green on hosted Linux, macOS, and Windows runners.
- [ ] At least two independent physical/VM Windows machines reproduce the documented install and real-agent acceptance protocol against the same signed RC artifacts.
- [ ] Every stable adapter has either privacy-safe real-agent evidence or an explicit unsupported/deferred status; configuration and synthetic probes are never reported as real capture.
- [ ] The Copilot transport ADR is accepted before any new Copilot transport implementation.
- [ ] Allocation corrections and recovery either satisfy the accepted append-only contract or the affected M2/M6 criteria remain non-verified and block stable release.
- [ ] Release artifacts include checksums, SBOMs, provenance, and a Cosign bundle that is verified against the repository workflow identity.
- [ ] The final M0–M6 evidence matrix contains no `NOT_RUN`, `BLOCKED`, missing command, or stale candidate/version claim for stable scope.
- [ ] Public README, install guide, changelog, CLI help, and milestone index agree with the final evidence matrix.

---

### Task 1: Fix POSIX Uninstall Ordering with a Black-Box Regression Test

**Files:**
- Modify: `installers/uninstall.sh`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Consumes: an installed executable at `$INSTALL_DIR/qlog` supporting `qlog uninstall --json`.
- Produces: POSIX uninstall contract: application cleanup succeeds before binary deletion; a cleanup failure preserves the executable and local data.

- [ ] **Step 1: Write the failing black-box test**

Add `TestShellUninstallerRunsOwnedCleanupBeforeRemovingBinary` to `internal/distribution/installers_test.go`. The test must skip Windows, create a fake executable that appends its arguments to `$QLOG_TEST_CALLS`, run `sh installers/uninstall.sh --install-dir <temp>/bin --no-modify-path`, then assert the call log is exactly `uninstall --json` and the executable is gone. Add a sibling `TestShellUninstallerRetainsBinaryWhenOwnedCleanupFails` whose fake executable exits 23 and assert the script fails while the executable remains.

```go
func TestShellUninstallerRunsOwnedCleanupBeforeRemovingBinary(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("POSIX shell test") }
	root := filepath.Join("..", "..")
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil { t.Fatal(err) }
	calls := filepath.Join(dir, "calls.txt")
	fake := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$QLOG_TEST_CALLS\"\n"
	qlog := filepath.Join(bin, "qlog")
	if err := os.WriteFile(qlog, []byte(fake), 0o755); err != nil { t.Fatal(err) }
	cmd := exec.Command("sh", filepath.Join(root, "installers", "uninstall.sh"), "--install-dir", bin, "--no-modify-path")
	cmd.Env = append(os.Environ(), "QLOG_TEST_CALLS="+calls)
	if output, err := cmd.CombinedOutput(); err != nil { t.Fatalf("uninstall: %v\n%s", err, output) }
	got, err := os.ReadFile(calls)
	if err != nil { t.Fatal(err) }
	if string(got) != "uninstall --json\n" { t.Fatalf("cleanup call = %q", got) }
	if _, err := os.Stat(qlog); !os.IsNotExist(err) { t.Fatalf("binary remains: %v", err) }
}
```

- [ ] **Step 2: Run the focused tests and verify the current bug**

Run: `go test -count=1 ./internal/distribution -run 'TestShellUninstaller(RunsOwnedCleanupBeforeRemovingBinary|RetainsBinaryWhenOwnedCleanupFails)' -v`

Expected: FAIL because current `installers/uninstall.sh` deletes the binary without calling `uninstall --json`.

- [ ] **Step 3: Implement fail-closed cleanup before deletion**

Replace the current direct `rm` block in `installers/uninstall.sh` with:

```sh
target="$INSTALL_DIR/qlog"
if [ -f "$target" ]; then
  if ! "$target" uninstall --json; then
    fail "qlog-owned cleanup failed; retained $target so cleanup can be retried"
  fi
  rm -f -- "$target"
  printf '%s\n' "removed $target"
else
  printf '%s\n' "qlog is not present at $target"
fi
```

Keep data preservation unchanged and do not add `--purge-data`.

- [ ] **Step 4: Run focused and contract tests**

Run: `go test -count=1 ./internal/distribution -run 'Test(ShellUninstaller|InstallerContracts)' -v`

Expected: PASS; failure-path test confirms the fake binary still exists.

- [ ] **Step 5: Commit the work unit**

```bash
git add installers/uninstall.sh internal/distribution/installers_test.go
git commit -m "fix(installer): clean owned setup before POSIX uninstall"
```

Rollback boundary: revert only this commit; it restores the former POSIX installer behavior without touching ledger data.

---

### Task 2: Add Guarded Repository Hygiene That Preserves the Local Ledger

**Files:**
- Create: `scripts/clean-generated.ps1`
- Create: `internal/distribution/repository_hygiene_test.go`
- Modify: `.gitignore`
- Delete when present and confirmed generated: `coverage.out`

**Interfaces:**
- Consumes: repository root and `-DryRun` switch.
- Produces: an allowlisted cleanup operation that can remove `dist/`, root `coverage.out`, root `qlog-external-acceptance.zip`, and root `*.test` binaries only; returns a non-zero exit for paths outside the repository.

- [ ] **Step 1: Write the failing black-box preservation test**

Create `internal/distribution/repository_hygiene_test.go`. In a temporary repository-shaped directory create `.qlog/qlog.db`, `.qlog/qlog.db-wal`, `dist/archive.zip`, and `coverage.out`; copy `scripts/clean-generated.ps1`; run it; assert the generated files are removed and both ledger files remain byte-for-byte unchanged.

```go
func TestCleanGeneratedPreservesLedger(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil { t.Skip("pwsh unavailable") }
	root := t.TempDir()
	mustWrite := func(name, value string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil { t.Fatal(err) }
	}
	mustWrite(".qlog/qlog.db", "ledger-sentinel")
	mustWrite(".qlog/qlog.db-wal", "wal-sentinel")
	mustWrite("dist/archive.zip", "generated")
	mustWrite("coverage.out", "generated")
	cmd := exec.Command(pwsh, "-NoProfile", "-File", filepath.Join("..", "..", "scripts", "clean-generated.ps1"), "-RepositoryRoot", root)
	if output, err := cmd.CombinedOutput(); err != nil { t.Fatalf("cleanup: %v\n%s", err, output) }
	for name, want := range map[string]string{".qlog/qlog.db": "ledger-sentinel", ".qlog/qlog.db-wal": "wal-sentinel"} {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != want { t.Fatalf("%s changed: %q, %v", name, got, err) }
	}
	for _, name := range []string{"dist", "coverage.out"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) { t.Fatalf("generated path remains: %s", name) }
	}
}
```

- [ ] **Step 2: Run the test and verify it fails because the script is absent**

Run: `go test -count=1 ./internal/distribution -run TestCleanGeneratedPreservesLedger -v`

Expected: FAIL with an error opening `scripts/clean-generated.ps1`.

- [ ] **Step 3: Implement the allowlisted cleaner**

Create `scripts/clean-generated.ps1` with `SupportsShouldProcess`; resolve the repository root with `GetFullPath`; reject a root lacking both `go.mod` and `.gitignore`; define only the four allowlisted targets above; explicitly reject any resolved target equal to or nested beneath `.qlog`; remove with `-LiteralPath`; print `preserved: <root>/.qlog`.

The core guard must be:

```powershell
$root = [System.IO.Path]::GetFullPath($RepositoryRoot)
if (-not (Test-Path -LiteralPath (Join-Path $root 'go.mod')) -or
    -not (Test-Path -LiteralPath (Join-Path $root '.gitignore'))) {
    throw "RepositoryRoot is not a Quantum Log checkout: $root"
}
$ledgerRoot = [System.IO.Path]::GetFullPath((Join-Path $root '.qlog'))
$targets = @('dist', 'coverage.out', 'qlog-external-acceptance.zip')
foreach ($relative in $targets) {
    $target = [System.IO.Path]::GetFullPath((Join-Path $root $relative))
    if ($target -eq $ledgerRoot -or $target.StartsWith($ledgerRoot + [IO.Path]::DirectorySeparatorChar)) {
        throw "Refusing to clean ledger path: $target"
    }
    if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Recurse -Force -WhatIf:$DryRun }
}
Get-ChildItem -LiteralPath $root -File -Filter '*.test' | Remove-Item -Force -WhatIf:$DryRun
Write-Output "preserved: $ledgerRoot"
```

- [ ] **Step 4: Ignore only reproducible local outputs**

Keep the existing `coverage.out` rule, add `/qlog-external-acceptance.zip` and `/*.test` to `.gitignore`, and do not broaden any cleanup rule to `.qlog/`. The ignore policy does not authorize deletion: the repository-local database and sidecars remain protected by the cleaner and its black-box test.

- [ ] **Step 5: Run the preservation and dry-run tests**

Run: `go test -count=1 ./internal/distribution -run TestCleanGenerated -v`

Expected: PASS. The dry-run sibling test must assert every file remains.

- [ ] **Step 6: Commit the work unit**

```bash
git add .gitignore scripts/clean-generated.ps1 internal/distribution/repository_hygiene_test.go
git rm --ignore-unmatch coverage.out
git commit -m "chore(repo): add ledger-safe generated cleanup"
```

Rollback boundary: the cleaner, its test, ignore rules, and removal of generated coverage output only.

---

### Task 3: Reconcile Documentation and Evidence Before New Claims

**Files:**
- Modify: `docs-int/milestones/README.md`
- Modify: `docs-int/verification/milestone-1-evidence.md`
- Modify: `docs-int/verification/m4-evidence.md`
- Modify: `docs-int/verification/m4-closure-backlog.md`
- Modify: `docs/verification/five-agent-evidence.md`
- Modify: `docs/verification/five-agent-external-evidence.md`
- Modify: `docs/INSTALL.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Consumes: current commands and recorded evidence only.
- Produces: one candidate/version vocabulary and milestone claims that can be mechanically checked.

- [ ] **Step 1: Add a failing documentation contract test**

Add `TestReleaseDocumentationDoesNotClaimUnsupportedEvidence` to `internal/distribution/installers_test.go`. It must read the files above and reject `M4 is VERIFIED`, a public stable-release claim before the final gate, obsolete `v0.3.2-rc.1` as the current candidate, and any claim that `PASS` means external verification.

- [ ] **Step 2: Run the contract test**

Run: `go test -count=1 ./internal/distribution -run TestReleaseDocumentationDoesNotClaimUnsupportedEvidence -v`

Expected: FAIL and name each stale file/string; the test must not silently rewrite documents.

- [ ] **Step 3: Reconcile source-of-truth language**

Use these exact semantics throughout: `IMPLEMENTED` means code exists; `READY_FOR_EXTERNAL_E2E` means it can be exercised; `PASS` means matching local evidence exists; `VERIFIED` requires the committed acceptance matrix and independent review. Keep M1 `BLOCKED` and M4 `IN_PROGRESS` until their matrices pass. Replace historical candidate text with a clearly dated history section rather than deleting evidence.

- [ ] **Step 4: Record commands and evidence, never conclusions without output**

For every matrix row, populate exact command, platform, candidate SHA/tag, result, and artifact path. If no current run exists, record `NOT_RUN`; do not convert an old run to current evidence.

- [ ] **Step 5: Run documentation contracts**

Run: `go test -count=1 ./internal/distribution -run 'Test(ReleaseDocumentation|InstallGuide|M4Evidence)' -v`

Expected: PASS; no unsupported milestone or release claim remains.

- [ ] **Step 6: Commit the work unit**

```bash
git add README.md CHANGELOG.md docs/INSTALL.md docs/verification docs-int/milestones docs-int/verification internal/distribution/installers_test.go
git commit -m "docs: reconcile release and milestone evidence"
```

Rollback boundary: evidence vocabulary and contract test only; no runtime behavior.

---

### Task 4: Build a Version-Parameterized Artifact Lifecycle Harness

**Files:**
- Create: `scripts/acceptance/release-lifecycle.sh`
- Create: `scripts/acceptance/release-lifecycle.ps1`
- Create: `scripts/acceptance/README.md`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Consumes: `QLOG_RELEASE_BASE`, `QLOG_FROM_VERSION`, `QLOG_TO_VERSION`, and a temporary `QLOG_HOME`.
- Produces: a privacy-safe evidence directory containing command exit codes, `before-version.txt`, `after-version.txt`, `doctor.json`, `verify.txt`, and SHA-256 hashes; never contains raw ledger rows or agent payloads.

- [ ] **Step 1: Add failing harness contract tests**

Extend `internal/distribution/installers_test.go` to require both scripts to reject missing versions, reject equal from/to versions for upgrade mode, pass explicit versions to installers, create a sentinel event before upgrade, run doctor and verify after upgrade, uninstall without purge, and assert the database still exists.

- [ ] **Step 2: Run contract tests**

Run: `go test -count=1 ./internal/distribution -run TestReleaseLifecycleHarnessContracts -v`

Expected: FAIL because the two harness scripts do not exist.

- [ ] **Step 3: Implement the POSIX lifecycle**

In `release-lifecycle.sh`, use `mktemp -d`, an explicit install directory, and `trap` cleanup. Run the repository installer with `--version "$QLOG_FROM_VERSION" --no-modify-path --no-bootstrap`, run `init`, append a deterministic sanitized JSONL fixture through the supported import command, hash `qlog.db`, install `QLOG_TO_VERSION`, run `doctor --json` and `verify`, invoke the uninstaller, assert `qlog.db` still exists, reinstall the target, and verify again. Never use `latest`.

- [ ] **Step 4: Implement the Windows lifecycle**

Mirror the same sequence in `release-lifecycle.ps1` using `-LiteralPath`, a GUID temporary root, explicit `$env:QLOG_HOME`, `Get-FileHash -Algorithm SHA256`, and `try/finally`. Use `powershell.exe -ExecutionPolicy Bypass` only to launch the installer under test; the harness itself must also run in `pwsh`.

- [ ] **Step 5: Run local dry-run contracts**

Run: `sh scripts/acceptance/release-lifecycle.sh --contract-only`

Expected: `PASS contract: explicit versions and isolated home`; no network and no filesystem writes outside the temporary directory.

Run: `pwsh -NoProfile -File scripts/acceptance/release-lifecycle.ps1 -ContractOnly`

Expected: same PASS line.

- [ ] **Step 6: Commit the work unit**

```bash
git add scripts/acceptance internal/distribution/installers_test.go
git commit -m "test(distribution): add versioned artifact lifecycle harness"
```

Rollback boundary: acceptance harness and contract tests; installers remain unchanged.

---

### Task 5: Run Artifact Lifecycle on Linux, macOS, and Windows

**Files:**
- Create: `.github/workflows/artifact-lifecycle.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs-int/releases/distribution.md`

**Interfaces:**
- Consumes: a published `from` release and the exact candidate artifacts staged at a release-base URL.
- Produces: three OS-specific evidence artifacts tied to the candidate tag and commit.

- [ ] **Step 1: Add a workflow-lint assertion**

Add a test in `internal/distribution/installers_test.go` that reads `.github/workflows/artifact-lifecycle.yml` and requires a three-OS matrix, explicit `from_version`/`to_version` inputs, the correct OS harness, and artifact upload even on failure.

- [ ] **Step 2: Verify the test fails**

Run: `go test -count=1 ./internal/distribution -run TestArtifactLifecycleWorkflowContract -v`

Expected: FAIL because the workflow is absent.

- [ ] **Step 3: Create the reusable workflow**

Define `workflow_call` inputs `from_version`, `to_version`, and `release_base`; use `ubuntu-latest`, `macos-latest`, and `windows-latest`; checkout the exact candidate SHA; run the correct harness; upload only its sanitized evidence directory using a name containing OS and `to_version`.

- [ ] **Step 4: Gate candidate CI with a non-moving artifact source**

Call the reusable workflow only when the candidate artifacts exist at the supplied immutable URL. Ordinary PR CI continues to run contract-only tests; it must not pretend to install unpublished artifacts.

- [ ] **Step 5: Validate workflow syntax and contracts**

Run: `go test -count=1 ./internal/distribution -run TestArtifactLifecycleWorkflowContract -v`

Expected: PASS.

Run: `actionlint .github/workflows/ci.yml .github/workflows/artifact-lifecycle.yml`

Expected: no diagnostics.

- [ ] **Step 6: Commit the work unit**

```bash
git add .github/workflows/ci.yml .github/workflows/artifact-lifecycle.yml docs-int/releases/distribution.md internal/distribution/installers_test.go
git commit -m "ci(distribution): verify release lifecycle on three platforms"
```

Rollback boundary: reusable lifecycle workflow, CI call, and distribution evidence instructions.

---

### Task 6: Gate, Sign, and Verify Release Authenticity

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `.goreleaser.yaml`
- Create: `scripts/acceptance/verify-release-authenticity.sh`
- Create: `scripts/acceptance/verify-release-authenticity.ps1`
- Modify: `docs/INSTALL.md`
- Modify: `docs-int/releases/distribution.md`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Consumes: exact tag, `checksums.txt`, `checksums.txt.sigstore.json`, archives, and SBOMs.
- Produces: a release whose checksum bundle verifies with issuer `https://token.actions.githubusercontent.com` and workflow identity for this repository's `release.yml`.

- [ ] **Step 1: Add failing release-authenticity contract tests**

Require release workflow stages in this order: source verification → tests/vet/build → GoReleaser check → snapshot artifact lifecycle → publish → sign checksum → upload bundle → verify published bundle. Require least-privilege permissions and reject unpinned moving action versions in the stable workflow.

- [ ] **Step 2: Run the focused contract test**

Run: `go test -count=1 ./internal/distribution -run TestReleaseAuthenticityContract -v`

Expected: FAIL because current workflow publishes before lifecycle verification and does not verify the uploaded bundle.

- [ ] **Step 3: Add authenticity verification scripts**

Both scripts download or accept local `checksums.txt` and its bundle, then run:

```sh
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/v'
```

They must then verify the current platform archive checksum. A missing Cosign binary is an explicit failure, not a warning.

- [ ] **Step 4: Reorder the release workflow around gates**

Use a pre-publish job for full tests and snapshot lifecycle, keep `contents: write` and `id-token: write` only on the publishing job, sign after GoReleaser creates the GitHub release, upload the bundle, then run the authenticity verifier against the published files. Do not mark the release stable if the post-upload verification fails; delete or keep it as a draft according to GitHub API capabilities rather than claiming success.

- [ ] **Step 5: Validate release configuration and workflow**

Run: `goreleaser check`

Expected: `configuration is valid`.

Run: `actionlint .github/workflows/release.yml`

Expected: no diagnostics.

Run: `go test -count=1 ./internal/distribution -run 'Test(ReleaseConfig|ReleaseAuthenticity)' -v`

Expected: PASS.

- [ ] **Step 6: Commit the work unit**

```bash
git add .github/workflows/release.yml .goreleaser.yaml scripts/acceptance/verify-release-authenticity.* docs/INSTALL.md docs-int/releases/distribution.md internal/distribution/installers_test.go
git commit -m "ci(release): gate and verify signed artifacts"
```

Rollback boundary: release workflow, authenticity scripts, and related documentation only.

---

### Task 7: Decide Copilot Transport Before Implementing Another Adapter

**Files:**
- Create: `docs/architecture/ADR-006-copilot-transport.md`
- Create: `docs-int/verification/copilot-transport-spike.md`
- Modify: `docs/adapters/copilot-cli/source-contract.md`
- Modify: `docs/adapters/copilot-vscode/source-contract.md`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Consumes: observed, sanitized behavior from current released GitHub Copilot CLI and VS Code builds.
- Produces: one accepted transport per product, or `unsupported` when no privacy-safe stable source exists; no product code.

- [ ] **Step 1: Freeze the decision criteria before running the spike**

Write the ADR in `proposed` state with weighted gates: documented/stable source, real tokens/model availability, project/session correlation, privacy, offline/local transport, install/uninstall ownership, cross-platform support, and version drift detection. A transport that exposes content by default or cannot be cleanly removed is rejected regardless of score.

- [ ] **Step 2: Capture real observed envelopes without raw content**

On a disposable profile, run one Copilot CLI session and one Copilot VS Code session. Record only product version, OS, transport availability, attribute names, event counts, and SHA-256 of the sanitized schema. Never commit prompts, responses, tool inputs/outputs, raw trace payloads, user paths, or credentials.

- [ ] **Step 3: Compare supported options**

Evaluate: official OTel, documented lifecycle hooks, extension settings, and an explicit wrapper. Reject private APIs, log scraping, UI automation, undocumented databases, and background packet interception.

- [ ] **Step 4: Accept or reject each adapter independently**

Set ADR status to `accepted` only when the evidence identifies a stable transport and exact uninstall ownership. Otherwise set that product to `unsupported` and keep its public maturity below verified. Do not start Task 8 for a rejected product.

- [ ] **Step 5: Structurally validate the ADR**

Run: `go test -count=1 ./internal/distribution -run TestCopilotTransportDecisionIsEvidenceBound -v`

Expected: PASS only when the ADR names versions, sanitized evidence, decision, rejected alternatives, privacy impact, and rollback.

- [ ] **Step 6: Commit the decision unit**

```bash
git add docs/architecture/ADR-006-copilot-transport.md docs-int/verification/copilot-transport-spike.md docs/adapters/copilot-*/source-contract.md internal/distribution/installers_test.go
git commit -m "docs(architecture): decide Copilot capture transports"
```

Rollback boundary: transport decision and evidence only. No adapter runtime changes belong in this commit.

---

### Task 8: Add Real-Agent Acceptance Adapters for Approved Stable Sources

**Files:**
- Create: `internal/acceptance/schema.go`
- Create: `internal/acceptance/schema_test.go`
- Create: `internal/acceptance/adapters.go`
- Create: `internal/acceptance/adapters_test.go`
- Modify: `internal/cli/acceptance.go`
- Modify: `internal/cli/acceptance_test.go`
- Create: `scripts/acceptance/real-agent-windows.ps1`
- Create: `scripts/acceptance/real-agent-posix.sh`
- Modify: `docs/EXTERNAL-ACCEPTANCE.md`

**Interfaces:**
- Consumes: adapter ID, evidence window start/end, installed adapter status, and ledger evidence produced by a real agent.
- Produces: schema `qlog.acceptance.real-agent/v1` containing candidate identity, OS/architecture, agent version, adapter ID, observed metric names/quality, privacy scan result, replay/dedupe result, and `PASS|FAIL|PENDING_EXTERNAL_E2E`; no raw event content.

- [ ] **Step 1: Write failing schema tests**

Define table tests requiring a candidate tag and commit, a non-empty real-agent version, `source_evidence=true`, a bounded evidence window, ledger verification PASS, privacy scan PASS, and replay/dedupe PASS before status may be PASS. Prove synthetic fixtures and setup-only state remain `PENDING_EXTERNAL_E2E`.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test -count=1 ./internal/acceptance ./internal/cli -run 'TestRealAgent|TestAcceptance' -v`

Expected: FAIL because `internal/acceptance` and the new manifest type do not exist.

- [ ] **Step 3: Implement the schema and pure evaluator**

Expose:

```go
type RealAgentEvidence struct {
	SchemaVersion string `json:"schema_version"`
	CandidateTag string `json:"candidate_tag"`
	CandidateCommit string `json:"candidate_commit"`
	Platform string `json:"platform"`
	AgentID string `json:"agent_id"`
	AgentVersion string `json:"agent_version"`
	StartedAt time.Time `json:"started_at"`
	EndedAt time.Time `json:"ended_at"`
	SourceEvidence bool `json:"source_evidence"`
	LedgerStatus string `json:"ledger_status"`
	PrivacyStatus string `json:"privacy_status"`
	ReplayStatus string `json:"replay_status"`
	Status string `json:"status"`
}

func EvaluateRealAgentEvidence(e RealAgentEvidence) (RealAgentEvidence, error)
```

The evaluator derives status; callers cannot force PASS.

- [ ] **Step 4: Add CLI packaging and platform runners**

Extend `qlog acceptance run` with repeatable `--real-agent-evidence <json>` inputs. The runners record a UTC start marker, instruct the operator to perform one normal real-agent action, then package ledger-derived summaries. They never execute an arbitrary user-provided command and never scrape agent logs.

- [ ] **Step 5: Run acceptance tests**

Run: `go test -count=1 ./internal/acceptance ./internal/cli -run 'TestRealAgent|TestAcceptance' -v`

Expected: PASS; fixture-based tests explicitly remain pending unless marked as a separately injected real-source fixture in test-only code.

- [ ] **Step 6: Commit the work unit**

```bash
git add internal/acceptance internal/cli/acceptance.go internal/cli/acceptance_test.go scripts/acceptance/real-agent-* docs/EXTERNAL-ACCEPTANCE.md
git commit -m "feat(acceptance): validate privacy-safe real-agent evidence"
```

Rollback boundary: evidence schema, evaluator, packaging flags, and operator runners.

---

### Task 9: Collect and Independently Validate Two-Machine Windows Evidence

**Files:**
- Create: `docs-int/verification/windows-machine-a.md`
- Create: `docs-int/verification/windows-machine-b.md`
- Create: `docs-int/verification/windows-e2e-summary.md`
- Modify: `docs-int/verification/m4-evidence.md`
- Modify: `docs/verification/five-agent-external-evidence.md`

**Interfaces:**
- Consumes: the same signed RC tag/commit on two independent Windows installations and their sanitized acceptance packages.
- Produces: evidence hashes and reviewer verdicts; packages themselves remain outside Git if they contain machine-specific metadata.

- [ ] **Step 1: Freeze one protocol for both machines**

The protocol must record Windows edition/build/architecture, PowerShell version, agent versions, exact signed RC tag, archive SHA-256, install command, setup result, real-agent exercise timestamp, adapter verification, replay result, privacy scan, uninstall result, and proof that `qlog.db` remains.

- [ ] **Step 2: Execute on machine A from a clean user profile**

Run the versioned lifecycle harness and every approved stable-adapter runner. Expected: lifecycle PASS; each adapter is PASS or explicitly unsupported according to Task 7; no synthetic proof is promoted.

- [ ] **Step 3: Execute on machine B without copying machine A state**

Download the same signed artifacts, create a new ledger, and repeat. Expected: same contract results and different local evidence-package SHA-256.

- [ ] **Step 4: Perform independent privacy and authenticity review**

The reviewer verifies package checksums, Sigstore identity, forbidden-field absence, candidate identity, and ledger preservation. A mismatch is `FAIL`, not a documentation note.

- [ ] **Step 5: Update M4 only from the combined result**

Mark an adapter verified only if both machines pass its source contract. Keep M4 `IN_PROGRESS` if any required stable adapter lacks evidence.

- [ ] **Step 6: Commit sanitized evidence summaries**

```bash
git add docs-int/verification/windows-machine-*.md docs-int/verification/windows-e2e-summary.md docs-int/verification/m4-evidence.md docs/verification/five-agent-external-evidence.md
git commit -m "test(acceptance): record two-machine Windows evidence"
```

Rollback boundary: sanitized evidence documents only.

---

### Task 10: Decide the Append-Only Allocation and Recovery Contract

**Files:**
- Create: `docs/architecture/ADR-007-allocation-corrections-and-recovery.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs-int/verification/milestone-1-evidence.md`
- Modify: `internal/distribution/installers_test.go`

**Interfaces:**
- Consumes: current `ReplaceAllocations`, allocation tables, raw-event chain, external anchor, backup/recovery commands, and M2/M6 acceptance criteria.
- Produces: an accepted storage contract and migration/rollback boundary before schema code is changed.

- [ ] **Step 1: Document the current contradiction with evidence**

Record that `ReplaceAllocations` mutates current rows while the project contract requires allocation changes to append auditable corrections with history and revert. Also map which recovery operations may write anchors, replay WAL, or rebuild derived projections.

- [ ] **Step 2: Evaluate exactly two bounded designs**

Compare (A) append-only allocation revision rows plus a current-state projection and (B) allocation commands encoded as raw ledger events with a rebuilt projection. Score transactionality, auditability, query cost, recovery determinism, migration risk, and compatibility.

- [ ] **Step 3: Select one contract and define invariants**

The accepted ADR must define revision identity, parent revision, idempotency key, author/source, reason, timestamp, basis-point validation, revert-as-new-revision, projection rebuild, crash recovery, anchor interaction, and old-database migration. No hard delete or in-place historical update is permitted.

- [ ] **Step 4: Define executable acceptance scenarios**

List exact scenarios: assign → correct → history; correct → revert; duplicate idempotency key; crash before/after commit; rebuild projection from revisions; database/anchor divergence; concurrent writers; legacy migration. Each scenario names the future test file in Task 11.

- [ ] **Step 5: Accept the ADR before implementation**

Run: `go test -count=1 ./internal/distribution -run TestAllocationRecoveryADRContract -v`

Expected: PASS only when the ADR is `accepted` and every invariant/scenario/rollback field is present.

- [ ] **Step 6: Commit the decision unit**

```bash
git add docs/architecture/ADR-007-allocation-corrections-and-recovery.md docs/ARCHITECTURE.md docs-int/verification/milestone-1-evidence.md internal/distribution/installers_test.go
git commit -m "docs(architecture): define append-only allocation recovery"
```

Rollback boundary: ADR and evidence map only; no database schema change.

---

### Task 11: Implement the Accepted Allocation Contract as a Vertical Slice

**Files:**
- Create: `internal/storage/sqlite/migrations/014_allocation_revisions.sql`
- Modify: `internal/domain/types.go`
- Modify: `internal/storage/sqlite/store.go`
- Create: `internal/storage/sqlite/allocation_history_test.go`
- Modify: `internal/storage/sqlite/reporting_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/server_test.go`
- Modify: `docs/en/cli-reference.md`
- Modify: `docs/es/referencia-cli.md`

**Interfaces:**
- Consumes: the exact ADR-007 schema and invariants.
- Produces: `AppendAllocationRevision`, `AllocationHistory`, `RevertAllocationRevision`, and deterministic projection rebuild used by CLI and MCP.

- [ ] **Step 1: Write failing storage tests for the ADR scenarios**

Create table-driven tests named `TestAllocationRevisionHistoryIsAppendOnly`, `TestAllocationRevertAppendsRevision`, `TestAllocationRevisionIdempotency`, `TestAllocationProjectionRebuild`, and `TestAllocationLegacyMigration`. Snapshot row counts and prior row bytes before correction; assert they never change.

- [ ] **Step 2: Run storage tests and verify RED**

Run: `go test -count=1 ./internal/storage/sqlite -run 'TestAllocation(Revision|Revert|Projection|Legacy)' -v`

Expected: FAIL because migration 014 and revision APIs do not exist.

- [ ] **Step 3: Add migration and storage API**

Implement only the selected ADR design. Expose exact methods:

```go
func (s *Store) AppendAllocationRevision(ctx context.Context, input AllocationRevisionInput) (AllocationRevision, error)
func (s *Store) AllocationHistory(ctx context.Context, subjectType, subjectID string) ([]AllocationRevision, error)
func (s *Store) RevertAllocationRevision(ctx context.Context, revisionID, idempotencyKey, reason string) (AllocationRevision, error)
func (s *Store) RebuildAllocationProjection(ctx context.Context) error
```

All write operations use one transaction under quiescence then writer lock, validate total basis points at or below 10,000, and append rather than update prior revisions.

- [ ] **Step 4: Make storage tests GREEN**

Run: `go test -count=1 ./internal/storage/sqlite -run 'TestAllocation|TestUsageGrouping' -v`

Expected: PASS with unchanged global observed token/cost totals.

- [ ] **Step 5: Add failing CLI and MCP contract tests**

Test `qlog allocation history`, `qlog allocation revert --revision <id> --idempotency-key <key> --reason <text>`, and equivalent versioned MCP operations. Assert missing idempotency key/reason fails and duplicate replay returns the existing revision.

- [ ] **Step 6: Implement CLI and MCP adapters over the same service**

Adapters call the storage/application interface; they do not duplicate allocation models or mutate rows directly.

- [ ] **Step 7: Run the vertical slice**

Run: `go test -count=1 ./internal/storage/sqlite ./internal/cli ./internal/mcpserver -run 'TestAllocation|TestAssignUsage|TestSplitUsage' -v`

Expected: PASS.

Run: `go test -race -count=1 ./internal/storage/sqlite ./internal/mcpserver`

Expected: PASS with no race report.

- [ ] **Step 8: Commit the work unit**

```bash
git add internal/storage/sqlite/migrations/014_allocation_revisions.sql internal/domain/types.go internal/storage/sqlite internal/cli internal/mcpserver docs/en/cli-reference.md docs/es/referencia-cli.md
git commit -m "feat(ledger): add append-only allocation revisions"
```

Rollback boundary: migration 014 and consumers. Because applied migrations are not deleted in released databases, rollback after publication means a forward compatibility migration, not removing migration 014.

---

### Task 12: Re-Audit M0–M6 Against Executable Acceptance Criteria

**Files:**
- Create: `docs-int/verification/milestone-0-evidence.md`
- Create: `docs-int/verification/milestone-2-evidence.md`
- Create: `docs-int/verification/milestone-3-evidence.md`
- Create: `docs-int/verification/milestone-5-evidence.md`
- Create: `docs-int/verification/milestone-6-evidence.md`
- Modify: `docs-int/verification/milestone-1-evidence.md`
- Modify: `docs-int/verification/m4-evidence.md`
- Modify: `docs-int/milestones/README.md`
- Modify: `QUANTUM_LOG_MASTER_PROMPT.md`

**Interfaces:**
- Consumes: final candidate bytes, all focused test/evidence outputs, release lifecycle artifacts, and two-machine Windows evidence.
- Produces: one row per M0–M6 acceptance criterion with command, candidate, platform, result, evidence, reviewer, and state.

- [ ] **Step 1: Generate the matrix from the master criteria without changing status**

Copy every M0–M6 criterion into the appropriate evidence file. Assign each a stable ID, exact runnable command, required platform, and expected observable result. Initial state is `NOT_RUN` unless current-candidate evidence is already linked.

- [ ] **Step 2: Run the universal gate once on frozen candidate bytes**

Run:

```bash
go mod tidy
git diff --exit-code go.mod go.sum
test -z "$(gofmt -l $(git ls-files '*.go'))"
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./...
go build ./...
goreleaser check
git diff --check
```

Expected: every command exits 0; any failure is recorded against affected milestones and blocks release.

- [ ] **Step 3: Run milestone-specific runtime gates**

M0/M1: init, append, tamper/truncation detection, quiescent doctor, backup/recovery. M2: grouped totals, allocation history/revert, pricing, export sanitization. M3: TUI/CLI parity and terminal-size goldens. M4: approved real-agent adapters and privacy/replay. M5: three-platform artifact lifecycle and authenticity. M6: MCP task lifecycle, multi-agent correlation, partial-data non-invention, and allocation correction/revert.

- [ ] **Step 4: Apply strict evidence states**

Only rows with observed expected result become `PASS`. `NOT_RUN`, `FAIL`, `BLOCKED`, unsupported required platform, or stale candidate blocks `VERIFIED`. Never infer a milestone from source presence.

- [ ] **Step 5: Obtain independent review of matrices and artifacts**

Reviewer confirms candidate hash, command/output binding, privacy, and completeness. Record reviewer verdict; do not let the implementer self-approve `VERIFIED`.

- [ ] **Step 6: Update the milestone index**

Set each M0–M6 state from its matrix. Stable release requires all in-scope criteria PASS; otherwise remain RC/NO-GO and list the smallest actionable blockers.

- [ ] **Step 7: Commit the audit unit**

```bash
git add QUANTUM_LOG_MASTER_PROMPT.md docs-int/milestones/README.md docs-int/verification/milestone-*-evidence.md
git commit -m "test: record final M0-M6 acceptance audit"
```

Rollback boundary: milestone matrices and index only.

---

### Task 13: Cut and Verify the Stable Release

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `README.md`
- Modify: `docs/INSTALL.md`
- Modify: `docs-int/releases/distribution.md`

**Interfaces:**
- Consumes: a GO result from Task 12 and the exact approved receipt/candidate commit.
- Produces: one stable tag and published signed artifacts; no published tag is rewritten.

- [ ] **Step 1: Freeze version and release notes from verified evidence**

Confirm `v0.4.0` as the stable semantic version after the audit. If `v0.4.0` already exists remotely, stop this task and create a new plan for the next unused patch version; never reuse or rewrite a published tag. Remove RC wording, list only verified capabilities, include known limitations, upgrade notes, ledger-preservation warning, and authenticity command.

- [ ] **Step 2: Re-run non-mutating final gates on the exact candidate**

Run: `go test -count=1 ./... && go vet ./... && go build ./... && goreleaser check && git diff --check`

Expected: exit 0 with no source changes. Any byte/path/mode change invalidates the candidate and returns to review.

- [ ] **Step 3: Commit release metadata**

```bash
git add CHANGELOG.md README.md docs/INSTALL.md docs-int/releases/distribution.md
git commit -m "chore(release): prepare stable Quantum Log release"
```

- [ ] **Step 4: Create and push one annotated tag**

Run: `git tag -a v0.4.0 -m "release: v0.4.0"`

Expected: annotated tag `v0.4.0` points at the reviewed release commit.

Run: `git push origin main v0.4.0`

Expected: branch and tag push succeed; never reuse or rewrite the tag after publication.

- [ ] **Step 5: Verify published artifacts and lifecycle**

Run the Task 6 authenticity script and Task 4 lifecycle harness against `v0.4.0` on all three platforms.

Expected: signature identity, checksums, install, init, doctor, verify, uninstall preservation, and reinstall all PASS.

- [ ] **Step 6: Record release GO/NO-GO**

If post-publication verification fails, mark the GitHub release affected and prepare a new patch version; do not move the existing tag or claim it passed.

Rollback boundary: before tag push, revert release metadata. After tag publication, forward-fix with a new version.

---

### Task 14: Keep Optional Product Expansion Behind the Stable Boundary

**Files:**
- Create: `docs-int/plans/post-stable-product-backlog.md`
- Modify: `docs-int/milestones/README.md`

**Interfaces:**
- Consumes: final M0–M6 audit blockers and user research.
- Produces: ordered post-stable backlog; no runtime code.

- [ ] **Step 1: Classify optional items**

Record passive import, dashboard, and Gemini as `DEFERRED_POST_STABLE` by default. Promote an item only when Task 12 identifies a concrete stable-release criterion that cannot pass without it.

- [ ] **Step 2: Define separate success metrics**

Passive import: supported source schema, idempotent import, privacy, and no file watching by default. Dashboard: shared query service parity, local-only binding, accessibility, and bounded resources. Gemini: documented stable source, real token/model evidence, privacy-safe uninstall, and cross-platform acceptance.

- [ ] **Step 3: Keep TokenTab out of implementation dependencies**

Document that TokenTab can inform product comparison but not architecture, code reuse, installation, or trust decisions. Quantum Log keeps Go because it already satisfies the CGO-free cross-platform core and avoids a high-risk rewrite before closure.

- [ ] **Step 4: Commit the backlog unit**

```bash
git add docs-int/plans/post-stable-product-backlog.md docs-int/milestones/README.md
git commit -m "docs: defer optional expansion until after stable"
```

Rollback boundary: backlog categorization only.

---

## Final Self-Review Checklist

- [ ] Every required scope item maps to at least one task: POSIX uninstall (1), safe hygiene (2), evidence reconciliation (3), cross-platform artifact lifecycle (4–5), real agents/two Windows machines (7–9), Copilot decision (7), release authenticity (6/13), append-only allocation/recovery (10–11), M0–M6 audit (12), optional backlog (14).
- [ ] No phase claims a network run, agent event, release artifact, signature, or external machine result before it is observed.
- [ ] Every code-bearing task begins with a failing focused test and ends with its focused runtime command and Conventional Commit.
- [ ] Every destructive operation has a ledger-preservation assertion and explicit rollback boundary.
- [ ] Stable publication remains blocked until current-candidate evidence, independent review, and three-platform artifact lifecycle all pass.
