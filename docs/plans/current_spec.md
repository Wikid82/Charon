# Login Page Issues Fix - Comprehensive Implementation Plan (REVISED)

**Date:** December 21, 2025
**Revision:** Post-Supervisor Review - Critical Implementation Flaws Addressed
**Target:** Login page at `http://100.98.12.109:8080/login`
**Issues Identified:** 3

---

## Executive Summary

Three issues have been identified on the login page that need resolution:

1. **401 Unauthorized from `/api/v1/auth/me`** - Expected behavior during initialization
2. **Cross-Origin-Opener-Policy (COOP) header warning** - Browser warning on non-localhost HTTP
3. **Missing autocomplete attribute on password input** - Accessibility/DOM warning

This plan analyzes each issue, determines if it's a bug or expected behavior, and provides actionable fixes with specific file locations and implementation details.

### 🔴 CRITICAL REVISION NOTICE

**Supervisor Review Identified Critical Implementation Flaw:**

The original plan proposed checking `c.Request.TLS != nil` to detect HTTPS connections. This approach is **fundamentally broken** in reverse proxy architectures:

- **Problem:** Caddy terminates TLS before forwarding requests to the backend
- **Result:** `c.Request.TLS` is ALWAYS `nil`, making HTTPS detection impossible
- **Impact:** COOP header would never be set, even in production HTTPS

**Corrected Approaches:**

1. **Option A:** Check `X-Forwarded-Proto` header (requires Caddy configuration verification)
2. **Option B:** Set COOP only when NOT in development mode (simpler, recommended)

**Additional Requirements Added:**

- Verify Caddy forwards `X-Forwarded-Proto` header in reverse proxy config
- Add integration tests for proxy header propagation
- Document mixed content warnings for production HTTPS requirements
- Add autocomplete compliance considerations for regulated industries

This revision ensures the implementation will work correctly in production reverse proxy deployments.

---

## Issue 1: GET /api/v1/auth/me Returns 401 (Unauthorized)

### Status: EXPECTED BEHAVIOR (Minor Enhancement Possible)

### Root Cause Analysis

**File:** `frontend/src/context/AuthContext.tsx` (lines 10-24)

The `AuthProvider` component runs a `checkAuth()` function on mount that:

```typescript
useEffect(() => {
  const checkAuth = async () => {
    try {
      const stored = localStorage.getItem('charon_auth_token');
      if (stored) {
        setAuthToken(stored);
      }
      const response = await client.get('/auth/me');  // Line 16
      setUser(response.data);
    } catch {
      setAuthToken(null);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  checkAuth();
}, []);
```

**Why This Happens:**

- On first load (before login), no auth token exists in localStorage
- The `/auth/me` call is made to check if the user has a valid session
- The backend correctly returns 401 because no valid authentication exists
- This is **expected behavior** - the error is caught silently and doesn't affect UX

**Backend Authentication Flow:**

**File:** `backend/internal/api/routes/routes.go` (line 154)

```go
protected.GET("/auth/me", authHandler.Me)
```

**File:** `backend/internal/api/middleware/auth.go` (lines 12-35)

- Checks Authorization header first
- Falls back to `auth_token` cookie
- Falls back to `token` query parameter (deprecated)
- Returns 401 if no valid token found

### Assessment

**Is this a bug?** No - this is expected behavior for an unauthenticated user.

**User Impact:** None - the error is silently caught and doesn't display to the user.

**Browser Console Impact:** Minimal - developers see a 401 in Network tab, but this is normal for auth checks.

### Recommended Action: ENHANCEMENT (OPTIONAL)

If we want to eliminate the 401 from appearing in dev tools, we can optimize the auth check:

**Option A: Skip `/auth/me` call if no token in localStorage**

**File:** `frontend/src/context/AuthContext.tsx`

```typescript
useEffect(() => {
  const checkAuth = async () => {
    try {
      const stored = localStorage.getItem('charon_auth_token');
      if (!stored) {
        // No token stored, skip API call
        setIsLoading(false);
        return;
      }

      setAuthToken(stored);
      const response = await client.get('/auth/me');
      setUser(response.data);
    } catch {
      setAuthToken(null);
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  checkAuth();
}, []);
```

**Option B: Add a dedicated "check session" endpoint that returns 200 with `authenticated: false` instead of 401**

**Backend File:** `backend/internal/api/handlers/auth_handler.go` (lines 271-304)

The `VerifyStatus` handler already exists and returns:

```go
c.JSON(http.StatusOK, gin.H{
  "authenticated": false,
})
```

**Frontend Change:** Use `/auth/verify-status` instead of `/auth/me` in checkAuth

