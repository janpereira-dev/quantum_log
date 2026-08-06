# External Acceptance Package

Run one post-install command after local setup and any real-agent exercise:

```bash
qlog acceptance run --output qlog-external-acceptance.zip
```

Platform wrappers call same command:

```powershell
scripts/verify-external-windows.ps1 -Output qlog-external-acceptance.zip
```

```sh
scripts/verify-external-linux.sh qlog-external-acceptance.zip
scripts/verify-external-macos.sh qlog-external-acceptance.zip
```

The ZIP contains `manifest.json`, sanitized collector and adapter state, session/model/lifecycle summaries, report JSON/CSV/text, diagnostics, and `SHA256SUMS`. It excludes raw events, payloads, paths, collector logs, prompts, responses, tool data, credentials, authorization data, and secrets. Diagnostics may include a SHA-256 fingerprint of a qlog-owned collector log, never its path or contents.

## Status Semantics

- `IMPLEMENTATION_COMPLETE`: package generator and five-agent contracts exist in installed qlog. It is not external verification.
- `READY_FOR_EXTERNAL_E2E`: adapter setup is available locally and can receive a real-agent exercise. It is not evidence that an agent emitted data.
- `PASS`: matching local evidence is present. It never claims an external verifier reviewed or reproduced that evidence.
- `PENDING_EXTERNAL_E2E`: real-agent evidence is absent. This is expected on clean machines and is not `FAIL` or `BLOCKED_EXTERNAL`.
- `FAIL`: local ledger verification failed. Do not use package for release acceptance.

Use package only as evidence input for external acceptance. Never manufacture events, inject OTLP, or treat configuration as capture proof.
