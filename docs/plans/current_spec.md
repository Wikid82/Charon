# Caddy Import Tests Reorganization: Move from Security Shard to Core

**Date:** 2026-02-26
**Status:** Ready for Implementation

---

## 1. Introduction

### Overview

The 5 Caddyfile import UI test files were manually moved from
`tests/security-enforcement/zzz-caddy-imports/` to `tests/core/caddy-import/`.
These tests verify Caddyfile parsing/import UI functionality and do **not**
require Cerberus middleware — they belong in the non-security (core) shard.

### Objectives

1. Update CI workflow to reflect the new file locations.
2. Simplify the Playwright config by removing the now-unnecessary
   `crossBrowserCaddyImportSpec` / `securityEnforcementExceptCrossBrowser`
   special-case regex logic.
3. Fix one broken relative import in the moved test files.
4. Confirm all security UI tests remain in the security shard untouched.

---

## 2. Research Findings

### 2.1 Current File State

**Moved to `tests/core/caddy-import/` (confirmed present):**

| File | Description |
|------|-------------|
| `caddy-import-cross-browser.spec.ts` | Cross-browser Caddyfile import scenarios |
| `caddy-import-debug.spec.ts` | Diagnostic/debug tests for import flow |
| `caddy-import-firefox.spec.ts` | Firefox-specific edge cases |
| `caddy-import-gaps.spec.ts` | Gap coverage (conflict details, session resume, etc.) |
| `caddy-import-webkit.spec.ts` | WebKit-specific edge cases |

**Old directory `tests/security-enforcement/zzz-caddy-imports/`:** Fully removed (confirmed via filesystem scan).

### 2.2 Security Shard — Intact (No Changes Needed)

**`tests/security-enforcement/`** (17 files + 1 subdirectory):
- `acl-enforcement.spec.ts`, `acl-waf-layering.spec.ts`, `auth-api-enforcement.spec.ts`,
  `auth-middleware-cascade.spec.ts`, `authorization-rbac.spec.ts`,
  `combined-enforcement.spec.ts`, `crowdsec-enforcement.spec.ts`,
  `emergency-reset.spec.ts`, `emergency-server/`, `emergency-token.spec.ts`,
  `multi-component-security-workflows.spec.ts`, `rate-limit-enforcement.spec.ts`,
  `security-headers-enforcement.spec.ts`, `waf-enforcement.spec.ts`,
  `waf-rate-limit-interaction.spec.ts`, `zzz-admin-whitelist-blocking.spec.ts`,
  `zzzz-break-glass-recovery.spec.ts`

**`tests/security-enforcement/zzz-security-ui/`** (5 files):
- `access-lists-crud.spec.ts`, `crowdsec-import.spec.ts`,
  `encryption-management.spec.ts`, `real-time-logs.spec.ts`,
  `system-security-settings.spec.ts`

**`tests/security/`** (15 files):
- `acl-integration.spec.ts`, `audit-logs.spec.ts`, `crowdsec-config.spec.ts`,
  `crowdsec-console-enrollment.spec.ts`, `crowdsec-decisions.spec.ts`,
  `crowdsec-diagnostics.spec.ts`, `crowdsec-import.spec.ts`,
  `emergency-operations.spec.ts`, `rate-limiting.spec.ts`,
  `security-dashboard.spec.ts`, `security-headers.spec.ts`,
  `suite-integration.spec.ts`, `system-settings-feature-toggles.spec.ts`,
  `waf-config.spec.ts`, `workflow-security.spec.ts`

All of these require Cerberus ON and stay in the security shard.

### 2.3 Broken Import

In `tests/core/caddy-import/caddy-import-gaps.spec.ts` (line 20):

```typescript
import type { TestDataManager } from '../utils/TestDataManager';
```

This resolves to `tests/core/utils/TestDataManager` — **does not exist**.
The actual file is at `tests/utils/TestDataManager.ts`.

**Fix:** Change to `../../utils/TestDataManager`.

All other imports (`../../fixtures/auth-fixtures`) resolve correctly from the
new location.

---

## 3. Technical Specifications

