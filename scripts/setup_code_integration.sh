#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for the first-run setup code (PR 30, cerberus batch 7).
#
# Starts throwaway Charon containers from charon:local and checks the
# first-account bootstrap path end to end:
#   TC-1: status reports setup required and setup code required
#   TC-2: setup without a code -> 403
#   TC-3: wrong code -> 403 with error_code setup_code_invalid
#   TC-4: correct code -> 201
#   TC-5: second setup after success -> 403 "Setup already completed";
#         status reports setup_code_required false
#   TC-6: more than 5 attempts from one client -> 429 (fresh container)
#   TC-7: a container started WITHOUT CHARON_SETUP_TOKEN logs exactly one
#         "Setup code:" line, and that logged code creates the first account
#
# Each run generates its own token and unique container and volume names.
# The setup code is never printed by this script: logs and failure output are
# redacted before they are shown.
#
# Assumes charon:local exists (CI loads it from the build artifact). If it is
# missing, the script builds it from the repository root.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# ============================================================================
# Configuration
# ============================================================================
IMAGE="charon:local"
RUN_ID="$(openssl rand -hex 4)"
MAIN_NAME="charon-setup-code-${RUN_ID}"
LOCKOUT_NAME="charon-setup-lockout-${RUN_ID}"
LOGGED_NAME="charon-setup-logged-${RUN_ID}"
CONTAINERS=("${MAIN_NAME}" "${LOCKOUT_NAME}" "${LOGGED_NAME}")

# Known setup code for the containers that preset CHARON_SETUP_TOKEN.
# 64 hex characters, well above the 32-character minimum.
SETUP_TOKEN="$(openssl rand -hex 32)"
export CHARON_SETUP_TOKEN="${SETUP_TOKEN}"

# Code read from the logged container. Set by TC-7, never printed.
LOGGED_CODE=""

# Response scratch file, removed on exit.
BODY_FILE="$(mktemp)"

ADMIN_EMAIL="setup-code-test@example.local"
ADMIN_PASSWORD="password123"

# Result of the most recent request, set by request().
HTTP_STATUS=""
HTTP_BODY=""

# ============================================================================
# Colors and counters
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_test() { echo -e "${BLUE}[TEST]${NC} $1"; }

PASSED=0
FAILED=0

pass() {
    PASSED=$((PASSED + 1))
    log_info "  ✓ $1"
}

fail() {
    FAILED=$((FAILED + 1))
    log_error "  ✗ $1"
}

# ============================================================================
# Helpers
# ============================================================================

# Replaces every secret value in stdin with a placeholder. Used for any log
# output that could contain a setup code.
redact() {
    local sed_args=(-e "s/${SETUP_TOKEN}/[REDACTED]/g")
    if [ -n "${LOGGED_CODE}" ]; then
        sed_args+=(-e "s/${LOGGED_CODE}/[REDACTED]/g")
    fi
    sed "${sed_args[@]}"
}

# Prints the last lines of each container's log, redacted. Only runs on failure.
dump_debug_info() {
    local name
    echo ""
    echo "=== FAILURE DEBUG INFO ==="
    for name in "${CONTAINERS[@]}"; do
        if docker inspect "${name}" >/dev/null 2>&1; then
            echo "=== Logs: ${name} (last 80 lines, redacted) ==="
            docker logs "${name}" 2>&1 | tail -n 80 | redact || true
            echo ""
        fi
    done
    echo "=== END DEBUG INFO ==="
}

cleanup() {
    local exit_code=$?
    if [ "${exit_code}" -ne 0 ]; then
        dump_debug_info
    fi

    log_info "Removing test containers and volumes..."
    local name
    for name in "${CONTAINERS[@]}"; do
        docker rm -f -v "${name}" >/dev/null 2>&1 || true
        docker volume rm "${name}-data" >/dev/null 2>&1 || true
    done
    rm -f "${BODY_FILE}" 2>/dev/null || true
    log_info "Cleanup complete"
    exit "${exit_code}"
}

# Starts one Charon container on a random loopback port. Extra arguments are
# passed to docker run (environment flags only).
start_charon() {
    local name=$1
    shift
    docker run -d --name "${name}" \
        --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
        -p 127.0.0.1::8080 \
        -e CHARON_ENV=development \
        -e CHARON_DEBUG=1 \
        -e CHARON_HTTP_PORT=8080 \
        -e CHARON_DB_PATH=/app/data/charon.db \
        -e CHARON_FRONTEND_DIR=/app/frontend/dist \
        -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
        -e CHARON_CADDY_BINARY=caddy \
        -v "${name}-data:/app/data" \
        "$@" \
        "${IMAGE}" >/dev/null
}

