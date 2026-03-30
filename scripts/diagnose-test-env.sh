#!/bin/bash
# E2E Test Environment Diagnostic Script
# Checks Cerberus, CrowdSec, and security module states

set -euo pipefail

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  E2E Environment Diagnostics"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if container is running
echo "1. Container Status:"
if docker ps --format '{{.Names}}' | grep -q "charon-e2e"; then
  echo -e "   ${GREEN}✓${NC} charon-e2e container is running"
  CONTAINER_RUNNING=true
else
  echo -e "   ${RED}✗${NC} charon-e2e container is NOT running"
  echo ""
  echo "   Run: .github/skills/scripts/skill-runner.sh docker-rebuild-e2e"
  exit 1
fi

echo ""

# Check emergency server
echo "2. Emergency Server Status:"
if curl -sf http://localhost:2020/health > /dev/null 2>&1; then
  echo -e "   ${GREEN}✓${NC} Emergency server (port 2020) is responding"
else
  echo -e "   ${RED}✗${NC} Emergency server is not responding"
fi

echo ""

# Check application server
echo "3. Application Server Status:"
if curl -sf http://localhost:8080/api/v1/health > /dev/null 2>&1; then
  echo -e "   ${GREEN}✓${NC} Application server (port 8080) is responding"
else
  echo -e "   ${RED}✗${NC} Application server is not responding"
fi

echo ""

# Get emergency credentials
EMERGENCY_TOKEN=$(grep EMERGENCY_TOKEN .env 2>/dev/null | cut -d= -f2 | tr -d '"' || echo "")

# Get Cerberus feature state
echo "4. Cerberus Feature State:"
if [ -z "$EMERGENCY_TOKEN" ]; then
  echo -e "   ${RED}✗${NC} Emergency token not found in .env"
  CERBERUS_STATE="NO_AUTH"
else
  CERBERUS_STATE=$(curl -sf -H "X-Emergency-Token: $EMERGENCY_TOKEN" http://localhost:2020/emergency/settings | jq -r '.feature.cerberus.enabled // "NOT FOUND"' 2>/dev/null || echo "ERROR")
fi

if [ "$CERBERUS_STATE" = "true" ]; then
  echo -e "   ${GREEN}✓${NC} feature.cerberus.enabled = true"
elif [ "$CERBERUS_STATE" = "false" ]; then
  echo -e "   ${YELLOW}⚠${NC} feature.cerberus.enabled = false"
else
  echo -e "   ${RED}✗${NC} feature.cerberus.enabled = $CERBERUS_STATE"
fi

echo ""

# Get security module states
echo "5. Security Module States:"
if [ -n "$EMERGENCY_TOKEN" ]; then
  SECURITY_JSON=$(curl -sf -H "X-Emergency-Token: $EMERGENCY_TOKEN" http://localhost:2020/emergency/settings | jq -r '.security // {}' 2>/dev/null || echo "{}")
else
  SECURITY_JSON="{}"
fi

echo "   ACL Enabled:          $(echo "$SECURITY_JSON" | jq -r '.acl.enabled // "NOT FOUND"')"
echo "   WAF Enabled:          $(echo "$SECURITY_JSON" | jq -r '.waf.enabled // "NOT FOUND"')"
echo "   Rate Limit Enabled:   $(echo "$SECURITY_JSON" | jq -r '.rate_limit.enabled // "NOT FOUND"')"
echo "   CrowdSec Enabled:     $(echo "$SECURITY_JSON" | jq -r '.crowdsec.enabled // "NOT FOUND"')"
echo "   CrowdSec Mode:        $(echo "$SECURITY_JSON" | jq -r '.crowdsec.mode // "NOT FOUND"')"
echo "   Cerberus Enabled:     $(echo "$SECURITY_JSON" | jq -r '.cerberus.enabled // "NOT FOUND"')"

echo ""

# Check CrowdSec process
echo "6. CrowdSec Process Status:"
if docker exec charon-e2e pgrep crowdsec > /dev/null 2>&1; then
  PID=$(docker exec charon-e2e pgrep crowdsec)
  echo -e "   ${GREEN}✓${NC} CrowdSec is RUNNING (PID: $PID)"
else
  echo -e "   ${YELLOW}⚠${NC} CrowdSec is NOT RUNNING"
fi

echo ""

# Check CrowdSec LAPI
echo "7. CrowdSec LAPI Status:"
if docker exec charon-e2e wget -qO /dev/null http://localhost:8090/health 2>/dev/null; then
  echo -e "   ${GREEN}✓${NC} CrowdSec LAPI is responding (port 8090)"
else
  echo -e "   ${YELLOW}⚠${NC} CrowdSec LAPI is not responding"
fi

echo ""

# Check relevant environment variables
echo "8. Container Environment Variables:"
RELEVANT_VARS=$(docker exec charon-e2e env | grep -E "CERBERUS|CROWDSEC|SECURITY|EMERGENCY" | sort || echo "")

if [ -n "$RELEVANT_VARS" ]; then
  echo "$RELEVANT_VARS" | while IFS= read -r line; do
    echo "   $line"
  done
else
  echo -e "   ${YELLOW}⚠${NC} No relevant environment variables found"
fi

echo ""

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Summary & Recommendations"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Analyze state and provide recommendations
if [ "$CERBERUS_STATE" = "false" ]; then
  echo -e "${YELLOW}⚠ WARNING:${NC} Cerberus is DISABLED"
  echo "   This will cause tests to skip when they check toggle.isDisabled()"
  echo ""
  echo "   Tests affected:"
  echo "   - Security Dashboard toggle tests"
  echo "   - Rate Limiting toggle tests"
  echo "   - Navigation tests (configure buttons disabled)"
  echo ""
  echo "   Recommendations:"
  echo "   1. Review tests/global-setup.ts emergency reset logic"
  echo "   2. Consider enabling Cerberus but disabling modules:"
  echo "      - feature.cerberus.enabled = true"
  echo "      - security.acl.enabled = false"
  echo "      - security.waf.enabled = false"
  echo "      - etc."
  echo ""
fi

if ! docker exec charon-e2e pgrep crowdsec > /dev/null 2>&1; then
  echo -e "${YELLOW}⚠ INFO:${NC} CrowdSec is NOT RUNNING"
  echo "   - CrowdSec decision tests are explicitly skipped (test.describe.skip)"
  echo "   - This is expected for E2E tests"
  echo "   - CrowdSec functionality is tested in integration tests"
  echo ""
fi

echo "For more details, see:"
echo "  - Triage Plan: docs/plans/e2e-test-triage-plan.md"
echo "  - Global Setup: tests/global-setup.ts"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
