#!/bin/sh
# Remove a qlog binary installed by install.sh. Local data is preserved.
set -eu

INSTALL_DIR=${QLOG_INSTALL_DIR:-$HOME/.local/bin}
MODIFY_PATH=1
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage: uninstall.sh [options]

Options:
  --install-dir DIRECTORY Remove qlog from DIRECTORY (default: ~/.local/bin).
  --no-modify-path        Leave ~/.profile or QLOG_PROFILE unchanged.
  --dry-run               Print planned changes without writing files.
  --help                  Show this help.

Local QUANTUM_LOG configuration and database files are never removed.
EOF
}

fail() { printf '%s\n' "uninstall.sh: $*" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --install-dir) shift; [ "$#" -gt 0 ] || fail "--install-dir requires a value"; INSTALL_DIR=$1 ;;
    --install-dir=*) INSTALL_DIR=${1#*=} ;;
    --no-modify-path) MODIFY_PATH=0 ;;
    --dry-run) DRY_RUN=1 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
  shift
done

profile=${QLOG_PROFILE:-$HOME/.profile}
printf '%s\n' "binary: $INSTALL_DIR/qlog"
if [ "$MODIFY_PATH" -eq 1 ]; then printf '%s\n' "profile: $profile"; fi
if [ "$DRY_RUN" -eq 1 ]; then
  printf '%s\n' "dry-run: no files changed; local data is preserved"
  exit 0
fi

if [ -f "$INSTALL_DIR/qlog" ]; then
  rm -f "$INSTALL_DIR/qlog"
  printf '%s\n' "removed $INSTALL_DIR/qlog"
else
  printf '%s\n' "qlog is not present at $INSTALL_DIR/qlog"
fi

if [ "$MODIFY_PATH" -eq 1 ] && [ -f "$profile" ] && grep -Fq '# >>> qlog >>>' "$profile"; then
  backup="$profile.qlog-backup.$(date +%Y%m%d%H%M%S)"
  cp "$profile" "$backup"
  temporary="$profile.qlog-remove.$$"
  awk '/# >>> qlog >>>/{skip=1; next} /# <<< qlog <<</{skip=0; next} !skip{print}' "$profile" > "$temporary"
  mv "$temporary" "$profile"
  printf '%s\n' "backed up $profile to $backup"
  printf '%s\n' "removed qlog PATH entry from $profile"
fi

rmdir "$INSTALL_DIR" 2>/dev/null || true
printf '%s\n' "uninstalled qlog; local QUANTUM_LOG data was preserved"
