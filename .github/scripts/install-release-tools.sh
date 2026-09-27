#!/usr/bin/env bash
# Installs the build tools the release and snapshot jobs run: GoReleaser
# and syft. Each archive is fetched from a fixed release URL and checked
# against a pinned SHA-256 before anything is extracted. No installer
# script and no "latest" lookup is involved.
#
# Usage: install-release-tools.sh <bin-dir>
#
# How each pin was derived, so the next upgrade repeats it:
#   1. Download the tool's checksums file and its Sigstore signature.
#   2. Verify the signature with cosign against the tool's own release
#      workflow identity (below).
#   3. Take the archive's line from the verified checksums file, and check
#      the downloaded archive against it.
#
# GoReleaser v2.18.1: checksums.txt, bundle checksums.txt.sigstore.json,
#   identity https://github.com/goreleaser/goreleaser/.github/workflows/release.yml@refs/tags/v2.18.1
# syft v1.42.3: syft_1.42.3_checksums.txt, .sig and .pem,
#   identity https://github.com/anchore/syft/.github/workflows/release.yaml@refs/heads/main
#
# Change a version and its hash together, in this file only. release.yml
# and release-snapshot.yml both call this script, so they cannot drift.

set -euo pipefail

GORELEASER_VERSION="2.18.1"
GORELEASER_SHA256="0c6122af0ad8fd65638889bf7d3757148b2f80eeff9f079682f0655df66ec8e8"
SYFT_VERSION="1.42.3"
SYFT_SHA256="0d6be741479eddd2c8644a288990c04f3df0d609bbc1599a005532a9dff63509"

bin_dir="${1:?usage: install-release-tools.sh <bin-dir>}"
mkdir -p "$bin_dir"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# fetch <url> <sha256> <archive member> <name>
fetch() {
  local url=$1 sum=$2 member=$3 name=$4
  curl -fsSL --retry 3 -o "$work/$name.tar.gz" "$url"
  if ! echo "$sum  $work/$name.tar.gz" | sha256sum -c --status -; then
    echo "install-release-tools: $name archive does not match its pinned SHA-256" >&2
    exit 1
  fi
  tar -xzf "$work/$name.tar.gz" -C "$work" "$member"
  install -m 0755 "$work/$member" "$bin_dir/$name"
  echo "install-release-tools: $name verified and installed"
}

fetch "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" \
  "$GORELEASER_SHA256" goreleaser goreleaser
fetch "https://github.com/anchore/syft/releases/download/v${SYFT_VERSION}/syft_${SYFT_VERSION}_linux_amd64.tar.gz" \
  "$SYFT_SHA256" syft syft

"$bin_dir/goreleaser" --version | grep -q "GitVersion: *v\?${GORELEASER_VERSION}$" ||
  { echo "install-release-tools: goreleaser reports an unexpected version" >&2; exit 1; }
"$bin_dir/syft" version | grep -q "^Version: *${SYFT_VERSION}$" ||
  { echo "install-release-tools: syft reports an unexpected version" >&2; exit 1; }
