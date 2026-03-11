# Major Dependency Upgrade Plan — ESLint v10, TypeScript 6.0, Vite 8

**Date:** 2026-03-11
**Author:** Planning Agent
**Status:** Ready for Review
**Confidence Score:** 88% (High for ESLint v10 + TS 6.0; Low for Vite 8 — unreleased)

---

## 1. Executive Summary

This plan covers the upgrade of three major frontend toolchain dependencies in the Charon project:

| Dependency | Current Version | Target Version | Status | Risk |
|---|---|---|---|---|
| **ESLint** | `^9.39.3 <10.0.0` | `^10.0.0` | Released | **Medium** — plugin compat gate |
| **TypeScript** | `^5.9.3` | `^6.0.0` | Beta (Feb 11) / RC (Mar 6) | **Medium** — 17+ deprecations |
| **Vite** | `^7.3.1` | `8.x` | **Does not exist** | **N/A** — monitor only |

### Key Findings

1. **ESLint v10** is released with a comprehensive migration guide. The primary blocker is a note in `lefthook.yml`: _"ESLint pinned at v9.x.x — do not upgrade until react-hooks plugin supports v10."_ The `eslint-plugin-react-hooks@7.0.1` must be verified for ESLint v10 compatibility before proceeding.

2. **TypeScript 6.0** is real (Beta: Feb 11, 2026; RC: Mar 6, 2026). It is explicitly designed as a **bridge release** between TS 5.9 and the native Go-based TS 7.0. It introduces 17+ deprecations/breaking changes (new defaults for `strict`, `module`, `target`, `types`, `rootDir`; removal of `outFile`, legacy module systems; deprecated `baseUrl`, `moduleResolution: node`). Charon's current `tsconfig.json` is well-positioned — it already uses `moduleResolution: bundler`, `strict: true`, and `module: ESNext`. The **critical impact** is the `types` default changing to `[]`.

3. **Vite 8 does not exist.** The latest Vite major is v7 (released June 2025). Charon is already on Vite 7.3.1. This section of the plan documents monitoring strategy and readiness posture only.

### Recommended Execution Order

```
PR-1: TypeScript 6.0 upgrade (fewer external dependencies, most self-contained)
PR-2: ESLint v10 upgrade (blocked on plugin compat verification)
PR-3: Vite 8 (deferred — version does not exist yet)
```

---

## 2. Current Dependency Inventory

### Root `package.json` (`/projects/Charon/package.json`)

| Package | Current Version | Category |
|---|---|---|
| `typescript` | `^5.9.3` | devDependency |
| `vite` | `^7.3.1` | devDependency |
| `@playwright/test` | `^1.58.2` | devDependency |
| `prettier` | `^3.8.1` | devDependency |
| `markdownlint-cli2` | `^0.21.0` | devDependency |

### Frontend `package.json` (`/projects/Charon/frontend/package.json`)

| Package | Current Version | Category |
|---|---|---|
| `typescript` | `^5.9.3` | devDependency |
| `vite` | `^7.3.1` | devDependency |
| `vitest` | `^4.0.18` | devDependency |
| `eslint` | `^9.39.3 <10.0.0` | devDependency |
| `@eslint/js` | `^9.39.3 <10.0.0` | devDependency |
| `@eslint/css` | `^1.0.0` | devDependency |
| `@eslint/json` | `^1.1.0` | devDependency |
| `@eslint/markdown` | `^7.5.1` | devDependency |
| `typescript-eslint` | `^8.57.0` | devDependency |
| `@typescript-eslint/eslint-plugin` | `^8.57.0` | devDependency |
| `@typescript-eslint/parser` | `^8.57.0` | devDependency |
| `@vitejs/plugin-react` | `^5.1.4` | devDependency |
| `@vitest/coverage-istanbul` | `^4.0.18` | devDependency |
| `@vitest/coverage-v8` | `^4.0.18` | devDependency |
| `@vitest/eslint-plugin` | `^1.6.10` | devDependency |
| `react` | `^19.2.4` | dependency |
| `react-dom` | `^19.2.4` | dependency |
| `react-router-dom` | `^7.13.1` | dependency |
| `@tanstack/react-query` | `^5.90.21` | dependency |

### ESLint Plugin Inventory (18 plugins)

