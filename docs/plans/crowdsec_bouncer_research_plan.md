# Caddy CrowdSec Bouncer JSON Configuration - Complete Research & Implementation Plan

**Date:** December 15, 2025
**Agent:** Planning
**Status:** 🔴 **CRITICAL - Unknown Plugin Configuration Schema**
**Priority:** P0 - Production Blocker
**Estimated Resolution Time:** 1-4 hours

---

## Executive Summary

**Critical Blocker:** The caddy-crowdsec-bouncer plugin rejects ALL field name variants tested in JSON configuration, completely preventing traffic blocking functionality.

**Current Status:**

- ✅ CrowdSec LAPI running correctly (port 8085) ✅ Bouncer API key generated
- ❌ **ZERO bouncers registered** (`cscli bouncers list` empty)
- ❌ **Plugin rejects config:** "json: unknown field" errors for `api_url`, `lapi_url`, `crowdsec_lapi_url`
- ❌ **No traffic blocking:** All requests pass through as "NORMAL"
- ❌ **Production impact:** Complete security enforcement failure

**Root Cause:** Plugin documentation only provides Caddyfile format, JSON schema is undocumented.

---

## 1. Research Findings & Evidence

### 1.1 Evidence from Working Plugins (WAF/Coraza)

**File:** `backend/internal/caddy/config.go` (Lines 846-930)

The WAF (Coraza) plugin successfully uses **inline handler configuration**:

```go
func buildWAFHandler(...) (Handler, error) {
    directives := buildWAFDirectives(secCfg, selected, rulesetPaths)
    if directives == "" {
        return nil, nil
    }
    h := Handler{
        "handler":    "waf",
        "directives": directives,
    }
    return h, nil
}
```

**Generated JSON (verified working):**

```json
{
  "handle": [
    {
      "handler": "waf",
      "directives": "SecRuleEngine On\nInclude /path/to/rules.conf"
    }
  ]
}
```

**Key Insight:** Other Caddy plugins (WAF, rate_limit, geoip) work with inline handler config in the routes array, suggesting CrowdSec SHOULD support this pattern too.

---

### 1.2 Evidence from Dockerfile Build

**File:** `Dockerfile` (Lines 123-128)

```dockerfile
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build v${CADDY_VERSION} \
    --with github.com/greenpau/caddy-security \
    --with github.com/corazawaf/coraza-caddy/v2 \
    --with github.com/hslatman/caddy-crowdsec-bouncer \
    --with github.com/zhangjiayin/caddy-geoip2 \
    --with github.com/mholt/caddy-ratelimit
```

**Critical Observations:**

1. **No version pinning:** Building from `main` branch (unstable)
2. **Plugin source:** `github.com/hslatman/caddy-crowdsec-bouncer`
3. **Build method:** xcaddy (builds custom Caddy with plugins)
4. **Potential issue:** Latest commit might have breaking changes

**Action:** Check plugin GitHub for recent breaking changes in JSON API.

---

### 1.3 Evidence from Caddyfile Documentation