### Priority: LOW (Cosmetic Enhancement)

---

## Issue 2: Cross-Origin-Opener-Policy Header Warning

### Status: EXPECTED FOR HTTP DEV ENVIRONMENT (Documentation Needed)

### Root Cause Analysis

**File:** `backend/internal/api/middleware/security.go` (lines 61-62)

```go
// Cross-Origin-Opener-Policy: Isolate browsing context
c.Header("Cross-Origin-Opener-Policy", "same-origin")
```

**Why This Happens:**

- The COOP header `same-origin` is a security feature that isolates the browsing context
- Browser warns about COOP on HTTP (non-HTTPS) connections on non-localhost IPs
- The warning states: "Cross-Origin-Opener-Policy policy would block the window.closed call"

**COOP Header Purpose:**

- Prevents other origins from accessing the window object
- Protects against Spectre-like attacks
- Required for using `SharedArrayBuffer` and high-resolution timers

**Current Behavior:**

- Header is applied globally to all responses
- No conditional logic for development vs production
- Same header value for HTTP and HTTPS

**File:** `backend/internal/api/routes/routes.go` (lines 36-40)

```go
securityHeadersCfg := middleware.SecurityHeadersConfig{
  IsDevelopment: cfg.Environment == "development",
}
router.Use(middleware.SecurityHeaders(securityHeadersCfg))
```

The `IsDevelopment` flag is passed but currently only affects CSP directives, not COOP.

### Assessment

**Is this a bug?** No - this is expected behavior when accessing the app via HTTP on a non-localhost IP.

**User Impact:**

- Visual warning in browser console (Chrome/Edge DevTools)
- No functional impact on the application
- COOP doesn't break any existing functionality

**Security Impact:**

- COOP is a security enhancement and should remain in production (HTTPS)
- Can be relaxed for local development HTTP

### Recommended Action: CONDITIONAL COOP HEADER

**Phase 1: Make COOP conditional on HTTPS**

**File:** `backend/internal/api/middleware/security.go`

**Current Implementation:** (lines 61-62)

```go
// Cross-Origin-Opener-Policy: Isolate browsing context
c.Header("Cross-Origin-Opener-Policy", "same-origin")
```

**❌ ORIGINAL APPROACH (FLAWED):**

```go
// CRITICAL FLAW: c.Request.TLS will ALWAYS be nil behind a reverse proxy!
// Caddy terminates TLS before forwarding to the backend.
if c.Request.TLS != nil {  // ⚠️ THIS WILL NEVER BE TRUE
  c.Header("Cross-Origin-Opener-Policy", "same-origin")
}
```

**✅ CORRECT APPROACH (Option A - Check X-Forwarded-Proto):**

```go
// Cross-Origin-Opener-Policy: Isolate browsing context
// Only set on HTTPS to avoid browser warnings on HTTP development
// Reference: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cross-Origin-Opener-Policy
//
// IMPORTANT: Behind reverse proxy (Caddy), TLS is terminated at proxy level.
// Must check X-Forwarded-Proto header instead of c.Request.TLS
isHTTPS := c.GetHeader("X-Forwarded-Proto") == "https"

if isHTTPS {
  c.Header("Cross-Origin-Opener-Policy", "same-origin")
}
```

**✅ CORRECT APPROACH (Option B - Simpler for Dev/Prod Split):**

```go
// Cross-Origin-Opener-Policy: Isolate browsing context
// Skip in development mode to avoid browser warnings on HTTP
// In production, Caddy always uses HTTPS, so safe to set unconditionally
if !cfg.IsDevelopment {
  c.Header("Cross-Origin-Opener-Policy", "same-origin")
}
```

**Recommended Implementation: Option B** (simpler, avoids header dependency)

**Rationale:**
- Development mode = always HTTP → skip COOP to avoid warnings
- Production mode = always HTTPS (enforced by load balancer/Caddy) → always set COOP
- Eliminates need to parse X-Forwarded-Proto header
- Fails safe: if misconfigured, production gets COOP anyway

**Phase 2: Verify Proxy Header Configuration**

**⚠️ CRITICAL: Ensure Caddy forwards X-Forwarded-Proto header**

**File:** `backend/internal/caddy/config.go` (verify line ~1216)

**Required Configuration:**

```go
// In reverse_proxy directive
reverseProxy := map[string]interface{}{
  "handler": "reverse_proxy",
  "upstreams": upstreams,
  "headers": map[string]interface{}{
    "request": map[string]interface{}{
      "set": map[string][]string{
        "X-Forwarded-Proto": ["{http.request.scheme}"],
        "X-Forwarded-Host":  ["{http.request.host}"],
        "X-Real-IP":         ["{http.request.remote.host}"],
      },
    },
  },
}
```

