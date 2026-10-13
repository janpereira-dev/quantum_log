# Simple local ledger validation

Date: 2026-10-13. Decision: **GO for local evaluation; no published release claim.**

## Scope and preservation

The default executable imports available local Codex session files, reports
usage by project/agent/model, and exports aggregate CSV. It has no collector,
watcher, HTTP receiver, startup registration, agent hooks, or MCP server.
The earlier entry point is preserved in `cmd/qlog-legacy`. No existing agent
configuration or original ledger was modified during this implementation.

The original source state (`1db0f204d7925691d34bac3952e88c3eb10d11d0`) was saved
as a verified complete Git bundle in local `dist/backup`. Copies of the
repository and installed ledger files were checked against their originals
by SHA-256. Generated backups are local, ignored, and excluded from the
portable executable package.

## Automated checks

- `go test ./...`: PASS, including retained legacy packages.
- `go vet ./...`: PASS.
- Eighteen new focused tests cover replay, copies and growing files, cumulative
  counters and subset dimensions, absent/zero/reset counters, malformed input,
  partial active tails, privacy, source project attribution, cancellation,
  directory selection, filters, CSV/JSON, default isolation, and read-only reports.
- Independent review identified relative source-directory attribution; it was
  fixed and covered by a regression test. The subsequent review reported no
  additional actionable findings in the reviewed changes.
- Windows executable built with Go 1.27.1, `-trimpath -ldflags '-s -w'`.
- Linux amd64 and macOS arm64 cross-compilation: PASS. Those platforms were not
  exercised at runtime.
- Race checks were not executed: this host reports `CGO_ENABLED=0`, and no C
  compiler was available on the current command path.

## Native source check

One completed native Codex session file, 423,965 bytes, was imported into a
fresh isolated ledger. No conversation content or project rows are included
in this report or the generated measurement evidence.

| Check | Result |
| --- | --- |
| First import | 9 accepted usage samples |
| Replay | 0 accepted samples, 9 duplicates |
| Imported token total | 531,665, equal to the independent final native counter |
| Source file before/after | SHA-256 unchanged |
| Report/export before/after | Database SHA-256 unchanged |
| Remaining executable processes | 0 |
| Cost quality | `unavailable` |

The initial local measured build SHA-256 was
`984e4b04b9bb55ffe7ac220d7acc2e0187da21b0533d8a8bba9daa2e4d86e11c`.
The source SHA-256 was
`92aa138c3314454383c6ece2f6da7d0ad8fb816e1f574e2ac2608a9f15890c7f`.

## Resource observations

Measurements were taken on Windows 10.0.26300 with 16 logical processors.
The script polls the child process; wall time includes startup and polling
overhead, and short memory peaks can be missed.

| Operation, final native run | Wall time | Observed peak working set |
| --- | ---: | ---: |
| Fresh import | 994.155 ms | 16,789,504 bytes |
| Replay | 598.578 ms | 14,475,264 bytes |
| Report | 99.704 ms | 9,134,080 bytes |
| Export | 105.785 ms | Not separately recorded |

Earlier native import runs ranged from roughly 0.99 to 6.21 seconds during
build/test activity. These are single-machine observations, not latency
percentiles or a historical speedup comparison.

With the same Go toolchain and build flags, the simple Windows binary was
7,844,864 bytes; the retained legacy binary was 23,705,088 bytes, a reduction
of about 67% from legacy to simple. This is a binary-size comparison,
not a claim that CPU or execution time improved by the same percentage.

Synthetic importer benchmarks, one iteration and 100 samples, with ledger
opening excluded: append 408.580 ms and 2,340,840 allocated bytes; replay
134.640 ms and 875,312 allocated bytes. Allocated bytes are not peak memory.

## Publication follow-up

Before uploading the source PR, the branch was placed on the current remote
main without replaying already-integrated historical work. CI and release
pins were updated to Go 1.26.9 and x/net to v0.60.0 to address the security
scan's confirmed findings. The portable build script now uses the exact
repository toolchain. This maintenance also requires x/term v0.46.0 and
x/text v0.42.0 through the module graph.

The Go 1.26.9 build was checked again against the same completed native file:
9 imported samples, 9 duplicates on replay, 531,665 tokens matching the source,
unchanged source/database hashes, and zero remaining executable processes.
Observed times were 3,151.671 ms for fresh import, 682.454 ms for replay, and
100.006 ms for report. Fresh-import peak working set was 48,328,704 bytes;
report peak was 9,527,296 bytes. These observations replace any assumption that
the initial build's memory measurement applies to all compiler versions.

The updated binary SHA-256 is
`9a9197a56ba17e2c411bd99220fee5caeed6993b12abec37c1001662305434ce`.
With the same patched compiler and build flags, simple/legacy binary sizes
were 7,416,832 / 23,262,208 bytes (about 68% reduction).
Fresh `govulncheck` v1.9.0 found no vulnerabilities, and the CI-matching
golangci-lint v2.14.0 reported zero issues after fixing error handling/style.
Published-release, SBOM execution, and multi-machine claims remain outside
this source PR's validation.

## Reproduction and limits

Build the portable Windows package with `scripts/build-simple.ps1`. Run
`scripts/verify-simple.ps1 -SourceFile <completed-session.jsonl>` with
PowerShell 7 to reproduce native import, replay, report/export preservation,
source-total comparison, and child-process measurements. The verification
script creates a fresh local ledger and writes sanitized `evidence.json` under
`dist/simple-verification`; it requires monotonic source counters.

Only available source logs are covered. Import is manual, directory traversal
is not bounded by the selected-file limit, and one large file can take longer.
Counter resets are skipped and rebased rather than inventing missing usage.
Usage samples are counter intervals, not exact model calls. Costs, complete
agent coverage, CPU budgets, sustained-load behavior, two-machine validation,
and release readiness are not established by these checks.
