# GitHub Copilot for VS Code Source Contract

Sources: GitHub's [OpenTelemetry agent-monitoring overview](https://docs.github.com/en/copilot/concepts/agents/opentelemetry)
and the linked official [VS Code Copilot monitoring reference](https://code.visualstudio.com/docs/agents/guides/monitoring-agents).

## Current status

Copilot for VS Code is unsupported for stable capture. Visual Studio Code 1.136.0
was present during the 2026-09-03 spike, but no Copilot extension was installed
and no authenticated real-device editor turn was available. CLI evidence is a
different product boundary and must not be inherited.

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
