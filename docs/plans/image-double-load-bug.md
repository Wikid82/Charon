# Image Double-Load Bug: Root Cause & Fix Plan

**Branch**: `feature/image-double-load-fix` (target: `main`)
**Date**: 2026-06-05
**Status**: Ready for implementation
**Reporter**: observed at `https://charon.hatfieldhosted.com`

---

## 1. Introduction

### Overview

Users of the Charon management UI observe the following Firefox Network tab artifacts on every
page navigation:

| Observed request | Status |
|---|---|
| `data:application/octet-stream;base64,` | Completed (0 bytes) |
| `/banner.png` | `NS_BINDING_ABORTED` |
| `/logo.png` | `NS_BINDING_ABORTED` |
| `/banner.png` (retry) | `200 OK` — ~959 ms |
| `/logo.png` (retry) | `200 OK` — ~408 ms |

The net result is every inter-page navigation causes two aborted requests and two slow retries,
adding ~1 second of visible latency on the first navigation after load and wasting bandwidth on
every subsequent navigation until the browser cache is warm.

### Objectives

1. Eliminate `NS_BINDING_ABORTED` on `banner.png` and `logo.png` by keeping `<Layout>` stable
   during lazy-page loading.
2. Eliminate the unnecessary hidden image request from the CSS-invisible mobile header on desktop.
3. Establish the most likely explanation for the `data:application/octet-stream;base64,` artifact
   so it can be validated and, if reproducible, eliminated.

---

## 2. Research Findings

### 2.1 Where Banner and Logo Are Loaded From

The backend serves them as static assets. In `backend/internal/server/server.go`:

```go
router.StaticFile("/banner.png", frontendDir+"/banner.png")
router.StaticFile("/logo.png",   frontendDir+"/logo.png")
```

The source files live in `frontend/public/` (`banner.png`, `banner.webp`, `banner.svg`, `logo.png`,
`logo.webp`, `logo.svg`, `favicon.png`) and are copied verbatim into the production `dist/`
directory by Vite at build time.

There is **no API endpoint** that returns image data, base64 blobs, or any dynamic branding
configuration. This was verified by exhaustive search of:

- `frontend/src/api/settings.ts` — only caddy/SSL/keepalive keys
- `frontend/src/api/featureFlags.ts` — `Record<string, boolean>`
- `frontend/src/pages/SystemSettings.tsx`, `Settings.tsx` — no branding tab
- All `.tsx` / `.ts` source files — no `FileReader`, `readAsDataURL`, `URL.createObjectURL`,
  `data:image`, or dynamic img src binding of any kind

### 2.2 Where the Image `src` Comes From

Every `<img>` element in the frontend has a **hardcoded static path**, never a dynamically
computed value:

| File | Line | `src` value | Notes |
|---|---|---|---|
| `frontend/src/components/Layout.tsx` | 138 | `/logo.png` | Mobile header — always in DOM |
| `frontend/src/components/Layout.tsx` | 155 | `/logo.png` | Sidebar — rendered when `isCollapsed = true` |
| `frontend/src/components/Layout.tsx` | 157 | `/banner.png` | Sidebar — rendered when `isCollapsed = false` |
| `frontend/src/pages/Login.tsx` | 78 | `/logo.png` | Static |
| `frontend/src/pages/Setup.tsx` | 98 | `/logo.png` | Static |
| `frontend/src/pages/AcceptInvite.tsx` | 143 | `/logo.png` | Static |

### 2.3 Initial Value of `src` Before Data Loads

`isCollapsed` is initialized via a **synchronous lazy initializer** in Layout.tsx:

```tsx
const [isCollapsed, setIsCollapsed] = useState(() => {
  const saved = localStorage.getItem('sidebarCollapsed')
  return saved ? JSON.parse(saved) : false
})
```

`localStorage.getItem` is synchronous. `sidebarCollapsed` is either:
- `null` → defaults to `false` (banner.png rendered in sidebar)
- `"true"` → `isCollapsed = true` (logo.png rendered in sidebar)
- `"false"` → `isCollapsed = false` (banner.png rendered in sidebar)