**Verification Steps:**

1. Check that Caddy config includes `X-Forwarded-Proto` header
2. Add integration test to verify header propagation
3. Test both HTTP (development) and HTTPS (production) scenarios

**Phase 3: Update Documentation**

**File:** `docs/security.md` (create new section: "Production Deployment Considerations")

Add a section explaining:

- Why COOP warning appears on HTTP development
- That it's expected and safe to ignore in local dev
- That COOP is enforced in production HTTPS
- **⚠️ CRITICAL WARNING: All production endpoints MUST use HTTPS**
- **Mixed HTTP/HTTPS content will break COOP and secure cookies**
- How to test with HTTPS locally (self-signed cert or mkcert)

**Add Mixed Content Warning:**

```markdown
### ⚠️ Production HTTPS Requirements

**All production deployments MUST enforce HTTPS for the following reasons:**

1. **Security Headers:** COOP (`Cross-Origin-Opener-Policy`) should only be set over HTTPS
2. **Secure Cookies:** `auth_token` cookie uses `Secure` flag, requires HTTPS
3. **Mixed Content:** Mixing HTTP and HTTPS will cause browser warnings and broken functionality
4. **Load Balancer Configuration:** Ensure your load balancer/CDN:
   - Terminates TLS with valid certificates
   - Forwards `X-Forwarded-Proto: https` header to backend
   - Redirects HTTP → HTTPS (301 permanent redirect)

**Consequences of HTTP in Production:**

- COOP header triggers browser warnings
- Secure cookies are not sent by browser
- Authentication breaks (users can't login)
- WebSocket connections fail
- Password managers may not save credentials

**How to Verify:**

```bash
# Check that load balancer forwards X-Forwarded-Proto
curl -H "X-Forwarded-Proto: https" https://your-domain.com/api/v1/health

# Response headers should include:
# Cross-Origin-Opener-Policy: same-origin
# Strict-Transport-Security: max-age=31536000; includeSubDomains
```
```

### Tests to Update

**File:** `backend/internal/api/middleware/security_test.go`

**⚠️ CRITICAL: Old tests using `req.TLS` are INVALID for reverse proxy scenario**

Add these test cases:

```go
// Test development mode - COOP should NOT be set
func TestSecurityHeaders_COOP_DevelopmentMode(t *testing.T) {
  gin.SetMode(gin.TestMode)
  router := gin.New()
  cfg := SecurityHeadersConfig{IsDevelopment: true}
  router.Use(SecurityHeaders(cfg))
  router.GET("/test", func(c *gin.Context) {
    c.Status(http.StatusOK)
  })

  req := httptest.NewRequest("GET", "/test", nil)
  resp := httptest.NewRecorder()
  router.ServeHTTP(resp, req)

  // COOP should NOT be set in development mode
  assert.Empty(t, resp.Header().Get("Cross-Origin-Opener-Policy"),
    "COOP header should not be set in development mode")
}

// Test production mode - COOP SHOULD be set
func TestSecurityHeaders_COOP_ProductionMode(t *testing.T) {
  gin.SetMode(gin.TestMode)
  router := gin.New()
  cfg := SecurityHeadersConfig{IsDevelopment: false}
  router.Use(SecurityHeaders(cfg))
  router.GET("/test", func(c *gin.Context) {
    c.Status(http.StatusOK)
  })

  req := httptest.NewRequest("GET", "/test", nil)
  resp := httptest.NewRecorder()
  router.ServeHTTP(resp, req)

  // COOP SHOULD be set in production mode
  assert.Equal(t, "same-origin", resp.Header().Get("Cross-Origin-Opener-Policy"),
    "COOP header must be set in production mode")
}

// ALTERNATIVE: If implementing Option A (X-Forwarded-Proto check)
// Test HTTP via proxy - COOP should NOT be set
func TestSecurityHeaders_COOP_HTTPViaProxy(t *testing.T) {
  gin.SetMode(gin.TestMode)
  router := gin.New()
  cfg := SecurityHeadersConfig{IsDevelopment: false}
  router.Use(SecurityHeaders(cfg))
  router.GET("/test", func(c *gin.Context) {
    c.Status(http.StatusOK)
  })

  req := httptest.NewRequest("GET", "/test", nil)
  req.Header.Set("X-Forwarded-Proto", "http")
  resp := httptest.NewRecorder()
  router.ServeHTTP(resp, req)

  // COOP should NOT be set when X-Forwarded-Proto is http
  assert.Empty(t, resp.Header().Get("Cross-Origin-Opener-Policy"),
    "COOP should not be set for HTTP requests (even in production)")
}

// Test HTTPS via proxy - COOP SHOULD be set
func TestSecurityHeaders_COOP_HTTPSViaProxy(t *testing.T) {
  gin.SetMode(gin.TestMode)
  router := gin.New()
  cfg := SecurityHeadersConfig{IsDevelopment: false}
  router.Use(SecurityHeaders(cfg))
  router.GET("/test", func(c *gin.Context) {
    c.Status(http.StatusOK)
  })

  req := httptest.NewRequest("GET", "/test", nil)
  req.Header.Set("X-Forwarded-Proto", "https")
  resp := httptest.NewRecorder()
  router.ServeHTTP(resp, req)

  // COOP SHOULD be set when X-Forwarded-Proto is https
  assert.Equal(t, "same-origin", resp.Header().Get("Cross-Origin-Opener-Policy"),
    "COOP must be set for HTTPS requests")
}
```

