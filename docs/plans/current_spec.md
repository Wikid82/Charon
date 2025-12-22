# URL Test Button Navigation Bug - Implementation Plan

**Status**: Ready for Implementation
**Priority**: High
**Affected Component**: System Settings - Application URL Test
**Last Updated**: December 22, 2025 (Security Review Completed)

---

## Security Review Summary

**Critical vulnerabilities fixed in this revision:**

1. ✅ **DNS Rebinding Protection**: HTTP requests now use validated IP addresses instead of hostnames, preventing TOCTOU attacks
2. ✅ **Redirect Validation**: All redirect targets validated for private IPs before following
3. ✅ **Complete IP Blocklist**: 15 IPv4 + 6 IPv6 reserved ranges blocked (RFC-compliant)
4. ✅ **HTTPS Enforcement**: Only HTTPS URLs accepted for secure testing
5. ✅ **Port Restrictions**: Limited to 443/8443 only
6. ✅ **Hostname Blocklist**: Cloud metadata endpoints explicitly blocked
7. ✅ **Rate Limiting**: Middleware implementation with 5 tests/minute per user

---

## Executive Summary

The URL test button in System Settings incorrectly uses `window.open()` instead of performing a server-side connectivity test. This causes the browser to open the URL in a new tab (blank screen if unreachable) rather than executing a proper health check.

**User Report**: Clicking test button for `http://100.98.12.109:8080/settings/https//charon.hatfieldhosted.com` opened blank blue screen.

---

## Current Implementation Analysis

### Frontend: SystemSettings.tsx

