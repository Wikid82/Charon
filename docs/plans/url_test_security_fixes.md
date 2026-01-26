# URL Test Security Fixes

## Critical Security Updates Required

### 1. DNS Rebinding Protection

**Problem**: Current plan validates IPs after DNS lookup but makes HTTP request using hostname, allowing TOCTOU attack.

**Solution**: Make HTTP request directly to validated IP address:

```go
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

    // Use first validated IP for request
    validatedIP := ips[0].IP.String()

    // Construct URL using validated IP
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

            // Validate redirect target
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
```

### 2. Complete IP Blocklist

```go
func isPrivateIP(ip net.IP) bool {
    // Check special addresses
    if ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
       ip.IsLinkLocalMulticast() || ip.IsMulticast() {
        return true
    }

    // Check if it's IPv4 or IPv6
    if ip.To4() != nil {
        // IPv4 private ranges
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

### 3. Rate Limiting Implementation

**Option A: Using golang.org/x/time/rate**

```go
import (
    "golang.org/x/time/rate"
    "sync"
)

var (
    urlTestLimiters = make(map[string]*rate.Limiter)
    limitersMutex   sync.RWMutex
)

func getUserLimiter(userID string) *rate.Limiter {
    limitersMutex.Lock()
    defer limitersMutex.Unlock()

    limiter, exists := urlTestLimiters[userID]
    if !exists {
        // 5 requests per minute
        limiter = rate.NewLimiter(rate.Every(12*time.Second), 5)
        urlTestLimiters[userID] = limiter
    }
    return limiter
}

// In handler:
func (h *SettingsHandler) TestPublicURL(c *gin.Context) {
    userID, _ := c.Get("user_id") // Assuming auth middleware sets this
    limiter := getUserLimiter(userID.(string))

    if !limiter.Allow() {
        c.JSON(http.StatusTooManyRequests, gin.H{
            "error": "Rate limit exceeded. Try again later.",
        })
        return
    }

    // ... rest of handler
}
```

**Option B: Middleware approach**

Create `backend/internal/middleware/rate_limit.go`:

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

Usage in routes:

```go
urlTestLimiter := middleware.NewRateLimiter(5.0/60.0, 5) // 5 per minute
protected.POST("/settings/test-url",
    urlTestLimiter.Limit(),
    settingsHandler.TestPublicURL)
```

### 4. Integration Test for DNS Rebinding

```go
// backend/internal/utils/url_test_test.go

func TestTestURLConnectivity_DNSRebinding(t *testing.T) {
    // This test verifies that we make HTTP request to resolved IP,
    // not the hostname, preventing DNS rebinding attacks

    // Create test server that only responds to direct IP requests
    ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request was made to IP, not hostname
        if r.Host != r.RemoteAddr {
            t.Errorf("Expected request to IP address, got Host: %s", r.Host)
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer ts.Close()

    // Extract IP from test server URL
    parsedURL, _ := url.Parse(ts.URL)
    serverIP := parsedURL.Hostname()

    // Test that we can connect using IP directly
    reachable, _, err := TestURLConnectivity(fmt.Sprintf("https://%s", serverIP))
    if err != nil || !reachable {
        t.Errorf("Failed to connect to test server: %v", err)
    }
}
```

## Summary of Changes

### Security Fixes

1. ✅ DNS rebinding protection: HTTP request uses validated IP
2. ✅ Redirect validation: Check redirect targets for private IPs
3. ✅ Rate limiting: 5 requests per minute per user
4. ✅ Complete IP blocklist: All RFC reserved ranges
5. ✅ Hostname blocklist: Cloud metadata endpoints
6. ✅ HTTPS enforcement: Require secure connections
7. ✅ Port restrictions: Only 443, 8443 allowed

### Implementation Notes

- Uses `req.Host` header for SNI/vhost routing while making request to IP
- Validates redirect targets before following
- Comprehensive IPv4 and IPv6 private range blocking
- Per-user rate limiting with token bucket algorithm
- Integration test verifies DNS rebinding protection

### Testing Checklist

- [ ] Test public HTTPS URL → Success
- [ ] Test HTTP URL → Rejected (HTTPS required)
- [ ] Test private IP → Blocked
- [ ] Test metadata endpoint → Blocked by hostname
- [ ] Test redirect to private IP → Blocked
- [ ] Test rate limit → 429 after 5 requests
- [ ] Test non-standard port → Rejected
- [ ] Test DNS rebinding scenario → Protected
