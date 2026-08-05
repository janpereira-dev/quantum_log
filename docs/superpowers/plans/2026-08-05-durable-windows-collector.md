# Durable Windows Collector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver consented Windows collector fallback with accurate status and setup safety regressions.

**Architecture:** Windows collector manager retains Scheduler primary mode and writes a per-user Run-key fallback only for `/Create` access denial. Shared collector status exposes mode and durable identity. Setup plans availability before mutation. VS Code settings compare current managed keys with marker values before writing.

**Tech Stack:** Go, Cobra, Windows Registry/process APIs, JSON state, Go tests.

## Global Constraints

- Work only in this linked worktree and commit locally.
- No push, PR, tag, or publication.
- No Scheduler-error continuation except Windows `/Create` access denied.
- Tests precede every behavior change.

### Task 1: Durable Windows fallback

**Files:** `internal/cli/collector.go`, `internal/cli/collector_windows.go`, `internal/cli/collector_windows_test.go`, `internal/cli/setup.go`, `internal/cli/setup_test.go`.

- [ ] Write RED tests for fallback install/start/status/uninstall and generic-error failure.
- [ ] Run Windows-focused tests; observe expected missing fallback behavior.
- [ ] Add Run-key state, detached launch, status mode, and identity-safe cleanup.
- [ ] Run focused tests; commit behavior and tests together.

### Task 2: Setup and adapter safety

**Files:** `internal/cli/setup.go`, `internal/cli/adapters.go`, `internal/cli/capture_commands_test.go`, `internal/cli/setup_test.go`.

- [ ] Write RED tests for no-mutation setup variants, unavailable planning exclusion, and explicit unavailable install rejection.
- [ ] Run focused tests; observe failures.
- [ ] Apply minimal plan/install guards; run focused tests; commit.

### Task 3: VS Code observability and M4 evidence

**Files:** `internal/adapters/vscode_settings.go`, `internal/adapters/*_test.go`, `docs-int/verification/m4-evidence.md`.

- [ ] Write RED tests for byte-identical equal installation and exact managed-key drift reporting.
- [ ] Run focused tests; observe failures.
- [ ] Add drift result descriptions without altering external settings; update only supplied redacted Copilot CLI evidence; run focused tests; commit.

### Task 4: Verification

- [ ] Run focused package tests, `go test -count=1 ./...`, `go vet ./...`, `go build -o qlog ./cmd/qlog`, and `git diff --check`.
- [ ] Inspect status, diff, and commits; report actual results and unresolved external evidence.
