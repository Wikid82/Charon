# Critical Bug Analysis: 500 Error on Proxy Host Save

## Summary

**Root Cause Identified:** The 500 error is caused by an invalid Caddy configuration structure where `trusted_proxies` is set as an **object** at the **handler level** (within `reverse_proxy`), but Caddy's `http.handlers.reverse_proxy` expects it to be either:
1. An **array of strings** at the handler level, OR
2. An **object** only at the **server level**

The error from Caddy logs:
```
json: cannot unmarshal object into Go struct field Handler.trusted_proxies of type []string
```

---

## 1. Complete File List with Key Functions

### Frontend Layer

| File | Key Functions/Lines | Purpose |
|------|---------------------|---------|
| [frontend/src/components/ProxyHostForm.tsx](../../frontend/src/components/ProxyHostForm.tsx) | `handleSubmit()` L302-332 | Form submission, calls `onSubmit(payloadWithoutUptime)` |
| [frontend/src/hooks/useProxyHosts.ts](../../frontend/src/hooks/useProxyHosts.ts) | `updateMutation` L25-31, `updateHost()` L50 | React Query mutation for PUT requests |
| [frontend/src/api/proxyHosts.ts](../../frontend/src/api/proxyHosts.ts) | `updateProxyHost()` L57-60 | API client - `PUT /proxy-hosts/{uuid}` |

### Backend Layer

| File | Key Functions/Lines | Purpose |
|------|---------------------|---------|
| [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go) | L341-342 | Route registration: `router.PUT("/proxy-hosts/:uuid", h.Update)` |
| [backend/internal/api/handlers/proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go) | `Update()` L133-262 | HTTP handler - parses payload, updates model, calls `ApplyConfig()` |
| [backend/internal/services/proxyhost_service.go](../../backend/internal/services/proxyhost_service.go) | `Update()` L57-76 | Business logic - validates domain uniqueness, DB update |
| [backend/internal/models/proxy_host.go](../../backend/internal/models/proxy_host.go) | `ProxyHost` struct L10-61 | Database model with all fields |

### Caddy Configuration Layer

| File | Key Functions/Lines | Purpose |
|------|---------------------|---------|
| [backend/internal/caddy/manager.go](../../backend/internal/caddy/manager.go) | `ApplyConfig()` L48-169 | Orchestrates config generation, validation, and application |
| [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go) | `GenerateConfig()` L22-310 | Builds complete Caddy JSON config from DB |
| **[backend/internal/caddy/types.go](../../backend/internal/caddy/types.go)** | **`ReverseProxyHandler()` L130-201** | **🔴 BUG LOCATION - Creates invalid `trusted_proxies` structure** |

---

## 2. Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│ FRONTEND                                                                             │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ProxyHostForm.tsx                                                                   │
│  ┌─────────────────────────────────────┐                                             │
│  │ formData = {                        │                                             │
│  │   name: "Test",                     │                                             │
│  │   enable_standard_headers: true,    │  ← User enables this checkbox              │
│  │   websocket_support: true,          │                                             │
│  │   ...                               │                                             │
│  │ }                                   │                                             │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ handleSubmit() L302-332                                         │
│  ┌─────────────────────────────────────┐                                             │
│  │ onSubmit(payloadWithoutUptime)      │                                             │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ useProxyHosts.ts updateHost() L50                               │
│  ┌─────────────────────────────────────┐                                             │
│  │ updateMutation.mutateAsync({        │                                             │
│  │   uuid: "...",                      │                                             │
│  │   data: payload                     │                                             │
│  │ })                                  │                                             │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ proxyHosts.ts updateProxyHost() L57-60                          │
│  ┌─────────────────────────────────────┐                                             │
│  │ client.put(`/proxy-hosts/${uuid}`,  │                                             │
│  │   host)                             │                                             │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
└────────────────────│────────────────────────────────────────────────────────────────┘
                     │
                     │ HTTP PUT /api/v1/proxy-hosts/{uuid}
                     │ JSON Body: { enable_standard_headers: true, ... }
                     ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│ BACKEND                                                                              │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  routes.go L341-342                                                                  │
