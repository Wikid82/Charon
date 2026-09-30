#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for security header profile handling (GH #1402).
#
# A proxy host's security header profile must be authoritative for every header
# it sets: when the upstream ALSO sends that header, the client must receive
# exactly ONE value (the profile's), not two. Headers the profile does not
# control must still pass through, and hosts without a profile must be left
# untouched.
#
# Steps:
# 1. Use the local image (CHARON_IMAGE, default charon:local); build it if missing
# 2. Start Charon and a tiny upstream (busybox httpd + CGI) that sends
#    X-Content-Type-Options: nosniff, Cross-Origin-Resource-Policy: cross-origin
#    and a custom X-Upstream-Only header, and can stream a slow response
# 3. Create a security header profile (CORP same-origin, nosniff) and two proxy
#    hosts: one assigned to the profile, one with headers disabled / no profile
# 4. Assert (HEAD and GET) that the profile host emits each profile-controlled
#    header exactly once with the profile value, keeps the custom header, and
#    that the plain host passes the upstream values through exactly once
# 5. Assert the Caddy admin config emits each response headers handler as a
#    non-deferred + deferred pair with identical values
# 6. Assert a streaming response through the profile host is incremental
# 7. Stop the upstream and assert the resulting 502 carries the profile headers
#
# Assertion failures are collected so a single run reports every failing check
# (and prints the raw header blocks); the script exits non-zero if any failed.
#
# Overridable environment (all optional):
#   CHARON_IMAGE   image to test (default charon:local)
#   SH_PREFIX      name prefix for containers/network/volumes (default secheaders)
#   SH_HTTP_PORT / SH_HTTPS_PORT / SH_API_PORT / SH_ADMIN_PORT  host ports

# Ensure we operate from repo root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# ============================================================================
# Configuration
# ============================================================================
CHARON_IMAGE="${CHARON_IMAGE:-charon:local}"
SH_PREFIX="${SH_PREFIX:-secheaders}"
HTTP_PORT="${SH_HTTP_PORT:-8182}"
HTTPS_PORT="${SH_HTTPS_PORT:-8144}"
API_PORT="${SH_API_PORT:-8282}"
ADMIN_PORT="${SH_ADMIN_PORT:-2121}"

CONTAINER_NAME="${SH_PREFIX}-charon"
UPSTREAM_CONTAINER="${SH_PREFIX}-upstream"
NETWORK_NAME="${SH_PREFIX}-net"
VOLUMES=("${SH_PREFIX}_data" "${SH_PREFIX}_caddy_data" "${SH_PREFIX}_caddy_config")
UPSTREAM_IMAGE="busybox:1.37"

PROFILE_DOMAIN="secheaders-profile.local"
PLAIN_DOMAIN="secheaders-plain.local"
API_URL="http://localhost:${API_PORT}/api/v1"
ADMIN_URL="http://localhost:${ADMIN_PORT}"
PROXY_URL="http://localhost:${HTTP_PORT}"

PROFILE_CORP="same-origin"
UPSTREAM_CORP="cross-origin"

FAILURES=0
TMP_COOKIE=""
TMP_DIR="$(mktemp -d)"

# ============================================================================
# Helper Functions
# ============================================================================

pass() { echo "  ✓ $*"; }
fail() { echo "  ✗ $*"; FAILURES=$((FAILURES + 1)); }

# header_count <header-block> <header-name>: case-insensitive count of lines
# that start with the header name (tolerates CRLF).
header_count() {
    local n
    n=$(printf '%s\n' "$1" | tr -d '\r' | grep -ci "^$2:" || true)
    echo "${n:-0}"
}

# header_values <header-block> <header-name>: the header's values, one per line.
header_values() {
    printf '%s\n' "$1" | tr -d '\r' | grep -i "^$2:" | sed -E 's/^[^:]*:[[:space:]]*//' || true
}

