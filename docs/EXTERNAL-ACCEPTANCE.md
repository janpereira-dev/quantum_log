# External Real-Agent Acceptance

Quantum Log packages privacy-safe evidence around one normal authenticated agent action. This workflow does not configure or replace transports, execute operator-provided commands, or scrape agent logs.

## Evidence boundary

`qlog acceptance begin` creates a persisted, one-use pre-action boundary. The boundary binds:

- schema `qlog.acceptance.real-agent-boundary/v1` and a random challenge;
- exact candidate tag, full commit, executable SHA-256, and actual `GOOS/GOARCH`;
- supported agent ID and strictly sanitized agent version;
- qlog-generated UTC start time, ledger position hash, and event count.

`qlog acceptance run --boundary <id>` rejects future, older-than-30-minute, modified, mismatched, duplicated, or previously consumed boundaries. Selected source evidence must occur strictly after the boundary and before packaging, and the ledger position must advance.

The resulting evidence uses `qlog.acceptance.real-agent/v1`. Qlog derives source evidence, ledger verification, privacy, observed metric names, and capture quality from the selected ledger/package data. Caller-provided JSON and caller-provided `PASS` values are not accepted. Privacy scans reject forbidden content fields and secret-like values.

Replay/dedupe remains `PENDING_EXTERNAL_E2E` because Task 8 has no safe executable real-source replay operation. Therefore a package cannot report real-agent `PASS` yet. Synthetic or setup-only rows also remain pending. This is intentional and release-safe.

GitHub Copilot CLI (`copilot`) and Copilot for VS Code (`copilot-vscode`) remain unsupported for stable capture under ADR-006 and cannot become `PASS`. Task 8 does not implement or modify either transport.

## Operator runners

The runners resolve only the installed `qlog` executable. They reject aliases, symlinks/reparse points, non-regular files, changed executable hashes, missing/empty packages, and packages that fail `qlog acceptance inspect`. There is no `QLOG_BIN`, `-Qlog`, arbitrary command, candidate identity, privacy, or replay override.

Windows:

```powershell
scripts/acceptance/real-agent-windows.ps1 `
  -AgentId codex `
  -AgentVersion 0.151.0 `
  -Output qlog-external-acceptance.zip
```

Linux/macOS:

```sh
scripts/acceptance/real-agent-posix.sh \
  codex 0.151.0 qlog-external-acceptance.zip
```

The runners create the boundary, instruct the operator to perform one normal action, package the bounded evidence, verify that the output is a non-empty regular file, and inspect its manifest/checksums with the same unchanged qlog binary. They read and write no global agent configuration.

Manual equivalent:

```sh
boundary=$(qlog acceptance begin --agent codex --agent-version 0.151.0)
# Perform one normal authenticated Codex action immediately.
qlog acceptance run --output qlog-external-acceptance.zip --boundary "$boundary"
qlog acceptance inspect --package qlog-external-acceptance.zip
```

The ZIP contains `manifest.json`, `real-agent-evidence.json` when boundaries are supplied, sanitized aggregate reports, diagnostics, and `SHA256SUMS`. It excludes raw events, payloads, paths, prompts, responses, tool arguments/results, commands, environment values, credentials, authorization data, and log contents.

## Status semantics

- `IMPLEMENTATION_COMPLETE`: boundary, packaging, and evaluation exist; this is not external verification.
- `READY_FOR_EXTERNAL_E2E`: local setup is available; this is not source evidence.
- `PASS`: reserved for evidence where every qlog-derived contract gate passes.
- `PENDING_EXTERNAL_E2E`: evidence is absent, synthetic/setup-only, unsupported, stale, or missing an executable replay proof.
- `FAIL`: a qlog-derived ledger, privacy, package, or future replay gate failed.

The package is input to independent two-machine acceptance. It does not by itself claim external review or a stable-release GO.
