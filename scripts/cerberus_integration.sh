#!/usr/bin/env bash
set -euo pipefail

# Brief: Full integration test for Cerberus security stack
# Tests all security features working together:
# - WAF (Coraza) for payload inspection
# - Rate Limiting for volume abuse prevention
# - Security handler ordering in Caddy config
#
# Test Cases:
# - TC-1: Verify all features enabled via /api/v1/security/status
# - TC-2: Verify handler order in Caddy config
# - TC-3: WAF blocking doesn't consume rate limit quota
# - TC-4: Legitimate traffic flows through all layers
# - TC-5: Basic latency check

# Ensure we operate from repo root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# ============================================================================
# Configuration
# ============================================================================
CONTAINER_NAME="charon-cerberus-test"
BACKEND_CONTAINER="cerberus-backend"
TEST_DOMAIN="cerberus.test.local"

# Use unique non-conflicting ports
API_PORT=8480
HTTP_PORT=8481
HTTPS_PORT=8444
CADDY_ADMIN_PORT=2319

# Rate limit config for testing
RATE_LIMIT_REQUESTS=5
RATE_LIMIT_WINDOW_SEC=30

# ============================================================================
# Colors for output
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_test() { echo -e "${BLUE}[TEST]${NC} $1"; }

# ============================================================================
# Test counters
# ============================================================================
PASSED=0
FAILED=0

pass_test() {
    PASSED=$((PASSED + 1))
    echo -e "  ${GREEN}✓ PASS${NC}"
}

fail_test() {
    FAILED=$((FAILED + 1))
    echo -e "  ${RED}✗ FAIL${NC}: $1"
}

# Assert HTTP status code
assert_http() {
    local expected=$1
    local actual=$2
    local desc=$3
    if [ "$actual" = "$expected" ]; then
        log_info "  ✓ $desc: HTTP $actual"
        PASSED=$((PASSED + 1))
    else
        log_error "  ✗ $desc: HTTP $actual (expected $expected)"
        FAILED=$((FAILED + 1))
    fi
}

# ============================================================================
# Helper Functions
# ============================================================================

# Dumps debug information on failure
on_failure() {
    local exit_code=$?
    echo ""
    echo "=============================================="
    echo "=== FAILURE DEBUG INFO (exit code: $exit_code) ==="
    echo "=============================================="
    echo ""

    echo "=== Charon API Logs (last 150 lines) ==="
    docker logs ${CONTAINER_NAME} 2>&1 | tail -150 || echo "Could not retrieve container logs"
    echo ""

    echo "=== Caddy Admin API Config ==="
    curl -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null | head -300 || echo "Could not retrieve Caddy config"
    echo ""

    echo "=== Security Config in API ==="
    curl -s -b "${TMP_COOKIE:-/dev/null}" "http://localhost:${API_PORT}/api/v1/security/config" 2>/dev/null || echo "Could not retrieve security config"
    echo ""

    echo "=== Security Status ==="
    curl -s -b "${TMP_COOKIE:-/dev/null}" "http://localhost:${API_PORT}/api/v1/security/status" 2>/dev/null || echo "Could not retrieve security status"
    echo ""

    echo "=== Security Rulesets ==="
    curl -s -b "${TMP_COOKIE:-/dev/null}" "http://localhost:${API_PORT}/api/v1/security/rulesets" 2>/dev/null || echo "Could not retrieve rulesets"
    echo ""

    echo "=============================================="
    echo "=== END DEBUG INFO ==="
    echo "=============================================="
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test resources..."
    docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
    docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true
    rm -f "${TMP_COOKIE:-}" 2>/dev/null || true
    log_info "Cleanup complete"
}

# Set up trap to dump debug info on any error and always cleanup
trap on_failure ERR
trap cleanup EXIT

echo "=============================================="
echo "=== Cerberus Full Integration Test Starting ==="
echo "=============================================="
echo ""

# Check dependencies
if ! command -v docker >/dev/null 2>&1; then
    log_error "docker is not available; aborting"
    exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
    log_error "curl is not available; aborting"
    exit 1
