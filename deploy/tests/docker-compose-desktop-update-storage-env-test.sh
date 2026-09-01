#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

storage_variables=$(mktemp "${TMPDIR:-/tmp}/sub2api-desktop-update-storage-env.XXXXXX")
cleanup() {
  rm -f "$storage_variables"
}
trap cleanup EXIT HUP INT TERM

cat > "$storage_variables" <<'EOF'
DESKTOP_UPDATE_STORAGE_ENDPOINT
DESKTOP_UPDATE_STORAGE_REGION	cn-hangzhou
DESKTOP_UPDATE_STORAGE_BUCKET
DESKTOP_UPDATE_STORAGE_ACCESS_KEY_ID
DESKTOP_UPDATE_STORAGE_SECRET_ACCESS_KEY
DESKTOP_UPDATE_STORAGE_PREFIX	desktop-updates/
DESKTOP_UPDATE_STORAGE_FORCE_PATH_STYLE	false
DESKTOP_UPDATE_STORAGE_PUBLIC_BASE_URL
DESKTOP_UPDATE_STORAGE_MAX_UPLOAD_BYTES	209715200
EOF

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  tab=$(printf '\t')
  while IFS="$tab" read -r key value; do
    expected=$(printf '      - %s=${%s:-%s}' "$key" "$key" "$value")
    expected_count=$(grep -Fxc "$expected" "$compose_file" || true)
    key_count=$(grep -Ec "^[[:space:]]*-[[:space:]]*${key}([[:space:]]*=.*)?[[:space:]]*$" "$compose_file" || true)
    if [ "$expected_count" -ne 1 ] || [ "$key_count" -ne 1 ]; then
      printf '%s must pass %s with the expected fallback exactly once\n' "$compose_file" "$key" >&2
      exit 1
    fi
  done < "$storage_variables"
done

while IFS="$(printf '\t')" read -r key _; do
  template_count=$(grep -Ec "^${key}=" deploy/.env.example || true)
  if [ "$template_count" -ne 1 ]; then
    printf 'deploy/.env.example must define %s exactly once\n' "$key" >&2
    exit 1
  fi

done < "$storage_variables"

printf 'docker compose Desktop update storage environment test passed\n'
