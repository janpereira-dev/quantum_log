# Five-Agent External Evidence Package

`qlog acceptance run --output <zip>` packages local, sanitized evidence for five stable capture adapters: Claude Code, Codex, GitHub Copilot CLI, GitHub Copilot for VS Code, and OpenCode.

| Adapter | Local package evidence | External E2E condition | Missing real event |
| --- | --- | --- | --- |
| Claude Code | Hook setup, lifecycle quality, collector state, lifecycle summary | Run normal Claude Code action after setup | `PENDING_EXTERNAL_E2E` |
| Codex | OTLP setup with prompt logging disabled, collector state, source/model summary | Record clean-device `response.completed` OTLP evidence | `PENDING_EXTERNAL_E2E` |
| Copilot CLI | Hook and local OTel environment status, lifecycle summary | Run authenticated interactive Copilot CLI action | `PENDING_EXTERNAL_E2E` |
| Copilot VS Code | JSONC OTel settings, collector state, source/model summary | Run Copilot Chat action in uniquely attributable registered workspace | `PENDING_EXTERNAL_E2E` |
| OpenCode | Plugin setup, lifecycle quality, collector state, lifecycle summary | Run normal OpenCode action after plugin installation | `PENDING_EXTERNAL_E2E` |

`IMPLEMENTATION_COMPLETE` states installed qlog implementation only. `READY_FOR_EXTERNAL_E2E` states local readiness only. Neither status proves external acceptance. A package `PASS` means matching local evidence exists; it does not claim external review or verification.

No package entry includes prompts, responses, raw tool inputs or outputs, credentials, authorization fields, secrets, raw event payloads, paths, or collector log contents. Verify ZIP checksums with `SHA256SUMS` before sharing evidence.
