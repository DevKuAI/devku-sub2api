#!/bin/bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

for version in 0.2.1 0.2.1.0 0.2.1.1 0.2.1.10 0.2.2.0 1.0.0.1 0.3.0-rc.1; do
    expected=$(printf 'tag=v%s\nversion=%s' "$version" "$version")
    test "$(sh "$ROOT_DIR/backend/scripts/release-version.sh" "$version")" = "$expected"
    test "$(sh "$ROOT_DIR/backend/scripts/release-version.sh" "v$version")" = "$expected"
done

for version in '' v1.2 v1.2.3.01 v01.2.3 v1.2.3.4.5 v1.2.3.1-rc.1 v1.2.3-01 v1.2.3+build main $'v1.2.3\nversion=9.9.9'; do
    if sh "$ROOT_DIR/backend/scripts/release-version.sh" "$version" > "$TEMP_DIR/output" 2>/dev/null; then
        echo "accepted invalid release version: $version" >&2
        exit 1
    fi
    test ! -s "$TEMP_DIR/output"
done

for version in v0.2.2 0.2.2; do
    test "$(sh "$ROOT_DIR/backend/scripts/release-version.sh" --upstream "$version")" = "$(printf 'tag=v0.2.2.0\nversion=0.2.2.0')"
done
for version in v0.2.2.0 0.2.2.0; do
    test "$(sh "$ROOT_DIR/backend/scripts/release-version.sh" --fork "$version")" = "$(printf 'tag=v0.2.2.0\nversion=0.2.2.0')"
done
for version in v0.2.2 v0.2.2-rc.1; do
    if sh "$ROOT_DIR/backend/scripts/release-version.sh" --fork "$version" >/dev/null 2>&1; then
        echo "accepted an upstream version as a new fork release: $version" >&2
        exit 1
    fi
done
for version in v0.2.2.0 v0.2.2-rc.1; do
    if sh "$ROOT_DIR/backend/scripts/release-version.sh" --upstream "$version" >/dev/null 2>&1; then
        echo "accepted invalid upstream stable version: $version" >&2
        exit 1
    fi
done

# Exercise source builds and exact tags without changing the working repository.
mkdir -p "$TEMP_DIR/repo/backend/scripts" "$TEMP_DIR/repo/backend/cmd/server"
cp "$ROOT_DIR/backend/scripts/resolve-version.sh" "$TEMP_DIR/repo/backend/scripts/"
printf '%s\n' 0.2.1.1 > "$TEMP_DIR/repo/backend/cmd/server/VERSION"
git -C "$TEMP_DIR/repo" init -q
git -C "$TEMP_DIR/repo" config user.name 'Version test'
git -C "$TEMP_DIR/repo" config user.email 'version@example.test'
git -C "$TEMP_DIR/repo" config commit.gpgsign false
git -C "$TEMP_DIR/repo" add backend
git -C "$TEMP_DIR/repo" commit -qm 'Version fixture'
test "$(sh "$TEMP_DIR/repo/backend/scripts/resolve-version.sh")" = 0.2.1.1
git -C "$TEMP_DIR/repo" -c tag.gpgsign=false tag v0.2.1.2
test "$(sh "$TEMP_DIR/repo/backend/scripts/resolve-version.sh")" = 0.2.1.2
git -C "$TEMP_DIR/repo" commit --allow-empty -qm 'Next upstream version'
git -C "$TEMP_DIR/repo" -c tag.gpgsign=false tag v0.2.2
test "$(sh "$TEMP_DIR/repo/backend/scripts/resolve-version.sh")" = 0.2.2
git -C "$TEMP_DIR/repo" commit --allow-empty -qm 'Fork baseline'
git -C "$TEMP_DIR/repo" -c tag.gpgsign=false tag v0.2.2.0
test "$(sh "$TEMP_DIR/repo/backend/scripts/resolve-version.sh")" = 0.2.2.0

# The server logs --version to stderr; retain the complete local revision.
sed -n '/^get_current_version()/,/^}/p' "$ROOT_DIR/deploy/install.sh" > "$TEMP_DIR/installer-version.sh"
source "$TEMP_DIR/installer-version.sh"
INSTALL_DIR="$TEMP_DIR/installed"
mkdir "$INSTALL_DIR"
printf '%s\n' '#!/bin/sh' 'echo "Sub2API 0.2.1.10 (commit: test, built: test)" >&2' > "$INSTALL_DIR/sub2api"
chmod +x "$INSTALL_DIR/sub2api"
test "$(get_current_version)" = 0.2.1.10
printf '%s\n' '#!/bin/sh' 'echo "Sub2API v0.2.2 (commit: test, built: test)"' > "$INSTALL_DIR/sub2api"
test "$(get_current_version)" = v0.2.2

echo 'Release version checks passed'
