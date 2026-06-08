---
goal: Fix CVE-2026-45135 — Ensure CI detects Caddy v2.11.4 not stale v2.11.2 in all build workflows
version: 1.0
date_created: 2026-06-08
status: 'Planned'
tags: [fix, security, infrastructure]
---

# Fix CVE-2026-45135: Caddy FastCGI Unsafe Unicode Handling

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

---

## 1. Introduction

### Overview

Charon's frontend exhibits a classic Flash of Unstyled Content (FOUC) during initial page load. The browser console surfaces it as:

> *"Layout was forced before the page was fully loaded. If stylesheets are not yet loaded this may cause a flash of unstyled content."*

Users experience a visible dark-to-light (or light-to-dark) colour flicker on every hard reload or cold navigation. For light-mode users this is especially severe — the entire page flashes a dark slate background before switching to light colours. The page also experiences a blank white frame while i18n initialises.

### Objectives

1. Eliminate the theme class FOUC so `<html>` carries the correct `.dark` / `.light` class **before** the first browser paint.
2. Eliminate the blank-page delay caused by async i18n initialisation.
3. Remove the forced-layout warning triggered by React's post-paint `useEffect` applying theme classes while CSS is still resolving.
4. Establish Playwright regression tests that prevent future regressions.

---

## 2. Research Findings

### 2.1 File Inventory

| File | Lines | Role |
|---|---|---|
| `frontend/index.html` | 13 | HTML shell served by Vite / Go backend |
| `frontend/src/main.tsx` | 46 | React application entry point |
| `frontend/src/context/ThemeContext.tsx` | 27 | Dark/light mode state and DOM application |
| `frontend/src/context/LanguageContext.tsx` | 34 | Language state provider |
| `frontend/src/i18n.ts` | 38 | i18next initialisation with bundled JSON resources |
| `frontend/src/index.css` | 300 | Global stylesheet: Tailwind v4, CSS custom properties, light-mode overrides |
| `frontend/src/App.tsx` | ~180 | Root component; all 28 pages are React.lazy() |
| `frontend/tailwind.config.js` | ~80 | darkMode: 'class' |
| `frontend/vite.config.ts` | 43 | Vite with rolldown, vendor chunk splitting |

### 2.2 Critical Code Paths

**`frontend/index.html`** (full file):

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/png" href="/favicon.png" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Charon</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Observations:
- No inline `<script>` in `<head>` to pre-apply the theme class.
- No `<link rel="preload">` for CSS or fonts.
- CSS enters exclusively through the JS module graph (`main.tsx → import './index.css'`).

**`frontend/src/context/ThemeContext.tsx`** lines 11–16:

```tsx
useEffect(() => {
  const root = window.document.documentElement
  root.classList.remove('light', 'dark')
  root.classList.add(theme)
  localStorage.setItem('theme', theme)
}, [theme])
```

Observations:
- `useEffect` fires **after** the browser has committed and painted the first frame.
- The `<html>` element carries no `dark` or `light` class between navigation start and this effect executing — typically 50–300 ms on first load.
- `useState` lazy initialiser correctly reads `localStorage.getItem('theme')` (lines 6–9), but the DOM class is not applied until the effect runs.

**`frontend/src/index.css`** lines 216–228 (`:root` block):

```css
:root {
  /* … custom properties (dark mode as default) … */
  color-scheme: dark;
  color: rgba(255, 255, 255, 0.87);
  background-color: #0f172a;  /* slate-900 */
}
.light {
  /* All surface/text/border variables overridden for light mode */
  --color-bg-base: 248 250 252;  /* slate-50 */
  /* … */
}
```

Observations:
- `:root` hardcodes `background-color: #0f172a` (dark slate) as the document default.
- Light-mode overrides live under `.light {}`, requiring the `.light` class on `<html>`.
- Without the class, every user — regardless of preference — renders the dark background on first paint.

**`frontend/src/main.tsx`** lines 41–46:

```tsx
if (i18n.isInitialized) {
  renderApp()
} else {
  i18n.on('initialized', renderApp)
}
```

**`frontend/src/i18n.ts`** — `init()` options:

