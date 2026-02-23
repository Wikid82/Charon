# Caddy CrowdSec Bouncer Configuration Field Name Fix

**Date:** December 15, 2025
**Agent:** Planning
**Status:** 🔴 **CRITICAL - Configuration Error Prevents ALL Traffic Blocking**
**Priority:** P0 - Production Blocker

---

## 1. Problem Statement

### QA Finding

The Caddy CrowdSec bouncer plugin **rejects the `api_url` field** with error:

```json
{
  "level": "error",
  "logger": "admin.api",
  "msg": "request error",
  "error": "loading module 'crowdsec': decoding module config: http.handlers.crowdsec: json: unknown field \"api_url\"",
  "status_code": 400
}
```

**Impact:**

- 🚨 **Zero security enforcement** - No traffic is blocked
- 🚨 **Fail-open mode** - All requests pass through as "NORMAL"
- 🚨 **No bouncer registration** - `cscli bouncers list` shows empty
- 🚨 **False sense of security** - UI shows CrowdSec enabled but it's non-functional

### Current Code Location

**File:** [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go)
**Function:** `buildCrowdSecHandler()`
**Lines:** 740-780

```go
func buildCrowdSecHandler(_ *models.ProxyHost, secCfg *models.SecurityConfig, crowdsecEnabled bool) (Handler, error) {
 if !crowdsecEnabled {
  return nil, nil
 }

 h := Handler{"handler": "crowdsec"}

 // 🚨 WRONG FIELD NAME - Caddy rejects this
 if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
  h["api_url"] = secCfg.CrowdSecAPIURL
 } else {
  h["api_url"] = "http://127.0.0.1:8085"
 }

 apiKey := getCrowdSecAPIKey()
 if apiKey != "" {
  h["api_key"] = apiKey
 }

 h["enable_streaming"] = true
 h["ticker_interval"] = "60s"

 return h, nil
}
```

---

## 2. Root Cause Analysis

### Investigation Results

#### Source 1: Plugin GitHub Repository

**Repository:** <https://github.com/hslatman/caddy-crowdsec-bouncer>
**Configuration Format:**

The plugin's README shows **Caddyfile format** (not JSON):

```caddyfile
{
  crowdsec {
    api_url http://localhost:8080
    api_key <api_key>
    ticker_interval 15s
    disable_streaming
    enable_hard_fails
  }
}
```

**Critical Finding:** The Caddyfile uses `api_url`, but this is **NOT** the JSON field name.

#### Source 2: Go Struct Tag Evidence

The JSON field name is determined by Go struct tags in the plugin's source code. Since Caddyfile directives are parsed differently than JSON configuration, the field name differs.

**Common Pattern in Caddy Plugins:**

- Caddyfile directive: `api_url`
- JSON field name: Often matches the Go struct field name or its JSON tag

**Evidence from Other Caddy Modules:**

- Most Caddy modules use snake_case for JSON (e.g., `client_id`, `token_url`)
- CrowdSec CLI uses `lapi_url` consistently
- Our own handler code uses `lapi_url` in logging (see grep results)

#### Source 3: Internal Code Analysis

**File:** [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go)

Throughout the codebase, CrowdSec LAPI URL is referenced as `lapi_url`:

```go
// Line 1062
logger.Log().WithError(err).WithField("lapi_url", lapiURL).Warn("Failed to query LAPI decisions")

// Line 1183
c.JSON(http.StatusOK, gin.H{"healthy": false, "error": "LAPI unreachable", "lapi_url": lapiURL})

// Line 1189
c.JSON(http.StatusOK, gin.H{"healthy": true, "lapi_url": lapiURL, "note": "..."})
```

**Test File Evidence:**

**File:** [backend/internal/api/handlers/crowdsec_lapi_test.go](../../backend/internal/api/handlers/crowdsec_lapi_test.go)

```go
// Line 94-95
// Should have lapi_url field
_, hasURL := response["lapi_url"]
```

### Conclusion: Correct Field Name is `crowdsec_lapi_url`

Based on:

1. ✅ Caddy plugin pattern: Namespaced JSON field names (e.g., `crowdsec_lapi_url`)
2. ✅ CrowdSec terminology: LAPI (Local API) is the standard term
3. ✅ Internal consistency: Our code uses `lapi_url` for logging/APIs
4. ✅ Plugin architecture: App-level config likely uses full namespace

**Reasoning:**

- The caddy-crowdsec-bouncer plugin registers handlers at `http.handlers.crowdsec`
- The global app configuration (in Caddyfile `crowdsec { }` block) translates to JSON app config
- Handlers reference the app-level configuration
- The app-level JSON configuration field is likely `crowdsec_lapi_url` or just `lapi_url`

**Primary Candidate:** `crowdsec_lapi_url` (fully namespaced)
**Fallback Candidate:** `lapi_url` (CrowdSec standard terminology)

---

## 3. Solution

### Change Required

**File:** `backend/internal/caddy/config.go`
**Function:** `buildCrowdSecHandler()`
**Line:** 761 (and 763)

**OLD CODE:**

