#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for Coraza WAF using Docker Compose and built image
# Steps:
# 1. Build the local image: docker build -t charon:local .
# 2. Start docker-compose.local.yml: docker compose -f docker-compose.local.yml up -d
# 3. Wait for API to be ready and then configure a ruleset that blocks a simple signature
# 4. Request a path containing the signature and verify 403 (or WAF block response)

echo "Starting Coraza integration test..."

# Ensure we operate from repo root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not available; aborting"
  exit 1
fi

docker build -t charon:local .
# Run charon using docker run to ensure we pass CHARON_SECURITY_WAF_MODE and control network membership for integration
docker rm -f charon-debug >/dev/null 2>&1 || true
if ! docker network inspect containers_default >/dev/null 2>&1; then
  docker network create containers_default
fi
docker run -d --name charon-debug --cap-add=SYS_PTRACE --security-opt seccomp=unconfined --network containers_default -p 80:80 -p 443:443 -p 8080:8080 -p 2345:2345 \
  -e CHARON_ENV=development -e CHARON_DEBUG=1 -e CHARON_HTTP_PORT=8080 -e CHARON_DB_PATH=/app/data/charon.db -e CHARON_FRONTEND_DIR=/app/frontend/dist \
  -e CHARON_CADDY_ADMIN_API=http://localhost:2019 -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy -e CHARON_CADDY_BINARY=caddy -e CHARON_IMPORT_CADDYFILE=/import/Caddyfile \
  -e CHARON_IMPORT_DIR=/app/data/imports -e CHARON_ACME_STAGING=false -e CHARON_SECURITY_WAF_MODE=block \
  -v charon_data:/app/data -v caddy_data:/data -v caddy_config:/config -v /var/run/docker.sock:/var/run/docker.sock:ro -v "$(pwd)/backend:/app/backend:ro" -v "$(pwd)/frontend/dist:/app/frontend/dist:ro" charon:local

echo "Waiting for Charon API to be ready..."
for i in {1..30}; do
  if curl -s -f http://localhost:8080/api/v1/ >/dev/null 2>&1; then
    break
  fi
  echo -n '.'
  sleep 1
done

echo "Skipping unauthenticated ruleset creation (will register and create with cookie later)..."
echo "Creating a backend container for proxy host..."
# ensure the overlay network exists (docker-compose uses containers_default)
CREATED_NETWORK=0
if ! docker network inspect containers_default >/dev/null 2>&1; then
  docker network create containers_default
  CREATED_NETWORK=1
fi

docker rm -f coraza-backend >/dev/null 2>&1 || true
docker run -d --name coraza-backend --network containers_default kennethreitz/httpbin

