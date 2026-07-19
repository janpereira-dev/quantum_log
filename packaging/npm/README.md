# @janpereira.dev/quantum-log

Install [QUANTUM_LOG](https://github.com/janpereira-dev/quantum_log), local-first observability and FinOps CLI for AI-assisted development.

This package provides `qlog`. It is not a JavaScript implementation of QUANTUM_LOG: its install step downloads the matching verified native binary from the official GitHub Release.

```text
npm install
    |
    v
Select OS and CPU architecture
    |
    v
Download GitHub Release archive + checksums.txt
    |
    v
Verify archive SHA-256
    |
    v
Extract qlog only
```

## Install

Node.js 18 or newer is required.

```sh
npm install -g @janpereira.dev/quantum-log
qlog --version
```

Then create local storage and register a repository:

```sh
qlog init
qlog project register --path . --name "My Project"
qlog project current --json
```

`qlog init` creates local SQLite storage. No account, SaaS backend, or API key is required for core commands.

## Supported Platforms

| Runtime platform | CPU architecture | Downloaded release archive |
|---|---|---|
| macOS (`darwin`) | x64, arm64 | `qlog_0.1.0_darwin_<arch>.tar.gz` |
| Linux (`linux`) | x64, arm64 | `qlog_0.1.0_linux_<arch>.tar.gz` |
| Windows (`win32`) | x64, arm64 | `qlog_0.1.0_windows_<arch>.zip` |

Unsupported operating systems or architectures fail during installation before any binary is extracted. For another platform, use the [GitHub Release](https://github.com/janpereira-dev/quantum_log/releases/tag/v0.1.0) assets or build from source.

## What npm Verifies

During `postinstall`, this package:

1. Selects archive from Node.js `process.platform` and `process.arch`.
2. Downloads `checksums.txt` and the matching archive over HTTPS from the same GitHub Release.
3. Finds the archive SHA-256 entry in `checksums.txt`.
4. Verifies archive bytes with Node.js built-in `crypto` before extraction.
5. Rejects unsafe archive paths and archives that do not contain exactly one expected `qlog` binary.

The package uses only Node.js built-in modules. It has no runtime npm dependencies.

Release archives also include SBOMs. `checksums.txt` has a keyless Sigstore bundle in the release for independent verification outside npm.

## Use QUANTUM_LOG

Start a task before recording work:

```sh
qlog task start --project my-project --type build --title "Initial AI-assisted work"
```

Capture executable evidence, then inspect results:

```sh
qlog run --project my-project -- npm test
qlog task finish
qlog usage month --group-by project,provider,model --json
qlog verify
```

`qlog run` records process evidence. It does not invent model tokens or cost. For model usage, ingest real OTLP telemetry, supported structured events, or adapter data. See the [repository README](https://github.com/janpereira-dev/quantum_log#readme) for capture methods, attribution, reports, pricing, budgets, terminal dashboard, MCP, and privacy details.

## Update

Install latest published npm version:

```sh
npm install -g @janpereira.dev/quantum-log@latest
qlog --version
```

Each package version is pinned to its matching GitHub Release version. `0.1.0` downloads assets from release tag `v0.1.0`.

## Uninstall

```sh
npm uninstall -g @janpereira.dev/quantum-log
```

Uninstalling the npm package removes its launcher and downloaded binary. It does not remove local QUANTUM_LOG data. If you intentionally want to delete recorded local data, find the directory with `qlog doctor` before deleting it.

## Troubleshooting

| Problem | Resolution |
|---|---|
| `qlog: command not found` | Open a new terminal. Check the global npm prefix with `npm config get prefix` and ensure its executable directory is on `PATH`. |
| `unsupported platform` or `unsupported architecture` | This distributor supports macOS, Linux, and Windows on x64 or arm64 only. Use a release archive or build from source. |
| `SHA-256 verification failed` | Do not bypass verification. Retry after clearing npm cache; ensure your network/proxy has not altered GitHub Release content. |
| GitHub download returns HTTP 404 | Upgrade to a published package version, or use matching release assets. |
| Install blocked by network policy | Permit HTTPS access to `github.com` release downloads, or install from a manually verified release archive. |
| Corporate TLS interception fails | Follow organization certificate policy. Do not disable TLS verification. |

For an installation failure, include Node.js version, npm version, platform, CPU architecture, and full installer error in a [GitHub issue](https://github.com/janpereira-dev/quantum_log/issues). Do not include local databases, prompts, tokens, API keys, or unredacted telemetry.

## Privacy And Limits

QUANTUM_LOG stores its core data locally in SQLite. This npm installer makes HTTPS download requests to GitHub to obtain the released binary and checksum manifest; it does not send usage telemetry to QUANTUM_LOG maintainers.

Imported prompts, responses, tool arguments, tool results, and secrets are removed from normalized model-call data. Exports include paths unless you explicitly pass `--redact-paths`.

Not every AI coding tool has a full-capture adapter. A process launched with `qlog run` is evidence of execution, not proof of model token usage. Do not treat estimated costs as provider invoices.

## Package Development

Run from `packaging/npm` in a source checkout:

```sh
npm test
npm run test:dry-run
npm pack --dry-run
```

`npm run test:dry-run` prints selected GitHub Release URLs without downloading or changing files. `QLOG_INSTALL_DRY_RUN=1` provides same behavior when invoking installer script directly.

## License

MIT. See repository [LICENSE](https://github.com/janpereira-dev/quantum_log/blob/main/LICENSE).