# assert_header_once <label> <header-block> <header-name> <expected-value>
assert_header_once() {
    local label="$1" block="$2" name="$3" expected="$4"
    local count values
    count=$(header_count "$block" "$name")
    values=$(header_values "$block" "$name" | tr '\n' ',' | sed 's/,$//')
    if [ "$count" = "1" ] && [ "$values" = "$expected" ]; then
        pass "${label}: ${name} appears once with '${expected}'"
    else
        fail "${label}: ${name} expected exactly one '${expected}', got ${count} line(s): [${values}]"
    fi
}

# fetch_head / fetch_get <host-header> <path>: raw response header block.
fetch_head() { curl -sI --max-time 15 -H "Host: $1" "${PROXY_URL}$2" || true; }
fetch_get() { curl -s --max-time 15 -D - -o /dev/null -H "Host: $1" "${PROXY_URL}$2" || true; }

# Dumps debug information on failure
on_failure() {
    local exit_code=$?
    echo ""
    echo "=============================================="
    echo "=== FAILURE DEBUG INFO (exit code: $exit_code) ==="
    echo "=============================================="
    echo "=== Charon logs (last 100 lines) ==="
    docker logs "${CONTAINER_NAME}" 2>&1 | tail -100 || true
    echo "=== Caddy admin config (first 200 lines) ==="
    curl -s "${ADMIN_URL}/config/" 2>/dev/null | head -200 || true
    echo "=== Proxy hosts ==="
    curl -s -b "${TMP_COOKIE}" "${API_URL}/proxy-hosts" 2>/dev/null | head -50 || true
    echo "=============================================="
}

# Cleanup function: removes only the resources this script created.
cleanup() {
    echo "Cleaning up test resources..."
    docker rm -f "${UPSTREAM_CONTAINER}" >/dev/null 2>&1 || true
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    docker network rm "${NETWORK_NAME}" >/dev/null 2>&1 || true
    docker volume rm "${VOLUMES[@]}" >/dev/null 2>&1 || true
    rm -rf "${TMP_DIR}" 2>/dev/null || true
    echo "Cleanup complete"
}

trap on_failure ERR
trap cleanup EXIT

# api_post <path> <json-payload>: POST as the logged-in admin; sets API_BODY/API_STATUS.
api_call() {
    local method="$1" path="$2" payload="$3" resp
    resp=$(curl -s -w "\n%{http_code}" -X "$method" -H "Content-Type: application/json" \
        -d "$payload" -b "${TMP_COOKIE}" "${API_URL}${path}")
    API_STATUS=$(echo "$resp" | tail -n1)
    API_BODY=$(echo "$resp" | head -n-1)
}

echo "=============================================="
echo "=== Security Headers Integration Test Starting ==="
echo "=============================================="
echo ""

for tool in docker curl jq; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "$tool is not available; aborting"
        exit 1
    fi
done

# ============================================================================
# Step 1: Build image if needed
# ============================================================================
if ! docker image inspect "${CHARON_IMAGE}" >/dev/null 2>&1; then
    echo "Building ${CHARON_IMAGE} image..."
    docker build -t "${CHARON_IMAGE}" .
else
    echo "Using existing ${CHARON_IMAGE} image"
fi

# ============================================================================
# Step 2: Start upstream and Charon
# ============================================================================
echo "Removing any leftover test containers..."
docker rm -f "${CONTAINER_NAME}" "${UPSTREAM_CONTAINER}" >/dev/null 2>&1 || true

docker network inspect "${NETWORK_NAME}" >/dev/null 2>&1 || docker network create "${NETWORK_NAME}" >/dev/null

