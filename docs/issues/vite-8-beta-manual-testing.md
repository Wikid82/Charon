# Manual Testing: Vite 8.0.0-beta.18 Upgrade

**Date:** 2026-03-12
**Status:** Open
**Priority:** Medium
**Related Commit:** chore(frontend): upgrade to Vite 8 beta with Rolldown bundler

---

## Context

Vite 8 replaces Rollup with Rolldown (Rust-based bundler) and esbuild with Oxc for
JS transforms/minification. Lightning CSS replaces esbuild for CSS minification. These
are fundamental changes to the build pipeline that automated tests may not fully cover.

## Manual Test Cases

### 1. Production Build Output Verification

- [ ] Deploy the Docker image to a staging environment
- [ ] Verify the application loads without console errors
- [ ] Verify all CSS renders correctly (Lightning CSS minification change)
- [ ] Check browser DevTools Network tab — confirm single JS bundle loads
- [ ] Verify sourcemaps work correctly in browser DevTools

### 2. CJS Interop Regression Check

Vite 8 changes how CommonJS default exports are handled.

- [ ] Verify axios API calls succeed (login, proxy host CRUD, settings)
- [ ] Verify react-hot-toast notifications render on success/error actions
- [ ] Verify react-hook-form validation works on all forms
- [ ] Verify @tanstack/react-query data fetching and caching works

### 3. Dynamic Import / Code Splitting

The `codeSplitting: false` config replaces the old `inlineDynamicImports: true`.

- [ ] Verify lazy-loaded routes load correctly
- [ ] Verify no "chunk load failed" errors during navigation
- [ ] Check that the React initialization issue (original reason for the workaround) does not resurface

### 4. Development Server

- [ ] Run `npm run dev` in frontend — verify HMR (Hot Module Replacement) works
- [ ] Make a CSS change — verify it hot-reloads without full page refresh
- [ ] Make a React component change — verify it hot-reloads preserving state
- [ ] Verify the dev server proxy to backend API still works

### 5. Cross-Browser Verification

Test in each browser to catch any Rolldown/Oxc output differences:

- [ ] Chrome/Chromium — full functional test
- [ ] Firefox — full functional test
- [ ] Safari/WebKit — full functional test

### 6. Docker Build Verification

- [ ] Build Docker image on the target deployment architecture
- [ ] Verify the image starts and passes health checks
- [ ] Verify Rolldown native bindings resolve correctly (no missing .node errors)
- [ ] Test with `--platform=linux/amd64` explicitly

### 7. Edge Cases

- [ ] Test with browser cache cleared (ensure no stale Vite 7 chunks cached)
- [ ] Test login flow end-to-end
- [ ] Test certificate management flows
- [ ] Test DNS provider configuration
- [ ] Test access list creation and assignment

## Known Issues to Monitor

1. **Oxc Minifier assumptions** — if runtime errors occur after build but not in dev, the minifier is the likely cause. Disable with `build.minify: false` to diagnose.
2. **Lightning CSS bundle size** — may differ slightly from esbuild. Compare `dist/assets/` sizes.
3. **Beta software stability** — track Vite 8 releases for fixes to any issues found.

## Pass Criteria

All checkboxes above must be verified. Any failure should be filed as a separate issue with the `vite-8-beta` label.
