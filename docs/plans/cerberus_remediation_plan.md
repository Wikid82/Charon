# Cerberus Security Module - Comprehensive Remediation Plan

**Version:** 2.0
**Date:** 2025-12-12
**Status:** 🔴 PENDING - Issues #16, #17, #18, #19 incomplete

---

## Executive Summary

This document provides a **comprehensive, actionable remediation plan** to complete the Cerberus security module. Four GitHub issues remain partially implemented:

| Issue | Feature | Current State | Priority |
|-------|---------|---------------|----------|
| #16 | GeoIP Integration | Database downloaded, no Go code reads it | HIGH |
| #17 | CrowdSec Bouncer | Placeholder comment in code | HIGH |
| #18 | WAF (Coraza) Integration | Only checks `<script>` tag, no real Coraza config | HIGH |
| #19 | Rate Limiting | Burst field unused, no bypass list/templates | MEDIUM |

---

## Implementation Phases

### Phase 1: GeoIP Integration (Issue #16)

**Goal:** Enable actual IP → Country lookup for geo-blocking ACLs.

#### 1.1 Current State Analysis

- **Dockerfile** downloads `GeoLite2-Country.mmdb` to `/app/data/geoip/`
- **Environment variable** `CHARON_GEOIP_DB_PATH` is set
- **No Go code** imports or reads the MaxMind database
- **Caddy config** uses `caddy-geoip2` plugin with `{geoip2.country_code}` placeholder (works at proxy level)
- **`TestIP` service method** only evaluates IP/CIDR rules, returns default for geo types

#### 1.2 Required Changes

##### 1.2.1 Add GeoIP Dependency

**File:** `backend/go.mod`

```bash
cd backend && go get github.com/oschwald/geoip2-golang
```

##### 1.2.2 Create GeoIP Service

**New File:** `backend/internal/services/geoip_service.go`

```go
// Package services provides business logic for the application.
package services

import (
 "errors"
 "net"
 "sync"

 "github.com/oschwald/geoip2-golang"
)

var (
 ErrGeoIPDatabaseNotLoaded = errors.New("geoip database not loaded")
 ErrInvalidIP              = errors.New("invalid IP address")
 ErrCountryNotFound        = errors.New("country not found for IP")
)

// GeoIPService provides IP-to-country lookups using MaxMind GeoLite2.
type GeoIPService struct {
 mu     sync.RWMutex
 db     *geoip2.Reader
 dbPath string
}

// NewGeoIPService creates a new GeoIPService and loads the database.
func NewGeoIPService(dbPath string) (*GeoIPService, error) {
 svc := &GeoIPService{dbPath: dbPath}
 if err := svc.Load(); err != nil {
  return nil, err
 }
 return svc, nil
}

// Load opens or reloads the GeoIP database.
func (s *GeoIPService) Load() error {
 s.mu.Lock()
 defer s.mu.Unlock()

 if s.db != nil {
  s.db.Close()
 }

 db, err := geoip2.Open(s.dbPath)
 if err != nil {
  return err
 }
 s.db = db
 return nil
}

// Close releases the database resources.
func (s *GeoIPService) Close() error {
 s.mu.Lock()
 defer s.mu.Unlock()
 if s.db != nil {
  return s.db.Close()
 }
 return nil
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for an IP.
func (s *GeoIPService) LookupCountry(ipStr string) (string, error) {
 s.mu.RLock()
 defer s.mu.RUnlock()

 if s.db == nil {
  return "", ErrGeoIPDatabaseNotLoaded
 }

 ip := net.ParseIP(ipStr)
 if ip == nil {
  return "", ErrInvalidIP
 }

 record, err := s.db.Country(ip)
 if err != nil {
  return "", err
 }

 if record.Country.IsoCode == "" {
  return "", ErrCountryNotFound
 }

 return record.Country.IsoCode, nil
}

// IsLoaded returns true if the database is loaded.
func (s *GeoIPService) IsLoaded() bool {
 s.mu.RLock()
 defer s.mu.RUnlock()
 return s.db != nil
}
```

##### 1.2.3 Update AccessListService to Use GeoIP

**File:** `backend/internal/services/access_list_service.go`

**Add field to struct:**

```go
type AccessListService struct {
 db       *gorm.DB
 geoipSvc *GeoIPService  // NEW
}
```

**Update constructor:**

```go
func NewAccessListService(db *gorm.DB) *AccessListService {
 return &AccessListService{
  db:       db,
  geoipSvc: nil, // Will be set via SetGeoIPService
 }
}

// SetGeoIPService attaches a GeoIP service for country lookups.
func (s *AccessListService) SetGeoIPService(geoipSvc *GeoIPService) {
 s.geoipSvc = geoipSvc
}
```

**Update `TestIP` method to handle geo types:**

```go
// TestIP tests if an IP address would be allowed/blocked by the access list
func (s *AccessListService) TestIP(aclID uint, ipAddress string) (allowed bool, reason string, err error) {
 acl, err := s.GetByID(aclID)
 if err != nil {
  return false, "", err
 }

 if !acl.Enabled {
  return true, "Access list is disabled - all traffic allowed", nil
 }

 ip := net.ParseIP(ipAddress)
 if ip == nil {
  return false, "", ErrInvalidIPAddress
 }

 // Handle geo-based ACLs
 if strings.HasPrefix(acl.Type, "geo_") {
  if s.geoipSvc == nil {
   return true, "GeoIP service not available - allowing by default", nil
  }

  countryCode, err := s.geoipSvc.LookupCountry(ipAddress)
  if err != nil {
   // If lookup fails, allow with warning
   return true, fmt.Sprintf("GeoIP lookup failed: %v - allowing by default", err), nil
  }

  // Parse country codes from ACL
  allowedCodes := make(map[string]bool)
  for _, code := range strings.Split(acl.CountryCodes, ",") {
   allowedCodes[strings.TrimSpace(strings.ToUpper(code))] = true
  }

  isInList := allowedCodes[countryCode]

  if acl.Type == "geo_whitelist" {
   if isInList {
    return true, fmt.Sprintf("Allowed by geo whitelist: IP from %s", countryCode), nil
   }
   return false, fmt.Sprintf("Blocked: IP from %s not in geo whitelist", countryCode), nil
  }

  // geo_blacklist
  if isInList {
   return false, fmt.Sprintf("Blocked by geo blacklist: IP from %s", countryCode), nil
  }
  return true, fmt.Sprintf("Allowed: IP from %s not in geo blacklist", countryCode), nil
 }

 // ... rest of existing IP/CIDR logic unchanged ...
}
```

