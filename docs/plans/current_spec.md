# Major Dependency Upgrade Plan — ESLint v10, TypeScript 6.0, Vite 8

**Date:** 2026-03-12
**Author:** Planning Agent
**Status:** Ready for Review
**Confidence Score:** 82% (High for ESLint v10 + TS 6.0; Medium for Vite 8 — beta with Rolldown migration)

---

## 1. Executive Summary

This plan covers the upgrade of three major frontend toolchain dependencies in the Charon project:

| Dependency | Current Version | Target Version | Status | Risk |
|---|---|---|---|---|
| **ESLint** | `^9.39.3 <10.0.0` | `^10.0.0` | Released | **Medium** — plugin compat gate |
| **TypeScript** | `^5.9.3` | `^6.0.0` | Beta (Feb 11) / RC (Mar 6) | **Medium** — 17+ deprecations |
| **Vite** | `^7.3.1` | `8.0.0-beta.18` | Beta (Dec 3, 2025) | **High** — beta, Rolldown replaces Rollup+esbuild |

### Key Findings

1. **ESLint v10** is released with a comprehensive migration guide. The primary blocker is a note in `lefthook.yml`: _"ESLint pinned at v9.x.x — do not upgrade until react-hooks plugin supports v10."_ The `eslint-plugin-react-hooks@7.0.1` must be verified for ESLint v10 compatibility before proceeding.

2. **TypeScript 6.0** is real (Beta: Feb 11, 2026; RC: Mar 6, 2026). It is explicitly designed as a **bridge release** between TS 5.9 and the native Go-based TS 7.0. It introduces 17+ deprecations/breaking changes (new defaults for `strict`, `module`, `target`, `types`, `rootDir`; removal of `outFile`, legacy module systems; deprecated `baseUrl`, `moduleResolution: node`). Charon's current `tsconfig.json` is well-positioned — it already uses `moduleResolution: bundler`, `strict: true`, and `module: ESNext`. The **critical impact** is the `types` default changing to `[]`.

3. **Vite 8 exists as `8.0.0-beta.18`** (announced Dec 3, 2025). The headline change is **Rolldown replaces both Rollup and esbuild**. JS transforms and minification now use Oxc; CSS minification uses Lightning CSS. The `build.rollupOptions` config key is deprecated in favor of `build.rolldownOptions`, and `output.manualChunks` (object form) is removed. Charon's `vite.config.ts` uses `rollupOptions` with `inlineDynamicImports: true` — both need migration. Ecosystem packages (`@vitejs/plugin-react`, `vitest`) require beta versions for Vite 8 compatibility.

### Recommended Execution Order

