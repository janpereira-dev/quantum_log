# Distribution Release Process

QUANTUM_LOG includes source-controlled installer and package-manager templates. They are not evidence that any release host, package, tap, bucket, publisher identifier, or AUR package currently exists.

## Published Status

`v0.4.0-rc10` is a published GitHub prerelease. No stable `v0.4.0` release, npm package, Homebrew tap, Scoop bucket, WinGet package, or AUR package is published by this repository. `packaging/npm` and the other `{{...}}` package definitions are templates, not verified current distribution channels; populate them only from a real release's `checksums.txt` before submitting to an external registry.

## Native Installer

`installers/install.sh` and `installers/install.ps1` resolve a GitHub Release from `janpereira-dev/quantum_log` by default. They fail rather than install an unchecked or unavailable binary. Set `QLOG_RELEASE_REPOSITORY` or the HTTPS-only `QLOG_RELEASE_BASE` only for an authorized mirror.

All native installers:

- support `--version`, `--channel`, `--install-dir`, `--no-modify-path`, and `--dry-run`;
- map `amd64` and `arm64` to GoReleaser archives; Linux detects and reports libc, though CGO-free artifacts do not require a libc-specific archive;
- download `checksums.txt`, find the exact archive entry, and compare SHA-256 before extraction;
- stage the binary before replacing the destination and run `qlog --version` afterward;
- install into a user-owned directory by default and never request elevation;
- update only a user shell profile or user PATH when permitted, with a backup before profile changes; and
- preserve the local QUANTUM_LOG data directory on uninstall.

## Capture Bootstrap

Official installers expose `--bootstrap` and `--no-bootstrap`. Bootstrap requires clear installer-level consent; after binary verification, it runs `qlog setup --yes`. `--no-bootstrap` suppresses that action. This configures only the four M4 stable adapters and a qlog-owned loopback collector. It does not prove an agent emitted evidence, a token source is available, or release acceptance passed.

If bootstrap is skipped or unavailable, run `qlog setup --dry-run` and then `qlog setup --yes` manually. Check `qlog collector status --json`; use `qlog collector logs` for recovery and `qlog collector uninstall` to remove only qlog-owned lifecycle state. The collector defaults to `127.0.0.1:4318`; non-loopback listening requires explicit `--allow-non-loopback` with its security implications reviewed.

Release documentation must keep source-evidence and clean-device real-agent acceptance separate from installer validation. A passing installer test, collector health check, or synthetic event is not real-agent E2E evidence.

## Hosted Artifact Lifecycle Gate

`.github/workflows/artifact-lifecycle.yml` is both manually dispatchable and reusable by other workflows. With no inputs it runs the lifecycle harness in contract-only mode on Linux, macOS, and Windows. Ordinary CI calls exactly that mode: it validates the cross-platform harness without downloading a release and MUST NOT be reported as live artifact E2E.

A maintainer can intentionally run the live artifact lifecycle by supplying all three inputs together:

- `from_version`: an exact immutable source tag;
- `to_version`: a different exact immutable target tag; and
- `release_base`: the HTTPS download base for those tagged artifacts.

Partial input sets, mutable `latest` selectors, non-HTTPS bases, queries, and fragments fail before any lifecycle job downloads an artifact. The live job installs the source version, ingests a deterministic sentinel, upgrades to the target version, runs diagnostics, uninstalls without deleting the ledger, reinstalls, and uploads bounded sanitized evidence even when a lifecycle step fails. The workflow needs no agent credentials or repository secrets.

`stable` and `latest` currently both resolve GitHub's latest non-prerelease release. A distinct latest channel needs a published release policy before its behavior can diverge.

## Verifiable Path

Do not treat a future `curl | sh` one-liner as equivalent to verification. Download a real tagged release archive and its `checksums.txt`, then verify the exact archive locally:

```sh
sha256sum qlog_VERSION_linux_amd64.tar.gz
grep '  qlog_VERSION_linux_amd64.tar.gz$' checksums.txt
```

The two hashes must match before running `tar -xzf` or the installer. macOS users can replace `sha256sum` with `shasum -a 256`.

SHA-256 alone proves only that the archive matches the downloaded manifest. To
authenticate the manifest, use the cross-platform acceptance verifier with an
exact immutable tag:

```sh
sh scripts/acceptance/verify-release-authenticity.sh \
  --version v0.4.0-rc10 \
  --release-base https://github.com/janpereira-dev/quantum_log/releases/download
```

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/acceptance/verify-release-authenticity.ps1 `
  -Version v0.4.0-rc10 `
  -ReleaseBase https://github.com/janpereira-dev/quantum_log/releases/download
```

The verifier requires Cosign and binds the bundle to issuer
`https://token.actions.githubusercontent.com` and certificate identity
`https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/<version>`.
It then independently checks the current platform archive and its SBOM against
unique entries in `checksums.txt`. Local mode uses `--artifact-dir` or
`-ArtifactDir`; download mode rejects mutable versions, non-HTTPS URLs, query
strings, and fragments.

The release workflow first validates the tag/source SHA, formatting, module
tidiness, vet, tests, race tests, build, GoReleaser configuration, and an
unpublished snapshot. The write/OIDC job then creates a draft, signs and uploads
the checksum bundle, downloads the hosted assets into a clean directory, runs
the verifier, and only then removes draft status. A failure is NO-GO and leaves
any created release as a draft for manual inspection; automation does not delete
or advertise it as stable.

This establishes checksum integrity and the signing workflow identity. It does
not constitute a separate SLSA build-provenance attestation or real-agent E2E
acceptance.

## Package Templates

Populate one template per release using the archive URL and matching hash from `checksums.txt`:

- `packaging/homebrew/quantum-log.rb.tmpl`
- `packaging/scoop/quantum-log.json.tmpl`
- `packaging/winget/*.tmpl`
- `packaging/aur/PKGBUILD.tmpl`
- `packaging/npm/package.json`

Do not submit a package-manager definition until its package name, publisher or tap/bucket repository, and release artifact URLs have been reserved and independently verified.