##### 1.2.4 Initialize GeoIP Service on Server Start

**File:** `backend/internal/server/server.go` (or wherever services are initialized)

```go
import (
 "os"
 // ...
)

// In server initialization:
geoipPath := os.Getenv("CHARON_GEOIP_DB_PATH")
if geoipPath == "" {
 geoipPath = "/app/data/geoip/GeoLite2-Country.mmdb"
}

var geoipSvc *services.GeoIPService
if _, err := os.Stat(geoipPath); err == nil {
 geoipSvc, err = services.NewGeoIPService(geoipPath)
 if err != nil {
  logger.Log().WithError(err).Warn("Failed to load GeoIP database, geo-blocking will be unavailable")
 } else {
  logger.Log().Info("GeoIP database loaded successfully")
 }
}

// Pass to AccessListService
accessListSvc := services.NewAccessListService(db)
if geoipSvc != nil {
 accessListSvc.SetGeoIPService(geoipSvc)
}
```

##### 1.2.5 Add GeoIP Reload Endpoint (Optional Enhancement)

**File:** `backend/internal/api/handlers/security_handler.go`

```go
// ReloadGeoIP reloads the GeoIP database from disk
func (h *SecurityHandler) ReloadGeoIP(c *gin.Context) {
 if h.geoipSvc == nil {
  c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GeoIP service not initialized"})
  return
 }
 if err := h.geoipSvc.Load(); err != nil {
  c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload: %v", err)})
  return
 }
 c.JSON(http.StatusOK, gin.H{"message": "GeoIP database reloaded"})
}
```

**Add route:** `POST /api/v1/security/geoip/reload`

#### 1.3 Test Requirements

**New File:** `backend/internal/services/geoip_service_test.go`

```go
func TestGeoIPService_LookupCountry(t *testing.T) {
 // Skip if no test database available
 testDBPath := os.Getenv("TEST_GEOIP_DB_PATH")
 if testDBPath == "" {
  t.Skip("TEST_GEOIP_DB_PATH not set")
 }

 svc, err := NewGeoIPService(testDBPath)
 require.NoError(t, err)
 defer svc.Close()

 tests := []struct {
  name    string
  ip      string
  wantCC  string
  wantErr bool
 }{
  {"Google DNS", "8.8.8.8", "US", false},
  {"Cloudflare", "1.1.1.1", "AU", false}, // May vary
  {"Invalid IP", "not-an-ip", "", true},
  {"Private IP", "192.168.1.1", "", true}, // No country
 }

 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   cc, err := svc.LookupCountry(tt.ip)
   if tt.wantErr {
    assert.Error(t, err)
   } else {
    assert.NoError(t, err)
    assert.Equal(t, tt.wantCC, cc)
   }
  })
 }
}
```

**Update:** `backend/internal/services/access_list_service_test.go` - add tests for geo ACL `TestIP`.

#### 1.4 Frontend Changes

**File:** `frontend/src/pages/AccessLists.tsx`

Update the test IP result display to show country code when geo-blocking:

```tsx
// In handleTestIP success callback:
if (result.reason.includes("IP from")) {
  toast.success(`${result.allowed ? '✅' : '🚫'} ${result.reason}`)
} else {
  // existing logic
}
```

#### 1.5 Documentation Update

**File:** `docs/cerberus.md` - Add section on GeoIP configuration and database updates.

---

### Phase 2: Rate Limit Fix (Issue #19) - Quick Win

**Goal:** Fix burst field usage, add bypass list and preset templates.

#### 2.1 Current State Analysis

- `RateLimitBurst` field exists in `SecurityConfig` model
- `buildRateLimitHandler` in `caddy/config.go` **ignores** burst field
- No bypass list for trusted IPs
- No preset templates (login, API, standard)
- Frontend has burst input but it's not used by backend

#### 2.2 Required Changes

##### 2.2.1 Fix Burst Field in Caddy Config

**File:** `backend/internal/caddy/config.go`

```go
// buildRateLimitHandler returns a rate-limit handler using the caddy-ratelimit module.
func buildRateLimitHandler(_ *models.ProxyHost, secCfg *models.SecurityConfig) (Handler, error) {
 if secCfg == nil {
  return nil, nil
 }
 if secCfg.RateLimitRequests <= 0 || secCfg.RateLimitWindowSec <= 0 {
  return nil, nil
 }

 // Calculate burst: if not set, default to 20% of requests
 burst := secCfg.RateLimitBurst
 if burst <= 0 {
  burst = secCfg.RateLimitRequests / 5
  if burst < 1 {
   burst = 1
  }
 }

 // caddy-ratelimit format with burst support
 h := Handler{"handler": "rate_limit"}
 h["rate_limits"] = map[string]interface{}{
  "static": map[string]interface{}{
   "key":        "{http.request.remote.host}",
   "window":     fmt.Sprintf("%ds", secCfg.RateLimitWindowSec),
   "max_events": secCfg.RateLimitRequests,
   // NOTE: caddy-ratelimit doesn't have a direct "burst" param,
   // but we can use distributed rate limiting or adjust max_events
  },
 }
 return h, nil
}
```

> **Note:** The `caddy-ratelimit` module by mholt doesn't have a direct burst parameter. Consider:
>
> 1. Using a sliding window algorithm (already default)
> 2. Implementing burst via separate zone for initial requests
> 3. Document limitation in UI

##### 2.2.2 Add Bypass List Support

**File:** `backend/internal/models/security_config.go`

