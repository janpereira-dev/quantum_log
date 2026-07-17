# @janpereira.dev/quantum-log

Thin npm distributor for verified QUANTUM_LOG CLI binary.

## Install

```sh
npm install -g @janpereira.dev/quantum-log
qlog --version
```

Installation selects current supported platform (`darwin`, `linux`, or `win32`) and architecture (`x64` or `arm64`), downloads matching `v0.1.0` GitHub Release archive and `checksums.txt`, verifies archive SHA-256, then extracts only `qlog` (or `qlog.exe`). It does not collect or transmit telemetry.

Only Node.js built-in modules are used. Node.js 18 or newer is required.

## Validation

```sh
npm test
npm run test:dry-run
npm pack --dry-run
```

`npm run test:dry-run` prints selected release URLs without downloading or changing files. `QLOG_INSTALL_DRY_RUN=1` provides same behavior for direct script invocation.
