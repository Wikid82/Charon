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

## Root Cause
The frontend auth context/provider does not check for valid authentication tokens on initial page load. Auth validation only occurs when API calls return 401 errors.

## Proposed Solution

### Option 1: Auth Guard in Route Protection
Add an auth check in the route protection layer that runs on initial load:

```typescript
// In AuthProvider or route guard
useEffect(() => {
  const token = localStorage.getItem('auth_token');
  if (!token) {
    navigate('/login');
    return;
  }

  // Optionally validate token with backend
  validateToken(token).catch(() => {
    clearAuth();
    navigate('/login');
  });
}, []);
```

### Option 2: API Client Interceptor
Add a 401 handler in the API client that redirects to login:

```typescript
// In API client setup
client.interceptors.response.use(
  response => response,
  error => {
    if (error.response?.status === 401) {
      clearAuth();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
```

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