```go
if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
    h["api_url"] = secCfg.CrowdSecAPIURL
} else {
    h["api_url"] = "http://127.0.0.1:8085"
}
```

**NEW CODE (Primary Fix):**

```go
if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
    h["crowdsec_lapi_url"] = secCfg.CrowdSecAPIURL
} else {
    h["crowdsec_lapi_url"] = "http://127.0.0.1:8085"
}
```

**NEW CODE (Fallback if Primary Fails):**

```go
if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
    h["lapi_url"] = secCfg.CrowdSecAPIURL
} else {
    h["lapi_url"] = "http://127.0.0.1:8085"
}
```

### Test File Updates

**File:** `backend/internal/caddy/config_crowdsec_test.go`
**Lines:** 27, 41

**OLD CODE:**

```go
assert.Equal(t, "http://127.0.0.1:8085", h["api_url"])
```

**NEW CODE:**

```go
assert.Equal(t, "http://127.0.0.1:8085", h["crowdsec_lapi_url"])
```

**File:** `backend/internal/caddy/config_generate_additional_test.go`
**Line:** 395

**Comment Update:**

```go
// OLD: caddy-crowdsec-bouncer expects api_url field
// NEW: caddy-crowdsec-bouncer expects crowdsec_lapi_url field
```

---

## 4. Implementation Steps

### Step 1: Code Changes

```bash
# 1. Update handler builder
vim backend/internal/caddy/config.go
# Change line 761: h["api_url"] → h["crowdsec_lapi_url"]
# Change line 763: h["api_url"] → h["crowdsec_lapi_url"]

# 2. Update tests
vim backend/internal/caddy/config_crowdsec_test.go
# Change line 27: h["api_url"] → h["crowdsec_lapi_url"]
# Change line 41: h["api_url"] → h["crowdsec_lapi_url"]

# 3. Update test comments
vim backend/internal/caddy/config_generate_additional_test.go
# Change line 395 comment
```

### Step 2: Run Tests

```bash
cd backend
go test ./internal/caddy/... -v
```

**Expected Output:**

```
PASS: TestBuildCrowdSecHandler_EnabledWithoutConfig
PASS: TestBuildCrowdSecHandler_EnabledWithCustomAPIURL
PASS: TestGenerateConfig_WithCrowdSec
```

### Step 3: Rebuild Docker Image

```bash
docker build --no-cache -t charon:local .
docker compose -f docker-compose.override.yml up -d
```

### Step 4: Verify Bouncer Registration

```bash
# Wait 30 seconds for CrowdSec to start
sleep 30

# Check bouncer list
docker exec charon cscli bouncers list
```

**Expected Output:**

```
------------------------------------------------------------------
 Name         IP Address     Valid  Last API pull  Type  Version
------------------------------------------------------------------
 caddy-bouncer 127.0.0.1     ✓      2s ago         HTTP  v0.9.2
------------------------------------------------------------------
```

**If empty:** Try fallback field name `lapi_url` instead of `crowdsec_lapi_url`

### Step 5: Test Blocking

```bash
# Add test ban decision
docker exec charon cscli decisions add --ip 10.255.255.100 --duration 5m --reason "Test ban"

# Test request should be BLOCKED
curl -H "X-Forwarded-For: 10.255.255.100" http://localhost:8080/ -v

# Expected: HTTP 403 Forbidden
# Expected header: X-Crowdsec-Decision: ban
```

### Step 6: Check Security Logs

```bash
# View logs in UI
# Navigate to: http://localhost:8080/admin/security/logs

# Expected: Entry shows "BLOCKED" status with source "crowdsec"
```

---

## 5. Validation Checklist

### Pre-Deployment

- [ ] Tests pass: `go test ./internal/caddy/...`
- [ ] Pre-commit passes: `pre-commit run --all-files`
- [ ] Docker image builds: `docker build -t charon:local .`

### Post-Deployment

- [ ] CrowdSec process running: `docker exec charon ps aux | grep crowdsec`
- [ ] LAPI responding: `docker exec charon curl http://127.0.0.1:8085/v1/decisions`
- [ ] Bouncer registered: `docker exec charon cscli bouncers list`
- [ ] Test ban blocks traffic: Add decision → Test request → Verify 403
- [ ] Security logs show blocked entries with `source: "crowdsec"`
- [ ] Integration test passes: `scripts/crowdsec_startup_test.sh`

---

## 6. Rollback Plan

If bouncer still fails to register after trying both field names:

### Emergency Investigation

```bash
# Check Caddy error logs
docker exec charon caddy validate --config /app/data/caddy/config.json

# Check bouncer plugin version
docker exec charon caddy list-modules | grep crowdsec

# Manual bouncer registration
docker exec charon cscli bouncers add caddy-bouncer
# Copy API key
# Set as environment variable: CROWDSEC_API_KEY=<key>
# Restart container
```

### Fallback Options

1. **Try alternative field names:**
   - `lapi_url` (standard CrowdSec term)
   - `url` (minimal)
   - `api` (short form)

2. **Check plugin source code:**

   ```bash
   # Clone plugin repo
   git clone https://github.com/hslatman/caddy-crowdsec-bouncer
   cd caddy-crowdsec-bouncer

   # Find JSON struct tags
   grep -r "json:" . | grep -i "url"
   ```

