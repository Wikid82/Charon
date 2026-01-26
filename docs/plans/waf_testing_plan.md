# Charon WAF (Web Application Firewall) Testing Plan

**Version:** 1.0
**Date:** 2025-12-12
**Status:** 🔵 READY FOR TESTING
**Issue:** #319

---

## 1. WAF Implementation Summary

### 1.1 Architecture Overview

Charon's WAF is powered by the **Coraza WAF** (via the `coraza-caddy` plugin), a ModSecurity-compatible web application firewall. The integration works as follows:

1. **Charon API** manages rulesets in the database (`SecurityRuleSet` model)
2. **Caddy Manager** writes ruleset files to disk (`data/caddy/coraza/rulesets/*.conf`)
3. **Caddy** loads the rules via the `coraza-caddy` plugin's `waf` handler
4. **Request processing**: Each request passes through the WAF handler before reaching the proxy backend

### 1.2 Key Components

| Component | File | Description |
|-----------|------|-------------|
| SecurityRuleSet Model | [security_ruleset.go](../../backend/internal/models/security_ruleset.go) | Database model for WAF rulesets |
| SecurityConfig Model | [security_config.go](../../backend/internal/models/security_config.go) | Global WAF mode and settings |
| Security Handler | [security_handler.go](../../backend/internal/api/handlers/security_handler.go) | API endpoints for WAF management |
| Caddy Manager | [manager.go](../../backend/internal/caddy/manager.go) | Writes ruleset files, builds Caddy config |
| Config Generator | [config.go](../../backend/internal/caddy/config.go) | `buildWAFHandler()` and `buildWAFDirectives()` |

### 1.3 WAF Modes

| Mode | `waf_mode` Value | SecRuleEngine | Behavior |
|------|------------------|---------------|----------|
| **Disabled** | `disabled` | N/A | WAF handler not added to routes |
| **Monitor** | `monitor` | `DetectionOnly` | Logs attacks but allows through |
| **Block** | `block` | `On` | Blocks malicious requests with HTTP 403 |

### 1.4 Configuration Model

**Source:** [backend/internal/models/security_config.go](../../backend/internal/models/security_config.go)

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| `WAFMode` | `string` | `waf_mode` | Mode: `disabled`, `monitor`, `block` |
| `WAFRulesSource` | `string` | `waf_rules_source` | Name of active ruleset |
| `WAFLearning` | `bool` | `waf_learning` | Learning mode (future use) |
| `WAFParanoiaLevel` | `int` | `waf_paranoia_level` | OWASP CRS paranoia (1-4) |
| `WAFExclusions` | `string` | `waf_exclusions` | JSON array of rule exclusions |

### 1.5 Ruleset Model

**Source:** [backend/internal/models/security_ruleset.go](../../backend/internal/models/security_ruleset.go)

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| `ID` | `uint` | `id` | Primary key |
| `UUID` | `string` | `uuid` | Unique identifier |
| `Name` | `string` | `name` | Ruleset name (used for matching) |
| `SourceURL` | `string` | `source_url` | Optional source URL |
| `Mode` | `string` | `mode` | Per-ruleset mode override |
| `Content` | `string` | `content` | ModSecurity directive content |
| `LastUpdated` | `time.Time` | `last_updated` | Last modification timestamp |

### 1.6 API Endpoints

