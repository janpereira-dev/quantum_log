#!/usr/bin/env sh
set -eu

usage() {
  printf '%s\n' "usage: real-agent-posix.sh <agent-id> <agent-version> <output.zip>"
}

[ "$#" -eq 3 ] || { usage >&2; exit 2; }
agent_id=$1
agent_version=$2
output=$3

qlog_path=$(command -v qlog) || { printf '%s\n' "qlog executable not found" >&2; exit 1; }
case "$qlog_path" in /*) ;; *) printf '%s\n' "qlog must resolve to an absolute path" >&2; exit 1 ;; esac
[ -f "$qlog_path" ] && [ ! -L "$qlog_path" ] && [ -x "$qlog_path" ] || { printf '%s\n' "qlog must be a regular non-symlink executable" >&2; exit 1; }

hash_qlog() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$qlog_path" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$qlog_path" | awk '{print $1}'
  else
    printf '%s\n' "no SHA-256 utility available" >&2
    return 1
  fi
}

qlog_hash=$(hash_qlog) || exit $?
if boundary_id=$("$qlog_path" acceptance begin --agent "$agent_id" --agent-version "$agent_version"); then :; else
  code=$?; printf '%s\n' "qlog could not create an acceptance boundary" >&2; exit "$code"
fi
case "$boundary_id" in
  *[!0-9a-f]*|'') printf '%s\n' "qlog returned an invalid acceptance boundary" >&2; exit 1 ;;
esac
[ "${#boundary_id}" -eq 64 ] || { printf '%s\n' "qlog returned an invalid acceptance boundary" >&2; exit 1; }

printf '%s\n' "Perform one normal authenticated $agent_id action now. Do not paste prompts, responses, paths, commands, environment values, or agent logs here."
printf '%s' "Press Enter immediately after the action completes: "
IFS= read -r _confirmation

[ "$(hash_qlog)" = "$qlog_hash" ] || { printf '%s\n' "qlog changed after the boundary was created" >&2; exit 1; }
if "$qlog_path" acceptance run --output "$output" --boundary "$boundary_id"; then :; else
  code=$?; printf '%s\n' "qlog acceptance packaging failed" >&2; exit "$code"
fi
[ -s "$output" ] && [ -f "$output" ] && [ ! -L "$output" ] || { printf '%s\n' "acceptance package is missing, empty, or unsafe" >&2; exit 1; }
[ "$(hash_qlog)" = "$qlog_hash" ] || { printf '%s\n' "qlog changed while packaging evidence" >&2; exit 1; }
if "$qlog_path" acceptance inspect --package "$output"; then :; else
  code=$?; printf '%s\n' "acceptance package inspection failed" >&2; exit "$code"
fi
printf '%s\n' "Sanitized acceptance package verified: $output"
