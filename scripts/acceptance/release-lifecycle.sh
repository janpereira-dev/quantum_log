#!/bin/sh
set -eu

PASS_LINE='PASS contract: explicit versions and isolated home'
CONTRACT_ONLY=0
case "${1:-}" in
  --contract-only) CONTRACT_ONLY=1 ;;
  "") ;;
  *) printf '%s\n' "usage: $0 [--contract-only]" >&2; exit 2 ;;
esac

make_temp_root() {
  mktemp -d "${TMPDIR:-/tmp}/qlog-release-lifecycle.XXXXXX"
}

if [ "$CONTRACT_ONLY" -eq 1 ]; then
  contract_root=$(make_temp_root)
  trap 'rm -rf "$contract_root"' EXIT HUP INT TERM
  contract_home="$contract_root/home"
  contract_install="$contract_root/bin"
  [ "$contract_home" != "$HOME" ]
  [ "$contract_install" != "$HOME/.local/bin" ]
  printf '%s\n' "$PASS_LINE"
  exit 0
fi

: "${QLOG_FROM_VERSION:?QLOG_FROM_VERSION is required}"
: "${QLOG_TO_VERSION:?QLOG_TO_VERSION is required}"
: "${QLOG_RELEASE_BASE:?QLOG_RELEASE_BASE is required}"
[ "$QLOG_FROM_VERSION" != "$QLOG_TO_VERSION" ] || {
  printf '%s\n' 'QLOG_FROM_VERSION and QLOG_TO_VERSION must differ' >&2
  exit 2
}
case "$QLOG_RELEASE_BASE" in https://*) ;; *) printf '%s\n' 'QLOG_RELEASE_BASE must use HTTPS' >&2; exit 2 ;; esac

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd -P)
INSTALLER=${QLOG_INSTALLER_SH:-$REPO_ROOT/installers/install.sh}
UNINSTALLER=${QLOG_UNINSTALLER_SH:-$REPO_ROOT/installers/uninstall.sh}
RUN_ROOT=$(make_temp_root)
EVIDENCE_DIR=${QLOG_EVIDENCE_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/qlog-release-evidence.XXXXXX")}
QLOG_HOME="$RUN_ROOT/home"
INSTALL_DIR="$RUN_ROOT/bin"
FIXTURE="$RUN_ROOT/sentinel.ndjson"
LEDGER="$QLOG_HOME/qlog.db"
export QLOG_HOME QLOG_RELEASE_BASE
trap 'rm -rf "$RUN_ROOT"' EXIT HUP INT TERM
mkdir -p "$EVIDENCE_DIR"
: > "$EVIDENCE_DIR/commands.tsv"

record_status() {
  label=$1
  shift
  if "$@"; then status=0; else status=$?; fi
  printf '%s\t%s\n' "$label" "$status" >> "$EVIDENCE_DIR/commands.tsv"
  [ "$status" -eq 0 ] || exit "$status"
}

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else printf '%s\n' 'sha256 tool is required' >&2; exit 2
  fi
}

record_status install-from sh "$INSTALLER" --version "$QLOG_FROM_VERSION" --install-dir "$INSTALL_DIR" --no-modify-path --no-bootstrap
"$INSTALL_DIR/qlog" --version > "$EVIDENCE_DIR/before-version.txt"
grep -F "${QLOG_FROM_VERSION#v}" "$EVIDENCE_DIR/before-version.txt" >/dev/null || { printf '%s\n' 'installed source version does not match QLOG_FROM_VERSION' >&2; exit 1; }
printf '%s\n' '{"source":"release-lifecycle","source_version":"1","session_id":"release-lifecycle-sentinel","event_type":"lifecycle.sentinel","occurred_at":"2026-01-01T00:00:00Z","payload":{"capture_quality":"lifecycle_only","sentinel":"qlog-release-lifecycle-v1"}}' > "$FIXTURE"
record_status init "$INSTALL_DIR/qlog" --home "$QLOG_HOME" init
record_status ingest "$INSTALL_DIR/qlog" --home "$QLOG_HOME" ingest file "$FIXTURE"
[ -f "$LEDGER" ] || { printf '%s\n' 'qlog.db was not created' >&2; exit 1; }
hash_file "$LEDGER" > "$EVIDENCE_DIR/ledger-before.sha256"

record_status install-to sh "$INSTALLER" --version "$QLOG_TO_VERSION" --install-dir "$INSTALL_DIR" --no-modify-path --no-bootstrap
"$INSTALL_DIR/qlog" --version > "$EVIDENCE_DIR/after-version.txt"
grep -F "${QLOG_TO_VERSION#v}" "$EVIDENCE_DIR/after-version.txt" >/dev/null || { printf '%s\n' 'installed target version does not match QLOG_TO_VERSION' >&2; exit 1; }
record_status doctor "$INSTALL_DIR/qlog" --home "$QLOG_HOME" doctor --json
"$INSTALL_DIR/qlog" --home "$QLOG_HOME" doctor --json | sed "s|$RUN_ROOT|<TEMP>|g" > "$EVIDENCE_DIR/doctor.json"
record_status verify "$INSTALL_DIR/qlog" --home "$QLOG_HOME" verify
"$INSTALL_DIR/qlog" --home "$QLOG_HOME" verify | sed "s|$RUN_ROOT|<TEMP>|g" > "$EVIDENCE_DIR/verify.txt"
hash_file "$LEDGER" > "$EVIDENCE_DIR/ledger-after-upgrade.sha256"
cmp -s "$EVIDENCE_DIR/ledger-before.sha256" "$EVIDENCE_DIR/ledger-after-upgrade.sha256" || { printf '%s\n' 'ledger hash changed during upgrade diagnostics' >&2; exit 1; }

record_status uninstall sh "$UNINSTALLER" --install-dir "$INSTALL_DIR" --no-modify-path
[ -f "$LEDGER" ] || { printf '%s\n' 'qlog.db was removed by uninstall' >&2; exit 1; }
hash_file "$LEDGER" > "$EVIDENCE_DIR/ledger-after-uninstall.sha256"
cmp -s "$EVIDENCE_DIR/ledger-before.sha256" "$EVIDENCE_DIR/ledger-after-uninstall.sha256" || { printf '%s\n' 'ledger hash changed during uninstall' >&2; exit 1; }

record_status reinstall-to sh "$INSTALLER" --version "$QLOG_TO_VERSION" --install-dir "$INSTALL_DIR" --no-modify-path --no-bootstrap
record_status verify-reinstall "$INSTALL_DIR/qlog" --home "$QLOG_HOME" verify
"$INSTALL_DIR/qlog" --home "$QLOG_HOME" verify | sed "s|$RUN_ROOT|<TEMP>|g" > "$EVIDENCE_DIR/verify-reinstall.txt"
hash_file "$LEDGER" > "$EVIDENCE_DIR/ledger-after-reinstall.sha256"
cmp -s "$EVIDENCE_DIR/ledger-before.sha256" "$EVIDENCE_DIR/ledger-after-reinstall.sha256" || { printf '%s\n' 'ledger hash changed during reinstall' >&2; exit 1; }

printf '%s\n' "PASS lifecycle: $QLOG_FROM_VERSION -> $QLOG_TO_VERSION"
printf '%s\n' "evidence: $EVIDENCE_DIR"