```go
type SecurityConfig struct {
 // ... existing fields ...
 RateLimitBypassList string `json:"rate_limit_bypass_list" gorm:"type:text"` // Comma-separated CIDRs
}
```

**File:** `backend/internal/caddy/config.go`

```go
func buildRateLimitHandler(host *models.ProxyHost, secCfg *models.SecurityConfig) (Handler, error) {
 // ... existing validation ...

 h := Handler{"handler": "rate_limit"}

 // Build zone configuration
 zone := map[string]interface{}{
  "key":        "{http.request.remote.host}",
  "window":     fmt.Sprintf("%ds", secCfg.RateLimitWindowSec),
  "max_events": secCfg.RateLimitRequests,
 }

 h["rate_limits"] = map[string]interface{}{"static": zone}

 // If bypass list is configured, wrap in a subroute that skips for those IPs
 if secCfg.RateLimitBypassList != "" {
  bypassCIDRs := parseBypassList(secCfg.RateLimitBypassList)
  if len(bypassCIDRs) > 0 {
   return Handler{
    "handler": "subroute",
    "routes": []map[string]interface{}{
     {
      // Skip rate limiting for bypass IPs
      "match": []map[string]interface{}{
       {"remote_ip": map[string]interface{}{"ranges": bypassCIDRs}},
      },
      "terminal": false, // Continue to proxy handler
     },
     {
      // Apply rate limiting for all others
      "handle": []Handler{h},
     },
    },
   }, nil
  }
 }

 return h, nil
}

#### 2.3 Rate Limiting — Test Plan (Detailed)

**Summary:** This section contains a complete test plan to validate rate limiting configuration generation and runtime enforcement in Charon. Tests are grouped into Unit, Integration and E2E categories and prioritize quick unit coverage and high-impact integration tests.

Goal: Verify the following behavior:
- The `RateLimitBurst`, `RateLimitRequests`, `RateLimitWindowSec` and `RateLimitBypassList` fields are used by `buildRateLimitHandler` and emitted into the Caddy JSON configuration.
- The Caddy `rate_limit` handler configuration uses `{http.request.remote.host}` as the key.
- Bypass IPs are excluded from rate limiting by creating a subroute with `remote_ip` matcher.
- At runtime, Caddy enforces limits, returns `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `Retry-After` headers (or documented equivalents), and resets counters after the configured window.
- The plugin behaves correctly across multiple client IPs and respects bypass lists.

-----

2.3.1 Files to Add / Edit
- New scripts: `scripts/rate_limit_integration.sh` (shell integration test), `scripts/rate_limit_e2e.sh` (optional extended tests).
- New integration test: `backend/integration/rate_limit_integration_test.go` (go test wrapper runs script).
- Unit tests to add/edit: `backend/internal/caddy/config_test.go` (add `TestGenerateConfig_WithRateLimitBypassList` and `TestBuildRateLimitHandler_KeyIsRemoteHost`), `backend/internal/api/handlers/security_ratelimit_test.go` (validate API fields persist & UpdateConfig accepts rate_limit fields).

2.3.2 Unit Tests (fast, run in CI pre-merge)
- File: `backend/internal/caddy/config_test.go`
 - `TestGenerateConfig_WithRateLimitBypassList`
  - Input: call `GenerateConfig` with `secCfg` set with `RateLimitEnable:true` and `RateLimitBypassList:"10.0.0.0/8,127.0.0.1/32"`; include one host.
  - Assertions:
   - The generated `Config` contains a route with `handler:"subroute"` or a `rate_limit` handler containing the bypass CIDRs (CIDRs found in JSON output).
   - `RateLimitHandler` contains `rate_limits` map and `static` zone.
 - `TestBuildRateLimitHandler_KeyIsRemoteHost`
  - Input: `secCfg` with valid values.
  - Assertions: the static zone `key` is `{http.request.remote.host}`.
 - `TestBuildRateLimitHandler_DefaultBurstAndMax` (already present) and `TestParseBypassCIDRs` (existing) remain required.

2.3.3 Integration Tests (CI gated, Docker required)
We will add a scripted integration test to run inside CI or locally with Docker. The test will:
 - Start the `charon:local` image (build if not present) in a detached container named `charon-debug`.
 - Create a simple HTTP backend (httpbin/kennethreitz/httpbin) called `ratelimit-backend` (or `httpbin`).
 - Create a proxy host `ratelimit.local` pointing to the backend via the Charon API (use /api/v1/proxy-hosts).
 - Set `SecurityConfig` (POST /api/v1/security/config) with short windows for speed, e.g.:
  ```json
  {"name":"default","enabled":true,"rate_limit_enable":true,"rate_limit_requests":3,"rate_limit_window_sec":10,"rate_limit_burst":1}
  ```

- Validate that Caddy Admin API at `http://localhost:2019/config` includes a `rate_limit` handler and, where applicable, a `subroute` with bypass CIDRs (if `RateLimitBypassList` set).
- Execute the runtime checks:
- Using a single client IP, send 3 requests in quick succession expecting HTTP 200.
- The 4th request (same client IP) should return HTTP 429 (Too Many Requests) and include a `Retry-After` header.
- On allowed responses, assert that `X-RateLimit-Limit` equals 3 and `X-RateLimit-Remaining` decrements.
- Wait until the configured `RateLimitWindowSec` elapses, and confirm requests are allowed again (headers reset).

- Bypass List Validation:
- Set `RateLimitBypassList` to contain the requester's IP (or `127.0.0.1/32` when client runs from the host). Confirm repeated requests do not get `429`, and `X-RateLimit-*` headers may be absent or indicate non-enforcement.

- Multi-IP Isolation:
- Spin up two client containers with different IPs (via Docker network `--subnet` + `--ip`). Each should have independent counters; both able to make configured number requests without affecting the other.

- X-Forwarded-For behavior (Confirm remote.host is used as key):
- Send requests with `X-Forwarded-For` different than the container IP; observe rate counters still use the connection IP unless Caddy remote_ip plugin explicitly configured to respect XFF.