**Integration Test for Proxy Headers:**

**File:** `backend/integration/proxy_headers_test.go` (new file)

```go
package integration

import (
  "net/http"
  "testing"
  "github.com/stretchr/testify/assert"
  "github.com/stretchr/testify/require"
)

// TestProxyHeaderPropagation verifies that Caddy forwards X-Forwarded-Proto
func TestProxyHeaderPropagation(t *testing.T) {
  // This test requires the full stack (Caddy + Backend) to be running
  if testing.Short() {
    t.Skip("Skipping integration test in short mode")
  }

  tests := []struct {
    name           string
    requestScheme  string
    expectCOOP     bool
  }{
    {
      name:          "HTTP request should not have COOP",
      requestScheme: "http",
      expectCOOP:    false,
    },
    {
      name:          "HTTPS request should have COOP",
      requestScheme: "https",
      expectCOOP:    true,
    },
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      // Make request through Caddy proxy
      req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/health", nil)
      require.NoError(t, err)

      // Simulate load balancer setting X-Forwarded-Proto
      req.Header.Set("X-Forwarded-Proto", tt.requestScheme)

      client := &http.Client{}
      resp, err := client.Do(req)
      require.NoError(t, err)
      defer resp.Body.Close()

      coopHeader := resp.Header.Get("Cross-Origin-Opener-Policy")

      if tt.expectCOOP {
        assert.Equal(t, "same-origin", coopHeader,
          "COOP header should be present for HTTPS")
      } else {
        assert.Empty(t, coopHeader,
          "COOP header should not be present for HTTP")
      }
    })
  }
}
```

### Priority: MEDIUM (User-facing warning, but not breaking)

---

## Issue 3: Missing Autocomplete Attribute on Password Input

### Status: ACCESSIBILITY BUG (MUST FIX)

### Root Cause Analysis

**File:** `frontend/src/pages/Login.tsx` (lines 93-100)

```tsx
<Input
  label={t('auth.password')}
  type="password"
  value={password}
  onChange={e => setPassword(e.target.value)}
  required
  placeholder="••••••••"
  disabled={loading}
/>
```

**Missing:** `autoComplete` attribute

**Why This Matters:**

1. **Accessibility:** Password managers rely on autocomplete to identify password fields
2. **User Experience:** Browsers can't offer to save/fill passwords without proper attributes
3. **DOM Standards:** HTML5 spec recommends autocomplete for all input fields
4. **Security:** Modern password managers use autocomplete to prevent phishing

**Component Implementation:**

**File:** `frontend/src/components/ui/Input.tsx` (lines 1-95)

The `Input` component is a controlled component that forwards all props to the native `<input>` element:

```tsx
export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  // Custom props...
}

<input
  ref={ref}
  type={isPassword ? (showPassword ? 'text' : 'password') : type}
  disabled={disabled}
  className={...}
  {...props}  // Line 64 - All other props are spread here
/>
```

The component already supports `autoComplete` through the spread operator, it just needs to be passed from the parent.

### Recommended Action: ADD AUTOCOMPLETE ATTRIBUTES

**Phase 1: Fix Login Page**

**File:** `frontend/src/pages/Login.tsx`

**Email Input** (lines 84-91):

```tsx
<Input
  label={t('auth.email')}
  type="email"
  value={email}
  onChange={e => setEmail(e.target.value)}
  required
  placeholder="admin@example.com"
  disabled={loading}
  autoComplete="email"  // ADD THIS
/>
```

**Password Input** (lines 93-100):

```tsx
<Input
  label={t('auth.password')}
  type="password"
  value={password}
  onChange={e => setPassword(e.target.value)}
  required
  placeholder="••••••••"
  disabled={loading}
  autoComplete="current-password"  // ADD THIS
/>
```

