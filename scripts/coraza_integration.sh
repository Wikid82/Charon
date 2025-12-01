#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for Coraza WAF using Docker Compose and built image
# Steps:
# 1. Build the local image: docker build -t charon:local .
# 2. Start docker-compose.local.yml: docker compose -f docker-compose.local.yml up -d
# 3. Wait for API to be ready and then configure a ruleset that blocks a simple signature
# 4. Request a path containing the signature and verify 403 (or WAF block response)

echo "Starting Coraza integration test..."

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not available; aborting"
  exit 1
fi

docker build -t charon:local .
docker compose -f docker-compose.local.yml up -d

echo "Waiting for Charon API to be ready..."
for i in {1..30}; do
  if curl -s -f http://localhost:8080/api/v1/ >/dev/null 2>&1; then
    break
  fi
  echo -n '.'
  sleep 1
done

echo "Creating simple WAF ruleset (XSS block)..."
RULESET='{"name":"integration-xss","content":"SecRule REQUEST_BODY \"<script>\" \"id:12345,phase:2,deny,status:403,msg:\'XSS blocked\'\""}'
curl -s -X POST -H "Content-Type: application/json" -d "${RULESET}" http://localhost:8080/api/v1/security/rulesets

echo "Apply rules and test payload..."
# create minimal proxy host if needed; omitted here for brevity; test will target local Caddy root

RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -d "<script>alert(1)</script>" http://localhost/)
if [ "$RESPONSE" = "403" ]; then
  echo "Coraza WAF blocked payload as expected (HTTP 403)"
else
  echo "Unexpected response code: $RESPONSE (expected 403)"
  exit 1
fi

echo "Coraza integration test complete. Cleaning up..."
docker compose -f docker-compose.local.yml down
echo "Done"
