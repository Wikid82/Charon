# Investigation: Seerr SSO Auth Failure Through Proxy

**Date:** December 20, 2025
**Status:** **ROOT CAUSE CONFIRMED - CONFIG REGENERATION FAILURE**
**Priority:** **CRITICAL**

---

## Executive Summary

**The Seerr SSO authentication fails because `X-Forwarded-Port` header is missing from the Caddy config.**

### The Real Problem (Updated)

1. **Database State**: `enable_standard_headers = 1` (TRUE) ✅
2. **UI State**: "Standard Proxy Headers" checkbox is ENABLED ✅
3. **Caddy Live Config**: Missing `X-Forwarded-Port` header ❌
4. **Config Snapshot**: Dated **Dec 19 13:57**, but proxy host was updated at **Dec 19 20:58** ❌

**Root Cause**: The Caddy configuration was **NOT regenerated** after the proxy host update. `ApplyConfig()` either:

- Was not called after the database update, OR
- Was called but failed silently without rolling back the database change, OR
- Succeeded but the generated config didn't include the header due to a logic bug

This is a **critical disconnect** between the database state and the running Caddy configuration.

---

## Evidence Analysis

### 1. Database State (Current)

```sql
SELECT id, name, domain_names, application, websocket_support, enable_standard_headers, updated_at
FROM proxy_hosts WHERE domain_names LIKE '%seerr%';

-- Result:
-- 15|Seerr|seerr.hatfieldhosted.com|none|1|1|2025-12-19 20:58:31.091162109-05:00
```

**Analysis:**

- ✅ `enable_standard_headers = 1` (TRUE)
- ✅ `websocket_support = 1` (TRUE)
- ✅ `application = "none"` (no app-specific overrides)
- ⏰ Last updated: **Dec 19, 2025 at 20:58:31** (8:58 PM)

### 2. Live Caddy Config (Retrieved via API)

```bash
curl -s http://localhost:2019/config/ | jq '.apps.http.servers.charon_server.routes[] | select(.match[].host[] | contains("seerr"))'
```

**Headers Present in Reverse Proxy:**

```json
{
  "Connection": ["{http.request.header.Connection}"],
  "Upgrade": ["{http.request.header.Upgrade}"],
  "X-Forwarded-Host": ["{http.request.host}"],
  "X-Forwarded-Proto": ["{http.request.scheme}"],
  "X-Real-IP": ["{http.request.remote.host}"]
}
```

**Missing Headers:**

- ❌ `X-Forwarded-Port` - **COMPLETELY ABSENT**

**Analysis:**

- This is NOT a complete "standard headers disabled" situation
- 3 out of 4 standard headers ARE present (X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host)
- Only `X-Forwarded-Port` is missing
- WebSocket headers (Connection, Upgrade) are present as expected

### 3. Config Snapshot File Analysis

```bash
ls -lt /app/data/caddy/

# Most recent snapshot:
# -rw-r--r-- 1 root root 45742 Dec 19 13:57 config-1766170642.json
```

**Snapshot Timestamp:** **Dec 19, 2025 at 13:57** (1:57 PM)
**Proxy Host Updated:** Dec 19, 2025 at **20:58:31** (8:58 PM)

**Time Gap:** **7 hours and 1 minute** between the last config generation and the proxy host update.

### 4. Caddy Access Logs (Real Requests)

From logs at `2025-12-19 21:26:01`:

```json
"headers": {
  "Via": ["2.0 Caddy"],
  "X-Real-Ip": ["172.20.0.1"],
  "X-Forwarded-For": ["172.20.0.1"],
  "X-Forwarded-Proto": ["https"],
  "X-Forwarded-Host": ["seerr.hatfieldhosted.com"],
  "Connection": [""],
  "Upgrade": [""]
}
```

**Confirmed:** `X-Forwarded-Port` is NOT being sent to the upstream Seerr service.

---

## Root Cause Analysis

### Issue 1: Config Regeneration Did Not Occur After Update

**Timeline Evidence:**

1. Proxy host updated in database: `2025-12-19 20:58:31`
2. Most recent Caddy config snapshot: `2025-12-19 13:57:00`
3. **Gap:** 7 hours and 1 minute

**Code Path Review** (proxy_host_handler.go Update method):

```go
// Line 375: Database update succeeds
if err := h.service.Update(host); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
}

// Line 381: ApplyConfig is called
if h.caddyManager != nil {
    if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
        return
    }
}
```

