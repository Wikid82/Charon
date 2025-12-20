# Trace Analysis: Proxy Host Update Handler - Missing Fields

**Date:** December 20, 2025
**Status:** Ready for Implementation
**Related Bugs:**

1. **Auth Pass-through Failure**: User can login to Seerr via IP:port but NOT through proxied domain
2. **500 Error on Proxy Host Save**: When editing/saving proxy host, returns 500 Internal Server Error

---

## Executive Summary

After comprehensive trace analysis of both data flows, I have identified **THREE missing fields** in the Update handler and **one configuration issue**:

### Critical Finding: Three Missing Fields in Update Handler

The `Update` handler in [proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go#L155) **does not process the following fields** from the payload:

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `enable_standard_headers` | `*bool` (nullable) | `true` | Controls X-Real-IP, X-Forwarded-* headers |
| `forward_auth_enabled` | `bool` | `false` | Enables forward auth via Charon |
| `waf_disabled` | `bool` | `false` | Disables WAF for this specific host |

This means:

1. When the frontend sends these fields during edit/save, the backend ignores them
2. The fields remain at their default/previous values
3. This could cause a 500 error if downstream code expects properly set values
4. Missing headers break auth pass-through for apps like Seerr/Overseerr

### Secondary Finding: WebSocket Header Configuration is Correct

The WebSocket and auth header pass-through configuration in [types.go](../../backend/internal/caddy/types.go#L127) appears correctly implemented. However, **if `enable_standard_headers` isn't being persisted**, then the generated Caddy config may be missing required headers.

---

## Implementation Plan

### Phase 1: Backend Handler Fix (All 3 Fields)

**Target File:** [backend/internal/api/handlers/proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go)

**Location:** After line ~220 (after `enabled` field handling, before `certificate_id`)

#### 1.1 Add `enable_standard_headers` Handler (Nullable Bool Pattern)

This field uses a nullable boolean (`*bool`), following the same pattern as `certificate_id`:

```go
// Handle enable_standard_headers (nullable bool - uses pointer pattern like certificate_id)
if v, ok := payload["enable_standard_headers"]; ok {
    if v == nil {
        host.EnableStandardHeaders = nil  // Explicit null → use default behavior
    } else if b, ok := v.(bool); ok {
        host.EnableStandardHeaders = &b   // Explicit true/false
    }
}
```

#### 1.2 Add `forward_auth_enabled` Handler (Regular Bool)

```go
// Handle forward_auth_enabled (regular bool)
if v, ok := payload["forward_auth_enabled"].(bool); ok {
    host.ForwardAuthEnabled = v
}
```

#### 1.3 Add `waf_disabled` Handler (Regular Bool)

```go
// Handle waf_disabled (regular bool)
if v, ok := payload["waf_disabled"].(bool); ok {
    host.WAFDisabled = v
}
```

### Phase 2: Unit Tests

**Target File:** `backend/internal/api/handlers/proxy_host_handler_test.go`

#### 2.1 TestUpdate_EnableStandardHeaders

```go
func TestUpdate_EnableStandardHeaders(t *testing.T) {
    // Setup: Create host with enable_standard_headers = nil (default)
    // Test 1: PUT with enable_standard_headers: true → verify DB has true
    // Test 2: PUT with enable_standard_headers: false → verify DB has false
    // Test 3: PUT with enable_standard_headers: null → verify DB has nil
    // Test 4: PUT without field → verify value unchanged
}
```

#### 2.2 TestUpdate_ForwardAuthEnabled

```go
func TestUpdate_ForwardAuthEnabled(t *testing.T) {
    // Setup: Create host with forward_auth_enabled = false (default)
    // Test 1: PUT with forward_auth_enabled: true → verify DB has true
    // Test 2: PUT with forward_auth_enabled: false → verify DB has false
    // Test 3: PUT without field → verify value unchanged
}
```

#### 2.3 TestUpdate_WAFDisabled

```go
func TestUpdate_WAFDisabled(t *testing.T) {
    // Setup: Create host with waf_disabled = false (default)
    // Test 1: PUT with waf_disabled: true → verify DB has true
    // Test 2: PUT with waf_disabled: false → verify DB has false
    // Test 3: PUT without field → verify value unchanged
}
```

#### 2.4 Integration Test: Create → Update → Verify Caddy Config

```go
func TestUpdate_IntegrationCaddyConfig(t *testing.T) {
    // 1. Create proxy host with enable_standard_headers: false
    // 2. Verify Caddy config does NOT have X-Forwarded-* headers
    // 3. Update host with enable_standard_headers: true
    // 4. Verify Caddy config NOW has X-Real-IP, X-Forwarded-Proto, etc.
}
```

#### 2.5 Regression Test: Existing Hosts Without Fields

```go
func TestUpdate_ExistingHostsBackwardCompatibility(t *testing.T) {
    // Simulate existing host created before these fields existed
    // 1. Insert host directly into DB without these fields (NULL values)
    // 2. Perform GET → should return without error
    // 3. Perform PUT with unrelated field change → should succeed
    // 4. Verify all three fields remain at defaults
}
```

### Phase 3: Integration Verification

After implementation, run full integration tests:

```bash
# Run backend tests with coverage
scripts/go-test-coverage.sh

# Run integration tests
scripts/integration-test.sh
```

---

## Null Handling Specification

### `enable_standard_headers` (Nullable Bool - `*bool`)

This field follows the same pattern as `certificate_id`:

| JSON Value | Go Value | Database | Caddy Behavior |
|------------|----------|----------|----------------|
| `"enable_standard_headers": true` | `*bool → true` | `1` | Add X-Real-IP, X-Forwarded-* headers |
| `"enable_standard_headers": false` | `*bool → false` | `0` | No standard headers |
| `"enable_standard_headers": null` | `*bool → nil` | `NULL` | Use default (true for new hosts) |
| Field omitted | No change | Unchanged | Unchanged |

**Default Behavior:** When `nil`, treated as `true` in config generation:

```go
// From config.go line 338
enableStdHeaders := host.EnableStandardHeaders == nil || *host.EnableStandardHeaders
```

### `forward_auth_enabled` and `waf_disabled` (Regular Bool)

| JSON Value | Go Value | Database |
|------------|----------|----------|
| `"field": true` | `bool → true` | `1` |
| `"field": false` | `bool → false` | `0` |
| Field omitted | No change | Unchanged |

---

## Files to Modify

### Backend (Must Modify)

| File | Change |
|------|--------|
| [proxy_host_handler.go](../../backend/internal/api/handlers/proxy_host_handler.go) | Add handlers for all 3 missing fields |
| [proxy_host_handler_test.go](../../backend/internal/api/handlers/proxy_host_handler_test.go) | Add unit tests for all 3 fields |

### Backend (Verify Only - No Changes Expected)

| File | Verification |
|------|--------------|
| [proxy_host.go](../../backend/internal/models/proxy_host.go) | ✅ All 3 fields exist with correct GORM tags |
| [proxyhost_service.go](../../backend/internal/services/proxyhost_service.go) | ✅ Uses `Select("*")` - passes all fields to DB |
| [config.go](../../backend/internal/caddy/config.go) | ✅ Correctly uses `EnableStandardHeaders` |
| [types.go](../../backend/internal/caddy/types.go) | ✅ `ReverseProxyHandler` handles `enableStandardHeaders` param |

### Frontend (Verify Only - No Changes Expected)

| File | Verification |
|------|--------------|
| [proxyHosts.ts](../../frontend/src/api/proxyHosts.ts) | ⚠️ Missing `forward_auth_enabled` and `waf_disabled` in TypeScript interface |

---

## Verification Checklist

### Pre-Implementation

- [ ] Read and understand all 3 field definitions in `proxy_host.go`
- [ ] Confirm `certificate_id` pattern for nullable bool handling
- [ ] Review existing test patterns in `proxy_host_handler_test.go`

### Implementation

- [ ] Add `enable_standard_headers` handler (nullable bool pattern)
- [ ] Add `forward_auth_enabled` handler (regular bool)
- [ ] Add `waf_disabled` handler (regular bool)
- [ ] Write unit tests for all 3 fields
- [ ] Write integration test for Caddy config generation

### Post-Implementation Verification

- [ ] Database value persists after PUT request (all 3 fields)
- [ ] GET returns updated value (all 3 fields)
- [ ] Caddy config reflects `enable_standard_headers` change
- [ ] All existing tests pass (`go test ./...`)
- [ ] Coverage threshold maintained (≥85%)

### Regression Testing

- [ ] Existing hosts without these fields still work
- [ ] Create new host → verify default values
- [ ] Update existing host → verify partial update works
- [ ] Frontend form submission → backend processes correctly

---

## Root Cause Analysis

### Why These Fields Were Missed

The `Update` handler uses a `map[string]interface{}` payload pattern with manual field extraction. When new fields were added to the model, they were not added to the handler's extraction logic.

**Evidence from proxy_host_handler.go:**

```go
// Lines 177-220: Only these fields are handled
if v, ok := payload["name"].(string); ok { host.Name = v }
if v, ok := payload["domain_names"].(string); ok { host.DomainNames = v }
// ... more fields ...
if v, ok := payload["enabled"].(bool); ok { host.Enabled = v }
// ⚠️ GAP: enable_standard_headers, forward_auth_enabled, waf_disabled NOT HERE
```

### Model Definition (Correct)

**From proxy_host.go:**

```go
// Forward Auth - Line 37
ForwardAuthEnabled bool `json:"forward_auth_enabled" gorm:"default:false"`

// WAF override - Line 40
WAFDisabled bool `json:"waf_disabled" gorm:"default:false"`

// Standard headers - Line 54
EnableStandardHeaders *bool `json:"enable_standard_headers,omitempty" gorm:"default:true"`
```

---

## Complete Handler Field List

Fields handled in Update (proxy_host_handler.go):

**String Fields:**

- [x] name
- [x] domain_names
- [x] forward_scheme
- [x] forward_host
- [x] application
- [x] advanced_config

**Integer Fields:**

- [x] forward_port

**Boolean Fields:**

- [x] ssl_forced
- [x] http2_support
- [x] hsts_enabled
- [x] hsts_subdomains
- [x] block_exploits
- [x] websocket_support
- [x] enabled
- [ ] **forward_auth_enabled** ← MISSING
- [ ] **waf_disabled** ← MISSING

**Nullable Fields:**

- [x] certificate_id
- [x] access_list_id
- [x] security_header_profile_id
- [ ] **enable_standard_headers** ← MISSING

**Complex Fields:**

- [x] locations (JSON array)

---

## Frontend TypeScript Interface Gap

**Current interface in proxyHosts.ts:**

```typescript
export interface ProxyHost {
  // ... existing fields ...
  enable_standard_headers?: boolean;  // ✅ Exists
  // ⚠️ MISSING: forward_auth_enabled
  // ⚠️ MISSING: waf_disabled
}
```

**Recommendation:** After backend fix, update TypeScript interface:

```typescript
export interface ProxyHost {
  // ... existing fields ...
  enable_standard_headers?: boolean;
  forward_auth_enabled?: boolean;  // ADD
  waf_disabled?: boolean;          // ADD
}
```

---

## Recent Related Changes

### Commit: 81085ec (Dec 19, 2025)

**Title:** feat: add standard proxy headers with backward compatibility

This commit introduced the `enable_standard_headers` feature but **missed adding the handler code for updates**:

- Added field to model ✓
- Added UI checkbox ✓
- Modified config generation ✓
- **Missing: Update handler code** ✗

---

## Appendix: Caddy Config Generation

When `enable_standard_headers` is properly saved and set to `true`, the generated config includes:

```json
{
  "handler": "reverse_proxy",
  "headers": {
    "request": {
      "set": {
        "X-Real-IP": ["{http.request.remote.host}"],
        "X-Forwarded-Proto": ["{http.request.scheme}"],
        "X-Forwarded-Host": ["{http.request.host}"],
        "X-Forwarded-Port": ["{http.request.port}"]
      }
    }
  }
}
```

These headers are **required** for:

- Plex SSO authentication pass-through
- Seerr/Overseerr login via proxy
- Any app that needs client's real IP
- Proper protocol detection (HTTPS)