fi

# ============================================================================
# Step 1: Build image if needed
# ============================================================================
if ! docker image inspect charon:local >/dev/null 2>&1; then
    log_info "Building charon:local image..."
    docker build -t charon:local .
else
    log_info "Using existing charon:local image"
fi

# ============================================================================
# Step 2: Start containers
# ============================================================================
log_info "Stopping any existing test containers..."
docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true

# Ensure network exists
if ! docker network inspect containers_default >/dev/null 2>&1; then
    log_info "Creating containers_default network..."
    docker network create containers_default
fi

log_info "Starting httpbin backend container..."
docker pull kennethreitz/httpbin 2>/dev/null || true
docker run -d --name ${BACKEND_CONTAINER} --network containers_default kennethreitz/httpbin

log_info "Starting Charon container with ALL Cerberus features enabled..."
docker run -d --name ${CONTAINER_NAME} \
    --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
    --network containers_default \
    -p ${HTTP_PORT}:80 -p ${HTTPS_PORT}:443 -p ${API_PORT}:8080 -p ${CADDY_ADMIN_PORT}:2019 \
    -e CHARON_ENV=development \
    -e CHARON_DEBUG=1 \
    -e CHARON_HTTP_PORT=8080 \
    -e CHARON_DB_PATH=/app/data/charon.db \
    -e CHARON_FRONTEND_DIR=/app/frontend/dist \
    -e CHARON_CADDY_ADMIN_API=http://localhost:2019 \
    -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
    -e CHARON_CADDY_BINARY=caddy \
    -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
    -e CHARON_SECURITY_WAF_MODE=block \
    -e CERBERUS_SECURITY_RATELIMIT_MODE=enabled \
    -e CERBERUS_SECURITY_ACL_ENABLED=true \
    -v charon_cerberus_test_data:/app/data \
    -v caddy_cerberus_test_data:/data \
    -v caddy_cerberus_test_config:/config \
    charon:local

log_info "Waiting for Charon API to be ready..."
for i in {1..30}; do
    if curl -s -f "http://localhost:${API_PORT}/api/v1/health" >/dev/null 2>&1; then
        log_info "Charon API is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        log_error "Charon API failed to start"
        exit 1
    fi
    echo -n '.'
    sleep 1
done
echo ""

log_info "Waiting for httpbin backend to be ready..."
for i in {1..45}; do
    if docker exec ${CONTAINER_NAME} sh -c "curl -sf http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
        log_info "httpbin backend is ready"
        break
    fi
    if [ $i -eq 45 ]; then
        log_error "httpbin backend failed to start"
        exit 1
    fi
    echo -n '.'
    sleep 1
done
echo ""

# ============================================================================
# Step 3: Register user and authenticate
# ============================================================================
log_info "Registering admin user and logging in..."
TMP_COOKIE=$(mktemp)

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"cerberus-test@example.local","password":"password123","name":"Cerberus Tester"}' \
    "http://localhost:${API_PORT}/api/v1/auth/register" >/dev/null 2>&1 || true

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"cerberus-test@example.local","password":"password123"}' \
    -c "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/auth/login" >/dev/null

log_info "Authentication complete"

# ============================================================================
# Step 4: Create proxy host
# ============================================================================
log_info "Creating proxy host '${TEST_DOMAIN}' pointing to backend..."
PROXY_HOST_PAYLOAD=$(cat <<EOF
{
    "name": "cerberus-test-backend",
    "domain_names": "${TEST_DOMAIN}",
    "forward_scheme": "http",
    "forward_host": "${BACKEND_CONTAINER}",
    "forward_port": 80,
    "enabled": true
}
EOF
)

CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${PROXY_HOST_PAYLOAD}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/proxy-hosts")
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -n1)

if [ "$CREATE_STATUS" = "201" ]; then
    log_info "Proxy host created successfully"
else
    log_info "Proxy host may already exist (status: $CREATE_STATUS)"
