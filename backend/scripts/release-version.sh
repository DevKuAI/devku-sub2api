#!/bin/sh
set -eu

MODE=release
case "${1:-}" in
  --upstream|--fork)
    MODE="${1#--}"
    shift
    ;;
esac
VERSION="${1:-}"
VERSION="${VERSION#v}"
case "$VERSION" in
  ''|*[!0-9A-Za-z.-]*)
    printf '%s\n' 'Release version is empty or contains unsupported characters.' >&2
    exit 1
    ;;
esac
NUMBER='(0|[1-9][0-9]*)'
BASE="$NUMBER\.$NUMBER\.$NUMBER"
PRERELEASE='(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)'

if [ "$MODE" = upstream ]; then
  if ! printf '%s\n' "$VERSION" | LC_ALL=C grep -Eq "^$BASE$"; then
    printf '%s\n' 'Expected a three-part upstream stable version.' >&2
    exit 1
  fi
  VERSION="$VERSION.0"
fi

if [ "$MODE" = fork ] && ! printf '%s\n' "$VERSION" | LC_ALL=C grep -Eq "^$BASE\.$NUMBER$"; then
  printf '%s\n' 'New fork releases require X.Y.Z.N; X.Y.Z is reserved for upstream.' >&2
  exit 1
fi

# Keep historical three-part tags compatible; new fork releases use X.Y.Z.N.
if ! printf '%s\n' "$VERSION" | LC_ALL=C grep -Eq "^$BASE(-$PRERELEASE(\.$PRERELEASE)*)?$|^$BASE\.$NUMBER$"; then
  printf '%s\n' 'Invalid release version: expected X.Y.Z.N (N >= 0) or a historical three-part tag.' >&2
  exit 1
fi

printf 'tag=v%s\nversion=%s\n' "$VERSION" "$VERSION"