```ts
i18n.use(LanguageDetector).use(initReactI18next).init({
  resources,         // Five bundled JSON files (no network fetch)
  fallbackLng: 'en',
  debug: false,
  interpolation: { escapeValue: false },
  detection: {
    order: ['localStorage', 'navigator'],
    caches: ['localStorage'],
    lookupLocalStorage: 'charon-language',
  },
})
```

Observations:
- All five translation JSONs are statically imported — no network fetch required.
- i18next's default `initImmediate` is `true`, which defers `init()` resolution to the next microtask tick even when everything is synchronous.
- Result: `i18n.isInitialized` is `false` when `main.tsx` first evaluates, so `renderApp()` is always pushed to the next event loop tick.
- The DOM shows an empty `<div id="root">` with the dark `:root` background for one tick — visible as a brief blank flash.

**`frontend/tailwind.config.js`**:

```js
export default {
  darkMode: 'class',
  // …
}
```

Confirms: all Tailwind `dark:` utilities require the `.dark` class on `<html>`.

**`frontend/vite.config.ts`** — `rolldownOptions`:

```ts
manualChunks(id) {
  if (id.includes('node_modules/i18next') || id.includes('node_modules/react-i18next'))
    return 'vendor-i18n'
  // …
}
```

Observations:
- `vendor-i18n` is a separate chunk; its load must complete before `i18n.init()` can run.
- Combined with async `initImmediate`, this adds an additional render-blocking step.

### 2.3 Layout-Reading Code (Forced Layout Audit)

Grep for `offsetWidth`, `offsetHeight`, `getBoundingClientRect`, `scrollHeight`, `clientHeight`, `scrollTop` across `frontend/src/**`:

| File | Lines | Context |
|---|---|---|
| `frontend/src/components/hecate/TunnelLogViewer.tsx` | 69–70 | Scroll position check inside a scroll-event handler (post-mount) |
| `frontend/src/components/LiveLogViewer.tsx` | 262, 269–271 | Auto-scroll logic inside `useEffect` (post-mount) |

Neither reads layout properties during the initial render phase. Both execute inside already-mounted event handlers and are **not** the source of the forced-layout warning.

The actual forced-layout trigger is `classList.add(theme)` in `ThemeContext.tsx`'s `useEffect`, which forces a style recalculation while the browser may not have finished resolving the CSS cascade from the JS-imported stylesheet.

### 2.4 Font Loading Audit

`frontend/src/index.css` references:

```css
font-family: Inter, system-ui, Avenir, Helvetica, Arial, sans-serif;
```

There are **no** `@font-face` declarations, no `@fontsource` imports, and no Google Fonts `<link>` in `index.html`. `Inter` falls through to `system-ui` on most systems. No FOIT/FOUT from web font loading is present. Font loading is not a contributing factor.

### 2.5 External Dependency Summary

| Library | FOUC Relevance |
|---|---|
| `react` 19+ | CSR only — no SSR hydration concerns |
| `i18next` | Async init default (fixed by `initImmediate: false`) |
| `tailwindcss` v4 | `darkMode: 'class'` requires class on `<html>` |
| `@vitejs/plugin-react` | CSS extracted to separate file in prod build automatically |
| `recharts` / `d3-*` | Vendor-split; loaded lazily — no initial FOUC |

---

## 3. Root Cause Analysis

### RC-1 — ThemeContext `useEffect` Applies Theme After First Paint

**File:** `frontend/src/context/ThemeContext.tsx` lines 11–16
**Severity:** Critical
**Affected users:** All users

`useEffect` is scheduled **after** React commits the DOM and the browser paints. During the window between page-start and effect execution (typically 50–300 ms on a cold load):

- `<html>` has no `dark` or `light` class.
- All Tailwind `dark:` utilities produce no styles (class-based dark mode requires the class on `<html>`).
- `:root` background is `#0f172a` (dark slate), but component-level `dark:` variant styles are absent.
- Light-mode users see the dark `:root` background; dark-mode users see components without `dark:` utility styles.

### RC-2 — No Anti-FOUC Inline Script in `index.html`

**File:** `frontend/index.html`
**Severity:** Critical (root of RC-1)

The standard pattern for preventing theme FOUC is a small synchronous `<script>` in `<head>` that reads the persisted theme from `localStorage` and applies the class to `<html>` before any rendering occurs. Without it, there is no mechanism to apply the theme class before React loads and its first `useEffect` runs.

