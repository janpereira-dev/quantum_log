# Public preview checks — 2026-10-25

Scope: the on-demand Codex usage ledger and its additive `qlog-simple` portable
channel. Existing `qlog` installers and ledger data retain their prior contract.
This is preview evidence, not certification of complete agent coverage.

## Source and review

- [PR 73](https://github.com/janpereira-dev/quantum_log/pull/73) fixes project
  registration errors and the `--` delimiter, updates terminal dependencies,
  and pins builds to Go 1.26.10. All required checks and the exact-head Codex
  review passed before merge.
- [PR 74](https://github.com/janpereira-dev/quantum_log/pull/74) contains the
  portable channel, archive smoke, authenticity checks and Windows path fix.
  Its required hosted checks and Codex review are the merge gates; its Release
  workflow is the separate gate for any published tag.
- Independent reviews found no P1/P2 blockers in the runtime, distribution
  and documentation changes. Regression tests reproduced both runtime bugs
  before the fixes, plus the bracketed Windows build-path failure.

## Local execution

Windows amd64, Intel Core i7-10870H, Go 1.26.10:

- `go test -count=1 ./...`, `go vet ./...`, `go build ./...` and `go mod verify`:
  passed. Local race execution requires a C compiler unavailable on this
  machine; Linux/macOS/Windows CI runs `go test -race ./...`.
- `govulncheck ./...`: no vulnerabilities found. The previous Go 1.26.9 scan
  reported [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809).
- GoReleaser 2.18 configuration and a complete unpublished snapshot: passed.
  Twelve archives and twelve SBOMs were generated; all 24 checksum entries
  matched. Cross-compilation does not establish native ARM execution.
- Extracted Windows simple archive: passed import, replay, report and CSV
  smoke; two samples, 150 tokens, unchanged source and database, no retained
  synthetic content, and preserved legacy sentinel. The archive contains only
  the executable, license, portable README and synthetic fixture.
- Actual bracketed-path source builds in PowerShell 5.1 and 7: passed, with
  location/toolchain restoration and no leftover temporary drive alias.
- Historical npm installation tests using checksum-verified rc.3 assets:
  seven passed, none skipped. npm ran offline with a temporary local cache.

## Bounded resource observation

`scripts/verify-simple.ps1` used the 855-byte synthetic fixture, not personal
sessions. The source-build binary SHA-256 was
`54146565411D234C90F72F610539B6A8C12D4D7570D72A7ED816F90E589B52BF`.

| Operation | Wall time | Observed peak working set |
| --- | ---: | ---: |
| Import two samples | 194 ms | 13.0 MiB |
| Replay unchanged source | 112 ms | 11.2 MiB |
| Report | 44 ms | 8.4 MiB |
| Export | 46 ms | not recorded |

The commands exited; the source and report/export database hashes were unchanged.
Polling may miss brief memory peaks, and wall time includes process startup.
These observations apply to one tiny fixture on one Windows machine. They do
not establish large-history performance, subscription cost, or completeness
of Codex session logs. A separate one-iteration 100-sample benchmark measured
389 ms for append and 133 ms for replay; these are observations, not budgets.

## Public use boundary

Use the synthetic walkthrough before choosing to import personal sessions.
Only selected metadata and reported usage are stored; project names can still
be private. Costs remain unavailable. A usage sample is not a model-call count.
Legacy real-time capture and other agent integrations remain historical scope.