# Prints the host base URL for a container's API port.
base_url_of() {
    local mapping
    mapping="$(docker port "$1" 8080/tcp | head -n 1)"
    echo "http://127.0.0.1:${mapping##*:}"
}

# Waits until the container's health endpoint answers.
wait_for_api() {
    local name=$1
    local _attempt
    for _attempt in $(seq 1 60); do
        if curl -s -f "$(base_url_of "${name}")/api/v1/health" >/dev/null 2>&1; then
            log_info "Charon API ready in ${name}"
            return 0
        fi
        sleep 1
    done
    log_error "Charon API failed to start in ${name}"
    return 1
}

# Sends one JSON request to the given container. Sets HTTP_STATUS and HTTP_BODY.
# Arguments: container name, method, path, optional JSON body.
request() {
    local name=$1
    local method=$2
    local path=$3
    local data=${4:-}
    local args=(-s -o "${BODY_FILE}" -w '%{http_code}' -X "${method}" -H 'Content-Type: application/json')
    if [ -n "${data}" ]; then
        args+=(--data "${data}")
    fi
    HTTP_STATUS="$(curl "${args[@]}" "$(base_url_of "${name}")${path}")" || HTTP_STATUS=000
    HTTP_BODY="$(cat "${BODY_FILE}")"
}

# Builds a setup request body. Arguments: setup code ("" omits the field).
setup_payload() {
    local code=$1
    if [ -n "${code}" ]; then
        jq -cn --arg name "Setup Tester" --arg email "${ADMIN_EMAIL}" \
            --arg password "${ADMIN_PASSWORD}" --arg code "${code}" \
            '{name: $name, email: $email, password: $password, setup_code: $code}'
    else
        jq -cn --arg name "Setup Tester" --arg email "${ADMIN_EMAIL}" \
            --arg password "${ADMIN_PASSWORD}" \
            '{name: $name, email: $email, password: $password}'
    fi
}

# Asserts an HTTP status. Arguments: expected status, description.
assert_status() {
    local expected=$1
    local desc=$2
    if [ "${HTTP_STATUS}" = "${expected}" ]; then
        pass "${desc}: HTTP ${HTTP_STATUS}"
    else
        fail "${desc}: HTTP ${HTTP_STATUS} (expected ${expected})"
    fi
}

# Asserts a jq boolean expression over HTTP_BODY. Arguments: jq filter, description.
assert_body() {
    local filter=$1
    local desc=$2
    if echo "${HTTP_BODY}" | jq -e "${filter}" >/dev/null 2>&1; then
        pass "${desc}"
    else
        fail "${desc} (body: $(echo "${HTTP_BODY}" | jq -c 'del(.token)' 2>/dev/null || echo 'non-JSON'))"
    fi
}

# ============================================================================
# Pre-flight
# ============================================================================
for tool in docker curl jq openssl; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
        log_error "${tool} is not available; aborting"
        exit 1
    fi
done

trap cleanup EXIT

echo "=============================================="
echo "=== First-Run Setup Code Integration Test ==="
echo "=============================================="
echo ""

if ! docker image inspect "${IMAGE}" >/dev/null 2>&1; then
    log_info "Building ${IMAGE} image..."
    docker build -t "${IMAGE}" .
else
    log_info "Using existing ${IMAGE} image"
fi

# ============================================================================
# Main container: TC-1 to TC-5 (3 attempts count against the limiter)
# ============================================================================
log_info "Starting main container with a known CHARON_SETUP_TOKEN..."
start_charon "${MAIN_NAME}" -e CHARON_SETUP_TOKEN
wait_for_api "${MAIN_NAME}"

log_test "TC-1: Status reports setup and code required"
request "${MAIN_NAME}" GET /api/v1/setup
assert_status 200 "GET /api/v1/setup"
assert_body '.setupRequired == true and .setup_code_required == true' \
    "setupRequired and setup_code_required are both true"

log_test "TC-2: Setup without a code is refused"
request "${MAIN_NAME}" POST /api/v1/setup "$(setup_payload "")"
assert_status 403 "POST /api/v1/setup without setup_code"

log_test "TC-3: Wrong code is refused with setup_code_invalid"
# Prefixing the real token guarantees a wrong value of valid length and shape.
request "${MAIN_NAME}" POST /api/v1/setup "$(setup_payload "x${SETUP_TOKEN}")"
assert_status 403 "POST /api/v1/setup with wrong setup_code"
assert_body '.error_code == "setup_code_invalid"' "error_code is setup_code_invalid"