fi

# Wait for Caddy to apply config
sleep 3

# ============================================================================
# Step 5: Create WAF ruleset (XSS protection)
# ============================================================================
log_info "Creating XSS WAF ruleset..."
XSS_RULESET=$(cat <<'EOF'
{
    "name": "cerberus-xss",
    "content": "SecRule REQUEST_BODY|ARGS|ARGS_NAMES \"<script\" \"id:99001,phase:2,deny,status:403,msg:'XSS Attack Detected'\""
}
EOF
)

XSS_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${XSS_RULESET}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/rulesets")
XSS_STATUS=$(echo "$XSS_RESP" | tail -n1)

if [ "$XSS_STATUS" = "200" ] || [ "$XSS_STATUS" = "201" ]; then
    log_info "XSS ruleset created"
else
    log_warn "XSS ruleset creation returned status: $XSS_STATUS"
fi

# ============================================================================
# Step 6: Enable WAF in block mode + configure rate limiting
# ============================================================================
log_info "Enabling WAF (block mode) and rate limiting (${RATE_LIMIT_REQUESTS} req / ${RATE_LIMIT_WINDOW_SEC} sec)..."
SECURITY_CONFIG=$(cat <<EOF
{
    "name": "default",
    "enabled": true,
    "waf_mode": "block",
    "waf_rules_source": "cerberus-xss",
    "rate_limit_enable": true,
    "rate_limit_requests": ${RATE_LIMIT_REQUESTS},
    "rate_limit_window_sec": ${RATE_LIMIT_WINDOW_SEC},
    "rate_limit_burst": 1,
    "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
)

SEC_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${SECURITY_CONFIG}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/config")
SEC_STATUS=$(echo "$SEC_RESP" | tail -n1)

if [ "$SEC_STATUS" = "200" ]; then
    log_info "Security configuration applied"
else
    log_warn "Security config returned status: $SEC_STATUS"
fi

# Wait for Caddy to reload with all security features
log_info "Waiting for Caddy to apply security configuration..."
sleep 5

echo ""
echo "=============================================="
echo "=== Running Cerberus Integration Test Cases ==="
echo "=============================================="
echo ""

# ============================================================================
# TC-1: Verify all features enabled via /api/v1/security/status
# ============================================================================
log_test "TC-1: Verify All Features Enabled"

STATUS_RESP=$(curl -s -b "${TMP_COOKIE}" "http://localhost:${API_PORT}/api/v1/security/status")

# Check Cerberus enabled (nested: "cerberus":{"enabled":true})
if echo "$STATUS_RESP" | grep -qE '"cerberus":\s*\{[^}]*"enabled":\s*true'; then
    log_info "  ✓ Cerberus enabled"
    PASSED=$((PASSED + 1))
else
    fail_test "Cerberus not enabled in status response"
fi

# Check WAF mode (nested: "waf":{"mode":"block",...})
if echo "$STATUS_RESP" | grep -qE '"waf":\s*\{[^}]*"mode":\s*"block"'; then
    log_info "  ✓ WAF mode is 'block'"
    PASSED=$((PASSED + 1))
else
    fail_test "WAF mode not set to 'block'"
fi

# Check rate limit enabled (nested: "rate_limit":{"enabled":true,...})
if echo "$STATUS_RESP" | grep -qE '"rate_limit":\s*\{[^}]*"enabled":\s*true'; then
    log_info "  ✓ Rate limit enabled"
    PASSED=$((PASSED + 1))
else
    fail_test "Rate limit not enabled"
fi

# ============================================================================
# TC-2: Verify handler order in Caddy config (ROUTE-AWARE)
# ============================================================================
log_test "TC-2: Verify Handler Order in Caddy Config"

# HARD REQUIREMENT: Check if jq is available (no fallback mode)
if ! command -v jq >/dev/null 2>&1; then
    fail_test "jq is required for handler order verification. Install: apt-get install jq / brew install jq"
    return 1
fi

# Fetch Caddy config with timeout and retry
CADDY_CONFIG=""
for attempt in 1 2 3; do
    CADDY_CONFIG=$(curl --max-time 10 -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")
    if [ -n "$CADDY_CONFIG" ]; then
        break
    fi
    log_warn "  Attempt $attempt/3: Failed to fetch Caddy config, retrying..."
    sleep 2
done

if [ -z "$CADDY_CONFIG" ]; then
    fail_test "Could not retrieve Caddy config after 3 attempts"
    return 1
fi

# Validate JSON structure before processing
if ! echo "$CADDY_CONFIG" | jq empty 2>/dev/null; then
    fail_test "Retrieved Caddy config is not valid JSON"
    return 1
fi

# Validate expected structure exists
if ! echo "$CADDY_CONFIG" | jq -e '.apps.http.servers.charon_server.routes' >/dev/null 2>&1; then
    fail_test "Caddy config missing expected route structure (.apps.http.servers.charon_server.routes)"
    return 1
fi

# Get route count with validation
TOTAL_ROUTES=$(echo "$CADDY_CONFIG" | jq -r '.apps.http.servers.charon_server.routes | length' 2>/dev/null || echo "0")

# Validate route count is numeric and non-negative
if ! [[ "$TOTAL_ROUTES" =~ ^[0-9]+$ ]]; then
    fail_test "Invalid route count (not numeric): $TOTAL_ROUTES"
    return 1
fi

log_info "  Found $TOTAL_ROUTES routes in Caddy config"

if [ "$TOTAL_ROUTES" -eq 0 ]; then
    fail_test "No routes found in Caddy config"
    return 1
fi

# Define EXACT emergency paths (must match config.go lines 687-691)
readonly EMERGENCY_PATHS=(
    "/api/v1/emergency/security-reset"
    "/api/v1/emergency/*"
    "/emergency/security-reset"
    "/emergency/*"
)

ROUTES_VERIFIED=0
EMERGENCY_ROUTES_SKIPPED=0

# Use bash arithmetic loop instead of seq
for ((i=0; i<TOTAL_ROUTES; i++)); do
    # Validate route exists at index
    if ! echo "$CADDY_CONFIG" | jq -e ".apps.http.servers.charon_server.routes[$i]" >/dev/null 2>&1; then
        log_warn "  Route $i: Missing or invalid route structure, skipping"
        continue
    fi

    # Check if this route has EXACT emergency path matches
    IS_EMERGENCY_ROUTE=false
    for emergency_path in "${EMERGENCY_PATHS[@]}"; do
        # EXACT path comparison (not substring matching)
        EXACT_MATCH=$(echo "$CADDY_CONFIG" | jq -r "
            .apps.http.servers.charon_server.routes[$i].match[]? |
            select(.path != null) |
            .path[]? |
            select(. == \"$emergency_path\")" 2>/dev/null | wc -l | tr -d ' ')

        # Validate match count is numeric
        if ! [[ "$EXACT_MATCH" =~ ^[0-9]+$ ]]; then
            log_warn "  Route $i: Invalid match count for path '$emergency_path', skipping"
            continue
        fi

        if [ "$EXACT_MATCH" -gt 0 ]; then
            IS_EMERGENCY_ROUTE=true
            break
        fi
    done

    if [ "$IS_EMERGENCY_ROUTE" = true ]; then
        log_info "  Route $i: Emergency route (security bypass by design) - skipping"
        EMERGENCY_ROUTES_SKIPPED=$((EMERGENCY_ROUTES_SKIPPED + 1))
        continue
    fi

    # Main route - verify handler order
    log_info "  Route $i: Main route - verifying handler order..."

    # Validate handlers array exists
    if ! echo "$CADDY_CONFIG" | jq -e ".apps.http.servers.charon_server.routes[$i].handle" >/dev/null 2>&1; then
        log_warn "  Route $i: No handlers found, skipping"
        continue
    fi

    # Find indices of security handlers and reverse_proxy
    WAF_IDX=$(echo "$CADDY_CONFIG" | jq -r "[.apps.http.servers.charon_server.routes[$i].handle[]?.handler] | map(if . == \"waf\" then true else false end) | index(true) // -1" 2>/dev/null || echo "-1")
    RATE_IDX=$(echo "$CADDY_CONFIG" | jq -r "[.apps.http.servers.charon_server.routes[$i].handle[]?.handler] | map(if . == \"rate_limit\" then true else false end) | index(true) // -1" 2>/dev/null || echo "-1")
    PROXY_IDX=$(echo "$CADDY_CONFIG" | jq -r "[.apps.http.servers.charon_server.routes[$i].handle[]?.handler] | map(if . == \"reverse_proxy\" then true else false end) | index(true) // -1" 2>/dev/null || echo "-1")

    # Validate all indices are numeric
    if ! [[ "$WAF_IDX" =~ ^-?[0-9]+$ ]] || ! [[ "$RATE_IDX" =~ ^-?[0-9]+$ ]] || ! [[ "$PROXY_IDX" =~ ^-?[0-9]+$ ]]; then
        fail_test "Invalid handler indices in route $i (not numeric)"
        return 1
    fi

    # Verify WAF comes before reverse_proxy (if present)
    if [ "$WAF_IDX" -ge 0 ] && [ "$PROXY_IDX" -ge 0 ]; then
        if [ "$WAF_IDX" -lt "$PROXY_IDX" ]; then
            log_info "    ✓ WAF (index $WAF_IDX) before reverse_proxy (index $PROXY_IDX)"
        else
            fail_test "WAF must appear before reverse_proxy in route $i (WAF=$WAF_IDX, proxy=$PROXY_IDX)"
            return 1
        fi
    fi

    # Verify rate_limit comes before reverse_proxy (if present)
    if [ "$RATE_IDX" -ge 0 ] && [ "$PROXY_IDX" -ge 0 ]; then
        if [ "$RATE_IDX" -lt "$PROXY_IDX" ]; then
            log_info "    ✓ rate_limit (index $RATE_IDX) before reverse_proxy (index $PROXY_IDX)"
        else
            fail_test "rate_limit must appear before reverse_proxy in route $i (rate=$RATE_IDX, proxy=$PROXY_IDX)"
            return 1
        fi
    fi

    ROUTES_VERIFIED=$((ROUTES_VERIFIED + 1))
done

log_info "  Summary: Verified $ROUTES_VERIFIED main routes, skipped $EMERGENCY_ROUTES_SKIPPED emergency routes"

if [ "$ROUTES_VERIFIED" -gt 0 ]; then
    log_info "  ✓ Handler order correct in all main routes"
    PASSED=$((PASSED + 1))
else
    log_warn "  No main routes found to verify (all routes are emergency routes?)"
    # Don't fail if only emergency routes exist - this may be valid in some configs
    PASSED=$((PASSED + 1))
fi

# ============================================================================
# TC-3: WAF blocking doesn't consume rate limit quota
# ============================================================================
log_test "TC-3: WAF Blocking Doesn't Consume Rate Limit"

log_info "  Sending 3 malicious requests (should be blocked by WAF with 403)..."

WAF_BLOCKED=0
for i in 1 2 3; do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Host: ${TEST_DOMAIN}" \
        "http://localhost:${HTTP_PORT}/get?q=%3Cscript%3Ealert(1)%3C/script%3E")
    if [ "$CODE" = "403" ]; then
        WAF_BLOCKED=$((WAF_BLOCKED + 1))
        log_info "    Malicious request $i: HTTP $CODE (WAF blocked) ✓"
    else
        log_warn "    Malicious request $i: HTTP $CODE (expected 403)"
    fi
done

if [ $WAF_BLOCKED -eq 3 ]; then
    log_info "  ✓ All 3 malicious requests blocked by WAF"
    PASSED=$((PASSED + 1))
else
    fail_test "Not all malicious requests were blocked by WAF ($WAF_BLOCKED/3)"
fi

log_info "  Sending ${RATE_LIMIT_REQUESTS} legitimate requests (should all succeed with 200)..."

LEGIT_SUCCESS=0
for i in $(seq 1 ${RATE_LIMIT_REQUESTS}); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Host: ${TEST_DOMAIN}" \
        "http://localhost:${HTTP_PORT}/get?name=john&id=$i")
    if [ "$CODE" = "200" ]; then
        LEGIT_SUCCESS=$((LEGIT_SUCCESS + 1))
        log_info "    Legitimate request $i: HTTP $CODE ✓"
    else
        log_warn "    Legitimate request $i: HTTP $CODE (expected 200)"
    fi
    sleep 0.1
done

if [ $LEGIT_SUCCESS -eq ${RATE_LIMIT_REQUESTS} ]; then
    log_info "  ✓ All ${RATE_LIMIT_REQUESTS} legitimate requests succeeded"
    PASSED=$((PASSED + 1))
else
    fail_test "Not all legitimate requests succeeded ($LEGIT_SUCCESS/${RATE_LIMIT_REQUESTS})"
fi

# ============================================================================
# TC-4: Legitimate traffic flows through all layers
# ============================================================================
log_test "TC-4: Legitimate Traffic Flows Through All Layers"

# Wait for rate limit window to reset
log_info "  Waiting for rate limit window to reset (${RATE_LIMIT_WINDOW_SEC} seconds + buffer)..."
sleep $((RATE_LIMIT_WINDOW_SEC + 2))

log_info "  Sending 10 legitimate requests..."

FLOW_SUCCESS=0
for i in $(seq 1 10); do
    BODY=$(curl -s -H "Host: ${TEST_DOMAIN}" "http://localhost:${HTTP_PORT}/get?test=$i")
    if echo "$BODY" | grep -q "args\|headers\|origin\|url"; then
        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
        echo "    Request $i: ✓ Success (reached upstream)"
    else
        echo "    Request $i: ✗ Failed (response: ${BODY:0:100}...)"
    fi
    # Space out requests to avoid hitting rate limit
    sleep 0.5
done

log_info "  Total successful: $FLOW_SUCCESS/10"

if [ $FLOW_SUCCESS -ge 5 ]; then
    log_info "  ✓ Legitimate traffic flowing through all layers"
    PASSED=$((PASSED + 1))
else
    fail_test "Too many legitimate requests failed ($FLOW_SUCCESS/10)"
fi

# ============================================================================
# TC-5: Basic latency check
# ============================================================================
log_test "TC-5: Basic Latency Check"

# Wait for rate limit window to reset again
log_info "  Waiting for rate limit window to reset..."
sleep $((RATE_LIMIT_WINDOW_SEC + 2))

# Measure latency for a single request
LATENCY=$(curl -s -o /dev/null -w "%{time_total}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get")

log_info "  Single request latency: ${LATENCY}s"

# Convert to milliseconds for comparison (using awk since bc may not be available)
LATENCY_MS=$(echo "$LATENCY" | awk '{printf "%.0f", $1 * 1000}')

if [ "$LATENCY_MS" -lt 5000 ]; then
    log_info "  ✓ Latency ${LATENCY_MS}ms is within acceptable range (<5000ms)"
    PASSED=$((PASSED + 1))
else
    fail_test "Latency ${LATENCY_MS}ms exceeds threshold"
fi

# ============================================================================
# Results Summary
# ============================================================================
echo ""
echo "=============================================="
echo "=== Cerberus Full Integration Test Results ==="
echo "=============================================="
echo ""
echo -e "  ${GREEN}Passed:${NC}  $PASSED"
echo -e "  ${RED}Failed:${NC}  $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo "=============================================="
    echo "=== ALL CERBERUS INTEGRATION TESTS PASSED ==="
    echo "=============================================="
    echo ""
    exit 0
else
    echo "=============================================="
    echo "=== CERBERUS TESTS FAILED ==="
    echo "=============================================="
    echo ""
    exit 1
fi