# The upstream is a busybox httpd serving two CGI scripts. Both emit the same
# security headers a real app (or Charon's own UI) would send:
#   /cgi-bin/headers  - static body
#   /cgi-bin/stream   - three lines, one second apart (incremental delivery)
UPSTREAM_INIT=$(cat <<'INIT_EOF'
mkdir -p /www/cgi-bin
cat > /www/cgi-bin/headers <<'CGI'
#!/bin/sh
printf 'Content-Type: text/plain\r\n'
printf 'X-Content-Type-Options: nosniff\r\n'
printf 'Cross-Origin-Resource-Policy: cross-origin\r\n'
printf 'X-Upstream-Only: keep\r\n\r\n'
echo ok
CGI
cat > /www/cgi-bin/stream <<'CGI'
#!/bin/sh
printf 'Content-Type: text/plain\r\n'
printf 'X-Content-Type-Options: nosniff\r\n'
printf 'Cross-Origin-Resource-Policy: cross-origin\r\n'
printf 'X-Upstream-Only: keep\r\n\r\n'
for i in 1 2 3; do
  echo "chunk-$i"
  sleep 1
done
CGI
chmod +x /www/cgi-bin/headers /www/cgi-bin/stream
exec httpd -f -vv -p 80 -h /www
INIT_EOF
)

echo "Starting upstream container..."
docker run -d --name "${UPSTREAM_CONTAINER}" --network "${NETWORK_NAME}" \
    "${UPSTREAM_IMAGE}" sh -c "${UPSTREAM_INIT}" >/dev/null

echo "Starting Charon container..."
docker run -d --name "${CONTAINER_NAME}" \
    --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
    --network "${NETWORK_NAME}" \
    -p "${HTTP_PORT}:80" -p "${HTTPS_PORT}:443" -p "${API_PORT}:8080" -p "${ADMIN_PORT}:2019" \
    -e CHARON_ENV=development \
    -e CHARON_DEBUG=1 \
    -e CHARON_HTTP_PORT=8080 \
    -e CHARON_DB_PATH=/app/data/charon.db \
    -e CHARON_FRONTEND_DIR=/app/frontend/dist \
    -e CHARON_CADDY_ADMIN_API=http://localhost:2019 \
    -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
    -e CHARON_CADDY_BINARY=caddy \
    -v "${VOLUMES[0]}:/app/data" \
    -v "${VOLUMES[1]}:/data" \
    -v "${VOLUMES[2]}:/config" \
    "${CHARON_IMAGE}" >/dev/null

echo "Waiting for Charon API to be ready..."
for i in {1..60}; do
    if curl -s -f "${API_URL}/health" >/dev/null 2>&1; then
        echo "✓ Charon API is ready"
        break
    fi
    if [ "$i" -eq 60 ]; then
        echo "✗ Charon API failed to start"
        exit 1
    fi
    sleep 1
done

echo "Waiting for upstream to be ready..."
for i in {1..30}; do
    if docker exec "${CONTAINER_NAME}" sh -c "wget -qO /dev/null http://${UPSTREAM_CONTAINER}/cgi-bin/headers" >/dev/null 2>&1; then
        echo "✓ upstream is ready"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "✗ upstream failed to start"
        exit 1
    fi
    sleep 1
done

# ============================================================================
# Step 3: Authenticate, create profile and proxy hosts
# ============================================================================
echo ""
echo "Setting up admin user and logging in..."
TMP_COOKIE="${TMP_DIR}/cookie"
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"secheaders@example.local","password":"password123","name":"Security Headers Tester"}' \
    "${API_URL}/setup" >/dev/null 2>&1 || true

LOGIN_STATUS=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d '{"email":"secheaders@example.local","password":"password123"}' \
    -c "${TMP_COOKIE}" "${API_URL}/auth/login" | tail -n1)
if [ "$LOGIN_STATUS" != "200" ]; then
    echo "✗ Login failed (HTTP $LOGIN_STATUS) — aborting"
    exit 1
fi
echo "✓ Authentication complete"

