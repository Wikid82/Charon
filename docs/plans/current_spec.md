# TypeScript v6 Migration Plan

**Branch**: `feature/ts-v6`
**Author**: Planning Agent
**Created**: 2025-07-27
**Status**: Draft — Pending Review

---

## 1. Executive Summary

TypeScript 6.0 is a **bridge release** to TypeScript 7 (the native Go-rewrite). It introduces new defaults that enforce modern best practices and deprecates legacy options that will be hard-removed in TS 7. Charon's frontend is already well-positioned: `strict: true`, `module: "ESNext"`, `moduleResolution: "bundler"`, zero enums, zero `any` in source, and consistent `import type` usage. The migration is **low-risk** with a focused set of configuration changes and no source code rewrites expected.

### Impact Assessment

| Category | Risk | Effort |
|---|---|---|
| tsconfig changes | Low | Small (3 files) |
| Source code changes | Minimal | Near-zero |
| Dependency compatibility | Low | Verification only |
| Build pipeline | Low | Verify-and-go |
| Test suite | Low | Run-and-confirm |

**Estimated total effort**: 1–2 hours of focused work, delivered as a single PR.

---

## 2. Current State Audit

### 2.1 Runtime and Tooling Versions

| Tool | Current Version | Spec Range |
|---|---|---|
| TypeScript | 5.9.3 | `^5.9.3` |
| Node.js | 20.20.0 | — |
| Vite | 7.3.1 | `^7.3.1` |
| Vitest | 4.0.18 | `^4.0.18` |
| ESLint | 9.39.3 | `^9.39.3 <10.0.0` |
| typescript-eslint | 8.56.1 | `^8.56.1` |
| React | 19.2.4 | `^19.2.4` |
| @types/react | 19.2.14 | `^19.2.14` |
| @types/react-dom | 19.2.3 | `^19.2.3` |
| @types/node | 25.3.5 | `^25.3.5` |
| Playwright | 1.58.2 | `^1.58.2` |

### 2.2 tsconfig Topology

Three config files form a reference chain:

```
tsconfig.json (main)
├── tsconfig.node.json (vite.config.ts — composite)
└── tsconfig.build.json (extends tsconfig.json, excludes tests)
```

### 2.3 Key Compiler Options (tsconfig.json)

| Option | Current Value | TS6 Default | Impact |
|---|---|---|---|
| `target` | `"ES2022"` | `"es2025"` | None — explicit value preserved |
| `module` | `"ESNext"` | `"esnext"` | None — already set |
| `moduleResolution` | `"bundler"` | unchanged | None |
| `strict` | `true` | `true` | None — already set |
| `isolatedModules` | `true` | unchanged | None |
| `noEmit` | `true` | unchanged | None |
| `lib` | `["ES2022", "DOM", "DOM.Iterable"]` | — | Cleanup: `DOM.Iterable` folded into `DOM` in TS6 |
| `types` | *(not set — inherits default)* | `[]` | **ACTION REQUIRED**: must set explicitly |
| `rootDir` | *(not set)* | `.` | Review needed for build config |
| `skipLibCheck` | `true` | unchanged | None |
| `esModuleInterop` | *(not set — TS5 default: `false`)* | `true` | None — `noEmit: true` + bundler mode; see §3.1 |
| `noUncheckedSideEffectImports` | *(not set)* | `true` | Low risk — 7 side-effect imports, all legitimate |

### 2.4 tsconfig.node.json Key Options

| Option | Current Value | TS6 Default | Impact |
|---|---|---|---|
| `module` | `"ESNext"` | `"esnext"` | None |
| `moduleResolution` | `"bundler"` | unchanged | None |
| `strict` | `true` | `true` | None |
| `allowSyntheticDefaultImports` | `true` | unchanged | None |
| `types` | *(not set)* | `[]` | **ACTION REQUIRED**: must set explicitly |

> **Note**: `vitest.config.ts` uses `process.env` but is **not** covered by any project tsconfig. This is fine — Vitest handles its own TypeScript compilation internally and does not rely on the project's `tsc` for config files.

### 2.5 Codebase Pattern Analysis

