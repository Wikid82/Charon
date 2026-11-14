#!/usr/bin/env bash
set -euo pipefail

# First-run setup code: passed to the container and sent with the setup request.
export CHARON_SETUP_TOKEN="${CHARON_SETUP_TOKEN:-$(openssl rand -hex 32)}"

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh integration-test-coraza
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh integration-test-coraza" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1

# Brief: Integration test for Coraza WAF using Docker Compose and built image
# Steps:
# 1. Use the local image (CHARON_IMAGE, default charon:local); build it if missing
# 2. Start a Charon container and a httpbin backend on a per-run network
# 3. Wait for API to be ready and then configure a ruleset that blocks a simple signature
# 4. Request a path containing the signature and verify 403 (or WAF block response)
#
# Every run owns its resources: containers, volumes and network are named after
# RUN_ID, host ports are bound to 127.0.0.1 (ephemeral unless CORAZA_*_PORT is
# set), and the EXIT trap removes everything the run created. The run never
# touches the shared compose volumes, the compose network or other containers.
#
# Overridable environment (all optional):
#   CHARON_IMAGE         image to test (default charon:local)
#   CORAZA_HTTP_PORT / CORAZA_HTTPS_PORT / CORAZA_API_PORT  fixed host ports
#   CORAZA_DEBUG=1       also publish CORAZA_DEBUG_PORT (default ephemeral),
#                        add SYS_PTRACE, unconfined seccomp and the Docker socket
#   CORAZA_DEBUG_PORT    host port for the debugger (only with CORAZA_DEBUG=1)

# Ensure we operate from repo root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# ============================================================================
# Run-scoped names
# ============================================================================
CHARON_IMAGE="${CHARON_IMAGE:-charon:local}"
RUN_ID="coraza-$$"
CHARON_CONTAINER="charon-${RUN_ID}"
BACKEND_CONTAINER="${RUN_ID}-backend"
NETWORK="${RUN_ID}-net"
VOLUMES=("charon-${RUN_ID}-data" "charon-${RUN_ID}-caddy-data" "charon-${RUN_ID}-caddy-config")
TMP_COOKIE=""

# Requested host ports (empty means Docker picks a free one). The bound ports
# are read back after start and used for every request.
CORAZA_HTTP_PORT="${CORAZA_HTTP_PORT:-}"
CORAZA_HTTPS_PORT="${CORAZA_HTTPS_PORT:-}"
CORAZA_API_PORT="${CORAZA_API_PORT:-}"
CORAZA_DEBUG_PORT="${CORAZA_DEBUG_PORT:-}"
HTTP_HOST_PORT=""
HTTPS_HOST_PORT=""
API_HOST_PORT=""

# ============================================================================
# Helper Functions
# ============================================================================

# publish_arg <requested-host-port> <container-port>: a -p value bound to
# loopback, with the requested host port or an ephemeral one.
publish_arg() {
    if [ -n "$1" ]; then
        printf '127.0.0.1:%s:%s' "$1" "$2"
    else
        printf '127.0.0.1::%s' "$2"
    fi
}

# bound_port <container> <container-port>: the loopback host port Docker bound.
bound_port() {
    docker port "$1" "$2/tcp" 2>/dev/null | sed -n 's/^127\.0\.0\.1:\([0-9]*\)$/\1/p' | head -n1 || true
}

# Removes exactly the containers, volumes and network this run created.
cleanup() {
    docker rm -f "$BACKEND_CONTAINER" >/dev/null 2>&1 || true
    docker rm -f "$CHARON_CONTAINER" >/dev/null 2>&1 || true
    docker volume rm "${VOLUMES[@]}" >/dev/null 2>&1 || true
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
    if [ -n "$TMP_COOKIE" ]; then
        rm -f "$TMP_COOKIE"
    fi
}