│  ┌─────────────────────────────────────┐                                             │
│  │ router.PUT("/proxy-hosts/:uuid",    │                                             │
│  │   h.Update)                         │                                             │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ proxy_host_handler.go Update() L133-262                         │
│  ┌─────────────────────────────────────┐                                             │
│  │ 1. GetByUUID(uuidStr) → host        │                                             │
│  │ 2. c.ShouldBindJSON(&payload)       │                                             │
│  │ 3. Parse fields from payload        │                                             │
│  │    - enable_standard_headers        │ ← Correctly parsed at L182-189             │
│  │ 4. h.service.Update(host)           │                                             │
│  │ 5. h.caddyManager.ApplyConfig()     │ ← 🔴 FAILURE POINT                          │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ proxyhost_service.go Update() L57-76                            │
│  ┌─────────────────────────────────────┐                                             │
│  │ 1. ValidateUniqueDomain()           │                                             │
│  │ 2. Normalize advanced_config        │                                             │
│  │ 3. db.Updates(host)                 │ ✅ Database update SUCCEEDS                 │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ manager.go ApplyConfig() L48-169                                │
│  ┌─────────────────────────────────────┐                                             │
│  │ 1. db.Find(&hosts)                  │                                             │
│  │ 2. GenerateConfig(hosts, ...)       │                                             │
│  │ 3. ValidateConfig(config)           │                                             │
│  │ 4. m.client.Load(ctx, config)       │ ← 🔴 CADDY REJECTS CONFIG                   │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ config.go GenerateConfig() L22-310                              │
│  ┌─────────────────────────────────────┐                                             │
│  │ For each enabled host:              │                                             │
│  │   ReverseProxyHandler(dial,         │                                             │
│  │     websocket, app,                 │                                             │
│  │     enableStandardHeaders)          │ ← 🔴 BUG TRIGGERED HERE                     │
│  └─────────────────────────────────────┘                                             │
│                    │                                                                 │
│                    ▼ types.go ReverseProxyHandler() L130-201                         │
│  ┌─────────────────────────────────────┐                                             │
│  │ 🔴 BUG: Creates invalid structure   │                                             │
│  │                                     │                                             │
│  │ h["trusted_proxies"] = map[string]  │                                             │
│  │   interface{}{                      │                                             │
│  │   "source": "static",               │ ← WRONG: Object at handler level            │
│  │   "ranges": []string{"private_..."},│                                             │
│  │ }                                   │                                             │
│  └─────────────────────────────────────┘                                             │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
                     │
                     │ POST /load to Caddy Admin API
                     │ (Invalid JSON structure)
                     ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│ CADDY                                                                                │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  EXPECTED by http.handlers.reverse_proxy:                                            │
│  ┌─────────────────────────────────────┐                                             │
│  │ "trusted_proxies": ["192.168.0.0/16",│ ← Array of strings                         │
│  │   "10.0.0.0/8", ...]                │                                             │
│  └─────────────────────────────────────┘                                             │
│                                                                                      │
│  RECEIVED (invalid):                                                                 │
│  ┌─────────────────────────────────────┐                                             │
│  │ "trusted_proxies": {                │ ← Object (wrong type!)                      │
│  │   "source": "static",               │                                             │
│  │   "ranges": ["private_ranges"]      │                                             │
│  │ }                                   │                                             │
│  └─────────────────────────────────────┘                                             │
│                                                                                      │
│  🔴 ERROR:                                                                           │
│  "json: cannot unmarshal object into Go struct field                                 │
│   Handler.trusted_proxies of type []string"                                          │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. JSON Payload Comparison

### What Frontend Sends (Correct)

