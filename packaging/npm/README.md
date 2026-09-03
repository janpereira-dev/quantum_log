# Legacy npm packaging fixture

This package is a legacy, unpublished packaging-validation fixture. Its source
metadata is pinned to `v0.3.2-rc.3`, while the public npm registry's historical
`latest` version is `0.1.0`. It is unsupported for installing or verifying the
current Quantum Log candidate.

Install published prerelease `v0.4.0-rc10` through the exact-version signed
GitHub Release installers documented in [`../../docs/INSTALL.md`](../../docs/INSTALL.md):

```sh
sh install.sh --version v0.4.0-rc10
```

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 --version v0.4.0-rc10
```

Do not publish or globally install this npm package as a substitute for those
installers.

## Historical fixture validation only

```sh
npm test
npm run test:dry-run
npm pack --dry-run
```

These commands exercise legacy packaging code; they are not current release or
external-E2E evidence. `npm run test:dry-run` prints selected historical URLs
without downloading or changing files. `QLOG_INSTALL_DRY_RUN=1` provides the
same behavior for direct script invocation.

`QLOG_INSTALL_LOCAL_ARTIFACT_DIR` and `npm run test:artifact` remain only for
maintaining this historical fixture against its pinned candidate. They do not
validate RC10 and must not be presented as a supported user installation path.