```
PR-1: TypeScript 6.0 upgrade (fewer external dependencies, most self-contained)
PR-2: ESLint v10 upgrade (blocked on plugin compat verification)
PR-3: Vite 8 upgrade (beta — stacked on PR-1 + PR-2 branch)
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

**Source:** [Vite 8 Beta Announcement](https://vite.dev/blog/announcing-vite8-beta) and [Migration from v7 Guide](https://main.vite.dev/guide/migration)

**Version:** `8.0.0-beta.18` (dist-tag: `beta`, announced Dec 3, 2025)

#### Core Architecture Change: Rolldown Replaces Rollup + esbuild

Vite 8's defining change is replacing **two bundlers** (esbuild for dev transforms, Rollup for production builds) with a single Rust-based toolchain:

| Component | Vite 7 | Vite 8 | Impact on Charon |
|---|---|---|---|
| **Bundler** | Rollup | **Rolldown** (`1.0.0-rc.8`) | `rollupOptions` → `rolldownOptions` |
| **JS Transforms** | esbuild | **Oxc** (`@oxc-project/runtime@0.115.0`) | `esbuild` config key deprecated |
| **JS Minification** | esbuild | **Oxc Minifier** | Different minification assumptions |
| **CSS Minification** | esbuild | **Lightning CSS** (`^1.31.1`) | Slightly different output, bundle size may change |
| **Dep Optimization** | esbuild | **Rolldown** | `optimizeDeps.esbuildOptions` deprecated |

#### Breaking Changes Impacting Charon

| # | Breaking Change | Impact on Charon | Action Required |
|---|---|---|---|
| 1 | **Node.js `^20.19.0 \|\| >=22.12.0`** required | None — already on Node 24.14.0 | None |
| 2 | **`build.rollupOptions` deprecated** → `build.rolldownOptions` | **HIGH** — `vite.config.ts` uses `rollupOptions` | Rename config key |
| 3 | **`output.manualChunks` object form removed**, function form deprecated | **HIGH** — config sets `manualChunks: undefined` | Remove or migrate to `codeSplitting` |
| 4 | **`output.inlineDynamicImports`** — supported in Rolldown but **deprecated** in favor of `codeSplitting: false` ([rolldown docs](https://rolldown.rs/reference/OutputOptions.inlineDynamicImports)) | **HIGH** — config uses `inlineDynamicImports: true` as temporary workaround | Migrate to `codeSplitting: false`; `inlineDynamicImports` works as fallback |
| 5 | **Default browser targets updated** (Chrome 107→111, Firefox 104→114, Safari 16.0→16.4) | Low — Charon doesn't set explicit `build.target` | None — new defaults are fine |
| 6 | **esbuild no longer a direct dependency** | Low — Charon doesn't use esbuild config | None |
| 7 | **Oxc Minifier** replaces esbuild minifier | Low — different assumptions about source code | Test build output; verify no minification breakage |
| 8 | **Lightning CSS** for CSS minification | Low — may produce slightly different CSS output | Verify CSS output visually |
| 9 | **Consistent CommonJS interop** — `default` import behavior changes for CJS modules | Medium — could affect CJS dependencies (axios, etc.) | Test all runtime imports |
| 10 | **Module resolution format sniffing removed** — `browser`/`module` field heuristic gone | Low — modern packages use `exports` field | Verify no resolution regressions |
| 11 | **`@vitejs/plugin-react` 5.x does NOT support Vite 8** — requires `6.0.0-beta.0` | **HIGH** — must upgrade plugin-react | Upgrade to `@vitejs/plugin-react@6.0.0-beta.0` |
| 12 | **Plugin-react 6.0 uses `@rolldown/pluginutils`** instead of Rollup utils | Low — internal plugin change | None — handled by plugin upgrade |

#### New Features Available

| Feature | Relevance to Charon |
|---|---|
| Built-in tsconfig `paths` support (`resolve.tsconfigPaths: true`) | Could replace manual alias config if needed |
| `emitDecoratorMetadata` support | Not needed — Charon doesn't use decorators |
| Performance: 10–30× faster production builds | Direct benefit — faster Docker builds and CI |
| Full Bundle Mode (upcoming) | Future — 3× faster dev server startup |
| Module-level persistent cache (upcoming) | Future — faster rebuilds |

#### Dockerfile Impact: Rollup Native Skip Flags

The current Dockerfile sets:

```dockerfile
ENV npm_config_rollup_skip_nodejs_native=1 \
    ROLLUP_SKIP_NODEJS_NATIVE=1
