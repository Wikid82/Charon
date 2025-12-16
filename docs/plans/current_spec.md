# Security Dashboard Live Log Viewer Bug Fix Plan

**Date:** December 16, 2025
**Issue:** Live Log Viewer shows "Disconnected" error with rapid connect/disconnect flashing

---

## Executive Summary

**ROOT CAUSE IDENTIFIED:** The WebSocket authentication token retrieval uses the wrong localStorage key.

The auth token is stored under `charon_auth_token` but the WebSocket code reads from `token` (which doesn't exist), causing **every WebSocket connection to be sent without authentication**, resulting in immediate 401 rejection.

---

## 1. Root Cause Analysis

### Token Storage Key Mismatch

| Component | localStorage Key Used | Correct? |
|-----------|----------------------|----------|
| AuthContext.tsx (login) | `charon_auth_token` | ✅ Source of truth |
| AuthContext.tsx (logout) | `charon_auth_token` | ✅ |
| client.ts (axios) | Gets token from AuthContext | ✅ |
| **logs.ts (WebSocket)** | **`token`** | ❌ **WRONG** |

### Code Evidence

**AuthContext.tsx (correct storage):**
```tsx
// Line 31 - stores token with correct key
localStorage.setItem('charon_auth_token', token);
```

**logs.ts (incorrect retrieval):**
```typescript
// Line 132 & 200 - reads wrong key
const token = localStorage.getItem('token'); // Returns null!
```

### Result

1. User logs in → token stored as `charon_auth_token`
2. Security Dashboard opens → WebSocket tries `localStorage.getItem('token')` → returns `null`
3. WebSocket URL: `/api/v1/cerberus/logs/ws` (no token query param)
4. Backend auth middleware rejects with 401 Unauthorized
5. Frontend receives `onError` event → shows "Disconnected" message
6. User sees rapid flashing if any auto-reconnect logic exists

---

## 2. Files to Modify

| File | Change Type | Description |
|------|-------------|-------------|
| `frontend/src/api/logs.ts` | **BUG FIX** | Change localStorage key from `token` to `charon_auth_token` |

**That's it!** This is a one-line fix in two locations within the same file.

---

## 3. Specific Code Changes

### File: `frontend/src/api/logs.ts`

#### Change #1: `connectLiveLogs` function (Line 132)

**Before:**
```typescript
// Get auth token from localStorage
const token = localStorage.getItem('token');
```

**After:**
```typescript
// Get auth token from localStorage (must match AuthContext key)
const token = localStorage.getItem('charon_auth_token');
```

#### Change #2: `connectSecurityLogs` function (Line 200)

**Before:**
```typescript
// Get auth token from localStorage
const token = localStorage.getItem('token');
```

**After:**
```typescript
// Get auth token from localStorage (must match AuthContext key)
const token = localStorage.getItem('charon_auth_token');
```

---

## 4. Phase 1: Backend Fix

**No backend changes required.**

The backend auth middleware already correctly handles token extraction from query parameters:

```go
// backend/internal/api/middleware/auth.go - Lines 21-24
// Try query param (token passthrough)
if token := c.Query("token"); token != "" {
    authHeader = "Bearer " + token
}
```

The middleware checks in this order:
1. `Authorization` header
2. `auth_token` cookie
3. `token` query parameter ← WebSocket uses this

The backend is working correctly. The issue is purely frontend.

---

## 5. Phase 2: Frontend Fix

### Implementation Steps

1. **Edit `frontend/src/api/logs.ts`:**
   - Line 132: Change `localStorage.getItem('token')` to `localStorage.getItem('charon_auth_token')`
   - Line 200: Change `localStorage.getItem('token')` to `localStorage.getItem('charon_auth_token')`

2. **Optional Enhancement - Add debug logging for auth issues:**
   ```typescript
   const token = localStorage.getItem('charon_auth_token');
   if (!token) {
     console.warn('WebSocket: No auth token found in localStorage');
   }
   ```

3. **Consider creating a shared constant for the token key:**
   ```typescript
   // In a shared constants file
   export const AUTH_TOKEN_KEY = 'charon_auth_token';
   ```

---

## 6. Testing Checklist

### Manual Testing Steps

1. **Clear browser data** (ensure clean state)
2. **Login to Charon** (stores token correctly)
3. **Navigate to Security Dashboard**
4. **Verify Live Log Viewer shows "Connected"** (green badge)
5. **Verify logs start streaming** (wait for traffic or trigger test request)
6. **Check browser DevTools:**
   - Network tab: WebSocket connection should be `101 Switching Protocols`
   - Console: Should show `Cerberus logs WebSocket connection established`
   - WebSocket frames: Should show JSON log entries, not error responses

### Verification Commands

```bash
# Test WebSocket endpoint with token (from container)
TOKEN=$(cat /tmp/test_token)  # or get from browser localStorage
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  "http://localhost:8080/api/v1/cerberus/logs/ws?token=$TOKEN"

# Should see: HTTP/1.1 101 Switching Protocols
```

### Unit Test Update

Update `frontend/src/components/__tests__/LiveLogViewer.test.tsx` if needed to mock the correct localStorage key:

```typescript
beforeEach(() => {
  // Mock localStorage with correct key
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation((key) => {
    if (key === 'charon_auth_token') return 'mock-jwt-token';
    return null;
  });
});
```

---

## 7. Regression Prevention

### Add ESLint Rule (Optional)

Consider adding a custom ESLint rule or code comment to prevent future mismatches:

```typescript
// frontend/src/constants/auth.ts
/**
 * IMPORTANT: This key must match across all auth-related code.
 * Used by: AuthContext.tsx, logs.ts (WebSocket)
 * @see AuthContext.tsx for login/logout logic
 */
export const AUTH_TOKEN_STORAGE_KEY = 'charon_auth_token';
```

### Update Contributing Guidelines

Add to `.github/copilot-instructions.md` or `CONTRIBUTING.md`:

> **Auth Token Key:** Always use `charon_auth_token` for localStorage operations. Import from shared constants when possible.

---

## 8. Summary

| Aspect | Details |
|--------|---------|
| **Bug** | WebSocket reads wrong localStorage key |
| **Impact** | Security Dashboard Live Logs never connects |
| **Fix** | Change `token` → `charon_auth_token` in 2 locations |
| **Risk** | Low - isolated change, no side effects |
| **Effort** | 5 minutes |
| **Testing** | Manual verification + existing unit tests |

---

## 9. Implementation Command

```bash
# Single sed command to fix both occurrences
sed -i "s/localStorage.getItem('token')/localStorage.getItem('charon_auth_token')/g" \
  frontend/src/api/logs.ts
```

Or apply the fix manually in VS Code.
