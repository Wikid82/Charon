---
goal: Issue #579 — [Frontend] Add Auth Guard on Page Reload (root-cause verification + regression hardening)
version: 1.0
date_created: 2026-06-09
status: 'Approved'
tags: [fix, frontend, auth, security]
---

# Issue #579 — Auth Guard on Page Reload

Branch: `fix/authprovider` (no worktrees, per CLAUDE.md)
Scope: Frontend only. No backend/model changes (GORM scan not required).

## Root Cause Analysis (verified 2026-06-09)

Traced the full flow per the Context First rule:

1. **Entry point**: page reload on a protected route (`/`).
2. **Transformation**: `frontend/src/context/AuthContext.tsx` `checkAuth()` runs on
   mount. When `localStorage` has no `charon_auth_token` it clears auth state and
   returns early (commit `901e824f`); when a token exists it validates it via
   `fetchSessionUser()` (`GET /api/v1/auth/me`) and clears state on failure.
3. **Persistence**: token lives only in `localStorage` (`charon_auth_token`);
   the axios Authorization header is mirrored via `setAuthToken()` in
   `frontend/src/api/client.ts`.
4. **Exit point**: `frontend/src/components/RequireAuth.tsx` redirects to `/login`
   when `isLoading` is false and either context state or the localStorage token is
   missing. A global 401 interceptor in `client.ts` invokes the handler registered
   by `AuthContext` via `setAuthErrorHandler()` (excluding `/auth/*` endpoints).

**Finding**: the guard described in issue #579 is already implemented on
`development` (route-guard fix merged 2026-01-30 plus commits `901e824f`,
`6777f6e8`). The acceptance test
`tests/core/authentication.spec.ts › should redirect to login when session expires`
**passes** against a container rebuilt from current source, as do all 16 tests in
`tests/core/authentication.spec.ts` (firefox). The issue is stale with respect to
the redirect behavior itself.

**Remaining defects/gaps (the actual work):**

1. **Stale auth-error handler (correctness gap)**: `setAuthErrorHandler` in
   `frontend/src/api/client.ts` only accepts `() => void`, and the registration
   `useEffect` in `AuthContext` has no cleanup, so after `AuthProvider` unmounts
   the axios interceptor keeps invoking a closure over dead state (calls
   `setUser` on an unmounted component; relevant for remounts, tests, HMR).
   Long-term fix: accept `(() => void) | null` and unregister on unmount.
2. **Zero unit-test regression coverage** for the guard behavior the issue is
   about: there are no Vitest tests for `AuthContext` (reload paths) or
   `RequireAuth` (redirect decision). The issue's own checklist requires
   "Add unit tests for auth error handling". Without them the behavior can
   silently regress again, which is how this issue arose.

## Phases

### Phase 1 — Harden auth-error handler lifecycle

- `frontend/src/api/client.ts`: change `setAuthErrorHandler(handler: () => void)`
  to `setAuthErrorHandler(handler: (() => void) | null)` (doc comment updated).
- `frontend/src/context/AuthContext.tsx`: in the registration `useEffect`, add
  cleanup `return () => setAuthErrorHandler(null);`.
- No behavioral change for the running app; eliminates the stale-closure hazard.

### Phase 2 — Regression unit tests (Vitest, jsdom)

- **New** `frontend/src/components/__tests__/RequireAuth.test.tsx`
  (render via `MemoryRouter` with `AuthContext.Provider`):
  - shows `LoadingOverlay` while `isLoading` is true (no premature redirect);
  - redirects to `/login` when `isAuthenticated` is false;
  - redirects to `/login` when context says authenticated but
    `localStorage.charon_auth_token` is absent (defense in depth);
  - renders children when authenticated and token present.
- **New** `frontend/src/context/__tests__/AuthContext.test.tsx`
  (mock `fetch` and the axios client module):
  - reload with no stored token → no `/auth/me` call, `user=null`,
    `isLoading=false`, `isAuthenticated=false`;
  - reload with stored token + `/auth/me` 401 → auth token cleared,
    `user=null`, `isAuthenticated=false`;
  - reload with stored token + `/auth/me` 200 → user populated;
  - registered auth-error handler clears `charon_auth_token` from localStorage
    and resets user state; handler is unregistered (set to null) on unmount.
- **Update** `frontend/src/api/__tests__/client.test.ts`: add case asserting that
  `setAuthErrorHandler(null)` unregisters (401 no longer invokes old handler).

### Phase 3 — Validation (Definition of Done, frontend-only)

1. E2E: `npx playwright test --project=firefox tests/core/authentication.spec.ts`
   against rebuilt `charon-e2e` container — all pass.
2. Unit tests + coverage: `scripts/frontend-test-coverage.sh` (≥85%).
3. `cd frontend && npm run type-check`.
4. `cd frontend && npm run build`.
5. `lefthook run pre-commit`.

## Commit Slicing Strategy (single PR `fix/authprovider` → `development`)

| # | Commit | Scope | Files | Gate |
|---|--------|-------|-------|------|
| 1 | `fix(auth): unregister auth error handler on AuthProvider unmount and add reload guard regression tests` | Phase 1 + Phase 2 | `client.ts`, `AuthContext.tsx`, new/updated test files, this plan | full DoD above |

One logical commit suffices: the production change is small and the new tests
validate it; splitting tests from the signature change would leave the first
commit without its validation gate.

## Ignore-file review

`.gitignore`, `.dockerignore`, `codecov.yml`: no new top-level paths introduced
(tests live in existing `__tests__` directories) — no changes required.
`Dockerfile`: unaffected.