echo ""
echo "Creating security header profile (CORP ${PROFILE_CORP}, nosniff)..."
api_call POST /security/headers/profiles '{
    "name": "secheaders-integration",
    "hsts_enabled": false,
    "csp_enabled": false,
    "x_frame_options": "DENY",
    "x_content_type_options": true,
    "referrer_policy": "no-referrer",
    "cross_origin_opener_policy": "same-origin",
    "cross_origin_resource_policy": "'"${PROFILE_CORP}"'",
    "xss_protection": false
}'
if [ "$API_STATUS" != "201" ]; then
    echo "✗ Profile creation failed (HTTP $API_STATUS): $API_BODY"
    exit 1
fi
PROFILE_UUID=$(echo "$API_BODY" | jq -r '.profile.uuid')
echo "✓ Profile created (${PROFILE_UUID})"

echo "Creating proxy host '${PROFILE_DOMAIN}' assigned to the profile..."
api_call POST /proxy-hosts '{
    "name": "secheaders-profile",
    "domain_names": "'"${PROFILE_DOMAIN}"'",
    "forward_scheme": "http",
    "forward_host": "'"${UPSTREAM_CONTAINER}"'",
    "forward_port": 80,
    "enabled": true,
    "security_headers_enabled": true,
    "security_header_profile_id": "'"${PROFILE_UUID}"'"
}'
if [ "$API_STATUS" != "201" ]; then
    echo "✗ Profile host creation failed (HTTP $API_STATUS): $API_BODY"
    exit 1
fi
echo "✓ Profile host created"

echo "Creating proxy host '${PLAIN_DOMAIN}' with no profile and headers disabled..."
api_call POST /proxy-hosts '{
    "name": "secheaders-plain",
    "domain_names": "'"${PLAIN_DOMAIN}"'",
    "forward_scheme": "http",
    "forward_host": "'"${UPSTREAM_CONTAINER}"'",
    "forward_port": 80,
    "enabled": true,
    "security_headers_enabled": false
}'
if [ "$API_STATUS" != "201" ]; then
    echo "✗ Plain host creation failed (HTTP $API_STATUS): $API_BODY"
    exit 1
fi
PLAIN_UUID=$(echo "$API_BODY" | jq -r '.uuid')

# The API cannot turn security_headers_enabled off (the column defaults to true,
# so a false on create is swallowed, and update ignores the field). Flip it in
# the database directly, then touch the host through the API so Charon
# regenerates and reloads the Caddy config.
docker exec "${CONTAINER_NAME}" sqlite3 /app/data/charon.db \
    "UPDATE proxy_hosts SET security_headers_enabled = 0 WHERE uuid = '${PLAIN_UUID}';"
api_call PUT "/proxy-hosts/${PLAIN_UUID}" '{"name": "secheaders-plain"}'
PLAIN_ENABLED=$(curl -s -b "${TMP_COOKIE}" "${API_URL}/proxy-hosts/${PLAIN_UUID}" | jq -r '.security_headers_enabled')
if [ "$PLAIN_ENABLED" != "false" ]; then
    echo "✗ Could not disable security headers on the plain host (got ${PLAIN_ENABLED})"
    exit 1
fi
echo "✓ Plain host created with security headers disabled"

echo "Waiting for Caddy to apply configuration..."
for i in {1..30}; do
    if [ "$(fetch_get "${PROFILE_DOMAIN}" /cgi-bin/headers | head -1 | grep -c ' 200')" = "1" ]; then
        echo "✓ Profile host is serving"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "✗ Profile host never started serving"
        exit 1
    fi
    sleep 1
done

