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

plain_version=${VERSION#v}
expected_assets="qlog_${plain_version}_darwin_amd64.tar.gz
qlog_${plain_version}_darwin_amd64.tar.gz.sbom.json
qlog_${plain_version}_darwin_arm64.tar.gz
qlog_${plain_version}_darwin_arm64.tar.gz.sbom.json
qlog_${plain_version}_linux_amd64.tar.gz
qlog_${plain_version}_linux_amd64.tar.gz.sbom.json
qlog_${plain_version}_linux_arm64.tar.gz
qlog_${plain_version}_linux_arm64.tar.gz.sbom.json
qlog_${plain_version}_windows_amd64.zip
qlog_${plain_version}_windows_amd64.zip.sbom.json
qlog_${plain_version}_windows_arm64.zip
qlog_${plain_version}_windows_arm64.zip.sbom.json"

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
  for name in checksums.txt checksums.txt.sigstore.json $expected_assets; do
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
for name in $expected_assets; do
  [ -f "$ARTIFACT_DIR/$name" ] || fail "expected release asset is missing: $name"
done

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
  expected=$(printf '%s\n' "$matches" | awk 'NF { print tolower($0); exit }')
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

awk 'NF != 2 || length($1) != 64 || $1 !~ /^[0-9A-Fa-f]+$/ || $2 ~ /\// { exit 1 }
     END { if (NR != 12) exit 1 }' "$manifest" ||
  fail 'checksums.txt does not contain the exact expected asset set'
for name in $expected_assets; do
  verify_entry "$name"
done
printf 'PASS authenticity: %s (all 12 archives and SBOMs)\n' "$VERSION"
