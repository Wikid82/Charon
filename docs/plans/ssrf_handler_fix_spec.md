# SSRF Remediation Plan: settings_handler.go TestPublicURL

**Status:** Planning - Revised
**Priority:** Critical
**Affected File:** `backend/internal/api/handlers/settings_handler.go`
**CodeQL Alert:** SSRF vulnerability via user-controlled URL input
**Date:** 2025-12-23
**Last Updated:** 2025-12-23 (Supervisor Review)

---

## Executive Summary

### Key Security Insights

1. **Triple-Layer Defense-in-Depth**:
   - Layer 1: Format validation (scheme, paths)
   - Layer 2: Pre-validation with DNS resolution (blocks private IPs)
   - Layer 3: Runtime re-validation at connection time (eliminates TOCTOU)

2. **DNS Rebinding/TOCTOU Protection**:
   - `ssrfSafeDialer()` performs **second DNS resolution** at connection time
   - Even if attacker changes DNS between validations, connection is blocked
   - Closes the Time-of-Check Time-of-Use (TOCTOU) window

3. **URL Parser Differential Protection**:
   - `ValidateExternalURL()` rejects URLs with embedded credentials
   - Prevents `http://evil.com@127.0.0.1/` bypass attacks

4. **CodeQL Taint Breaking**:
   - Works because `ValidateExternalURL()` returns **new value** (not passthrough)
   - Static analysis sees taint chain broken by value transformation
   - NOT based on function name recognition

5. **HTTP Status Code Consistency**:
   - SSRF blocks return `200 OK` with `reachable: false` (existing behavior)
   - Only format errors return `400 Bad Request`
   - Maintains consistent API contract for clients

---

## 1. Vulnerability Analysis

### 1.1 Data Flow

CodeQL has identified the following taint flow:

```
Source (Line 285): req.URL (user input from TestURLRequest)
    ↓
Step 2-7: utils.ValidateURL() - Format validation only
    ↓
Step 8: normalized URL (still tainted)
    ↓
Step 9-10: utils.TestURLConnectivity() → http.NewRequestWithContext() [SINK]
```

### 1.2 Root Cause

**Format Validation Only**: `utils.ValidateURL()` performs only superficial format checks:

- Validates scheme (http/https)
- Rejects paths beyond "/"
- Returns warning for HTTP
- **Does NOT validate for SSRF risks** (private IPs, localhost, cloud metadata)

**Runtime Protection Not Recognized**: While `TestURLConnectivity()` has SSRF protection via `ssrfSafeDialer()`:

- The dialer blocks private IPs at connection time
- CodeQL's static analysis cannot detect this runtime protection
- The taint chain remains unbroken from the analysis perspective

**The Problem**: Static analysis sees unvalidated user input flowing directly to a network request sink without explicit SSRF pre-validation.

### 1.3 Existing Security Infrastructure

We have a comprehensive SSRF validator already implemented:

**File:** `backend/internal/security/url_validator.go`

- `ValidateExternalURL()`: Full SSRF protection with DNS resolution
- Blocks private IPs, loopback, link-local, cloud metadata endpoints
- **Rejects URLs with embedded credentials** (prevents parser differentials like `http://evil.com@127.0.0.1/`)
- Configurable options (WithAllowLocalhost, WithAllowHTTP, WithTimeout)
- Well-tested and production-ready

**File:** `backend/internal/utils/url_testing.go`

- `TestURLConnectivity()`: Has runtime SSRF protection via `ssrfSafeDialer()`
- **DNS Rebinding/TOCTOU Protection**: `ssrfSafeDialer()` performs second DNS resolution and IP validation at connection time
- `isPrivateIP()`: Comprehensive IP blocking logic
- Already protects against SSRF at connection time

**Defense-in-Depth Architecture**:

1. **Handler Validation** → `ValidateExternalURL()` - Pre-validates URL and resolves DNS
2. **TestURLConnectivity Re-Validation** → `ssrfSafeDialer()` - Validates IP again at connection time
3. This eliminates DNS rebinding/TOCTOU vulnerabilities (attacker can't change DNS between validations)

---

## 2. Fix Options

### Option A: Add Explicit Pre-Validation in Handler (RECOMMENDED)

**Description**: Insert explicit SSRF validation in `TestPublicURL` handler before calling `TestURLConnectivity()`.

**Implementation**:

```go
func (h *SettingsHandler) TestPublicURL(c *gin.Context) {
 // ... existing auth check ...

 var req TestURLRequest
 if err := c.ShouldBindJSON(&req); err != nil {
  c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  return
 }

 // Step 1: Format validation
 normalized, _, err := utils.ValidateURL(req.URL)
 if err != nil {
  c.JSON(http.StatusBadRequest, gin.H{
   "reachable": false,
   "error":     "Invalid URL format",
  })
  return
 }

 // Step 2: SSRF validation (BREAKS TAINT CHAIN)
 validatedURL, err := security.ValidateExternalURL(normalized,
  security.WithAllowHTTP())
 if err != nil {
  c.JSON(http.StatusOK, gin.H{
   "reachable": false,
   "error":     err.Error(),
  })
  return
 }

 // Step 3: Connectivity test (now using validated URL)
 reachable, latency, err := utils.TestURLConnectivity(validatedURL)
 // ... rest of handler ...
}
```

**Pros**:

- ✅ Minimal code changes (localized to one handler)
- ✅ Explicitly breaks taint chain for CodeQL
- ✅ Uses existing, well-tested `security.ValidateExternalURL()`
- ✅ Clear separation of concerns (format → security → connectivity)
- ✅ Easy to audit and understand

**Cons**:

- ❌ Requires adding security package import to handler
- ❌ Two-step validation might seem redundant (but is defense-in-depth)

---

### Option B: Enhance ValidateURL() with SSRF Checks

**Description**: Modify `utils.ValidateURL()` to include SSRF validation.

**Implementation**:

```go
// In utils/url.go
func ValidateURL(rawURL string, options ...security.ValidationOption) (normalized string, warning string, err error) {
 // Parse URL
 parsed, parseErr := url.Parse(rawURL)
 if parseErr != nil {
  return "", "", parseErr
 }

 // Validate scheme
 if parsed.Scheme != "http" && parsed.Scheme != "https" {
  return "", "", &url.Error{Op: "parse", URL: rawURL, Err: nil}
 }

 // Warn if HTTP
 if parsed.Scheme == "http" {
  warning = "Using HTTP is not recommended. Consider using HTTPS for security."
 }

 // Reject URLs with path components
 if parsed.Path != "" && parsed.Path != "/" {
  return "", "", &url.Error{Op: "validate", URL: rawURL, Err: nil}
 }

 // SSRF validation (NEW)
 normalized = strings.TrimSuffix(rawURL, "/")
 validatedURL, err := security.ValidateExternalURL(normalized,
  security.WithAllowHTTP())
 if err != nil {
  return "", "", fmt.Errorf("SSRF validation failed: %w", err)
 }

 return validatedURL, warning, nil
}
```

**Pros**:

- ✅ Single validation point
- ✅ All callers of `ValidateURL()` automatically get SSRF protection
- ✅ No changes needed in handlers

**Cons**:

- ❌ Changes signature/behavior of widely-used function
- ❌ Mixes concerns (format validation + security validation)
- ❌ May break existing tests that expect `ValidateURL()` to accept localhost
- ❌ Adds cross-package dependency (utils → security)
- ❌ Could impact other callers unexpectedly

---

### Option C: Create Dedicated ValidateURLForTesting() Wrapper

**Description**: Create a new function specifically for the test endpoint that combines all validations.

**Implementation**:

```go
// In utils/url.go or security/url_validator.go
func ValidateURLForTesting(rawURL string) (normalized string, warning string, err error) {
 // Step 1: Format validation
 normalized, warning, err = ValidateURL(rawURL)
 if err != nil {
  return "", "", err
 }

 // Step 2: SSRF validation
 validatedURL, err := security.ValidateExternalURL(normalized,
  security.WithAllowHTTP())
 if err != nil {
  return "", warning, fmt.Errorf("security validation failed: %w", err)
 }

 return validatedURL, warning, nil
}
```

**Handler Usage**:

```go
func (h *SettingsHandler) TestPublicURL(c *gin.Context) {
 // ... auth check ...

 var req TestURLRequest
 if err := c.ShouldBindJSON(&req); err != nil {
  c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  return
 }

 // Combined validation
 normalized, _, err := utils.ValidateURLForTesting(req.URL)
 if err != nil {
  c.JSON(http.StatusBadRequest, gin.H{
   "reachable": false,
   "error":     err.Error(),
  })
  return
 }

 // Connectivity test
 reachable, latency, err := utils.TestURLConnectivity(normalized)
 // ... rest of handler ...
}
```

**Pros**:

- ✅ Clean API for the specific use case
- ✅ Doesn't modify existing `ValidateURL()` behavior
- ✅ Self-documenting function name
- ✅ Encapsulates the two-step validation

**Cons**:

- ❌ Adds another function to maintain
- ❌ Possible confusion about when to use `ValidateURL()` vs `ValidateURLForTesting()`

---

## 3. Recommended Approach: Option A (Explicit Pre-Validation)

**Rationale**:

1. **Minimal Risk**: Localized change to a single handler
2. **Clear Intent**: The two-step validation makes security concerns explicit
3. **Defense in Depth**: Format validation + SSRF validation + runtime protection
4. **Static Analysis Friendly**: CodeQL can clearly see the taint break
5. **Reuses Battle-Tested Code**: Leverages existing `security.ValidateExternalURL()`
6. **No Breaking Changes**: Doesn't modify shared utility functions

---

## 4. Implementation Plan

### 4.0 HTTP Status Code Strategy

**Existing Behavior** (from `settings_handler_test.go:605`):

- SSRF blocks return `200 OK` with `reachable: false` and error message
- This maintains consistent API contract: endpoint always returns structured JSON
- Only format validation errors return `400 Bad Request`

**Rationale**:

- SSRF validation is a *connectivity constraint*, not a request format error
- Returning 200 allows clients to distinguish between "URL malformed" vs "URL blocked by security policy"
- Consistent with existing test: `TestSettingsHandler_TestPublicURL_PrivateIPBlocked` expects `StatusOK`

**Implementation Rule**:

- `400 Bad Request` → Format errors (invalid scheme, paths, malformed JSON)
- `200 OK` → All SSRF/connectivity failures (return `reachable: false` with error details)

### 4.1 Code Changes

**File:** `backend/internal/api/handlers/settings_handler.go`

1. Add import:

   ```go
   import (
       // ... existing imports ...
       "github.com/Wikid82/charon/backend/internal/security"
   )
   ```

2. Modify `TestPublicURL` handler (lines 269-316):

   ```go
   func (h *SettingsHandler) TestPublicURL(c *gin.Context) {
       // Admin-only access check
       role, exists := c.Get("role")
       if !exists || role != "admin" {
           c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
           return
       }

       // Parse request body
       type TestURLRequest struct {
           URL string `json:"url" binding:"required"`
       }

       var req TestURLRequest
       if err := c.ShouldBindJSON(&req); err != nil {
           c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
           return
       }

       // Step 1: Validate URL format (scheme, no paths)
       normalized, _, err := utils.ValidateURL(req.URL)
       if err != nil {
           c.JSON(http.StatusBadRequest, gin.H{
               "reachable": false,
               "error":     "Invalid URL format",
           })
           return
       }

       // Step 2: SSRF validation (DNS resolution + IP blocking)
       // This explicitly validates against private IPs, loopback, link-local,
       // and cloud metadata endpoints, breaking the taint chain for static analysis.
       validatedURL, err := security.ValidateExternalURL(normalized,
           security.WithAllowHTTP())
       if err != nil {
           c.JSON(http.StatusOK, gin.H{
               "reachable": false,
               "error":     err.Error(),
           })
           return
       }

       // Step 3: Perform connectivity test with runtime SSRF protection
       reachable, latency, err := utils.TestURLConnectivity(validatedURL)
       if err != nil {
           c.JSON(http.StatusOK, gin.H{
               "reachable": false,
               "error":     err.Error(),
           })
           return
       }

       // Return success response
       c.JSON(http.StatusOK, gin.H{
           "reachable": reachable,
           "latency":   latency,
           "message":   fmt.Sprintf("URL reachable (%.0fms)", latency),
       })
   }
   ```

### 4.2 Test Cases

**File:** `backend/internal/api/handlers/settings_handler_test.go`

Add comprehensive test cases:

```go
func TestTestPublicURL_SSRFProtection(t *testing.T) {
 tests := []struct {
  name           string
  url            string
  expectedStatus int
  expectReachable bool
  expectError    string
 }{
  {
   name:           "Valid public URL",
   url:            "https://example.com",
   expectedStatus: http.StatusOK,
   expectReachable: true,
  },
  {
   name:           "Private IP blocked - 10.0.0.1",
   url:            "http://10.0.0.1",
   expectedStatus: http.StatusOK,
   expectError:    "private ip",
  },
  {
   name:           "Localhost blocked - 127.0.0.1",
   url:            "http://127.0.0.1",
   expectedStatus: http.StatusOK,
   expectError:    "private ip",
  },
  {
   name:           "Localhost blocked - localhost",
   url:            "http://localhost:8080",
   expectedStatus: http.StatusOK,
   expectError:    "private ip",
  },
  {
   name:           "Cloud metadata blocked - 169.254.169.254",
   url:            "http://169.254.169.254",
   expectedStatus: http.StatusOK,
   expectError:    "cloud metadata",
  },
  {
   name:           "Link-local blocked - 169.254.1.1",
   url:            "http://169.254.1.1",
   expectedStatus: http.StatusOK,
   expectError:    "private ip",
  },
  {
   name:           "Private IPv4 blocked - 192.168.1.1",
   url:            "http://192.168.1.1",
   expectedStatus: http.StatusOK,
   expectError:    "private ip",
  },
  {
   name:           "Private IPv4 blocked - 172.16.0.1",
   url:            "http://172.16.0.1",
   expectedStatus: http.StatusOK,
   expectError:    "private ip",
  },
  {
   name:           "Invalid scheme rejected",
   url:            "ftp://example.com",
   expectedStatus: http.StatusBadRequest,
   expectError:    "Invalid URL format",
  },
  {
   name:           "Path component rejected",
   url:            "https://example.com/path",
   expectedStatus: http.StatusBadRequest,
   expectError:    "Invalid URL format",
  },
  {
   name:           "Empty URL field",
   url:            "",
   expectedStatus: http.StatusBadRequest,
   expectError:    "required",
  },
  {
   name:           "URL with embedded credentials blocked",
   url:            "http://user:pass@example.com",
   expectedStatus: http.StatusOK,
   expectError:    "credentials",
  },
  {
   name:           "HTTP URL allowed with WithAllowHTTP option",
   url:            "http://example.com",
   expectedStatus: http.StatusOK,
   expectReachable: true, // Should succeed if example.com is reachable
  },
 }

 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   // Setup test environment
   db := setupTestDB(t)
   handler := NewSettingsHandler(db)
   router := gin.New()
   router.Use(func(c *gin.Context) {
    c.Set("role", "admin")
   })
   router.POST("/api/settings/test-url", handler.TestPublicURL)

   // Create request
   body := fmt.Sprintf(`{"url": "%s"}`, tt.url)
   req, _ := http.NewRequest("POST", "/api/settings/test-url",
    strings.NewReader(body))
   req.Header.Set("Content-Type", "application/json")

   // Execute request
   w := httptest.NewRecorder()
   router.ServeHTTP(w, req)

   // Verify response
   assert.Equal(t, tt.expectedStatus, w.Code)

   var resp map[string]interface{}
   err := json.Unmarshal(w.Body.Bytes(), &resp)
   assert.NoError(t, err)

   if tt.expectError != "" {
    assert.Contains(t,
     strings.ToLower(resp["error"].(string)),
     strings.ToLower(tt.expectError))
   }

   if tt.expectReachable {
    assert.True(t, resp["reachable"].(bool))
   }
  })
 }
}

func TestTestPublicURL_RequiresAdmin(t *testing.T) {
 db := setupTestDB(t)
 handler := NewSettingsHandler(db)
 router := gin.New()

 // Non-admin user
 router.Use(func(c *gin.Context) {
  c.Set("role", "user")
 })
 router.POST("/api/settings/test-url", handler.TestPublicURL)

 body := `{"url": "https://example.com"}`
 req, _ := http.NewRequest("POST", "/api/settings/test-url",
  strings.NewReader(body))
 req.Header.Set("Content-Type", "application/json")

 w := httptest.NewRecorder()
 router.ServeHTTP(w, req)

 assert.Equal(t, http.StatusForbidden, w.Code)
}
```

### 4.3 Documentation

Add inline comment in the handler explaining the multi-layer protection:

```go
// TestPublicURL performs a server-side connectivity test with comprehensive SSRF protection.
// This endpoint implements defense-in-depth security:
// 1. Format validation: Ensures valid HTTP/HTTPS URLs without path components
// 2. SSRF validation: Pre-validates DNS resolution and blocks private/reserved IPs
// 3. Runtime protection: ssrfSafeDialer validates IPs again at connection time
// This multi-layer approach satisfies both static analysis (CodeQL) and runtime security.
```

---

## 5. Similar Issues in Codebase

### 5.1 Other Handlers Using User URLs

**Search Results**: No other handlers currently accept user-provided URLs for outbound requests.

**Checked Files**:

- `remote_server_handler.go`: Uses `net.DialTimeout()` for TCP, not HTTP requests
- `proxy_host_handler.go`: Manages proxy configs but doesn't make outbound requests
- Other handlers: No URL input parameters

**Conclusion**: This is an isolated instance. The `TestPublicURL` handler is the only place where user-supplied URLs are used for outbound HTTP requests.

### 5.2 Preventive Measures

To prevent similar issues in the future:

1. **Code Review Checklist**: Add SSRF check for any handler accepting URLs
2. **Linting Rule**: Consider adding a custom linter rule to detect `http.NewRequest*()` calls with untainted strings
3. **Documentation**: Update security guidelines to require SSRF validation for URL inputs

---

## 6. CodeQL Satisfaction Strategy

### 6.1 Why CodeQL Flags This

CodeQL's taint analysis tracks data flow from sources (user input) to sinks (network operations). It flags this because:

1. **Source**: `req.URL` is user-controlled
2. **No Explicit Barrier**: `ValidateURL()` doesn't signal to CodeQL that it performs security checks
3. **Sink**: The tainted string reaches `http.NewRequestWithContext()`

### 6.2 How Our Fix Satisfies CodeQL

By inserting `security.ValidateExternalURL()`:

1. **Returns New Value (Breaks Taint Chain)**: The function performs DNS resolution and IP validation, then returns a **new string value**. From static analysis perspective, this new return value is not the same tainted input—it's a validated, sanitized result. CodeQL's taint tracking stops here because the data flow breaks.
2. **Technical Explanation**: Static analysis tools like CodeQL track data flow through variables. When a function takes tainted input and returns a different value (not just a passthrough), the analysis must assume the return value could be untainted. Since `ValidateExternalURL()` performs validation logic (DNS resolution, IP checks) before returning, CodeQL treats the output as clean.
3. **Defense-in-Depth Signal**: Using a dedicated security validation function makes the security boundary explicit, making code review and auditing easier.

**Important**: CodeQL does NOT recognize `ValidateExternalURL()` by name alone. It works because the function returns a new value that breaks the taint chain. Any validation function that transforms input into a new output would have the same effect.

### 6.3 Expected Result

After implementation:

- ✅ CodeQL should recognize the taint chain is broken
- ✅ Alert should resolve or downgrade to "False Positive"
- ✅ Future scans should not flag this pattern

---

## 6.5 DNS Rebinding/TOCTOU Protection Architecture

### The TOCTOU Problem

A classic SSRF bypass technique is DNS rebinding, also known as Time-of-Check Time-of-Use (TOCTOU):

1. **Check Time**: Handler calls `ValidateExternalURL()` which resolves `attacker.com` to public IP `1.2.3.4` ✅ Allowed
2. **Use Time**: Milliseconds later, `TestURLConnectivity()` resolves `attacker.com` again but attacker has changed DNS to `127.0.0.1` ❌ SSRF!

### Our Defense-in-Depth Solution

**Triple-Layer Protection**:

1. **Layer 1: Format Validation** (`utils.ValidateURL()`)
   - Validates URL scheme (http/https only)
   - Blocks URLs with paths (prevents bypasses like `http://example.com/../internal`)
   - Rejects invalid URL formats

2. **Layer 2: Pre-Validation** (`security.ValidateExternalURL()`)
   - Performs DNS resolution
   - Validates resolved IPs against block list
   - Blocks embedded credentials (prevents `http://evil.com@127.0.0.1/` parser differentials)
   - **Purpose**: Breaks CodeQL taint chain and provides early fail-fast behavior

3. **Layer 3: Runtime Validation** (`TestURLConnectivity()` → `ssrfSafeDialer()`)
   - **Second DNS resolution** at connection time
   - Validates ALL resolved IPs before connecting
   - Uses first valid IP for connection (prevents IP hopping)
   - **Purpose**: Eliminates DNS rebinding/TOCTOU window

**From `backend/internal/utils/url_testing.go`**:

```go
// ssrfSafeDialer creates a custom dialer that validates IP addresses at connection time.
// This prevents DNS rebinding attacks by validating the IP just before connecting.
func ssrfSafeDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
 return func(ctx context.Context, network, addr string) (net.Conn, error) {
  // Resolve DNS with context timeout
  ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
  if err != nil {
   return nil, fmt.Errorf("DNS resolution failed: %w", err)
  }

  // Validate ALL resolved IPs - if any are private, reject immediately
  for _, ip := range ips {
   if isPrivateIP(ip.IP) {
    return nil, fmt.Errorf("access to private IP addresses is blocked (resolved to %s)", ip.IP)
   }
  }

  // Connect to the first valid IP (prevents DNS rebinding)
  dialer := &net.Dialer{Timeout: 5 * time.Second}
  return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
 }
}
```

### Why This Eliminates TOCTOU

Even if an attacker changes DNS between Layer 2 and Layer 3:

- Layer 2 validates at time T1: `attacker.com` → `1.2.3.4` ✅
- Attacker changes DNS
- Layer 3 resolves again at time T2: `attacker.com` → `127.0.0.1` ❌ **BLOCKED by ssrfSafeDialer**

The second validation happens **inside the HTTP client's dialer**, at the moment of connection. This closes the TOCTOU window.

---

## 6.6 URL Parser Differential Protection

### The Attack Vector

Some SSRF bypasses exploit differences in how various URL parsers interpret URLs:

```
http://evil.com@127.0.0.1/
```

**Vulnerable Parser** interprets as:

- User: `evil.com`
- Host: `127.0.0.1` ← SSRF target

**Strict Parser** interprets as:

- User: `evil.com`
- Host: `127.0.0.1`
- But rejects due to embedded credentials

### Our Protection

`security.ValidateExternalURL()` (in `url_validator.go`) rejects URLs with embedded credentials:

```go
if parsed.User != nil {
    return "", fmt.Errorf("URL must not contain embedded credentials")
}
```

This prevents parser differential attacks where credentials are used to disguise the real target.

---

## 6.7 Pattern Comparison: Why This Handler Uses Simple Validation

### Our Pattern (TestPublicURL)

```go
// Step 1: Format validation
normalized, _, err := utils.ValidateURL(req.URL)

// Step 2: SSRF pre-validation
validatedURL, err := security.ValidateExternalURL(normalized, security.WithAllowHTTP())

// Step 3: Test connectivity
reachable, latency, err := utils.TestURLConnectivity(validatedURL)
```

**Characteristics**:

- Two-step validation (format → security)
- Uses `ValidateExternalURL()` for simplicity
- Relies on `ssrfSafeDialer()` for runtime protection
- **Use Case**: Simple connectivity test, no custom IP selection needed

### Security Notifications Pattern

**Reference**: `backend/internal/api/handlers/security_notifications.go` (lines 50-65)

```go
if config.WebhookURL != "" {
    if _, err := security.ValidateExternalURL(config.WebhookURL,
        security.WithAllowLocalhost(),
        security.WithAllowHTTP(),
    ); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": fmt.Sprintf("Invalid webhook URL: %v", err),
        })
        return
    }
}
```

**Characteristics**:

- Single validation step
- Uses `WithAllowLocalhost()` option (allows testing webhooks locally)
- No connectivity test required (URL is stored, not immediately used)
- **Use Case**: Configuration storage, not immediate outbound request

### Why The Difference?

| Aspect | TestPublicURL | Security Notifications |
|--------|---------------|------------------------|
| **Immediate Use** | Yes, tests connectivity now | No, stores for later use |
| **Localhost** | Blocked (security risk) | Allowed (dev convenience) |
| **HTTP Allowed** | Yes (some APIs are HTTP-only) | Yes (same reason) |
| **Runtime Protection** | Yes (ssrfSafeDialer) | N/A (no immediate request) |

Both handlers use the same `ValidateExternalURL()` function but with different options based on their use case.

---

## 7. Testing Strategy

### 7.1 Unit Tests

- ✅ Test all SSRF scenarios (private IPs, localhost, cloud metadata)
- ✅ Test valid public URLs (ensure functionality still works)
- ✅ Test authorization (admin-only requirement)
- ✅ Test format validation errors
- ✅ Test connectivity failures (unreachable URLs)

### 7.2 Integration Tests

1. Deploy fix to staging environment
2. Test with real DNS resolution (ensure no false positives)
3. Verify error messages are user-friendly
4. Confirm public URLs are still testable

**Future Work**: Add DNS rebinding integration test:

- Set up test DNS server that changes responses dynamically
- Verify that `ssrfSafeDialer()` blocks the rebind attempt
- Test sequence: Initial DNS → Public IP (allowed) → DNS change → Private IP (blocked)
- This requires test infrastructure not currently available but should be prioritized for comprehensive security validation

### 7.3 Security Validation

1. Run CodeQL scan after implementation
2. Verify alert is resolved
3. Test with OWASP ZAP or Burp Suite to confirm SSRF protection
4. Penetration test with DNS rebinding attacks

---

## 8. Rollout Plan

### Phase 1: Implementation (Day 1)

- [ ] Implement code changes in `settings_handler.go`
- [ ] Add comprehensive unit tests
- [ ] Verify existing tests still pass

### Phase 2: Validation (Day 1-2)

- [ ] Run CodeQL scan locally
- [ ] Manual security testing
- [ ] Code review with security focus

### Phase 3: Deployment (Day 2-3)

- [ ] Merge to main branch
- [ ] Deploy to staging
- [ ] Run automated security scan
- [ ] Deploy to production

### Phase 4: Verification (Day 3+)

- [ ] Monitor logs for validation errors
- [ ] Verify CodeQL alert closure
- [ ] Update security documentation

---

## 9. Risk Assessment

### Security Risks (PRE-FIX)

- **Critical**: Admins could test internal endpoints (localhost, metadata services)
- **High**: Potential information disclosure about internal network topology
- **Medium**: DNS rebinding attack surface

### Security Risks (POST-FIX)

- **None**: Comprehensive SSRF protection at multiple layers

### Functional Risks

- **Low**: Error messages might be too restrictive if users try to test localhost
  - **Mitigation**: Clear error message explaining security rationale
- **Low**: Public URL testing might fail due to DNS issues
  - **Mitigation**: Existing timeout and error handling remains in place

---

## 10. Success Criteria

✅ **Security**:

- CodeQL SSRF alert resolved
- All SSRF test vectors blocked (localhost, private IPs, cloud metadata)
- No regression in other security scans

✅ **Functionality**:

- Public URLs can still be tested successfully
- Appropriate error messages for invalid/blocked URLs
- Latency measurements remain accurate

✅ **Code Quality**:

- 100% test coverage for new code paths
- No breaking changes to existing functionality
- Clear documentation and inline comments

---

## 11. References

- **OWASP SSRF**: <https://owasp.org/www-community/attacks/Server_Side_Request_Forgery>
- **CWE-918 (SSRF)**: <https://cwe.mitre.org/data/definitions/918.html>
- **DNS Rebinding Attacks**: <https://en.wikipedia.org/wiki/DNS_rebinding>
- **TOCTOU Vulnerabilities**: <https://cwe.mitre.org/data/definitions/367.html>
- **URL Parser Confusion**: <https://claroty.com/team82/research/exploiting-url-parsing-confusion>
- **Existing Implementation**: `backend/internal/security/url_validator.go`
- **Runtime Protection**: `backend/internal/utils/url_testing.go::ssrfSafeDialer()`
- **Comparison Pattern**: `backend/internal/api/handlers/security_notifications.go`

---

## Appendix: Alternative Mitigations Considered

### A. Whitelist-Only Approach

**Description**: Only allow testing URLs from a pre-configured whitelist.
**Rejected**: Too restrictive for legitimate admin use cases.

### B. Remove Test Endpoint

**Description**: Remove the `TestPublicURL` endpoint entirely.
**Rejected**: Legitimate functionality for validating public URL configuration.

### C. Client-Side Testing Only

**Description**: Move connectivity testing to the frontend.
**Rejected**: Cannot validate server-side reachability from client.

---

**Plan Status**: ✅ Revised and Ready for Implementation
**Revision Date**: 2025-12-23 (Post-Supervisor Review)
**Next Steps**:

1. Backend_Dev to implement Option A following this revised specification
2. Ensure HTTP status codes match existing test behavior (200 OK for SSRF blocks)
3. Use `err.Error()` directly without wrapping
4. Verify triple-layer protection works end-to-end

**Key Changes from Original Plan**:

- Fixed CodeQL taint analysis explanation (value transformation, not name recognition)
- Documented DNS rebinding/TOCTOU protection via ssrfSafeDialer
- Changed HTTP status codes to 200 OK for SSRF blocks (matches existing tests)
- Fixed error message handling (no wrapper prefix)
- Added URL parser differential protection documentation
- Added pattern comparison with security_notifications.go
- Added comprehensive architecture documentation for TestURLConnectivity
- Added missing test cases (empty URL, embedded credentials, WithAllowHTTP)