| Pattern | Count | Location | Risk |
|---|---|---|---|
| `enum` declarations | 0 | — | None |
| `any` type in source | 0 | — | None |
| `@ts-expect-error` | 2 | Test files only | None |
| `import type` (standalone) | 30+ | Throughout | None — already correct |
| `import type` (inline) | 2 | — | None |
| Side-effect imports | 7 | Test setup + `main.tsx` CSS | Low — all legitimate |
| `import * as` | 20+ | Tests (vi.spyOn mocking) | None |
| `/// <reference>` directives | 4 | `.d.ts` + test setup | None |
| Path aliases (`paths`/`baseUrl`) | 0 | — | None — `baseUrl` deprecation irrelevant |
| Namespace/module declarations | 0 | — | None |
| Import assertions (`assert`) | 0 | — | None |
| `no-default-lib` directives | 0 | — | None |

---

## 3. TypeScript 6 Breaking Changes Analysis

### 3.1 New Defaults (Applies to Charon)

#### `types` defaults to `[]`

**Before (TS5)**: When `types` is omitted, TypeScript auto-includes all `@types/*` packages from `node_modules/@types/`.

**After (TS6)**: When `types` is omitted, TypeScript includes **no** `@types/*` packages. Only packages explicitly referenced via `import`, `/// <reference types="...">`, or listed in `types` are available.

**Charon Impact**: **Medium — requires explicit configuration**.
- `@types/node` globals (e.g., `process.env`) will no longer be available unless configured.
- `@types/react` and `@types/react-dom` are referenced via imports and JSX transforms, so they are likely fine.
- Vitest globals are already handled via `/// <reference types="vitest/globals">` in test setup.

**Required Action**: Add `"types": ["node"]` to `tsconfig.json` and `tsconfig.node.json`.

#### `noUncheckedSideEffectImports` defaults to `true`

**Before (TS5)**: Side-effect imports (`import "module"`) are not type-checked.

**After (TS6)**: Side-effect imports are type-checked — TS verifies the module resolves.

**Charon Impact**: **Low**. All 7 side-effect imports are legitimate:
- 5× `import '@testing-library/jest-dom/vitest'` (test files)
- 1× `import '@testing-library/jest-dom'` (setupTests.ts)
- 1× `import './index.css'` (main.tsx — CSS, handled by Vite)

CSS imports resolve fine because Vite provides type declarations via `vite/client`. Testing-library imports are proper npm packages with type declarations.

**Required Action**: None — verify by running `tsc` after upgrade.

#### `DOM.Iterable` folded into `DOM`

**Before (TS5)**: `DOM.Iterable` is a separate lib target.

**After (TS6)**: `DOM.Iterable` types are included in `DOM`. Specifying `DOM.Iterable` still works (no error) but is redundant.

**Charon Impact**: **Cosmetic cleanup only**.

**Required Action**: Remove `"DOM.Iterable"` from the `lib` array in `tsconfig.json`.

#### `rootDir` defaults to `.`

**Before (TS5)**: When `rootDir` is omitted, TypeScript infers it as the common parent of all input files.

**After (TS6)**: When `rootDir` is omitted, it defaults to `.` (the directory of the tsconfig).

**Charon Impact**: **None for main tsconfig** (`noEmit: true` means `rootDir` has no effect on output structure). **Low risk for tsconfig.build.json** which extends main tsconfig but inherits `noEmit: true` — however the build command is `tsc -p tsconfig.build.json && vite build`, so if `tsc` emits (contradicting `noEmit`), this matters. Since `noEmit: true` is inherited, the build tsconfig also does not emit. Vite handles the actual bundling.

**Required Action**: None — `noEmit: true` makes this irrelevant. Add a comment for documentation clarity.

#### `strict` defaults to `true`

**Charon Impact**: **None** — already explicitly set to `true`.

#### `module` defaults to `esnext` (when target ≥ es2025)

**Charon Impact**: **None** — already explicitly set to `"ESNext"`.

#### `esModuleInterop` defaults to `true`

**Before (TS5)**: When `esModuleInterop` is omitted, it defaults to `false`. CJS default imports require `import * as` syntax.