**Source:** Plugin README (<https://github.com/hslatman/caddy-crowdsec-bouncer>)

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

**Critical Observations:**

1. This is **app-level configuration** (inside global options block `{ }`)
2. **NOT handler-level** (not inside route handlers)
3. **Caddyfile directive names ≠ JSON field names** (common Caddy pattern)

**Primary Hypothesis:** CrowdSec requires app-level configuration structure:

```json
{
  "apps": {
    "http": {...},
    "crowdsec": {
      "api_url": "http://127.0.0.1:8085",
      "api_key": "..."
    }
  }
}
```

Handler becomes minimal reference: `{"handler": "crowdsec"}`

---

### 1.4 Evidence from Current Type Definitions

**File:** `backend/internal/caddy/types.go` (Lines 57-60)

```go
// Apps contains all Caddy app modules.
type Apps struct {
 HTTP *HTTPApp `json:"http,omitempty"`
 TLS  *TLSApp  `json:"tls,omitempty"`
}
```

**Problem:** Our `Apps` struct only supports `http` and `tls`, not `crowdsec`.

**If app-level config is required (Hypothesis 1):**

- Must extend `Apps` struct with `CrowdSec *CrowdSecApp`
- Define the CrowdSecApp configuration schema
- Generate app config at same level as HTTP/TLS

---

### 1.5 Evidence from Caddy Plugin Architecture

**Common Caddy Plugin Patterns:**

Most Caddy modules that need app-level configuration follow this structure:

```go
// App-level configuration (shared state)
type SomeApp struct {
    APIURL string `json:"api_url"`
    APIKey string `json:"api_key"`
}

// Handler (references app config, minimal inline config)
type SomeHandler struct {
    // Handler does NOT duplicate app config
}
```

**Examples in our build:**

- **caddy-security:** Has app-level config for OAuth/SAML, handlers reference it
- **CrowdSec bouncer:** Likely follows same pattern (hypothesis)

---

## 2. Hypothesis Decision Tree

### 🎯 Hypothesis 1: App-Level Configuration (PRIMARY)

**Confidence:** 70%
**Priority:** Test First
**Estimated Time:** 30-45 minutes

#### Theory

Plugin expects configuration in the `apps` section of Caddy JSON config, with handler being just a reference/trigger.

#### Expected JSON Structure

```json
{
  "apps": {
    "http": {
      "servers": {...}
    },
    "crowdsec": {
      "api_url": "http://127.0.0.1:8085",
      "api_key": "abc123...",
      "ticker_interval": "60s",
      "enable_streaming": true
    }
  }
}
```

Handler becomes:

```json
{
  "handler": "crowdsec"
}
```

#### Evidence Supporting This Hypothesis

✅ **Caddyfile shows app-level block** (`crowdsec { }` at global scope)
✅ **Matches caddy-security pattern** (also in our Dockerfile)
✅ **Explains why inline config rejected** (wrong location)
✅ **Common pattern for shared app state** (multiple routes referencing same config)
✅ **Makes architectural sense** (LAPI connection is app-wide, not per-route)

#### Implementation Steps

**Step 1: Extend Type Definitions**

File: `backend/internal/caddy/types.go`

```go
// Add after line 60
type CrowdSecApp struct {
    APIURL          string `json:"api_url"`
    APIKey          string `json:"api_key,omitempty"`
    TickerInterval  string `json:"ticker_interval,omitempty"`
    EnableStreaming bool   `json:"enable_streaming,omitempty"`
    // Optional advanced fields
    DisableStreaming bool `json:"disable_streaming,omitempty"`
    EnableHardFails  bool `json:"enable_hard_fails,omitempty"`
}

// Modify Apps struct
type Apps struct {
    HTTP     *HTTPApp     `json:"http,omitempty"`
    TLS      *TLSApp      `json:"tls,omitempty"`
    CrowdSec *CrowdSecApp `json:"crowdsec,omitempty"` // NEW
}
```

**Step 2: Update Config Generation**

File: `backend/internal/caddy/config.go`

Modify `GenerateConfig()` function (around line 70-100, after TLS app setup):

```go
// After TLS app configuration block, add:
if crowdsecEnabled {
    apiKey := getCrowdSecAPIKey()
    apiURL := "http://127.0.0.1:8085"
    if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
        apiURL = secCfg.CrowdSecAPIURL
    }

    config.Apps.CrowdSec = &CrowdSecApp{
        APIURL:          apiURL,
        APIKey:          apiKey,
        TickerInterval:  "60s",
        EnableStreaming: true,
    }
}
```

**Step 3: Simplify Handler Builder**

File: `backend/internal/caddy/config.go`

Modify `buildCrowdSecHandler()` function (lines 750-780):

```go
func buildCrowdSecHandler(_ *models.ProxyHost, secCfg *models.SecurityConfig, crowdsecEnabled bool) (Handler, error) {
    if !crowdsecEnabled {
        return nil, nil
    }

    // Handler now just references the app-level config
    // No inline configuration needed
    return Handler{"handler": "crowdsec"}, nil
}
```

**Step 4: Update Unit Tests**

File: `backend/internal/caddy/config_crowdsec_test.go`

Update expectations in tests:

```go
func TestBuildCrowdSecHandler_EnabledWithoutConfig(t *testing.T) {
    h, err := buildCrowdSecHandler(nil, nil, true)
    require.NoError(t, err)
    require.NotNil(t, h)

    // Handler should only have "handler" field
    assert.Equal(t, "crowdsec", h["handler"])
    assert.Len(t, h, 1) // No other fields
}

func TestGenerateConfig_WithCrowdSec(t *testing.T) {
    host := models.ProxyHost{/*...*/}
    sec := &models.SecurityConfig{
        CrowdSecAPIURL: "http://test.local:8085",
    }

    cfg, err := GenerateConfig(/*...*/, true, /*...*/, sec)
    require.NoError(t, err)

    // Check app-level config
    require.NotNil(t, cfg.Apps.CrowdSec)
    assert.Equal(t, "http://test.local:8085", cfg.Apps.CrowdSec.APIURL)
    assert.True(t, cfg.Apps.CrowdSec.EnableStreaming)

    // Check handler is minimal
    route := cfg.Apps.HTTP.Servers["charon_server"].Routes[0]
    found := false
    for _, h := range route.Handle {
        if hn, ok := h["handler"].(string); ok && hn == "crowdsec" {
            assert.Len(t, h, 1) // Only "handler" field
            found = true
            break
        }
    }
    require.True(t, found)
}
```

#### Verification Steps

1. **Run unit tests:**

   ```bash
   cd backend
   go test ./internal/caddy/... -v -run TestCrowdSec
   ```

2. **Rebuild Docker image:**

   ```bash
   docker build --no-cache -t charon:local .
   docker compose -f docker-compose.override.yml up -d
   ```

3. **Check Caddy logs for errors:**

   ```bash
   docker logs charon 2>&1 | grep -i "json: unknown field"
   ```

   Expected: No errors

4. **Verify bouncer registration:**

   ```bash
   docker exec charon cscli bouncers list
   ```

   Expected: `caddy-bouncer` appears with recent `last_pull` timestamp

5. **Test blocking:**

   ```bash
   # Add test block
   docker exec charon cscli decisions add --ip 1.2.3.4 --duration 1h --reason "Test"

   # Test request (simulate from blocked IP)
   curl -H "X-Forwarded-For: 1.2.3.4" http://localhost/
   ```

   Expected: 403 Forbidden

6. **Check Security Logs in UI:**
   Expected: `source: "crowdsec"`, `blocked: true`

#### Success Criteria

- ✅ No "json: unknown field" errors in Caddy logs
- ✅ `cscli bouncers list` shows active bouncer with `last_pull` timestamp
- ✅ Blocked IPs return 403 Forbidden responses
- ✅ Security Logs show `source: "crowdsec"` for blocked traffic
- ✅ All unit tests pass

#### Rollback Plan

If this hypothesis fails:

1. Revert changes to `types.go` and `config.go`
2. Restore original `buildCrowdSecHandler()` implementation
3. Proceed to Hypothesis 2

---

### 🎯 Hypothesis 2: Alternative Field Names (FALLBACK)

**Confidence:** 20%
**Priority:** Test if Hypothesis 1 fails
**Estimated Time:** 15 minutes

#### Theory

Plugin accepts inline handler config, but with different/undocumented field names.

#### Variants to Test Sequentially

```go
// Variant A: Short names
Handler{
    "handler": "crowdsec",
    "url":     "http://127.0.0.1:8085",
    "key":     apiKey,
}

// Variant B: CrowdSec standard terms
Handler{
    "handler":     "crowdsec",
    "lapi":        "http://127.0.0.1:8085",
    "bouncer_key": apiKey,
}

// Variant C: Fully qualified
Handler{
    "handler":           "crowdsec",
    "crowdsec_api_url":  "http://127.0.0.1:8085",
    "crowdsec_api_key":  apiKey,
}

// Variant D: Underscores instead of camelCase
Handler{
    "handler":           "crowdsec",
    "api_url":           "http://127.0.0.1:8085",
    "api_key":           apiKey,
    "enable_streaming":  true,
}
```

#### Implementation

Test each variant by modifying `buildCrowdSecHandler()`, rebuild, check Caddy logs.

#### Success Criteria

Any variant that doesn't produce "json: unknown field" error.

---

### 🎯 Hypothesis 3: HTTP App Nested Config

**Confidence:** 10%
**Priority:** Test if Hypothesis 1-2 fail
**Estimated Time:** 20 minutes

#### Theory

Configuration goes under `apps.http.crowdsec` instead of separate `apps.crowdsec`.

#### Expected Structure

```json
{
  "apps": {
    "http": {
      "crowdsec": {
        "api_url": "http://127.0.0.1:8085",
        "api_key": "..."
      },
      "servers": {...}
    }
  }
}
```

#### Implementation

Modify `HTTPApp` struct in `types.go`:

```go
type HTTPApp struct {
    Servers  map[string]*Server `json:"servers"`
    CrowdSec *CrowdSecApp       `json:"crowdsec,omitempty"` // NEW
}
```

Populate in `GenerateConfig()` before creating servers.

---

### 🎯 Hypothesis 4: Plugin Version/Breaking Change

**Confidence:** 5%
**Priority:** Last resort / parallel investigation
**Estimated Time:** 2-4 hours

#### Theory

Latest plugin version (from `main` branch) broke JSON API compatibility.

#### Investigation Steps

1. **Check plugin GitHub:**
   - Look for recent commits with "BREAKING CHANGE"
   - Check issues for JSON configuration questions
   - Review pull requests for API changes

2. **Clone and analyze source:**

   ```bash
   git clone https://github.com/hslatman/caddy-crowdsec-bouncer /tmp/plugin
   cd /tmp/plugin

   # Find JSON struct tags
   grep -r "json:" --include="*.go" | grep -i "url\|key\|api"

   # Check main handler struct
   cat crowdsec.go | grep -A 20 "type.*struct"
   ```

3. **Test with older version:**
   Modify Dockerfile to pin specific version:

   ```dockerfile
   --with github.com/hslatman/caddy-crowdsec-bouncer@v0.4.0
   ```

#### Success Criteria

Find exact JSON schema from source code or older version that works.

---

## 3. Fallback: Caddyfile Adapter Method

**If all hypotheses fail**, use Caddy's built-in adapter to reverse-engineer the JSON schema.

### Steps

1. **Create test Caddyfile:**

   ```bash
   docker exec charon sh -c 'cat > /tmp/test.caddyfile << "EOF"
   {
     crowdsec {
       api_url http://127.0.0.1:8085
       api_key test-key-12345
       ticker_interval 60s
     }
   }

   example.com {
     reverse_proxy localhost:8080
   }
   EOF'
   ```

2. **Convert to JSON:**

   ```bash
   docker exec charon caddy adapt --config /tmp/test.caddyfile --pretty
   ```

3. **Analyze output:**
   - Look for `apps.crowdsec` or `apps.http.crowdsec` section
   - Note exact field names and structure
   - Implement matching structure in Go code

**Advantage:** Guaranteed to work (uses official parser)
**Disadvantage:** Requires test container and manual analysis

---

## 4. Verification Checklist

### Pre-Flight Checks (Before Testing)

- [ ] CrowdSec LAPI is running: `curl http://127.0.0.1:8085/health`
- [ ] API key exists: `docker exec charon cat /etc/crowdsec/bouncers/caddy-bouncer.key`
- [ ] Bouncer registration script available: `/usr/local/bin/register_bouncer.sh`

### Configuration Checks (After Implementation)

- [ ] Caddy config loads without errors
- [ ] No "json: unknown field" in logs: `docker logs charon 2>&1 | grep "unknown field"`
- [ ] Caddy admin API responds: `curl http://localhost:2019/config/`

### Bouncer Registration (Critical Check)

```bash
docker exec charon cscli bouncers list
```

**Expected output:**

```
┌──────────────┬──────────────────────────┬─────────┬───────────────────────┬───────────┐
│     Name     │         API Key          │ Revoked │     Last Pull         │  Type     │
├──────────────┼──────────────────────────┼─────────┼───────────────────────┼───────────┤
│ caddy-bouncer│ abc123...               │  false  │ 2025-12-15T17:30:45Z  │ crowdsec │
└──────────────┴──────────────────────────┴─────────┴───────────────────────┴───────────┘
```

**If empty:** Bouncer is not connecting to LAPI (config still wrong)

### Traffic Blocking Test

```bash
# 1. Add test block
docker exec charon cscli decisions add --ip 1.2.3.4 --duration 1h --reason "Test block"

# 2. Verify decision exists
docker exec charon cscli decisions list

# 3. Test from blocked IP
curl -H "X-Forwarded-For: 1.2.3.4" http://localhost/

# Expected: 403 Forbidden with body "Forbidden"

# 4. Check Security Logs in UI
# Expected: Entry with source="crowdsec", blocked=true, decision_type="ban"

# 5. Cleanup
docker exec charon cscli decisions delete --ip 1.2.3.4
```

---

## 5. Success Metrics

### Blockers Resolved

- ✅ Bouncer appears in `cscli bouncers list` with recent `last_pull`
- ✅ No "json: unknown field" errors in Caddy logs
- ✅ Blocked IPs receive 403 Forbidden responses
- ✅ Security Logs correctly show `source: "crowdsec"` for blocks
- ✅ Response headers include `X-Crowdsec-Decision` for blocked requests

### Production Ready Checklist

- ✅ All unit tests pass (`go test ./internal/caddy/... -v`)
- ✅ Integration test passes (`scripts/crowdsec_integration.sh`)
- ✅ Pre-commit hooks pass (`pre-commit run --all-files`)
- ✅ Documentation updated (see Section 6)

---

## 6. Documentation Updates Required

After successful implementation:

### Files to Update

1. **`docs/features.md`**
   - Add section: "CrowdSec Configuration (App-Level)"
   - Document the JSON structure
   - Explain app-level vs handler-level config

2. **`docs/security.md`**
   - Document bouncer integration architecture
   - Add troubleshooting section for bouncer registration

3. **`docs/troubleshooting/crowdsec_bouncer_config.md`** (NEW)
   - Common configuration errors
   - How to verify bouncer connection
   - Manual registration steps

4. **`backend/internal/caddy/config.go`**
   - Update function comments (lines 741-749)
   - Document app-level configuration pattern
   - Add example JSON in comments

5. **`.github/copilot-instructions.md`**
   - Add CrowdSec configuration pattern to "Big Picture"
   - Note that CrowdSec uses app-level config (unlike WAF/rate_limit)

6. **`IMPLEMENTATION_SUMMARY.md`**
   - Add to "Lessons Learned" section
   - Document Caddyfile ≠ JSON pattern discovery

---

## 7. Rollback Plan

### If All Hypotheses Fail

1. **Immediate Actions:**
   - Revert all code changes to `types.go` and `config.go`
   - Set `CHARON_SECURITY_CROWDSEC_MODE=disabled` in docker-compose files
   - Document blocker in GitHub issue (link to this plan)

2. **Contact Plugin Maintainer:**
   - Open issue: <https://github.com/hslatman/caddy-crowdsec-bouncer/issues>
   - Title: "JSON Configuration Schema Undocumented - Request Examples"
   - Include: Our tested field names, error messages, Caddy version
   - Ask: Exact JSON schema or working example

3. **Evaluate Alternatives:**
   - **Option A:** Use different CrowdSec bouncer (Nginx, Traefik)
   - **Option B:** Direct LAPI integration in Go (bypass Caddy plugin)
   - **Option C:** CrowdSec standalone with iptables remediation

### If Plugin is Broken/Abandoned

- Fork plugin and fix JSON unmarshaling ourselves
- Contribute fix back via pull request
- Document custom fork in Dockerfile and README

---

## 8. External Resources

### Plugin Resources

- **GitHub Repo:** <https://github.com/hslatman/caddy-crowdsec-bouncer>
- **Issues:** <https://github.com/hslatman/caddy-crowdsec-bouncer/issues>
- **Latest Release:** Check for version tags and changelog

### Caddy Documentation

- **JSON Config:** <https://caddyserver.com/docs/json/>
- **App Modules:** <https://caddyserver.com/docs/json/apps/>
- **HTTP Handlers:** <https://caddyserver.com/docs/json/apps/http/servers/routes/handle/>

### CrowdSec Documentation

- **Bouncer API:** <https://docs.crowdsec.net/docs/next/bouncers/intro/>
- **Local API (LAPI):** <https://docs.crowdsec.net/docs/next/local_api/intro/>

---

## 9. Implementation Sequence

**Recommended Order:**

1. **Phase 1 (30-45 min):** Implement Hypothesis 1 (App-Level Config)
   - Highest confidence (70%)
   - Best architectural fit
   - Most maintainable long-term

2. **Phase 2 (15 min):** If Phase 1 fails, test Hypothesis 2 (Field Name Variants)
   - Quick to test
   - Low effort

3. **Phase 3 (20 min):** If Phase 1-2 fail, try Hypothesis 3 (HTTP App Nested)
   - Less common but possible

4. **Phase 4 (1-2 hours):** If all fail, use Caddyfile Adapter Method
   - Guaranteed to reveal correct structure
   - Requires container and manual analysis

5. **Phase 5 (2-4 hours):** Nuclear option - investigate plugin source code
   - Last resort
   - Most time-consuming
   - May require filing GitHub issue

---

## 10. Next Actions

**IMMEDIATE:** Implement Hypothesis 1 (App-Level Configuration)

**Owner:** Implementation Agent
**Blocker Status:** This is the ONLY remaining blocker for CrowdSec production deployment
**ETA:** 30-45 minutes to first test
**Confidence:** 70% success rate

**After Resolution:**

- Update all documentation
- Run full integration test suite
- Mark issue #17 as complete
- Consider PR to plugin repo documenting JSON schema

---

**END OF RESEARCH PLAN**

This plan provides 3-5 concrete, testable approaches ranked by likelihood. Proceed with Hypothesis 1 immediately.
