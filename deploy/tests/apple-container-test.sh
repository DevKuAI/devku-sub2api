#!/bin/bash

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${TEST_DIR}/.." && pwd)"
SCRIPT="${DEPLOY_DIR}/apple-container.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-apple-test.XXXXXX")"
STATE_DIR="${TEST_ROOT}/state"
ENV_FILE="${TEST_ROOT}/sub2api.env"

cleanup() {
    rm -rf "${TEST_ROOT}"
}
trap cleanup EXIT

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_exists() {
    [[ -e "$1" ]] || fail "Expected path to exist: $1"
}

assert_missing() {
    [[ ! -e "$1" ]] || fail "Expected path to be absent: $1"
}

export FAKE_CONTAINER_STATE="${STATE_DIR}"
export PATH="${TEST_DIR}/fixtures/bin:${PATH}"
export SUB2API_ENV_FILE="${ENV_FILE}"

mkdir -p "${STATE_DIR}"

"${SCRIPT}" init
[[ "$(stat -f '%Lp' "${ENV_FILE}")" == "600" ]] || fail "init did not create a mode-600 env file"
grep -q '^POSTGRES_PASSWORD=change_this_secure_password$' "${ENV_FILE}" && fail "init retained the placeholder password"

chmod 644 "${ENV_FILE}"
if "${SCRIPT}" up >/dev/null 2>&1; then
    fail "up accepted an insecure env file"
fi
chmod 600 "${ENV_FILE}"

"${SCRIPT}" up
assert_exists "${STATE_DIR}/containers/sub2api-apple"
assert_exists "${STATE_DIR}/containers/sub2api-apple-postgres"
assert_exists "${STATE_DIR}/containers/sub2api-apple-redis"
assert_exists "${STATE_DIR}/running/sub2api-apple"
grep -Fxq 'max_connections=1024' "${STATE_DIR}/create-args/sub2api-apple-postgres" || fail "PostgreSQL max_connections tuning was not applied"
grep -Fxq 'shared_buffers=1GB' "${STATE_DIR}/create-args/sub2api-apple-postgres" || fail "PostgreSQL shared_buffers tuning was not applied"
grep -Fxq 'effective_cache_size=4GB' "${STATE_DIR}/create-args/sub2api-apple-postgres" || fail "PostgreSQL effective_cache_size tuning was not applied"
grep -Fxq 'maintenance_work_mem=128MB' "${STATE_DIR}/create-args/sub2api-apple-postgres" || fail "PostgreSQL maintenance_work_mem tuning was not applied"
grep -Fxq 'REDIS_MAXCLIENTS=50000' "${STATE_DIR}/env-files/sub2api-apple-redis" || fail "Redis maxclients tuning was not applied"
grep -Fq -- '--maxclients "$REDIS_MAXCLIENTS"' "${STATE_DIR}/create-args/sub2api-apple-redis" || fail "Redis maxclients command was not applied"
"${SCRIPT}" status >/dev/null

"${SCRIPT}" up --recreate
assert_exists "${STATE_DIR}/running/sub2api-apple"
"${SCRIPT}" down
assert_missing "${STATE_DIR}/running/sub2api-apple"
assert_missing "${STATE_DIR}/running/sub2api-apple-postgres"
assert_missing "${STATE_DIR}/running/sub2api-apple-redis"

"${SCRIPT}" destroy --yes
assert_missing "${STATE_DIR}/containers/sub2api-apple"
assert_missing "${STATE_DIR}/networks/sub2api-apple"
assert_exists "${STATE_DIR}/volumes/sub2api-apple-data"

"${SCRIPT}" up
"${SCRIPT}" destroy --volumes --yes
assert_missing "${STATE_DIR}/volumes/sub2api-apple-data"
assert_missing "${STATE_DIR}/volumes/sub2api-apple-postgres-data"
assert_missing "${STATE_DIR}/volumes/sub2api-apple-redis-data"

touch "${STATE_DIR}/system-running"
touch "${STATE_DIR}/containers/sub2api-apple"
touch "${STATE_DIR}/unowned/container/sub2api-apple"
if "${SCRIPT}" status >/dev/null 2>&1; then
    fail "status accepted an unowned same-name container"
fi

printf 'Apple container lifecycle tests passed.\n'
