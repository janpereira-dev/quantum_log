# Security Policy

Report vulnerabilities privately through GitHub Security Advisories for this repository. Do not include secrets, production databases, prompts, or private remote URLs in public issues.

The current on-demand executable imports only selected metadata and reported
usage counters from local Codex session files. It does not retain prompt,
response, tool argument/result, or source-file content. It does not start a
server or configure agents. See the [current guide](docs/SIMPLE.md) for its
storage paths and explicit `--home` override.

Local session and project metadata can still be sensitive. Protect the data
directory and backups, and review project names and model/provider metadata
before sharing aggregate CSV. The hash chain does not protect against a
compromised computer or replace filesystem access controls.

Verify a downloaded archive against its release's `checksums.txt` before using
it. Portable `qlog-simple` assets and legacy `qlog` installers have different
command sets. The [historical security documentation](docs/en/privacy-security.md)
describes the earlier integrations and does not expand the current runtime.
