#!/usr/bin/env sh
set -eu

usage() {
  printf '%s\n' "usage: real-agent-posix.sh <agent-id> <agent-version> <candidate-tag> <candidate-commit> <output.zip> [privacy-status] [replay-status]"
}

[ "$#" -ge 5 ] && [ "$#" -le 7 ] || { usage >&2; exit 2; }

agent_id=$1
agent_version=$2
candidate_tag=$3
candidate_commit=$4
output=$5
privacy_status=${6:-PENDING_EXTERNAL_E2E}
replay_status=${7:-PENDING_EXTERNAL_E2E}
qlog_bin=${QLOG_BIN:-qlog}

case "$agent_id" in
  codex|claude-code|opencode|copilot|copilot-vscode) ;;
  *) printf '%s\n' "unsupported agent id" >&2; exit 2 ;;
esac
case "$agent_version$candidate_tag" in
  *[!A-Za-z0-9._/+:-]*) printf '%s\n' "agent version and candidate tag must contain sanitized metadata characters only" >&2; exit 2 ;;
esac
[ "${#candidate_commit}" -eq 40 ] || { printf '%s\n' "candidate commit must be a full 40-character hexadecimal commit" >&2; exit 2; }
case "$candidate_commit" in
  *[!0-9a-fA-F]*) printf '%s\n' "candidate commit must be a full 40-character hexadecimal commit" >&2; exit 2 ;;
esac
case "$privacy_status" in PASS|FAIL|PENDING_EXTERNAL_E2E) ;; *) printf '%s\n' "invalid privacy status" >&2; exit 2 ;; esac
case "$replay_status" in PASS|FAIL|PENDING_EXTERNAL_E2E) ;; *) printf '%s\n' "invalid replay status" >&2; exit 2 ;; esac

started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '%s\n' "UTC evidence window started: $started_at"
printf '%s\n' "Perform one normal authenticated $agent_id action now. Do not paste prompts, responses, paths, commands, environment values, or agent logs here."
printf '%s' "Press Enter immediately after the action completes: "
IFS= read -r _confirmation
ended_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
platform=$(uname -s | tr '[:upper:]' '[:lower:]')/$(uname -m)

evidence=$(printf '{"schema_version":"qlog.acceptance.real-agent/v1","candidate_tag":"%s","candidate_commit":"%s","platform":"%s","agent_id":"%s","agent_version":"%s","started_at":"%s","ended_at":"%s","source_evidence":true,"ledger_status":"PASS","privacy_status":"%s","replay_status":"%s","status":"PENDING_EXTERNAL_E2E"}' \
  "$candidate_tag" "$candidate_commit" "$platform" "$agent_id" "$agent_version" "$started_at" "$ended_at" "$privacy_status" "$replay_status")

"$qlog_bin" acceptance run --output "$output" --real-agent-evidence "$evidence"
printf '%s\n' "Sanitized acceptance package: $output"
printf '%s\n' "PASS is derived only when qlog finds matching ledger source evidence and every supplied gate is PASS."
