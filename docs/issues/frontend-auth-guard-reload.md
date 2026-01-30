# [Frontend] Add Auth Guard on Page Reload

## Summary

The frontend does not validate authentication state on page load/reload. When a user's session expires or authentication tokens are cleared, reloading the page should redirect to `/login`, but currently it does not.

## Failing Test

- **File**: `tests/core/authentication.spec.ts`
- **Test**: `should redirect to login when session expires`
- **Line**: ~310

## Steps to Reproduce

1. Log in to the application
2. Open browser dev tools
3. Clear localStorage and cookies
4. Reload the page
5. **Expected**: Redirect to `/login`
6. **Actual**: Page remains on current route (e.g., `/dashboard`)

---

## Research Findings

### Auth Architecture Overview

| File | Purpose |
|------|---------|
| [context/AuthContext.tsx](../../frontend/src/context/AuthContext.tsx) | Main `AuthProvider` - manages user state, login/logout, token handling |
| [context/AuthContextValue.ts](../../frontend/src/context/AuthContextValue.ts) | Type definitions: `User`, `AuthContextType` |
| [hooks/useAuth.ts](../../frontend/src/hooks/useAuth.ts) | Custom hook to access auth context |
| [components/RequireAuth.tsx](../../frontend/src/components/RequireAuth.tsx) | Route guard - redirects to `/login` if not authenticated |
| [api/client.ts](../../frontend/src/api/client.ts) | Axios instance with auth token handling |
| [App.tsx](../../frontend/src/App.tsx) | Router setup with `AuthProvider` and `RequireAuth` |

### Current Auth Flow

```
Page Load → AuthProvider.useEffect() → checkAuth()
                                           │
                            ┌──────────────┴──────────────┐
                            ▼                             ▼
                    localStorage.get()               GET /auth/me
                    setAuthToken(stored)                   │
                                              ┌───────────┴───────────┐
                                              ▼                       ▼
                                          Success                   Error
                                        setUser(data)         setUser(null)
                                                              setAuthToken(null)
                                              │                       │
                                              ▼                       ▼
                                        isLoading=false         isLoading=false
                                        isAuthenticated=true    isAuthenticated=false
```

### Current Implementation (AuthContext.tsx lines 9-25)

```typescript
useEffect(() => {
  const checkAuth = async () => {
    try {
      const stored = localStorage.getItem('charon_auth_token');
      if (stored) {
        setAuthToken(stored);
      }
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

### RequireAuth Component (RequireAuth.tsx)

```typescript
const RequireAuth: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return <LoadingOverlay message="Authenticating..." />;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return children;
};
```

### API Client 401 Handler (client.ts lines 23-31)

```typescript
client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      console.warn('Authentication failed:', error.config?.url);
    }
    return Promise.reject(error);
  }
);
```

---

## Root Cause Analysis

**The existing implementation already handles this correctly!**

Looking at the code flow:

1. **AuthProvider** runs `checkAuth()` on mount (`useEffect` with `[]`)
2. It calls `GET /auth/me` to validate the session
3. On error (401), it sets `user = null` and `isAuthenticated = false`
4. **RequireAuth** reads `isAuthenticated` and redirects to `/login` if false

**The issue is likely one of:**

1. **Race condition**: `RequireAuth` renders before `checkAuth()` completes
2. **Token without validation**: If token exists in localStorage but is invalid, the `GET /auth/me` fails, but something may not be updating properly
3. **Caching issue**: `isLoading` may not be set correctly on certain paths

### Verified Behavior

- `isLoading` starts as `true` (line 8)
- `RequireAuth` shows loading overlay while `isLoading` is true
- `checkAuth()` sets `isLoading=false` in `finally` block
- If `/auth/me` fails, `user=null` → `isAuthenticated=false` → redirect to `/login`

**This should work!** Need to verify with E2E test what's actually happening.

---

## Potential Issues to Investigate

### 1. API Client Not Clearing Token on 401

The interceptor only logs, doesn't clear state:

```typescript
if (error.response?.status === 401) {
  console.warn('Authentication failed:', error.config?.url);  // Just logs!
}
```

### 2. No Global Auth State Reset

When a 401 occurs on any API call (not just `/auth/me`), there's no mechanism to force logout.

### 3. localStorage Token Persists After Session Expiry

Backend sessions expire, but frontend keeps the localStorage token.

---

## Recommended Solution

### Option A: Enhanced API Interceptor (Minimal Change) ✅ RECOMMENDED

Modify [api/client.ts](../../frontend/src/api/client.ts) to clear auth state on 401:

```typescript
// Add global auth reset callback
let onAuthError: (() => void) | null = null;

export const setOnAuthError = (callback: (() => void) | null) => {
  onAuthError = callback;
};

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      console.warn('Authentication failed:', error.config?.url);
      localStorage.removeItem('charon_auth_token');
      setAuthToken(null);
      onAuthError?.();  // Trigger state reset
    }
    return Promise.reject(error);
  }
);
```

Then in **AuthContext.tsx**, register the callback:

```typescript
useEffect(() => {
  setOnAuthError(() => {
    setUser(null);
    // Navigate will happen via RequireAuth
  });
  return () => setOnAuthError(null);
}, []);
```

### Option B: Direct Window Navigation (Simpler)

In the 401 interceptor, redirect immediately:

```typescript
if (error.response?.status === 401 && !error.config?.url?.includes('/auth/me')) {
  localStorage.removeItem('charon_auth_token');
  window.location.href = '/login';
}
```

**Note**: This causes a full page reload and loses SPA state.

---

## Files to Modify

| File | Change |
|------|--------|
| `frontend/src/api/client.ts` | Add 401 handler with auth reset |
| `frontend/src/context/AuthContext.tsx` | Register auth error callback |

## Implementation Checklist

- [ ] Update `api/client.ts` with enhanced 401 interceptor
- [ ] Update `AuthContext.tsx` to register the callback
- [ ] Add unit tests for auth error handling
- [ ] Verify E2E test `should redirect to login when session expires` passes

---

## Priority

**Medium** - Security improvement but not critical since API calls still require valid auth.

## Labels

- frontend
- security
- auth
- enhancement

## Related

- Fixes E2E test: `should redirect to login when session expires`
- Part of Phase 1 E2E testing backlog
