#!/bin/sh
set -eu

fail() {
  printf 'release authenticity verification failed: %s\n' "$1" >&2
  exit 1
}

VERSION=''
ARTIFACT_DIR=''
RELEASE_BASE=''

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || fail '--version requires a value'
      VERSION=$2
      shift 2
      ;;
    --artifact-dir)
      [ "$#" -ge 2 ] || fail '--artifact-dir requires a value'
      ARTIFACT_DIR=$2
      shift 2
      ;;
    --release-base)
      [ "$#" -ge 2 ] || fail '--release-base requires a value'
      RELEASE_BASE=$2
      shift 2
      ;;
    *) fail "unknown argument: $1" ;;
  esac
done

case "$VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) fail 'version must be an exact immutable vX.Y.Z tag, never latest' ;;
esac
command -v cosign >/dev/null 2>&1 || fail 'cosign is required'
printf '%s' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$' ||
  fail 'version must be an exact immutable vX.Y.Z tag, never latest'

if [ -n "$ARTIFACT_DIR" ] && [ -n "$RELEASE_BASE" ]; then
  fail 'choose exactly one mode: --artifact-dir or --release-base'
fi
if [ -z "$ARTIFACT_DIR" ] && [ -z "$RELEASE_BASE" ]; then
  fail 'choose exactly one mode: --artifact-dir or --release-base'
fi
cleanup_dir=''
cleanup() {
  if [ -n "$cleanup_dir" ] && [ -d "$cleanup_dir" ]; then
    rm -rf -- "$cleanup_dir"
  fi
}
trap cleanup EXIT HUP INT TERM

case "$(uname -s)" in
  Linux) platform=linux ;;
  Darwin) platform=darwin ;;
  *) fail 'unsupported platform' ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) fail 'unsupported architecture' ;;
esac

plain_version=${VERSION#v}
archive="qlog_${plain_version}_${platform}_${arch}.tar.gz"
sbom="${archive}.sbom.json"

if [ -n "$RELEASE_BASE" ]; then
  case "$RELEASE_BASE" in
    https://*) ;;
    *) fail 'release base must use HTTPS' ;;
  esac
  case "$RELEASE_BASE" in
    *\?*|*\#*) fail 'release base must not contain a query or fragment' ;;
  esac
  command -v curl >/dev/null 2>&1 || fail 'curl is required in download mode'
  cleanup_dir=$(mktemp -d) || fail 'could not create temporary directory'
  ARTIFACT_DIR=$cleanup_dir
  base=${RELEASE_BASE%/}
  for name in checksums.txt checksums.txt.sigstore.json "$archive" "$sbom"; do
    curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
      --output "$ARTIFACT_DIR/$name" "$base/$VERSION/$name" || fail "download failed: $name"
  done
else
  [ -d "$ARTIFACT_DIR" ] || fail 'artifact directory does not exist'
fi

manifest="$ARTIFACT_DIR/checksums.txt"
bundle="$ARTIFACT_DIR/checksums.txt.sigstore.json"
[ -f "$manifest" ] || fail 'checksums.txt is missing'
[ -f "$bundle" ] || fail 'checksum Sigstore bundle is missing'
[ -f "$ARTIFACT_DIR/$archive" ] || fail "platform archive is missing: $archive"
[ -f "$ARTIFACT_DIR/$sbom" ] || fail "platform SBOM is missing: $sbom"

identity="https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/$VERSION"
cosign verify-blob \
  --bundle "$bundle" \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --certificate-identity "$identity" \
  "$manifest" >/dev/null || fail 'checksum signature or workflow identity is invalid'

verify_entry() {
  name=$1
  matches=$(awk -v name="$name" '$2 == name { print $1 }' "$manifest")
  count=$(printf '%s\n' "$matches" | awk 'NF { count++ } END { print count+0 }')
  [ "$count" -eq 1 ] || fail "checksums.txt must contain exactly one entry for $name"
  expected=$(printf '%s\n' "$matches" | awk 'NF { print; exit }')
  case "$expected" in
    *[!0-9a-fA-F]*|'') fail "invalid checksum entry for $name" ;;
  esac
  [ "${#expected}" -eq 64 ] || fail "invalid checksum entry for $name"
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$ARTIFACT_DIR/$name" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$ARTIFACT_DIR/$name" | awk '{print $1}')
  else
    fail 'sha256sum or shasum is required'
  fi
  [ "$actual" = "$expected" ] || fail "checksum mismatch for $name"
}

verify_entry "$archive"
verify_entry "$sbom"
printf 'PASS authenticity: %s (%s and %s)\n' "$VERSION" "$archive" "$sbom"