**Phase 2: Fix Setup Page**

**File:** `frontend/src/pages/Setup.tsx`

**Email Input** (lines 121-129):

```tsx
<Input
  id="email"
  name="email"
  label={t('setup.emailLabel')}
  type="email"
  required
  placeholder={t('setup.emailPlaceholder')}
  value={formData.email}
  onChange={(e) => setFormData({ ...formData, email: e.target.value })}
  className={...}
  autoComplete="email"  // ADD THIS
/>
```

**Password Input** (lines 135-142):

```tsx
<Input
  id="password"
  name="password"
  label={t('setup.passwordLabel')}
  type="password"
  required
  placeholder="••••••••"
  value={formData.password}
  onChange={(e) => setFormData({ ...formData, password: e.target.value })}
  autoComplete="new-password"  // ADD THIS - "new-password" for registration forms
/>
```

**Phase 3: Fix Account Page (Password Change)**

**File:** `frontend/src/pages/Account.tsx`

**Current Password** (lines 376-381):

```tsx
<Input
  id="current-password"
  type="password"
  value={oldPassword}
  onChange={(e) => setOldPassword(e.target.value)}
  required
  autoComplete="current-password"  // ADD THIS
/>
```

**New Password** (lines 386-391):

```tsx
<Input
  id="new-password"
  type="password"
  value={newPassword}
  onChange={(e) => setNewPassword(e.target.value)}
  required
  autoComplete="new-password"  // ADD THIS
/>
```

**Confirm Password** (lines 398-403):

```tsx
<Input
  id="confirm-password"
  type="password"
  value={confirmPassword}
  onChange={(e) => setConfirmPassword(e.target.value)}
  required
  error={...}
  autoComplete="new-password"  // ADD THIS
/>
```

**Phase 4: Fix SMTP Settings Page**

**File:** `frontend/src/pages/SMTPSettings.tsx`

**SMTP Username** (lines 172-178):

```tsx
<Input
  id="smtp-username"
  type="text"
  value={username}
  onChange={(e) => setUsername(e.target.value)}
  placeholder="your@email.com"
  autoComplete="username"  // ADD THIS
/>
```

**SMTP Password** (lines 182-188):

```tsx
<Input
  id="smtp-password"
  type="password"
  value={password}
  onChange={(e) => setPassword(e.target.value)}
  placeholder="••••••••"
  helperText={t('smtp.passwordHelper')}
  autoComplete="current-password"  // ADD THIS
/>
```

**Phase 5: Fix Accept Invite Page**

**File:** `frontend/src/pages/AcceptInvite.tsx`

**Password** (lines 169-175):

```tsx
<Input
  label={t('auth.password')}
  type="password"
  value={password}
  onChange={(e) => setPassword(e.target.value)}
  placeholder="••••••••"
  required
  autoComplete="new-password"  // ADD THIS - new account being created
/>
```

**Confirm Password** (lines 178-190):

```tsx
<Input
  label={t('acceptInvite.confirmPassword')}
  type="password"
  value={confirmPassword}
  onChange={(e) => setConfirmPassword(e.target.value)}
  placeholder="••••••••"
  required
  error={...}
  autoComplete="new-password"  // ADD THIS
/>
```

### Autocomplete Values Reference