**All endpoints require authentication (session cookie)**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/security/status` | GET | Get WAF status (mode, enabled) |
| `/api/v1/security/config` | GET | Get full security config |
| `/api/v1/security/config` | POST | Update security config |
| `/api/v1/security/rulesets` | GET | List all rulesets |
| `/api/v1/security/rulesets` | POST | Create/update a ruleset |
| `/api/v1/security/rulesets/:id` | DELETE | Delete a ruleset |
| `/api/v1/security/waf/exclusions` | GET | Get WAF rule exclusions |
| `/api/v1/security/waf/exclusions` | POST | Add a WAF rule exclusion |
| `/api/v1/security/waf/exclusions/:rule_id` | DELETE | Remove a WAF rule exclusion |

### 1.7 Enabling WAF

WAF requires **two conditions** to be active:

1. **Cerberus must be enabled:**
   - Set `feature.cerberus.enabled` = `true` in Settings
   - OR set env `CERBERUS_SECURITY_CERBERUS_ENABLED=true`

2. **WAF Mode must be non-disabled:**
   - Via API: Set `waf_mode` to `monitor` or `block` in SecurityConfig
   - Via env: `CHARON_SECURITY_WAF_MODE=block`

3. **A ruleset must be configured:**
   - At least one `SecurityRuleSet` must exist in the database
   - `waf_rules_source` should reference that ruleset name

### 1.8 Ruleset File Location

Rulesets are written to: `data/caddy/coraza/rulesets/{sanitized-name}-{hash}.conf`

The hash is derived from content to ensure Caddy reloads when rules change.

### 1.9 Generated Caddy JSON Structure

**Source:** [backend/internal/caddy/config.go#L840-L980](../../backend/internal/caddy/config.go)

```json
{
  "handler": "waf",
  "directives": "SecRuleEngine On\nSecRequestBodyAccess On\nSecResponseBodyAccess Off\nInclude /app/data/caddy/coraza/rulesets/integration-xss-abc12345.conf"
}
```

---

## 2. Existing Test Coverage

### 2.1 Unit Tests

| Test File | Coverage |
|-----------|----------|
| [security_handler_waf_test.go](../../backend/internal/api/handlers/security_handler_waf_test.go) | WAF exclusion endpoints |
| [config_test.go](../../backend/internal/caddy/config_test.go) | `buildWAFHandler` tests |
| [manager.go](../../backend/internal/caddy/manager.go) | Ruleset file writing |

### 2.2 Integration Tests

| Script | Go Test | Coverage |
|--------|---------|----------|
| [scripts/coraza_integration.sh](../../scripts/coraza_integration.sh) | [coraza_integration_test.go](../../backend/integration/coraza_integration_test.go) | XSS blocking, mode switching |

### 2.3 Existing Integration Test Analysis

The existing `coraza_integration.sh` tests:

- ✅ XSS payload blocking (`<script>alert(1)</script>`)
- ✅ BLOCK mode (expects HTTP 403)
- ✅ MONITOR mode switching (expects HTTP 200 after mode change)
- ⚠️ Does NOT test SQL injection patterns
- ⚠️ Does NOT test multiple rulesets
- ⚠️ Does NOT test OWASP CRS specifically

---

## 3. Test Environment Setup

### 3.1 Prerequisites

1. **Docker running** with `charon:local` image built
2. **Cerberus enabled** via environment variables
3. **Network access** to ports 8080 (API), 80 (Caddy HTTP), 2019 (Caddy Admin)

### 3.2 Docker Run Command

```bash
# Build the image
docker build -t charon:local .

# Remove any existing test container
docker rm -f charon-waf-test 2>/dev/null || true

# Ensure network exists
docker network inspect containers_default >/dev/null 2>&1 || docker network create containers_default

# Run Charon with WAF enabled
docker run -d --name charon-waf-test \
  --network containers_default \
  -p 80:80 \
  -p 443:443 \
  -p 8080:8080 \
  -p 2019:2019 \
  -e CHARON_ENV=development \
  -e CHARON_DEBUG=1 \
  -e CHARON_HTTP_PORT=8080 \
  -e CHARON_DB_PATH=/app/data/charon.db \
  -e CHARON_FRONTEND_DIR=/app/frontend/dist \
  -e CHARON_CADDY_ADMIN_API=http://localhost:2019 \
  -e CHARON_CADDY_CONFIG_DIR=/app/data/caddy \
  -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
  -e CHARON_SECURITY_WAF_MODE=block \
  -v charon_data:/app/data \
  charon:local
```

### 3.3 Backend Container for Testing

```bash
# Start httpbin as a test backend
docker rm -f waf-backend 2>/dev/null || true
docker run -d --name waf-backend --network containers_default kennethreitz/httpbin
```

### 3.4 Authentication Setup

```bash
TMP_COOKIE=$(mktemp)

# Register test user
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"email":"waf-test@example.local","password":"password123","name":"WAF Tester"}' \
  http://localhost:8080/api/v1/auth/register

# Login and save cookie
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"email":"waf-test@example.local","password":"password123"}' \
  -c ${TMP_COOKIE} \
  http://localhost:8080/api/v1/auth/login
```

### 3.5 Create Proxy Host

```bash
# Create proxy host pointing to backend
PROXY_HOST_PAYLOAD='{
  "name": "waf-test-backend",
  "domain_names": "waf.test.local",
  "forward_scheme": "http",
  "forward_host": "waf-backend",
  "forward_port": 80,
  "enabled": true
}'

curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d "${PROXY_HOST_PAYLOAD}" \
  http://localhost:8080/api/v1/proxy-hosts
```

---

## 4. Test Cases

### 4.1 Test Case: Create Custom SQLi Protection Ruleset

**Objective:** Create a ruleset that blocks SQL injection patterns

**Curl Command:**

```bash
echo "=== TC-1: Create SQLi Ruleset ==="

SQLI_RULESET='{
  "name": "sqli-protection",
  "content": "SecRule ARGS|ARGS_NAMES|REQUEST_BODY \"(?i:(?:''|\\''|--|#|\\/\\*|\\*\\/|%27|%23|%2D%2D|UNION\\s+SELECT|SELECT\\s+.+\\s+FROM|INSERT\\s+INTO|DELETE\\s+FROM|UPDATE\\s+.+\\s+SET|DROP\\s+TABLE|OR\\s+1\\s*=\\s*1|OR\\s+''1''\\s*=\\s*''1)\" \"id:10001,phase:2,deny,status:403,msg:''SQL Injection Attempt''\""
}'

RESP=$(curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d "${SQLI_RULESET}" \
  http://localhost:8080/api/v1/security/rulesets)

echo "$RESP" | jq .
# Expected: {"ruleset": {"name": "sqli-protection", ...}}
```

**Expected Response:**

```json
{
  "ruleset": {
    "id": 1,
    "uuid": "...",
    "name": "sqli-protection",
    "content": "SecRule ARGS|ARGS_NAMES|REQUEST_BODY ...",
    "last_updated": "..."
  }
}
```

---

### 4.2 Test Case: Create XSS Protection Ruleset

**Objective:** Create a ruleset that blocks XSS patterns

**Curl Command:**

```bash
echo "=== TC-2: Create XSS Ruleset ==="

XSS_RULESET='{
  "name": "xss-protection",
  "content": "SecRule REQUEST_BODY|ARGS|ARGS_NAMES \"<script\" \"id:10002,phase:2,deny,status:403,msg:''XSS Attack Detected''\""
}'

RESP=$(curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d "${XSS_RULESET}" \
  http://localhost:8080/api/v1/security/rulesets)

echo "$RESP" | jq .
```

---

### 4.3 Test Case: Enable WAF in Block Mode

**Objective:** Set WAF mode to blocking with a specific ruleset

**Curl Command:**

```bash
echo "=== TC-3: Enable WAF (Block Mode) ==="

WAF_CONFIG='{
  "name": "default",
  "enabled": true,
  "waf_mode": "block",
  "waf_rules_source": "sqli-protection",
  "admin_whitelist": "0.0.0.0/0"
}'

