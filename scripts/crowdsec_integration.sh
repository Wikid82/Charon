#!/usr/bin/env bash
# shellcheck disable=SC2016 # jq filters are single-quoted on purpose; their $variables are jq variables bound with --arg/--argjson.
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

# Defaults match CI. Local runs override them so the throwaway container never shares a name, volume,
# network or host port with a running Charon. Volumes are named after the container, so each run starts
# with an empty data directory and an empty hub cache.
IMAGE="${CHARON_IT_IMAGE:-charon:local}"
CONTAINER="${CHARON_IT_CONTAINER:-charon-debug}"
NETWORK="${CHARON_IT_NETWORK:-containers_default}"
HOST_PORT="${CHARON_IT_HOST_PORT:-8080}"
VOLUME_DATA="${CONTAINER}-data"
VOLUME_CADDY_DATA="${CONTAINER}-caddy-data"
VOLUME_CADDY_CONFIG="${CONTAINER}-caddy-config"
API_BASE="http://localhost:${HOST_PORT}"

# The hub index and item are read from the CrowdSec CDN, the same source the Charon hub client uses.
HUB_BASE_URL="https://cdn-hub.crowdsec.net/crowdsecurity/master"
HUB_INDEX_URL="${HUB_BASE_URL}/.index.json"

# Hub items used by the hub section. sshd is a stable collection with one section. The discord whitelist
# name is in two sections (collections and postoverflows), so a pull without a type must be refused.
SLUG="crowdsecurity/sshd"
SLUG_TYPE="collections"
AMBIGUOUS_SLUG="crowdsecurity/discord-crawler-whitelist"
AMBIGUOUS_TYPES='["collections","postoverflows"]'

TMP_COOKIE=$(mktemp)
HUB_INDEX_FILE=$(mktemp)
API_BODY_FILE=$(mktemp)
API_CODE="000"
API_BODY=""

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not available; aborting"
  exit 1
fi

# cleanup always runs: on failure it dumps the container log first, then removes the throwaway container,
# its volumes and the temp files.
cleanup() {
  local rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "Error occurred (exit ${rc}), last container log lines:"
    docker logs "$CONTAINER" 2>&1 | tail -200 || true
  fi
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$VOLUME_DATA" "$VOLUME_CADDY_DATA" "$VOLUME_CADDY_CONFIG" >/dev/null 2>&1 || true
  rm -f "$TMP_COOKIE" "$HUB_INDEX_FILE" "$API_BODY_FILE"
  return "$rc"
}
trap cleanup EXIT

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo "Building ${IMAGE} image..."
  docker build -t "$IMAGE" .
else
  echo "Using existing ${IMAGE} image"
fi

docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker volume rm "$VOLUME_DATA" "$VOLUME_CADDY_DATA" "$VOLUME_CADDY_CONFIG" >/dev/null 2>&1 || true
if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
  docker network create "$NETWORK"
fi

docker run -d --name "$CONTAINER" --network "$NETWORK" -p "${HOST_PORT}:8080" \
  -e CHARON_ENV=development -e CHARON_HTTP_PORT=8080 -e CHARON_DB_PATH=/app/data/charon.db -e CHARON_FRONTEND_DIR=/app/frontend/dist \
  -e CHARON_CADDY_ADMIN_API=http://localhost:2019 -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy -e CHARON_CADDY_BINARY=caddy -e CHARON_IMPORT_CADDYFILE=/import/Caddyfile \
  -e CHARON_IMPORT_DIR=/app/data/imports -e CHARON_ACME_STAGING=false -e FEATURE_CERBERUS_ENABLED=true \
  -e CHARON_SECURITY_TESTS_ENABLED=false \
  -v "${VOLUME_DATA}:/app/data" -v "${VOLUME_CADDY_DATA}:/data" -v "${VOLUME_CADDY_CONFIG}:/config" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro "$IMAGE"

echo "Waiting for Charon API to be ready..."
API_READY=false
for _ in {1..120}; do
  if curl -s -f "${API_BASE}/api/v1/health" >/dev/null 2>&1; then
    API_READY=true
    break
  fi
  echo -n '.'
  sleep 1
done
echo ""

if [ "${API_READY}" != "true" ]; then
  echo "ERROR: Charon API did not become ready after 120 seconds"
  echo "--- Container status ---"
  docker ps -a --filter "name=^${CONTAINER}$" --format 'Status: {{.Status}}' || true
  exit 1
fi

# api METHOD PATH [JSON]: calls the API with the session cookie. Sets API_CODE (HTTP status) and API_BODY.
api() {
  local method="$1" path="$2" body="${3:-}"
  local -a args=(-s -X "$method" -H "Content-Type: application/json" -b "$TMP_COOKIE" -o "$API_BODY_FILE" -w '%{http_code}' "${API_BASE}${path}")
  if [ -n "$body" ]; then
    args+=(-d "$body")
  fi
  API_CODE=$(curl "${args[@]}" 2>/dev/null) || API_CODE="000"
  API_BODY=$(cat "$API_BODY_FILE")
}