echo "Creating proxy host 'integration.local' pointing to backend..."
PROXY_HOST_PAYLOAD=$(cat <<EOF
{
  "name": "integration-backend",
  "domain_names": "integration.local",
  "forward_scheme": "http",
  "forward_host": "coraza-backend",
  "forward_port": 80,
  "enabled": true,
  "advanced_config": "{\"handler\":\"waf\",\"ruleset_name\":\"integration-xss\"}"
}
EOF
)
CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" http://localhost:8080/api/v1/proxy-hosts)
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -n1)
if [ "$CREATE_STATUS" != "201" ]; then
  echo "Proxy host create failed or already exists; attempting to update existing host..."
  # Find the existing host UUID by searching for the domain in the proxy-hosts list
  EXISTING_UUID=$(curl -s http://localhost:8080/api/v1/proxy-hosts | grep -o '{[^}]*"domain_names":"integration.local"[^}]*}' | head -n1 | grep -o '"uuid":"[^"]*"' | sed 's/"uuid":"\([^"]*\)"/\1/')
  if [ -n "$EXISTING_UUID" ]; then
    echo "Updating existing host $EXISTING_UUID with Coraza handler"
    curl -s -X PUT -H "Content-Type: application/json" -d "${PROXY_HOST_PAYLOAD}" http://localhost:8080/api/v1/proxy-hosts/$EXISTING_UUID
  else
    echo "Could not find existing host; create response:"
    echo "$CREATE_RESP"
  fi
fi

echo "Registering admin user and logging in to retrieve session cookie..."
TMP_COOKIE=$(mktemp)
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123","name":"Integration Tester"}' http://localhost:8080/api/v1/auth/register >/dev/null || true
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123"}' -c ${TMP_COOKIE} http://localhost:8080/api/v1/auth/login >/dev/null

echo "Give Caddy a moment to apply configuration..."
sleep 3

echo "Creating simple WAF ruleset (XSS block)..."
RULESET=$(cat <<'EOF'
{"name":"integration-xss","content":"SecRule REQUEST_BODY \"<script>\" \"id:12345,phase:2,deny,status:403,msg:'XSS blocked'\""}
EOF
)
curl -s -X POST -H "Content-Type: application/json" -d "${RULESET}" -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/rulesets

echo "Enable WAF globally and set ruleset source to integration-xss..."
SEC_CFG_PAYLOAD='{"name":"default","enabled":true,"waf_mode":"block","waf_rules_source":"integration-xss","admin_whitelist":"0.0.0.0/0"}'
curl -s -X POST -H "Content-Type: application/json" -d "${SEC_CFG_PAYLOAD}" -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/config

echo "Apply rules and test payload..."
# create minimal proxy host if needed; omitted here for brevity; test will target local Caddy root

echo "Dumping Caddy config routes to verify waf handler and rules_files..."
curl -s http://localhost:2019/config | grep -n "waf" || true
curl -s http://localhost:2019/config | grep -n "integration-xss" || true

echo "Inspecting ruleset file inside container..."
docker exec charon-debug cat /app/data/caddy/coraza/rulesets/integration-xss.conf || true

echo "Recent caddy logs (may contain plugin errors):"
docker logs charon-debug | tail -n 200 || true

RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -d "<script>alert(1)</script>" -H "Host: integration.local" http://localhost/post)
if [ "$RESPONSE" = "403" ]; then
  echo "✓ Coraza WAF blocked payload as expected (HTTP 403) in BLOCK mode"
else
  echo "✗ Unexpected response code: $RESPONSE (expected 403) in BLOCK mode"
  exit 1
fi

echo ""
echo "=== Testing MONITOR mode (DetectionOnly) ==="
echo "Switching WAF to monitor mode..."
SEC_CFG_MONITOR='{"name":"default","enabled":true,"waf_mode":"monitor","waf_rules_source":"integration-xss","admin_whitelist":"0.0.0.0/0"}'
curl -s -X POST -H "Content-Type: application/json" -d "${SEC_CFG_MONITOR}" -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/config

echo "Wait for Caddy to apply monitor mode config..."
sleep 2

echo "Inspecting ruleset file (should now have DetectionOnly)..."
docker exec charon-debug cat /app/data/caddy/coraza/rulesets/integration-xss.conf | head -5 || true

RESPONSE_MONITOR=$(curl -s -o /dev/null -w "%{http_code}" -d "<script>alert(1)</script>" -H "Host: integration.local" http://localhost/post)
if [ "$RESPONSE_MONITOR" = "200" ]; then
  echo "✓ Coraza WAF in MONITOR mode allowed payload through (HTTP 200) as expected"
else
  echo "✗ Unexpected response code: $RESPONSE_MONITOR (expected 200) in MONITOR mode"
  echo "  Note: Monitor mode should log but not block"
  exit 1
fi

echo ""
echo "=== All Coraza integration tests passed ==="

echo ""
echo "=== All Coraza integration tests passed ==="
echo "Cleaning up..."
docker rm -f coraza-backend || true
if [ "$CREATED_NETWORK" -eq 1 ]; then
  docker network rm containers_default || true
fi
docker rm -f charon-debug || true
echo "Done"