# ============================================================================
# Step 4: Header assertions (HEAD and GET)
# ============================================================================
echo ""
echo "=============================================="
echo "=== Profile host: one value per profile header ==="
echo "=============================================="
for method in HEAD GET; do
    echo ""
    if [ "$method" = "HEAD" ]; then
        BLOCK=$(fetch_head "${PROFILE_DOMAIN}" /cgi-bin/headers)
    else
        BLOCK=$(fetch_get "${PROFILE_DOMAIN}" /cgi-bin/headers)
    fi
    echo "--- raw ${method} response headers (profile host) ---"
    printf '%s\n' "$BLOCK" | tr -d '\r'
    echo "--- end ---"
    assert_header_once "${method} profile host" "$BLOCK" "x-content-type-options" "nosniff"
    assert_header_once "${method} profile host" "$BLOCK" "cross-origin-resource-policy" "${PROFILE_CORP}"
    assert_header_once "${method} profile host" "$BLOCK" "x-upstream-only" "keep"
done

echo ""
echo "=============================================="
echo "=== Plain host: upstream values pass through untouched ==="
echo "=============================================="
for method in HEAD GET; do
    echo ""
    if [ "$method" = "HEAD" ]; then
        BLOCK=$(fetch_head "${PLAIN_DOMAIN}" /cgi-bin/headers)
    else
        BLOCK=$(fetch_get "${PLAIN_DOMAIN}" /cgi-bin/headers)
    fi
    echo "--- raw ${method} response headers (plain host) ---"
    printf '%s\n' "$BLOCK" | tr -d '\r'
    echo "--- end ---"
    assert_header_once "${method} plain host" "$BLOCK" "cross-origin-resource-policy" "${UPSTREAM_CORP}"
    assert_header_once "${method} plain host" "$BLOCK" "x-content-type-options" "nosniff"
    assert_header_once "${method} plain host" "$BLOCK" "x-upstream-only" "keep"
done

# ============================================================================
# Step 5: Caddy config emits each headers handler as a non-deferred + deferred pair
# ============================================================================
echo ""
echo "=============================================="
echo "=== Caddy config: headers handlers are non-deferred + deferred pairs ==="
echo "=============================================="
CADDY_CONFIG=$(curl -s "${ADMIN_URL}/config/")
# The non-deferred set covers Caddy-generated responses (e.g. 502), the deferred
# set runs after reverse_proxy copied the upstream headers so it replaces them.
# Count response headers handlers by kind, and count handlers with no partner:
# every non-deferred handler must be immediately followed by a deferred handler
# with the same "set", and every deferred handler must have that predecessor.
# shellcheck disable=SC2016  # jq program, not a shell expansion
PAIR_JQ='
def is_hdr: type == "object" and .handler? == "headers" and (.response? | type) == "object";
def is_plain: is_hdr and (.response.deferred != true);
def is_def: is_hdr and (.response.deferred == true);
[.. | arrays | . as $a | range(0; length) | select(
    ($a[.] | is_plain) and ((. + 1 >= ($a | length)) or ((($a[. + 1] | is_def) and ($a[. + 1].response.set == $a[.].response.set)) | not))
  )] | length'
PLAIN_COUNT=$(echo "$CADDY_CONFIG" | jq '[.. | objects | select(.handler? == "headers") | .response? | select(. != null and .deferred != true)] | length')
DEFERRED_COUNT=$(echo "$CADDY_CONFIG" | jq '[.. | objects | select(.handler? == "headers") | .response? | select(. != null and .deferred == true)] | length')
UNPAIRED_COUNT=$(echo "$CADDY_CONFIG" | jq "$PAIR_JQ")
if [ "$DEFERRED_COUNT" -ge 1 ] && [ "$PLAIN_COUNT" -eq "$DEFERRED_COUNT" ] && [ "$UNPAIRED_COUNT" -eq 0 ]; then
    pass "${DEFERRED_COUNT} response headers handler pair(s): each non-deferred set is followed by an identical deferred set"
else
    fail "expected every response headers handler as a non-deferred+deferred pair (non-deferred=${PLAIN_COUNT}, deferred=${DEFERRED_COUNT}, unpaired non-deferred=${UNPAIRED_COUNT})"
fi

