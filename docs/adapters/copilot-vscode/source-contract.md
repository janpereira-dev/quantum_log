# GitHub Copilot for VS Code Source Contract

> Historical integration contract for `cmd/qlog-legacy`. The current product
> supports local Codex session files; see the [current guide](../../SIMPLE.md).

Sources: GitHub's [OpenTelemetry agent-monitoring overview](https://docs.github.com/en/copilot/concepts/agents/opentelemetry)
and the linked official [VS Code Copilot monitoring reference](https://code.visualstudio.com/docs/agents/guides/monitoring-agents).

## Current status

Copilot for VS Code remains unsupported for stable capture. On 2026-11-01,
installed VS Code 1.144.0 with its unmodified official Copilot Chat 0.72.0
extension completed a chat turn against a synthetic loopback model configured
through the documented [Custom Endpoint provider](https://code.visualstudio.com/docs/agent-customization/language-models).
An isolated extension-development window loaded that installed extension and a
command-only test driver; the driver invoked the application's native chat
command and did not replace the model provider or telemetry exporter. The
generated `qlog-code` launcher supplied HTTPS trust and authentication. The
collector built from `f2e2d77` persisted three raw events and exactly one model
call with 10 input tokens, 5 output tokens and 15 total tokens. The prompt and
answer markers were absent from the ledger. Foreign-CA and plaintext receivers
on the stopped collector's exact port received zero HTTP requests while the
same real application completed its synthetic turn.

No real account, provider credentials, user workspace, or existing editor profile
was used. This proves native application transport and reported synthetic usage;
it does not prove authenticated provider execution, project attribution, or the
full stable-capture gate. The earlier 2026-09-03 spike with VS Code 1.136.0 remains
historical evidence: no Copilot extension was installed and no authenticated
real-device editor turn was available at that time. CLI evidence is a different
product boundary and must not be inherited.

Official documentation lists OTLP and file exporter settings. They are capability
descriptions, not source evidence. Neither transport is approved until a pinned
extension/device run proves model, token, session/project, privacy, lifecycle,
and drift behavior. A file exporter is a diagnostic candidate subject to the
same transient-content and ownership veto in ADR-006.

## Evidence gate

The sanitized record must contain only signal/event counts, attribute names, a
reproducible canonical-schema hash, command result, and forbidden-marker scan.
It must identify the VS Code and Copilot extension versions and prove exact
setting restoration. Until then qlog must not claim stable capture, verified
tokens, or compatibility from Copilot CLI.

Private APIs, debug databases, log scraping, UI automation/interception, and
packet interception remain outside this contract.