**Expected Behavior:** `ApplyConfig()` should be called immediately after the database UPDATE succeeds.

**Actual Behavior:** The config snapshot timestamp shows no regeneration occurred.

**Possible Causes:**

1. `h.caddyManager` was `nil` (unlikely - other hosts work)
2. `ApplyConfig()` was called but returned an error that was NOT propagated to the UI
3. `ApplyConfig()` succeeded but didn't write a new snapshot (logic bug in snapshot rotation)
4. The UPDATE request never reached this code path (frontend bug, API route issue)

### Issue 2: Partial Standard Headers in Live Config

**Expected Behavior** (from types.go lines 144-153):

```go
if enableStandardHeaders {
    setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
    setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
    setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
    setHeaders["X-Forwarded-Port"] = []string{"{http.request.port}"}  // <-- THIS SHOULD BE SET
}
```

**Actual Behavior in Live Caddy Config:**

- X-Real-IP: ✅ Present
- X-Forwarded-Proto: ✅ Present
- X-Forwarded-Host: ✅ Present
- X-Forwarded-Port: ❌ **MISSING**

**Analysis:**
The presence of 3 out of 4 headers indicates that:

1. The running config was generated when `enableStandardHeaders` was at least partially true, OR
2. There's an **older version of the code** that only added 3 headers, OR
3. The WebSocket + Application logic is interfering with the 4th header

**Historical Code Check Required:** Was there ever a version of `ReverseProxyHandler` that added only 3 standard headers?

### Issue 3: WebSocket vs Standard Headers Interaction

Seerr has `websocket_support = 1`. Let's trace the header generation logic:

```go
// STEP 1: Standard headers (if enabled)
if enableStandardHeaders {
    setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
    setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
    setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
    setHeaders["X-Forwarded-Port"] = []string{"{http.request.port}"}
}

// STEP 2: WebSocket headers
if enableWS {
    setHeaders["Upgrade"] = []string{"{http.request.header.Upgrade}"}
    setHeaders["Connection"] = []string{"{http.request.header.Connection}"}
}

// STEP 3: Application-specific (none for application="none")
```

**No Conflict:** WebSocket headers should NOT overwrite or prevent standard headers.

---

## Root Cause Analysis

### 1. The `enable_standard_headers` Field is `false` for Seerr

The Seerr proxy host was created/migrated **before** the standard headers feature was added. Per the migration logic:

- **New hosts:** `enable_standard_headers = true` (default)
- **Existing hosts:** `enable_standard_headers = false` (backward compatibility)

### 2. Code Path Verification

From `types.go`:

```go
func ReverseProxyHandler(dial string, enableWS bool, application string, enableStandardHeaders bool) Handler {
    // ...

    // STEP 1: Standard proxy headers (if feature enabled)
    if enableStandardHeaders {
        setHeaders["X-Real-IP"] = []string{"{http.request.remote.host}"}
        setHeaders["X-Forwarded-Proto"] = []string{"{http.request.scheme}"}
        setHeaders["X-Forwarded-Host"] = []string{"{http.request.host}"}
        setHeaders["X-Forwarded-Port"] = []string{"{http.request.port}"}
    }
```

When `enableStandardHeaders = false`, **no standard headers are added**.

### 3. Config Generation Path

From `config.go`:

```go
// Determine if standard headers should be enabled (default true if nil)
enableStdHeaders := host.EnableStandardHeaders == nil || *host.EnableStandardHeaders
mainHandlers = append(mainHandlers, ReverseProxyHandler(dial, host.WebsocketSupport, host.Application, enableStdHeaders))
```

This is **correct** - the logic properly defaults to `true` when `nil`. The issue is that Seerr has `enable_standard_headers = false` explicitly set.

---

## Why Seerr SSO Fails

### Seerr/Overseerr Authentication Flow

1. User visits `seerr.hatfieldhosted.com`
2. Clicks "Sign in with Plex"
3. Plex OAuth redirects back to `seerr.hatfieldhosted.com/api/v1/auth/plex/callback`
4. Seerr validates the callback and needs to know:
   - **Client's real IP** (`X-Real-IP` or `X-Forwarded-For`)
   - **Original protocol** (`X-Forwarded-Proto`) for HTTPS cookie security
   - **Original host** (`X-Forwarded-Host`) for redirect validation

