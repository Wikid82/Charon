# Charon Rate Limiter Testing Plan

**Version:** 1.0
**Date:** 2025-12-12
**Status:** 🔵 READY FOR TESTING

---

## 1. Rate Limiter Implementation Summary

### 1.1 Algorithm

The Charon rate limiter uses the **`mholt/caddy-ratelimit`** Caddy module, which implements a **sliding window algorithm** with a **ring buffer** implementation.

**Key characteristics:**

- **Sliding window**: Looks back `<window>` duration and checks if `<max_events>` events have occurred
- **Ring buffer**: Memory-efficient `O(Kn)` where K = max events, n = number of rate limiters
- **Automatic Retry-After**: The module automatically sets `Retry-After` header on 429 responses
- **Scoped by client IP**: Rate limiting uses `{http.request.remote.host}` as the key

### 1.2 Configuration Model

**Source:** [backend/internal/models/security_config.go](../../backend/internal/models/security_config.go)

| Field | Type | JSON Key | Description |
|-------|------|----------|-------------|
| `RateLimitEnable` | `bool` | `rate_limit_enable` | Master toggle for rate limiting |
| `RateLimitRequests` | `int` | `rate_limit_requests` | Max requests allowed in window |
| `RateLimitWindowSec` | `int` | `rate_limit_window_sec` | Time window in seconds |
| `RateLimitBurst` | `int` | `rate_limit_burst` | Burst allowance (default: 20% of requests) |
| `RateLimitBypassList` | `string` | `rate_limit_bypass_list` | Comma-separated CIDRs to bypass |

### 1.3 Default Values & Presets

**Source:** [backend/internal/api/handlers/security_ratelimit_test.go](../../backend/internal/api/handlers/security_ratelimit_test.go)

| Preset ID | Name | Requests | Window (sec) | Burst | Use Case |
|-----------|------|----------|--------------|-------|----------|
| `standard` | Standard Web | 100 | 60 | 20 | General web traffic |
| `api` | API | (varies) | (varies) | (varies) | API endpoints |
| `login` | Login Protection | 5 | 300 | 2 | Authentication endpoints |
| `relaxed` | Relaxed | (varies) | (varies) | (varies) | Low-risk endpoints |

### 1.4 Client Identification

**Key:** `{http.request.remote.host}`

Rate limiting is scoped per-client using the remote host IP address. Each unique client IP gets its own rate limit counter.

### 1.5 Headers Returned

The `caddy-ratelimit` module returns:

- **HTTP 429** response when limit exceeded
- **`Retry-After`** header indicating seconds until the client can retry

> **Note:** The module does NOT return `X-RateLimit-Limit`, `X-RateLimit-Remaining` headers by default. These are not part of the `caddy-ratelimit` module's standard output.

### 1.6 Enabling Rate Limiting

Rate limiting requires **two conditions**:

1. **Cerberus must be enabled:**
   - Set `feature.cerberus.enabled` = `true` in Settings
   - OR set env `CERBERUS_SECURITY_CERBERUS_ENABLED=true`

2. **Rate Limit Mode must be enabled:**
   - Set env `CERBERUS_SECURITY_RATELIMIT_MODE=enabled`
   - (There is no runtime DB toggle for rate limit mode currently)