```json
{
  "name": "Test Service",
  "domain_names": "test.example.com",
  "forward_scheme": "http",
  "forward_host": "192.168.1.100",
  "forward_port": 8080,
  "ssl_forced": true,
  "http2_support": true,
  "hsts_enabled": true,
  "hsts_subdomains": true,
  "block_exploits": true,
  "websocket_support": true,
  "enable_standard_headers": true,
  "application": "none",
  "enabled": true,
  "certificate_id": null,
  "access_list_id": null,
  "security_header_profile_id": null
}
```

### What Backend Generates for Caddy (INVALID)

```json
{
  "handler": "reverse_proxy",
  "upstreams": [{ "dial": "192.168.1.100:8080" }],
  "flush_interval": -1,
  "headers": {
    "request": {
      "set": {
        "X-Real-IP": ["{http.request.remote.host}"],
        "X-Forwarded-Proto": ["{http.request.scheme}"],
        "X-Forwarded-Host": ["{http.request.host}"],
        "X-Forwarded-Port": ["{http.request.port}"]
      }
    }
  },
  "trusted_proxies": {
    "source": "static",
    "ranges": ["private_ranges"]
  }
}
```

### What Caddy EXPECTS (Correct Structure)

According to Caddy's documentation, at the **handler level**, `trusted_proxies` must be an **array of CIDR strings**:

```json
{
  "handler": "reverse_proxy",
  "upstreams": [{ "dial": "192.168.1.100:8080" }],
  "flush_interval": -1,
  "headers": {
    "request": {
      "set": {
        "X-Real-IP": ["{http.request.remote.host}"],
        "X-Forwarded-Proto": ["{http.request.scheme}"],
        "X-Forwarded-Host": ["{http.request.host}"],
        "X-Forwarded-Port": ["{http.request.port}"]
      }
    }
  },
  "trusted_proxies": [
    "192.168.0.0/16",
    "10.0.0.0/8",
    "172.16.0.0/12",
    "127.0.0.1/32",
    "::1/128"
  ]
}
```

**Note:** The object structure with `source` and `ranges` is valid at the **server level** (in `Server.TrustedProxies`), not at the handler level.

---

## 4. Root Cause Analysis

### The Bug Location

**File:** `backend/internal/caddy/types.go`
**Function:** `ReverseProxyHandler()`
**Lines:** 186-189

```go
// STEP 4: Always configure trusted_proxies for security when headers are set
// This prevents IP spoofing attacks by only trusting headers from known proxy sources
if len(setHeaders) > 0 {
    h["trusted_proxies"] = map[string]interface{}{      // 🔴 BUG: Object structure
        "source": "static",
        "ranges": []string{"private_ranges"},           // 🔴 "private_ranges" is also invalid
    }
}
```

### Problems

1. **Wrong Type:** `trusted_proxies` at handler level must be `[]string`, not `map[string]interface{}`
2. **Invalid Value:** `"private_ranges"` is not a valid CIDR - Caddy doesn't expand this magic string at the handler level
3. **Server vs Handler Confusion:** The object structure `{source, ranges}` is only valid in `apps.http.servers.*.trusted_proxies`, not in route handlers

### Why This Wasn't Caught Before

1. **Unit tests pass** because they only check the Go structure, not Caddy's actual validation
2. **Integration tests** may have been run with a different Caddy version or config
3. **Recent addition:** The `trusted_proxies` at handler level was added as part of the standard headers feature

---

## 5. Proposed Fix

### Option A: Remove Handler-Level trusted_proxies (Recommended)

The server-level `trusted_proxies` in `config.go` is already correctly configured and applies to ALL routes. The handler-level setting is redundant and incorrect.

**File:** `backend/internal/caddy/types.go`
**Change:** Remove lines 184-189

```go
// REMOVE THIS ENTIRE BLOCK (lines 184-189):
// STEP 4: Always configure trusted_proxies for security when headers are set
// This prevents IP spoofing attacks by only trusting headers from known proxy sources
if len(setHeaders) > 0 {
    h["trusted_proxies"] = map[string]interface{}{
        "source": "static",
        "ranges": []string{"private_ranges"},
    }
}
```

**Rationale:** The server already has trusted_proxies configured at config.go L295-306:

```go
// Configure trusted proxies for proper client IP detection from X-Forwarded-For headers
trustedProxies := &TrustedProxies{
    Source: "static",
    Ranges: []string{
        "127.0.0.1/32",   // Localhost
        "::1/128",        // IPv6 localhost
        "172.16.0.0/12",  // Docker bridge networks
        "10.0.0.0/8",     // Private network
        "192.168.0.0/16", // Private network
    },
}

config.Apps.HTTP.Servers["charon_server"] = &Server{
    ...
    TrustedProxies: trustedProxies,  // ← This is correct and sufficient
    ...
}
```

### Option B: Fix Handler-Level Structure (If Per-Route Control Needed)

If per-route trusted_proxies control is needed in the future, fix the structure:

```go
// STEP 4: Always configure trusted_proxies for security when headers are set
if len(setHeaders) > 0 {
    h["trusted_proxies"] = []string{   // ← ARRAY of CIDRs
        "192.168.0.0/16",
        "10.0.0.0/8",
        "172.16.0.0/12",
        "127.0.0.1/32",
        "::1/128",
    }
}
```

### Tests to Update

**File:** `backend/internal/caddy/types_extra_test.go`

Update tests that expect the object structure:
- L87-93: `TestReverseProxyHandler_StandardHeadersEnabled`
- L133-139: `TestReverseProxyHandler_WebSocketWithApplication`
- L256-279: `TestReverseProxyHandler_TrustedProxiesConfiguration`

---

## 6. Verification Steps

After applying the fix:

1. **Rebuild container:**
   ```bash
   docker build --no-cache -t charon:local .
   docker compose -f docker-compose.override.yml up -d
   ```

2. **Check logs for successful config application:**
   ```bash
   docker logs charon 2>&1 | grep -i "caddy config"
   # Should see: "Successfully applied initial Caddy config"
   ```

3. **Test proxy host save:**
   - Edit any proxy host in UI
   - Toggle "Enable Standard Proxy Headers"
   - Click Save
   - Should succeed (200 response, no 500 error)

4. **Verify Caddy config:**
   ```bash
   curl -s http://localhost:2019/config/ | jq '.apps.http.servers.charon_server.trusted_proxies'
   # Should show server-level trusted_proxies (not in individual routes)
   ```

---

## 7. Timeline of Events

1. **Standard Proxy Headers Feature Added** - Added `enable_standard_headers` field
2. **trusted_proxies Added to Handler** - Someone added trusted_proxies at the handler level for security
3. **Wrong Structure Used** - Used the server-level object format instead of handler-level array format
4. **Tests Pass** - Go unit tests check structure, not Caddy validation
5. **Container Deployed** - Bug deployed to production
6. **User Saves Proxy Host** - Triggers config regeneration with invalid structure
7. **Caddy Rejects Config** - Returns 400 error
8. **Handler Returns 500** - `ApplyConfig` failure triggers 500 response to user

---

## 8. Files Changed Summary

| File | Line(s) | Change Required |
|------|---------|-----------------|
| `backend/internal/caddy/types.go` | 184-189 | **DELETE** the handler-level trusted_proxies block |
| `backend/internal/caddy/types_extra_test.go` | 87-93, 133-139, 256-279 | **UPDATE** tests to not expect handler-level trusted_proxies |

---

## 9. Risk Assessment

| Risk | Level | Mitigation |
|------|-------|------------|
| Breaking existing configs | Low | Server-level trusted_proxies already provides same protection |
| Security regression | None | Server-level config is equivalent protection |
| Test failures | Medium | Need to update 3 test functions |
| Rollback needed | Low | Simple code deletion, easy to revert |

---

## Conclusion

The 500 error is caused by **incorrect JSON structure** for `trusted_proxies` at the reverse_proxy handler level. The fix is to **remove the handler-level trusted_proxies** since the server-level configuration already provides the same security protection.

**Recommended Action:** Delete lines 184-189 in `types.go` and update the corresponding tests.