# Verifies WAF handler is present in Caddy config with correct ruleset
verify_waf_config() {
    local expected_ruleset="${1:-integration-xss}"
    local retries=10
    local wait=3

    echo "Verifying WAF config (expecting ruleset: ${expected_ruleset})..."

    for i in $(seq 1 $retries); do
        # Fetch Caddy config via admin API
        local caddy_config
        caddy_config=$(docker exec "$CHARON_CONTAINER" /app/charon caddy-admin get /config/ 2>/dev/null || echo "")

        if [ -z "$caddy_config" ]; then
            echo "  Attempt $i/$retries: Caddy admin API not responding, retrying..."
            sleep $wait
            continue
        fi

        # Check for WAF handler
        if echo "$caddy_config" | grep -q '"handler":"waf"'; then
            echo "  ✓ WAF handler found in Caddy config"

            # Also verify the directives include our ruleset
            if echo "$caddy_config" | grep -q "$expected_ruleset"; then
                echo "  ✓ Ruleset '${expected_ruleset}' found in directives"
                return 0
            else
                echo "  ⚠ WAF handler present but ruleset '${expected_ruleset}' not found in directives"
            fi
        else
            echo "  Attempt $i/$retries: WAF handler not found, waiting..."
        fi

        sleep $wait
    done

    echo "  ✗ WAF handler verification failed after $retries attempts"
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
    docker logs "$CHARON_CONTAINER" 2>&1 | tail -150 || echo "Could not retrieve container logs"
    echo ""

    echo "=== Caddy Admin API Config ==="
    docker exec "$CHARON_CONTAINER" /app/charon caddy-admin get /config/ 2>/dev/null | head -300 || echo "Could not retrieve Caddy config"
    echo ""

    echo "=== Ruleset Files in Container ==="
    docker exec "$CHARON_CONTAINER" sh -c 'ls -la /app/data/caddy/coraza/rulesets/ 2>/dev/null' || echo "No rulesets directory found"
    echo ""

    echo "=== Ruleset File Contents ==="
    docker exec "$CHARON_CONTAINER" sh -c 'cat /app/data/caddy/coraza/rulesets/*.conf 2>/dev/null' || echo "No ruleset files found"
    echo ""

    echo "=== Security Config in API ==="
    curl -s "http://localhost:${API_HOST_PORT}/api/v1/security/config" 2>/dev/null || echo "Could not retrieve security config"
    echo ""

    echo "=== Proxy Hosts ==="
    curl -s "http://localhost:${API_HOST_PORT}/api/v1/proxy-hosts" 2>/dev/null | head -50 || echo "Could not retrieve proxy hosts"
    echo ""

    echo "=============================================="
    echo "=== END DEBUG INFO ==="
    echo "=============================================="
}

# Dump debug info on any error; always clean up this run's resources on exit.
trap on_failure ERR
trap cleanup EXIT

echo "Starting Coraza integration test (run ${RUN_ID})..."

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not available; aborting"
  exit 1
fi

# Build the image if it doesn't already exist (CI workflow builds it beforehand)
if ! docker image inspect "$CHARON_IMAGE" >/dev/null 2>&1; then
  echo "Building ${CHARON_IMAGE} image..."
  docker build -t "$CHARON_IMAGE" .
else
  echo "Using existing ${CHARON_IMAGE} image"
fi

# Debug-only additions: the debugger port, ptrace and the Docker socket are
# never exposed in a default run.
DEBUG_ARGS=()
if [ "${CORAZA_DEBUG:-0}" = "1" ]; then
  echo "CORAZA_DEBUG=1: publishing the debug port and enabling ptrace and the Docker socket"
  DEBUG_ARGS=(
    --cap-add=SYS_PTRACE
    --security-opt seccomp=unconfined
    -p "$(publish_arg "$CORAZA_DEBUG_PORT" 2345)"
    -v /var/run/docker.sock:/var/run/docker.sock:ro
  )
fi

docker network create "$NETWORK" >/dev/null

# NOTE: We intentionally do NOT mount $(pwd)/backend or $(pwd)/frontend/dist here.
# In CI, frontend/dist does not exist (it's built inside the Docker image).
# Mounting a non-existent directory would override the built frontend with an empty dir.
# For local development with hot-reload, use .docker/compose/docker-compose.local.yml instead.
docker run -d --name "$CHARON_CONTAINER" --network "$NETWORK" \
  -p "$(publish_arg "$CORAZA_HTTP_PORT" 80)" \
  -p "$(publish_arg "$CORAZA_HTTPS_PORT" 443)" \
  -p "$(publish_arg "$CORAZA_API_PORT" 8080)" \
  ${DEBUG_ARGS[@]+"${DEBUG_ARGS[@]}"} \
  -e CHARON_ENV=development -e CHARON_DEBUG=1 -e CHARON_SETUP_TOKEN -e CHARON_HTTP_PORT=8080 -e CHARON_DB_PATH=/app/data/charon.db -e CHARON_FRONTEND_DIR=/app/frontend/dist \
  -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy -e CHARON_CADDY_BINARY=caddy -e CHARON_IMPORT_CADDYFILE=/import/Caddyfile \
  -e CHARON_IMPORT_DIR=/app/data/imports -e CHARON_ACME_STAGING=false -e CHARON_SECURITY_WAF_MODE=block \
  -v "charon-${RUN_ID}-data:/app/data" -v "charon-${RUN_ID}-caddy-data:/data" -v "charon-${RUN_ID}-caddy-config:/config" \
  "$CHARON_IMAGE" >/dev/null