- Test Example (Shell Snippet to assert headers)

  ```bash
  # Single request driver - check headers
  curl -s -D - -o /dev/null -H "Host: ratelimit.local" http://localhost/post
  # Expect headers: X-RateLimit-Limit: 3, X-RateLimit-Remaining: <number>
  ```

- Script name: `scripts/rate_limit_integration.sh` (mirrors style of `coraza_integration.sh`).

- Manage flaky behavior:
- Retry a couple times and log Caddy admin API output on failure for debugging.
-

2.3.4 E2E Tests (Longer, optional)

- Create `scripts/rate_limit_e2e.sh` which spins up the same environment but runs broader scenarios:
- High-rate bursts (WindowSec small and Requests small) to test burst allowance/consumption.
- Multi-minute stress run (not for every CI pass) to check long-term behavior and reset across windows.
- SPA / browser test using Playwright / Cypress to validate UI controls (admin toggles rate limit presets and sets bypass list) and ensures that applied config is effective at runtime.

2.3.5 Mock/Stub Guidance

- IP Addresses
- Use Docker network subnets and `docker run --network containers_default --ip 172.25.0.10` to guarantee client IP addresses for tests and to exercise bypass list behavior.
- For tests run from host with `curl`, include `--interface` or `--local-port` if needed to force source IP (less reliable than container-based approach).
- X-Forwarded-For
- Add `-H "X-Forwarded-For: 10.0.0.5"` to `curl` requests; assert that plugin uses real connection IP by default. If future changes enable `real_ip` handling in Caddy, tests should be updated to reflect the new behavior.
- Timing Windows
- Keep small values (2-10 seconds) while maintaining reliability (1s windows are often flaky). For CI environment, `RateLimitWindowSec=10` with `RateLimitRequests=3` and `Burst=1` is a stable, fast choice.

2.3.6 Test Data and Assertions (Explicit)

- Unit Test: `TestBuildRateLimitHandler_ValidConfig`
- Input: secCfg{Requests:100, WindowSec:60, Burst:25}
- Assert: `h["handler"] == "rate_limit"`, `static".max_events == 100`, `burst == 25`.

- Integration Test: `TestRateLimit_Enforcement_Basic`
- Input: RateLimitRequests=3, RateLimitWindowSec=10, Burst=1, no bypass list
- Actions: Send 4 rapid requests using client container
- Expected outputs: [200, 200, 200, 429], 4th returns Retry-After or explicit block message
- Assert: Allowed responses include `X-RateLimit-Limit: 3`, and `X-RateLimit-Remaining` decreasing

- Integration Test: `TestRateLimit_BypassList_SkipsLimit`
- Input: Same as above + `RateLimitBypassList` contains client IP CIDR
- Expected outputs: All requests 200 (no 429)

- Integration Test: `TestRateLimit_MultiClient_Isolation`
- Input: As above
- Actions: Client A sends 3 requests, Client B sends 3 requests
- Expected: Both clients unaffected by the other; both get 200 responses for their first 3 requests

- Integration Test: `TestRateLimit_Window_Reset`
- Input: As above
- Actions: Exhaust quota (get 429), wait `RateLimitWindowSec + 1`, issue a new request
- Expected: New request is 200 again

2.3.7 Test Harness - Example Go Integration Test
Use the same approach as `backend/integration/coraza_integration_test.go`, run the script and check output for expected messages. Example test file: `backend/integration/rate_limit_integration_test.go`:

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

