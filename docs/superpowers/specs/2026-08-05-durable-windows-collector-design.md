# Durable Windows Collector Design

## Goal

Keep a consented collector running after `qlog setup --yes` when Windows Task Scheduler denies only `/Create` with access denied, without requiring another terminal.

## Decision

Use qlog-owned per-user `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` fallback. Persist JSON state under qlog collector state directory with durable executable, home, listen, log path, PID, and process-start identity. Start detached immediately, then Run key invokes same durable collector at login. Scheduler remains preferred. All other Scheduler errors remain fatal.

## Status And Cleanup

`collector status --json` exposes `mode` as `scheduler`, `user_fallback`, or `none`, and reports durable installation separately from reachability. Fallback cleanup deletes only qlog Run value and qlog state, and terminates only process matching persisted identity.

## Adjacent Fixes

Setup planning selects available adapters only. Explicit unavailable adapter installation fails before writes. Bare and dry-run setup paths remain side-effect free. VS Code managed settings report exact drifted keys and leave equal reinstallation byte-identical.

## Evidence

Add supplied redacted Copilot CLI PASS facts only. Preserve Codex and VS Code `BLOCKED_EXTERNAL` findings.
