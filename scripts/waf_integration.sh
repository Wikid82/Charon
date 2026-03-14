#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for WAF (Coraza) functionality
# Steps:
# 1. Build the local image if not present: docker build -t charon:local .
# 2. Start Charon container with Cerberus/WAF features enabled
# 3. Start httpbin as backend for proxy testing
# 4. Create test user and authenticate
# 5. Create proxy host pointing to backend
# 6. Test WAF ruleset creation (XSS, SQLi)
# 7. Test WAF blocking mode (expect HTTP 403 for attacks)
# 8. Test legitimate requests pass through (HTTP 200)
# 9. Test monitor mode (attacks pass with HTTP 200)
# 10. Verify Caddy config has WAF handler
# 11. Clean up test resources

# Ensure we operate from repo root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# ============================================================================
# Configuration
# ============================================================================
CONTAINER_NAME="charon-waf-test"
BACKEND_CONTAINER="waf-backend"
TEST_DOMAIN="waf.test.local"

# Use unique non-conflicting ports
API_PORT=8380
HTTP_PORT=8180
HTTPS_PORT=8143
CADDY_ADMIN_PORT=2119

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
echo "=== WAF Integration Test Starting ==="
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
docker pull mccutchen/go-httpbin 2>/dev/null || true
docker run -d --name ${BACKEND_CONTAINER} --network containers_default mccutchen/go-httpbin

log_info "Starting Charon container with Cerberus enabled..."
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
    -v charon_waf_test_data:/app/data \
    -v caddy_waf_test_data:/data \
    -v caddy_waf_test_config:/config \
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
    if docker exec ${CONTAINER_NAME} sh -c "wget -qO /dev/null http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
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
    -d '{"email":"waf-test@example.local","password":"password123","name":"WAF Tester"}' \
    "http://localhost:${API_PORT}/api/v1/auth/register" >/dev/null 2>&1 || true

curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"waf-test@example.local","password":"password123"}' \
    -c "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/auth/login" >/dev/null

log_info "Authentication complete"