RESP=$(curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d "${WAF_CONFIG}" \
  http://localhost:8080/api/v1/security/config)

echo "$RESP" | jq .

# Wait for Caddy to reload
sleep 5
```

**Verification:**

```bash
# Check WAF status
curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/status | jq '.waf'
# Expected: {"mode": "block", "enabled": true}
```

---

### 4.4 Test Case: SQL Injection Blocking

**Objective:** Verify SQLi patterns are blocked with HTTP 403

**Test Payloads:**

```bash
echo "=== TC-4: SQL Injection Blocking ==="

# Test 1: Classic OR 1=1
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?id=1%27%20OR%20%271%27=%271")
echo "SQLi OR 1=1: HTTP $RESP (expect 403)"

# Test 2: UNION SELECT
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?id=1%20UNION%20SELECT%20*%20FROM%20users")
echo "SQLi UNION SELECT: HTTP $RESP (expect 403)"

# Test 3: Comment injection
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?id=1--")
echo "SQLi Comment: HTTP $RESP (expect 403)"

# Test 4: POST body injection
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=admin' OR '1'='1" \
  http://localhost/post)
echo "SQLi POST body: HTTP $RESP (expect 403)"
```

**Expected Results:**

- All requests return HTTP 403

---

### 4.5 Test Case: XSS Blocking

**Objective:** Verify XSS patterns are blocked with HTTP 403

**Curl Commands:**

```bash
echo "=== TC-5: XSS Blocking ==="

# First, switch to XSS ruleset
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{"name":"default","enabled":true,"waf_mode":"block","waf_rules_source":"xss-protection","admin_whitelist":"0.0.0.0/0"}' \
  http://localhost:8080/api/v1/security/config
sleep 5

# Test 1: Script tag in query param
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?q=%3Cscript%3Ealert(1)%3C/script%3E")
echo "XSS script tag (query): HTTP $RESP (expect 403)"

# Test 2: Script tag in POST body
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -d "<script>alert(1)</script>" \
  http://localhost/post)
echo "XSS script tag (POST): HTTP $RESP (expect 403)"

# Test 3: Script tag in JSON
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -H "Content-Type: application/json" \
  -d '{"comment":"<script>alert(1)</script>"}' \
  http://localhost/post)
echo "XSS script tag (JSON): HTTP $RESP (expect 403)"
```

**Expected Results:**

- All requests return HTTP 403

---

### 4.6 Test Case: Detection (Monitor) Mode

**Objective:** Verify requests pass but are logged in monitor mode

**Curl Commands:**

```bash
echo "=== TC-6: Detection Mode ==="

# Switch to monitor mode
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{"name":"default","enabled":true,"waf_mode":"monitor","waf_rules_source":"xss-protection","admin_whitelist":"0.0.0.0/0"}' \
  http://localhost:8080/api/v1/security/config
sleep 5

# Verify mode changed
curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/status | jq '.waf'
# Expected: {"mode": "monitor", "enabled": true}

# Send malicious payload - should pass through
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -d "<script>alert(1)</script>" \
  http://localhost/post)
echo "XSS in monitor mode: HTTP $RESP (expect 200)"

# Check Caddy logs for detection (inside container)
docker exec charon-waf-test sh -c 'tail -50 /var/log/caddy/access.log 2>/dev/null | grep -i "xss\|waf"' || \
  echo "Note: Check container logs for WAF detection entries"
```

**Expected Results:**

- HTTP 200 response (request passes through)
- WAF detection logged (in Caddy access logs or Coraza logs)

---

### 4.7 Test Case: Multiple Rulesets

**Objective:** Verify both SQLi and XSS rules can be combined

**Curl Commands:**

```bash
echo "=== TC-7: Multiple Rulesets (Combined) ==="

# Create a combined ruleset
COMBINED_RULESET='{
  "name": "combined-protection",
  "content": "SecRule ARGS|REQUEST_BODY \"(?i:OR\\s+1\\s*=\\s*1|UNION\\s+SELECT)\" \"id:10001,phase:2,deny,status:403,msg:''SQLi''\"\nSecRule ARGS|REQUEST_BODY \"<script\" \"id:10002,phase:2,deny,status:403,msg:''XSS''\""
}'

curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d "${COMBINED_RULESET}" \
  http://localhost:8080/api/v1/security/rulesets

# Enable the combined ruleset
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{"name":"default","enabled":true,"waf_mode":"block","waf_rules_source":"combined-protection","admin_whitelist":"0.0.0.0/0"}' \
  http://localhost:8080/api/v1/security/config
sleep 5

# Test SQLi (should be blocked)
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?id=1%20OR%201=1")
echo "Combined - SQLi: HTTP $RESP (expect 403)"

# Test XSS (should be blocked)
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -d "<script>alert(1)</script>" \
  http://localhost/post)
echo "Combined - XSS: HTTP $RESP (expect 403)"

# Test legitimate request (should pass)
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?name=john&age=25")
echo "Combined - Legitimate: HTTP $RESP (expect 200)"
```

---

### 4.8 Test Case: List Rulesets

**Objective:** Verify all rulesets are listed correctly

**Curl Command:**

```bash
echo "=== TC-8: List Rulesets ==="

RESP=$(curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/rulesets)
echo "$RESP" | jq '.rulesets[] | {name, mode, last_updated}'
```

**Expected Response:**

```json
[
  {"name": "sqli-protection", "mode": "", "last_updated": "..."},
  {"name": "xss-protection", "mode": "", "last_updated": "..."},
  {"name": "combined-protection", "mode": "", "last_updated": "..."}
]
```

---

### 4.9 Test Case: WAF Rule Exclusions

**Objective:** Add and remove WAF rule exclusions for false positives

**Curl Commands:**

```bash
echo "=== TC-9: WAF Rule Exclusions ==="