log_test "TC-4: Correct code creates the first account"
request "${MAIN_NAME}" POST /api/v1/setup "$(setup_payload "${SETUP_TOKEN}")"
assert_status 201 "POST /api/v1/setup with correct setup_code"

log_test "TC-5: Second setup is closed and status no longer requires a code"
request "${MAIN_NAME}" POST /api/v1/setup "$(setup_payload "${SETUP_TOKEN}")"
assert_status 403 "second POST /api/v1/setup"
assert_body '.error == "Setup already completed"' "error is 'Setup already completed'"
request "${MAIN_NAME}" GET /api/v1/setup
assert_status 200 "GET /api/v1/setup after setup"
assert_body '.setupRequired == false and .setup_code_required == false' \
    "setupRequired and setup_code_required are both false"

# ============================================================================
# Lockout container: TC-6 (fresh limiter)
# ============================================================================
log_test "TC-6: Repeated wrong codes from one client are throttled"
start_charon "${LOCKOUT_NAME}" -e CHARON_SETUP_TOKEN
wait_for_api "${LOCKOUT_NAME}"

# The limiter allows 5 attempts per client, then answers 429 before the code is checked.
for attempt in 1 2 3 4 5; do
    request "${LOCKOUT_NAME}" POST /api/v1/setup "$(setup_payload "x${SETUP_TOKEN}")"
    assert_status 403 "wrong code attempt ${attempt}"
done
for attempt in 6 7; do
    request "${LOCKOUT_NAME}" POST /api/v1/setup "$(setup_payload "x${SETUP_TOKEN}")"
    assert_status 429 "wrong code attempt ${attempt} (throttled)"
done

# ============================================================================
# Logged container: TC-7 (no CHARON_SETUP_TOKEN, code comes from the log)
# ============================================================================
log_test "TC-7: Container without CHARON_SETUP_TOKEN logs one setup code"
# Run without the token in the environment: unset it for this start only.
(
    unset CHARON_SETUP_TOKEN
    start_charon "${LOGGED_NAME}"
)
wait_for_api "${LOGGED_NAME}"

setup_code_lines="$(docker logs "${LOGGED_NAME}" 2>&1 | grep -c 'Setup code:' || true)"
if [ "${setup_code_lines}" = "1" ]; then
    pass "exactly one 'Setup code:' line logged"
else
    fail "expected exactly one 'Setup code:' line, found ${setup_code_lines}"
fi

# Read the code from the log without echoing it. Generated codes are base32, so the
# value ends at the first period that follows it.
LOGGED_CODE="$(docker logs "${LOGGED_NAME}" 2>&1 | sed -n 's/.*Setup code: \([A-Z2-7]*\)\..*/\1/p' | head -n 1)"
if [ -n "${LOGGED_CODE}" ]; then
    pass "logged setup code parsed (value masked)"
else
    fail "could not parse the logged setup code"
fi

request "${LOGGED_NAME}" GET /api/v1/setup
assert_body '.setup_code_required == true' "status requires the code"

request "${LOGGED_NAME}" POST /api/v1/setup "$(setup_payload "${LOGGED_CODE}")"
assert_status 201 "first account created with the logged code"

# The main container uses a preset token, so it must not log a code.
# Checks both the log line and the raw value, so a reworded log line cannot hide a leak.
main_code_lines="$(docker logs "${MAIN_NAME}" 2>&1 | grep -c 'Setup code:' || true)"
main_token_hits="$(docker logs "${MAIN_NAME}" 2>&1 | grep -c -F "${SETUP_TOKEN}" || true)"
if [ "${main_code_lines}" = "0" ] && [ "${main_token_hits}" = "0" ]; then
    pass "preset token is not written to the log"
else
    fail "CHARON_SETUP_TOKEN is written to the log (${main_code_lines} 'Setup code:' line(s), ${main_token_hits} raw value match(es), expected 0); see routes.go IssueStartupSetupCode path"
fi

# ============================================================================
# Results
# ============================================================================
echo ""
echo "=============================================="
echo "=== First-Run Setup Code Test Results ==="
echo "=============================================="
echo ""
echo -e "  ${GREEN}Passed:${NC}  ${PASSED}"
echo -e "  ${RED}Failed:${NC}  ${FAILED}"
echo ""

if [ "${FAILED}" -eq 0 ]; then
    echo "=== ALL FIRST-RUN SETUP CODE TESTS PASSED ==="
    exit 0
fi

echo "=== FIRST-RUN SETUP CODE TESTS FAILED ==="
exit 1
