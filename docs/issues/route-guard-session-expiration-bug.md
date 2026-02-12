# Route Guard Bug: Session Expiration Not Redirecting to Login

## Issue

After clearing authentication data (cookies + localStorage) and reloading the page, the application still loads the dashboard instead of redirecting to `/login`.

## Evidence

- Test: `tests/core/authentication.spec.ts:322` - "should redirect to login when session expires"
- Error: "Expected redirect to login or session expired message. Dashboard loaded instead, indicating missing auth validation."
- Video: `test-results/core-authentication-Authen-e89dd--login-when-session-expires-firefox/video.webm`
- Screenshot: `test-results/core-authentication-Authen-e89dd--login-when-session-expires-firefox/test-failed-1.png`

## Steps to Reproduce

1. Login to application
2. Clear all cookies: `await page.context().clearCookies()`
3. Clear localStorage: `localStorage.removeItem('token'); localStorage.removeItem('authToken'); localStorage.removeItem('charon_auth_token'); sessionStorage.clear()`
4. Reload page: `await page.reload()`
5. **Expected**: Redirect to `/login`
6. **Actual**: Dashboard loads, full access granted

## Root Cause Analysis

The route guard fix in `frontend/src/components/RequireAuth.tsx` and `frontend/src/context/AuthContext.tsx` may not handle the page reload scenario properly. Possible causes:

- `RequireAuth` not re-evaluating auth state after reload
- `AuthContext.checkAuth()` restoring session from HttpOnly cookie despite no localStorage token
- Router cache or React state persisting auth status

## Impact

**CRITICAL SECURITY ISSUE**: Users can access protected routes after clearing their session.

## Assigned To

Frontend Dev

## Files to Investigate

- `frontend/src/components/RequireAuth.tsx`
- `frontend/src/context/AuthContext.tsx`
- `frontend/src/routes.tsx` (router configuration)

## Acceptance Criteria

- [ ] Test `tests/core/authentication.spec.ts:322` passes
- [ ] Manual verification: After logout + clear storage + reload, user redirected to /login
- [ ] All protected routes blocked when auth data cleared