**After (TS6)**: When `esModuleInterop` is omitted, it defaults to `true`. CJS modules can use `import foo from 'bar'` syntax.

**Charon Impact**: **None**. The main `tsconfig.json` does not set `esModuleInterop`, so the effective value flips from `false` to `true`. However, this is safe because:
1. Charon uses `noEmit: true` — TypeScript never emits CJS interop helpers, so the runtime behaviour is unchanged.
2. Charon uses `moduleResolution: "bundler"` — Vite handles all module resolution and interop at build time.
3. No source files use `import * as` for CJS default imports; all such usage is in tests for `vi.spyOn` mocking (which is unrelated to CJS interop).

**Required Action**: None.

### 3.2 Deprecations (Not Affecting Charon)

The following TS6 deprecations do **not** apply because Charon does not use these features:

| Deprecated Feature | Charon Status |
|---|---|
| `target: "es5"` / `target: "es3"` | Uses `"ES2022"` |
| `moduleResolution: "node10"` / `"classic"` | Uses `"bundler"` |
| `baseUrl` (for module resolution) | Not configured |
| `outFile` | Not configured |
| `esModuleInterop: false` | Not configured — default flip analyzed in §3.1; safe |
| `allowSyntheticDefaultImports: false` | Explicitly `true` in `tsconfig.node.json` |
| `module: "amd"` / `"umd"` / `"system"` | Uses `"ESNext"` |
| `--downlevelIteration` | Not configured |
| Legacy `module` namespace syntax | Not used |
| `asserts` import keyword | Not used |
| `no-default-lib` directives | Not used |

### 3.3 New Features Available After Upgrade

| Feature | Description | Relevance |
|---|---|---|
| `es2025` target/lib | Iterator helpers, `Promise.try`, `Set` methods | Available if `target` is bumped |
| `Temporal` types | Modern date/time API types | Available via `"lib": ["esnext"]` |
| `RegExp.escape` | Safe regex escaping | Available |
| `--stableTypeOrdering` | Deterministic declaration emit | Useful for library authors (not Charon) |
| `#/` subpath imports | Package-internal imports | Available for future refactoring |

---

## 4. Dependency Compatibility Matrix

### 4.1 Critical Dependencies

| Package | Current | TS6 Support | Notes |
|---|---|---|---|
| `typescript` | 5.9.3 | — | Upgrading to 6.x |
| `vite` | 7.3.1 | ✅ Expected | Vite 7 ships with TS6 awareness; `moduleResolution: "bundler"` is the recommended path |
| `@vitejs/plugin-react` | 5.1.4 | ✅ Expected | Follows Vite compatibility |
| `vitest` | 4.0.18 | ✅ Expected | Vitest 4 tracks latest TS; verify with `npx vitest typecheck` |
| `typescript-eslint` | 8.56.1 | ⚠️ Verify | Major TS releases sometimes need typescript-eslint patch updates. Check release notes. May need bump to 8.57+ |
| `eslint` | 9.39.3 | ✅ Unaffected | ESLint itself is TS-agnostic; parsing is handled by typescript-eslint |

### 4.2 Type Definitions

| Package | Current | TS6 Support | Notes |
|---|---|---|---|
| `@types/node` | 25.3.5 | ✅ Expected | Tracks Node.js API, TS-version-independent |
| `@types/react` | 19.2.14 | ✅ Expected | Published by DefinitelyTyped, TS-version-independent |
| `@types/react-dom` | 19.2.3 | ✅ Expected | Same as above |

### 4.3 Build and Test Tooling

| Package | Current | TS6 Support | Notes |
|---|---|---|---|
| `@playwright/test` | 1.58.2 | ✅ Unaffected | JS config file, no TS compilation dependency |
| `postcss` | 8.5.8 | ✅ Unaffected | CSS tooling, no TS dependency |
| `tailwindcss` | 4.2.1 | ✅ Unaffected | CSS tooling, no TS dependency |
| `jsdom` | 28.1.0 | ✅ Unaffected | Runtime dependency, no TS dependency |
| `knip` | 5.86.0 | ⚠️ Verify | Uses TS compiler API internally; may need update |