There is **no render phase** where the image `src` is empty or `undefined` due to this state.
React renders the correct `src` on the very first paint.

### 2.4 Why the `data:application/octet-stream;base64,` Request Appears

**No source in the codebase explicitly produces this URI.** The most defensible explanation,
based on the browser artifacts and the architecture:

> During React 18's concurrent mode Suspense transitions, when `<Layout>` unmounts (see §2.5),
> the `<img>` elements are removed from the DOM mid-flight. Firefox's Network DevTools records
> the aborted resource fetch as `data:application/octet-stream;base64,` — the browser's internal
> representation of an aborted/empty fetch against a data-URI placeholder that React's concurrent
> renderer briefly creates during the unmount→remount transition.

A secondary possibility is that this is a **browser-extension artifact** (e.g., an ad-blocker or
privacy tool intercepts and rewrites the request). This can be verified by reproducing in a clean
Firefox profile.

---

## 3. Root Cause Analysis

### RC-1 (Primary): Suspense Boundary Unmounts `<Layout>` on Navigation

**File**: `frontend/src/App.tsx`

```tsx
// ❌ Current: Suspense wraps ALL routes, including Layout
<Suspense fallback={<LoadingOverlay message="Loading application..." />}>
  <Routes>
    <Route path="/" element={
      <SetupGuard>
        <RequireAuth>
          <Layout>
            <Outlet />   {/* ← lazy page component renders here */}
          </Layout>
        </RequireAuth>
      </SetupGuard>
    }>
      <Route index element={<Dashboard />} />   {/* lazy loaded */}
      <Route path="proxy-hosts" element={<ProxyHosts />} /> {/* lazy loaded */}
      ...
```

**Sequence of events that causes the bug:**

```
1. User lands on "/" — Layout mounts, images start fetching (may take 400–900 ms)
2. User clicks "Proxy Hosts" before images finish loading
3. ProxyHosts (lazy) suspends while its JS chunk downloads
4. Suspense activates → replaces entire Routes subtree with <LoadingOverlay>
5. Layout UNMOUNTS → browser cancels in-flight /banner.png and /logo.png → NS_BINDING_ABORTED
6. ProxyHosts chunk arrives → Suspense deactivates → Layout REMOUNTS
7. Layout re-requests /banner.png and /logo.png → they load successfully (cache miss: 400–900 ms)
```

This is why the retry has high latency: the images were not yet cached when they were aborted.
After the retry completes (and the browser caches them), subsequent navigations are fast.

### RC-2 (Secondary): Mobile Header Always Emits a Hidden Image Request

**File**: `frontend/src/components/Layout.tsx`, line 138

```tsx
{/* ❌ Always in the DOM even on desktop — CSS hides it but browser still fetches it */}
<div className="lg:hidden fixed top-0 left-0 right-0 h-16 ...">
  <img src="/logo.png" alt="Charon" className="h-10 w-auto" />
</div>
```

`lg:hidden` applies `display: none` on `lg` (≥1024 px) breakpoints, but does NOT prevent the
browser from requesting the image resource. On desktop screens:

- Mobile header img → requests `/logo.png`
- Sidebar img (collapsed) → ALSO requests `/logo.png`
- **Result**: two simultaneous identical requests to `/logo.png` on desktop with collapsed sidebar

When `isCollapsed = false`, the sidebar requests `/banner.png`, and the mobile header requests
`/logo.png` — two different images for two elements where only one is ever visible at a time.

---

## 4. Technical Specifications

### 4.1 Fix 1 — Move `<Suspense>` Inside `<Layout>` (Primary Fix)

**Strategy**: `<Layout>` must remain mounted while lazy page chunks are loading. The Suspense
boundary should only wrap the content area (`<Outlet>`), not the shell.

