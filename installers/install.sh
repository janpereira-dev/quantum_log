#!/bin/sh
# Install qlog from a verified GitHub Release. No administrator privileges required.
set -eu

REPOSITORY=${QLOG_RELEASE_REPOSITORY:-janpereira-dev/quantum_log}
RELEASE_BASE=${QLOG_RELEASE_BASE:-https://github.com/$REPOSITORY/releases/download}
VERSION=""
CHANNEL=stable
INSTALL_DIR=${QLOG_INSTALL_DIR:-$HOME/.local/bin}
MODIFY_PATH=1
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --version VERSION       Install a fixed release version (for example, v1.2.3).
  --channel CHANNEL       stable (default) or latest. Both resolve GitHub's latest
                            non-prerelease release until a separate latest channel exists.
  --install-dir DIRECTORY Install qlog in DIRECTORY (default: ~/.local/bin).
  --no-modify-path        Do not edit ~/.profile or QLOG_PROFILE.
  --dry-run               Print planned changes without downloading or writing files.
  --help                  Show this help.

Environment:
  QLOG_RELEASE_REPOSITORY GitHub owner/repository to query for releases.
  QLOG_RELEASE_BASE       HTTPS release-download base URL, without tag or filename.
  QLOG_PROFILE            Shell profile to update when PATH needs an entry.
EOF
}

fail() { printf '%s\n' "install.sh: $*" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) shift; [ "$#" -gt 0 ] || fail "--version requires a value"; VERSION=$1 ;;
    --version=*) VERSION=${1#*=} ;;
    --channel) shift; [ "$#" -gt 0 ] || fail "--channel requires a value"; CHANNEL=$1 ;;
    --channel=*) CHANNEL=${1#*=} ;;
    --install-dir) shift; [ "$#" -gt 0 ] || fail "--install-dir requires a value"; INSTALL_DIR=$1 ;;
    --install-dir=*) INSTALL_DIR=${1#*=} ;;
    --no-modify-path) MODIFY_PATH=0 ;;
    --dry-run) DRY_RUN=1 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
  shift
done

case "$CHANNEL" in stable|latest) ;; *) fail "--channel must be stable or latest" ;; esac
case "$RELEASE_BASE" in https://*) ;; *) fail "QLOG_RELEASE_BASE must use HTTPS" ;; esac
case "$INSTALL_DIR" in *'"'*) fail "--install-dir cannot contain a double quote" ;; esac

download() {
  url=$1
  destination=$2
  if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error "$url" --output "$destination"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$destination" "$url"
  else
    fail "curl or wget is required to download releases"
  fi
}

download_stdout() {
  url=$1
  if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O - "$url"
  else
    fail "curl or wget is required to resolve releases"
  fi
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    fail "sha256sum or shasum is required to verify releases"
  fi
}

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) fail "unsupported operating system: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac
if [ "$OS" = linux ]; then
  if getconf GNU_LIBC_VERSION >/dev/null 2>&1; then LIBC=glibc; else LIBC=musl-or-unknown; fi
else
  LIBC=not-applicable
fi

if [ -n "$VERSION" ]; then
  case "$VERSION" in *[!0-9A-Za-z._-]*) fail "invalid version: $VERSION" ;; esac
  case "$VERSION" in v*) TAG=$VERSION; ARTIFACT_VERSION=${VERSION#v} ;; *) TAG=v$VERSION; ARTIFACT_VERSION=$VERSION ;; esac
elif [ "$DRY_RUN" -eq 1 ]; then
  TAG='<latest-release>'
  ARTIFACT_VERSION='<latest-release>'
else
  api_url="https://api.github.com/repos/$REPOSITORY/releases/latest"
  TAG=$(download_stdout "$api_url" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$TAG" ] || fail "could not resolve latest release from $api_url"
  case "$TAG" in v*) ARTIFACT_VERSION=${TAG#v} ;; *) ARTIFACT_VERSION=$TAG; TAG=v$TAG ;; esac
fi

ARCHIVE="qlog_${ARTIFACT_VERSION}_${OS}_${ARCH}.tar.gz"
RELEASE_URL="$RELEASE_BASE/$TAG"
CHECKSUM_URL="$RELEASE_URL/checksums.txt"
ARCHIVE_URL="$RELEASE_URL/$ARCHIVE"

printf '%s\n' "platform: $OS/$ARCH (libc: $LIBC)"
printf '%s\n' "channel: $CHANNEL"
printf '%s\n' "release: $TAG"
printf '%s\n' "manifest: $CHECKSUM_URL"
printf '%s\n' "archive: $ARCHIVE_URL"
printf '%s\n' "install dir: $INSTALL_DIR"

if [ "$DRY_RUN" -eq 1 ]; then
  printf '%s\n' "dry-run: no files downloaded or changed"
  exit 0
fi

command -v tar >/dev/null 2>&1 || fail "tar is required to extract releases"
workdir=$(mktemp -d "${TMPDIR:-/tmp}/qlog-install.XXXXXX")
trap 'rm -rf "$workdir"' EXIT HUP INT TERM
download "$CHECKSUM_URL" "$workdir/checksums.txt"
download "$ARCHIVE_URL" "$workdir/$ARCHIVE"
expected=$(awk -v file="$ARCHIVE" '$2 == file { print $1; exit }' "$workdir/checksums.txt")
case "$expected" in [0-9A-Fa-f][0-9A-Fa-f]*) ;; *) fail "checksum manifest has no SHA-256 entry for $ARCHIVE" ;; esac
[ "${#expected}" -eq 64 ] || fail "checksum manifest contains an invalid SHA-256 value"
actual=$(sha256_file "$workdir/$ARCHIVE")
[ "$actual" = "$expected" ] || fail "SHA-256 verification failed for $ARCHIVE"

mkdir -p "$workdir/extract"
tar -xzf "$workdir/$ARCHIVE" -C "$workdir/extract"
binary=$(find "$workdir/extract" -type f -name qlog -print | head -n 1)
[ -n "$binary" ] || fail "release archive does not contain qlog"
mkdir -p "$INSTALL_DIR"
staged="$INSTALL_DIR/.qlog.$$"
cp "$binary" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$INSTALL_DIR/qlog"
"$INSTALL_DIR/qlog" --version

if [ "$MODIFY_PATH" -eq 1 ]; then
  case ":${PATH:-}:" in
    *":$INSTALL_DIR:"*) printf '%s\n' "PATH already contains $INSTALL_DIR" ;;
    *)
      profile=${QLOG_PROFILE:-$HOME/.profile}
      if [ -f "$profile" ]; then
        backup="$profile.qlog-backup.$(date +%Y%m%d%H%M%S)"
        cp "$profile" "$backup"
        printf '%s\n' "backed up $profile to $backup"
      else
        : > "$profile"
      fi
      printf '\n# >>> qlog >>>\nexport PATH="%s:$PATH"\n# <<< qlog <<<\n' "$INSTALL_DIR" >> "$profile"
      printf '%s\n' "updated PATH in $profile"
      ;;
  esac
else
  printf '%s\n' "PATH was not modified; add $INSTALL_DIR to PATH to run qlog by name"
fi

printf '%s\n' "installed qlog at $INSTALL_DIR/qlog"
printf '%s\n' "next: qlog doctor"