### 4.4 UI Libraries (No TS6 Risk)

All of the following ship their own types or use standard TS features. No compatibility risk:
- `@radix-ui/*` (v1.x–v2.x)
- `@tanstack/react-query` (v5.90.21)
- `react-hook-form` (v7.71.2)
- `react-router-dom` (v7.13.1)
- `i18next` / `react-i18next`
- `axios`, `clsx`, `date-fns`, `lucide-react`

---

## 5. Required Changes

### 5.1 Configuration Changes

#### 5.1.1 `frontend/tsconfig.json`

```diff
 {
   "compilerOptions": {
     "target": "ES2022",
     "useDefineForClassFields": true,
-    "lib": ["ES2022", "DOM", "DOM.Iterable"],
+    "lib": ["ES2022", "DOM"],
     "module": "ESNext",
     "skipLibCheck": true,
+    "types": ["node"],

     /* Bundler mode */
     "moduleResolution": "bundler",
```

Changes:
1. **Add `"types": ["node"]`** — Restores `@types/node` globals that TS6 no longer auto-includes.
2. **Remove `"DOM.Iterable"`** — Folded into `"DOM"` in TS6; redundant.

#### 5.1.2 `frontend/tsconfig.node.json`

```diff
 {
   "compilerOptions": {
     "composite": true,
     "skipLibCheck": true,
     "module": "ESNext",
     "moduleResolution": "bundler",
     "allowSyntheticDefaultImports": true,
-    "strict": true
+    "strict": true,
+    "types": ["node"]
   },
   "include": ["vite.config.ts"]
 }
```

Changes:
1. **Add `"types": ["node"]`** — Vite's types reference Node.js globals (e.g., `NodeJS.Timeout`), and `types: ["node"]` ensures these resolve correctly during build-time type-checking.

#### 5.1.3 `frontend/tsconfig.build.json`

No changes required. It extends `tsconfig.json` and inherits the updated `types` and `lib` fields.

#### 5.1.4 `frontend/package.json`

```diff
-    "typescript": "^5.9.3",
+    "typescript": "~6.0.0",
```

Use `~6.0.0` (tilde) to pin to 6.0.x patch releases, avoiding accidental 6.1+ breakage during initial adoption. Broaden to `^6.0.0` once the ecosystem stabilises.

### 5.2 Source Code Changes

**None expected.** The codebase is clean:
- Zero enums, zero `any`, zero namespace declarations
- All imports use proper `import type` syntax
- No deprecated patterns (no `baseUrl`, no legacy module resolution)
- Side-effect imports resolve to real modules

### 5.3 Test Configuration Changes

**None expected.** Vitest configuration uses `/// <reference>` directives for type augmentation, which remain fully supported in TS6.

---

## 6. Migration Steps (Ordered)

### Step 1: Pre-Flight Check and Update TypeScript Package