This script must be:
- Inline (not a module import — avoids network round-trip and async execution).
- Located in `<head>`, before `<body>` renders.
- Wrapped in a try/catch IIFE (`localStorage` can throw in restricted contexts).

### RC-3 — i18next Async Init Delays React Render

**File:** `frontend/src/main.tsx` lines 41–46; `frontend/src/i18n.ts`
**Severity:** Major
**Result:** Blank page for one event-loop tick before `renderApp()` is called

i18next's default `initImmediate: true` defers `init()` resolution to the next microtask, even when all resources are bundled synchronously. Because `i18n.isInitialized` is `false` when `main.tsx` first evaluates, `renderApp()` is always deferred. The DOM shows an empty `<div id="root">` with the `:root` styles for one tick — visible as a brief blank flash before the first React render.

Setting `initImmediate: false` makes `init()` synchronous when no async plugins are involved. `i18n.isInitialized` is then `true` immediately, and `renderApp()` is called inline without waiting for the event loop.

### RC-4 — `:root` Defaults to Dark; Light Mode Requires a Class

**File:** `frontend/src/index.css` lines 216–228
**Severity:** Moderate (amplifies RC-1 for light-mode users)

The `:root` block includes `background-color: #0f172a` and `color: rgba(255,255,255,0.87)`. Light-mode CSS custom property overrides live under `.light {}`. Without the `.light` class, the page is visually dark regardless of user preference. This creates a dark flash for light-mode users that persists until the theme class is applied.

The primary fix (RC-2 inline script) resolves this by applying the class before the first paint. However, the `.light {}` block must also explicitly override `background-color`, `color-scheme`, and `color` — without these overrides, light-mode users still see a dark flash on first paint because `:root` hardcodes dark values that `.light {}` does not currently cancel. See Fix 4.4.

### RC-5 — Vendor Chunk Splitting Delays i18n Chunk Load

**File:** `frontend/vite.config.ts` lines 25–27
**Severity:** Minor (amplifies RC-3)

`vendor-i18n` is a separate chunk. Its network fetch and parse must complete before `i18n.init()` can run. With RC-3 fixed (`initImmediate: false`), the chunk loading delay still exists — but init completes synchronously once the chunk is available, rather than deferring again to the next tick.

---

## 4. Technical Specifications

### 4.1 Fix for RC-1 + RC-2: Anti-FOUC Inline Script

**Target file:** `frontend/index.html`
**Mechanism:** Synchronous inline script in `<head>` that reads `localStorage` and applies the theme class to `<html>` before any rendering

```html
<head>
  <meta charset="UTF-8" />
  <link rel="icon" type="image/png" href="/favicon.png" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Charon</title>
  <script>(function(){try{var t=localStorage.getItem('theme');document.documentElement.classList.add(t==='light'?'light':'dark')}catch(_){}})()</script>
</head>
```

Constraints:
- Plain `<script>` (not `type="module"`) — module scripts are deferred and do not execute synchronously.
- Placed in `<head>` before `<body>` — ensures execution before any layout is calculated.
- IIFE + try/catch guards against `localStorage` throwing in restricted contexts (private browsing, cross-origin iframes).
- Defaults to `'dark'` when no preference is stored — matches `ThemeContext.tsx`'s default.
- Minified to a single line — this script is render-blocking, so minimising its byte size matters.

### 4.2 Fix for RC-3: Synchronous i18n Initialisation

**Target file:** `frontend/src/i18n.ts`
**Mechanism:** Add `initImmediate: false` to `i18n.init()` options

```ts
// Add initImmediate: false to the init options
i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    debug: false,
    initImmediate: false,   // Makes init synchronous for bundled resources
    interpolation: { escapeValue: false },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: 'charon-language',
    },
  })
```

With all five translation JSONs statically imported (no `HttpBackend`, no network fetch), i18next has no async work. `initImmediate: false` completes init synchronously, making `i18n.isInitialized` true immediately after the `.init()` call returns. No change to `main.tsx` is needed — the existing `if (i18n.isInitialized)` guard handles this correctly.

### 4.3 Fix for RC-4: ThemeContext `useLayoutEffect`

