#!/usr/bin/env bash
# Verify a release archive against the release's checksums.txt.
#
# Usage: verify-checksum.sh <checksums.txt> <archive>
#
# checksums.txt is the GoReleaser file: one "<sha256>  <file name>" line per
# release asset. The script exits 0 only when exactly one line names the
# archive and its hash equals the archive's sha256. Every other case exits
# non-zero, so the caller never extracts an archive it could not verify.
#
# @spec spec-ci-action
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: verify-checksum.sh <checksums.txt> <archive>" >&2
  exit 2
fi

sums="$1"
archive="$2"
name=$(basename "$archive")

if [ ! -s "$sums" ]; then
  echo "checksums file is missing or empty: $sums" >&2
  exit 1
fi
if [ ! -f "$archive" ]; then
  echo "archive is missing: $archive" >&2
  exit 1
fi

# Match the file name exactly in the second field. The "*" prefix is the
# binary-mode marker some sha256sum builds write.
matches=$(awk -v n="$name" '$2 == n || $2 == "*" n { print $1 }' "$sums")
count=$(printf '%s' "$matches" | grep -c '' || true)
if [ "$count" -ne 1 ]; then
  echo "expected one checksum entry for $name in $sums, found $count" >&2
  exit 1
fi

expected=$(printf '%s' "$matches" | tr '[:upper:]' '[:lower:]')
if ! printf '%s' "$expected" | grep -Eq '^[0-9a-f]{64}$'; then
  echo "checksum entry for $name is not a sha256 hash: $expected" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$archive" | awk '{ print $1 }')
else
  echo "no sha256 tool found (need sha256sum or shasum)" >&2
  exit 1
fi

if [ "$actual" != "$expected" ]; then
  echo "sha256 mismatch for $name: checksums.txt says $expected, archive is $actual" >&2
  exit 1
fi

echo "sha256 verified for $name"