HTTP_HOST_PORT="$(bound_port "$CHARON_CONTAINER" 80)"
HTTPS_HOST_PORT="$(bound_port "$CHARON_CONTAINER" 443)"
API_HOST_PORT="$(bound_port "$CHARON_CONTAINER" 8080)"
if [ -z "$HTTP_HOST_PORT" ] || [ -z "$HTTPS_HOST_PORT" ] || [ -z "$API_HOST_PORT" ]; then
  echo "✗ Could not read the loopback ports Docker bound for ${CHARON_CONTAINER}"
  exit 1
fi
echo "Host ports on 127.0.0.1: HTTP ${HTTP_HOST_PORT}, HTTPS ${HTTPS_HOST_PORT}, API ${API_HOST_PORT}"

echo "Waiting for Charon API to be ready..."
for i in {1..30}; do
  if curl -s -f "http://localhost:${API_HOST_PORT}/api/v1/" >/dev/null 2>&1; then
    break
  fi
  echo -n '.'
  sleep 1
done

echo "Skipping unauthenticated ruleset creation (will set up admin and create with cookie later)..."
echo "Creating a backend container for proxy host..."

docker run -d --name "$BACKEND_CONTAINER" --network "$NETWORK" -e PORT=80 mccutchen/go-httpbin >/dev/null

echo "Waiting for httpbin backend to be ready..."
for i in {1..20}; do
  # Check if container is running and has network connectivity
  if docker exec "$CHARON_CONTAINER" sh -c "wget -qO /dev/null http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
    echo "✓ httpbin backend is ready"
    break
  fi
  if [ "$i" -eq 20 ]; then
    echo "✗ httpbin backend failed to start"
    echo "Container status:"
    docker ps -a --filter "name=${BACKEND_CONTAINER}"
    echo "Container logs:"
    docker logs "$BACKEND_CONTAINER" 2>&1 | tail -20
    exit 1
  fi
  echo -n '.'
  sleep 1
done

echo "Setting up admin user and logging in to retrieve session cookie..."
TMP_COOKIE=$(mktemp)
curl -s -X POST -H "Content-Type: application/json" -d "{\"email\":\"integration@example.local\",\"password\":\"password123\",\"name\":\"Integration Tester\",\"setup_code\":\"${CHARON_SETUP_TOKEN}\"}" "http://localhost:${API_HOST_PORT}/api/v1/setup" >/dev/null || true
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123"}' -c "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/auth/login" >/dev/null

echo "Creating proxy host 'integration.local' pointing to backend..."
PROXY_HOST_PAYLOAD=$(cat <<EOF
{
  "name": "integration-backend",
  "domain_names": "integration.local",
  "forward_scheme": "http",
  "forward_host": "${BACKEND_CONTAINER}",
  "forward_port": 80,
  "enabled": true,
  "advanced_config": "{\"handler\":\"waf\",\"ruleset_name\":\"integration-xss\"}"
}
EOF
)
CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" -b "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/proxy-hosts")
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -n1)
if [ "$CREATE_STATUS" != "201" ]; then
  echo "Proxy host create failed or already exists; attempting to update existing host..."
  # Find the existing host UUID by searching for the domain in the proxy-hosts list
  EXISTING_UUID=$(curl -s -b "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/proxy-hosts" | grep -o '{[^}]*"domain_names":"integration.local"[^}]*}' | head -n1 | grep -o '"uuid":"[^"]*"' | sed 's/"uuid":"\([^"]*\)"/\1/')
  if [ -n "$EXISTING_UUID" ]; then
    echo "Updating existing host $EXISTING_UUID with Coraza handler"
    curl -s -X PUT -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" -b "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/proxy-hosts/$EXISTING_UUID"
  else
    echo "Could not find existing host; create response:"
    echo "$CREATE_RESP"
  fi
fi

echo "Give Caddy a moment to apply configuration..."
sleep 3

