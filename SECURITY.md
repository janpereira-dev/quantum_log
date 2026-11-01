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

Authenticate `checksums.txt` with Cosign, its `checksums.txt.sigstore.json` bundle,
the GitHub Actions issuer and this repository's exact tagged release-workflow
identity before checking an archive's hash. The current installers fail closed
without that verification; old unsigned releases are refused. Portable
`qlog-simple` assets and legacy `qlog` installers have different
command sets. The [historical security documentation](docs/en/privacy-security.md)
describes the earlier integrations and does not expand the current runtime.

Legacy ingestion requires HTTPS with a per-installation certificate and an
`X-Qlog-Token`. Managed producers trust that installation's certificate before
sending credentials or events; HTTP, an unrelated certificate and redirects
are refused. The certificate and private key remain inside the protected legacy
home; no system trust store is modified. A process merely occupying the collector
port cannot obtain the token. A process able to read the same account's private
files remains inside the trust boundary.

Reinstall managed legacy adapters after upgrading to regenerate their HTTPS
settings and the v2 collector token. Tokens from the former plaintext transport
are rejected without changing the prompt-hashing key or historical records.
For VS Code, close every existing instance and relaunch through the generated
private-home `qlog-code` launcher; native VS Code settings contain no reusable
collector token. Missing, inconsistent or expired TLS identity files fail closed
and are preserved for explicit recovery, rather than silently replaced.
Claude Code's native HTTP exporter did not accept its configured CA in actual
client verification. Managed setup therefore disables its native exporters and
supplies no native collector token; its qlog hooks retain lifecycle-only capture.
Model and token usage are not claimed for that integration.
The simple executable still starts no collector, creates no collector identity
or token, and configures no agents.
Hook forwarding sends no prompt text, even in full mode; full content capture
remains a direct local opt-in.
Existing historical rows are retained, including content captured by old versions;
these fixes do not silently sanitize or delete a user's history.
