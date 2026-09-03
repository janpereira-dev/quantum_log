# Five-Agent External Evidence Package

`qlog acceptance run --output <zip>` packages local, sanitized evidence for five stable capture adapters: Claude Code, Codex, GitHub Copilot CLI, GitHub Copilot for VS Code, and OpenCode.

| Adapter | Local package evidence | External E2E condition | Missing real event |
| --- | --- | --- | --- |
| Claude Code | Hook setup, lifecycle quality, collector state, lifecycle summary | Run normal Claude Code action after setup | `PENDING_EXTERNAL_E2E` |
| Codex | OTLP setup with prompt logging disabled, collector state, source/model summary | Record clean-device `response.completed` OTLP evidence | `PENDING_EXTERNAL_E2E` |
| Copilot CLI | Hook and local OTel environment status, lifecycle summary | Run authenticated interactive Copilot CLI action | `PENDING_EXTERNAL_E2E` |
| Copilot VS Code | JSONC OTel settings, collector state, source/model summary | Run Copilot Chat action in uniquely attributable registered workspace | `PENDING_EXTERNAL_E2E` |
| OpenCode | Plugin setup, lifecycle quality, collector state, lifecycle summary | Run normal OpenCode action after plugin installation | `PENDING_EXTERNAL_E2E` |

`IMPLEMENTED` means code exists. `READY_FOR_EXTERNAL_E2E` means the implementation can be exercised under this protocol. `PASS` means matching local evidence exists for the recorded command, candidate, and platform. `VERIFIED` requires the committed acceptance matrix and independent review. None of these five adapters currently has recorded two-machine current-candidate verification.

No package entry includes prompts, responses, raw tool inputs or outputs, credentials, authorization fields, secrets, raw event payloads, paths, or collector log contents. Verify ZIP checksums with `SHA256SUMS` before sharing evidence.
