# Issue #825: User Cannot Login After Fresh Install

**Date:** 2026-03-14
**Status:** Root Cause Identified — Code Bug + Frontend Fragility
**Issue:** Login API returns 200 but GET `/api/v1/auth/me` immediately returns 401
**Previous Plan:** Archived as `docs/plans/telegram_remediation_spec.md`

---

## 1. Introduction

A user reports that after a fresh install with remapped ports (`82:80`, `445:443`, `8080:8080`), accessing Charon via a separate external Caddy reverse proxy, the login succeeds (200) but the session validation (`/auth/me`) immediately fails (401).

### Objectives

1. Identify the root cause of the login→401 failure chain
2. Determine whether this is a code bug or a user configuration issue
3. Propose a targeted fix with minimal blast radius

---

## 2. Research Findings

### 2.1 Auth Login Flow

**File:** `backend/internal/api/handlers/auth_handler.go` (lines 172-189)

The `Login` handler:
1. Validates email/password via `authService.Login()`
2. Generates a JWT token (HS256, 24h expiry, includes `user_id`, `role`, `session_version`)
3. Sets an `auth_token` HttpOnly cookie via `setSecureCookie()`
4. Returns the token in the JSON response body: `{"token": "<jwt>"}`

The frontend (`frontend/src/pages/Login.tsx`, lines 43-46):
1. POSTs to `/auth/login` via the axios client (which has `withCredentials: true`)
2. Extracts the token from the response body
3. Calls `login(token)` on the AuthContext

### 2.2 Frontend AuthContext Login Flow

**File:** `frontend/src/context/AuthContext.tsx` (lines 84-110)

The `login()` function:
1. Stores the token in `localStorage` as `charon_auth_token`
2. Sets the `Authorization` header on the **axios** client via `setAuthToken(token)`
3. Calls `fetchSessionUser()` to validate the session

**Critical finding — `fetchSessionUser()` uses raw `fetch`, NOT the axios client:**

```typescript
const fetchSessionUser = useCallback(async (): Promise<User> => {
    const response = await fetch('/api/v1/auth/me', {
      method: 'GET',
      credentials: 'include',
      headers: { Accept: 'application/json' },
    });
    // ...
}, []);
```

This means `fetchSessionUser()` does NOT include the `Authorization: Bearer <token>` header. It relies **exclusively** on the browser sending the `auth_token` cookie via `credentials: 'include'`.

### 2.3 Cookie Secure Flag Logic

**File:** `backend/internal/api/handlers/auth_handler.go` (lines 132-163)

```go
func setSecureCookie(c *gin.Context, name, value string, maxAge int) {
    scheme := requestScheme(c)
    secure := true                     // ← Defaults to true
    sameSite := http.SameSiteStrictMode

    if scheme != "https" {
        sameSite = http.SameSiteLaxMode
        if isLocalRequest(c) {         // ← Only sets secure=false for localhost/127.0.0.1
            secure = false
        }
    }
    // ...
}
```

**`isLocalHost()` only matches `localhost` and loopback IPs:**

```go
func isLocalHost(host string) bool {
    if strings.EqualFold(host, "localhost") { return true }
    if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() { return true }
    return false
}
```

This function does **NOT** match:
- Private network IPs: `192.168.x.x`, `10.x.x.x`, `172.16.x.x`
- Custom hostnames: `charon.local`, `myserver.home`
- Any non-loopback IP address

### 2.4 Auth Middleware (Protects `/auth/me`)

**File:** `backend/internal/api/middleware/auth.go` (lines 12-45)

The `AuthMiddleware` extracts tokens in priority order:
1. `Authorization: Bearer <token>` header
2. `auth_token` cookie (fallback)
3. `?token=<token>` query parameter (deprecated fallback)

If no token is found, it returns `401 {"error": "Authorization header required"}`.

### 2.5 Route Registration

**File:** `backend/internal/api/routes/routes.go` (lines 260-267)

`/auth/me` is registered under the `protected` group which uses `authMiddleware`:
```go
protected.GET("/auth/me", authHandler.Me)
```

### 2.6 Database Migration & Seeding

- `AutoMigrate` runs on startup for all models including `User`, `Setting`, `SecurityConfig`
- The seed command (`backend/cmd/seed/main.go`) is a **separate CLI tool**, not run during normal startup
- Fresh install uses the `/api/v1/setup` endpoint to create the first admin user
- The setup handler creates the user and an ACME email setting in a transaction
- **No missing migration or seeding is involved in this bug** — tables are auto-migrated, and setup creates the user correctly

### 2.7 Trusted Proxy Configuration

**File:** `backend/internal/server/server.go` (lines 14-17)

```go
_ = router.SetTrustedProxies(nil)
```

Gin's `SetTrustedProxies(nil)` disables trusting forwarded headers for `c.ClientIP()`. However, the `requestScheme()` function reads `X-Forwarded-Proto` directly from the request header, bypassing Gin's trust mechanism. This is intentional for scheme detection.

### 2.8 Existing Test Confirmation

**File:** `backend/internal/api/handlers/auth_handler_test.go` (lines 84-99)