**Before** (`App.tsx`):
```tsx
<Suspense fallback={<LoadingOverlay message="Loading application..." />}>
  <Routes>
    <Route path="/" element={
      <SetupGuard>
        <RequireAuth>
          <Layout>
            <Outlet />
          </Layout>
        </RequireAuth>
      </SetupGuard>
    }>
```

**After** (`App.tsx`):
```tsx
{/* No Suspense wrapper at Routes level — only wraps unauthenticated lazy pages */}
<Routes>
  <Route path="/login" element={
    <Suspense fallback={<LoadingOverlay message="Loading..." />}>
      <Login />
    </Suspense>
  } />
  <Route path="/setup" element={
    <Suspense fallback={<LoadingOverlay message="Loading..." />}>
      <Setup />
    </Suspense>
  } />
  <Route path="/accept-invite" element={
    <Suspense fallback={<LoadingOverlay message="Loading..." />}>
      <AcceptInvite />
    </Suspense>
  } />
  ...
  <Route path="/" element={
    <SetupGuard>
      <RequireAuth>
        <Layout>
          <Outlet />   {/* Layout.tsx wraps Outlet in Suspense — see Fix 2 */}
        </Layout>
      </RequireAuth>
    </SetupGuard>
  }>
    <Route index element={<Dashboard />} />
    ...
  </Route>
</Routes>
```

**And** inside `Layout.tsx`, wrap children rendering:
```tsx
{/* ✅ Suspense only wraps the content area; sidebar/header stay mounted */}
<Suspense fallback={<PageLoadingFallback />}>
  {children}
</Suspense>
```

Where `PageLoadingFallback` is a lightweight skeleton/spinner rendered inside the existing content
area, not replacing the entire Layout shell.

### 4.2 Fix 2 — Eliminate Hidden Mobile Header Image on Desktop

**Strategy**: Use React state driven by `window.matchMedia` to avoid rendering the mobile header
image on desktop screens, eliminating the invisible network request.

Two acceptable implementation options:

**Option A — `useMediaQuery` hook (preferred for testability)**:
```tsx
// frontend/src/hooks/useMediaQuery.ts (new file)
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches)
  useEffect(() => {
    const mq = window.matchMedia(query)
    const handler = (e: MediaQueryListEvent) => setMatches(e.matches)
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [query])
  return matches
}

// In Layout.tsx
const isMobile = useMediaQuery('(max-width: 1023px)')   // < lg breakpoint

{/* Only renders (and fetches) the image when actually on mobile */}
{isMobile && (
  <div className="fixed top-0 left-0 right-0 h-16 ...">
    <img src="/logo.png" alt="Charon" className="h-10 w-auto" />
  </div>
)}
```

**Option B — `loading="lazy"` + `decoding="async"` (simpler, CSS-safe)**:
```tsx
<img
  src="/logo.png"
  alt="Charon"
  className="h-10 w-auto"
  loading="lazy"
  decoding="async"
/>
```

`loading="lazy"` defers fetch until the element is near the viewport. Because `display:none`
elements have zero layout dimensions, browsers should not eagerly fetch them. This approach is
simpler but relies on browser-specific behavior and may not work consistently across all browsers.

**Recommended**: Option A (explicit conditional render) for predictability. Option B as a
progressive enhancement if A is deferred.

### 4.3 Fix 3 — Add `fetchPriority` and `decoding` to Visible Images

The sidebar image IS always visible and should be prioritized by the browser:

```tsx
{isCollapsed ? (
  <img src="/logo.png" alt="Charon" className="h-12 w-auto"
       fetchPriority="high" decoding="async" />
) : (
  <img src="/banner.png" alt="Charon" className="h-14 w-auto max-w-[200px] object-contain"
       fetchPriority="high" decoding="async" />
)}
```

This is a non-breaking progressive enhancement and does not change the fix strategy.

### 4.4 Data Flow After Fix

