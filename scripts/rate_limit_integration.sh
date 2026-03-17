#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for Rate Limiting using Docker Compose and built image
# Steps:
# 1. Build the local image if not present: docker build -t charon:local .
# 2. Start Charon container with rate limiting enabled
# 3. Create a test proxy host via API
# 4. Configure rate limiting with short windows (3 requests per 10 seconds)
# 5. Send rapid requests and verify:
#    - First N requests return HTTP 200
#    - Request N+1 returns HTTP 429
#    - Retry-After header is present on blocked response
# 6. Wait for window to reset, verify requests allowed again
# 7. Clean up test resources

# Ensure we operate from repo root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# ============================================================================
# Configuration
# ============================================================================
RATE_LIMIT_REQUESTS=3
RATE_LIMIT_WINDOW_SEC=10
RATE_LIMIT_BURST=1
CONTAINER_NAME="charon-ratelimit-test"
BACKEND_CONTAINER="ratelimit-backend"
TEST_DOMAIN="ratelimit.local"

# ============================================================================
# Helper Functions
# ============================================================================

# Verifies rate limit handler is present in Caddy config
verify_rate_limit_config() {
    local retries=10
    local wait=5

    echo "Verifying rate limit config in Caddy..."

    for i in $(seq 1 $retries); do
        # Fetch Caddy config via admin API
        local caddy_config
        caddy_config=$(curl -s http://localhost:2119/config/ 2>/dev/null || echo "")

        if [ -z "$caddy_config" ]; then
            echo "  Attempt $i/$retries: Caddy admin API not responding, retrying..."
            sleep $wait
            continue
        fi

        # Check for rate_limit handler
        if echo "$caddy_config" | grep -q '"handler":"rate_limit"'; then
            echo "  ✓ rate_limit handler found in Caddy config"
            return 0
        else
            echo "  Attempt $i/$retries: rate_limit handler not found, waiting..."
        fi

        sleep $wait
    done

    echo "  ✗ rate_limit handler verification failed after $retries attempts"
    return 1
}

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
    curl -s http://localhost:2119/config/ 2>/dev/null | head -300 || echo "Could not retrieve Caddy config"
    echo ""

    echo "=== Security Config in API ==="
    curl -s http://localhost:8280/api/v1/security/config 2>/dev/null || echo "Could not retrieve security config"
    echo ""

    echo "=== Proxy Hosts ==="
    curl -s http://localhost:8280/api/v1/proxy-hosts 2>/dev/null | head -50 || echo "Could not retrieve proxy hosts"
    echo ""

    echo "=============================================="
    echo "=== END DEBUG INFO ==="
    echo "=============================================="
}

# Cleanup function
cleanup() {
    echo "Cleaning up test resources..."
    docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true
    docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
    rm -f "${TMP_COOKIE:-}" 2>/dev/null || true
    echo "Cleanup complete"
}

# Set up trap to dump debug info on any error
trap on_failure ERR

echo "=============================================="
echo "=== Rate Limit Integration Test Starting ==="
echo "=============================================="
echo ""

# Check dependencies
if ! command -v docker >/dev/null 2>&1; then
    echo "docker is not available; aborting"
    exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is not available; aborting"
    exit 1
fi

# ============================================================================
# Step 1: Build image if needed
# ============================================================================
if ! docker image inspect charon:local >/dev/null 2>&1; then
    echo "Building charon:local image..."
    docker build -t charon:local .
else
    echo "Using existing charon:local image"
fi

# ============================================================================
# Step 2: Start Charon container
# ============================================================================
echo "Stopping any existing test containers..."
docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true

# Ensure network exists
if ! docker network inspect containers_default >/dev/null 2>&1; then
    echo "Creating containers_default network..."
    docker network create containers_default
fi

echo "Starting Charon container..."
docker run -d --name ${CONTAINER_NAME} \
    --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
    --network containers_default \
    -p 8180:80 -p 8143:443 -p 8280:8080 -p 2119:2019 \
    -e CHARON_ENV=development \
    -e CHARON_DEBUG=1 \
    -e CHARON_HTTP_PORT=8080 \
    -e CHARON_DB_PATH=/app/data/charon.db \
    -e CHARON_FRONTEND_DIR=/app/frontend/dist \
    -e CHARON_CADDY_ADMIN_API=http://localhost:2019 \
    -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
    -e CHARON_CADDY_BINARY=caddy \
    -v charon_ratelimit_data:/app/data \
    -v caddy_ratelimit_data:/data \
    -v caddy_ratelimit_config:/config \
    charon:local

echo "Waiting for Charon API to be ready..."
for i in {1..30}; do
    if curl -s -f http://localhost:8280/api/v1/health >/dev/null 2>&1; then
        echo "✓ Charon API is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "✗ Charon API failed to start"
        exit 1
    fi
    echo -n '.'
    sleep 1
done

# ============================================================================
# Step 3: Create backend container
# ============================================================================
echo ""
echo "Creating backend container for proxy host..."
docker pull mccutchen/go-httpbin 2>/dev/null || true
docker run -d --name ${BACKEND_CONTAINER} --network containers_default -e PORT=80 mccutchen/go-httpbin