According to HTML5 spec (https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#autofill):

- `email` - Email address
- `username` - Username or account name
- `current-password` - Current password (for login)
- `new-password` - New password (for registration or password change)

### Tests to Add/Update

**File:** `frontend/src/pages/__tests__/Login.test.tsx`

```typescript
it('has proper autocomplete attributes for password managers', () => {
  renderWithProviders(<Login />)

  const emailInput = screen.getByPlaceholderText(/admin@example.com/i)
  const passwordInput = screen.getByPlaceholderText(/••••••••/i)

  expect(emailInput).toHaveAttribute('autocomplete', 'email')
  expect(passwordInput).toHaveAttribute('autocomplete', 'current-password')
})
```

### Autocomplete Security Considerations

**⚠️ NOTE: Some regulated industries may require disabling autocomplete**

**OWASP/NIST Recommendation:** Modern security guidelines **recommend AGAINST** disabling autocomplete:

- Password managers improve security by enabling stronger, unique passwords
- Users reuse weak passwords when managers are blocked
- Disabling autocomplete reduces security, not improves it

**References:**
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html#password-managers)
- [NIST SP 800-63B Section 5.1.1.2](https://pages.nist.gov/800-63-3/sp800-63b.html#memsecretver)

**If compliance requires disabling autocomplete:**

Implement as **opt-in** environment variable (not default):

```bash
# .env
DISABLE_PASSWORD_AUTOCOMPLETE=true  # Only for specific compliance requirements
```

**Implementation:**

```tsx
// frontend/src/pages/Login.tsx
const disableAutocomplete = import.meta.env.VITE_DISABLE_PASSWORD_AUTOCOMPLETE === 'true'

<Input
  autoComplete={disableAutocomplete ? "off" : "current-password"}
  // ... other props
/>
```

**Default Behavior:** Autocomplete ENABLED (best practice)

### Priority: HIGH (Accessibility and UX impact)

---

## Configuration File Review

### .gitignore

**File:** `/projects/Charon/.gitignore`

**Current State:** Well-structured and comprehensive

**Recommended Changes:** None - the file properly excludes:

- Coverage artifacts (`*.cover`, `*.html`, `coverage/`)
- Test outputs (`test-results/`, `*.sarif`)
- Docker overrides (`docker-compose.override.yml`)
- Temporary files at root (`/caddy_*.json`, `/trivy-*.txt`)

**Verification:**

- ✅ Excludes test artifacts
- ✅ Excludes build outputs
- ✅ Excludes sensitive files (`.env`)
- ✅ Excludes CodeQL/security scan results

### codecov.yml

**Status:** File does not exist in repository

**Finding:** `file_search` and `read_file` both confirm no `codecov.yml` exists

**Recommendation:**

- If using Codecov for coverage reporting, create a `codecov.yml` at root
- If not using Codecov, no action needed
- Current CI/CD workflows may use inline coverage settings

**Suggested Content** (if needed):

```yaml
# codecov.yml - Code coverage configuration
coverage:
  status:
    project:
      default:
        target: 85%
        threshold: 1%
    patch:
      default:
        target: 80%
        threshold: 1%

comment:
  layout: "reach,diff,flags,files"
  behavior: default
  require_changes: false

ignore:
  - "**/__tests__/**"
  - "**/*.test.ts"
  - "**/*.test.tsx"
  - "**/test-*.ts"
```

**Priority:** LOW (Only if using Codecov service)

### .dockerignore

**File:** `/projects/Charon/.dockerignore`

**Current State:** Well-maintained and comprehensive

**Recommended Changes:** None - the file properly excludes:

- Build artifacts and coverage files
- Test directories
- Node modules
- Git and CI/CD directories
- Documentation (except key files)
- CodeQL and security scan results

**Verification:**

- ✅ Reduces Docker build context size
- ✅ Excludes test artifacts
- ✅ Keeps README and LICENSE
- ✅ Excludes sensitive files

### Dockerfile

**File:** `/projects/Charon/Dockerfile`

**Current State:** Multi-stage build with security best practices

**Recommended Changes:** None - the Dockerfile already implements:

- ✅ Multi-stage builds (frontend, backend, Caddy, CrowdSec builders)
- ✅ Non-root user (`charon:charon` with UID/GID 1000)
- ✅ Healthcheck endpoint
- ✅ Security labels (OCI image spec)
- ✅ Minimal runtime dependencies
- ✅ Recent base images (Alpine 3.23, Go 1.25, Node 24.12)

**Security Verification:**

- ✅ Runs as non-root user (line 366: `USER charon`)
- ✅ Includes security scanning (Trivy in CI/CD)
- ✅ Uses HEALTHCHECK for monitoring
- ✅ Proper volume permissions handled in entrypoint

---

## Implementation Plan

### Phase 1: Quick Wins (1-2 hours)

**Priority: HIGH**

1. **Add autocomplete attributes to all password/email inputs**
   - Files: `Login.tsx`, `Setup.tsx`, `Account.tsx`, `AcceptInvite.tsx`, `SMTPSettings.tsx`
   - Impact: Immediate UX and accessibility improvement
   - Testing: Manual test with browser password manager
   - Add unit tests for autocomplete attributes

2. **Write tests for autocomplete attributes**
   - File: `frontend/src/pages/__tests__/Login.test.tsx`
   - Verify email and password inputs have correct attributes

### Phase 2: Documentation (1 hour)

**Priority: MEDIUM**

1. **Document COOP warning in development**
   - Create or update: `docs/getting-started.md` or `docs/security.md`
   - Explain why the warning appears on HTTP
   - Provide context about COOP security benefits
   - Optional: Add instructions for local HTTPS testing

2. **Document auth flow in README or architecture docs**
   - Explain that 401 on `/auth/me` during login is expected
   - Describe the three-tier authentication (header > cookie > query param)

### Phase 3: Optional Enhancements (2-3 hours)

**Priority: LOW**

1. **Optimize AuthContext to skip `/auth/me` if no token**
   - File: `frontend/src/context/AuthContext.tsx`
   - Reduces unnecessary 401 errors in console
   - Slightly faster initial load

2. **Make COOP header conditional on HTTPS**
   - File: `backend/internal/api/middleware/security.go`
   - Add logic to skip COOP on HTTP development
   - Update tests to verify conditional behavior
   - Testing: Verify COOP is present on HTTPS, absent on HTTP dev

---

## Testing Strategy

### Unit Tests

**Frontend:**

- `frontend/src/pages/__tests__/Login.test.tsx` - Add autocomplete verification
- `frontend/src/pages/__tests__/Setup.test.tsx` - Verify autocomplete on setup form
- `frontend/src/components/ui/__tests__/Input.test.tsx` - Verify autocomplete prop forwarding

**Backend:**

- `backend/internal/api/middleware/security_test.go` - Add COOP conditional tests
- `backend/internal/api/middleware/auth_test.go` - Verify existing auth flow (already comprehensive)

### Integration Tests

1. **Manual Testing:**
   - Navigate to login page without existing session
   - Verify browser DevTools shows autocomplete attributes
   - Test password manager save/fill functionality
   - Check browser console for COOP warning (should exist on HTTP, not on HTTPS)

2. **E2E Testing (if Playwright is set up):**
   - Test login flow with password manager
   - Verify autocomplete suggestions appear

### Acceptance Criteria

**Issue 1 (401 Error):**

- ✅ Documented as expected behavior
- ✅ Optional: AuthContext skips API call if no token

**Issue 2 (COOP Warning):**

- ✅ COOP header conditional on HTTPS (or documented as expected)
- ✅ Tests verify conditional behavior
- ✅ Documentation explains the warning

**Issue 3 (Autocomplete):**

- ✅ All password fields have `autoComplete="current-password"` or `"new-password"`
- ✅ All email fields have `autoComplete="email"`
- ✅ All username fields have `autoComplete="username"`
- ✅ Tests verify autocomplete attributes
- ✅ Password managers can save and fill credentials

---

## Risk Assessment

### Low Risk

- Adding autocomplete attributes (native HTML feature)
- Documentation updates

### Medium Risk

- Making COOP conditional (requires testing on multiple browsers)
- Modifying AuthContext initialization (could affect auth flow)

### Mitigation

- Comprehensive unit and integration tests
- Manual testing on Chrome, Firefox, Safari
- Rollback plan: revert commits if issues arise

---

## Files Requiring Changes

### Frontend Files (High Priority)

1. `frontend/src/pages/Login.tsx` - Add autocomplete to email/password inputs
2. `frontend/src/pages/Setup.tsx` - Add autocomplete to registration form
3. `frontend/src/pages/Account.tsx` - Add autocomplete to password change form
4. `frontend/src/pages/AcceptInvite.tsx` - Add autocomplete to invite acceptance form
5. `frontend/src/pages/SMTPSettings.tsx` - Add autocomplete to SMTP credentials
6. `frontend/src/pages/__tests__/Login.test.tsx` - Add autocomplete attribute tests
7. `frontend/src/context/AuthContext.tsx` - (Optional) Optimize checkAuth

### Backend Files (Medium Priority)

1. `backend/internal/api/middleware/security.go` - Make COOP conditional
2. `backend/internal/api/middleware/security_test.go` - Add COOP conditional tests

### Documentation Files (Medium Priority)

1. `docs/getting-started.md` or `docs/security.md` - Document COOP warning
2. `README.md` - (Optional) Add auth flow documentation

### Configuration Files

1. `.gitignore` - ✅ No changes needed
2. `.dockerignore` - ✅ No changes needed
3. `Dockerfile` - ✅ No changes needed
4. `codecov.yml` - ✅ Does not exist (create only if using Codecov)

---

## Summary

| Issue | Status | Priority | Effort | Risk |
|-------|--------|----------|--------|------|
| 401 on /auth/me | Expected Behavior | LOW | 1h (optional optimization) | Low |
| COOP Header Warning | Expected on HTTP | MEDIUM | 2h | Medium |
| Missing Autocomplete | Bug | HIGH | 2h | Low |

**Total Estimated Effort:** 6-8 hours (includes proxy verification, testing, and documentation)

**Time Breakdown:**
- Autocomplete fixes: 2 hours
- COOP implementation fix: 2 hours
- Proxy header verification: 1 hour
- Integration tests: 2 hours
- Documentation: 1-2 hours

**Recommended Order:**

1. Fix autocomplete attributes (HIGH priority, LOW risk, immediate user benefit)
2. Document COOP warning (MEDIUM priority, no code changes)
3. Optionally make COOP conditional (MEDIUM priority, requires testing)
4. Optionally optimize AuthContext (LOW priority, minor improvement)

---

## Appendix A: Related Files and Components

### Authentication Flow Components

- `frontend/src/context/AuthContext.tsx` - Main auth state management
- `frontend/src/context/AuthContextValue.ts` - Auth context type definitions
- `frontend/src/hooks/useAuth.ts` - Auth hook for components
- `frontend/src/api/client.ts` - Axios client with auth interceptor
- `frontend/src/components/RequireAuth.tsx` - Route guard component
- `backend/internal/api/middleware/auth.go` - Auth middleware
- `backend/internal/api/handlers/auth_handler.go` - Auth endpoints
- `backend/internal/services/auth_service.go` - Auth business logic

### Security Headers Components

- `backend/internal/api/middleware/security.go` - Security headers middleware
- `backend/internal/api/middleware/security_test.go` - Security middleware tests
- `backend/internal/caddy/config.go` - Caddy security header config (line 1216)

### Form Components

- `frontend/src/components/ui/Input.tsx` - Reusable input component
- `frontend/src/components/PasswordStrengthMeter.tsx` - Password validation UI

---

## Appendix B: Browser Compatibility

### Autocomplete Attribute Support

| Browser | Version | Support |
|---------|---------|---------|
| Chrome | 14+ | ✅ Full support |
| Firefox | 4+ | ✅ Full support |
| Safari | 6+ | ✅ Full support |
| Edge | 12+ | ✅ Full support |

### COOP Header Support

| Browser | Version | Support |
|---------|---------|---------|
| Chrome | 83+ | ✅ Full support |
| Firefox | 79+ | ✅ Full support |
| Safari | 15.2+ | ✅ Full support |
| Edge | 83+ | ✅ Full support |

**Note:** All modern browsers support both features. Legacy browser users (IE11 and below) will safely ignore these attributes/headers.

---

## Appendix C: Relevant Documentation Links

- [HTML Autocomplete Spec](https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#autofill)
- [MDN: autocomplete attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Attributes/autocomplete)
- [MDN: Cross-Origin-Opener-Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cross-Origin-Opener-Policy)
- [OWASP: Authentication Best Practices](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [X-Forwarded-Proto Header Spec](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Proto)
- [Caddy Reverse Proxy Documentation](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)

---

## Appendix D: Reverse Proxy Architecture Considerations

### Why `c.Request.TLS` Doesn't Work Behind a Reverse Proxy

**Architecture Overview:**

```
[Client Browser] --HTTPS--> [Load Balancer/Caddy] --HTTP--> [Backend (Go/Gin)]
                              ↑                              ↑
                         TLS terminates here          c.Request.TLS == nil
```

**Key Points:**

1. **TLS Termination:** Load balancers and reverse proxies (Caddy, nginx, HAProxy) terminate TLS connections
2. **Backend Protocol:** Communication between proxy and backend is typically plain HTTP over private network
3. **Request Object:** Go's `http.Request.TLS` field is only populated for direct TLS connections
4. **Detection Method:** Backend must rely on headers set by the proxy (`X-Forwarded-Proto`, `X-Forwarded-For`)

**Common Pitfalls:**

```go
// ❌ WRONG: Will always be false behind reverse proxy
if c.Request.TLS != nil {
  // This code is never reached!
}

// ✅ CORRECT: Check forwarded protocol header
if c.GetHeader("X-Forwarded-Proto") == "https" {
  // This works correctly
}

// ✅ ALSO CORRECT: Trust deployment configuration
if !cfg.IsDevelopment {
  // Production = HTTPS enforced at load balancer
}
```

**Security Implications:**

1. **Trust Boundary:** Backend must trust headers set by reverse proxy
2. **Header Spoofing:** If backend is directly exposed (bypassing proxy), malicious clients could set `X-Forwarded-Proto: https`
3. **Mitigation:** Ensure backend only accepts connections from trusted proxy (firewall rules, network policies)

**Testing Considerations:**

- Unit tests cannot test `c.Request.TLS` behavior in proxy scenarios
- Integration tests must include full proxy stack
- Mock `X-Forwarded-Proto` header in tests to simulate proxy behavior

**Production Deployment Checklist:**

- [ ] Verify load balancer/CDN forwards `X-Forwarded-Proto` header
- [ ] Confirm backend firewall blocks direct public access
- [ ] Test HTTPS redirect (HTTP → HTTPS 301)
- [ ] Verify `Strict-Transport-Security` header is set
- [ ] Check that secure cookies (`Secure` flag) work correctly
- [ ] Validate COOP header is present on HTTPS responses
- [ ] Test WebSocket connections over HTTPS

---

**End of Plan**
