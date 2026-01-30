#!/bin/bash
# Validates E2E authentication setup for TestDataManager

set -eo pipefail

echo "=== E2E Authentication Validation ==="

# Check 0: Verify required dependencies
if ! command -v jq &> /dev/null; then
  echo "❌ jq is required but not installed."
  echo "   Install with: brew install jq (macOS) or apt-get install jq (Linux)"
  exit 1
fi
echo "✅ jq is installed"

# Check 1: Verify PLAYWRIGHT_BASE_URL uses localhost
if [[ -n "$PLAYWRIGHT_BASE_URL" && "$PLAYWRIGHT_BASE_URL" != *"localhost"* ]]; then
  echo "❌ PLAYWRIGHT_BASE_URL ($PLAYWRIGHT_BASE_URL) does not use localhost"
  echo "   Fix: export PLAYWRIGHT_BASE_URL=http://localhost:8080"
  exit 1
fi
echo "✅ PLAYWRIGHT_BASE_URL is localhost or unset (defaults to localhost)"

# Check 2: Verify Docker container is running
if ! docker ps | grep -q charon-e2e; then
  echo "⚠️ charon-e2e container not running. Starting..."
  docker compose -f .docker/compose/docker-compose.playwright-local.yml up -d
  echo "Waiting for container health..."
  sleep 10
fi
echo "✅ charon-e2e container is running"

# Check 3: Verify API is accessible at localhost:8080
if ! curl -sf http://localhost:8080/api/v1/health > /dev/null; then
  echo "❌ API not accessible at http://localhost:8080"
  exit 1
fi
echo "✅ API accessible at localhost:8080"

# Check 4: Run auth setup and verify cookie domain
echo ""
echo "Running auth setup..."
if ! npx playwright test --project=setup; then
  echo "❌ Auth setup failed"
  exit 1
fi

# Check 5: Verify stored cookie domain
AUTH_FILE="playwright/.auth/user.json"
if [[ -f "$AUTH_FILE" ]]; then
  COOKIE_DOMAIN=$(jq -r '.cookies[] | select(.name=="auth_token") | .domain // empty' "$AUTH_FILE" 2>/dev/null || echo "")
  if [[ -z "$COOKIE_DOMAIN" ]]; then
    echo "❌ No auth_token cookie found in $AUTH_FILE"
    exit 1
  elif [[ "$COOKIE_DOMAIN" == "localhost" || "$COOKIE_DOMAIN" == ".localhost" ]]; then
    echo "✅ Auth cookie domain is localhost"
  else
    echo "❌ Auth cookie domain is '$COOKIE_DOMAIN' (expected 'localhost')"
    exit 1
  fi
else
  echo "❌ Auth state file not found at $AUTH_FILE"
  exit 1
fi

echo ""
echo "=== All validation checks passed ==="
echo "You can now run the user management tests:"
echo "  npx playwright test tests/settings/user-management.spec.ts --project=chromium"