The test `TestSetSecureCookie_HTTP_Lax` explicitly asserts the current (buggy) behavior:
```go
// HTTP request from non-local IP 192.0.2.10
req := httptest.NewRequest("POST", "http://192.0.2.10/login", http.NoBody)
req.Header.Set("X-Forwarded-Proto", "http")
// ...
assert.True(t, c.Secure)  // ← Asserts Secure=true on HTTP!
```

Note: `192.0.2.10` is TEST-NET-1 (RFC 5737), a documentation address — NOT a private IP. This test is actually correct for public IPs and needs no change.

### 2.9 CORS Configuration

No CORS middleware was found in the backend. The frontend uses relative URLs (`baseURL: '/api/v1'`), so all API requests are same-origin. CORS is not a factor in this bug.

---

## 3. Root Cause Analysis

### Primary Root Cause: `Secure` cookie flag set to `true` on non-HTTPS, non-local connections

When a user accesses Charon from a LAN IP (e.g., `192.168.1.50:8080`) over plain HTTP:

| Step | Function | Value | Result |
|------|----------|-------|--------|
| 1 | `requestScheme(c)` | `"http"` | No X-Forwarded-Proto or TLS |
| 2 | `secure` default | `true` | — |
| 3 | `scheme != "https"` | `true` | Enters HTTP branch |
| 4 | `isLocalRequest(c)` | `false` | Host is `192.168.1.50`, not `localhost`/`127.0.0.1` |
| 5 | Final `secure` | `true` | **Cookie marked Secure on HTTP connection** |

**Result:** The browser receives `Set-Cookie: auth_token=...; Secure; HttpOnly; Path=/; SameSite=Lax` over an HTTP connection. Per RFC 6265bis §5.4, browsers **reject** `Secure` cookies delivered over non-secure (HTTP) channels.

### Secondary Root Cause: `fetchSessionUser()` has no fallback to Bearer token

Even though the JWT token is stored in `localStorage` and set on the axios client's `Authorization` header, `fetchSessionUser()` uses raw `fetch()` without the `Authorization` header. When the cookie is rejected, there is no fallback.

### Failure Chain

```
Browser (HTTP to 192.168.x.x:8080)
  → POST /auth/login → 200 + Set-Cookie: auth_token=...; Secure
  → Browser REJECTS Secure cookie (connection is HTTP)
  → Frontend stores token in localStorage, sets it on axios client
  → fetchSessionUser() calls GET /auth/me via raw fetch (no Auth header, no cookie)
  → Auth middleware: no token found → 401
  → User sees login failure
```

### External Caddy Scenario (likely works, but fragile)

When accessing via an external Caddy that terminates TLS:
- If Caddy sends `X-Forwarded-Proto: https` → `scheme = "https"` → `secure = true`, `sameSite = Strict`
- Browser sees HTTPS → accepts Secure cookie → `/auth/me` succeeds
- **But:** If the user accesses _directly_ on port 8080 for any reason, it breaks

---

## 4. Verdict

**This is a code bug, not a user configuration issue.**

The `setSecureCookie` function has a logic gap: when the scheme is HTTP and the request is from a non-loopback private IP, it still sets `Secure: true`. This makes it impossible to authenticate over HTTP from any non-localhost address, which is a valid and common deployment scenario (LAN access, Docker port mapping without TLS).

The secondary issue (frontend `fetchSessionUser` not sending a Bearer token) means there is no graceful fallback when the cookie is rejected — the user gets a hard 401 with no recovery path, even though the token is available in memory.

---

## 5. Technical Specification

### 5.1 Backend Fix: Expand `isLocalHost` to include RFC 1918 private IPs

**WHEN** the request scheme is HTTP,
**AND** the request originates from a private network IP (RFC 1918/RFC 4193),
**THE SYSTEM SHALL** set the `Secure` cookie flag to `false`.

**File:** `backend/internal/api/handlers/auth_handler.go` (line 80)

**Change:** Extend `isLocalHost` to also return `true` for RFC 1918 private IPs:

```go
func isLocalHost(host string) bool {
    if strings.EqualFold(host, "localhost") {
        return true
    }

    ip := net.ParseIP(host)
    if ip == nil {
        return false
    }

    if ip.IsLoopback() {
        return true
    }

    if ip.IsPrivate() {
        return true
    }

    return false
}
```

`net.IP.IsPrivate()` (Go 1.17+) checks for:
- `10.0.0.0/8`
- `172.16.0.0/12`
- `192.168.0.0/16`
- `fc00::/7` (IPv6 ULA)

This **does not** change behavior for public IPs or HTTPS — `Secure: true` is preserved for all HTTPS connections and for public HTTP connections.

### 5.2 Frontend Fix: Add Bearer token to `fetchSessionUser`

**File:** `frontend/src/context/AuthContext.tsx` (line 12)

**Change:** Include the `Authorization` header in `fetchSessionUser` when a token is available in localStorage:

```typescript
const fetchSessionUser = useCallback(async (): Promise<User> => {
    const headers: Record<string, string> = { Accept: 'application/json' };
    const stored = localStorage.getItem('charon_auth_token');
    if (stored) {
        headers['Authorization'] = `Bearer ${stored}`;
    }

    const response = await fetch('/api/v1/auth/me', {
        method: 'GET',
        credentials: 'include',
        headers,
    });

    if (!response.ok) {
        throw new Error('Session validation failed');
    }

    return response.json() as Promise<User>;
}, []);
```

This provides a belt-and-suspenders approach: the cookie is preferred (HttpOnly, auto-sent), but if the cookie is absent (rejected, cross-domain, etc.), the Bearer token from localStorage is used as a fallback.

### 5.3 Test Updates

**Existing test `TestSetSecureCookie_HTTP_Lax`:** Uses `192.0.2.10` (TEST-NET-1, RFC 5737) which is NOT a private IP → assertion unchanged (`Secure: true`).

**New test cases needed:**

| Test Name | Host | Scheme | Expected Secure | Expected SameSite |
|-----------|------|--------|-----------------|--------------------|
| `TestSetSecureCookie_HTTP_PrivateIP_Insecure` | `192.168.1.50` | `http` | `false` | `Lax` |
| `TestSetSecureCookie_HTTP_10Network_Insecure` | `10.0.0.5` | `http` | `false` | `Lax` |
| `TestSetSecureCookie_HTTP_172Network_Insecure` | `172.16.0.1` | `http` | `false` | `Lax` |
| `TestSetSecureCookie_HTTPS_PrivateIP_Secure` | `192.168.1.50` | `https` | `true` | `Strict` |
| `TestSetSecureCookie_HTTP_PublicIP_Secure` | `203.0.113.5` | `http` | `true` | `Lax` |

**`isLocalHost` unit test additions:**

| Input | Expected |
|-------|----------|
| `192.168.1.50` | `true` (new) |
| `10.0.0.1` | `true` (new) |
| `172.16.0.1` | `true` (new) |
| `203.0.113.5` | `false` |

---

## 6. Implementation Plan

### Phase 1: Backend Cookie Fix

1. Modify `isLocalHost` in `auth_handler.go` to include `ip.IsPrivate()`
2. Verify existing test `TestSetSecureCookie_HTTP_Lax` is unchanged (TEST-NET IP)
3. Add new test cases per table in §5.3
4. Add `isLocalHost` unit tests for private IPs

### Phase 2: Frontend `fetchSessionUser` Fix

1. Modify `fetchSessionUser` in `AuthContext.tsx` to include `Authorization` header from localStorage
2. Verify existing frontend tests still pass

### Phase 3: E2E Validation

1. Rebuild E2E Docker environment
2. Run the login/auth Playwright tests to validate no regressions

---

## 7. Acceptance Criteria

- [ ] `isLocalHost("192.168.1.50")` returns `true`
- [ ] `isLocalHost("10.0.0.1")` returns `true`
- [ ] `isLocalHost("172.16.0.1")` returns `true`
- [ ] `isLocalHost("203.0.113.5")` returns `false` (public IP unchanged)
- [ ] HTTP login from a private LAN IP sets `Secure: false` on `auth_token` cookie
- [ ] HTTPS login from a private LAN IP still sets `Secure: true`
- [ ] `fetchSessionUser()` sends `Authorization: Bearer <token>` when token is in localStorage
- [ ] All existing auth handler tests pass
- [ ] New test cases from §5.3 pass
- [ ] E2E login tests pass

---

## 8. Commit Slicing Strategy

**Decision:** Single PR

**Rationale:** Both changes are tightly coupled to the same authentication flow. The backend fix alone resolves the primary issue, and the frontend fix is a small defense-in-depth addition. Total change is ~20 lines of production code + ~60 lines of tests. Splitting would create unnecessary review overhead.

### PR-1: Fix auth cookie Secure flag for private networks + frontend Bearer fallback

**Scope:**
- `backend/internal/api/handlers/auth_handler.go` — Expand `isLocalHost` to include `ip.IsPrivate()`
- `backend/internal/api/handlers/auth_handler_test.go` — Add new test cases, verify existing
- `frontend/src/context/AuthContext.tsx` — Add Authorization header to `fetchSessionUser`

**Validation Gates:**
- `go test ./backend/internal/api/handlers/...` — all pass
- `go test ./backend/internal/api/middleware/...` — all pass
- E2E Playwright login suite — all pass

**Rollback:** Revert the single commit. No database changes, no API contract changes.

---

## 9. Edge Cases & Risks

| Risk | Mitigation |
|------|------------|
| `net.IP.IsPrivate()` requires Go 1.17+ | Charon requires Go 1.21+, no risk |
| Public HTTP deployments now get `Secure: true` (no change) | Intentional: public HTTP is insecure regardless |
| localStorage token exposed to XSS | Existing risk (unchanged); primary auth remains HttpOnly cookie |
| `isLocalHost` name now misleading (covers private IPs) | Consider renaming to `isPrivateOrLocalHost` in follow-up refactor |
| External reverse proxy without X-Forwarded-Proto | Frontend Bearer fallback covers this case now |