| Plugin | Current Version | ESLint v10 Risk |
|---|---|---|
| `eslint-plugin-react-hooks` | `^7.0.1` | **HIGH** — explicit blocker in `lefthook.yml` |
| `eslint-plugin-react-compiler` | `^19.1.0-rc.2` | Medium — RC, check compat |
| `eslint-plugin-react-refresh` | `^0.5.2` | Low |
| `eslint-plugin-import-x` | `^4.16.1` | Low — modern fork |
| `eslint-plugin-jsx-a11y` | `^6.10.2` | Medium |
| `eslint-plugin-security` | `^4.0.0` | Low |
| `eslint-plugin-sonarjs` | `^4.0.2` | Low |
| `eslint-plugin-unicorn` | `^63.0.0` | Low — actively maintained |
| `eslint-plugin-promise` | `^7.2.1` | Low |
| `eslint-plugin-unused-imports` | `^4.4.1` | Low |
| `eslint-plugin-no-unsanitized` | `^4.1.5` | Medium |
| `eslint-plugin-testing-library` | `^7.16.0` | Low |
| `typescript-eslint` | `^8.57.0` | Low — tracks ESLint closely |
| `@vitest/eslint-plugin` | `^1.6.10` | Low |
| `@eslint/css` | `^1.0.0` | Low — official ESLint |
| `@eslint/json` | `^1.1.0` | Low — official ESLint |
| `@eslint/markdown` | `^7.5.1` | Low — official ESLint |

### Config Files Affected

| File | Impact Area |
|---|---|
| `frontend/tsconfig.json` | TS 6.0 — `types`, `lib`, defaults |
| `frontend/tsconfig.node.json` | TS 6.0 — minor |
| `frontend/tsconfig.build.json` | TS 6.0 — extends base |
| `frontend/eslint.config.js` | ESLint v10 — plugin compat |
| `eslint.config.js` (root) | ESLint v10 — imports frontend config |
| `frontend/package.json` | All — version bumps |
| `package.json` (root) | TS + Vite version bumps |
| `lefthook.yml` | ESLint v10 — remove pin note |
| `Dockerfile` | Node.js version (already compatible) |

### Infrastructure

- **Node.js:** `24.14.0-alpine` (Dockerfile) — meets all upgrade requirements
- **No `.npmrc` file exists** in the project
- **Go:** `1.26.1` (not affected by frontend upgrades)

---

## 3. Breaking Changes Analysis

### 3.1 ESLint v10 Breaking Changes