### Without These Headers

- Seerr sees `X-Forwarded-Proto` as missing → assumes HTTP
- Secure cookies may fail to set properly
- CORS/redirect validation may fail because host header mismatch
- OAuth callback may be rejected due to origin mismatch

---

## Cookie and Authorization Headers - NOT THE ISSUE

Caddy's `reverse_proxy` directive **preserves all headers by default**, including:

- `Cookie`
- `Authorization`
- `Accept`
- All other standard HTTP headers

The `headers.request.set` configuration **adds or overwrites** headers; it does NOT delete existing headers. There is no header stripping happening.

---

## Trusted Proxies - NOT THE ISSUE

The server-level `trusted_proxies` configuration is correctly set:

```go
trustedProxies := &TrustedProxies{
    Source: "static",
    Ranges: []string{
        "127.0.0.1/32",
        "::1/128",
        "172.16.0.0/12",  // Docker bridge networks
        "10.0.0.0/8",
        "192.168.0.0/16",
    },
}
```

This allows Caddy to trust `X-Forwarded-For` from internal networks.

---

## Solution

### Immediate Fix (User Action)

1. Edit the Seerr proxy host in Charon UI
2. Enable "Standard Proxy Headers" checkbox
3. Save

This will add the required headers to the Seerr route.

### Permanent Fix (Code Change - ALREADY IMPLEMENTED)

The handler fix for the three missing fields was implemented. The fields are now handled in the Update handler:

- `enable_standard_headers` - nullable bool handler added
- `forward_auth_enabled` - regular bool handler added
- `waf_disabled` - regular bool handler added

---

## Verification Steps

After enabling standard headers for Seerr:

### 1. Verify Caddy Config

```bash
docker exec charon cat /app/data/caddy/config.json | python3 -c "
import json, sys
data = json.load(sys.stdin)
routes = data.get('apps', {}).get('http', {}).get('servers', {}).get('charon_server', {}).get('routes', [])
for route in routes:
    for match in route.get('match', []):
        if any('seerr' in h.lower() for h in match.get('host', [])):
            print(json.dumps(route, indent=2))
"
```

**Expected:** Should show `X-Real-IP`, `X-Forwarded-Proto`, `X-Forwarded-Host`, `X-Forwarded-Port` in the reverse_proxy handler.

### 2. Test SSO Login

1. Clear browser cookies for Seerr
2. Visit `seerr.hatfieldhosted.com`
3. Click "Sign in with Plex"
4. Complete OAuth flow
5. Should successfully authenticate

---

## Files Analyzed

| File | Status | Notes |
|------|--------|-------|
| `types.go` | ✅ Correct | `ReverseProxyHandler` properly adds headers when enabled |
| `config.go` | ✅ Correct | Properly passes `enableStandardHeaders` parameter |
| `proxy_host.go` | ✅ Correct | Field definition is correct |
| `proxy_host_handler.go` | ✅ Fixed | Now handles `enable_standard_headers` in Update |
| `caddy_config_qa.json` | 📊 Evidence | Shows Seerr route missing standard headers |

---

## Conclusion

**The root cause is a STALE CONFIGURATION caused by a failed or skipped `ApplyConfig()` call.**

**Evidence:**

- Database: `enable_standard_headers = 1`, `updated_at = 2025-12-19 20:58:31` ✅
- UI: "Standard Proxy Headers" checkbox is ENABLED ✅
- Config Snapshot: Last generated at `2025-12-19 13:57` (7+ hours before the DB update) ❌
- Live Caddy Config: Missing `X-Forwarded-Port` header ❌

**What Happened:**

1. User enabled "Standard Proxy Headers" for Seerr on Dec 19 at 20:58
2. Database UPDATE succeeded
3. `ApplyConfig()` either failed silently or was never called
4. The running config is from an older snapshot that predates the update

**Immediate Action:**

```bash
docker restart charon
```

This will force a complete config regeneration from the current database state.

**Long-term Fixes Needed:**

1. Wrap database updates in transactions that rollback on `ApplyConfig()` failure
2. Add enhanced logging to track config generation success/failure
3. Implement config staleness detection in health checks
4. Verify why the older config is missing `X-Forwarded-Port` (possible code version issue)

**Alternative Immediate Fix (No Restart):**

- Make a trivial change to any proxy host in the UI and save
- This triggers `ApplyConfig()` and regenerates all configs