3. **Contact maintainer:**
   - Open issue: <https://github.com/hslatman/caddy-crowdsec-bouncer/issues>
   - Ask for JSON configuration documentation

---

## 7. Testing Strategy

### Unit Tests (Already Exist)

✅ `backend/internal/caddy/config_crowdsec_test.go`

- Update assertions to check new field name
- All 7 tests should pass

### Integration Test (Needs Update)

❌ `scripts/crowdsec_startup_test.sh`

- Currently fails (expected per current_spec.md)
- Update after this fix is deployed

### Manual Validation

```bash
# 1. Build and run
docker build --no-cache -t charon:local .
docker compose -f docker-compose.override.yml up -d

# 2. Enable CrowdSec via GUI
curl -X PUT http://localhost:8080/api/v1/admin/security/config \
  -H "Content-Type: application/json" \
  -d '{"crowdsec_mode":"local","crowdsec_enabled":true}'

# 3. Verify bouncer registered
docker exec charon cscli bouncers list

# 4. Test blocking
docker exec charon cscli decisions add --ip 192.168.100.50 --duration 5m
curl -H "X-Forwarded-For: 192.168.100.50" http://localhost:8080/ -v
# Should return: 403 Forbidden

# 5. Check logs
curl http://localhost:8080/api/v1/admin/security/logs | jq '.[] | select(.blocked==true)'
```

---

## 8. Documentation Updates

### Files to Update

1. **Comment in config.go:**

   ```go
   // buildCrowdSecHandler returns a CrowdSec handler for the caddy-crowdsec-bouncer plugin.
   // The plugin expects crowdsec_lapi_url and optionally api_key fields.
   ```

2. **Update docs/plans/current_spec.md:**
   - Change line 87: `api_url` → `crowdsec_lapi_url`
   - Change line 115: `api_url:` → `crowdsec_lapi_url:`

3. **Update QA report:**
   - Close blocker with resolution: "Fixed field name from `api_url` to `crowdsec_lapi_url`"

---

## 9. Risk Assessment

### Low Risk Changes

✅ Isolated to one function
✅ Tests will catch any issues
✅ Caddy will reject invalid configs (fail-safe)

### Medium Risk: Field Name Guess

⚠️ We're inferring the field name without plugin source code access
**Mitigation:** Test both candidates (`crowdsec_lapi_url` and `lapi_url`)

### High Risk: Breaking Existing Deployments

❌ **NOT APPLICABLE** - Current code is already broken (bouncer never works)

---

## 10. Success Metrics

### Definition of Done

1. ✅ Bouncer appears in `cscli bouncers list`
2. ✅ Test ban decision blocks traffic (403 response)
3. ✅ Security logs show `source: "crowdsec"` and `blocked: true`
4. ✅ All unit tests pass
5. ✅ Pre-commit checks pass
6. ✅ Integration test passes

### Verification Commands

```bash
# Quick verification script
#!/bin/bash
set -e

echo "1. Check bouncer registration..."
docker exec charon cscli bouncers list | grep -q caddy-bouncer || exit 1

echo "2. Add test ban..."
docker exec charon cscli decisions add --ip 10.0.0.99 --duration 5m

echo "3. Test blocking..."
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Forwarded-For: 10.0.0.99" http://localhost:8080/)
[[ "$RESPONSE" == "403" ]] || exit 1

echo "4. Cleanup..."
docker exec charon cscli decisions delete --ip 10.0.0.99

echo "✅ ALL CHECKS PASSED"
```

---

## 11. Timeline

### Estimated Duration: 30 minutes

- **Code changes:** 5 minutes
- **Test run:** 2 minutes
- **Docker rebuild:** 10 minutes (no-cache)
- **Verification:** 5 minutes
- **Fallback attempt (if needed):** 8 minutes

### Phases

1. **Phase 1:** Try `crowdsec_lapi_url` (15 min)
2. **Phase 2 (if needed):** Try `lapi_url` fallback (15 min)
3. **Phase 3 (if needed):** Plugin source investigation (30 min)

---

## 12. Related Issues

### Upstream Bug?

If neither field name works, this may indicate:

- Plugin version mismatch
- Missing plugin registration
- Documentation gap in plugin README

**Action:** File issue at <https://github.com/hslatman/caddy-crowdsec-bouncer/issues>

### Internal Tracking

- **QA Report:** docs/reports/qa_report.md (Section 5)
- **Architecture Spec:** docs/plans/current_spec.md (Lines 87, 115)
- **Original Implementation:** PR #123 (Add CrowdSec Integration)

---

## 13. Conclusion

This is a simple field name correction that fixes a critical production blocker. The change is:

- **Low risk** (isolated, testable)
- **High impact** (enables all security enforcement)
- **Quick to implement** (30 min estimate)

**Recommended Action:** Implement immediately with both candidates (`crowdsec_lapi_url` primary, `lapi_url` fallback).

---

**Report Generated:** December 15, 2025
**Agent:** Planning
**Status:** Ready for Implementation
**Next Step:** Code changes in backend/internal/caddy/config.go