**Target file:** `frontend/src/context/ThemeContext.tsx`
**Mechanism:** Replace `useEffect` with `useLayoutEffect`

```tsx
// Replace useEffect with useLayoutEffect for synchronous class application
useLayoutEffect(() => {
  const root = window.document.documentElement
  root.classList.remove('light', 'dark')
  root.classList.add(theme)
  localStorage.setItem('theme', theme)
}, [theme])
```

`useLayoutEffect` fires synchronously after React commits DOM mutations but **before** the browser paints. After the inline script (Fix 4.1) handles the initial load, `useLayoutEffect` becomes the mechanism for theme-toggle transitions — ensuring they are seamless with no visible flash.

`useLayoutEffect` is safe here because Charon is CSR-only (no SSR). If SSR is ever introduced, a `typeof window !== 'undefined'` guard must be added.

The `useEffect` import may become unused if `ThemeContext.tsx` uses only `useLayoutEffect` and `useState`. Remove unused imports when making this change.

### 4.4 Fix for CSS-1: Light-Mode Property Overrides in `index.css`

**Target file:** `frontend/src/index.css`
**Mechanism:** Add explicit `background-color`, `color-scheme`, and `color` overrides to the `.light {}` block

The current `.light {}` block overrides CSS custom properties (e.g., `--color-bg-base`) but does not override the physical properties set on `:root` — `background-color: #0f172a`, `color-scheme: dark`, and `color: rgba(255, 255, 255, 0.87)`. Because these physical properties are not CSS variables, they are not overridden by variable redefinition.

**Change:**

```css
.light {
  background-color: #f0f4f8;   /* matches Layout.tsx root div; cancels :root dark default */
  color-scheme: light;
  color: rgba(0, 0, 0, 0.87);
  /* … existing variable overrides unchanged … */
}
```

`#f0f4f8` is used (not `#f8fafc`) to match the colour applied by `Layout.tsx`'s root `<div>`. This ensures that even in the instant between the inline script applying `.light` to `<html>` and React mounting, the page renders with a light background rather than the dark `:root` default.

---

### 4.5 Data Flow Diagram (After All Fixes)

```
Browser parses HTML
  ↓
<head> inline script executes synchronously (render-blocking, ~0.1ms)
  → reads localStorage.getItem('theme')
  → document.documentElement.classList.add('dark' | 'light')
  ↓
CSS parsed (Tailwind .dark: / .light: classes now resolve correctly)
  ↓
vendor-i18n chunk loads
  ↓
main.tsx module evaluates
  → i18n.init() called
  → initImmediate: false → completes synchronously
  → i18n.isInitialized === true
  → renderApp() called immediately (no next-tick deferral)
  ↓
React renders
  → ThemeContext reads localStorage — same value as the class already on <html>
  → useLayoutEffect fires synchronously (class already present → no-op)
  ↓
First browser paint: correct theme applied, no flash, no forced-layout warning
```

### 4.6 Edge Cases

| Scenario | Expected Behaviour |
|---|---|
| `localStorage` unavailable (private mode, restricted iframe) | try/catch in inline script falls back silently to dark (`:root` default). FOUC not eliminated but gracefully degraded. |
| User clears `localStorage` between visits | Inline script falls back to dark. Matches `ThemeContext` default. |
| First-ever visit (no `theme` key) | `localStorage.getItem('theme')` returns `null`; `null === 'light'` is `false`; `dark` applied. Correct. |
| Theme toggled mid-session | `useLayoutEffect` fires synchronously before browser paint — no visible transition flash. |
| `initImmediate: false` + future async i18n plugin | If `HttpBackend` is added, synchronous init completes before async ops. Missing translations fall back to `fallbackLng: 'en'` until async load finishes — acceptable. |
| User navigates with `?theme=light` query param | Not currently a feature; not in scope. |

---

## 5. Implementation Plan

### Phase 1 — Playwright Tests (Red State)

Write failing tests that document expected post-fix behaviour. Tests must fail against the current codebase; their passage confirms the fix is complete.

**New file:** `tests/core/theme-fouc.spec.ts`

Test cases:

