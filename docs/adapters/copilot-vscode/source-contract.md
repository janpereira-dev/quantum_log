# GitHub Copilot for VS Code Source Contract

Sources: GitHub's
[OpenTelemetry agent-monitoring overview](https://docs.github.com/en/copilot/concepts/agents/opentelemetry)
and the linked official
[VS Code Copilot monitoring reference](https://code.visualstudio.com/docs/agents/guides/monitoring-agents).

## Decision status

Copilot for VS Code is unsupported for stable capture. Visual Studio Code 1.136.0
was present during the 2026-09-03 spike, but no Copilot extension was installed
and no authenticated real-device editor turn was available. The CLI observation
is a different product boundary and must not claim VS Code evidence.

The official documentation lists both OTLP and file exporter settings, including
`github.copilot.chat.otel.exporterType = "file"`,
`github.copilot.chat.otel.outfile`, and content capture disabled by default. The
file exporter is the preferred next candidate because it may avoid a persistent
collector, but capability documentation is not an E2E result.

## Evidence required before implementation

A stable decision requires an installed, authenticated real-device run with
pinned VS Code and Copilot extension versions. The sanitized record must include
signal/event counts, attribute names, a schema hash, content-capture state,
privacy scan, and exact setting ownership and restoration behavior. It must prove
reported model and token fields plus project/session correlation.

Until that record exists, qlog must not claim stable VS Code capture, verified
tokens, or compatibility inherited from Copilot CLI. Existing experimental OTLP
settings are not acceptance evidence.

## Future ownership boundary

If the documented file exporter passes the evidence gate, setup may manage only
the specific qlog-owned VS Code setting values and output/checkpoint files. It
must preserve unrelated settings, back up before mutation, restore only values it
changed, and keep content capture false. Private APIs, debug databases, log
scraping, UI automation/interception, and packet interception remain outside this
contract.