**Pre-flight**: Before installing, check the [typescript-eslint release notes](https://github.com/typescript-eslint/typescript-eslint/releases) for TS6 compatibility. If a bump is required (e.g., to 8.57+), include it in the same install step.

```bash
cd frontend
# If typescript-eslint needs a bump, combine:
npm install --save-dev typescript@~6.0.0 typescript-eslint@latest
# Otherwise:
npm install --save-dev typescript@~6.0.0
npx tsc --version  # Verify 6.0.x
```

### Step 2: Apply tsconfig Changes

Edit the three files per Section 5.1:
1. `tsconfig.json` — add `types`, remove `DOM.Iterable`
2. `tsconfig.node.json` — add `types`
3. `tsconfig.build.json` — no changes (verify inheritance)

### Step 3: Verify Type-Check

```bash
cd frontend
npx tsc --noEmit          # Main project type-check
npx tsc -p tsconfig.node.json --noEmit  # Vite config type-check
npx tsc -p tsconfig.build.json --noEmit # Build config type-check
```

All three must exit 0 with no errors.

### Step 4: Verify Build

```bash
cd frontend
npm run build   # Runs tsc -p tsconfig.build.json && vite build
```

Must produce `dist/` output without errors.

### Step 5: Run Unit Tests

```bash
cd frontend
npm test        # Vitest
```

All tests must pass. Watch for:
- Type errors in test files (unlikely but possible from `types` change)
- Side-effect import resolution issues (unlikely)

### Step 6: Verify Vitest Type-Check

```bash
cd frontend
npx vitest typecheck --run
```

Verifies that Vitest's built-in type-checking (which uses its own TS compilation) works with TS6.

### Step 7: Run Lint

```bash
cd frontend
npx eslint .
```

Watch for typescript-eslint compatibility issues. If errors appear, check for a newer `typescript-eslint` release with TS6 support.

### Step 8: Verify Knip

```bash
cd frontend
npx knip
```

Knip uses the TS compiler API. Verify it still works.

### Step 9: Run E2E Tests

```bash
cd /projects/Charon
npx playwright test --project=firefox
```

E2E tests use compiled/bundled output and should be unaffected, but this validates the full pipeline.

### Step 10: Update typescript-eslint (If Needed)

If Step 7 fails or emits warnings about TS6:

```bash
cd frontend
npm install --save-dev typescript-eslint@latest @typescript-eslint/eslint-plugin@latest @typescript-eslint/parser@latest
```

### Step 11: Update knip (If Needed)

If Step 8 fails:

```bash
cd frontend
npm install --save-dev knip@latest
```

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `typescript-eslint` incompatibility | Medium | Medium (lint failures) | Pre-flight check in Step 1; pin typescript-eslint to latest; check release notes before install |
| `knip` incompatibility | Low | Low (dev tool only) | Update or temporarily disable |
| Side-effect import resolution failure | Very Low | Medium (type errors) | All imports verified as legitimate; CSS handled by Vite types |
| `@types/node` not resolving after `types: ["node"]` | Very Low | High (build failure) | Explicit `types` array is the documented fix |
| Vitest or Vite internal TS version mismatch | Very Low | Low | Both tools use their own TS internally for compilation |
| Hidden dependency on auto-included `@types/*` | Low | Medium (type errors in unexpected places) | Full `tsc --noEmit` check catches this immediately |

### Rollback Plan

If the migration encounters **blocking** issues:

1. Revert `package.json` typescript version to `^5.9.3`
2. Revert tsconfig changes (restore `"DOM.Iterable"`, remove `"types"` field)
3. Run `npm install` to restore TS 5.9.3
4. Verify with `npx tsc --noEmit && npm run build && npm test`

Total rollback time: < 5 minutes. All changes are localised to `frontend/` configuration files.

---

## 8. Testing Strategy

### 8.1 Type-Check Validation

WHEN typescript is upgraded to v6, THE SYSTEM SHALL pass `tsc --noEmit` on all three tsconfig files with zero errors.

### 8.2 Build Validation

WHEN the frontend build command executes, THE SYSTEM SHALL produce `dist/` output identical in structure to pre-migration output.

### 8.3 Unit Test Validation

WHEN the vitest suite runs, THE SYSTEM SHALL pass all existing tests with no regressions.

### 8.4 Lint Validation

WHEN eslint runs with typescript-eslint, THE SYSTEM SHALL report no new errors attributable to the TS version change.

### 8.5 E2E Validation

WHEN Playwright tests execute against the built application, THE SYSTEM SHALL pass all existing tests with no regressions.

### 8.6 Acceptance Criteria

- [ ] `npx tsc --noEmit` exits 0
- [ ] `npx tsc -p tsconfig.node.json --noEmit` exits 0
- [ ] `npx tsc -p tsconfig.build.json --noEmit` exits 0
- [ ] `npm run build` exits 0 and produces `dist/`
- [ ] `npm test` — all tests pass
- [ ] `npm run type-check` — exits 0
- [ ] `npx vitest typecheck --run` — exits 0
- [ ] `npx eslint .` — no new errors
- [ ] `npx playwright test --project=firefox` — all tests pass
- [ ] `npx knip` — no new findings from TS upgrade

---

## 9. Commit Slicing Strategy

### Decision: Single PR

**Rationale**: This migration touches only configuration files in `frontend/`. There are zero source code changes, zero API changes, and zero database changes. The scope is small, the risk is low, and all changes are atomically related. A single PR provides the simplest review surface and avoids unnecessary coordination overhead.

**Trigger reasons evaluated**:
- Cross-domain changes? **No** — frontend only
- Risk zones? **No** — no source code changes
- Review size? **Small** — 3 config files + package.json
- Rollback complexity? **Trivial** — revert package version + 2 config fields

### PR-1: TypeScript v6 Migration

**Scope**: All changes described in Section 5.

**Files Modified**:
1. `frontend/package.json` — TypeScript version bump
2. `frontend/tsconfig.json` — `types` and `lib` changes
3. `frontend/tsconfig.node.json` — `types` addition
4. `frontend/package-lock.json` — auto-updated by npm install

**Dependencies**: None — self-contained.

**Validation Gates**:
1. Type-check (3 tsconfigs)
2. Build (`npm run build`)
3. Unit tests (`npm test`)
4. Lint (`npx eslint .`)
5. E2E (`npx playwright test --project=firefox`)
6. Lefthook pre-commit

**Rollback**: Revert the 3 modified files + `npm install`.

---

## 10. Post-Migration Considerations

### 10.1 Future-Proofing for TypeScript 7

TS6 is explicitly a bridge to TS7 (native Go port). Deprecations in TS6 become hard removals in TS7. Charon is **already compliant** with all known TS7 requirements:
- No deprecated options in use
- Modern module resolution (`bundler`)
- Modern module system (`ESNext`)
- Modern target (`ES2022`)
- Explicit `types` array (after this migration)

### 10.2 Optional Improvements (Not In Scope)

These are not required for the migration but could be considered in follow-up work:

| Improvement | Benefit | Effort |
|---|---|---|
| Bump `target` to `"ES2025"` | Access to Iterator helpers, Set methods, `Promise.try` | Low — verify browser support matrix |
| Add `verbatimModuleSyntax: true` | Stricter import/export type checking | Low — already using `import type` consistently |
| Add `isolatedDeclarations: true` | Faster declaration emit, parallelizable builds | Low — no declaration emit currently |
| Remove `useDefineForClassFields` | TS6 defaults align with modern class semantics | Trivial — verify no class field usage relying on legacy |
| Add `noUncheckedSideEffectImports: true` explicitly | Self-documenting; already the TS6 default | Trivial |

---

## Appendix A: Side-Effect Imports Inventory

| File | Import | Type | Risk |
|---|---|---|---|
| `src/main.tsx` | `import './index.css'` | CSS | None — Vite `vite/client` types handle CSS modules |
| `src/test/setup.ts` | `import '@testing-library/jest-dom'` | Test augmentation | None — has type declarations |
| `src/test/setup.ts` | `import '@testing-library/jest-dom/vitest'` | Test augmentation | None — has type declarations |
| Multiple test files | `import '@testing-library/jest-dom/vitest'` | Test augmentation | None — same as above |

## Appendix B: `/// <reference>` Directives Inventory

| File | Directive | Purpose |
|---|---|---|
| `src/vite-env.d.ts` | `/// <reference types="vite/client" />` | Vite client types (CSS modules, env vars) |
| `src/test-shims.d.ts` | `/// <reference types="@testing-library/jest-dom/vitest" />` | Jest-DOM matchers for Vitest |
| `src/test/setup.ts` | `/// <reference types="vitest/globals" />` | Vitest global test functions |
| `src/test/setup.ts` | `/// <reference types="@testing-library/jest-dom/vitest" />` | Jest-DOM matchers for Vitest |

## Appendix C: ts5to6 Migration Tool

The `@andrewbranch/ts5to6` tool automates two specific migrations:
- `--fixBaseUrl` — rewrites `baseUrl`-relative imports to explicit relative paths
- `--fixRootDir` — adds explicit `rootDir` when inference changes would affect output

**Charon applicability**: Neither fix is needed. Charon does not use `baseUrl` and uses `noEmit: true` (making `rootDir` irrelevant for output structure).

Command (for reference only):
```bash
npx @andrewbranch/ts5to6 --fixBaseUrl . && npx @andrewbranch/ts5to6 --fixRootDir .
```