| ID | Description | How Verified |
|---|---|---|
| T1 | Default (no stored preference): `<html>` carries `.dark` at `DOMContentLoaded`, before React runs | `addInitScript`: register a `DOMContentLoaded` listener that captures `document.documentElement.className` into `window.__domContentClasses`; assert it contains `dark` |
| T2 | Stored light preference: `<html>` carries `.light` at `DOMContentLoaded` | `addInitScript`: set `localStorage.theme='light'` and register `DOMContentLoaded` capture into `window.__domContentClasses`; assert it contains `light` |
| T3 | Stored dark preference: `<html>` carries `.dark` and not `.light` at `DOMContentLoaded` | `addInitScript`: set `localStorage.theme='dark'` and register `DOMContentLoaded` capture; assert `window.__domContentClasses` contains `dark` but not `light` |
| T4 | Light-mode background is not dark slate *(depends on CSS-1: Fix 4.4)* | Set light preference, goto, assert computed `background-color` of `<html>` is not `#0f172a`; requires `.light {}` `background-color` override to be present |
| T5 | No dual-class false positive: only `.light` on `<html>` when light theme stored | `addInitScript`: set `localStorage.theme='light'` and register `DOMContentLoaded` capture; assert `window.__domContentClasses` does not contain `dark` |
| T6 | Theme toggle applies `.light` synchronously | Login, navigate to settings, click toggle, evaluate classList before next animation frame |

Playwright approach for T1–T3 and T5 uses `page.addInitScript()` for two purposes: (1) setting `localStorage` to simulate a returning user's stored preference, and (2) registering a `DOMContentLoaded` listener that captures `document.documentElement.className` into `window.__domContentClasses`. Tests then assert on `window.__domContentClasses` rather than the final DOM state. This is critical: asserting only on final state would pass even if the inline script were removed from `index.html`, because React's `useLayoutEffect` would still apply the correct class before the assertion evaluates. Capturing at `DOMContentLoaded` — before React has executed — ensures the test detects that regression.

### Phase 2 — No Backend Changes

All fixes are frontend-only. Backend, database, and Go code are not affected.

### Phase 3 — Fix RC-1 + RC-2: `frontend/index.html`

Single-line change: add the minified anti-FOUC `<script>` to `<head>`.

- **Effort:** ~5 minutes
- **Risk:** Very low — defensive script, no breaking changes

### Phase 4 — Fix RC-3: `frontend/src/i18n.ts`

Single-line change: add `initImmediate: false` to `init()` options.

- **Effort:** ~5 minutes
- **Risk:** Low — safe with bundled resources; regression covered by T6 and existing auth tests

### Phase 5 — Fix CSS-1: `frontend/src/index.css`

Three-line addition inside the existing `.light {}` block: `background-color: #f0f4f8`, `color-scheme: light`, `color: rgba(0, 0, 0, 0.87)`.

- **Effort:** ~5 minutes
- **Risk:** Very low — additive CSS change with no structural impact

### Phase 6 — Fix RC-4: `frontend/src/context/ThemeContext.tsx`

Single-line change: `useEffect` → `useLayoutEffect`. Potentially remove unused `useEffect` import.

- **Effort:** ~5 minutes
- **Risk:** Low for CSR; document SSR caveat in comment or PR description

### Phase 7 — Validate

1. Run `tests/core/theme-fouc.spec.ts` — all T1–T6 must pass (Firefox).
2. Run full non-security Playwright suite — no regressions.
3. Manual browser check:
   - Set `localStorage.theme = 'light'`, hard-reload, confirm no dark flash.
   - Open Chrome DevTools Performance tab, record page load, confirm no "Layout was forced" warning.
4. `tsc --noEmit` — no new TypeScript errors.

---

## 6. Commit Slicing Strategy

**Decision:** Single PR with three ordered logical commits.

All changes are:
- Frontend-only (no cross-domain risk)
- Independently small (≤ 3 lines each)
- Closely related (all address the same user-visible bug)

A single PR keeps the context and test evidence together for reviewers.

---

### Commit 1 — `test(fouc): add regression tests for theme FOUC and blank page delay`

**Scope:** `tests/core/theme-fouc.spec.ts` (new file)
**Files:** `tests/core/theme-fouc.spec.ts`
**Dependencies:** None
**Validation gate:** Tests run; T1–T6 fail on current codebase (expected red state)

