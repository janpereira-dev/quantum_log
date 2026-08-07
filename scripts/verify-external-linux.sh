#!/usr/bin/env sh
set -eu

qlog_bin="${QLOG_BIN:-qlog}"
output="${1:-$(pwd)/qlog-external-acceptance.zip}"

"$qlog_bin" acceptance run --output "$output"
printf '%s\n' "Local acceptance package: $output"
printf '%s\n' "This package does not claim external verification. Review PENDING_EXTERNAL_E2E before release."