echo "Creating simple WAF ruleset (XSS block)..."
RULESET=$(cat <<'EOF'
{"name":"integration-xss","content":"SecRule REQUEST_BODY \"<script>\" \"id:12345,phase:2,deny,status:403,msg:'XSS blocked'\""}
EOF
)
curl -s -X POST -H "Content-Type: application/json" -d "${RULESET}" -b "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/security/rulesets"

echo "Enable WAF globally and set ruleset source to integration-xss..."
SEC_CFG_PAYLOAD='{"name":"default","enabled":true,"waf_mode":"block","waf_rules_source":"integration-xss","admin_whitelist":"127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"}'
curl -s -X POST -H "Content-Type: application/json" -d "${SEC_CFG_PAYLOAD}" -b "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/security/config"

echo "Waiting for Caddy to apply WAF configuration..."
sleep 10

# Verify WAF handler is properly configured before proceeding
# Note: This is advisory - if admin API is restarting we'll proceed anyway
if ! verify_waf_config "integration-xss"; then
    echo "WARNING: WAF configuration verification failed (admin API may be restarting)"
    echo "Proceeding with test anyway..."
fi

echo "Apply rules and test payload..."
# create minimal proxy host if needed; omitted here for brevity; test will target local Caddy root

echo "Verifying Caddy config has WAF handler..."
docker exec "$CHARON_CONTAINER" /app/charon caddy-admin get /config/ | grep -E '"handler":"waf"' || echo "WARNING: WAF handler not found in initial config check"

echo "Inspecting ruleset file inside container..."
docker exec "$CHARON_CONTAINER" sh -c 'cat /app/data/caddy/coraza/rulesets/integration-xss-*.conf' || echo "WARNING: Could not read ruleset file"

echo ""
echo "=== Testing BLOCK mode ==="

MAX_RETRIES=3
for attempt in $(seq 1 $MAX_RETRIES); do
  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -d "<script>alert(1)</script>" -H "Host: integration.local" "http://localhost:${HTTP_HOST_PORT}/post")
  if [ "$RESPONSE" = "403" ]; then
    echo "✓ Coraza WAF blocked payload as expected (HTTP 403) in BLOCK mode"
    break
  fi
  if [ "$attempt" -eq $MAX_RETRIES ]; then
    echo "✗ Unexpected response code: $RESPONSE (expected 403) in BLOCK mode after $MAX_RETRIES attempts"
    exit 1
  fi
  echo "  Attempt $attempt: Got $RESPONSE, retrying in 2s..."
  sleep 2
done

echo ""
echo "=== Testing MONITOR mode (DetectionOnly) ==="
echo "Switching WAF to monitor mode..."
SEC_CFG_MONITOR='{"name":"default","enabled":true,"waf_mode":"monitor","waf_rules_source":"integration-xss","admin_whitelist":"127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"}'
curl -s -X POST -H "Content-Type: application/json" -d "${SEC_CFG_MONITOR}" -b "${TMP_COOKIE}" "http://localhost:${API_HOST_PORT}/api/v1/security/config"

echo "Wait for Caddy to apply monitor mode config..."
sleep 12

# Verify WAF handler is still present after mode switch
# Note: This is advisory - if admin API is restarting we'll proceed anyway
if ! verify_waf_config "integration-xss"; then
    echo "WARNING: WAF config verification failed after mode switch (admin API may be restarting)"
    echo "Proceeding with test anyway..."
fi

echo "Inspecting ruleset file (should now have DetectionOnly)..."
docker exec "$CHARON_CONTAINER" sh -c 'cat /app/data/caddy/coraza/rulesets/integration-xss-*.conf | head -5' || true

for attempt in $(seq 1 $MAX_RETRIES); do
  RESPONSE_MONITOR=$(curl -s -o /dev/null -w "%{http_code}" -d "<script>alert(1)</script>" -H "Host: integration.local" "http://localhost:${HTTP_HOST_PORT}/post")
  if [ "$RESPONSE_MONITOR" = "200" ]; then
    echo "✓ Coraza WAF in MONITOR mode allowed payload through (HTTP 200) as expected"
    break
  fi
  if [ "$attempt" -eq $MAX_RETRIES ]; then
    echo "✗ Unexpected response code: $RESPONSE_MONITOR (expected 200) in MONITOR mode after $MAX_RETRIES attempts"
    echo "  Note: Monitor mode should log but not block"
    exit 1
  fi
  echo "  Attempt $attempt: Got $RESPONSE_MONITOR, retrying in 2s..."
  sleep 2
done

echo ""
echo "=== All Coraza integration tests passed ==="
echo "Done (run resources are removed on exit)"