echo "Waiting for httpbin backend to be ready..."
for i in {1..45}; do
    if docker exec ${CONTAINER_NAME} sh -c "wget -qO /dev/null http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
        echo "✓ httpbin backend is ready"
        break
    fi
    if [ $i -eq 45 ]; then
        echo "✗ httpbin backend failed to start"
        exit 1
    fi
    echo -n '.'
    sleep 1
done

# ============================================================================
# Step 4: Register user and authenticate
# ============================================================================
echo ""
echo "Registering admin user and logging in..."
TMP_COOKIE=$(mktemp)
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"ratelimit@example.local","password":"password123","name":"Rate Limit Tester"}' \
    http://localhost:8280/api/v1/auth/register >/dev/null 2>&1 || true

LOGIN_STATUS=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d '{"email":"ratelimit@example.local","password":"password123"}' \
    -c ${TMP_COOKIE} \
    http://localhost:8280/api/v1/auth/login | tail -n1)

if [ "$LOGIN_STATUS" != "200" ]; then
    echo "✗ Login failed (HTTP $LOGIN_STATUS) — aborting"
    exit 1
fi
echo "✓ Authentication complete (HTTP $LOGIN_STATUS)"

# ============================================================================
# Step 5: Create proxy host
# ============================================================================
echo ""
echo "Creating proxy host '${TEST_DOMAIN}' pointing to backend..."
PROXY_HOST_PAYLOAD=$(cat <<EOF
{
    "name": "ratelimit-backend",
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
    -b ${TMP_COOKIE} \
    http://localhost:8280/api/v1/proxy-hosts)
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -n1)

if [ "$CREATE_STATUS" = "201" ]; then
    echo "✓ Proxy host created successfully"
elif [ "$CREATE_STATUS" = "401" ] || [ "$CREATE_STATUS" = "403" ]; then
    echo "✗ Proxy host creation failed — authentication/authorization error (HTTP $CREATE_STATUS)"
    exit 1
else
    echo "  Proxy host may already exist or was created (status: $CREATE_STATUS) — continuing"
fi

# ============================================================================
# Step 6: Configure rate limiting
# ============================================================================
echo ""
echo "Configuring rate limiting: ${RATE_LIMIT_REQUESTS} requests per ${RATE_LIMIT_WINDOW_SEC} seconds..."
SEC_CFG_PAYLOAD=$(cat <<EOF
{
    "name": "default",
    "enabled": true,
    "rate_limit_enable": true,
    "rate_limit_requests": ${RATE_LIMIT_REQUESTS},
    "rate_limit_window_sec": ${RATE_LIMIT_WINDOW_SEC},
    "rate_limit_burst": ${RATE_LIMIT_BURST},
    "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
)

echo "Waiting for Caddy admin API to be ready..."
for i in {1..20}; do
    if curl -s -f http://localhost:2119/config/ >/dev/null 2>&1; then
        echo "✓ Caddy admin API is ready"
        break
    fi
    if [ $i -eq 20 ]; then
        echo "✗ Caddy admin API failed to become ready"
        exit 1
    fi
    echo -n '.'
    sleep 1
done

SEC_CONFIG_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${SEC_CFG_PAYLOAD}" \
    -b ${TMP_COOKIE} \
    http://localhost:8280/api/v1/security/config)
SEC_CONFIG_STATUS=$(echo "$SEC_CONFIG_RESP" | tail -n1)
SEC_CONFIG_BODY=$(echo "$SEC_CONFIG_RESP" | head -n-1)

if [ "$SEC_CONFIG_STATUS" != "200" ]; then
    echo "✗ Security config update failed (HTTP $SEC_CONFIG_STATUS)"
    echo "  Response body: $SEC_CONFIG_BODY"
    echo "  Verify the auth cookie is valid and the user has the admin role."
    exit 1
fi
echo "✓ Rate limiting configured (HTTP $SEC_CONFIG_STATUS)"

echo "Waiting for Caddy to apply configuration..."
sleep 8

# Verify rate limit handler is configured — this is a hard requirement
if ! verify_rate_limit_config; then
    echo "✗ Rate limit handler verification failed — aborting test"
    echo "  The handler must be present in Caddy config before enforcement can be tested."
    echo ""
    echo "=== Caddy admin API full config ==="
    curl -s http://localhost:2119/config/ 2>/dev/null | head -200 || echo "Admin API not responding"
    echo ""
    echo "=== Security config from API ==="
    curl -s -b ${TMP_COOKIE} http://localhost:8280/api/v1/security/config 2>/dev/null || echo "API not responding"
    exit 1
fi

# ============================================================================
# Step 7: Test rate limiting enforcement
# ============================================================================
echo ""
echo "=============================================="
echo "=== Testing Rate Limit Enforcement ==="
echo "=============================================="
echo ""
echo "Sending ${RATE_LIMIT_REQUESTS} rapid requests (should all return 200)..."

SUCCESS_COUNT=0
for i in $(seq 1 ${RATE_LIMIT_REQUESTS}); do
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: ${TEST_DOMAIN}" http://localhost:8180/get)
    if [ "$RESPONSE" = "200" ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        echo "  Request $i: HTTP $RESPONSE ✓"
    else
        echo "  Request $i: HTTP $RESPONSE (expected 200)"
    fi
    # Small delay to avoid overwhelming, but still within the window
    sleep 0.1
done

if [ $SUCCESS_COUNT -ne ${RATE_LIMIT_REQUESTS} ]; then
    echo ""
    echo "✗ Not all allowed requests succeeded ($SUCCESS_COUNT/${RATE_LIMIT_REQUESTS})"
    echo "Rate limit enforcement test FAILED"
    cleanup
    exit 1
fi

echo ""
echo "Sending request ${RATE_LIMIT_REQUESTS}+1 (should return 429 Too Many Requests)..."

# Capture headers too for Retry-After check
BLOCKED_RESPONSE=$(curl -s -D - -o /dev/null -H "Host: ${TEST_DOMAIN}" http://localhost:8180/get)
BLOCKED_STATUS=$(echo "$BLOCKED_RESPONSE" | head -1 | grep -o '[0-9]\{3\}' | head -1)

if [ "$BLOCKED_STATUS" = "429" ]; then
    echo "  ✓ Request blocked with HTTP 429 as expected"

    # Check for Retry-After header
    if echo "$BLOCKED_RESPONSE" | grep -qi "Retry-After"; then
        RETRY_AFTER=$(echo "$BLOCKED_RESPONSE" | grep -i "Retry-After" | head -1)
        echo "  ✓ Retry-After header present: $RETRY_AFTER"
    else
        echo "  ⚠ Retry-After header not found (may be plugin-dependent)"
    fi
else
    echo "  ✗ Expected HTTP 429, got HTTP $BLOCKED_STATUS"
    echo ""
    echo "=== DEBUG: SecurityConfig from API ==="
    curl -s -b ${TMP_COOKIE} http://localhost:8280/api/v1/security/config | jq .
    echo ""
    echo "=== DEBUG: SecurityStatus from API ==="
    curl -s -b ${TMP_COOKIE} http://localhost:8280/api/v1/security/status | jq .
    echo ""
    echo "=== DEBUG: Caddy config (first proxy route handlers) ==="
    curl -s http://localhost:2119/config/ | jq '.apps.http.servers.charon_server.routes[0].handle // []'
    echo ""
    echo "=== DEBUG: Container logs (last 100 lines) ==="
    docker logs ${CONTAINER_NAME} 2>&1 | tail -100
    echo ""
    echo "Rate limit enforcement test FAILED"
    echo "Container left running for manual inspection"
    echo "Run: docker logs ${CONTAINER_NAME}"
    echo "Run: docker rm -f ${CONTAINER_NAME} ${BACKEND_CONTAINER}"
    exit 1
fi

# ============================================================================
# Step 8: Test window reset
# ============================================================================
echo ""
echo "=============================================="
echo "=== Testing Window Reset ==="
echo "=============================================="
echo ""
echo "Waiting for rate limit window to reset (${RATE_LIMIT_WINDOW_SEC} seconds + buffer)..."
sleep $((RATE_LIMIT_WINDOW_SEC + 2))

echo "Sending request after window reset (should return 200)..."
RESET_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: ${TEST_DOMAIN}" http://localhost:8180/get)

if [ "$RESET_RESPONSE" = "200" ]; then
    echo "  ✓ Request allowed after window reset (HTTP 200)"
else
    echo "  ✗ Expected HTTP 200 after reset, got HTTP $RESET_RESPONSE"
    echo ""
    echo "Rate limit window reset test FAILED"
    cleanup
    exit 1
fi

# ============================================================================
# Step 9: Cleanup and report
# ============================================================================
echo ""
echo "=============================================="
echo "=== Rate Limit Integration Test Results ==="
echo "=============================================="
echo ""
echo "✓ Rate limit enforcement succeeded"
echo "  - ${RATE_LIMIT_REQUESTS} requests allowed within window"
echo "  - Request ${RATE_LIMIT_REQUESTS}+1 blocked with HTTP 429"
echo "  - Requests allowed again after window reset"
echo ""

# Remove test proxy host from database
echo "Removing test proxy host from database..."
INTEGRATION_UUID=$(curl -s -b ${TMP_COOKIE} http://localhost:8280/api/v1/proxy-hosts | \
    grep -o '"uuid":"[^"]*"[^}]*"domain_names":"'${TEST_DOMAIN}'"' | head -n1 | \
    grep -o '"uuid":"[^"]*"' | sed 's/"uuid":"\([^"]*\)"/\1/')

if [ -n "$INTEGRATION_UUID" ]; then
    curl -s -X DELETE -b ${TMP_COOKIE} \
        "http://localhost:8280/api/v1/proxy-hosts/${INTEGRATION_UUID}?delete_uptime=true" >/dev/null
    echo "✓ Deleted test proxy host ${INTEGRATION_UUID}"
fi

cleanup

echo ""
echo "=============================================="
echo "=== ALL RATE LIMIT TESTS PASSED ==="
echo "=============================================="
echo ""