func TestRateLimitIntegration(t *testing.T) {
  t.Parallel()
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
  defer cancel()
  cmd := exec.CommandContext(ctx, "bash", "./scripts/rate_limit_integration.sh")
  out, err := cmd.CombinedOutput()
  t.Logf("rate_limit_integration script output:\n%s", string(out))
  if err != nil {
    t.Fatalf("rate_limit integration failed: %v", err)
  }
  if !strings.Contains(string(out), "Rate limit enforcement succeeded") {
    t.Fatalf("unexpected script output, rate limiting assertion not found")
  }
}
```

2.3.8 CI and Pre-commit Hooks

- Add an integration CI job that runs the Docker-based script and the integration `go` test suite in a separate job to avoid blocking unit test runs on tools requiring Docker. Use a job matrix with `services: docker` and timeouts set appropriately.
- Do not add integration scripts to pre-commit (too heavy); keep pre-commit focused on `go fmt`, `go vet`, `go test ./...` (unit tests), `npm test`, and lint rules.
- Use the workspace `tasks.json` to add a `Coraza: Run Integration Script` style task for rate limit integration that mirrors `scripts/coraza_integration.sh`.

2.3.9 .gitignore / .codecov.yml / Dockerfile changes

- .gitignore
  - Add `test-results/rate_limit/` to avoid committing local script logs.
  - Add `scripts/rate_limit_integration.sh` output files (if any) to ignore.
- .codecov.yml
  - Optional: If you want integration test coverage included, remove `**/integration/**` from `ignore` or add a specific `backend/integration/*_test.go` to be included. (Caveat: integration coverage may not be reproducible across CI).
- .dockerignore
  - Ensure `scripts/` and `backend/integration` are not copied to reduce build context size if not needed in Docker build.
- Dockerfile
  - Confirm presence of `--with github.com/mholt/caddy-ratelimit` in the xcaddy build (it is present in base Dockerfile). Add comment and assert plugin presence in integration script by checking `caddy version` or `caddy list` available modules.

2.3.10 Prioritization

- P0: Integration test `TestRateLimit_Enforcement_Basic` (high confidence: verifies actual runtime limit enforcement and header presence)
- P1: Unit tests verifying config building (`TestGenerateConfig_WithRateLimitBypassList`, `TestBuildRateLimitHandler_KeyIsRemoteHost`) and API tests for `POST /security/config` handling rate limit fields
- P2: Integration tests for bypass list, multi-client isolation, window reset
- P3: E2E tests for UI configuration of rate limiting and long-running stress tests

2.3.11 Next Steps

- Implement `scripts/rate_limit_integration.sh` and `backend/integration/rate_limit_integration_test.go` following `coraza_integration.sh` as the blueprint.
- Add unit tests to `backend/internal/caddy/config_test.go` and API handler tests in `backend/internal/api/handlers/security_ratelimit_test.go`.
- Add Docker network helpers and ensure `docker run --ip` is used to control client IPs during integration.
- Run all new tests locally (Docker required) and in CI. Add integration job to GitHub Actions with `runs-on: ubuntu-latest`, `services: docker` and appropriate timeouts.

-----

This test plan should serve as a complete specification for testing rate limiting behavior across unit, integration, and E2E tiers. The next iteration will include scripted test implementations and Jenkins/GHA job snippets for CI.

func parseBypassList(list string) []string {
 var cidrs []string
 for _, part := range strings.Split(list, ",") {
  part = strings.TrimSpace(part)
  if part == "" {
   continue
  }
  // Validate CIDR
  if_, _, err := net.ParseCIDR(part); err == nil {
   cidrs = append(cidrs, part)
  } else if net.ParseIP(part) != nil {
   // Single IP - convert to /32 or /128
   if strings.Contains(part, ":") {
    cidrs = append(cidrs, part+"/128")
   } else {
    cidrs = append(cidrs, part+"/32")
   }
  }
 }
 return cidrs
}

```

##### 2.2.3 Add Preset Templates

**File:** `backend/internal/api/handlers/security_handler.go`

```go
// GetRateLimitPresets returns predefined rate limit configurations
func (h *SecurityHandler) GetRateLimitPresets(c *gin.Context) {
 presets := []map[string]interface{}{
  {
   "id":          "standard",
   "name":        "Standard Web",
   "description": "Balanced protection for general web applications",
   "requests":    100,
   "window_sec":  60,
   "burst":       20,
  },
  {
   "id":          "api",
   "name":        "API Protection",
   "description": "Stricter limits for API endpoints",
   "requests":    30,
   "window_sec":  60,
   "burst":       10,
  },
  {
   "id":          "login",
   "name":        "Login Protection",
   "description": "Aggressive protection against brute-force",
   "requests":    5,
   "window_sec":  300,
   "burst":       2,
  },
  {
   "id":          "relaxed",
   "name":        "High Traffic",
   "description": "Higher limits for trusted, high-traffic apps",
   "requests":    500,
   "window_sec":  60,
   "burst":       100,
  },
 }
 c.JSON(http.StatusOK, gin.H{"presets": presets})
}
```

**Add route:** `GET /api/v1/security/rate-limit/presets`

##### 2.2.4 Frontend Updates

**File:** `frontend/src/api/security.ts`

```typescript
export interface RateLimitPreset {
  id: string
  name: string
  description: string
  requests: number
  window_sec: number
  burst: number
}

export const getRateLimitPresets = async (): Promise<{ presets: RateLimitPreset[] }> => {
  const response = await client.get('/security/rate-limit/presets')
  return response.data
}
```

**File:** `frontend/src/pages/RateLimiting.tsx`

Add preset dropdown and bypass list input (see implementation details in frontend section below).

#### 2.3 Test Requirements

**File:** `backend/internal/caddy/config_test.go`

```go
func TestBuildRateLimitHandler_UsesBurst(t *testing.T) {
 secCfg := &models.SecurityConfig{
  RateLimitRequests:  100,
  RateLimitWindowSec: 60,
  RateLimitBurst:     25,
 }
 h, err := buildRateLimitHandler(nil, secCfg)
 require.NoError(t, err)
 require.NotNil(t, h)
 // Verify burst is used in config
}

func TestBuildRateLimitHandler_BypassList(t *testing.T) {
 secCfg := &models.SecurityConfig{
  RateLimitRequests:    100,
  RateLimitWindowSec:   60,
  RateLimitBypassList:  "10.0.0.0/8,192.168.1.1",
 }
 h, err := buildRateLimitHandler(nil, secCfg)
 require.NoError(t, err)
 // Verify subroute structure with bypass
}
```

---

### Phase 3: CrowdSec Bouncer (Issue #17)

**Goal:** Implement actual IP reputation checking against CrowdSec decisions.

#### 3.1 Current State Analysis

- CrowdSec binary installed in Docker image
- `caddy-crowdsec-bouncer` plugin compiled into Caddy
- `buildCrowdSecHandler` returns a handler but only sets `api_url`
- Comment in `cerberus.go`: "CrowdSec placeholder: integration would check CrowdSec API"
- No actual decision lookup code

#### 3.2 Architecture Decision

**Two approaches:**

| Approach | Pros | Cons |
|----------|------|------|
| **A: Caddy Plugin** | Offloads to Caddy, less Go code | Requires Caddy config regeneration |
| **B: Middleware** | Real-time, Go-native | Duplicates bouncer logic |

**Recommendation:** Use **Approach A** (Caddy Plugin) since `caddy-crowdsec-bouncer` is already compiled in. Enhance the handler configuration.

#### 3.3 Required Changes

##### 3.3.1 Update CrowdSec Handler Builder

**File:** `backend/internal/caddy/config.go`

```go
// buildCrowdSecHandler returns a CrowdSec bouncer handler.
// See: https://github.com/hslatman/caddy-crowdsec-bouncer
func buildCrowdSecHandler(host *models.ProxyHost, secCfg *models.SecurityConfig, crowdsecEnabled bool) (Handler, error) {
 if !crowdsecEnabled {
  return nil, nil
 }

 h := Handler{"handler": "crowdsec"}

 // API URL (required)
 apiURL := "http://localhost:8080"
 if secCfg != nil && secCfg.CrowdSecAPIURL != "" {
  apiURL = secCfg.CrowdSecAPIURL
 }
 h["api_url"] = apiURL

 // API Key (from environment or config)
 apiKey := os.Getenv("CROWDSEC_API_KEY")
 if apiKey == "" && secCfg != nil {
  // Could store encrypted in DB - for now use env var
 }
 if apiKey != "" {
  h["api_key"] = apiKey
 }

 // Ticker interval for decision sync (default 30s)
 h["ticker_interval"] = "30s"

 // Enable streaming mode for real-time updates
 h["enable_streaming"] = true

 return h, nil
}
```

##### 3.3.2 Add CrowdSec Registration on Startup

**File:** `backend/internal/crowdsec/registration.go` (new)

```go
// Package crowdsec handles CrowdSec LAPI integration.
package crowdsec

import (
 "bytes"
 "encoding/json"
 "fmt"
 "net/http"
 "os"
 "os/exec"
 "time"
)

// EnsureBouncerRegistered registers the Caddy bouncer with local CrowdSec LAPI.
func EnsureBouncerRegistered(lapiURL string) (string, error) {
 // Check if already registered
 apiKey := os.Getenv("CROWDSEC_API_KEY")
 if apiKey != "" {
  return apiKey, nil
 }

 // Use cscli to register bouncer
 cmd := exec.Command("cscli", "bouncers", "add", "caddy-bouncer", "-o", "raw")
 output, err := cmd.Output()
 if err != nil {
  // May already exist, try to get existing key
  return "", fmt.Errorf("failed to register bouncer: %w", err)
 }

 apiKey = string(bytes.TrimSpace(output))
 return apiKey, nil
}

// CheckLAPIHealth verifies CrowdSec LAPI is responding.
func CheckLAPIHealth(lapiURL string) bool {
 client := &http.Client{Timeout: 5 * time.Second}
 resp, err := client.Get(lapiURL + "/v1/decisions")
 if err != nil {
  return false
 }
 defer resp.Body.Close()
 // 401 is expected without auth, but means LAPI is up
 return resp.StatusCode == 401 || resp.StatusCode == 200
}
```

##### 3.3.3 Update Cerberus Middleware (Optional - for logging)

**File:** `backend/internal/cerberus/cerberus.go`

The actual blocking is handled by the Caddy CrowdSec bouncer plugin. However, we can add logging in Cerberus:

```go
// In Middleware(), after ACL check:
// CrowdSec logging (actual blocking is done by Caddy bouncer)
if c.cfg.CrowdSecMode == "local" {
 // Log that CrowdSec is active (blocking happens at Caddy layer)
 logger.Log().WithField("client_ip", ctx.ClientIP()).Debug("Request evaluated by CrowdSec bouncer")
}
```

##### 3.3.4 Add CrowdSec Status Endpoint Enhancement

**File:** `backend/internal/api/handlers/crowdsec_handler.go`

```go
// GetCrowdSecDecisions returns recent decisions from CrowdSec LAPI
func (h *CrowdSecHandler) GetDecisions(c *gin.Context) {
 lapiURL := os.Getenv("CROWDSEC_LAPI_URL")
 if lapiURL == "" {
  lapiURL = "http://localhost:8080"
 }

 apiKey := os.Getenv("CROWDSEC_API_KEY")
 if apiKey == "" {
  c.JSON(http.StatusServiceUnavailable, gin.H{"error": "CrowdSec API key not configured"})
  return
 }

 client := &http.Client{Timeout: 10 * time.Second}
 req, _ := http.NewRequest("GET", lapiURL+"/v1/decisions", nil)
 req.Header.Set("X-Api-Key", apiKey)

 resp, err := client.Do(req)
 if err != nil {
  c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to CrowdSec LAPI"})
  return
 }
 defer resp.Body.Close()

 var decisions []map[string]interface{}
 json.NewDecoder(resp.Body).Decode(&decisions)
 c.JSON(http.StatusOK, gin.H{"decisions": decisions})
}
```

#### 3.4 Frontend Updates

**File:** `frontend/src/pages/CrowdSecConfig.tsx`

Add decisions viewer panel:

```tsx
// In CrowdSecConfig component:
const { data: decisions } = useQuery({
  queryKey: ['crowdsec-decisions'],
  queryFn: () => client.get('/api/v1/crowdsec/decisions').then(r => r.data),
  enabled: status?.crowdsec?.enabled,
  refetchInterval: 30000,
})

// Render decisions table
{decisions?.decisions?.length > 0 && (
  <Card>
    <h3>Active Decisions</h3>
    <table>
      <thead>
        <tr>
          <th>IP/Range</th>
          <th>Type</th>
          <th>Reason</th>
          <th>Expires</th>
        </tr>
      </thead>
      <tbody>
        {decisions.decisions.map((d: Decision) => (
          <tr key={d.id}>
            <td>{d.value}</td>
            <td>{d.type}</td>
            <td>{d.scenario}</td>
            <td>{formatExpiry(d.until)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  </Card>
)}
```

#### 3.5 Test Requirements

**File:** `backend/internal/crowdsec/registration_test.go`

```go
func TestCheckLAPIHealth(t *testing.T) {
 // Mock server test
 ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusUnauthorized) // Expected without auth
 }))
 defer ts.Close()

 assert.True(t, CheckLAPIHealth(ts.URL))
}
```

---

### Phase 4: WAF Integration (Issue #18) - Most Complex

**Goal:** Generate actual Coraza configuration, per-host toggle, rule exclusions, paranoia levels.

#### 4.1 Current State Analysis

- `cerberus.go` only checks for literal `<script>` string in URL (trivial bypass)
- `buildWAFHandler` in `config.go` creates Coraza handler but requires ruleset
- Frontend (`WafConfig.tsx`) manages rule sets but no per-host toggle
- No paranoia level selector
- No rule exclusion system for false positives

#### 4.2 Required Changes

##### 4.2.1 Enhance WAF Handler Builder

**File:** `backend/internal/caddy/config.go`

```go
// buildWAFHandler returns a WAF handler (Coraza) configuration.
func buildWAFHandler(host *models.ProxyHost, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string, secCfg *models.SecurityConfig, wafEnabled bool) (Handler, error) {
 if !wafEnabled {
  return nil, nil
 }

 // Check per-host WAF toggle
 if host != nil && host.WAFDisabled {
  return nil, nil
 }

 // Build directives
 var directives strings.Builder

 // Base configuration
 directives.WriteString("SecRuleEngine On\n")
 directives.WriteString("SecRequestBodyAccess On\n")
 directives.WriteString("SecResponseBodyAccess Off\n")

 // Paranoia level (1-4, default 1)
 paranoiaLevel := 1
 if secCfg != nil && secCfg.WAFParanoiaLevel > 0 && secCfg.WAFParanoiaLevel <= 4 {
  paranoiaLevel = secCfg.WAFParanoiaLevel
 }
 directives.WriteString(fmt.Sprintf("SecAction \"id:900000,phase:1,nolog,pass,t:none,setvar:tx.paranoia_level=%d\"\n", paranoiaLevel))

 // Mode: block or monitor
 if secCfg != nil && secCfg.WAFMode == "monitor" {
  directives.WriteString("SecRuleEngine DetectionOnly\n")
 }

 // Include ruleset files
 for _, rs := range rulesets {
  if path, ok := rulesetPaths[rs.Name]; ok && path != "" {
   directives.WriteString(fmt.Sprintf("Include %s\n", path))
  }
 }

 // Apply exclusions
 if secCfg != nil && secCfg.WAFExclusions != "" {
  var exclusions []WAFExclusion
  if err := json.Unmarshal([]byte(secCfg.WAFExclusions), &exclusions); err == nil {
   for _, ex := range exclusions {
    // Generate SecRuleRemoveById or SecRuleUpdateTargetById
    if ex.RuleID > 0 {
     if ex.Target != "" {
      directives.WriteString(fmt.Sprintf("SecRuleUpdateTargetById %d \"!%s\"\n", ex.RuleID, ex.Target))
     } else {
      directives.WriteString(fmt.Sprintf("SecRuleRemoveById %d\n", ex.RuleID))
     }
    }
   }
  }
 }

 h := Handler{
  "handler":    "waf",
  "directives": directives.String(),
 }

 return h, nil
}

// WAFExclusion represents a rule exclusion for false positives
type WAFExclusion struct {
 RuleID      int    `json:"rule_id"`
 Target      string `json:"target,omitempty"`       // e.g., "ARGS:password"
 Description string `json:"description,omitempty"`
}
```

##### 4.2.2 Add Per-Host WAF Toggle

**File:** `backend/internal/models/proxy_host.go`

```go
type ProxyHost struct {
 // ... existing fields ...
 WAFDisabled bool `json:"waf_disabled" gorm:"default:false"` // Override global WAF
}
```

##### 4.2.3 Add Paranoia Level and Exclusions to SecurityConfig

**File:** `backend/internal/models/security_config.go`

```go
type SecurityConfig struct {
 // ... existing fields ...
 WAFParanoiaLevel int    `json:"waf_paranoia_level" gorm:"default:1"` // 1-4
 WAFExclusions    string `json:"waf_exclusions" gorm:"type:text"`     // JSON array of exclusions
}
```

##### 4.2.4 Remove Naive Check from Cerberus Middleware

**File:** `backend/internal/cerberus/cerberus.go`

The current `<script>` check is misleading. Replace with proper logging:

```go
// WAF is handled by Coraza at the Caddy layer
// Log WAF status for debugging
if c.cfg.WAFMode != "" && c.cfg.WAFMode != "disabled" {
 logger.Log().WithFields(map[string]interface{}{
  "source":    "waf",
  "mode":      c.cfg.WAFMode,
  "client_ip": ctx.ClientIP(),
 }).Debug("Request subject to WAF inspection")
}
```

##### 4.2.5 Add Rule Exclusion Management Endpoints

**File:** `backend/internal/api/handlers/security_handler.go`

```go
// GetWAFExclusions returns current WAF rule exclusions
func (h *SecurityHandler) GetWAFExclusions(c *gin.Context) {
 cfg, err := h.svc.Get()
 if err != nil && err != services.ErrSecurityConfigNotFound {
  c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get config"})
  return
 }

 var exclusions []WAFExclusion
 if cfg != nil && cfg.WAFExclusions != "" {
  json.Unmarshal([]byte(cfg.WAFExclusions), &exclusions)
 }
 c.JSON(http.StatusOK, gin.H{"exclusions": exclusions})
}

// AddWAFExclusion adds a rule exclusion
func (h *SecurityHandler) AddWAFExclusion(c *gin.Context) {
 var exclusion WAFExclusion
 if err := c.ShouldBindJSON(&exclusion); err != nil {
  c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
  return
 }

 cfg, _ := h.svc.Get()
 if cfg == nil {
  cfg = &models.SecurityConfig{Name: "default"}
 }

 var exclusions []WAFExclusion
 if cfg.WAFExclusions != "" {
  json.Unmarshal([]byte(cfg.WAFExclusions), &exclusions)
 }

 exclusions = append(exclusions, exclusion)
 data, _ := json.Marshal(exclusions)
 cfg.WAFExclusions = string(data)

 if err := h.svc.Upsert(cfg); err != nil {
  c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
  return
 }

 // Apply to Caddy
 if h.caddyManager != nil {
  h.caddyManager.ApplyConfig(c.Request.Context())
 }

 c.JSON(http.StatusOK, gin.H{"exclusion": exclusion})
}
```

**Routes:**

- `GET /api/v1/security/waf/exclusions`
- `POST /api/v1/security/waf/exclusions`
- `DELETE /api/v1/security/waf/exclusions/:id`

#### 4.3 Frontend Updates

##### 4.3.1 Update WafConfig Page

**File:** `frontend/src/pages/WafConfig.tsx`

Add:

1. Paranoia level selector (1-4 with descriptions)
2. Exclusions management panel
3. Per-host override indicator

```tsx
// Paranoia level selector
const PARANOIA_LEVELS = [
  { level: 1, name: 'Low', description: 'Minimal false positives, basic protection' },
  { level: 2, name: 'Medium', description: 'Balanced protection, some false positives' },
  { level: 3, name: 'High', description: 'Strong protection, requires tuning' },
  { level: 4, name: 'Paranoid', description: 'Maximum protection, many false positives' },
]

// In render:
<div>
  <label>Paranoia Level</label>
  <select value={paranoiaLevel} onChange={...}>
    {PARANOIA_LEVELS.map(p => (
      <option key={p.level} value={p.level}>
        {p.level} - {p.name}
      </option>
    ))}
  </select>
  <p className="text-xs text-gray-500">
    {PARANOIA_LEVELS.find(p => p.level === paranoiaLevel)?.description}
  </p>
</div>
```

##### 4.3.2 Add Per-Host WAF Toggle to Proxy Host Form

**File:** `frontend/src/pages/ProxyHostForm.tsx` (or similar)

```tsx
<Switch
  label="Disable WAF for this host"
  checked={formData.waf_disabled}
  onChange={(e) => setFormData({...formData, waf_disabled: e.target.checked})}
  helperText="Override global WAF settings for this specific host"
/>
```

#### 4.4 Test Requirements

**File:** `backend/internal/caddy/config_waf_test.go`

```go
func TestBuildWAFHandler_ParanoiaLevel(t *testing.T) {
 secCfg := &models.SecurityConfig{
  WAFMode:          "block",
  WAFParanoiaLevel: 2,
 }
 h, err := buildWAFHandler(nil, nil, nil, secCfg, true)
 require.NoError(t, err)
 directives := h["directives"].(string)
 assert.Contains(t, directives, "tx.paranoia_level=2")
}

func TestBuildWAFHandler_Exclusions(t *testing.T) {
 exclusions := []WAFExclusion{
  {RuleID: 942100, Description: "SQL injection false positive"},
 }
 data, _ := json.Marshal(exclusions)
 secCfg := &models.SecurityConfig{
  WAFMode:       "block",
  WAFExclusions: string(data),
 }
 h, err := buildWAFHandler(nil, nil, nil, secCfg, true)
 require.NoError(t, err)
 directives := h["directives"].(string)
 assert.Contains(t, directives, "SecRuleRemoveById 942100")
}

func TestBuildWAFHandler_PerHostDisabled(t *testing.T) {
 host := &models.ProxyHost{WAFDisabled: true}
 secCfg := &models.SecurityConfig{WAFMode: "block"}
 h, err := buildWAFHandler(host, nil, nil, secCfg, true)
 require.NoError(t, err)
 assert.Nil(t, h)
}
```

---

## File Change Summary

### New Files

| File | Purpose |
|------|---------|
| `backend/internal/services/geoip_service.go` | GeoIP lookup service |
| `backend/internal/services/geoip_service_test.go` | GeoIP unit tests |
| `backend/internal/crowdsec/registration.go` | CrowdSec bouncer registration |
| `backend/internal/crowdsec/registration_test.go` | CrowdSec registration tests |

### Modified Files

| File | Changes |
|------|---------|
| `backend/go.mod` | Add `github.com/oschwald/geoip2-golang` |
| `backend/internal/models/security_config.go` | Add `WAFParanoiaLevel`, `WAFExclusions`, `RateLimitBypassList` |
| `backend/internal/models/proxy_host.go` | Add `WAFDisabled` |
| `backend/internal/services/access_list_service.go` | Add GeoIP integration to `TestIP` |
| `backend/internal/caddy/config.go` | Fix burst, add bypass, enhance WAF handler |
| `backend/internal/cerberus/cerberus.go` | Remove naive `<script>` check, add logging |
| `backend/internal/api/handlers/security_handler.go` | Add presets, exclusions, GeoIP reload endpoints |
| `backend/internal/server/server.go` | Initialize GeoIP service |
| `frontend/src/api/security.ts` | Add new API types |
| `frontend/src/pages/RateLimiting.tsx` | Add presets, bypass list UI |
| `frontend/src/pages/WafConfig.tsx` | Add paranoia selector, exclusions UI |
| `frontend/src/pages/AccessLists.tsx` | Enhance geo test result display |

---

## Database Migrations

Auto-migrate will handle new fields. For explicit migration:

```sql
-- Add new SecurityConfig fields
ALTER TABLE security_configs ADD COLUMN waf_paranoia_level INTEGER DEFAULT 1;
ALTER TABLE security_configs ADD COLUMN waf_exclusions TEXT;
ALTER TABLE security_configs ADD COLUMN rate_limit_bypass_list TEXT;

-- Add ProxyHost WAF toggle
ALTER TABLE proxy_hosts ADD COLUMN waf_disabled BOOLEAN DEFAULT FALSE;
```

---

## Implementation Order

1. **Phase 2 (Rate Limit)** - Quickest, enables immediate value
2. **Phase 1 (GeoIP)** - Most user-requested feature
3. **Phase 4 (WAF)** - Most complex but highest security impact
4. **Phase 3 (CrowdSec)** - Requires external service coordination

---

## Definition of Done Checklist

For each phase:

- [ ] All new code has unit tests with >85% coverage
- [ ] Pre-commit hooks pass (`pre-commit run --all-files`)
- [ ] Backend compiles without warnings (`go build ./...`)
- [ ] Frontend builds without errors (`npm run build`)
- [ ] Integration tests pass (if applicable)
- [ ] Documentation updated in `docs/cerberus.md`
- [ ] API changes reflected in `docs/api.md`
- [ ] Feature documented in `docs/features.md`

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| GeoIP database missing | Geo-blocking fails silently | Graceful fallback with warning |
| CrowdSec LAPI unavailable | No IP reputation | Caddy continues without bouncer |
| WAF exclusions break rules | Security gap | Validate rule IDs, log exclusions |
| Rate limit bypass abused | DDoS possible | Audit bypass list, alert on changes |

---

## Appendix: API Routes Summary

| Method | Endpoint | Phase |
|--------|----------|-------|
| POST | `/api/v1/security/geoip/reload` | 1 |
| GET | `/api/v1/security/rate-limit/presets` | 2 |
| GET | `/api/v1/crowdsec/decisions` | 3 |
| GET | `/api/v1/security/waf/exclusions` | 4 |
| POST | `/api/v1/security/waf/exclusions` | 4 |
| DELETE | `/api/v1/security/waf/exclusions/:id` | 4 |
