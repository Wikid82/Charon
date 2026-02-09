#!/usr/bin/env bash
set -euo pipefail

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh integration-test-crowdsec
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh integration-test-crowdsec" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

trap 'echo "Error occurred, dumping debug info..."; docker logs charon-debug 2>&1 | tail -200 || true' ERR

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not available; aborting"
  exit 1
fi

echo "Building charon:local image..."
docker build -t charon:local .

docker rm -f charon-debug >/dev/null 2>&1 || true
if ! docker network inspect containers_default >/dev/null 2>&1; then
  docker network create containers_default
fi

docker run -d --name charon-debug --cap-add=SYS_PTRACE --security-opt seccomp=unconfined --network containers_default -p 80:80 -p 443:443 -p 8080:8080 -p 2019:2019 -p 2345:2345 \
  -e CHARON_ENV=development -e CHARON_DEBUG=1 -e CHARON_HTTP_PORT=8080 -e CHARON_DB_PATH=/app/data/charon.db -e CHARON_FRONTEND_DIR=/app/frontend/dist \
  -e CHARON_CADDY_ADMIN_API=http://localhost:2019 -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy -e CHARON_CADDY_BINARY=caddy -e CHARON_IMPORT_CADDYFILE=/import/Caddyfile \
  -e CHARON_IMPORT_DIR=/app/data/imports -e CHARON_ACME_STAGING=false -e FEATURE_CERBERUS_ENABLED=true \
  -v charon_data:/app/data -v caddy_data:/data -v caddy_config:/config -v /var/run/docker.sock:/var/run/docker.sock:ro charon:local

echo "Waiting for Charon API to be ready..."
for i in {1..30}; do
  if curl -s -f http://localhost:8080/api/v1/ >/dev/null 2>&1; then
    break
  fi
  echo -n '.'
  sleep 1
done

echo "Registering admin user and logging in..."
TMP_COOKIE=$(mktemp)
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123","name":"Integration Tester"}' http://localhost:8080/api/v1/auth/register >/dev/null || true
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123"}' -c ${TMP_COOKIE} http://localhost:8080/api/v1/auth/login >/dev/null

# Check hub availability first
echo "Checking CrowdSec Hub availability..."
HUB_AVAILABLE=false
if curl -sf --max-time 10 "https://hub-data.crowdsec.net/api/index.json" > /dev/null 2>&1; then
    HUB_AVAILABLE=true
    echo "✓ CrowdSec Hub is available"
else
    echo "⚠ CrowdSec Hub is unavailable - skipping hub preset tests"
fi

# Only test hub presets if hub is available
if [ "$HUB_AVAILABLE" = true ]; then
    echo "Pulled presets list..."
    LIST=$(curl -s -H "Content-Type: application/json" -b ${TMP_COOKIE} http://localhost:8080/api/v1/admin/crowdsec/presets)
    echo "$LIST" | jq -r .presets | head -20

    SLUG="bot-mitigation-essentials"
    echo "Pulling preset $SLUG"
    PULL_RESP=$(curl -s -X POST -H "Content-Type: application/json" -d '{"slug":"'${SLUG}'"}' -b ${TMP_COOKIE} http://localhost:8080/api/v1/admin/crowdsec/presets/pull)
    echo "Pull response: $PULL_RESP"
    if ! echo "$PULL_RESP" | jq -e .status >/dev/null 2>&1; then
      echo "Pull failed: $PULL_RESP"
      exit 1
    fi
    if [ "$(echo "$PULL_RESP" | jq -r .status)" != "pulled" ]; then
      echo "Unexpected pull status: $(echo $PULL_RESP | jq -r .status)"
      exit 1
    fi
    CACHE_KEY=$(echo "$PULL_RESP" | jq -r .cache_key)

    echo "Applying preset $SLUG"
    APPLY_RESP=$(curl -s -X POST -H "Content-Type: application/json" -d '{"slug":"'${SLUG}'"}' -b ${TMP_COOKIE} http://localhost:8080/api/v1/admin/crowdsec/presets/apply)
    echo "Apply response: $APPLY_RESP"
    if ! echo "$APPLY_RESP" | jq -e .status >/dev/null 2>&1; then
      echo "Apply failed: $APPLY_RESP"
      exit 1
    fi
    if [ "$(echo "$APPLY_RESP" | jq -r .status)" != "applied" ]; then
      echo "Unexpected apply status: $(echo $APPLY_RESP | jq -r .status)"
      exit 1
    fi
fi

echo "Cleanup and exit"
docker rm -f charon-debug >/dev/null 2>&1 || true
rm -f ${TMP_COOKIE}
echo "Done"
