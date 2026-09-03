# External Real-Agent Acceptance

Quantum Log packages privacy-safe evidence after an operator performs one normal authenticated agent action. Packaging does not configure or replace an agent transport, execute an operator-provided command, or scrape agent logs.

## Evidence contract

Each repeatable `--real-agent-evidence <json>` value uses schema `qlog.acceptance.real-agent/v1` and contains only:

- exact candidate tag and full 40-character commit;
- platform, supported agent ID, and non-empty agent version;
- UTC start/end timestamps covering no more than 30 minutes;
- booleans/statuses for real source evidence, ledger verification, privacy, and replay/dedupe;
- a derived status.

The evaluator ignores a caller-supplied `status`. `PASS` requires the exact binary candidate identity, a supported agent identity, matching ledger source evidence inside the bounded window, ledger verification `PASS`, privacy `PASS`, and replay/dedupe `PASS`. Missing, setup-only, synthetic, mismatched, or incomplete evidence remains `PENDING_EXTERNAL_E2E`; an observed failed gate is `FAIL`.

GitHub Copilot CLI (`copilot`) and Copilot for VS Code (`copilot-vscode`) are unsupported for stable capture under ADR-006. Their evidence can be packaged for diagnosis but cannot become `PASS`. This command does not implement or alter those transports.

## Operator runners

Use a signed candidate and record its exact tag, commit, and agent version. The runners start a UTC window, pause while you perform one normal agent action, and pass only sanitized metadata to `qlog acceptance run`. They do not read or modify global agent configuration.

Windows:

```powershell
scripts/acceptance/real-agent-windows.ps1 `
  -AgentId codex `
  -AgentVersion 0.151.0 `
  -CandidateTag v0.4.0-rc11 `
  -CandidateCommit <40-character-commit> `
  -Output qlog-external-acceptance.zip `
  -PrivacyStatus PASS `
  -ReplayStatus PASS
```

Linux/macOS:

```sh
scripts/acceptance/real-agent-posix.sh \
  codex 0.151.0 v0.4.0-rc11 <40-character-commit> \
  qlog-external-acceptance.zip PASS PASS
```

Use `PASS` for privacy or replay only when the corresponding documented check was actually observed. The default is `PENDING_EXTERNAL_E2E`.

For already prepared versioned JSON summaries, package one or more values directly:

```sh
qlog acceptance run \
  --output qlog-external-acceptance.zip \
  --real-agent-evidence '{"schema_version":"qlog.acceptance.real-agent/v1",...}'
```

The ZIP contains `manifest.json`, `real-agent-evidence.json` when inputs are present, sanitized aggregate reports, diagnostics, and `SHA256SUMS`. It excludes raw events, payloads, paths, prompts, responses, tool data, commands, environment values, credentials, authorization data, and log contents.

## Status semantics

- `IMPLEMENTATION_COMPLETE`: packaging and evaluation exist; this is not external verification.
- `READY_FOR_EXTERNAL_E2E`: local setup is available; this is not source evidence.
- `PASS`: every contract gate passed for the exact candidate and supported agent.
- `PENDING_EXTERNAL_E2E`: evidence is absent, synthetic/setup-only, unsupported, mismatched, or incomplete.
- `FAIL`: a recorded verification, privacy, replay, or local ledger gate failed.

The package is an input to independent two-machine acceptance. It does not by itself claim external review or a stable-release GO.
