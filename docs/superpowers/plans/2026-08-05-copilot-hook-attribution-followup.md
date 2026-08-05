# Copilot Hook Attribution Follow-Up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Windows Copilot hooks unambiguous, plan-only setup observable, and real VS Code attribution/evidence safe.

**Architecture:** Hook config retains Unix `bash` and adds generic Windows PowerShell command preserving pipeline stdin. Setup output converts non-mutating changes into explicit plans. Attribution stays central and requires explicit operator repair.

**Tech Stack:** Go, Cobra, PowerShell, JSON, local SQLite.

## Global Constraints

- No agent simulation or manual `qlog hook` provenance claim.
- Do not infer project ownership from provider, model, agent, or Git remote.
- Commit locally only; no push, PR, tag, or release.

### Task 1: Hook Command
- [ ] Write failing generator/parser tests for generic PowerShell command, stdin forwarding, and empty success stdout.
- [ ] Run focused tests; implement command wrapper; validate with `powershell -NoProfile -Command [scriptblock]::Create(...)`.

### Task 2: Plan-Only Output
- [ ] Write failing black-box snapshot tests for bare and dry-run setup output/state.
- [ ] Run focused tests; add explicit `planned` actions and plan-only JSON metadata without writes.

### Task 3: Attribution And Evidence
- [ ] Write failing tests/documentation assertions for explicit unattributed repair and provenance gate.
- [ ] Keep unresolved VS Code event unattributed; add privacy-safe observed telemetry and retract Copilot CLI PASS pending native hook evidence.

### Task 4: Verify
- [ ] Run focused tests, full `go test -count=1 ./...`, `go vet ./...`, build, PowerShell parser validation, and `git diff --check`.