```
Page load (authenticated user):
  SetupGuard (isLoading) → LoadingDiv
  SetupGuard (done)      → RequireAuth
  RequireAuth (isLoading)→ LoadingOverlay
  RequireAuth (done)     → Layout [MOUNTS, images fetch /logo.png + /banner.png]
                           └─ Suspense (page content only)
                              └─ Dashboard [lazy, suspends]
                                 └─ LoadingFallback shown in content area only

Navigation to /proxy-hosts:
  Layout stays MOUNTED   → images NOT re-fetched, browser cache is warm
  Outlet's Suspense      → ProxyHosts [lazy, suspends briefly]
                           └─ PageLoadingFallback shown in content area only
  ProxyHosts loads       → content area shows ProxyHosts, Layout stable throughout
```

---

## 5. Implementation Plan

### Phase 1: Playwright Tests (Specification)

Write E2E tests that define the expected (fixed) behaviour BEFORE implementing the fixes.
These tests will fail initially and pass after implementation.

**File**: `tests/image-double-load.spec.ts`

```
Test 1: Images load exactly once on initial page load
  - Navigate to /
  - Intercept network requests to /logo.png and /banner.png
  - Assert each URL is requested exactly 1 time
  - Assert no NS_BINDING_ABORTED (requests complete with status 200)

Test 2: Images are NOT re-fetched on inter-page navigation
  - Navigate to /
  - Wait for images to load
  - Navigate to /proxy-hosts
  - Assert /banner.png and /logo.png are NOT re-requested
  - (Browser cache means 0 new requests, not new 200s)

Test 3: No data: URI image requests appear in Network log
  - Navigate to /
  - Assert no request with URL matching /^data:/ is made for image MIME types
```

### Phase 2: Backend (No Changes Required)

The backend correctly serves `/logo.png` and `/banner.png` as static files. No backend changes
are required for this fix.

### Phase 3: Frontend Implementation

#### Task 3.1 — Move Suspense boundary

| | Detail |
|---|---|
| **File** | `frontend/src/App.tsx` |
| **Change** | Remove outer `<Suspense>` from `<Routes>`. Add per-route `<Suspense>` for `/login`, `/setup`, `/accept-invite`, `/passthrough` (these are independently lazy-loaded and not inside Layout). Keep Layout's children wrapped in a lightweight internal Suspense (see Task 3.2). |
| **Risk** | Low — behavior-equivalent for unauthenticated routes; improves stability for authenticated routes |
| **Validation** | Playwright Test 1 and Test 2 pass |

#### Task 3.2 — Add `PageLoadingFallback` inside Layout

| | Detail |
|---|---|
| **File** | `frontend/src/components/Layout.tsx` |
| **Change** | Wrap `{children}` in `<Suspense fallback={<PageLoadingFallback />}>`. `PageLoadingFallback` is a small pulse/skeleton component styled to fit the content area. |
| **New file** | `frontend/src/components/PageLoadingFallback.tsx` (optional, can be inline) |
| **Risk** | Low — improves UX (skeleton instead of full-screen overlay) |
| **Validation** | No layout flicker on navigation |

#### Task 3.3 — Fix mobile header image (conditional render)

| | Detail |
|---|---|
| **File** | `frontend/src/components/Layout.tsx` |
| **Change** | Add `useMediaQuery('(max-width: 1023px)')` hook call. Wrap mobile header `<img>` in `{isMobile && ...}`. |
| **New file** | `frontend/src/hooks/useMediaQuery.ts` |
| **Risk** | Very low — mobile behavior is unchanged; desktop eliminates one redundant request |
| **Validation** | On desktop (≥1024px), network tab shows only 1 request for logo/banner; on mobile, shows the mobile header image as expected |

#### Task 3.4 — Add `fetchPriority` and `decoding` to sidebar images

| | Detail |
|---|---|
| **File** | `frontend/src/components/Layout.tsx` lines 155–157 |
| **Change** | Add `fetchPriority="high" decoding="async"` to both sidebar `<img>` elements |
| **Risk** | None — purely additive attributes |

#### Task 3.5 — Unit tests

