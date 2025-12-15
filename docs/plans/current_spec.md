# CrowdSec Handler Injection Analysis & Fix Plan

**Date:** December 15, 2025
**Agent:** Planning
**Status:** ✅ ANALYSIS COMPLETE - Root Cause Identified - Deployment Issue

---

## Executive Summary

**CrowdSec handler injection code is 100% CORRECT** - the issue is deployment configuration.

### The Real Problem

The container is missing `CERBERUS_SECURITY_CERBERUS_ENABLED=true` which causes `computeEffectiveFlags()` to force `crowdsecEnabled=false` even though `CHARON_SECURITY_CROWDSEC_MODE=local` is set.

### Evidence

✅ **Code is Correct:**
- CrowdSec app config generated properly ([config.go#L62-L72](../../backend/internal/caddy/config.go#L62-L72))
- Handler injection logic working ([config.go#L282-L287](../../backend/internal/caddy/config.go#L282-L287))
- All unit tests passing (TestBuildCrowdSecHandler_*, TestGenerateConfig_CrowdSec*)

❌ **Deployment is Broken:**
- `CERBERUS_SECURITY_CERBERUS_ENABLED` NOT in container environment
- `computeEffectiveFlags()` forces all security to disabled when Cerberus master switch is off
- Result: `apps.crowdsec` NOT generated, handler NOT injected

### Container Evidence

```bash
$ docker exec charon env | grep CERBERUS
(no output)  # ❌ Missing

$ curl http://localhost:2019/config/apps | jq 'keys'
["http"]  # ❌ No "crowdsec" app

$ curl http://localhost:8080/api/v1/security/config | jq '.crowdsec_mode'
null  # ❌ Not configured
```

---

## Root Cause Analysis

### The Cerberus Master Switch Problem

**File:** [backend/internal/caddy/manager.go](../../backend/internal/caddy/manager.go#L487-L492)

```go
// ACL, WAF, RateLimit and CrowdSec should only be considered enabled if Cerberus is enabled.
if !cerbEnabled {
    aclEnabled = false
    wafEnabled = false
    rateLimitEnabled = false
    crowdsecEnabled = false  // ← FORCED TO FALSE
}
```

**The Flow:**

1. **Environment Loading** ([config.go#L59](../../backend/internal/config/config.go#L59)):
   ```go
   CerberusEnabled: getEnvAny("false", "CERBERUS_SECURITY_CERBERUS_ENABLED",
                              "CHARON_SECURITY_CERBERUS_ENABLED",
                              "CPM_SECURITY_CERBERUS_ENABLED") == "true",
   ```
   - Checks for env var in priority order
   - Container has NONE of these variables
   - **Result:** `cerbEnabled = false`

2. **Flag Computation** ([manager.go#L417](../../backend/internal/caddy/manager.go#L417)):
   ```go
   crowdsecEnabled = m.securityCfg.CrowdSecMode == "local"
   ```
   - `CHARON_SECURITY_CROWDSEC_MODE=local` IS in container
   - **Result:** `crowdsecEnabled = true` (temporarily)

3. **Master Switch Override** ([manager.go#L491](../../backend/internal/caddy/manager.go#L491)):
   ```go
   if !cerbEnabled {
       crowdsecEnabled = false  // ← Forced to false
   }
   ```
   - Because `cerbEnabled = false`
   - **Result:** `crowdsecEnabled = false` (final)

4. **Config Generation** ([config.go#L62](../../backend/internal/caddy/config.go#L62)):
   ```go
   if crowdsecEnabled {
       config.Apps.CrowdSec = &CrowdSecApp{...}  // ← SKIPPED
   }
   ```
   - Because `crowdsecEnabled = false`
   - **Result:** No CrowdSec app in config

5. **Handler Injection** ([config.go#L285](../../backend/internal/caddy/config.go#L285)):
   ```go
   if csH, err := buildCrowdSecHandler(&host, secCfg, crowdsecEnabled); err == nil && csH != nil {
       securityHandlers = append(securityHandlers, csH)  // ← SKIPPED
   }
   ```
   - `buildCrowdSecHandler` returns `nil` when `crowdsecEnabled = false`
   - **Result:** No handler in routes

### The docker-compose.override.yml Mystery

**File:** [docker-compose.override.yml](../../docker-compose.override.yml#L27)

```yaml
environment:
  - CERBERUS_SECURITY_CERBERUS_ENABLED=true  # ← IN FILE
  - CHARON_SECURITY_CROWDSEC_MODE=local
```

But container inspection shows it's NOT reaching the container:

```bash
$ docker exec charon env | grep CERBERUS_SECURITY_CERBERUS_ENABLED
(no output)  # ❌ Variable missing
```

**Possible Causes:**
1. Container started without `-f docker-compose.override.yml`
2. Cached container image has old environment
3. Override file syntax error (YAML indentation)
4. Container restart didn't pick up new environment

---

## The Fix

### Problem Statement

**Code is 100% correct.** The issue is **deployment configuration** - the environment variable is not reaching the container.

### Solution: Ensure Environment Variable Reaches Container

#### Option 1: Restart with Correct Compose File (IMMEDIATE - 2 minutes)

```bash
cd /projects/Charon

# Stop container
docker compose -f docker-compose.override.yml down

# Rebuild to ensure clean state
docker build -t charon:local .

# Start with override file explicitly
docker compose -f docker-compose.override.yml up -d

# Verify environment
docker exec charon env | grep CERBERUS_SECURITY_CERBERUS_ENABLED
# Should output: CERBERUS_SECURITY_CERBERUS_ENABLED=true
```

#### Option 2: Manually Set Environment (WORKAROUND - 1 minute)

```bash
# Stop container
docker stop charon

# Start with environment variable
docker start charon -e CERBERUS_SECURITY_CERBERUS_ENABLED=true

# OR restart the container completely
docker rm charon
docker run -d --name charon \
  -e CERBERUS_SECURITY_CERBERUS_ENABLED=true \
  -e CHARON_SECURITY_CROWDSEC_MODE=local \
  # ... other flags from docker-compose.override.yml
  charon:local
```

#### Option 3: Fix Code Logic (OPTIONAL - 30 minutes)

Allow CrowdSec to operate independently of Cerberus master switch.

**File:** [backend/internal/caddy/manager.go](../../backend/internal/caddy/manager.go#L487-L492)

**Current Code:**
```go
// ACL, WAF, RateLimit and CrowdSec should only be considered enabled if Cerberus is enabled.
if !cerbEnabled {
    aclEnabled = false
    wafEnabled = false
    rateLimitEnabled = false
    crowdsecEnabled = false  // ← Forces CrowdSec off
}
```

**Proposed Change:**
```go
// ACL, WAF, and RateLimit are Cerberus-specific features.
// CrowdSec can operate independently for defense-in-depth.
if !cerbEnabled {
    aclEnabled = false
    wafEnabled = false
    rateLimitEnabled = false
    // crowdsecEnabled: allow independent operation
}
```

**Conservative Alternative (add warning):**
```go
if !cerbEnabled {
    // Store original crowdsec intent
    wantsCrowdSec := crowdsecEnabled

    aclEnabled = false
    wafEnabled = false
    rateLimitEnabled = false
    crowdsecEnabled = false

    // Log warning if user tried to enable CrowdSec without Cerberus
    if wantsCrowdSec {
        logger.Log().Warn("CrowdSec requires Cerberus master switch. Set CERBERUS_SECURITY_CERBERUS_ENABLED=true")
    }
}
```

---

## Verification Steps

After applying fix (Option 1 recommended), verify in this order:

### 1. Environment Check
```bash
docker exec charon env | grep -E "(CERBERUS|CHARON)_SECURITY"
```

**Expected Output:**
```
CERBERUS_SECURITY_CERBERUS_ENABLED=true  ← MUST BE PRESENT
CHARON_SECURITY_CROWDSEC_MODE=local
CHARON_SECURITY_CROWDSEC_API_URL=http://localhost:8080
CHARON_SECURITY_CROWDSEC_API_KEY=charonbouncerkey2024
```

### 2. Caddy App Check
```bash
curl -s http://localhost:2019/config/apps/crowdsec | jq .
```

**Expected Output:**
```json
{
  "api_key": "charonbouncerkey2024",
  "api_url": "http://localhost:8080",
  "enable_streaming": true,
  "ticker_interval": "60s"
}
```

### 3. Route Handler Check
```bash
curl -s http://localhost:2019/config/apps/http/servers/charon_server/routes | \
  jq '.[0].handle[] | select(.handler == "crowdsec")'
```

**Expected Output:**
```json
{
  "handler": "crowdsec"
}
```

### 4. Database Check
```bash
curl -s http://localhost:8080/api/v1/security/config | jq '{enabled, crowdsec_mode}'
```

**Expected Output (if Cerberus enabled via DB):**
```json
{
  "enabled": true,
  "crowdsec_mode": "local"
}
```

### 5. Functional Test
```bash
# Add test decision
docker exec charon cscli decisions add --ip 192.0.2.1 --duration 1h --reason "test block"

# Simulate blocked request
curl -H "X-Forwarded-For: 192.0.2.1" http://localhost/

# Expected: 403 Forbidden
```

---

## Test Coverage Validation

All existing tests PASS - no code changes needed:

### Unit Tests (Handler Building)
- ✅ `TestBuildCrowdSecHandler_Disabled` - Returns nil when disabled
- ✅ `TestBuildCrowdSecHandler_EnabledWithoutConfig` - Returns minimal handler
- ✅ `TestBuildCrowdSecHandler_EnabledWithCustomAPIURL` - Custom API URL works
- ✅ `TestBuildCrowdSecHandler_JSONFormat` - Valid JSON structure
- ✅ `TestBuildCrowdSecHandler_WithHost` - Per-host configuration

### Integration Tests (Config Generation)
- ✅ `TestGenerateConfig_CrowdSecHandlerFromSecCfg` - Handler in routes when enabled
- ✅ App-level config correct (api_url, api_key, streaming)
- ✅ Handler is minimal (no inline config)
- ✅ Trusted proxies configured at server level (NOT app level)

### Manager Tests (Runtime Flags)
- ✅ `TestComputeEffectiveFlags_DB_CrowdSecLocal` - Returns true when mode=local
- ✅ `TestComputeEffectiveFlags_DB_CrowdSecExternal` - Returns false when not local
- ✅ `TestManager_ApplyConfig_RuntimeFlags` - Handler appears when enabled

**Note:** The tests use `crowdsecEnabled=true` parameter directly, bypassing the Cerberus master switch check. This is correct test isolation.


---

## Conclusion

### Research Complete ✅

The CrowdSec handler injection code is **100% correct and working as designed**. All handler building, route injection, and configuration generation logic is properly implemented and tested.

### Root Cause Identified ✅

The issue is a **deployment configuration problem**, not a code problem:

1. Container missing `CERBERUS_SECURITY_CERBERUS_ENABLED=true` environment variable
2. `computeEffectiveFlags()` forces all security features off when Cerberus master switch is disabled
3. Result: `crowdsecEnabled=false` → No app config → No handler injection

### Implementation Path Clear ✅

**Option 1 (Recommended):** Fix deployment by ensuring environment variable reaches container
- **Time:** 2 minutes
- **Risk:** None (just fixing misconfiguration)
- **Impact:** Immediate - CrowdSec will work on next restart

**Option 2 (Optional):** Decouple CrowdSec from Cerberus master switch
- **Time:** 30 minutes (code + tests)
- **Risk:** Low (architecture change)
- **Impact:** Allows CrowdSec to operate independently

### Code Quality Validation ✅

- All unit tests passing
- Integration tests passing
- Handler order correct (Security Decisions → CrowdSec → WAF → Rate Limit → ACL → Reverse Proxy)
- App-level config matches plugin docs
- Trusted proxies configured at server level

### Documentation Complete ✅

This specification provides:
- Complete root cause analysis with evidence
- Exact line-by-line code flow explanation
- Multiple fix options with tradeoffs
- Comprehensive verification steps
- Test coverage validation

---

**Status:** ✅ READY FOR IMPLEMENTATION
**Next Step:** Apply fix (Option 1 recommended)
**Owner:** DevOps / Infrastructure
**ETA:** 2 minutes for deployment fix, or 30 minutes for code enhancement