# Add an exclusion for rule 10001 (SQLi)
RESP=$(curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{"rule_id": 10001, "description": "False positive on search form"}' \
  http://localhost:8080/api/v1/security/waf/exclusions)
echo "Add exclusion: $RESP"

# List exclusions
RESP=$(curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/waf/exclusions)
echo "Exclusions list: $RESP"

# Remove exclusion
RESP=$(curl -s -X DELETE -b ${TMP_COOKIE} \
  http://localhost:8080/api/v1/security/waf/exclusions/10001)
echo "Delete exclusion: $RESP"
```

---

### 4.10 Test Case: Verify Caddy Config

**Objective:** Confirm WAF handler is present in running Caddy config

**Curl Command:**

```bash
echo "=== TC-10: Verify Caddy Config ==="

# Get Caddy config
CONFIG=$(curl -s http://localhost:2019/config)

# Check for WAF handler
if echo "$CONFIG" | grep -q '"handler":"waf"'; then
  echo "✓ WAF handler found in Caddy config"
else
  echo "✗ WAF handler NOT found in Caddy config"
fi

# Check for ruleset include
if echo "$CONFIG" | grep -q 'Include'; then
  echo "✓ Ruleset Include directive found"
else
  echo "✗ Ruleset Include NOT found"
fi

# Check SecRuleEngine mode
if echo "$CONFIG" | grep -q 'SecRuleEngine On'; then
  echo "✓ SecRuleEngine is On (blocking mode)"
elif echo "$CONFIG" | grep -q 'SecRuleEngine DetectionOnly'; then
  echo "✓ SecRuleEngine is DetectionOnly (monitor mode)"
else
  echo "⚠ SecRuleEngine directive not found"
fi
```

---

### 4.11 Test Case: Delete Ruleset

**Objective:** Verify ruleset can be deleted

**Curl Commands:**

```bash
echo "=== TC-11: Delete Ruleset ==="

# Get ruleset ID
RULESET_ID=$(curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/rulesets | \
  jq -r '.rulesets[] | select(.name == "sqli-protection") | .id')

# Delete the ruleset
RESP=$(curl -s -X DELETE -b ${TMP_COOKIE} \
  http://localhost:8080/api/v1/security/rulesets/${RULESET_ID})
echo "$RESP"
# Expected: {"deleted": true}

# Verify deletion
curl -s -b ${TMP_COOKIE} http://localhost:8080/api/v1/security/rulesets | jq '.rulesets[].name'
```

---

## 5. Integration Test Script

### 5.1 Script Location

`scripts/waf_integration.sh`

### 5.2 Script Outline

```bash
#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for WAF (Coraza) functionality
# Tests: Ruleset creation, blocking mode, monitor mode, multiple rulesets

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

PASSED=0
FAILED=0

assert_http() {
  local expected=$1
  local actual=$2
  local desc=$3
  if [ "$actual" = "$expected" ]; then
    log_info "  ✓ $desc: HTTP $actual"
    ((PASSED++))
  else
    log_error "  ✗ $desc: HTTP $actual (expected $expected)"
    ((FAILED++))
  fi
}

# Cleanup function
cleanup() {
  log_info "Cleaning up..."
  docker rm -f charon-waf-test waf-backend 2>/dev/null || true
  rm -f "${TMP_COOKIE:-}" 2>/dev/null || true
}
trap cleanup EXIT ERR

# Build and start
log_info "Building charon:local image..."
docker build -t charon:local .

log_info "Starting containers..."
docker network inspect containers_default >/dev/null 2>&1 || docker network create containers_default
docker rm -f charon-waf-test waf-backend 2>/dev/null || true

docker run -d --name waf-backend --network containers_default kennethreitz/httpbin

docker run -d --name charon-waf-test \
  --network containers_default \
  -p 80:80 -p 8080:8080 -p 2019:2019 \
  -e CHARON_ENV=development \
  -e CHARON_DEBUG=1 \
  -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
  -e CHARON_SECURITY_WAF_MODE=block \
  charon:local

log_info "Waiting for API..."
for i in {1..30}; do
  curl -sf http://localhost:8080/api/v1/ >/dev/null 2>&1 && break
  sleep 1
done

log_info "Waiting for backend..."
for i in {1..20}; do
  docker exec charon-waf-test curl -s http://waf-backend/get >/dev/null 2>&1 && break
  sleep 1
done

# Authenticate
TMP_COOKIE=$(mktemp)
curl -sf -X POST -H "Content-Type: application/json" \
  -d '{"email":"waf-test@example.local","password":"password123","name":"WAF Tester"}' \
  http://localhost:8080/api/v1/auth/register >/dev/null || true
curl -sf -X POST -H "Content-Type: application/json" \
  -d '{"email":"waf-test@example.local","password":"password123"}' \
  -c ${TMP_COOKIE} http://localhost:8080/api/v1/auth/login >/dev/null

# Create proxy host
curl -sf -X POST -H "Content-Type: application/json" -b ${TMP_COOKIE} \
  -d '{"name":"waf-test","domain_names":"waf.test.local","forward_scheme":"http","forward_host":"waf-backend","forward_port":80,"enabled":true}' \
  http://localhost:8080/api/v1/proxy-hosts >/dev/null

sleep 3

# === TEST 1: Create XSS Ruleset ===
log_info "TEST 1: Create XSS Ruleset"
curl -sf -X POST -H "Content-Type: application/json" -b ${TMP_COOKIE} \
  -d '{"name":"test-xss","content":"SecRule REQUEST_BODY \"<script\" \"id:12345,phase:2,deny,status:403,msg:XSS blocked\""}' \
  http://localhost:8080/api/v1/security/rulesets >/dev/null
log_info "  ✓ Ruleset created"
((PASSED++))

# === TEST 2: Enable WAF (Block Mode) ===
log_info "TEST 2: Enable WAF (Block Mode)"
curl -sf -X POST -H "Content-Type: application/json" -b ${TMP_COOKIE} \
  -d '{"name":"default","enabled":true,"waf_mode":"block","waf_rules_source":"test-xss","admin_whitelist":"0.0.0.0/0"}' \
  http://localhost:8080/api/v1/security/config >/dev/null
sleep 5
log_info "  ✓ WAF enabled in block mode"
((PASSED++))

# === TEST 3: XSS Blocking ===
log_info "TEST 3: XSS Blocking"
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -d "<script>alert(1)</script>" \
  http://localhost/post)
assert_http "403" "$RESP" "XSS payload blocked"

# === TEST 4: Legitimate Request ===
log_info "TEST 4: Legitimate Request"
RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -d "name=john&age=25" \
  http://localhost/post)
assert_http "200" "$RESP" "Legitimate request passed"

# === TEST 5: Monitor Mode ===
log_info "TEST 5: Switch to Monitor Mode"
curl -sf -X POST -H "Content-Type: application/json" -b ${TMP_COOKIE} \
  -d '{"name":"default","enabled":true,"waf_mode":"monitor","waf_rules_source":"test-xss","admin_whitelist":"0.0.0.0/0"}' \
  http://localhost:8080/api/v1/security/config >/dev/null
sleep 5

RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  -d "<script>alert(1)</script>" \
  http://localhost/post)
assert_http "200" "$RESP" "XSS in monitor mode (allowed through)"

# === TEST 6: Create SQLi Ruleset ===
log_info "TEST 6: Create SQLi Ruleset"
curl -sf -X POST -H "Content-Type: application/json" -b ${TMP_COOKIE} \
  -d '{"name":"test-sqli","content":"SecRule ARGS \"(?i:OR\\s+1\\s*=\\s*1)\" \"id:12346,phase:2,deny,status:403,msg:SQLi blocked\""}' \
  http://localhost:8080/api/v1/security/rulesets >/dev/null
log_info "  ✓ SQLi ruleset created"
((PASSED++))

# === TEST 7: SQLi Blocking ===
log_info "TEST 7: SQLi Blocking"
curl -sf -X POST -H "Content-Type: application/json" -b ${TMP_COOKIE} \
  -d '{"name":"default","enabled":true,"waf_mode":"block","waf_rules_source":"test-sqli","admin_whitelist":"0.0.0.0/0"}' \
  http://localhost:8080/api/v1/security/config >/dev/null
sleep 5

RESP=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: waf.test.local" \
  "http://localhost/get?id=1%20OR%201=1")
assert_http "403" "$RESP" "SQLi payload blocked"

# === SUMMARY ===
echo ""
log_info "=== WAF Integration Test Results ==="
log_info "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
  log_error "Failed: $FAILED"
  exit 1
else
  log_info "Failed: $FAILED"
  log_info "=== All WAF tests passed ==="
fi
```

### 5.3 Go Test Wrapper

Location: `backend/integration/waf_integration_test.go`

```go
//go:build integration
// +build integration

package integration

import (
 "context"
 "os/exec"
 "strings"
 "testing"
 "time"
)

// TestWAFIntegration runs the scripts/waf_integration.sh and ensures it completes successfully.
func TestWAFIntegration(t *testing.T) {
 t.Parallel()

 ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
 defer cancel()

 cmd := exec.CommandContext(ctx, "bash", "./scripts/waf_integration.sh")
 cmd.Dir = "../.."

 out, err := cmd.CombinedOutput()
 t.Logf("waf_integration script output:\n%s", string(out))

 if err != nil {
  t.Fatalf("waf integration failed: %v", err)
 }

 if !strings.Contains(string(out), "All WAF tests passed") {
  t.Fatalf("unexpected script output, expected pass assertion not found")
 }
}
```

---

## 6. VS Code Task

Add to `.vscode/tasks.json`:

```json
{
  "label": "WAF: Run Integration Script",
  "type": "shell",
  "command": "bash",
  "args": ["./scripts/waf_integration.sh"],
  "group": "test"
},
{
  "label": "WAF: Run Integration Go Test",
  "type": "shell",
  "command": "sh",
  "args": [
    "-c",
    "cd backend && go test -tags=integration ./integration -run TestWAFIntegration -v"
  ],
  "group": "test"
}
```

---

## 7. Verification Checklist

### 7.1 Ruleset Management

- [ ] Create new ruleset via API
- [ ] Update existing ruleset content
- [ ] Delete ruleset via API
- [ ] List all rulesets
- [ ] Ruleset file written to `data/caddy/coraza/rulesets/`

### 7.2 WAF Modes

- [ ] `disabled` - No WAF handler in Caddy config
- [ ] `monitor` - Requests pass, attacks logged
- [ ] `block` - Malicious requests return HTTP 403

### 7.3 Attack Detection

- [ ] SQL injection patterns blocked
- [ ] XSS patterns blocked
- [ ] Legitimate requests pass through
- [ ] POST body inspection works
- [ ] Query parameter inspection works

### 7.4 Configuration

- [ ] `waf_mode` setting persists in database
- [ ] `waf_rules_source` links to correct ruleset
- [ ] Mode changes take effect after Caddy reload
- [ ] WAF exclusions can be added/removed

### 7.5 Integration

- [ ] Cerberus must be enabled for WAF to work
- [ ] WAF handler appears in Caddy admin API config
- [ ] Ruleset `Include` directive present in directives

---

## 8. Known Limitations

1. **No OWASP CRS bundled:** Charon doesn't include OWASP Core Rule Set by default; users must upload custom rules or import CRS manually.

2. **Single active ruleset:** The `waf_rules_source` field points to one ruleset at a time; combining multiple rulesets requires creating a merged ruleset.

3. **No audit logging UI:** WAF detections are logged to Caddy/Coraza logs, not surfaced in the Charon UI.

4. **ModSecurity directives only:** The ruleset content must use ModSecurity directive syntax compatible with Coraza.

5. **Paranoia level not fully implemented:** The `waf_paranoia_level` field exists but may not be applied to custom rulesets (only meaningful for OWASP CRS).

---

## 9. Debug Commands

### View Caddy WAF Handler

```bash
curl -s http://localhost:2019/config | jq '.. | objects | select(.handler == "waf")'
```

### View Ruleset Files in Container

```bash
docker exec charon-waf-test ls -la /app/data/caddy/coraza/rulesets/
docker exec charon-waf-test cat /app/data/caddy/coraza/rulesets/*.conf
```

### Check Caddy Logs for WAF Events

```bash
docker logs charon-waf-test 2>&1 | grep -i "waf\|coraza\|blocked"
```

### Verify SecRuleEngine Mode

```bash
docker exec charon-waf-test cat /app/data/caddy/coraza/rulesets/*.conf | grep SecRuleEngine
```

---

## 10. References

- [Coraza WAF Documentation](https://coraza.io/docs/)
- [coraza-caddy Plugin](https://github.com/corazawaf/coraza-caddy)
- [ModSecurity Directive Reference](https://github.com/owasp-modsecurity/ModSecurity/wiki/Reference-Manual)
- [OWASP Core Rule Set](https://coreruleset.org/)
- [coraza_integration.sh](../../scripts/coraza_integration.sh) - Existing integration test
- [security_handler.go](../../backend/internal/api/handlers/security_handler.go) - API handlers
- [config.go](../../backend/internal/caddy/config.go) - Caddy config generation

---

**Document Status:** Complete
**Last Updated:** 2025-12-12