```

These env vars are **Rollup-specific** for cross-platform builds. With Vite 8, Rollup is replaced by Rolldown, which uses its own native bindings (`@rolldown/binding-linux-x64-musl` for Alpine). These env vars become no-ops but do not cause harm. Rolldown's native bindings are installed per-platform by npm's `optionalDependencies` mechanism — the same mechanism that works for the `$BUILDPLATFORM` Docker flag.

**Action:** Remove the Rollup skip flags from Dockerfile and verify cross-platform builds still work. Rolldown includes `@rolldown/binding-linux-x64-musl` which is exactly what Alpine requires.

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
| Vite 8 | 20.19 / 22.12+ | 24.14.0 | Compatible |

### Vite 8 Ecosystem Compatibility Matrix

All Vite-related packages must be updated together. Stable releases do **not** support Vite 8.

| Package | Current Version | Vite 8 Compatible? | Required Version | Override Needed? |
|---|---|---|---|---|
| `vite` | `^7.3.1` | — | `8.0.0-beta.18` | No — direct install |
| `@vitejs/plugin-react` | `^5.1.4` | **No** (5.x peer: `vite: ^4.2.0 \|\| ^5.0.0 \|\| ^6.0.0 \|\| ^7.0.0`) | `6.0.0-beta.0` (peer: `vite: ^8.0.0` — verified via `npm info`) | No — direct install |
| `vitest` | `^4.0.18` | **No** (deps: `^6.0.0 \|\| ^7.0.0`) | `4.1.0-beta.6` (deps: `^6.0.0 \|\| ^7.0.0 \|\| ^8.0.0-0`) | No — 4.1.0-beta.6 dep range includes Vite 8 |
| `@vitest/coverage-istanbul` | `^4.0.18` | **No** (peer: `vitest: 4.0.18`) | `4.1.0-beta.6` | No — matches vitest beta |
| `@vitest/coverage-v8` | `^4.0.18` | **No** (peer: `vitest: 4.0.18`) | `4.1.0-beta.6` | No — matches vitest beta |
| `@vitest/ui` | `^4.0.18` | **No** (peer: `vitest: 4.0.18`) | `4.1.0-beta.6` | No — matches vitest beta |
| `@vitest/eslint-plugin` | `^1.6.10` | Yes (peer: `vitest: *`) | Keep current | No |
| `@bgotink/playwright-coverage` | `^0.3.2` | Yes (no Vite peer dep) | Keep current | No |
| `@playwright/test` | `^1.58.2` | Yes (no Vite peer dep) | Keep current | No |

**Key constraints:**

- `vitest@4.0.18` has `vite` in its **dependencies** (not peer deps) pinned to `^6.0.0 || ^7.0.0` — this will refuse Vite 8 unless overridden
- `vitest@4.1.0-beta.6` extends this to `^6.0.0 || ^7.0.0 || ^8.0.0-0` — supports Vite 8 beta
- `@vitejs/plugin-react@6.0.0-beta.0` peers on `vite: ^8.0.0` (verified via `npm info`). New optional peer deps: `@rolldown/plugin-babel` and `babel-plugin-react-compiler` (both optional — not required)
- All `@vitest/*` packages at `4.1.0-beta.6` must be installed together (strict peer version matching: `vitest: 4.1.0-beta.6`)
- Since `vitest@4.1.0-beta.6` already includes `^8.0.0-0` in its `vite` dependency range, and all `@vitest/*` packages peer to exact `vitest: 4.1.0-beta.6`, **no npm overrides are needed** when all packages are installed in lockstep at their beta versions

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

**Vite 8 requires Dockerfile changes** — the Rollup native skip flags become irrelevant:

```diff
  # Set environment to bypass native binary requirement for cross-arch builds
- ENV npm_config_rollup_skip_nodejs_native=1 \
-     ROLLUP_SKIP_NODEJS_NATIVE=1
+ # Vite 8 uses Rolldown (Rust native bindings, auto-resolved per platform)
+ # No skip flags needed — Rolldown's optionalDependencies handle cross-platform
```

Current Dockerfile state (frontend-builder stage):

```dockerfile
FROM --platform=$BUILDPLATFORM node:24.14.0-alpine AS frontend-builder
# ...
ENV npm_config_rollup_skip_nodejs_native=1 \
    ROLLUP_SKIP_NODEJS_NATIVE=1
RUN npm ci
COPY frontend/ ./
RUN npm run build
```

- Node.js 24.14.0 meets Vite 8's requirement (`^20.19.0 || >=22.12.0`)
- `npm ci` will install Rolldown's `@rolldown/binding-linux-x64-musl` automatically on Alpine
- `--platform=$BUILDPLATFORM` ensures native bindings match the build machine architecture
- The `VITE_APP_VERSION` env var and build output (`dist/`) remain unchanged
- No new environment variables or build args needed

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

### Phase 5: Vite 8 Upgrade (PR-3 — stacked commit on same branch)

**Prerequisites:** PR-1 (TypeScript 6.0) and PR-2 (ESLint v10) already committed on branch.

**Scope:** Vite `^7.3.1` → `8.0.0-beta.18`, plugin-react `^5.1.4` → `6.0.0-beta.0`, vitest `^4.0.18` → `4.1.0-beta.6`, vite.config.ts migration, Dockerfile cleanup.

#### Step 1: Install Vite 8 and ecosystem packages

```bash
cd /projects/Charon/frontend

# Core Vite upgrade
npm install -D vite@8.0.0-beta.18

# Plugin-react upgrade (6.x required for Vite 8)
npm install -D @vitejs/plugin-react@6.0.0-beta.0

# Vitest + coverage upgrades (4.1.0-beta.6 supports Vite 8)
npm install -D vitest@4.1.0-beta.6 \
  @vitest/coverage-istanbul@4.1.0-beta.6 \
  @vitest/coverage-v8@4.1.0-beta.6 \
  @vitest/ui@4.1.0-beta.6
```

#### Step 2: Update root `package.json` (direct version bump only — no overrides)

The root `package.json` only has `vite` as a direct devDependency (used by Playwright). It does **not** need overrides — just a version bump:

```bash
cd /projects/Charon
npm install -D vite@8.0.0-beta.18
```

#### Step 3: Verify peer dep resolution (overrides likely NOT needed)

With all packages at their Vite 8-compatible versions, overrides should not be necessary:

- `vitest@4.1.0-beta.6` depends on `vite: ^6.0.0 || ^7.0.0 || ^8.0.0-0` — already includes Vite 8
- `@vitejs/plugin-react@6.0.0-beta.0` peers on `vite: ^8.0.0` — matches
- All `@vitest/*@4.1.0-beta.6` peer on `vitest: 4.1.0-beta.6` — matches when installed in lockstep

Run `npm install` and check for peer dep warnings. **Only add overrides in `frontend/package.json`** (following the established pattern from TS 6.0 and ESLint v10 phases) if specific transitive packages fail to resolve:

```jsonc
// frontend/package.json — ONLY if npm install reports unresolved peer deps
{
  "overrides": {
    // ... existing TS and ESLint overrides ...
    // Add scoped overrides ONLY for the specific package that fails, e.g.:
    // "some-transitive-package": { "vite": "8.0.0-beta.18" }
  }
}
```

**Do NOT add a top-level `"vite": "8.0.0-beta.18"` override** — this forces every transitive Vite consumer to resolve to the beta, which is overly broad. If a broad override is truly needed after testing, add it with a comment explaining which transitive package requires it.

#### Step 4: Migrate `vite.config.ts`

```diff
  import react from '@vitejs/plugin-react'
  import { defineConfig } from 'vite'

  export default defineConfig({
    plugins: [react()],
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: 'dist',
      sourcemap: true,
-     // TEMPORARY: Disable code splitting to diagnose React initialization issue
-     // If this works, the problem is module loading order in async chunks
      chunkSizeWarningLimit: 2000,
-     rollupOptions: {
-       output: {
-         // Disable code splitting - bundle everything into one file
-         manualChunks: undefined,
-         inlineDynamicImports: true
-       }
-     }
+     rolldownOptions: {
+       output: {
+         // Disable code splitting — single bundle for React init stability
+         // codeSplitting: false is the Rolldown-native approach
+         // (inlineDynamicImports is deprecated in Rolldown)
+         codeSplitting: false
+       }
+     }
    }
  })
```

**Key changes:**
1. `rollupOptions` → `rolldownOptions` (Rollup config key deprecated)
2. `manualChunks: undefined` removed (object form no longer supported; was already a no-op since `undefined`)
3. `inlineDynamicImports: true` replaced with `codeSplitting: false` — the Rolldown-native equivalent. Rolldown supports `inlineDynamicImports` but marks it as [deprecated](https://rolldown.rs/reference/OutputOptions.inlineDynamicImports) in favor of `codeSplitting: false`.
4. The TEMPORARY comment is preserved in intent — this workaround may still be needed

**Fallback if `codeSplitting: false` behaves differently than expected:**

```ts
build: {
  rolldownOptions: {
    output: {
      // Deprecated but still functional in Rolldown 1.0.0-rc.8
      inlineDynamicImports: true
    }
  }
}
```

#### Step 5: Update Dockerfile

Remove the now-irrelevant Rollup native skip flags:

```diff
- ENV npm_config_rollup_skip_nodejs_native=1 \
-     ROLLUP_SKIP_NODEJS_NATIVE=1
+ # Vite 8: Rolldown native bindings auto-resolved per platform via optionalDependencies
```

#### Step 6: Run `npm install` to regenerate lock file

```bash
cd /projects/Charon && npm install
cd /projects/Charon/frontend && npm install
```

#### Step 7: Verify builds and tests

```bash
# 1. Frontend build (most critical — tests Rolldown bundling)
cd /projects/Charon/frontend && npx vite build

# 2. Type-check (should be unaffected)
cd /projects/Charon/frontend && npx tsc --noEmit

# 3. Lint (should be unaffected)
cd /projects/Charon && npm run lint

# 4. Unit tests
cd /projects/Charon/frontend && npx vitest run

# 5. Docker build (tests Rolldown on Alpine/musl)
docker build -t charon:vite8-test .

# 6. Playwright E2E (tests the built app end-to-end)
cd /projects/Charon && npx playwright test --project=firefox

# 7. CJS interop smoke test (verify axios, react-hot-toast, react-hook-form)
# Run the app and manually verify pages that use CJS dependencies render correctly
# See Step 9 for detailed CJS interop verification checklist
```

#### Step 8: Verify build output

```bash
# Compare build output size and structure
ls -la frontend/dist/assets/
# Should still produce index-*.js, index-*.css
# With codeSplitting: false, should be a single JS bundle
```

#### Step 9: Verify CJS interop (Vite 8 behavior change)

Vite 8's consistent CJS interop may affect imports from CJS packages like `axios` and `react-hot-toast`. **Explicitly verify these packages work at runtime:**

```bash
# After Docker build or vite build + preview:
# 1. Verify axios API calls work (CJS package with __esModule flag)
#    - Navigate to any page that makes API calls (e.g., Dashboard)
#    - Check browser console for "default is not a function" errors
# 2. Verify react-hot-toast renders (CJS package)
#    - Trigger a toast notification (e.g., save settings)
#    - Check browser console for import errors
# 3. Verify react-hook-form works (CJS interop)
#    - Open any form page, submit a form
```

If any runtime errors appear (e.g., `default is not a function`), use the temporary escape hatch:

```ts
// vite.config.ts — ONLY if CJS interop breaks
export default defineConfig({
  legacy: {
    inconsistentCjsInterop: true
  }
})
```

#### Step 10: Update `ARCHITECTURE.md`

Update the Frontend technology stack table and directory structure to reflect current versions:

```diff
  ### Frontend
  | Component | Technology | Version | Purpose |
- | **Build Tool** | Vite | 6.1.9 | Fast bundler and dev server |
+ | **Build Tool** | Vite | 8.0.0-beta.18 | Fast bundler and dev server |
- | **CSS Framework** | Tailwind CSS | 3.x | Utility-first CSS |
+ | **CSS Framework** | Tailwind CSS | 4.2.1 | Utility-first CSS |
- | **Unit Testing** | Vitest | 2.x | Fast unit test runner |
+ | **Unit Testing** | Vitest | 4.1.0-beta.6 | Fast unit test runner |
- | **E2E Testing** | Playwright | 1.50.x | Browser automation |
+ | **E2E Testing** | Playwright | 1.58.2 | Browser automation |
```

Also fix the directory structure reference:

```diff
- │   └── vite.config.js          # Vite configuration
+ │   └── vite.config.ts          # Vite configuration
```

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

### Vite 8 Rollback (PR-3 commit)

1. Revert `vite` version in both `package.json` files:

   ```diff
   - "vite": "8.0.0-beta.18"
   + "vite": "^7.3.1"
   ```

2. Revert ecosystem packages in `frontend/package.json`:

   ```diff
   - "@vitejs/plugin-react": "6.0.0-beta.0"
   + "@vitejs/plugin-react": "^5.1.4"
   - "vitest": "4.1.0-beta.6"
   + "vitest": "^4.0.18"
   - "@vitest/coverage-istanbul": "4.1.0-beta.6"
   + "@vitest/coverage-istanbul": "^4.0.18"
   - "@vitest/coverage-v8": "4.1.0-beta.6"
   + "@vitest/coverage-v8": "^4.0.18"
   - "@vitest/ui": "4.1.0-beta.6"
   + "@vitest/ui": "^4.0.18"
   ```

3. Revert `vite.config.ts`: `rolldownOptions` → `rollupOptions`, restore `manualChunks: undefined`

4. Revert Dockerfile: restore `ROLLUP_SKIP_NODEJS_NATIVE=1` env vars

5. Remove Vite 8 overrides from `frontend/package.json`

6. Run `npm install` to restore lock file

7. Verify: `cd frontend && npx vite build && npx vitest run`

**Risk:** Medium — Vite 8 is a pre-release beta. More likely to need rollback than stable upgrades. Since this is a stacked commit on the same branch, `git revert HEAD` cleanly removes only the Vite 8 changes while preserving TS 6.0 and ESLint v10.

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

### Decision: 3 Stacked Commits on Single Branch

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

### PR-3: Vite 8 Upgrade (stacked commit on same branch)

| Attribute | Detail |
|---|---|
| **Scope** | Vite 7→8, plugin-react 5→6, vitest 4.0→4.1-beta, vite.config.ts migration, Dockerfile cleanup |
| **Files** | `package.json` (root), `frontend/package.json`, `package-lock.json`, `frontend/vite.config.ts`, `Dockerfile`, `ARCHITECTURE.md` |
| **Dependencies** | PR-1 (TS 6.0) and PR-2 (ESLint v10) already committed on branch |
| **Validation Gate** | `vite build` succeeds with Rolldown, `vitest run` passes, Docker build succeeds, Playwright E2E passes |
| **Estimated Complexity** | **High** — beta software, bundler engine swap (Rollup→Rolldown), multiple ecosystem packages at beta versions |
| **Rollback** | `git revert HEAD` — cleanly removes only the Vite 8 commit |

#### npm Overrides for PR-3

**No overrides expected** when all packages are installed at their beta versions in lockstep:
- `vitest@4.1.0-beta.6` deps include `vite: ^8.0.0-0` — resolves Vite 8 without override
- `@vitest/*@4.1.0-beta.6` peer on `vitest: 4.1.0-beta.6` — satisfied by direct install

If `npm install` fails, add **scoped** overrides in `frontend/package.json` only for the failing package. Do not add a broad `"vite": "8.0.0-beta.18"` override.

### Contingency

- If TS 6.0 stable is delayed past RC, pin to `typescript@6.0.0-rc` temporarily
- If ESLint v10 plugin compat is blocked for >30 days, consider temporarily dropping the blocker plugin or using `--rulesdir` workaround
- If a plugin is permanently abandoned, research replacement plugins
- If Vite 8 beta has blocking regressions, `git revert` the Vite 8 commit and wait for the next beta or stable release — TS 6.0 + ESLint v10 upgrades remain unaffected
- If `vitest@4.1.0-beta.6` fails tests, try pinning `vitest@4.0.18` with an `overrides` entry for its `vite` dependency (force it to accept `^8.0.0-0`)
- If Rolldown's `codeSplitting: false` behaves differently than expected, try the deprecated `inlineDynamicImports: true` as a fallback, or re-investigate the React initialization issue that motivated the workaround

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

1. **Beta software** — `8.0.0-beta.18` is pre-release. Expect edge cases and undocumented behavior. File issues at `https://github.com/vitejs/rolldown-vite/issues`.

2. **Rolldown bundler is RC, not stable** — Vite 8 depends on `rolldown@1.0.0-rc.8`. Rolldown is feature-complete but may have edge cases with complex chunk splitting configurations.

3. **`codeSplitting: false` replaces `inlineDynamicImports: true`** — `frontend/vite.config.ts` has a `TEMPORARY` workaround for a "React init issue". Rolldown supports `inlineDynamicImports` but marks it as [deprecated](https://rolldown.rs/reference/OutputOptions.inlineDynamicImports) in favor of `codeSplitting: false`. The migration uses `codeSplitting: false` as the primary approach; `inlineDynamicImports: true` can be used as a deprecated fallback.

4. **Oxc Minifier assumptions differ from esbuild** — The Oxc Minifier makes [different assumptions](https://oxc.rs/docs/guide/usage/minifier.html#assumptions) about source code than esbuild. If runtime errors appear after build but not in dev, the minifier is the likely culprit. Use `build.minify: false` temporarily to diagnose.

5. **CJS interop behavior change** — Vite 8 changes how `default` imports from CommonJS modules work. Packages like `axios` (CJS) may be affected. The `legacy.inconsistentCjsInterop: true` escape hatch exists if needed.

6. **All ecosystem packages are beta** — `@vitejs/plugin-react@6.0.0-beta.0`, `vitest@4.1.0-beta.6`, and all `@vitest/*` packages are pre-release. They are tightly version-locked (e.g., `@vitest/coverage-v8` peers to exact `vitest: 4.1.0-beta.6`).

7. **Plugin-react 6.0 API change** — The new `@vitejs/plugin-react@6.0.0-beta.0` uses `@rolldown/pluginutils` internally instead of `@rollup/pluginutils`. The public API (`react()` call in config) appears unchanged. New optional peer deps (`@rolldown/plugin-babel`, `babel-plugin-react-compiler`) are not required for Charon's usage.

8. **Lightning CSS may increase CSS bundle size** — Lightning CSS produces slightly different output than esbuild's CSS minifier. Verify CSS output and check for visual regressions.

9. **Cross-platform Docker builds** — Rolldown uses native Rust bindings per platform (`@rolldown/binding-linux-x64-musl` for Alpine). The `--platform=$BUILDPLATFORM` Docker flag ensures the correct binding is installed. If cross-arch builds fail, verify the correct `@rolldown/binding-*` package is being resolved.

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
| Vite 8 beta breaks production build | **Medium** | **High** — blocks Docker/deployment | Test `vite build` thoroughly; rollback with `git revert` |
| Rolldown CJS interop breaks runtime imports | **Medium** | **Medium** — runtime errors on CJS packages | Test all CJS deps (axios, etc.); use `legacy.inconsistentCjsInterop` escape |
| Oxc Minifier causes runtime errors | **Low** | **High** — minification bugs are subtle | Compare dev vs prod behavior; use `build.minify: false` to diagnose |
| `vitest@4.1.0-beta.6` incompatible with test suite | **Low** | **Medium** — blocks unit test validation | Pin to `4.0.18` + override vite peer if needed |
| `@vitejs/plugin-react@6.0.0-beta.0` breaks React HMR | **Low** | **Medium** — dev experience degraded | Rollback to 5.1.4 + Vite 7 if critical |
| Rolldown native binding fails on Alpine cross-build | **Low** | **High** — blocks Docker build entirely | Verify `@rolldown/binding-linux-x64-musl` resolves; fall back to non-cross-platform build |
| Lightning CSS produces visual CSS regressions | **Low** | **Low** — cosmetic issues only | Visual diff E2E screenshots |
| Docker build fails after upgrades | **Low** | **Medium** — blocks CI/deployment | Test Docker build in PR CI |
| Playwright E2E failures from TS changes | **Very Low** | **High** — blocks merge | Run full E2E suite before merge |

### Overall Risk: **MEDIUM-HIGH**

- TypeScript 6.0 is well-characterized and Charon's tsconfig is well-aligned with the new defaults
- ESLint v10 is dependent on ecosystem readiness (plugin compatibility)
- **Vite 8 is the highest-risk change** — beta software with a complete bundler engine swap (Rollup→Rolldown). The saving grace is that all three upgrades are separate commits on the same branch, enabling surgical rollback of just the Vite 8 commit if needed

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

- [ ] `vite` upgraded to `8.0.0-beta.18` in root and frontend `package.json`
- [ ] `@vitejs/plugin-react` upgraded to `6.0.0-beta.0`
- [ ] `vitest` upgraded to `4.1.0-beta.6` with matching `@vitest/*` packages
- [ ] `vite.config.ts` migrated: `rollupOptions` → `rolldownOptions`, `manualChunks` removed
- [ ] npm overrides verified: no broad overrides needed (or scoped overrides added with justification)
- [ ] Dockerfile: Rollup native skip flags removed
- [ ] `vite build` produces correct output with Rolldown bundler
- [ ] `vitest run` passes all unit tests
- [ ] `tsc --noEmit` still passes (unchanged from PR-1)
- [ ] Docker build succeeds with Rolldown on Alpine/musl
- [ ] Playwright E2E tests pass (all browsers)
- [ ] No CJS interop runtime errors (axios, react-hot-toast, etc.)
- [ ] CJS interop verified: axios API calls, react-hot-toast renders, react-hook-form submits work
- [ ] CSS output visually correct (Lightning CSS minification)
- [ ] `ARCHITECTURE.md` updated: Vite 8.0.0-beta.18, Vitest 4.1.0-beta.6, Playwright 1.58.2, Tailwind CSS 4.2.1, `vite.config.ts` filename
- [ ] Pre-commit hooks pass (`lefthook`)
