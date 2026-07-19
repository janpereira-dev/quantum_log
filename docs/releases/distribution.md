# Distribution Release Process

QUANTUM_LOG publishes signed release artifacts through GitHub Releases. Package-manager availability changes independently; verify the current channel status before relying on it in automation.

## Published Status

GitHub Release `v0.1.0` is public and includes archives, checksums, SBOMs, and a Sigstore checksum bundle. The public npm package is `@janpereira.dev/quantum-log`; Homebrew is available through `janpereira-dev/tap/quantum-log`; Scoop is available through `https://github.com/janpereira-dev/scoop-bucket`. WinGet and AUR publication remain subject to their external review and validation workflows.

## Native Installer

`installers/install.sh` and `installers/install.ps1` resolve a GitHub Release from `janpereira-dev/quantum_log` by default. Until an actual release tag and its artifacts exist, they fail rather than install an unchecked binary. Set `QLOG_RELEASE_REPOSITORY` or the HTTPS-only `QLOG_RELEASE_BASE` only for an authorized mirror.

All native installers:

- support `--version`, `--channel`, `--install-dir`, `--no-modify-path`, and `--dry-run`;
- map `amd64` and `arm64` to GoReleaser archives; Linux detects and reports libc, though CGO-free artifacts do not require a libc-specific archive;
- download `checksums.txt`, find the exact archive entry, and compare SHA-256 before extraction;
- stage the binary before replacing the destination and run `qlog --version` afterward;
- install into a user-owned directory by default and never request elevation;
- update only a user shell profile or user PATH when permitted, with a backup before profile changes; and
- preserve the local QUANTUM_LOG data directory on uninstall.

`stable` and `latest` currently both resolve GitHub's latest non-prerelease release. A distinct latest channel needs a published release policy before its behavior can diverge.

## Verifiable Path

Do not treat a future `curl | sh` one-liner as equivalent to verification. Download a real tagged release archive and its `checksums.txt`, then verify the exact archive locally:

```sh
sha256sum qlog_VERSION_linux_amd64.tar.gz
grep '  qlog_VERSION_linux_amd64.tar.gz$' checksums.txt
```

The two hashes must match before running `tar -xzf` or the installer. macOS users can replace `sha256sum` with `shasum -a 256`.

## Package Templates

Populate one template per release using the archive URL and matching hash from `checksums.txt`:

- `packaging/homebrew/quantum-log.rb.tmpl`
- `packaging/scoop/quantum-log.json.tmpl`
- `packaging/winget/*.tmpl`
- `packaging/aur/PKGBUILD.tmpl`
- `packaging/npm/package.json`

Do not submit a package-manager definition until its package name, publisher or tap/bucket repository, and release artifact URLs have been reserved and independently verified.