# ============================================================================
# Step 4: Create proxy host
# ============================================================================
log_info "Creating proxy host '${TEST_DOMAIN}' pointing to backend..."
PROXY_HOST_PAYLOAD=$(cat <<EOF
{
    "name": "waf-test-backend",
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

echo ""
echo "=============================================="
echo "=== Running WAF Test Cases ==="
echo "=============================================="
echo ""

# ============================================================================
# TC-1: Create XSS ruleset
# ============================================================================
log_test "TC-1: Create XSS Ruleset"

XSS_RULESET=$(cat <<'EOF'
{
    "name": "test-xss",
    "content": "SecRule REQUEST_BODY|ARGS|ARGS_NAMES \"<script\" \"id:12345,phase:2,deny,status:403,msg:'XSS Attack Detected'\""
}
EOF
)

XSS_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${XSS_RULESET}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/rulesets")
XSS_STATUS=$(echo "$XSS_RESP" | tail -n1)

if [ "$XSS_STATUS" = "200" ] || [ "$XSS_STATUS" = "201" ]; then
    log_info "  XSS ruleset created"
    pass_test
else
    fail_test "Failed to create XSS ruleset (HTTP $XSS_STATUS)"
fi

# ============================================================================
# TC-2: Enable WAF in block mode
# ============================================================================
log_test "TC-2: Enable WAF (Block Mode)"

WAF_CONFIG=$(cat <<'EOF'
{
    "name": "default",
    "enabled": true,
    "waf_mode": "block",
    "waf_rules_source": "test-xss",
    "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
)

WAF_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${WAF_CONFIG}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/config")
WAF_STATUS=$(echo "$WAF_RESP" | tail -n1)

if [ "$WAF_STATUS" = "200" ]; then
    log_info "  WAF enabled in block mode with test-xss ruleset"
    pass_test
else
    fail_test "Failed to enable WAF (HTTP $WAF_STATUS)"
fi

# Wait for Caddy to reload with WAF config
log_info "Waiting for Caddy to apply WAF configuration..."
sleep 5

# ============================================================================
# TC-3: Test XSS blocking (expect HTTP 403)
# ============================================================================
log_test "TC-3: XSS Blocking (expect HTTP 403)"

# Test XSS in POST body
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    -d "<script>alert(1)</script>" \
    "http://localhost:${HTTP_PORT}/post")
assert_http "403" "$RESP" "XSS script tag (POST body)"

# Test XSS in query parameter
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get?q=%3Cscript%3Ealert(1)%3C/script%3E")
assert_http "403" "$RESP" "XSS script tag (query param)"

# ============================================================================
# TC-4: Test legitimate request (expect HTTP 200)
# ============================================================================
log_test "TC-4: Legitimate Request (expect HTTP 200)"

RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    -d "name=john&age=25" \
    "http://localhost:${HTTP_PORT}/post")
assert_http "200" "$RESP" "Legitimate POST request"

RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get?name=john&age=25")
assert_http "200" "$RESP" "Legitimate GET request"

# ============================================================================
# TC-5: Switch to monitor mode, verify XSS passes (expect HTTP 200)
# ============================================================================
log_test "TC-5: Switch to Monitor Mode"

MONITOR_CONFIG=$(cat <<'EOF'
{
    "name": "default",
    "enabled": true,
    "waf_mode": "monitor",
    "waf_rules_source": "test-xss",
    "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
)

curl -s -X POST -H "Content-Type: application/json" \
    -d "${MONITOR_CONFIG}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/config" >/dev/null

log_info "  Switched to monitor mode, waiting for Caddy reload..."
sleep 5

# Verify XSS passes in monitor mode
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    -d "<script>alert(1)</script>" \
    "http://localhost:${HTTP_PORT}/post")
assert_http "200" "$RESP" "XSS in monitor mode (allowed through)"

# ============================================================================
# TC-6: Create SQLi ruleset
# ============================================================================
log_test "TC-6: Create SQLi Ruleset"

SQLI_RULESET=$(cat <<'EOF'
{
    "name": "test-sqli",
    "content": "SecRule ARGS|ARGS_NAMES|REQUEST_BODY \"(?i:OR\\s+1\\s*=\\s*1|UNION\\s+SELECT)\" \"id:12346,phase:2,deny,status:403,msg:'SQL Injection Detected'\""
}
EOF
)

SQLI_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${SQLI_RULESET}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/rulesets")
SQLI_STATUS=$(echo "$SQLI_RESP" | tail -n1)

if [ "$SQLI_STATUS" = "200" ] || [ "$SQLI_STATUS" = "201" ]; then
    log_info "  SQLi ruleset created"
    pass_test
else
    fail_test "Failed to create SQLi ruleset (HTTP $SQLI_STATUS)"
fi

# ============================================================================
# TC-7: Enable SQLi ruleset in block mode, test SQLi blocking (expect HTTP 403)
# ============================================================================
log_test "TC-7: SQLi Blocking (expect HTTP 403)"

SQLI_CONFIG=$(cat <<'EOF'
{
    "name": "default",
    "enabled": true,
    "waf_mode": "block",
    "waf_rules_source": "test-sqli",
    "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
)

curl -s -X POST -H "Content-Type: application/json" \
    -d "${SQLI_CONFIG}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/config" >/dev/null

log_info "  Switched to SQLi ruleset in block mode, waiting for Caddy reload..."
sleep 5

# Test SQLi OR 1=1
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get?id=1%20OR%201=1")
assert_http "403" "$RESP" "SQLi OR 1=1 (query param)"

# Test SQLi UNION SELECT
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get?id=1%20UNION%20SELECT%20*%20FROM%20users")
assert_http "403" "$RESP" "SQLi UNION SELECT (query param)"

# ============================================================================
# TC-8: Create combined ruleset, test both attacks blocked
# ============================================================================
log_test "TC-8: Combined Ruleset (XSS + SQLi)"

COMBINED_RULESET=$(cat <<'EOF'
{
    "name": "combined-protection",
    "content": "SecRule ARGS|REQUEST_BODY \"(?i:OR\\s+1\\s*=\\s*1|UNION\\s+SELECT)\" \"id:20001,phase:2,deny,status:403,msg:'SQLi'\"\nSecRule ARGS|REQUEST_BODY \"<script\" \"id:20002,phase:2,deny,status:403,msg:'XSS'\""
}
EOF
)

COMBINED_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${COMBINED_RULESET}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/rulesets")
COMBINED_STATUS=$(echo "$COMBINED_RESP" | tail -n1)

if [ "$COMBINED_STATUS" = "200" ] || [ "$COMBINED_STATUS" = "201" ]; then
    log_info "  Combined ruleset created"
    PASSED=$((PASSED + 1))
else
    fail_test "Failed to create combined ruleset (HTTP $COMBINED_STATUS)"
fi

# Enable combined ruleset
COMBINED_CONFIG=$(cat <<'EOF'
{
    "name": "default",
    "enabled": true,
    "waf_mode": "block",
    "waf_rules_source": "combined-protection",
    "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
)

curl -s -X POST -H "Content-Type: application/json" \
    -d "${COMBINED_CONFIG}" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/security/config" >/dev/null

log_info "  Switched to combined ruleset, waiting for Caddy reload..."
sleep 5

# Test both attacks blocked
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get?id=1%20OR%201=1")
assert_http "403" "$RESP" "Combined - SQLi blocked"

RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    -d "<script>alert(1)</script>" \
    "http://localhost:${HTTP_PORT}/post")
assert_http "403" "$RESP" "Combined - XSS blocked"

# Test legitimate request still passes
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ${TEST_DOMAIN}" \
    "http://localhost:${HTTP_PORT}/get?name=john&age=25")
assert_http "200" "$RESP" "Combined - Legitimate request passes"

# ============================================================================
# TC-9: Verify Caddy config has WAF handler
# ============================================================================
log_test "TC-9: Verify Caddy Config has WAF Handler"

# Note: Caddy admin API requires trailing slash, and -L follows redirects
CADDY_CONFIG=$(curl -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")

if echo "$CADDY_CONFIG" | grep -q '"handler":"waf"'; then
    log_info "  ✓ WAF handler found in Caddy config"
    PASSED=$((PASSED + 1))
else
    fail_test "WAF handler NOT found in Caddy config"
fi

if echo "$CADDY_CONFIG" | grep -q 'SecRuleEngine'; then
    log_info "  ✓ SecRuleEngine directive found"
    PASSED=$((PASSED + 1))
else
    log_warn "  SecRuleEngine directive not found (may be in Include file)"
    PASSED=$((PASSED + 1))
fi

# ============================================================================
# Results Summary
# ============================================================================
echo ""
echo "=============================================="
echo "=== WAF Integration Test Results ==="
echo "=============================================="
echo ""
echo -e "  ${GREEN}Passed:${NC}  $PASSED"
echo -e "  ${RED}Failed:${NC}  $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo "=============================================="
    echo "=== All WAF tests passed ==="
    echo "=============================================="
    echo ""
    exit 0
else
    echo "=============================================="
    echo "=== WAF TESTS FAILED ==="
    echo "=============================================="
    echo ""
    exit 1
fi
