# Cerberus Full Integration Testing Plan

**Version:** 1.0
**Date:** 2025-12-12
**Issue:** #319
**Status:** ✅ COMPLETE

---

## 1. Overview

This plan tests all Cerberus security features **working together** to ensure no conflicts or ordering issues when multiple features are enabled simultaneously.

### 1.1 Features Under Test

| Feature | Handler Name | Purpose |
|---------|--------------|---------|
| Security Decisions | `subroute` (IP block) | Manual IP blocks via API |
| CrowdSec | `crowdsec` | IP reputation bouncer |
| WAF | `waf` (Coraza) | Payload inspection |
| Rate Limiting | `rate_limit` | Volume abuse prevention |
| ACL | `subroute` (IP/Geo) | Static allow/deny lists |

### 1.2 Pipeline Order (from `config.go`)

The security handlers execute in this order per-route:

```
Request → [Security Decisions] → [CrowdSec] → [WAF] → [Rate Limit] → [ACL] → [Proxy]
```

**Source:** [backend/internal/caddy/config.go#L216-290](../../backend/internal/caddy/config.go)

---

## 2. Test Environment Setup

### 2.1 Prerequisites

- Docker with `charon:local` image built
- Access to container ports: 8080 (API), 80/443 (proxy), 2019 (Caddy admin)

### 2.2 Environment Variables

```bash
# Enable all security features
export CERBERUS_SECURITY_CERBERUS_ENABLED=true
export CERBERUS_SECURITY_WAF_MODE=block
export CERBERUS_SECURITY_CROWDSEC_MODE=disabled  # or local if LAPI available
export CERBERUS_SECURITY_RATELIMIT_MODE=enabled
export CERBERUS_SECURITY_ACL_ENABLED=true
```

### 2.3 Container Startup

```bash
docker run -d --name charon-cerberus-test \
  -p 8080:8080 -p 80:80 -p 2019:2019 \
  -e CHARON_ENV=development \
  -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
  -e CERBERUS_SECURITY_WAF_MODE=block \
  -e CERBERUS_SECURITY_RATELIMIT_MODE=enabled \
  -e CERBERUS_SECURITY_ACL_ENABLED=true \
  charon:local
```

### 2.4 Test Proxy Host Creation

```bash
# Login and create test proxy host
curl -s -X POST http://localhost:8080/api/v1/proxy-hosts \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "domain_names": "cerberus.test.local",
    "forward_scheme": "http",
    "forward_host": "httpbin.org",
    "forward_port": 80,
    "enabled": true
  }'
```

---

## 3. Test Cases

### TC-1: Enable All Features Simultaneously

**Objective:** Verify all Cerberus features can be enabled without startup errors.

**Steps:**

1. Start container with all env vars set
2. Wait for Caddy reload
3. Query `/api/v1/security/status` to confirm all features enabled

**Verification:**

```bash
# Check security status
curl -s http://localhost:8080/api/v1/security/status | jq

# Expected:
# {
#   "enabled": true,
#   "waf_mode": "block",
#   "crowdsec_mode": "local|disabled",
#   "acl_enabled": true,
#   "rate_limit_enabled": true
# }
```

**Pass Criteria:**

- [ ] Container starts without errors
- [ ] All features report enabled in status
- [ ] No Caddy config validation errors in logs

---

### TC-2: Verify Pipeline Handler Order

**Objective:** Confirm security handlers are in correct order in Caddy config.

**Steps:**

1. Query Caddy admin API for running config
2. Parse handler order for the test route
3. Verify order matches expected: Decisions → CrowdSec → WAF → Rate Limit → ACL → Proxy

**Verification:**

```bash
# Extract handler order from Caddy config
curl -s http://localhost:2019/config | python3 -c "
import sys, json
config = json.load(sys.stdin)
servers = config.get('apps', {}).get('http', {}).get('servers', {})
for name, server in servers.items():
    for route in server.get('routes', []):
        hosts = []
        for m in route.get('match', []):
            hosts.extend(m.get('host', []))
        if 'cerberus.test.local' in hosts:
            handlers = [h.get('handler') for h in route.get('handle', [])]
            print('Handler order:', handlers)
"
```

**Expected Order:**

1. `subroute` (if decisions exist)
2. `crowdsec` (if enabled)
3. `waf` (if enabled)
4. `rate_limit` or `subroute` (rate limit wrapper)
5. `subroute` (ACL if enabled)
6. `reverse_proxy`

**Pass Criteria:**

- [ ] Security handlers appear before `reverse_proxy`
- [ ] Order matches expected pipeline

---

### TC-3: WAF Blocking Doesn't Break Rate Limiter

**Objective:** Ensure WAF-blocked requests don't consume rate limit quota incorrectly.

**Steps:**

1. Configure rate limit: 5 requests / 30 seconds
2. Send 3 malicious requests (WAF blocks)
3. Send 5 legitimate requests
4. Verify all 5 legitimate requests succeed (200)

**Verification:**

```bash
# Configure rate limit
curl -s -X POST http://localhost:8080/api/v1/security/config \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "rate_limit_requests": 5,
    "rate_limit_window_sec": 30
  }'

sleep 3

# 3 malicious requests (should be blocked by WAF with 403)
for i in 1 2 3; do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: cerberus.test.local" \
    "http://localhost/get?q=<script>alert(1)</script>")
  echo "Malicious $i: HTTP $CODE (expect 403)"
done

# 5 legitimate requests (should all succeed)
for i in 1 2 3 4 5; do
  CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: cerberus.test.local" \
    "http://localhost/get")
  echo "Legitimate $i: HTTP $CODE (expect 200)"
done
```

**Pass Criteria:**

- [ ] All malicious requests return 403 (WAF block)
- [ ] All 5 legitimate requests return 200 (not rate limited)

---

### TC-4: Rate Limiter Doesn't Break CrowdSec Decisions

**Objective:** Verify CrowdSec decisions are enforced even if rate limit not exceeded.

**Steps:**

1. Create a manual security decision blocking a test IP
2. Send request from that IP (via X-Forwarded-For if needed)
3. Verify request is blocked with 403 before rate limit check

**Verification:**

```bash
# Add manual IP block decision
curl -s -X POST http://localhost:8080/api/v1/security/decisions \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{
    "ip": "203.0.113.50",
    "action": "block",
    "reason": "Test block for TC-4"
  }'

sleep 3

# Request should be blocked immediately (decisions come first)
CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: cerberus.test.local" \
  -H "X-Forwarded-For: 203.0.113.50" \
  "http://localhost/get")
echo "Blocked IP request: HTTP $CODE (expect 403)"
```

**Pass Criteria:**

- [ ] Request from blocked IP returns 403
- [ ] Response body contains "Blocked by security decision"

---

### TC-5: Legitimate Traffic Flows Through All Layers

**Objective:** Confirm clean traffic passes through all security layers to upstream.

**Steps:**

1. Enable all features with permissive config
2. Send 10 legitimate requests
3. Verify all return 200 with upstream response

**Verification:**

```bash
echo "=== Testing legitimate traffic flow ==="

# Send 10 legitimate requests
SUCCESS=0
for i in $(seq 1 10); do
  BODY=$(curl -s -H "Host: cerberus.test.local" "http://localhost/get")
  if echo "$BODY" | grep -q "httpbin.org"; then
    ((SUCCESS++))
    echo "Request $i: ✓ Success (reached upstream)"
  else
    echo "Request $i: ✗ Failed"
  fi
done

echo "Total successful: $SUCCESS/10"
```

**Pass Criteria:**

- [ ] All 10 requests return 200
- [ ] Response body contains upstream (httpbin) content
- [ ] No unexpected 403/429 responses

---

### TC-6: Basic Latency Benchmark

**Objective:** Measure latency overhead when all Cerberus features are enabled.

**Steps:**

1. Measure baseline latency (Cerberus disabled)
2. Enable all Cerberus features
3. Measure latency with full security stack
4. Calculate overhead percentage

**Verification:**

```bash
# Baseline (security disabled)
echo "=== Baseline (Cerberus disabled) ==="
BASELINE=$(curl -s -o /dev/null -w "%{time_total}" \
  -H "Host: cerberus.test.local" "http://localhost/get")
echo "Baseline: ${BASELINE}s"

# With full security (already enabled)
echo "=== With all Cerberus features ==="
SECURED=$(curl -s -o /dev/null -w "%{time_total}" \
  -H "Host: cerberus.test.local" "http://localhost/get")
echo "Secured: ${SECURED}s"

# Calculate overhead (requires bc)
# OVERHEAD=$(echo "scale=2; ($SECURED - $BASELINE) / $BASELINE * 100" | bc)
# echo "Overhead: ${OVERHEAD}%"
```

**Load Test (10 concurrent, 100 total):**

```bash
# Using Apache Bench (ab)
ab -n 100 -c 10 -H "Host: cerberus.test.local" http://localhost/get

# Or using hey (Go HTTP load tester)
hey -n 100 -c 10 -host "cerberus.test.local" http://localhost/get
```

**Pass Criteria:**

- [ ] Average latency overhead < 50ms per request
- [ ] 95th percentile latency < 500ms
- [ ] No request failures under load

---

## 4. Integration Script Outline

Create `scripts/cerberus_integration.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Integration test for full Cerberus security stack
# Tests all features enabled simultaneously

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

CONTAINER_NAME="charon-cerberus-test"
TEST_DOMAIN="cerberus.test.local"

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
}
trap cleanup EXIT

# Build and start
echo "=== Building and starting Charon with full Cerberus ==="
docker build -t charon:local .
docker run -d --name ${CONTAINER_NAME} \
  -p 8380:8080 -p 8381:80 -p 2319:2019 \
  -e CHARON_ENV=development \
  -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
  -e CERBERUS_SECURITY_WAF_MODE=block \
  -e CERBERUS_SECURITY_RATELIMIT_MODE=enabled \
  -e CERBERUS_SECURITY_ACL_ENABLED=true \
  charon:local

# Wait for startup
sleep 10

# TC-1: Check status
echo "=== TC-1: Verifying all features enabled ==="
STATUS=$(curl -s http://localhost:8380/api/v1/security/status)
echo "$STATUS" | grep -q '"enabled":true' && echo "✓ Cerberus enabled" || exit 1

# TC-2: Check handler order
echo "=== TC-2: Verifying handler order ==="
# ... (parse Caddy config as shown in TC-2)

# TC-3: WAF + Rate Limit interaction
echo "=== TC-3: WAF blocking vs rate limiting ==="
# ... (implement test)

# TC-5: Legitimate traffic
echo "=== TC-5: Legitimate traffic flow ==="
# ... (implement test)

echo ""
echo "==========================================="
echo "ALL CERBERUS INTEGRATION TESTS PASSED"
echo "==========================================="
```

---

## 5. Go Test Wrapper

Create `backend/integration/cerberus_integration_test.go`:

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

func TestCerberusIntegration(t *testing.T) {
    t.Parallel()

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    cmd := exec.CommandContext(ctx, "bash", "../scripts/cerberus_integration.sh")
    cmd.Dir = ".."

    out, err := cmd.CombinedOutput()
    t.Logf("cerberus_integration output:\n%s", string(out))

    if err != nil {
        t.Fatalf("cerberus integration failed: %v", err)
    }

    if !strings.Contains(string(out), "ALL CERBERUS INTEGRATION TESTS PASSED") {
        t.Fatalf("unexpected output: final success message not found")
    }
}
```

---

## 6. VS Code Task

Add to `.vscode/tasks.json`:

```json
{
  "label": "Cerberus: Run Full Integration Script",
  "type": "shell",
  "command": "bash",
  "args": ["./scripts/cerberus_integration.sh"],
  "group": "test"
},
{
  "label": "Cerberus: Run Full Integration Go Test",
  "type": "shell",
  "command": "sh",
  "args": [
    "-c",
    "cd backend && go test -tags=integration ./integration -run TestCerberusIntegration -v"
  ],
  "group": "test"
}
```

---

## 7. Expected Issues & Mitigations

| Issue | Likelihood | Mitigation |
|-------|------------|------------|
| Handler order wrong | Low | Unit test in `config_test.go` covers this |
| WAF blocks counting as rate limit | Medium | WAF returns 403 before rate_limit handler runs |
| ACL conflicts with decisions | Low | Both use subroutes, evaluated in order |
| CrowdSec LAPI unavailable | High | Test with `crowdsec_mode=disabled` first |
| High latency under load | Medium | Benchmark and profile if > 100ms overhead |

---

## 8. Success Criteria

All test cases pass:

- [ ] **TC-1:** All features enable without errors
- [ ] **TC-2:** Handler order verified in Caddy config
- [ ] **TC-3:** WAF blocks don't consume rate limit
- [ ] **TC-4:** Decisions enforced before rate limit
- [ ] **TC-5:** 100% legitimate traffic success rate
- [ ] **TC-6:** Latency overhead < 50ms average

---

**Document Status:** Complete
**Last Updated:** 2025-12-12