# ============================================================================
# Step 6: Streaming stays incremental and carries the profile headers once
# ============================================================================
echo ""
echo "=============================================="
echo "=== Streaming response through the profile host ==="
echo "=============================================="
STREAM_HDR="${TMP_DIR}/stream.hdr"
STREAM_OUT="${TMP_DIR}/stream.out"
curl -sN --max-time 15 -D "${STREAM_HDR}" -H "Host: ${PROFILE_DOMAIN}" "${PROXY_URL}/cgi-bin/stream" |
    while IFS= read -r line; do
        printf '%s %s\n' "$(date +%s.%N)" "$line"
    done > "${STREAM_OUT}" || true
STREAM_BLOCK=$(cat "${STREAM_HDR}" 2>/dev/null || true)
echo "--- raw streaming response headers ---"
printf '%s\n' "$STREAM_BLOCK" | tr -d '\r'
echo "--- chunk arrival times ---"
cat "${STREAM_OUT}"
echo "--- end ---"

if printf '%s\n' "$STREAM_BLOCK" | head -1 | grep -q ' 200'; then
    pass "stream: HTTP 200"
else
    fail "stream: expected HTTP 200"
fi
assert_header_once "stream" "$STREAM_BLOCK" "x-content-type-options" "nosniff"
assert_header_once "stream" "$STREAM_BLOCK" "cross-origin-resource-policy" "${PROFILE_CORP}"
CHUNKS=$(wc -l < "${STREAM_OUT}" | tr -d ' ')
SPREAD=$(awk 'NR==1{first=$1} {last=$1} END{printf "%.1f", last-first}' "${STREAM_OUT}")
if [ "$CHUNKS" -ge 2 ] && awk -v s="$SPREAD" 'BEGIN{exit !(s >= 1.0)}'; then
    pass "stream: ${CHUNKS} chunks arrived incrementally over ${SPREAD}s"
else
    fail "stream: expected >= 2 chunks spread over >= 1s, got ${CHUNKS} chunk(s) over ${SPREAD}s"
fi

# ============================================================================
# Step 7: Caddy-generated 502 (upstream down) carries the profile headers
# ============================================================================
echo ""
echo "=============================================="
echo "=== Upstream down: plain 502 carries profile headers ==="
echo "=============================================="
docker stop "${UPSTREAM_CONTAINER}" >/dev/null
BLOCK=""
for _retry in 1 2 3 4 5; do
    BLOCK=$(fetch_get "${PROFILE_DOMAIN}" /cgi-bin/headers)
    if printf '%s\n' "$BLOCK" | head -1 | grep -q ' 502'; then
        break
    fi
    sleep 2
done
echo "--- raw 502 response headers (profile host) ---"
printf '%s\n' "$BLOCK" | tr -d '\r'
echo "--- end ---"
if printf '%s\n' "$BLOCK" | head -1 | grep -q ' 502'; then
    pass "upstream down: HTTP 502"
else
    fail "upstream down: expected HTTP 502"
fi
assert_header_once "502 profile host" "$BLOCK" "x-content-type-options" "nosniff"
assert_header_once "502 profile host" "$BLOCK" "cross-origin-resource-policy" "${PROFILE_CORP}"

# ============================================================================
# Report
# ============================================================================
echo ""
echo "=============================================="
echo "=== Security Headers Integration Test Results ==="
echo "=============================================="
if [ "$FAILURES" -ne 0 ]; then
    echo "✗ ${FAILURES} security header assertion(s) failed"
    exit 1
fi

echo "✓ Security header profile handling succeeded"
echo "  - Profile host emits each profile header exactly once (HEAD and GET)"
echo "  - Headers outside the profile pass through"
echo "  - Host without a profile is left untouched"
echo "  - Caddy config emits each headers handler as a non-deferred + deferred pair"
echo "  - Streaming stays incremental; 502 carries the profile headers"
echo ""
echo "=============================================="
echo "=== ALL SECURITY HEADERS TESTS PASSED ==="
echo "=============================================="