# expect_code CODE STEP: stops the run with the step name and the API answer unless the last call returned CODE.
expect_code() {
  if [ "$API_CODE" != "$1" ]; then
    fail_step "$2: expected HTTP $1"
  fi
}

# fail_step MESSAGE: prints the failed step with the last API answer and exits.
fail_step() {
  echo "FAIL: $1"
  echo "  last HTTP status: ${API_CODE}"
  echo "  last response body: ${API_BODY}"
  exit 1
}

# check MESSAGE JQ_FILTER [JQ_ARGS...]: asserts the last response body satisfies a jq filter.
check() {
  local message="$1" filter="$2"
  shift 2
  if ! jq -e "$@" "$filter" <<<"$API_BODY" >/dev/null 2>&1; then
    fail_step "$message"
  fi
  echo "  ok: $message"
}

echo "Setting up admin user and logging in..."
curl -s -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123","name":"Integration Tester"}' "${API_BASE}/api/v1/setup" >/dev/null || true
LOGIN_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "Content-Type: application/json" -d '{"email":"integration@example.local","password":"password123"}' -c "$TMP_COOKIE" "${API_BASE}/api/v1/auth/login" || echo "000")
if [ "$LOGIN_CODE" != "200" ]; then
  echo "ERROR: login failed with HTTP ${LOGIN_CODE}"
  exit 1
fi

# The hub section needs the CDN. When it cannot be reached, the section is skipped, not failed. The probe
# requires a JSON index with a collections map, so an HTML error page cannot pass it.
echo "Checking CrowdSec Hub availability..."
HUB_AVAILABLE=false
if curl -sf --max-time 30 -o "$HUB_INDEX_FILE" "$HUB_INDEX_URL" && jq -e '.collections | type == "object"' "$HUB_INDEX_FILE" >/dev/null 2>&1; then
  HUB_AVAILABLE=true
  echo "✓ CrowdSec Hub is available"
else
  echo "⚠ CrowdSec Hub is unavailable - skipping hub preset tests"
fi