| | Detail |
|---|---|
| **File** | `frontend/src/components/__tests__/Layout.test.tsx` (existing or new) |
| **Tests** | (a) Renders banner.png when `isCollapsed = false`; (b) Renders logo.png when `isCollapsed = true`; (c) Does NOT render mobile header image when `isMobile = false`; (d) Renders mobile header image when `isMobile = true` |

### Phase 4: Integration & Validation

- Run full Playwright suite (all browsers) against Docker E2E environment
- Verify no regressions in mobile viewport tests
- Verify sidebar collapse/expand still works correctly
- Run backend unit tests (no backend changes but verify nothing broken)
- Check coverage gate: ≥85% overall, patch coverage reviewed

### Phase 5: Documentation

- Update `CHANGELOG.md` with fix entry
- No user-facing documentation changes required (internal bug fix)

---

## 6. Commit Slicing Strategy

**Decision**: Single PR with 3 ordered logical commits. The fix is tightly scoped to two files
plus one new hook file. Splitting into multiple PRs would add overhead without review benefit.

| Commit | Scope | Files | Dependency | Validation Gate |
|---|---|---|---|---|
| **1** | `test(e2e): add image double-load regression tests` | `tests/image-double-load.spec.ts` | None | Tests fail (expected — TDD) |
| **2** | `fix(layout): move Suspense inside Layout to prevent image abort on navigation` | `frontend/src/App.tsx`, `frontend/src/components/Layout.tsx`, `frontend/src/components/PageLoadingFallback.tsx` (new) | Commit 1 | Playwright Tests 1 & 2 pass |
| **3** | `fix(layout): conditionally render mobile header image to avoid hidden request` | `frontend/src/hooks/useMediaQuery.ts` (new), `frontend/src/components/Layout.tsx`, `frontend/src/components/__tests__/Layout.test.tsx` | Commit 2 | Playwright Test 3 passes; unit tests pass; no regression in mobile viewport |

**Rollback note**: Each commit is independently revertable. Commit 2 (Suspense move) is the
critical fix; Commit 3 (mobile image) is an independent optimization.

---

## 7. Acceptance Criteria

### Definition of Done

- [ ] `NS_BINDING_ABORTED` no longer appears for `/banner.png` or `/logo.png` in Firefox Network
      tab during inter-page navigation (verified manually and via Playwright)
- [ ] Playwright regression tests in `tests/image-double-load.spec.ts` pass on Firefox, Chromium,
      and WebKit
- [ ] On desktop (≥1024px), exactly **one** image is requested on initial load (either `logo.png`
      OR `banner.png`, not both)
- [ ] On mobile (<1024px), the mobile header image and the sidebar image both load correctly
- [ ] Sidebar collapse/expand toggles between `logo.png` and `banner.png` without visible glitches
- [ ] Unit test coverage for the new `useMediaQuery` hook reaches 100%
- [ ] Overall frontend test coverage ≥85%; patch coverage reviewed with no uncovered feature paths
- [ ] GORM security scan passes (no backend changes — gate satisfied by no-op)
- [ ] No new accessibility violations introduced (skip link and img alt text preserved)

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `useMediaQuery` causes SSR hydration mismatch (if SSR is added later) | Low | Low | Hook uses `() => window.matchMedia(...).matches` lazy init — safe for CSR; document if SSR is added |
| Moving Suspense changes loading UX for unauthenticated pages | Low | Low | Each unauthenticated route gets its own Suspense with the same fallback as before |
| `PageLoadingFallback` causes content-area layout shift | Low | Medium | Skeleton should match the approximate height of the content area; test with Lighthouse CLS metric |
| `fetchPriority` attribute not supported in all browsers | Very Low | Negligible | Attribute is safely ignored by non-supporting browsers |
| `data:application/octet-stream;base64,` artifact not eliminated by fixes | Medium | Low | If it persists after Fix 1 & 2, it is a browser/extension artifact unrelated to app code; document as known external artifact |
