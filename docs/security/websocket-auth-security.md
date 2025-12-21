---
title: WebSocket Authentication Security
description: Security documentation for WebSocket authentication in Charon. HttpOnly cookie implementation and token protection.
---

## WebSocket Authentication Security

### Overview

This document explains the security improvements made to WebSocket authentication in Charon to prevent JWT tokens from being exposed in access logs.

### Security Issue

### Before (Insecure)

Previously, WebSocket connections authenticated by passing the JWT token as a query parameter:

```
wss://example.com/api/v1/logs/live?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Security Risk:**

- Query parameters are logged in web server access logs (Caddy, nginx, Apache, etc.)
- Tokens appear in proxy logs
- Tokens may be stored in browser history
- Tokens can be captured in monitoring and telemetry systems
- An attacker with access to these logs can replay the token to impersonate a user

### After (Secure)

WebSocket connections now authenticate using HttpOnly cookies:

```
wss://example.com/api/v1/logs/live?source=waf&level=error
```

The browser automatically sends the `auth_token` cookie with the WebSocket upgrade request.

**Security Benefits:**

- ✅ HttpOnly cookies are **not logged** by web servers
- ✅ HttpOnly cookies **cannot be accessed** by JavaScript (XSS protection)
- ✅ Cookies are **not visible** in browser history
- ✅ Cookies are **not captured** in URL-based monitoring
- ✅ Token replay attacks are mitigated (tokens still have expiration)

## Implementation Details

### Frontend Changes

**Location:** `frontend/src/api/logs.ts`

Removed:

```typescript
const token = localStorage.getItem('charon_auth_token');
if (token) {
  params.append('token', token);
}
```

The browser automatically sends the `auth_token` cookie when establishing WebSocket connections due to:

1. The cookie is set by the backend during login with `HttpOnly`, `Secure`, and `SameSite` flags
2. The axios client has `withCredentials: true`, enabling cookie transmission

### Backend Changes

**Location:** `backend/internal/api/middleware/auth.go`

Authentication priority order:

1. **Authorization header** (Bearer token) - for API clients
2. **auth_token cookie** (HttpOnly) - **preferred for browsers and WebSockets**
3. **token query parameter** - **deprecated**, kept for backward compatibility only

The query parameter fallback is marked as deprecated and will be removed in a future version.

### Cookie Configuration

**Location:** `backend/internal/api/handlers/auth_handler.go`

The `auth_token` cookie is set with security best practices:

- **HttpOnly**: `true` - prevents JavaScript access (XSS protection)
- **Secure**: `true` (in production with HTTPS) - prevents transmission over HTTP
- **SameSite**: `Strict` (HTTPS) or `Lax` (HTTP/IP) - CSRF protection
- **Path**: `/` - available for all routes
- **MaxAge**: 24 hours - automatic expiration

## Verification

### Test Coverage

**Location:** `backend/internal/api/middleware/auth_test.go`

- `TestAuthMiddleware_Cookie` - verifies cookie authentication works
- `TestAuthMiddleware_QueryParamFallback` - verifies deprecated query param still works
- `TestAuthMiddleware_PrefersCookieOverQueryParam` - verifies cookie is prioritized over query param
- `TestAuthMiddleware_PrefersAuthorizationHeader` - verifies header takes highest priority

### Log Verification

To verify tokens are not logged:

1. **Before the fix:** Check Caddy access logs for token exposure:

   ```bash
   docker logs charon 2>&1 | grep "token=" | grep -o "token=[^&]*"
   ```

   Would show: `token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...`

2. **After the fix:** Check that WebSocket URLs are clean:

   ```bash
   docker logs charon 2>&1 | grep "/logs/live\|/cerberus/logs/ws"
   ```

   Shows: `/api/v1/logs/live?source=waf&level=error` (no token)

## Migration Path

### For Users

No action required. The change is transparent:

- Login sets the HttpOnly cookie
- WebSocket connections automatically use the cookie
- Existing sessions continue to work

### For API Clients

API clients using Authorization headers are unaffected.

### Deprecation Timeline

1. **Current:** Query parameter authentication is deprecated but still functional
2. **Future (v2.0):** Query parameter authentication will be removed entirely
3. **Recommendation:** Any custom scripts or tools should migrate to using Authorization headers or cookie-based authentication

## Related Documentation

- [Authentication Flow](../plans/prev_spec_websocket_fix_dec16.md#authentication-flow)
- [Security Best Practices](https://owasp.org/www-community/HttpOnly)
- [WebSocket Security](https://datatracker.ietf.org/doc/html/rfc6455#section-10)