if [ "$HUB_AVAILABLE" = true ]; then
  # Expected values come from the same index snapshot the API reads: the latest version and its digest.
  ITEM_PATH=$(jq -r --arg s "$SLUG" '.collections[$s].path // empty' "$HUB_INDEX_FILE")
  EXPECTED_VERSION=$(jq -r --arg s "$SLUG" '.collections[$s].version // empty' "$HUB_INDEX_FILE")
  EXPECTED_DIGEST=$(jq -r --arg s "$SLUG" --arg v "$EXPECTED_VERSION" '.collections[$s].versions[$v].digest // empty' "$HUB_INDEX_FILE")
  if [ -z "$ITEM_PATH" ] || [ -z "$EXPECTED_VERSION" ] || ! [[ "$EXPECTED_DIGEST" =~ ^[0-9a-f]{64}$ ]]; then
    echo "FAIL: hub index has no usable record for ${SLUG} (path='${ITEM_PATH}' version='${EXPECTED_VERSION}')"
    exit 1
  fi

  # Independent digest check: the hub index digest must match the file the CDN serves for that path.
  ACTUAL_DIGEST=$(curl -sf --max-time 30 "${HUB_BASE_URL}/${ITEM_PATH}" | sha256sum | cut -d' ' -f1) || ACTUAL_DIGEST=""
  if [ "$ACTUAL_DIGEST" != "$EXPECTED_DIGEST" ]; then
    echo "FAIL: hub index digest for ${SLUG} ${EXPECTED_VERSION} is ${EXPECTED_DIGEST}, the CDN file hashes to ${ACTUAL_DIGEST}"
    exit 1
  fi
  echo "  ok: CDN file for ${SLUG} ${EXPECTED_VERSION} matches the index digest"

  echo "Listing presets..."
  api GET /api/v1/admin/crowdsec/presets
  expect_code 200 "list presets"
  check "hub item ${SLUG} (${SLUG_TYPE}) is listed as available" \
    '.presets[] | select(.slug == $s and .type == $t and .available == true)' --arg s "$SLUG" --arg t "$SLUG_TYPE"

  # A bare name is not a hub item name (owner/name). The API must refuse it before any hub call.
  echo "Pulling a bare name (must be refused)..."
  api POST /api/v1/admin/crowdsec/presets/pull '{"slug":"bot-mitigation-essentials"}'
  expect_code 400 "pull bare name"
  check "bare name answers invalid slug" '.error == "invalid slug"'

  # A name in two hub sections needs a type. Without one the API lists the sections and pulls nothing.
  echo "Pulling an ambiguous name without a type (must be refused)..."
  api POST /api/v1/admin/crowdsec/presets/pull "{\"slug\":\"${AMBIGUOUS_SLUG}\"}"
  expect_code 409 "pull ambiguous name"
  check "ambiguous name answers preset_type_ambiguous with both sections" \
    '.error_code == "preset_type_ambiguous" and .types == $types' --argjson types "$AMBIGUOUS_TYPES"

  echo "Pulling ambiguous name with a type..."
  api POST /api/v1/admin/crowdsec/presets/pull "{\"slug\":\"${AMBIGUOUS_SLUG}\",\"type\":\"collections\"}"
  expect_code 200 "pull ambiguous name with type"
  check "ambiguous name pulled as collections" '.status == "pulled" and .type == "collections"'

  # Pull the target without a type: the index is the only source of its section, so it must resolve.
  echo "Pulling preset ${SLUG}..."
  api POST /api/v1/admin/crowdsec/presets/pull "{\"slug\":\"${SLUG}\"}"
  expect_code 200 "pull ${SLUG}"
  check "pull status is pulled" '.status == "pulled"'
  check "pull resolved the section to ${SLUG_TYPE}" '.type == $t' --arg t "$SLUG_TYPE"
  check "pull reports the item digest that the hub index publishes" '.item_sha256 == $d' --arg d "$EXPECTED_DIGEST"
  check "pull returns a preview of the item" '(.preview | type == "string") and (.preview | contains("name: " + $s))' --arg s "$SLUG"
  check "pull returns a cache key" '(.cache_key // "") | length > 0'
  CACHE_KEY=$(jq -r '.cache_key' <<<"$API_BODY")

  # A second pull is served from the cache, so the cache key does not change.
  echo "Pulling ${SLUG} again (must come from the cache)..."
  api POST /api/v1/admin/crowdsec/presets/pull "{\"slug\":\"${SLUG}\",\"type\":\"${SLUG_TYPE}\"}"
  expect_code 200 "second pull of ${SLUG}"
  check "second pull returns the cached entry" '.cache_key == $k' --arg k "$CACHE_KEY"

  echo "Reading the cached preview..."
  api GET "/api/v1/admin/crowdsec/presets/cache/${SLUG}?type=${SLUG_TYPE}"
  expect_code 200 "cached preview of ${SLUG}"
  check "cached preview has the same cache key and item" \
    '.cache_key == $k and .type == $t and (.preview | contains("name: " + $s))' --arg k "$CACHE_KEY" --arg t "$SLUG_TYPE" --arg s "$SLUG"

  echo "Checking the preset list shows the cached entry..."
  api GET /api/v1/admin/crowdsec/presets
  expect_code 200 "list presets after pull"
  check "list marks ${SLUG} as cached with the same key" \
    '.presets[] | select(.slug == $s and .type == $t and .cached == true and .cache_key == $k)' \
    --arg s "$SLUG" --arg t "$SLUG_TYPE" --arg k "$CACHE_KEY"

  # An apply that names a digest other than the cached item must be refused before anything is installed.
  echo "Applying with a stale digest (must be refused)..."
  api POST /api/v1/admin/crowdsec/presets/apply "{\"slug\":\"${SLUG}\",\"type\":\"${SLUG_TYPE}\",\"cache_key\":\"${CACHE_KEY}\",\"item_sha256\":\"$(printf '0%.0s' {1..64})\"}"
  expect_code 409 "apply with stale digest"
  check "stale digest answers preset_cache_stale" '.error_code == "preset_cache_stale"'

  echo "Applying preset ${SLUG}..."
  api POST /api/v1/admin/crowdsec/presets/apply "{\"slug\":\"${SLUG}\",\"type\":\"${SLUG_TYPE}\",\"cache_key\":\"${CACHE_KEY}\",\"item_sha256\":\"${EXPECTED_DIGEST}\"}"
  expect_code 200 "apply ${SLUG}"
  check "apply status is applied" '.status == "applied"'
  check "apply reports the slug, section and cache key it installed" \
    '.slug == $s and .type == $t and .cache_key == $k and .used_cscli == true' \
    --arg s "$SLUG" --arg t "$SLUG_TYPE" --arg k "$CACHE_KEY"

  # The response is not enough on its own: the item must be installed in the running CrowdSec.
  echo "Confirming ${SLUG} is installed in CrowdSec..."
  if ! docker exec "$CONTAINER" cscli collections list -o json | jq -e --arg s "$SLUG" '.collections[]? | select(.name == $s and (.status | test("enabled|installed")))' >/dev/null; then
    echo "FAIL: cscli does not list ${SLUG} as installed"
    docker exec "$CONTAINER" cscli collections list 2>&1 || true
    exit 1
  fi
  echo "  ok: cscli lists ${SLUG} as installed"
fi

echo "Done"