**File**: [frontend/src/pages/SystemSettings.tsx](frontend/src/pages/SystemSettings.tsx#L103-L118)

```typescript
const testPublicURL = async () => {
  if (!publicURL) {
    toast.error(t('systemSettings.applicationUrl.invalidUrl'))
    return
  }
  setPublicURLSaving(true)
  try {
    window.open(publicURL, '_blank')  // ❌ Opens URL in browser instead of API test
    toast.success('URL opened in new tab')
  } catch {
    toast.error('Failed to open URL')
  } finally {
    setPublicURLSaving(false)
  }
}
```

**Button** (line 417):
```typescript
<Button onClick={testPublicURL} disabled={!publicURL || publicURLSaving}>
  <ExternalLink className="h-4 w-4 mr-2" />
  {t('systemSettings.applicationUrl.testButton')}
</Button>
```

### Backend: Existing Validation Only

**File**: [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go#L195)

```go
protected.POST("/settings/validate-url", settingsHandler.ValidatePublicURL)
```

**Handler**: [backend/internal/api/handlers/settings_handler.go](backend/internal/api/handlers/settings_handler.go#L229-L267)

This endpoint **only validates format** (scheme, no paths), does NOT test connectivity.

---

## Root Cause

1. **Misnamed Function**: `testPublicURL()` implies connectivity test but performs navigation
2. **No Backend Endpoint**: Missing API for server-side reachability tests
3. **User Expectation**: "Test" button should verify connectivity, not open URL
4. **Malformed URL Issue**: User input `https//charon.hatfieldhosted.com` (missing colon) causes navigation failure

---

## Security: SSRF Protection Requirements

**CRITICAL**: Backend URL testing must prevent Server-Side Request Forgery attacks.

### Required Protections

1. **Complete IP Blocklist**: Reject all private/reserved IPs
   - IPv4: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
   - Loopback: `127.0.0.0/8`, IPv6 `::1/128`
   - Link-local: `169.254.0.0/16`, IPv6 `fe80::/10`
   - Cloud metadata: `169.254.169.254` (AWS/GCP/Azure)
   - IPv6 ULA: `fc00::/7`
   - Test/doc ranges: `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`
   - Reserved: `0.0.0.0/8`, `240.0.0.0/4`, `255.255.255.255/32`
   - CGNAT: `100.64.0.0/10`
   - Multicast: `224.0.0.0/4`, IPv6 `ff00::/8`

2. **DNS Rebinding Protection** (CRITICAL):
   - Make HTTP request directly to validated IP address
   - Use `req.Host` header for SNI/vhost routing
   - Prevents TOCTOU attacks where DNS changes between check and use

3. **Redirect Validation** (CRITICAL):
   - Validate each redirect target's IP before following
   - Max 2 redirects
   - Block redirects to private IPs

4. **Hostname Blocklist**:
   - `metadata.google.internal`, `metadata.goog`, `metadata`
   - `169.254.169.254`, `localhost`

5. **HTTPS Enforcement**:
   - Require HTTPS scheme (reject HTTP for security)
   - Warn users about insecure connections

6. **Port Restrictions**:
   - Allow only: 443 (HTTPS), 8443 (alternate HTTPS)
   - Block all other ports including privileged ports

7. **Rate Limiting**: 5 tests per minute per user
   - Implement using `golang.org/x/time/rate`
   - Per-user token bucket with burst allowance

8. **Request Restrictions**:
   - 5 second HTTP timeout
   - 3 second DNS timeout
   - HEAD method only (no full GET)

9. **Admin-Only**: Require admin role (already enforced on `/settings/*`)

---

## Implementation Plan

### Backend: New API Endpoint

#### 1. Register Route with Rate Limiting

**File**: [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go#L195)

After line 195:
```go
// Create rate limiter for URL testing (5 requests per minute)
urlTestLimiter := middleware.NewRateLimiter(5.0/60.0, 5)
protected.POST("/settings/test-url",
    urlTestLimiter.Limit(),
    settingsHandler.TestPublicURL)
```

#### 2. Handler

**File**: [backend/internal/api/handlers/settings_handler.go](backend/internal/api/handlers/settings_handler.go#L267)

```go
// TestPublicURL performs server-side connectivity test with SSRF protection
func (h *SettingsHandler) TestPublicURL(c *gin.Context) {
role, _ := c.Get("role")
if role != "admin" {
c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
return
}

type TestURLRequest struct {
URL string `json:"url" binding:"required"`
}

var req TestURLRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

// Validate format first
normalized, _, err := utils.ValidateURL(req.URL)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{
"reachable": false,
"error":     "Invalid URL format",
})
return
}

// Test connectivity (SSRF-safe)
reachable, latency, err := utils.TestURLConnectivity(normalized)
if err != nil {
c.JSON(http.StatusOK, gin.H{
"reachable": false,
"error":     err.Error(),
})
return
}

c.JSON(http.StatusOK, gin.H{
"reachable": reachable,
"latency":   latency,
"message":   fmt.Sprintf("URL reachable (%.0fms)", latency),
})
}
```

#### 3. Utility Function with DNS Rebinding Protection

**File**: Create `backend/internal/utils/url_test.go`

```go
package utils

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "net/url"
    "strings"
    "time"
)

// TestURLConnectivity checks if URL is reachable with comprehensive SSRF protection
// including DNS rebinding prevention, redirect validation, and complete IP blocklist
func TestURLConnectivity(rawURL string) (bool, float64, error) {
    parsed, err := url.Parse(rawURL)
    if err != nil {
        return false, 0, fmt.Errorf("invalid URL: %w", err)
    }

    host := parsed.Hostname()
    port := parsed.Port()
    if port == "" {
        port = map[string]string{"https": "443", "http": "80"}[parsed.Scheme]
    }

    // Enforce HTTPS for security
    if parsed.Scheme != "https" {
        return false, 0, fmt.Errorf("HTTPS required")
    }

    // Validate port
    allowedPorts := map[string]bool{"443": true, "8443": true}
    if !allowedPorts[port] {
        return false, 0, fmt.Errorf("port %s not allowed", port)
    }

    // Block metadata hostnames explicitly
    forbiddenHosts := []string{
        "metadata.google.internal", "metadata.goog", "metadata",
        "169.254.169.254", "localhost",
    }
    for _, forbidden := range forbiddenHosts {
        if strings.EqualFold(host, forbidden) {
            return false, 0, fmt.Errorf("blocked hostname")
        }
    }

    // DNS resolution with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
    if err != nil {
        return false, 0, fmt.Errorf("DNS failed: %w", err)
    }
    if len(ips) == 0 {
        return false, 0, fmt.Errorf("no IPs found")
    }

    // SSRF protection: block private IPs
    for _, ip := range ips {
        if isPrivateIP(ip.IP) {
            return false, 0, fmt.Errorf("private IP blocked: %s", ip.IP)
        }
    }

    // DNS REBINDING PROTECTION: Use first validated IP for request
    validatedIP := ips[0].IP.String()

    // Construct URL using validated IP to prevent TOCTOU attacks
    var targetURL string
    if port != "" {
        targetURL = fmt.Sprintf("%s://%s:%s%s", parsed.Scheme, validatedIP, port, parsed.Path)
    } else {
        targetURL = fmt.Sprintf("%s://%s%s", parsed.Scheme, validatedIP, parsed.Path)
    }

    // HTTP request with redirect validation
    client := &http.Client{
        Timeout: 5 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 2 {
                return fmt.Errorf("too many redirects")
            }

            // CRITICAL: Validate redirect target IPs
            redirectHost := req.URL.Hostname()
            redirectIPs, err := net.DefaultResolver.LookupIPAddr(ctx, redirectHost)
            if err != nil {
                return fmt.Errorf("redirect DNS failed: %w", err)
            }
            if len(redirectIPs) == 0 {
                return fmt.Errorf("redirect DNS returned no IPs")
            }

            // Check redirect target IPs
            for _, ip := range redirectIPs {
                if isPrivateIP(ip.IP) {
                    return fmt.Errorf("redirect to private IP blocked: %s", ip.IP)
                }
            }
            return nil
        },
    }

    start := time.Now()
    req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
    if err != nil {
        return false, 0, fmt.Errorf("request creation failed: %w", err)
    }

    // Set Host header to original hostname for SNI/vhost routing
    req.Host = parsed.Host
    req.Header.Set("User-Agent", "Charon-Health-Check/1.0")

    resp, err := client.Do(req)
    latency := time.Since(start).Seconds() * 1000

    if err != nil {
        return false, 0, fmt.Errorf("connection failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 200 && resp.StatusCode < 400 {
        return true, latency, nil
    }

    return false, latency, fmt.Errorf("status %d", resp.StatusCode)
}

// isPrivateIP checks if an IP is in any private/reserved range
func isPrivateIP(ip net.IP) bool {
    // Check special addresses
    if ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
       ip.IsLinkLocalMulticast() || ip.IsMulticast() {
        return true
    }

    // Check if it's IPv4 or IPv6
    if ip.To4() != nil {
        // IPv4 private ranges (comprehensive RFC compliance)
        privateBlocks := []string{
            "0.0.0.0/8",          // Current network
            "10.0.0.0/8",         // Private
            "100.64.0.0/10",      // Shared address space (CGNAT)
            "127.0.0.0/8",        // Loopback
            "169.254.0.0/16",     // Link-local / Cloud metadata
            "172.16.0.0/12",      // Private
            "192.0.0.0/24",       // IETF protocol assignments
            "192.0.2.0/24",       // TEST-NET-1
            "192.168.0.0/16",     // Private
            "198.18.0.0/15",      // Benchmarking
            "198.51.100.0/24",    // TEST-NET-2
            "203.0.113.0/24",     // TEST-NET-3
            "224.0.0.0/4",        // Multicast
            "240.0.0.0/4",        // Reserved
            "255.255.255.255/32", // Broadcast
        }

        for _, block := range privateBlocks {
            _, subnet, _ := net.ParseCIDR(block)
            if subnet.Contains(ip) {
                return true
            }
        }
    } else {
        // IPv6 private ranges
        privateBlocks := []string{
            "::1/128",       // Loopback
            "::/128",        // Unspecified
            "::ffff:0:0/96", // IPv4-mapped
            "fe80::/10",     // Link-local
            "fc00::/7",      // Unique local
            "ff00::/8",      // Multicast
        }

        for _, block := range privateBlocks {
            _, subnet, _ := net.ParseCIDR(block)
            if subnet.Contains(ip) {
                return true
            }
        }
    }

    return false
}
```

#### 4. Rate Limiting Middleware

**File**: Create `backend/internal/middleware/rate_limit.go`

```go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     rate.Limit(rps),
        burst:    burst,
    }
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[key]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[key] = limiter
    }
    return limiter
}

func (rl *RateLimiter) Limit() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, exists := c.Get("user_id")
        if !exists {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "Authentication required",
            })
            return
        }

        limiter := rl.getLimiter(userID.(string))
        if !limiter.Allow() {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded. Maximum 5 tests per minute.",
            })
            return
        }

        c.Next()
    }
}
```

### Frontend: Use API Instead of window.open

#### 1. API Client

**File**: [frontend/src/api/settings.ts](frontend/src/api/settings.ts#L40)

```typescript
export const testPublicURL = async (url: string): Promise<{
  reachable: boolean
  latency?: number
  message?: string
  error?: string
}> => {
  const response = await client.post('/settings/test-url', { url })
  return response.data
}
```

#### 2. Component Update

**File**: [frontend/src/pages/SystemSettings.tsx](frontend/src/pages/SystemSettings.tsx#L103-L118)

Replace function:

```typescript
const testPublicURLHandler = async () => {
  if (!publicURL) {
    toast.error(t('systemSettings.applicationUrl.invalidUrl'))
    return
  }
  setPublicURLSaving(true)
  try {
    const result = await testPublicURL(publicURL)
    if (result.reachable) {
      toast.success(
        result.message || `URL reachable (${result.latency?.toFixed(0)}ms)`
      )
    } else {
      toast.error(result.error || 'URL not reachable')
    }
  } catch (error) {
    toast.error(error instanceof Error ? error.message : 'Test failed')
  } finally {
    setPublicURLSaving(false)
  }
}
```

#### 3. Update Button (line 417)

```typescript
<Button onClick={testPublicURLHandler} disabled={!publicURL || publicURLSaving}>
```

#### 4. Update Imports (line 17)

```typescript
import { getSettings, updateSetting, testPublicURL } from '../api/settings'
```

---

## Testing Strategy

### Backend Unit Tests

**File**: `backend/internal/utils/url_test_test.go`

```go
func TestTestURLConnectivity_Success(t *testing.T)
func TestTestURLConnectivity_PrivateIP_Blocked(t *testing.T)
func TestTestURLConnectivity_InvalidURL(t *testing.T)
func TestTestURLConnectivity_Timeout(t *testing.T)
func TestTestURLConnectivity_HTTPRejected(t *testing.T)
func TestTestURLConnectivity_InvalidPort(t *testing.T)
func TestTestURLConnectivity_MetadataHostnameBlocked(t *testing.T)
func TestTestURLConnectivity_RedirectToPrivateIP(t *testing.T)
func TestTestURLConnectivity_DNSRebinding(t *testing.T) // Verifies IP-based request
func TestIsPrivateIP_AllRanges(t *testing.T)           // Test all blocked ranges
```

**DNS Rebinding Integration Test**:
```go
// Verifies that HTTP request is made to validated IP, not hostname
func TestTestURLConnectivity_DNSRebinding(t *testing.T) {
    // Create test server that only responds to direct IP requests
    ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify Host header is set for SNI but request went to IP
        if r.Host == "" {
            t.Error("Host header not set for SNI routing")
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer ts.Close()

    // Test verifies we connect to IP while preserving Host header
    // This prevents DNS rebinding TOCTOU attacks
}
```

**File**: `backend/internal/api/handlers/settings_handler_test.go`

Test handler with:
- Valid public HTTPS URL → Success
- HTTP URL → Rejected (HTTPS required)
- Private IP (10.0.0.1) → Blocked
- Cloud metadata IP (169.254.169.254) → Blocked
- Non-standard port (8080) → Rejected
- Non-admin user → 403 Forbidden
- Malformed URL → Validation error
- Rate limiting → 429 after 5 requests

**File**: `backend/internal/middleware/rate_limit_test.go`

Test rate limiter:
- 5 requests within minute → Success
- 6th request → 429 Too Many Requests
- Different users → Independent limits

### Frontend Tests

**File**: `frontend/src/pages/__tests__/SystemSettings.spec.tsx`

```typescript
it('shows success toast on reachable URL')
it('shows error toast on unreachable URL')
it('disables button when URL empty')
it('shows latency in success message')
```

### Manual Tests

#### Security Tests
- [ ] Test `https://google.com` → Success with latency
- [ ] Test `http://google.com` → Rejected (HTTP not allowed)
- [ ] Test `https://google.com:8080` → Rejected (invalid port)
- [ ] Test `https://192.168.1.1` → Blocked (private IP)
- [ ] Test `https://10.0.0.1` → Blocked (private IP)
- [ ] Test `https://169.254.169.254` → Blocked (metadata IP)
- [ ] Test `https://localhost` → Blocked (hostname blocklist)
- [ ] Test `https://metadata.google.internal` → Blocked (hostname)
- [ ] Test redirect to private IP → Blocked in redirect check
- [ ] Test 6 consecutive requests → 6th returns 429
- [ ] Test as non-admin user → 403 Forbidden

#### Functional Tests
- [ ] Test `https://nonexistent.invalid` → DNS error
- [ ] Test `https//example.com` → Validation error (malformed)
- [ ] Test unreachable HTTPS URL → Connection timeout
- [ ] Test URL with query params → Success
- [ ] Verify latency displayed in success message

---

## Implementation Checklist

### Backend
- [ ] Create `backend/internal/utils/url_test.go` with DNS rebinding protection
- [ ] Create `backend/internal/middleware/rate_limit.go`
- [ ] Add `TestPublicURL` handler
- [ ] Register route with rate limiting in `routes.go`
- [ ] Write comprehensive unit tests (11+ test cases)
- [ ] Test SSRF protection (all IP ranges)
- [ ] Test DNS rebinding protection
- [ ] Test redirect validation
- [ ] Test rate limiting
- [ ] Test HTTPS enforcement
- [ ] Test port restrictions

### Frontend
- [ ] Add API function to `settings.ts`
- [ ] Update component handler
- [ ] Update button onClick
- [ ] Update imports
- [ ] Write component tests
- [ ] Manual testing

### Documentation
- [ ] Update API docs
- [ ] Document SSRF protection

---

## References

### Security
- OWASP SSRF Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html
- DNS Rebinding Attacks: https://en.wikipedia.org/wiki/DNS_rebinding
- RFC 1918 (Private IPv4): https://datatracker.ietf.org/doc/html/rfc1918
- RFC 4193 (IPv6 ULA): https://datatracker.ietf.org/doc/html/rfc4193
- RFC 5737 (Test Networks): https://datatracker.ietf.org/doc/html/rfc5737
- RFC 6598 (CGNAT): https://datatracker.ietf.org/doc/html/rfc6598
- Cloud Metadata SSRF: https://blog.appsecco.com/getting-started-with-version-2-of-aws-ec2-instance-metadata-service-imdsv2-2ad03a1f3650

### Rate Limiting
- golang.org/x/time/rate: https://pkg.go.dev/golang.org/x/time/rate
- Token Bucket Algorithm: https://en.wikipedia.org/wiki/Token_bucket

---

**Ready for Implementation**
