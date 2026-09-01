#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

active_variables=$(mktemp "${TMPDIR:-/tmp}/sub2api-application-env.XXXXXX")
cleanup() {
  rm -f "$active_variables"
}
trap cleanup EXIT HUP INT TERM

awk -F= '/^[A-Z][A-Z0-9_]*=/ { print $1 }' deploy/.env.example > "$active_variables"

duplicate_variables=$(sort "$active_variables" | uniq -d)
if [ -n "$duplicate_variables" ]; then
  printf 'deploy/.env.example defines duplicate active variables:\n%s\n' "$duplicate_variables" >&2
  exit 1
fi

application_setting_count() {
  compose_file=$1
  expected=$2

  awk -v expected="$expected" '
    /^  sub2api:/ { in_app = 1; next }
    /^  [[:alnum:]_-]+:/ { in_app = 0 }
    in_app && /^[[:space:]]*-[[:space:]]*[A-Z][A-Z0-9_]*=/ {
      line = $0
      sub(/^[[:space:]]*-[[:space:]]*/, "", line)
      sub(/=.*/, "", line)
      if (line == expected) { count++ }
    }
    END { print count + 0 }
  ' "$compose_file"
}

assert_contains_count() {
  target_file=$1
  expected=$2
  wanted_count=$3
  count=$(grep -Fc -- "$expected" "$target_file" || true)
  if [ "$count" -ne "$wanted_count" ]; then
    printf '%s must contain %s exactly %s time(s)\n' "$target_file" "$expected" "$wanted_count" >&2
    exit 1
  fi
}

assert_contains_once() {
  assert_contains_count "$1" "$2" 1
}

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  while IFS= read -r variable; do
    case "$variable" in
      BIND_HOST|APPLE_CONTAINER_*|POSTGRES_USER|POSTGRES_PASSWORD|POSTGRES_DB|POSTGRES_MAX_CONNECTIONS|POSTGRES_SHARED_BUFFERS|POSTGRES_EFFECTIVE_CACHE_SIZE|POSTGRES_MAINTENANCE_WORK_MEM|REDIS_MAXCLIENTS)
        continue
        ;;
    esac

    count=$(application_setting_count "$compose_file" "$variable")
    if [ "$count" -ne 1 ]; then
      printf '%s must map active application variable %s exactly once\n' "$compose_file" "$variable" >&2
      exit 1
    fi
  done < "$active_variables"

  assert_contains_once "$compose_file" '      - DATABASE_MAX_OPEN_CONNS=${DATABASE_MAX_OPEN_CONNS:-256}'
  assert_contains_once "$compose_file" '      - DATABASE_MAX_IDLE_CONNS=${DATABASE_MAX_IDLE_CONNS:-128}'
  assert_contains_once "$compose_file" '      - REDIS_POOL_SIZE=${REDIS_POOL_SIZE:-1024}'
  assert_contains_once "$compose_file" '      - REDIS_MIN_IDLE_CONNS=${REDIS_MIN_IDLE_CONNS:-128}'
  assert_contains_once "$compose_file" '      - JWT_ACCESS_TOKEN_EXPIRE_MINUTES=${JWT_ACCESS_TOKEN_EXPIRE_MINUTES:-0}'
  assert_contains_once "$compose_file" '      - SERVER_H2C_ENABLED=${SERVER_H2C_ENABLED:-false}'
  assert_contains_once "$compose_file" '      - LOG_FORMAT=${LOG_FORMAT:-console}'
done

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.dev.yml
do
  assert_contains_once "$compose_file" '-c max_connections=${POSTGRES_MAX_CONNECTIONS:-100}'
  assert_contains_once "$compose_file" '-c shared_buffers=${POSTGRES_SHARED_BUFFERS:-128MB}'
  assert_contains_once "$compose_file" '-c effective_cache_size=${POSTGRES_EFFECTIVE_CACHE_SIZE:-4GB}'
  assert_contains_once "$compose_file" '-c maintenance_work_mem=${POSTGRES_MAINTENANCE_WORK_MEM:-64MB}'
  assert_contains_once "$compose_file" '      - REDIS_MAXCLIENTS=${REDIS_MAXCLIENTS:-10000}'
  assert_contains_once "$compose_file" '      - REDISCLI_AUTH=${REDIS_PASSWORD:-}'
  assert_contains_once "$compose_file" '--maxclients "$${REDIS_MAXCLIENTS}"'
  assert_contains_once "$compose_file" '$${REDISCLI_AUTH:+--requirepass "$$REDISCLI_AUTH"}'
done

assert_contains_once deploy/apple-container.sh 'POSTGRES_MAX_CONNECTIONS="$(read_env_value POSTGRES_MAX_CONNECTIONS 100)"'
assert_contains_once deploy/apple-container.sh '-c "max_connections=${POSTGRES_MAX_CONNECTIONS}"'
assert_contains_once deploy/apple-container.sh 'REDIS_MAXCLIENTS="$(read_env_value REDIS_MAXCLIENTS 10000)"'
assert_contains_once deploy/apple-container.sh '--maxclients "$REDIS_MAXCLIENTS"'
assert_contains_once deploy/docker-deploy.sh 'validate_application_configuration() {'
assert_contains_once deploy/docker-deploy.sh '    validate_application_configuration'

printf 'docker compose application environment test passed\n'