### 3.1 CI Workflow Changes

**File:** `.github/workflows/e2e-tests-split.yml`

The non-security shards explicitly list test directories. Since they already
include `tests/core`, the new `tests/core/caddy-import/` directory is
**automatically picked up** — no CI changes needed for test path inclusion.

The security shards explicitly list `tests/security-enforcement/` and
`tests/security/`. Since `zzz-caddy-imports/` was removed from
`tests/security-enforcement/`, the caddy import tests are **automatically
excluded** from the security shard — no CI changes needed.

**Verification matrix:**

| Shard Type | Test Paths in Workflow | Picks Up `tests/core/caddy-import/`? |
|---|---|---|
| Security (Chromium, line 331-333) | `tests/security-enforcement/`, `tests/security/`, `tests/integration/multi-feature-workflows.spec.ts` | No |
| Security (Firefox, line 540-542) | Same pattern | No |
| Security (WebKit, line 749-751) | Same pattern | No |
| Non-Security Chromium (line 945-952) | `tests/core`, `tests/dns-provider-crud.spec.ts`, `tests/dns-provider-types.spec.ts`, `tests/integration`, `tests/manual-dns-provider.spec.ts`, `tests/monitoring`, `tests/settings`, `tests/tasks` | **Yes** (via `tests/core`) |
| Non-Security Firefox (line 1157-1164) | Same pattern | **Yes** |
| Non-Security WebKit (line 1369-1376) | Same pattern | **Yes** |

**Result: No CI workflow file changes required.**

### 3.2 Playwright Config Changes

**File:** `playwright.config.js`

The config has special-case regex logic (lines 38-41) that was created to
handle the old `zzz-caddy-imports` location within `security-enforcement/`:

```javascript
// CURRENT (lines 38-41) — references old, non-existent path
const crossBrowserCaddyImportSpec =
  /security-enforcement\/zzz-caddy-imports\/caddy-import-cross-browser\.spec\.(ts|js)$/;
const securityEnforcementExceptCrossBrowser =
  /security-enforcement\/(?!zzz-caddy-imports\/caddy-import-cross-browser\.spec\.(ts|js)$).*/;
```

Now that the caddy import tests live under `tests/core/caddy-import/`:
- `crossBrowserCaddyImportSpec` no longer matches any file — dead code.
- `securityEnforcementExceptCrossBrowser` negative lookahead is now
  unnecessary — all files in `security-enforcement/` are security tests.