**Source:** [backend/internal/caddy/manager.go#L412-465](../../backend/internal/caddy/manager.go)

### 1.7 Generated Caddy JSON Structure

**Source:** [backend/internal/caddy/config.go#L990-1065](../../backend/internal/caddy/config.go)

```json
{
  "handler": "rate_limit",
  "rate_limits": {
    "static": {
      "key": "{http.request.remote.host}",
      "window": "60s",
      "max_events": 100,
      "burst": 20
    }
  }
}
```

With bypass list, wraps in subroute:

```json
{
  "handler": "subroute",
  "routes": [
    {
      "match": [{"remote_ip": {"ranges": ["10.0.0.0/8", "192.168.1.0/24"]}}],
      "handle": []
    },
    {
      "handle": [{"handler": "rate_limit", ...}]
    }
  ]
}
```

---

## 2. Existing Test Coverage

### 2.1 Unit Tests

| Test File | Tests | Coverage |
|-----------|-------|----------|
| [config_test.go](../../backend/internal/caddy/config_test.go) | `TestBuildRateLimitHandler_*` (8 tests) | Handler building, burst defaults, bypass list parsing |
| [security_ratelimit_test.go](../../backend/internal/api/handlers/security_ratelimit_test.go) | `TestSecurityHandler_GetRateLimitPresets*` (3 tests) | Preset endpoint |

### 2.2 Unit Test Functions

From [config_test.go](../../backend/internal/caddy/config_test.go):

- `TestBuildRateLimitHandler_Disabled` - nil config returns nil handler
- `TestBuildRateLimitHandler_InvalidValues` - zero/negative values return nil
- `TestBuildRateLimitHandler_ValidConfig` - correct caddy-ratelimit format
- `TestBuildRateLimitHandler_JSONFormat` - valid JSON output
- `TestGenerateConfig_WithRateLimiting` - handler included in route
- `TestBuildRateLimitHandler_UsesBurst` - configured burst is used
- `TestBuildRateLimitHandler_DefaultBurst` - default 20% calculation
- `TestBuildRateLimitHandler_BypassList` - subroute with bypass CIDRs
- `TestBuildRateLimitHandler_BypassList_PlainIPs` - IP to CIDR conversion
- `TestBuildRateLimitHandler_BypassList_InvalidEntries` - invalid entries ignored
- `TestBuildRateLimitHandler_BypassList_Empty` - empty list = plain handler
- `TestBuildRateLimitHandler_BypassList_AllInvalid` - all invalid = plain handler

### 2.3 Missing Integration Tests

There is **NO existing integration test** for rate limiting. The pattern to follow is:

- [scripts/coraza_integration.sh](../../scripts/coraza_integration.sh) - WAF integration test
- [backend/integration/coraza_integration_test.go](../../backend/integration/coraza_integration_test.go) - Go test wrapper

---

## 3. Testing Plan

### 3.1 Prerequisites

1. **Docker running** with Charon image built
2. **Cerberus enabled**:

   ```bash
   export CERBERUS_SECURITY_CERBERUS_ENABLED=true
   export CERBERUS_SECURITY_RATELIMIT_MODE=enabled
   ```

3. **Proxy host configured** that can receive test requests

### 3.2 Test Setup Commands

```bash
# Build and run Charon with rate limiting enabled
docker build -t charon:local .
docker run -d --name charon-ratelimit-test \
  -p 8080:8080 \
  -p 80:80 \
  -e CHARON_ENV=development \
  -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
  -e CERBERUS_SECURITY_RATELIMIT_MODE=enabled \
  charon:local

# Wait for startup
sleep 5

# Create temp cookie file
TMP_COOKIE=$(mktemp)

# Register and login
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123","name":"Tester"}' \
  http://localhost:8080/api/v1/auth/register

curl -s -X POST -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}' \
  -c ${TMP_COOKIE} \
  http://localhost:8080/api/v1/auth/login

# Create a simple proxy host for testing
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{
    "domain_names": "ratelimit.test.local",
    "forward_scheme": "http",
    "forward_host": "httpbin.org",
    "forward_port": 80,
    "enabled": true
  }' \
  http://localhost:8080/api/v1/proxy-hosts
```

### 3.3 Configure Rate Limit via API

```bash
# Set aggressive rate limit for testing (3 requests per 10 seconds)
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{
    "name": "default",
    "enabled": true,
    "rate_limit_enable": true,
    "rate_limit_requests": 3,
    "rate_limit_window_sec": 10,
    "rate_limit_burst": 1,
    "rate_limit_bypass_list": ""
  }' \
  http://localhost:8080/api/v1/security/config

# Wait for Caddy to reload
sleep 5
```

---

## 4. Test Cases & Curl Commands

### 4.1 Test Case: Basic Rate Limit Enforcement

**Objective:** Verify requests are blocked after exceeding limit

```bash
echo "=== TEST: Basic Rate Limit Enforcement ==="
echo "Config: 3 requests / 10 seconds"

# Make 5 rapid requests, expect 3 to succeed, 2 to fail
for i in 1 2 3 4 5; do
  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ratelimit.test.local" \
    http://localhost/get)
  echo "Request $i: HTTP $RESPONSE"
done

# Expected output:
# Request 1: HTTP 200
# Request 2: HTTP 200
# Request 3: HTTP 200
# Request 4: HTTP 429
# Request 5: HTTP 429
```

### 4.2 Test Case: Retry-After Header

**Objective:** Verify 429 response includes Retry-After header

```bash
echo "=== TEST: Retry-After Header ==="

# Exhaust rate limit
for i in 1 2 3; do
  curl -s -o /dev/null -H "Host: ratelimit.test.local" http://localhost/get
done

# Check 429 response headers
HEADERS=$(curl -s -D - -o /dev/null \
  -H "Host: ratelimit.test.local" \
  http://localhost/get)

echo "$HEADERS" | grep -i "retry-after"
# Expected: Retry-After: <seconds>
```

### 4.3 Test Case: Window Reset

**Objective:** Verify rate limit resets after window expires

```bash
echo "=== TEST: Window Reset ==="

# Exhaust rate limit
for i in 1 2 3; do
  curl -s -o /dev/null -H "Host: ratelimit.test.local" http://localhost/get
done

# Verify blocked
BLOCKED=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: ratelimit.test.local" http://localhost/get)
echo "Immediately after: HTTP $BLOCKED (expect 429)"

# Wait for window to reset (10 sec + buffer)
echo "Waiting 12 seconds for window reset..."
sleep 12

# Should succeed again
RESET=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: ratelimit.test.local" http://localhost/get)
echo "After window reset: HTTP $RESET (expect 200)"
```

### 4.4 Test Case: Bypass List

**Objective:** Verify bypass IPs are not rate limited

```bash
echo "=== TEST: Bypass List ==="

# First, configure bypass list with localhost
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{
    "name": "default",
    "enabled": true,
    "rate_limit_enable": true,
    "rate_limit_requests": 3,
    "rate_limit_window_sec": 10,
    "rate_limit_burst": 1,
    "rate_limit_bypass_list": "127.0.0.1/32, 172.17.0.0/16"
  }' \
  http://localhost:8080/api/v1/security/config

sleep 5

# Make 10 requests - all should succeed if IP is bypassed
for i in $(seq 1 10); do
  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ratelimit.test.local" \
    http://localhost/get)
  echo "Request $i: HTTP $RESPONSE (expect 200)"
done
```

### 4.5 Test Case: Different Clients Isolated

**Objective:** Verify rate limits are per-client, not global

```bash
echo "=== TEST: Client Isolation ==="

# This test requires two different source IPs
# In Docker, you can test by:
# 1. Making requests from host (one IP)
# 2. Making requests from inside another container (different IP)

# From container A (simulate with X-Forwarded-For if configured)
docker run --rm --network host curlimages/curl:latest \
  curl -s -o /dev/null -w "%{http_code}" \
  -H "Host: ratelimit.test.local" \
  http://localhost/get

# Note: True client isolation testing requires actual different IPs
# This is better suited for an integration test script
```

### 4.6 Test Case: Burst Handling

**Objective:** Verify burst allowance works correctly

```bash
echo "=== TEST: Burst Handling ==="

# Configure with burst > 1
curl -s -X POST -H "Content-Type: application/json" \
  -b ${TMP_COOKIE} \
  -d '{
    "name": "default",
    "enabled": true,
    "rate_limit_enable": true,
    "rate_limit_requests": 10,
    "rate_limit_window_sec": 60,
    "rate_limit_burst": 5,
    "rate_limit_bypass_list": ""
  }' \
  http://localhost:8080/api/v1/security/config

sleep 5

# Make 5 rapid requests (burst should allow)
for i in 1 2 3 4 5; do
  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Host: ratelimit.test.local" \
    http://localhost/get)
  echo "Request $i: HTTP $RESPONSE"
done
# All should be 200 due to burst allowance
```

### 4.7 Test Case: Verify Caddy Config Contains Rate Limit

**Objective:** Confirm rate_limit handler is in running Caddy config

```bash
echo "=== TEST: Caddy Config Verification ==="

# Query Caddy admin API for current config
CONFIG=$(curl -s http://localhost:2019/config)

# Check for rate_limit handler
echo "$CONFIG" | grep -o '"handler":"rate_limit"' | head -1
# Expected: "handler":"rate_limit"

# Check for correct key
echo "$CONFIG" | grep -o '"key":"{http.request.remote.host}"' | head -1
# Expected: "key":"{http.request.remote.host}"

# Full handler inspection
echo "$CONFIG" | python3 -c "
import sys, json
config = json.load(sys.stdin)
servers = config.get('apps', {}).get('http', {}).get('servers', {})
for name, server in servers.items():
    for route in server.get('routes', []):
        for handler in route.get('handle', []):
            if handler.get('handler') == 'rate_limit':
                print(json.dumps(handler, indent=2))
" 2>/dev/null || echo "Rate limit handler found (use jq for pretty print)"
```

---

## 5. Verification Checklist

### 5.1 Unit Test Checklist

- [ ] `go test ./internal/caddy/... -v -run TestBuildRateLimitHandler` passes
- [ ] `go test ./internal/api/handlers/... -v -run TestSecurityHandler_GetRateLimitPresets` passes

### 5.2 Configuration Checklist

- [ ] `SecurityConfig.RateLimitRequests` is stored and retrieved correctly
- [ ] `SecurityConfig.RateLimitWindowSec` is stored and retrieved correctly
- [ ] `SecurityConfig.RateLimitBurst` defaults to 20% when zero
- [ ] `SecurityConfig.RateLimitBypassList` parses comma-separated CIDRs
- [ ] Plain IPs in bypass list are converted to /32 CIDRs
- [ ] Invalid entries in bypass list are silently ignored

### 5.3 Runtime Checklist

- [ ] Rate limiting only active when `CERBERUS_SECURITY_RATELIMIT_MODE=enabled`
- [ ] Rate limiting only active when `CERBERUS_SECURITY_CERBERUS_ENABLED=true`
- [ ] Caddy config includes `rate_limit` handler when enabled
- [ ] Handler uses `{http.request.remote.host}` as key

### 5.4 Response Behavior Checklist

- [ ] HTTP 429 returned when limit exceeded
- [ ] `Retry-After` header present on 429 responses
- [ ] Rate limit resets after window expires
- [ ] Bypassed IPs not rate limited

### 5.5 Edge Case Checklist

- [ ] Zero/negative requests value = no rate limiting
- [ ] Zero/negative window value = no rate limiting
- [ ] Empty bypass list = no subroute wrapper
- [ ] All-invalid bypass list = no subroute wrapper

---

## 6. Existing Test Files Reference

| File | Purpose |
|------|---------|
| [backend/internal/caddy/config_test.go](../../backend/internal/caddy/config_test.go) | Unit tests for `buildRateLimitHandler` |
| [backend/internal/caddy/config_generate_additional_test.go](../../backend/internal/caddy/config_generate_additional_test.go) | `GenerateConfig` with rate limiting |
| [backend/internal/api/handlers/security_ratelimit_test.go](../../backend/internal/api/handlers/security_ratelimit_test.go) | Preset API endpoint tests |
| [scripts/coraza_integration.sh](../../scripts/coraza_integration.sh) | Template for integration script |
| [backend/integration/coraza_integration_test.go](../../backend/integration/coraza_integration_test.go) | Template for Go integration test wrapper |

---

## 7. Recommended Next Steps

1. **Run existing unit tests:**

   ```bash
   cd backend && go test ./internal/caddy/... -v -run TestBuildRateLimitHandler
   cd backend && go test ./internal/api/handlers/... -v -run TestSecurityHandler_GetRateLimitPresets
   ```

2. **Create integration test script:** `scripts/rate_limit_integration.sh` following the `coraza_integration.sh` pattern

3. **Create Go test wrapper:** `backend/integration/rate_limit_integration_test.go`

4. **Add VS Code task:** Update `.vscode/tasks.json` with rate limit integration task

5. **Manual verification:** Run curl commands from Section 4 against a live Charon instance

---

## 8. Known Limitations

1. **No `X-RateLimit-*` headers:** The `caddy-ratelimit` module only provides `Retry-After`, not standard `X-RateLimit-Limit`/`X-RateLimit-Remaining` headers.

2. **No runtime toggle:** Rate limit mode is set via environment variable only; there's no DB-backed runtime toggle like WAF/ACL have.

3. **Burst behavior:** The `caddy-ratelimit` module documentation notes burst is not a direct parameter; Charon passes it but actual behavior depends on the module's implementation.

4. **Distributed mode not configured:** The `caddy-ratelimit` module supports distributed rate limiting across a cluster, but Charon doesn't currently expose this configuration.

---

**Document Status:** Complete
**Last Updated:** 2025-12-12