Tests document expected behaviour and serve as the acceptance signal for Commits 2 and 3.

---

### Commit 2 — `fix(ui): eliminate FOUC with anti-FOUC script, synchronous i18n init, and light-mode CSS overrides`

**Scope:**
- `frontend/index.html` — add inline `<script>` to `<head>`
- `frontend/src/i18n.ts` — add `initImmediate: false`
- `frontend/src/index.css` — add `background-color`, `color-scheme`, `color` overrides to `.light {}`

**Files:** `frontend/index.html`, `frontend/src/i18n.ts`, `frontend/src/index.css`
**Dependencies:** Commit 1
**Validation gate:** T1–T5 now pass; no blank-page delay observable in browser; light-mode background resolves to `#f0f4f8` not `#0f172a`

This commit resolves the two highest-severity root causes (RC-1, RC-2, RC-3) and applies the CSS-1 light-mode property fix. Theme class is applied synchronously before paint; React renders on the same tick as i18n init; light-mode users see the correct background immediately.

---

### Commit 3 — `fix(theme): use useLayoutEffect to apply theme class before browser paint`

**Scope:**
- `frontend/src/context/ThemeContext.tsx` — `useEffect` → `useLayoutEffect`

**Files:** `frontend/src/context/ThemeContext.tsx`
**Dependencies:** Commit 2
**Validation gate:** T6 passes; DevTools Performance tab clean; full Playwright suite green

Eliminates the forced-layout warning for theme-toggle transitions and makes the initial class application synchronous with React's commit phase.

---

### Rollback Notes

All three commits are independently safe to revert:
- Commit 1: delete `tests/core/theme-fouc.spec.ts`.
- Commit 2: remove the `<script>` from `index.html`; remove `initImmediate: false` from `i18n.ts`; remove the three property overrides from `.light {}` in `index.css`.
- Commit 3: revert `useLayoutEffect` to `useEffect`; restore `useEffect` import.

No migrations, API changes, or infrastructure changes are involved.

---

## 7. Acceptance Criteria (Definition of Done)

### Functional
- [ ] Hard-reloading with `localStorage.theme = 'dark'` shows no flash of light content.
- [ ] Hard-reloading with `localStorage.theme = 'light'` shows no dark background flash.
- [ ] Hard-reloading with no `theme` key defaults to dark mode with no flash.
- [ ] Theme toggle in Settings applies the new theme with no visible flicker.
- [ ] Browser DevTools Performance tab shows no "Layout was forced before the page was fully loaded" warning.

### Testing
- [ ] `tests/core/theme-fouc.spec.ts` — all T1–T6 pass in Firefox.
- [ ] T4 passes (depends on CSS-1: `.light {}` overrides `background-color`, `color-scheme`, and `color`).
- [ ] Full non-security Playwright suite is green.
- [ ] No new console errors or warnings from `localStorage`, `classList`, or `useLayoutEffect`.

### Code Quality
- [ ] Inline script in `index.html` is a single minified line.
- [ ] Unused `useEffect` import removed from `ThemeContext.tsx` if no longer needed.
- [ ] `tsc --noEmit` clean — no TypeScript errors introduced.

### Performance
- [ ] CSS and JS chunks load in the same order as before (waterfall unchanged).
- [ ] No observable FCP regression (no Lighthouse gate — no pre-fix baseline is defined).

---

## 8. Out of Scope

The following were identified during research and confirmed as non-contributing or separate concerns:

- **Font loading** — `Inter` and `JetBrains Mono` fall back to `system-ui`. No `@font-face` or external CDN. No FOIT/FOUT. Separate font-bundling work would be its own feature.
- **CSS preload hints** — Vite automatically injects `<link rel="stylesheet">` and `<link rel="modulepreload">` in production builds. The dev-server gap is acceptable.
- **Log viewer layout reads** — `TunnelLogViewer.tsx` and `LiveLogViewer.tsx` read scroll properties inside post-mount event handlers, not during initial render. These are benign.
- **Vendor chunk splitting** — The `vendor-i18n` chunk incurs a network round-trip on cold load. This is a performance concern separate from FOUC and belongs in a performance audit.
- **SSR / hydration** — Charon is CSR-only. No hydration concerns exist.