- The browser projects' `testIgnore` already includes `'**/security/**'` and
  the simplified `security-enforcement` pattern will exclude all security tests.

**Required change:** Remove the special-case variables and simplify `testIgnore`
to use a plain `**/security-enforcement/**` glob.

#### Diff: `playwright.config.js`

```diff
 const skipSecurityDeps = process.env.PLAYWRIGHT_SKIP_SECURITY_DEPS !== '0';
 const browserDependencies = skipSecurityDeps ? ['setup'] : ['setup', 'security-tests'];
-const crossBrowserCaddyImportSpec =
-  /security-enforcement\/zzz-caddy-imports\/caddy-import-cross-browser\.spec\.(ts|js)$/;
-const securityEnforcementExceptCrossBrowser =
-  /security-enforcement\/(?!zzz-caddy-imports\/caddy-import-cross-browser\.spec\.(ts|js)$).*/;
```

For each of the 3 browser projects (chromium, firefox, webkit), change:

```diff
-      testMatch: [crossBrowserCaddyImportSpec, /.*\.spec\.(ts|js)$/],
-      testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**', securityEnforcementExceptCrossBrowser, '**/security/**'],
+      testMatch: /.*\.spec\.(ts|js)$/,
+      testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**', '**/security-enforcement/**', '**/security/**'],
```

**Rationale:** The `crossBrowserCaddyImportSpec` regex was a workaround to
include one specific file from the security-enforcement directory in cross-browser
runs. Now that all caddy import tests are under `tests/core/`, they are
naturally included by the default `.*\.spec\.(ts|js)$` pattern and naturally
excluded from the security ignore patterns.

### 3.3 Broken Import Fix

**File:** `tests/core/caddy-import/caddy-import-gaps.spec.ts` (line 20)

```diff
-import type { TestDataManager } from '../utils/TestDataManager';
+import type { TestDataManager } from '../../utils/TestDataManager';
```

**Rationale:** From the new location `tests/core/caddy-import/`, the correct
relative path to `tests/utils/TestDataManager.ts` is `../../utils/TestDataManager`.

---

## 4. Implementation Plan

### Phase 1: Fix Broken Import (1 file)

| Task | File | Change |
|------|------|--------|
| Fix `TestDataManager` import path | `tests/core/caddy-import/caddy-import-gaps.spec.ts:20` | `../utils/TestDataManager` → `../../utils/TestDataManager` |

### Phase 2: Simplify Playwright Config (1 file, 4 locations)

| Task | File | Lines | Change |
|------|------|-------|--------|
| Remove `crossBrowserCaddyImportSpec` variable | `playwright.config.js` | 38-39 | Delete |
| Remove `securityEnforcementExceptCrossBrowser` variable | `playwright.config.js` | 40-41 | Delete |
| Simplify Chromium project config | `playwright.config.js` | 269-270 | Replace `testMatch`/`testIgnore` |
| Simplify Firefox project config | `playwright.config.js` | 280-281 | Replace `testMatch`/`testIgnore` |
| Simplify WebKit project config | `playwright.config.js` | 291-292 | Replace `testMatch`/`testIgnore` |

### Phase 3: Validation

| Task | Command | Expected Result |
|------|---------|-----------------|
| Run caddy import tests locally (Firefox) | `npx playwright test --project=firefox tests/core/caddy-import/` | All 5 files discovered, tests execute |
| Run caddy import tests locally (all browsers) | `npx playwright test tests/core/caddy-import/` | Tests run on chromium, firefox, webkit |
| Verify security tests excluded from non-security run | `npx playwright test --project=firefox --list tests/core` | No security-enforcement files listed |
| Verify security shard unchanged | `npx playwright test --project=security-tests --list` | All security-enforcement + security files listed |

### Phase 4: Documentation

No external documentation changes needed. The archive docs in
`docs/reports/archive/` reference old paths but are historical records
and should not be updated.

---

## 5. Acceptance Criteria

- [ ] `tests/core/caddy-import/` contains all 5 caddy import test files.
- [ ] `tests/security-enforcement/zzz-caddy-imports/` no longer exists.
- [ ] All security UI tests remain in `tests/security-enforcement/zzz-security-ui/` and `tests/security/`.
- [ ] `caddy-import-gaps.spec.ts` import path resolves correctly.
- [ ] `playwright.config.js` has no references to `zzz-caddy-imports`.
- [ ] Non-security shards automatically pick up `tests/core/caddy-import/` via `tests/core`.
- [ ] Security shards do not run caddy import tests.
- [ ] No CI workflow file changes needed (paths already correct).
- [ ] Playwright test discovery lists caddy import files under all 3 browser projects.

---

## 6. PR Slicing Strategy

**Decision:** Single PR.

**Rationale:**
- Small scope: 2 files changed (1 import fix + 1 config simplification).
- Low risk: Test-only changes, no production code affected.
- No cross-domain concerns.
- Fully reversible.

### PR-1: Caddy Import Test Reorganization Cleanup

| Attribute | Value |
|-----------|-------|
| Scope | Fix broken import + simplify playwright config |
| Files | `tests/core/caddy-import/caddy-import-gaps.spec.ts`, `playwright.config.js` |
| Dependencies | None (file move already done manually) |
| Validation | Run `npx playwright test --project=firefox tests/core/caddy-import/` |
| Rollback | Revert the 2-file change |

---

## 7. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Caddy import tests silently dropped from CI | Low | High | Verify with `--list` that files are discovered |
| Security tests accidentally run in non-security shard | Low | Medium | `testIgnore` patterns verified against all security paths |
| Other tests break from playwright config change | Very Low | Medium | Only `testMatch`/`testIgnore` simplified; no new exclusions added |