**Source:** [ESLint v10 Migration Guide](https://eslint.org/docs/latest/use/migrate-to-10.0.0)

| # | Breaking Change | Impact on Charon | Action Required |
|---|---|---|---|
| 1 | **Node.js ≥ v20.19, v22.13, or v24** required | None — already on Node 24.14.0 | None |
| 2 | **`eslint:recommended` updated** — 3 new rules: `no-unassigned-vars`, `no-useless-assignment`, `preserve-caught-error` | May flag new violations in codebase | Fix flagged code or disable rules |
| 3 | **New config file lookup** — searches from linted file, not cwd | Flat config already used; minor risk for monorepo patterns | Verify root config is found correctly |
| 4 | **Old `.eslintrc` format completely removed** | None — already using flat config | None |
| 5 | **JSX references now tracked** — fixes `no-unused-vars` for JSX components | Positive — fewer false positives | May surface new true positives |
| 6 | **`eslint-env` comments reported as errors** | Search codebase for `/* eslint-env */` | Remove if found |
| 7 | **Jiti ≥ v2.2.0 required** | Check transitive dep version | May need explicit install |
| 8 | **Removed deprecated `context` members** — `context.getScope()`, `context.getAncestors()`, etc. | Affects **plugins**, not our config directly | All 18 plugins must be compatible |
| 9 | **Removed deprecated `SourceCode` methods** | Same — plugin concern | Plugin compat verification |
| 10 | **Program AST node range spans entire source** | Unlikely to affect us | None |

**Critical Plugin Gate:** The `eslint-plugin-react-hooks` compatibility with ESLint v10 must be verified. The `lefthook.yml` at line ~98 explicitly states: _"NOTE: ESLint pinned at v9.x.x — do not upgrade until react-hooks plugin supports v10."_

### 3.2 TypeScript 6.0 Breaking Changes

**Source:** [TypeScript 6.0 Beta Announcement](https://devblogs.microsoft.com/typescript/announcing-typescript-6-0-beta/) and [6.0 Deprecation List](https://github.com/microsoft/TypeScript/issues/54500)

#### Default Value Changes

| Setting | Old Default | New Default | Charon Current | Action |
|---|---|---|---|---|
| `strict` | `false` | **`true`** | `true` (explicit) | None — already set |
| `module` | `commonjs` | **`esnext`** | `ESNext` (explicit) | None — already set |
| `target` | `es5` | **`es2025`** (floating) | `ES2022` (explicit) | None — already set |
| `types` | `["*"]` (all @types) | **`[]`** (none) | **Not set** | **ACTION: Add `"types": []`** |
| `rootDir` | inferred | **`.`** (tsconfig dir) | Not set | Verify — no emit, `noEmit: true` |
| `noUncheckedSideEffectImports` | `false` | **`true`** | Not set | Verify no side-effect import issues |
| `libReplacement` | `true` | **`false`** | Not set | None — improves perf |

#### Deprecations (with `ignoreDeprecations: "6.0"` escape hatch)

| Deprecation | Charon Uses? | Impact |
|---|---|---|
| `target: es5` | No (`ES2022`) | None |
| `--outFile` | No | None |
| `--downlevelIteration` | No | None |
| `--moduleResolution node/node10` | No (`bundler`) | None |
| `--moduleResolution classic` | No | None |
| `--baseUrl` | No | None |
| `module: amd/umd/systemjs` | No (`ESNext`) | None |
| `esModuleInterop: false` | Not explicitly set | None |
| `allowSyntheticDefaultImports: false` | Not set (`true` in tsconfig.node) | None |
| `alwaysStrict: false` | Not set (`strict: true` covers) | None |
| Legacy `module` keyword for namespaces | No | None |
| `asserts` keyword on imports | No | None |
| `no-default-lib` directives | No | None |

#### New Features Available

| Feature | Relevance |
|---|---|
| `import defer` syntax | Future use — deferred module evaluation |
| `--module node20` | Not needed — using bundler |
| `es2025` target/lib | Can update `target` from `ES2022` to `ES2025` |
| Temporal types | Available via `esnext` lib |
| `dom.iterable` included in `dom` | Can simplify `lib` array |
| `--stableTypeOrdering` | Useful for TS 7.0 migration prep |
| Expandable hovers | Editor UX improvement |
| `Map.getOrInsert` / `getOrInsertComputed` | Available via `esnext` lib |
| `RegExp.escape` | Available via `es2025` lib |
| `#/` subpath imports | Available for future module aliasing |

#### lib.d.ts Changes — ArrayBuffer/Buffer Breaking Change

TypeScript 5.9 introduced a behavioral change where `ArrayBuffer` is no longer a supertype of several `TypedArray` types. This may cause errors like:

```
error TS2345: Argument of type 'ArrayBufferLike' is not assignable to parameter of type 'BufferSource'.
error TS2322: Type 'Buffer' is not assignable to type 'Uint8Array<ArrayBufferLike>'.
```

**Mitigation:** Ensure `@types/node` is at latest version. This is a 5.9 → 6.0 carryover that must be verified.

### 3.3 Vite 8 Breaking Changes

**Status: Vite 8 does not exist.** The latest major is Vite 7 (released June 24, 2025). Charon is already on Vite 7.3.1.

**Monitoring targets:**

- Vite GitHub: `https://github.com/vitejs/vite/releases`
- Vite Blog: `https://vite.dev/blog/`
- Vitest compatibility (currently 4.0.18, compatible with Vite 7)

**When Vite 8 is announced, revisit this plan with:**

- Node.js minimum version
- Browser target defaults
- Plugin API changes (`@vitejs/plugin-react` compat)
- Vitest version compatibility
- Rolldown integration changes

---

## 4. Compatibility Matrix

### ESLint v10 Plugin Compatibility Verification Matrix

Each plugin must be verified before the ESLint v10 upgrade. The agent performing PR-2 must run these checks:

```bash
# For each plugin, check peer dependency support
npm info eslint-plugin-react-hooks peerDependencies
npm info eslint-plugin-react-compiler peerDependencies
npm info eslint-plugin-jsx-a11y peerDependencies
npm info eslint-plugin-import-x peerDependencies
npm info eslint-plugin-security peerDependencies
npm info eslint-plugin-sonarjs peerDependencies
npm info eslint-plugin-unicorn peerDependencies
npm info eslint-plugin-promise peerDependencies
npm info eslint-plugin-unused-imports peerDependencies
npm info eslint-plugin-no-unsanitized peerDependencies
npm info eslint-plugin-testing-library peerDependencies
npm info eslint-plugin-react-refresh peerDependencies
npm info @vitest/eslint-plugin peerDependencies
npm info typescript-eslint peerDependencies
npm info @eslint/css peerDependencies
npm info @eslint/json peerDependencies
npm info @eslint/markdown peerDependencies
```

**Decision Gate:** If `eslint-plugin-react-hooks` does NOT support ESLint v10 in its `peerDependencies`, the ESLint v10 upgrade is **BLOCKED**. Do not use `--legacy-peer-deps` or `--force` as a workaround.

### TypeScript 6.0 Ecosystem Compatibility

| Tool | TS 6.0 Compat | Notes |
|---|---|---|
| `typescript-eslint@8.57.0` | Likely — tracks TS closely | Verify with `npm install` |
| `vite@7.3.1` | Yes — Vite uses esbuild/swc, not tsc directly | Type-check is separate |
| `vitest@4.0.18` | Yes — same reasoning | Type-check is separate |
| `@vitejs/plugin-react@5.1.4` | Yes | No TS compiler dependency |
| `react@19.2.4` / `@types/react` | Yes | Ensure `@types/react` latest |
| `@tanstack/react-query@5.90.21` | Likely — popular library | TanStack already preparing for TS 6 |
| `knip@5.86.0` | Verify | Uses TS programmatic API |

### Node.js Compatibility

| Tool | Min Node.js | Charon Node.js | Status |
|---|---|---|---|
| ESLint v10 | 20.19 / 22.13 / 24+ | 24.14.0 | Compatible |
| TypeScript 6.0 | TBD (likely same as 5.9) | 24.14.0 | Compatible |
| Vite 7 | 20.19 / 22.12+ | 24.14.0 | Compatible |

---

## 5. `.npmrc` Configuration

**No `.npmrc` file currently exists in the project.** No changes needed for these upgrades.

If plugin compatibility issues arise during ESLint v10 upgrade, **do NOT create an `.npmrc` with `legacy-peer-deps=true`**. Instead, wait for plugin updates or use granular `overrides` in `package.json`:

```jsonc
// package.json — ONLY if a specific plugin ships a fix before updating peerDeps
{
  "overrides": {
    "eslint-plugin-EXAMPLE": {
      "eslint": "^10.0.0"
    }
  }
}
```

---

## 6. Dockerfile Changes

**No Dockerfile changes required** for ESLint v10 or TypeScript 6.0.

Current Dockerfile state:

```dockerfile
# Frontend builder stage
FROM node:24.14.0-alpine AS frontend-builder
# ...
RUN npm ci
RUN npm run build
```

- Node.js 24.14.0 meets all minimum requirements
- `npm ci` will install the upgraded versions from `package-lock.json`
- No new environment variables or build args needed
- Rollup native skip flags (`npm_config_rollup_skip_nodejs_native=1`) remain unchanged

**Future (Vite 8):** If Vite 8 requires a higher Node.js, upgrade the base image at that time.

---

## 7. Config File Changes

### 7.1 TypeScript 6.0 — `frontend/tsconfig.json`

```diff
  {
    "compilerOptions": {
      "target": "ES2022",
+     // Consider upgrading to "ES2025" (TS 6.0 new target)
      "useDefineForClassFields": true,
-     "lib": ["ES2022", "DOM", "DOM.Iterable"],
+     "lib": ["ES2022", "DOM"],
+     // DOM.Iterable is now included in DOM as of TS 6.0
      "module": "ESNext",
      "skipLibCheck": true,

      /* Bundler mode */
      "moduleResolution": "bundler",
      "allowImportingTsExtensions": true,
      "isolatedModules": true,
      "moduleDetection": "force",
      "noEmit": true,
      "jsx": "react-jsx",

      /* Linting */
      "strict": true,
      "noUnusedLocals": true,
      "noUnusedParameters": true,
      "noFallthroughCasesInSwitch": true,
+
+     /* TS 6.0 — explicit types to override new default of [] */
+     "types": []
    },
    "include": ["src"],
    "references": [{ "path": "./tsconfig.node.json" }]
  }
```

**Key changes:**

1. **`"types": []`** — Explicitly set to `[]`. Charon uses `noEmit: true` and doesn't rely on global `@types` packages in the main tsconfig. All types come from explicit imports.
2. **`"lib"` simplification** — Remove `"DOM.Iterable"` since TS 6.0 includes it in `"DOM"` automatically.
3. **`"target"` consideration** — Can optionally upgrade from `ES2022` to `ES2025` to access `RegExp.escape` and other ES2025 types natively. Not required.

### 7.2 TypeScript 6.0 — `frontend/tsconfig.node.json`

```diff
  {
    "compilerOptions": {
      "composite": true,
      "skipLibCheck": true,
      "module": "ESNext",
      "moduleResolution": "bundler",
      "allowSyntheticDefaultImports": true,
-     "strict": true
+     "strict": true,
+     "types": []
    },
    "include": ["vite.config.ts"]
  }
```

**Note:** `allowSyntheticDefaultImports` is fine — TS 6.0 deprecates setting it to `false`, not `true`. Setting it to `true` remains valid.

### 7.3 ESLint v10 — `frontend/package.json` Version Caps

```diff
  "devDependencies": {
-   "eslint": "^9.39.3 <10.0.0",
+   "eslint": "^10.0.0",
-   "@eslint/js": "^9.39.3 <10.0.0",
+   "@eslint/js": "^10.0.0",
    // ... all other ESLint plugins may need version bumps
  }
```

### 7.4 ESLint v10 — `frontend/eslint.config.js`

Likely no structural changes needed since Charon already uses flat config. Potential changes:

- Remove any `/* eslint-env */` comments found in source files
- Handle new `eslint:recommended` rules (`no-unassigned-vars`, `no-useless-assignment`, `preserve-caught-error`)
- Verify `tseslint.config()` wrapper compatibility

### 7.5 ESLint v10 — `lefthook.yml`

```diff
+ # NOTE: ESLint v10 is supported — plugin compatibility verified on [DATE]
- # NOTE: ESLint pinned at v9.x.x — do not upgrade until react-hooks plugin supports v10.
```

### 7.6 TypeScript 6.0 — `package.json` (Root + Frontend)

```diff
  "devDependencies": {
-   "typescript": "^5.9.3",
+   "typescript": "^6.0.0",
  }
```

---

## 8. Phase-by-Phase Implementation Plan

### Phase 1: Pre-Upgrade Verification (Both PRs)

**Owner:** Frontend_Dev agent (or whoever picks up the PR)

1. **Snapshot current state:**

   ```bash
   cd /projects/Charon && npm run lint 2>&1 | tee /tmp/eslint-v9-baseline.log
   cd /projects/Charon/frontend && npx tsc --noEmit 2>&1 | tee /tmp/tsc-v5-baseline.log
   ```

2. **Verify ESLint plugin compatibility (PR-2 gate):**

   ```bash
   for plugin in eslint-plugin-react-hooks eslint-plugin-react-compiler \
     eslint-plugin-jsx-a11y eslint-plugin-import-x eslint-plugin-security \
     eslint-plugin-sonarjs eslint-plugin-unicorn eslint-plugin-promise \
     eslint-plugin-unused-imports eslint-plugin-no-unsanitized \
     eslint-plugin-testing-library eslint-plugin-react-refresh \
     @vitest/eslint-plugin typescript-eslint @eslint/css @eslint/json @eslint/markdown; do
     echo "=== $plugin ===" && npm info "$plugin" peerDependencies 2>/dev/null
   done
   ```

3. **Search for `eslint-env` comments:**

   ```bash
   grep -r "eslint-env" frontend/src/ --include="*.ts" --include="*.tsx" --include="*.js"
   ```

### Phase 2: TypeScript 6.0 Upgrade (PR-1)

**Scope:** TypeScript version bump + tsconfig adjustments

1. Update `typescript` version in both `package.json` files:
   - Root: `^5.9.3` → `^6.0.0`
   - Frontend: `^5.9.3` → `^6.0.0`

2. Apply tsconfig changes (Section 7.1 and 7.2 above):
   - Add `"types": []` to `tsconfig.json` and `tsconfig.node.json`
   - Remove `"DOM.Iterable"` from `lib` array (now included in `"DOM"`)

3. Run `npm install` to update lock file

4. Run type-check and fix any new errors:

   ```bash
   cd frontend && npx tsc --noEmit
   ```

5. Common expected issues:
   - Missing types from `@types/*` packages (solved by `"types": []` since we don't use globals)
   - `ArrayBuffer`/`Buffer` type narrowing (from TS 5.9 lib.d.ts changes)
   - Type argument inference changes (may need explicit type annotations)

6. Run full test suite:

   ```bash
   cd frontend && npx vitest run
   ```

7. Run Playwright E2E tests to verify build works:

   ```bash
   # The Dockerfile builds with npm ci && npm run build
   # Verify: cd frontend && npx vite build
   ```

### Phase 3: ESLint v10 Upgrade (PR-2)

**Prerequisite:** Phase 1 plugin verification passes. `eslint-plugin-react-hooks` must declare ESLint v10 support.

1. Remove version cap and update ESLint packages:

   ```bash
   cd frontend
   npm install -D eslint@^10.0.0 @eslint/js@^10.0.0
   ```

2. Update any plugins that need version bumps for ESLint v10 compat

3. Run ESLint and compare against baseline:

   ```bash
   cd /projects/Charon && npm run lint 2>&1 | tee /tmp/eslint-v10-output.log
   diff /tmp/eslint-v9-baseline.log /tmp/eslint-v10-output.log
   ```

4. Address new violations from updated `eslint:recommended`:
   - `no-unassigned-vars` — variables declared but never assigned
   - `no-useless-assignment` — assignments that are immediately overwritten
   - `preserve-caught-error` — catch clause variables that are declared but unused

5. Remove any `/* eslint-env */` comments found in Phase 1

6. Update `lefthook.yml` — remove the ESLint v9 pin note

7. Run full test suite to confirm no regressions

### Phase 4: Integration Testing

1. **Full lint + type-check:**

   ```bash
   cd /projects/Charon && npm run lint && cd frontend && npx tsc --noEmit
   ```

2. **Frontend build:**

   ```bash
   cd frontend && npx vite build
   ```

3. **Unit tests:**

   ```bash
   cd frontend && npx vitest run
   ```

4. **Playwright E2E tests (all browsers):**

   ```bash
   npx playwright test --project=chromium
   npx playwright test --project=firefox
   npx playwright test --project=webkit
   ```

5. **Docker build verification:**

   ```bash
   docker build -t charon:upgrade-test .
   ```

### Phase 5: Vite 8 (Deferred)

**Action:** No implementation. Monitor only.

- [ ] Subscribe to Vite releases: `https://github.com/vitejs/vite/releases`
- [ ] When Vite 8 is announced, create a new plan with breaking changes analysis
- [ ] Key areas to watch: Node.js minimum, browser target defaults, plugin API, Vitest compat, Rolldown integration

---

## 9. Rollback Strategy

### TypeScript 6.0 Rollback (PR-1)

1. Revert `package.json` changes (both root and frontend):

   ```diff
   - "typescript": "^6.0.0"
   + "typescript": "^5.9.3"
   ```

2. Revert `tsconfig.json` changes (remove `"types": []`, restore `"DOM.Iterable"`)
3. Run `npm install` to restore lock file
4. Verify: `cd frontend && npx tsc --noEmit && npx vitest run`

**Risk:** Low — TypeScript version is a devDependency only. No runtime impact. `git revert` of the PR commit is sufficient.

### ESLint v10 Rollback (PR-2)

1. Revert `package.json` changes:

   ```diff
   - "eslint": "^10.0.0"
   + "eslint": "^9.39.3 <10.0.0"
   - "@eslint/js": "^10.0.0"
   + "@eslint/js": "^9.39.3 <10.0.0"
   ```

2. Revert any plugin version bumps
3. Revert `lefthook.yml` comment change
4. Run `npm install` to restore lock file
5. Verify: `cd /projects/Charon && npm run lint`

**Risk:** Low — ESLint is a devDependency only. Code changes (fixing new rule violations) are harmless to keep even if ESLint is rolled back.

### Vite 8 Rollback

N/A — no upgrade to perform.

---

## 10. Testing Strategy

### Automated Test Coverage

| Test Layer | Tool | What It Validates |
|---|---|---|
| Type checking | `tsc --noEmit` | TS 6.0 compatibility, tsconfig changes |
| Linting | `eslint` | ESLint v10 config + plugin compat |
| Unit tests | `vitest run` | No runtime regressions from TS changes |
| E2E tests | Playwright (Chromium, Firefox, WebKit) | Full app build + functionality |
| Docker build | `docker build` | Dockerfile still works with new deps |
| Pre-commit hooks | `lefthook` | All hooks pass with new versions |

### Specific Test Scenarios for TS 6.0

1. **Build output verification:**

   ```bash
   cd frontend && npx vite build
   # Verify dist/ output is correct, no new warnings
   ```

2. **Type-check with `--stableTypeOrdering`** (prep for TS 7.0):

   ```bash
   cd frontend && npx tsc --noEmit --stableTypeOrdering
   # Note any differences — these will be real in TS 7.0
   ```

3. **Verify no `@types` resolution issues:**

   ```bash
   # With types: [], ensure no global type errors appear
   cd frontend && npx tsc --noEmit 2>&1 | grep "Cannot find"
   ```

### Specific Test Scenarios for ESLint v10

1. **Verify all 18 plugins load without errors:**

   ```bash
   cd /projects/Charon && npx eslint --print-config frontend/src/App.tsx | head -20
   ```

2. **Count new violations vs baseline:**

   ```bash
   npx eslint frontend/src/ --format json 2>/dev/null | jq '.[] | .errorCount' | paste -sd+ | bc
   ```

3. **Verify config lookup works correctly in monorepo:**

   ```bash
   # Lint a file from the root — should find root eslint.config.js
   npx eslint frontend/src/App.tsx
   ```

---

## 11. Commit Slicing Strategy

### Decision: 2 Independent PRs + 1 Deferred

**Trigger reasons:**

- Cross-domain changes (TS and ESLint are independent tools)
- Risk isolation (if one breaks, the other can still merge)
- Review size (each PR is focused and reviewable)
- Plugin compatibility gate (ESLint v10 may be blocked)

### PR-1: TypeScript 6.0 Upgrade

| Attribute | Detail |
|---|---|
| **Scope** | TypeScript ^5.9.3 → ^6.0.0, tsconfig changes, fix type errors |
| **Files** | `package.json` (root), `frontend/package.json`, `package-lock.json`, `frontend/tsconfig.json`, `frontend/tsconfig.node.json`, possibly source files with type fixes |
| **Dependencies** | None — can start immediately |
| **Validation Gate** | `tsc --noEmit` passes, `vitest run` passes, `vite build` succeeds, Docker build succeeds |
| **Estimated Complexity** | Medium — mostly defaults are already correct, `types: []` is the main change |
| **Rollback** | `git revert` + `npm install` |

### PR-2: ESLint v10 Upgrade

| Attribute | Detail |
|---|---|
| **Scope** | ESLint ^9.x → ^10.0.0, plugin updates, fix new violations, update lefthook |
| **Files** | `frontend/package.json`, `package-lock.json`, `frontend/eslint.config.js` (if needed), `lefthook.yml`, source files with new violations |
| **Dependencies** | **BLOCKED** until `eslint-plugin-react-hooks` declares ESLint v10 support |
| **Validation Gate** | `npm run lint` passes, all plugins load, no new unhandled violations |
| **Estimated Complexity** | Medium — depends on plugin ecosystem readiness |
| **Rollback** | `git revert` + `npm install` |

### PR-3: Vite 8 (Deferred)

| Attribute | Detail |
|---|---|
| **Scope** | N/A — Vite 8 does not exist |
| **Dependencies** | Vite 8 release |
| **Action** | Monitor `https://github.com/vitejs/vite/releases` |

### Contingency

- If TS 6.0 stable is delayed past RC, pin to `typescript@6.0.0-rc` temporarily
- If ESLint v10 plugin compat is blocked for >30 days, consider temporarily dropping the blocker plugin or using `--rulesdir` workaround
- If a plugin is permanently abandoned, research replacement plugins

---

## 12. Known Issues & Gotchas

### ESLint v10

1. **react-hooks plugin blocker** — `lefthook.yml` explicitly states the upgrade is blocked until `eslint-plugin-react-hooks` supports v10. This is the #1 risk.

2. **Config file lookup change** — ESLint v10 finds config files starting from the linted file and walking up. In Charon's monorepo setup (root `eslint.config.js` imports `frontend/eslint.config.js`), verify the root config is still discovered when linting `frontend/src/**`.

3. **Jiti dependency** — ESLint v10 requires `jiti >= v2.2.0` for loading config files. This is typically a transitive dependency but may need explicit installation if conflicts arise.

4. **Plugin API breakage** — Plugins that use deprecated `context.getScope()`, `context.getAncestors()`, `context.parserOptions`, or `context.parserPath` will break. All 18 plugins must be verified.

### TypeScript 6.0

1. **`types: []` default** — This is the highest-impact change for Charon. Without explicitly setting `"types"`, TS 6.0 will not auto-load any `@types/*` packages. Since Charon uses `noEmit: true` and explicit imports, this should be fine, but test thoroughly.

2. **TS 6.0 is a transition release** — It is explicitly designed as a bridge to TS 7.0 (native Go port). Adopting TS 6.0 now prepares us for TS 7.0 later. The `ignoreDeprecations: "6.0"` escape hatch exists if needed.

3. **`typescript-eslint` compatibility** — If `typescript-eslint@8.57.0` doesn't support TS 6.0, we may need to update it. Check for a release that adds TS 6.0 support.

4. **`knip` compatibility** — `knip` (`^5.86.0`) uses TS programmatic API internally. Verify it works with TS 6.0.

5. **ArrayBuffer/Buffer types** — TS 5.9 changes to `lib.d.ts` around `ArrayBuffer` not being a supertype of `TypedArray` may surface with TS 6.0. Ensure `@types/node` is at latest.

6. **`ts5to6` migration tool** — The experimental [ts5to6](https://github.com/andrewbranch/ts5to6) tool can automatically adjust `baseUrl` and `rootDir`. Charon doesn't use `baseUrl`, so this is of limited value, but worth knowing about.

### Vite 8

1. **Does not exist** — No action possible. The latest major is Vite 7.

2. **Rolldown integration** — Vite 7 introduced `rolldown-vite` as an alternative bundler. Vite 8 may make Rolldown the default. Monitor this.

3. **`inlineDynamicImports: true` workaround** — `frontend/vite.config.ts` has a `TEMPORARY` comment on `inlineDynamicImports: true` for a "React init issue". This should be investigated independently regardless of any Vite upgrade.

---

## 13. Risk Assessment

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| `eslint-plugin-react-hooks` doesn't support ESLint v10 | **Medium** | **High** — blocks PR-2 entirely | Monitor npm for updates; check GitHub issues |
| Other ESLint plugins break on v10 | **Low** | **Medium** — individual plugins can be disabled | Verify all 18 plugins; have disable config ready |
| TS 6.0 `types: []` causes unexpected errors | **Medium** | **Low** — easy to fix by adding types | Test with `tsc --noEmit`; add specific types |
| `typescript-eslint` incompatible with TS 6.0 | **Low** | **Medium** — blocks type-aware linting | Check releases; may need to update |
| `knip` breaks with TS 6.0 | **Low** | **Low** — `knip` is optional tooling | Test separately; pin if needed |
| TS 6.0 stable delayed | **Low** | **Low** — RC already available | Use RC or pin beta |
| Vite 8 released with breaking changes | **Unknown** | **Unknown** | Create new plan when announced |
| Docker build fails after upgrades | **Low** | **Medium** — blocks CI/deployment | Test Docker build in PR CI |
| Playwright E2E failures from TS changes | **Very Low** | **High** — blocks merge | Run full E2E suite before merge |

### Overall Risk: **MEDIUM**

- TypeScript 6.0 is well-characterized and Charon's tsconfig is well-aligned with the new defaults
- ESLint v10 is dependent on ecosystem readiness (plugin compatibility)
- Vite 8 is a non-issue (doesn't exist)

---

## Acceptance Criteria

### PR-1 (TypeScript 6.0)

- [ ] `typescript` upgraded to `^6.0.0` in root and frontend `package.json`
- [ ] `tsconfig.json` updated with `types: []` and simplified `lib`
- [ ] `tsc --noEmit` passes with zero errors
- [ ] `vitest run` passes all tests
- [ ] `vite build` produces correct output
- [ ] Docker build succeeds
- [ ] No new `ignoreDeprecations` usage (clean upgrade)

### PR-2 (ESLint v10)

- [ ] Plugin compatibility verified for all 18 plugins
- [ ] `eslint` and `@eslint/js` upgraded to `^10.0.0`
- [ ] Version cap (`<10.0.0`) removed from both packages
- [ ] `npm run lint` passes (new violations fixed)
- [ ] `lefthook.yml` pin note removed/updated
- [ ] All pre-commit hooks pass

### PR-3 (Vite 8)

- [ ] Monitoring established for Vite releases
- [ ] Plan will be recreated when Vite 8 is announced
