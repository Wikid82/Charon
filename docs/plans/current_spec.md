# WAF Integration Workflow Fix: wget-style curl Syntax Migration

**Plan ID**: WAF-2026-001
**Status**: 📋 PENDING
**Priority**: High
**Created**: 2026-01-25
**Scope**: Fix integration test scripts using incorrect wget-style curl syntax

---

## Problem Summary

After migrating the Docker base image from Alpine to Debian Trixie (PR #550), the WAF integration workflow is failing. The root cause is **not** a missing `wget` command, but rather several integration test scripts using **wget-style options with curl** that don't work correctly.

### Root Cause

Multiple scripts use `curl -q -O-` which is **wget syntax, not curl syntax**:

| Syntax | Tool | Meaning |
|--------|------|---------|
| `-q` | **wget** | Quiet mode |
| `-q` | **curl** | **Invalid** - does nothing useful |
| `-O-` | **wget** | Output to stdout |
| `-O-` | **curl** | **Wrong** - `-O` means "save with remote filename", `-` is treated as a separate URL |

The correct curl equivalents are:
| wget | curl | Notes |
|------|------|-------|
| `wget -q` | `curl -s` | Silent mode |
| `wget -O-` | `curl -s` | stdout is curl's default output |
| `wget -q -O- URL` | `curl -s URL` | Full equivalent |
| `wget -O filename` | `curl -o filename` | Note: lowercase `-o` in curl |

---

## Files Requiring Changes

### Priority 1: Integration Test Scripts (Blocking WAF Workflow)

| File | Line | Current Code | Issue |
|------|------|--------------|-------|
| [scripts/waf_integration.sh](../../scripts/waf_integration.sh#L205) | 205 | `curl -q -O- http://${BACKEND_CONTAINER}/get` | wget syntax |
| [scripts/cerberus_integration.sh](../../scripts/cerberus_integration.sh#L214) | 214 | `curl -q -O- http://${BACKEND_CONTAINER}/get` | wget syntax |
| [scripts/rate_limit_integration.sh](../../scripts/rate_limit_integration.sh#L190) | 190 | `curl -q -O- http://${BACKEND_CONTAINER}/get` | wget syntax |
| [scripts/crowdsec_startup_test.sh](../../scripts/crowdsec_startup_test.sh#L178) | 178 | `curl -q -O- http://127.0.0.1:8085/health` | wget syntax |

### Priority 2: Utility Scripts

| File | Line | Current Code | Issue |
|------|------|--------------|-------|
| [scripts/install-go-1.25.5.sh](../../scripts/install-go-1.25.5.sh#L18) | 18 | `curl -q -O "$TMPFILE" "URL"` | Wrong syntax - `-O` doesn't take an argument in curl |

---

## Detailed Fixes

### Fix 1: scripts/waf_integration.sh (Line 205)

**Current (broken):**
```bash
if docker exec ${CONTAINER_NAME} sh -c "curl -q -O- http://${BACKEND_CONTAINER}/get 2>/dev/null || curl -s http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
```

**Fixed:**
```bash
if docker exec ${CONTAINER_NAME} sh -c "curl -sf http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
```

**Notes:**
- `-s` = silent (no progress meter)
- `-f` = fail silently on HTTP errors (returns non-zero exit code)
- Removed redundant fallback since the fix makes the command work correctly

---

### Fix 2: scripts/cerberus_integration.sh (Line 214)

**Current (broken):**
```bash
if docker exec ${CONTAINER_NAME} sh -c "curl -q -O- http://${BACKEND_CONTAINER}/get 2>/dev/null || curl -s http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
```

**Fixed:**
```bash
if docker exec ${CONTAINER_NAME} sh -c "curl -sf http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
```

---

### Fix 3: scripts/rate_limit_integration.sh (Line 190)

**Current (broken):**
```bash
if docker exec ${CONTAINER_NAME} sh -c "curl -q -O- http://${BACKEND_CONTAINER}/get 2>/dev/null || curl -s http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
```

**Fixed:**
```bash
if docker exec ${CONTAINER_NAME} sh -c "curl -sf http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
```

---

### Fix 4: scripts/crowdsec_startup_test.sh (Line 178)

**Current (broken):**
```bash
LAPI_HEALTH=$(docker exec ${CONTAINER_NAME} curl -q -O- http://127.0.0.1:8085/health 2>/dev/null || echo "FAILED")
```

**Fixed:**
```bash
LAPI_HEALTH=$(docker exec ${CONTAINER_NAME} curl -sf http://127.0.0.1:8085/health 2>/dev/null || echo "FAILED")
```

---

### Fix 5: scripts/install-go-1.25.5.sh (Line 18)

**Current (broken):**
```bash
curl -q -O "$TMPFILE" "https://go.dev/dl/${TARFILE}"
```

**Fixed:**
```bash
curl -sSfL -o "$TMPFILE" "https://go.dev/dl/${TARFILE}"
```

**Notes:**
- `-s` = silent
- `-S` = show errors even in silent mode
- `-f` = fail on HTTP errors
- `-L` = follow redirects (important for go.dev downloads)
- `-o filename` = output to specified file (lowercase `-o`)

---

## Verification Commands

After applying fixes, verify each script works:

```bash
# Test WAF integration
./scripts/waf_integration.sh

# Test Cerberus integration
./scripts/cerberus_integration.sh

# Test Rate Limit integration
./scripts/rate_limit_integration.sh

# Test CrowdSec startup
./scripts/crowdsec_startup_test.sh

# Verify Go install script syntax
bash -n ./scripts/install-go-1.25.5.sh
```

---

## Behavior Differences: wget vs curl

When migrating from wget to curl, be aware of these differences:

| Behavior | wget | curl |
|----------|------|------|
| Output destination | File by default | stdout by default |
| Follow redirects | Yes by default | Requires `-L` flag |
| Retry on failure | Built-in retry | Requires `--retry N` |
| Progress display | Text progress bar | Progress meter (use `-s` to hide) |
| HTTP error handling | Non-zero exit on 404 | Requires `-f` for non-zero exit on HTTP errors |
| Quiet mode | `-q` | `-s` (silent) |
| Output to file | `-O filename` (uppercase) | `-o filename` (lowercase) |
| Save with remote name | `-O` (no arg) | `-O` (uppercase, no arg) |

---

## Execution Checklist

- [ ] **Fix 1**: Update `scripts/waf_integration.sh` line 205
- [ ] **Fix 2**: Update `scripts/cerberus_integration.sh` line 214
- [ ] **Fix 3**: Update `scripts/rate_limit_integration.sh` line 190
- [ ] **Fix 4**: Update `scripts/crowdsec_startup_test.sh` line 178
- [ ] **Fix 5**: Update `scripts/install-go-1.25.5.sh` line 18
- [ ] **Verify**: Run each integration test locally
- [ ] **CI**: Confirm WAF integration workflow passes

---

## Notes

1. **Deprecated Scripts**: Several affected scripts are marked deprecated (will be removed in v2.0.0). However, they are still used by CI workflows, so fixes are required.

2. **Skill-Based Replacements**: The `.github/skills/scripts/` directory was checked and contains no wget usage - those scripts already use correct curl syntax.

3. **Docker Compose Files**: All health checks in docker-compose files already use correct curl syntax (`curl -f`, `curl -fsS`).

4. **Dockerfile**: The main Dockerfile correctly installs `curl` and uses correct curl syntax in the HEALTHCHECK instruction.

---

# Previous Plan (Archived)

The previous Git & Workflow Recovery Plan has been archived below.

---

# Git & Workflow Recovery Plan (ARCHIVED)

**Plan ID**: GIT-2026-001
**Status**: ✅ ARCHIVED
**Priority**: High
**Created**: 2026-01-25
**Scope**: Git recovery, Renovate fix, Workflow simplification

---

## Problem Summary

1. **Git State**: Feature branch `feature/beta-release` is in a broken rebase state
2. **Renovate**: Targeting feature branches creates orphaned PRs and merge conflicts
3. **Propagate Workflow**: Overly complex cascade (`main → development → nightly → feature/*`) causes confusion
4. **Nightly Branch**: Unnecessary intermediate branch adding complexity

---

## Phase 1: Git Recovery

### Step 1.1 — Abort the Rebase

```bash
# Check current state
git status

# Abort the in-progress rebase
git rebase --abort

# Verify clean state
git status
```

### Step 1.2 — Fetch Latest from Origin

```bash
# Fetch all branches
git fetch origin --prune

# Ensure we're on the feature branch
git checkout feature/beta-release
```

### Step 1.3 — Merge Development into Feature Branch

**Use merge, NOT rebase** to preserve commit history and avoid force-push issues.

```bash
# Merge development into feature/beta-release
git merge origin/development --no-ff -m "Merge development into feature/beta-release"
```

### Step 1.4 — Resolve Conflicts (if any)

Likely conflict files based on Renovate activity:
- `package.json` / `package-lock.json` (version bumps)
- `backend/go.mod` / `backend/go.sum` (Go dependency updates)
- `.github/workflows/*.yml` (action digest pins)

**Resolution strategy:**
```bash
# For package.json - accept development's versions, then run npm install
git checkout --theirs package.json package-lock.json
npm install
git add package.json package-lock.json

# For go.mod/go.sum - accept development's versions, then tidy
git checkout --theirs backend/go.mod backend/go.sum
cd backend && go mod tidy && cd ..
git add backend/go.mod backend/go.sum

# For workflow files - usually safe to accept development
git checkout --theirs .github/workflows/

# Complete the merge
git commit
```

### Step 1.5 — Push the Merged Branch

```bash
git push origin feature/beta-release
```

---

## Phase 2: Renovate Fix

### Problem

Current config in `.github/renovate.json`:
```json
"baseBranches": [
  "development",
  "feature/beta-release"
]
```

This causes:
- Duplicate PRs for the same dependency (one per branch)
- Orphaned branches like `renovate/feature/beta-release-*` when feature merges
- Constant merge conflicts between branches

### Solution

Only target `development`. Changes flow naturally via propagate workflow.

### Old Config (REMOVE)

```json
{
  "baseBranches": [
    "development",
    "feature/beta-release"
  ],
  ...
}
```

### New Config (REPLACE WITH)

```json
{
  "baseBranches": [
    "development"
  ],
  ...
}
```

### File to Edit

**File**: `.github/renovate.json`
**Line**: ~12-15

---

## Phase 3: Propagate Workflow Fix

### Problem

Current workflow in `.github/workflows/propagate-changes.yml`:

```yaml
on:
  push:
    branches:
      - main
      - development
      - nightly  # <-- Unnecessary
```

Cascade logic:
- `main` → `development` ✅ (Correct)
- `development` → `nightly` ❌ (Unnecessary)
- `nightly` → `feature/*` ❌ (Overly complex)

### Solution

Simplify to **only** `main → development` propagation.

### Old Trigger (REMOVE)

```yaml
on:
  push:
    branches:
      - main
      - development
      - nightly
```

### New Trigger (REPLACE WITH)

```yaml
on:
  push:
    branches:
      - main
```

### Old Script Logic (REMOVE)

```javascript
if (currentBranch === 'main') {
  // Main -> Development
  await createPR('main', 'development');
} else if (currentBranch === 'development') {
  // Development -> Nightly
  await createPR('development', 'nightly');
} else if (currentBranch === 'nightly') {
  // Nightly -> Feature branches
  const branches = await github.paginate(github.rest.repos.listBranches, {
    owner: context.repo.owner,
    repo: context.repo.repo,
  });

  const featureBranches = branches
    .map(b => b.name)
    .filter(name => name.startsWith('feature/'));

  core.info(`Found ${featureBranches.length} feature branches: ${featureBranches.join(', ')}`);

  for (const featureBranch of featureBranches) {
    await createPR('development', featureBranch);
  }
}
```

### New Script Logic (REPLACE WITH)

```javascript
if (currentBranch === 'main') {
  // Main -> Development (only propagation needed)
  await createPR('main', 'development');
}
```

### File to Edit

**File**: `.github/workflows/propagate-changes.yml`

---

## Phase 4: Cleanup

### Step 4.1 — Delete Nightly Branch

```bash
# Delete remote nightly branch (if exists)
git push origin --delete nightly 2>/dev/null || echo "nightly branch does not exist"

# Delete local tracking branch
git branch -D nightly 2>/dev/null || true
```

### Step 4.2 — Delete Orphaned Renovate Branches

```bash
# List all renovate branches targeting feature/beta-release
git fetch origin
git branch -r | grep 'renovate/feature/beta-release' | while read branch; do
  remote_branch="${branch#origin/}"
  echo "Deleting: $remote_branch"
  git push origin --delete "$remote_branch"
done
```

### Step 4.3 — Close Orphaned Renovate PRs

After branches are deleted, any associated PRs will be automatically closed by GitHub.

---

## Execution Checklist

- [ ] **Phase 1**: Git Recovery
  - [ ] 1.1 Abort rebase
  - [ ] 1.2 Fetch latest
  - [ ] 1.3 Merge development
  - [ ] 1.4 Resolve conflicts
  - [ ] 1.5 Push merged branch

- [ ] **Phase 2**: Renovate Fix
  - [ ] Edit `.github/renovate.json` - remove `feature/beta-release` from baseBranches
  - [ ] Commit and push

- [ ] **Phase 3**: Propagate Workflow Fix
  - [ ] Edit `.github/workflows/propagate-changes.yml` - simplify triggers and logic
  - [ ] Commit and push

- [ ] **Phase 4**: Cleanup
  - [ ] 4.1 Delete nightly branch
  - [ ] 4.2 Delete orphaned `renovate/feature/beta-release-*` branches
  - [ ] 4.3 Verify orphaned PRs are closed

---

## Verification

After all phases complete:

```bash
# Confirm no rebase in progress
git status
# Expected: "On branch feature/beta-release" with clean state

# Confirm nightly deleted
git branch -r | grep nightly
# Expected: no output

# Confirm orphaned renovate branches deleted
git branch -r | grep 'renovate/feature/beta-release'
# Expected: no output

# Confirm Renovate config only targets development
cat .github/renovate.json | grep -A2 baseBranches
# Expected: only "development"
```

---

## Rollback Plan

If issues occur:

1. **Git Recovery Failed**:
   ```bash
   git fetch origin
   git checkout feature/beta-release
   git reset --hard origin/feature/beta-release
   ```

2. **Renovate Changes Broke Something**: Revert the commit to `.github/renovate.json`

3. **Propagate Workflow Issues**: Revert the commit to `.github/workflows/propagate-changes.yml`

---

## Archived Spec (Prior Implementation)

# Security Fix: Remove Hardcoded Encryption Keys from Docker Compose Files

**Plan ID**: SEC-2026-001
**Status**: ✅ IMPLEMENTED
**Priority**: Critical (Security)
**Created**: 2026-01-25
**Implemented By**: Management Agent

---

### Summary

Removed hardcoded encryption keys from Docker Compose test files and implemented ephemeral key generation in CI workflows.

### Changes Applied

| File | Change |
|------|--------|
| `.docker/compose/docker-compose.playwright.yml` | Replaced hardcoded key with `${CHARON_ENCRYPTION_KEY:?...}` |
| `.docker/compose/docker-compose.e2e.yml` | Replaced hardcoded key with `${CHARON_ENCRYPTION_KEY:?...}` |
| `.github/workflows/e2e-tests.yml` | Added ephemeral key generation step |
| `.env.test.example` | Added prominent documentation |

### Security Notes

- The old key `ucDWy5ScLubd3QwCHhQa2SY7wL2OF48p/c9nZhyW1mA=` exists in git history
- This key should **NEVER** be used in any production environment
- Each CI run now generates a unique ephemeral key

### Testing

```bash
# Verify compose fails without key
unset CHARON_ENCRYPTION_KEY
docker compose -f .docker/compose/docker-compose.playwright.yml config 2>&1
# Expected: "CHARON_ENCRYPTION_KEY is required"

# Verify compose succeeds with key
export CHARON_ENCRYPTION_KEY=$(openssl rand -base64 32)
docker compose -f .docker/compose/docker-compose.playwright.yml config
# Expected: Valid YAML output
```

### References

- **OWASP**: [A02:2021 – Cryptographic Failures](https://owasp.org/Top10/A02_2021-Cryptographic_Failures/)

---

# Playwright Security Test Helpers

**Plan ID**: E2E-SEC-001
**Status**: ✅ COMPLETED
**Priority**: Critical (Blocking 230/707 E2E test failures)
**Created**: 2026-01-25
**Completed**: 2026-01-25
**Scope**: Add security test helpers to prevent ACL deadlock in E2E tests

---

## Completion Notes

**Implementation Summary:**
- Created `tests/utils/security-helpers.ts` with full security state management utilities
- Functions implemented: `getSecurityStatus`, `setSecurityModuleEnabled`, `captureSecurityState`, `restoreSecurityState`, `withSecurityEnabled`, `disableAllSecurityModules`
- Pattern enables guaranteed cleanup via Playwright's `test.afterAll()` fixture

**Documentation:**
- See [Security Test Helpers Guide](../testing/security-helpers.md) for usage examples

---

## Problem Summary

During E2E testing, if ACL is left enabled from a previous test run (e.g., due to test failure), it can create a **deadlock**:
1. ACL blocks API requests → returns 403 Forbidden
2. Global cleanup can't run → API blocked
3. Auth setup fails → tests skip
4. Manual intervention required to reset volumes

**Root Cause Analysis:**
- `security-dashboard.spec.ts` has tests that toggle ACL, WAF, and Rate Limiting
- The tests attempt to "toggle back" but if a test fails mid-execution, cleanup doesn't run
- Playwright's `test.afterAll` with fixtures guarantees cleanup even on failure
- The current tests don't use fixtures for security state management

## Solution Architecture

### API Endpoints (Backend Already Supports)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/security/status` | GET | Returns current state of all security modules |
| `/api/v1/settings` | POST | Toggle settings with `{ key: "security.acl.enabled", value: "true/false" }` |

### Settings Keys

| Key | Values | Description |
|-----|--------|-------------|
| `security.acl.enabled` | `"true"` / `"false"` | Toggle ACL enforcement |
| `security.waf.enabled` | `"true"` / `"false"` | Toggle WAF enforcement |
| `security.rate_limit.enabled` | `"true"` / `"false"` | Toggle Rate Limiting |
| `security.crowdsec.enabled` | `"true"` / `"false"` | Toggle CrowdSec |
| `feature.cerberus.enabled` | `"true"` / `"false"` | Master toggle for all security |

---

## Implementation Plan

### File 1: `tests/utils/security-helpers.ts` (CREATE)

```typescript
/**
 * Security Test Helpers - Safe ACL/WAF/Rate Limit toggle for E2E tests
 *
 * These helpers provide safe mechanisms to temporarily enable security features
 * during tests, with guaranteed cleanup even on test failure.
 *
 * Problem: If ACL is left enabled after a test failure, it blocks all API requests
 * causing subsequent tests to fail with 403 Forbidden (deadlock).
 *
 * Solution: Use Playwright's test.afterAll() with captured original state to
 * guarantee restoration regardless of test outcome.
 *
 * @example
 * ```typescript
 * import { withSecurityEnabled, getSecurityStatus } from './utils/security-helpers';
 *
 * test.describe('ACL Tests', () => {
 *   let cleanup: () => Promise<void>;
 *
 *   test.beforeAll(async ({ request }) => {
 *     cleanup = await withSecurityEnabled(request, { acl: true });
 *   });
 *
 *   test.afterAll(async () => {
 *     await cleanup();
 *   });
 *
 *   test('should enforce ACL', async ({ page }) => {
 *     // ACL is now enabled, test enforcement
 *   });
 * });
 * ```
 */

import { APIRequestContext } from '@playwright/test';

/**
 * Security module status from GET /api/v1/security/status
 */
export interface SecurityStatus {
  cerberus: { enabled: boolean };
  crowdsec: { mode: string; api_url: string; enabled: boolean };
  waf: { mode: string; enabled: boolean };
  rate_limit: { mode: string; enabled: boolean };
  acl: { mode: string; enabled: boolean };
}

/**
 * Options for enabling specific security modules
 */
export interface SecurityModuleOptions {
  /** Enable ACL enforcement */
  acl?: boolean;
  /** Enable WAF protection */
  waf?: boolean;
  /** Enable rate limiting */
  rateLimit?: boolean;
  /** Enable CrowdSec */
  crowdsec?: boolean;
  /** Enable master Cerberus toggle (required for other modules) */
  cerberus?: boolean;
}

/**
 * Captured state for restoration
 */
export interface CapturedSecurityState {
  acl: boolean;
  waf: boolean;
  rateLimit: boolean;
  crowdsec: boolean;
  cerberus: boolean;
}

/**
 * Mapping of module names to their settings keys
 */
const SECURITY_SETTINGS_KEYS: Record<keyof SecurityModuleOptions, string> = {
  acl: 'security.acl.enabled',
  waf: 'security.waf.enabled',
  rateLimit: 'security.rate_limit.enabled',
  crowdsec: 'security.crowdsec.enabled',
  cerberus: 'feature.cerberus.enabled',
};

/**
 * Get current security status from the API
 * @param request - Playwright APIRequestContext (authenticated)
 * @returns Current security status
 */
export async function getSecurityStatus(
  request: APIRequestContext
): Promise<SecurityStatus> {
  const response = await request.get('/api/v1/security/status');

  if (!response.ok()) {
    throw new Error(
      `Failed to get security status: ${response.status()} ${await response.text()}`
    );
  }

  return response.json();
}

/**
 * Set a specific security module's enabled state
 * @param request - Playwright APIRequestContext (authenticated)
 * @param module - Which module to toggle
 * @param enabled - Whether to enable or disable
 */
export async function setSecurityModuleEnabled(
  request: APIRequestContext,
  module: keyof SecurityModuleOptions,
  enabled: boolean
): Promise<void> {
  const key = SECURITY_SETTINGS_KEYS[module];
  const value = enabled ? 'true' : 'false';

  const response = await request.post('/api/v1/settings', {
    data: { key, value },
  });

  if (!response.ok()) {
    throw new Error(
      `Failed to set ${module} to ${enabled}: ${response.status()} ${await response.text()}`
    );
  }

  // Wait a brief moment for Caddy config reload
  await new Promise((resolve) => setTimeout(resolve, 500));
}

/**
 * Capture current security state for later restoration
 * @param request - Playwright APIRequestContext (authenticated)
 * @returns Captured state object
 */
export async function captureSecurityState(
  request: APIRequestContext
): Promise<CapturedSecurityState> {
  const status = await getSecurityStatus(request);

  return {
    acl: status.acl.enabled,
    waf: status.waf.enabled,
    rateLimit: status.rate_limit.enabled,
    crowdsec: status.crowdsec.enabled,
    cerberus: status.cerberus.enabled,
  };
}

/**
 * Restore security state to previously captured values
 * @param request - Playwright APIRequestContext (authenticated)
 * @param state - Previously captured state
 */
export async function restoreSecurityState(
  request: APIRequestContext,
  state: CapturedSecurityState
): Promise<void> {
  const currentStatus = await getSecurityStatus(request);

  // Restore in reverse dependency order (features before master toggle)
  const modules: (keyof SecurityModuleOptions)[] = ['acl', 'waf', 'rateLimit', 'crowdsec', 'cerberus'];

  for (const module of modules) {
    const currentValue = module === 'rateLimit'
      ? currentStatus.rate_limit.enabled
      : module === 'crowdsec'
      ? currentStatus.crowdsec.enabled
      : currentStatus[module].enabled;

    if (currentValue !== state[module]) {
      await setSecurityModuleEnabled(request, module, state[module]);
    }
  }
}

/**
 * Enable security modules temporarily with guaranteed cleanup.
 *
 * Returns a cleanup function that MUST be called in test.afterAll().
 * The cleanup function restores the original state even if tests fail.
 *
 * @param request - Playwright APIRequestContext (authenticated)
 * @param options - Which modules to enable
 * @returns Cleanup function to restore original state
 *
 * @example
 * ```typescript
 * test.describe('ACL Tests', () => {
 *   let cleanup: () => Promise<void>;
 *
 *   test.beforeAll(async ({ request }) => {
 *     cleanup = await withSecurityEnabled(request, { acl: true, cerberus: true });
 *   });
 *
 *   test.afterAll(async () => {
 *     await cleanup();
 *   });
 * });
 * ```
 */
export async function withSecurityEnabled(
  request: APIRequestContext,
  options: SecurityModuleOptions
): Promise<() => Promise<void>> {
  // Capture original state BEFORE making any changes
  const originalState = await captureSecurityState(request);

  // Enable Cerberus first (master toggle) if any security module is requested
  const needsCerberus = options.acl || options.waf || options.rateLimit || options.crowdsec;
  if ((needsCerberus || options.cerberus) && !originalState.cerberus) {
    await setSecurityModuleEnabled(request, 'cerberus', true);
  }

  // Enable requested modules
  if (options.acl) {
    await setSecurityModuleEnabled(request, 'acl', true);
  }
  if (options.waf) {
    await setSecurityModuleEnabled(request, 'waf', true);
  }
  if (options.rateLimit) {
    await setSecurityModuleEnabled(request, 'rateLimit', true);
  }
  if (options.crowdsec) {
    await setSecurityModuleEnabled(request, 'crowdsec', true);
  }

  // Return cleanup function that restores original state
  return async () => {
    try {
      await restoreSecurityState(request, originalState);
    } catch (error) {
      // Log error but don't throw - cleanup should not fail tests
      console.error('Failed to restore security state:', error);
      // Try emergency disable of ACL to prevent deadlock
      try {
        await setSecurityModuleEnabled(request, 'acl', false);
      } catch {
        console.error('Emergency ACL disable also failed - manual intervention may be required');
      }
    }
  };
}

/**
 * Disable all security modules (emergency reset).
 * Use this in global-setup.ts or when tests need a clean slate.
 *
 * @param request - Playwright APIRequestContext (authenticated)
 */
export async function disableAllSecurityModules(
  request: APIRequestContext
): Promise<void> {
  const modules: (keyof SecurityModuleOptions)[] = ['acl', 'waf', 'rateLimit', 'crowdsec'];

  for (const module of modules) {
    try {
      await setSecurityModuleEnabled(request, module, false);
    } catch (error) {
      console.warn(`Failed to disable ${module}:`, error);
    }
  }
}

/**
 * Check if ACL is currently blocking requests.
 * Useful for debugging test failures.
 *
 * @param request - Playwright APIRequestContext
 * @returns True if ACL is enabled and blocking
 */
export async function isAclBlocking(request: APIRequestContext): Promise<boolean> {
  try {
    const status = await getSecurityStatus(request);
    return status.acl.enabled && status.cerberus.enabled;
  } catch {
    // If we can't get status, ACL might be blocking
    return true;
  }
}
```

---

### File 2: `tests/security/security-dashboard.spec.ts` (MODIFY)

**Changes Required:**

1. Import the new security helpers
2. Add `test.beforeAll` to capture initial state
3. Add `test.afterAll` to guarantee cleanup
4. Remove redundant "toggle back" steps in individual tests
5. Group toggle tests in a separate describe block with isolated cleanup

**Exact Changes:**

```typescript
// ADD after existing imports (around line 12)
import {
  withSecurityEnabled,
  captureSecurityState,
  restoreSecurityState,
  CapturedSecurityState,
} from '../utils/security-helpers';
```

```typescript
// REPLACE the entire 'Module Toggle Actions' describe block (lines ~80-180)
// with this safer implementation:

test.describe('Module Toggle Actions', () => {
  // Capture state ONCE for this describe block
  let originalState: CapturedSecurityState;
  let request: APIRequestContext;

  test.beforeAll(async ({ request: req }) => {
    request = req;
    originalState = await captureSecurityState(request);
  });

  test.afterAll(async () => {
    // CRITICAL: Restore original state even if tests fail
    if (originalState) {
      await restoreSecurityState(request, originalState);
    }
  });

  test('should toggle ACL enabled/disabled', async ({ page }) => {
    const toggle = page.getByTestId('toggle-acl');

    const isDisabled = await toggle.isDisabled();
    if (isDisabled) {
      test.info().annotations.push({
        type: 'skip-reason',
        description: 'Toggle is disabled because Cerberus security is not enabled',
      });
      test.skip();
      return;
    }

    await test.step('Toggle ACL state', async () => {
      await page.waitForLoadState('networkidle');
      await toggle.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await toggle.click({ force: true });
      await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
    });

    // NOTE: Do NOT toggle back here - afterAll handles cleanup
  });

  test('should toggle WAF enabled/disabled', async ({ page }) => {
    const toggle = page.getByTestId('toggle-waf');

    const isDisabled = await toggle.isDisabled();
    if (isDisabled) {
      test.info().annotations.push({
        type: 'skip-reason',
        description: 'Toggle is disabled because Cerberus security is not enabled',
      });
      test.skip();
      return;
    }

    await test.step('Toggle WAF state', async () => {
      await page.waitForLoadState('networkidle');
      await toggle.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await toggle.click({ force: true });
      await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
    });

    // NOTE: Do NOT toggle back here - afterAll handles cleanup
  });

  test('should toggle Rate Limiting enabled/disabled', async ({ page }) => {
    const toggle = page.getByTestId('toggle-rate-limit');

    const isDisabled = await toggle.isDisabled();
    if (isDisabled) {
      test.info().annotations.push({
        type: 'skip-reason',
        description: 'Toggle is disabled because Cerberus security is not enabled',
      });
      test.skip();
      return;
    }

    await test.step('Toggle Rate Limit state', async () => {
      await page.waitForLoadState('networkidle');
      await toggle.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await toggle.click({ force: true });
      await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
    });

    // NOTE: Do NOT toggle back here - afterAll handles cleanup
  });

  test('should persist toggle state after page reload', async ({ page }) => {
    const toggle = page.getByTestId('toggle-acl');

    const isDisabled = await toggle.isDisabled();
    if (isDisabled) {
      test.info().annotations.push({
        type: 'skip-reason',
        description: 'Toggle is disabled because Cerberus security is not enabled',
      });
      test.skip();
      return;
    }

    const initialChecked = await toggle.isChecked();

    await test.step('Toggle ACL state', async () => {
      await page.waitForLoadState('networkidle');
      await toggle.scrollIntoViewIfNeeded();
      await page.waitForTimeout(200);
      await toggle.click({ force: true });
      await waitForToast(page, /updated|success|enabled|disabled/i, 10000);
    });

    await test.step('Reload page', async () => {
      await page.reload();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify state persisted', async () => {
      const newChecked = await page.getByTestId('toggle-acl').isChecked();
      expect(newChecked).toBe(!initialChecked);
    });

    // NOTE: Do NOT restore here - afterAll handles cleanup
  });
});
```

---

### File 3: `tests/global-setup.ts` (MODIFY)

**Add Emergency Security Reset:**

```typescript
// ADD to the end of the global setup function, before returning

// Import at top of file
import { request as playwrightRequest } from '@playwright/test';
import { existsSync, readFileSync } from 'fs';
import { STORAGE_STATE } from './constants';

// ADD in globalSetup function, after auth state is created:

async function emergencySecurityReset(baseURL: string) {
  // Only run if auth state exists (meaning we can make authenticated requests)
  if (!existsSync(STORAGE_STATE)) {
    return;
  }

  try {
    const authenticatedContext = await playwrightRequest.newContext({
      baseURL,
      storageState: STORAGE_STATE,
    });

    // Disable ACL to prevent deadlock from previous failed runs
    await authenticatedContext.post('/api/v1/settings', {
      data: { key: 'security.acl.enabled', value: 'false' },
    });

    await authenticatedContext.dispose();
    console.log('✓ Security reset: ACL disabled');
  } catch (error) {
    console.warn('⚠️ Could not reset security state:', error);
  }
}

// Call at end of globalSetup:
await emergencySecurityReset(process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080');
```

---

### File 4: `tests/fixtures/auth-fixtures.ts` (OPTIONAL ENHANCEMENT)

**Add security fixture for tests that need it:**

```typescript
// ADD after existing imports
import {
  withSecurityEnabled,
  SecurityModuleOptions,
  CapturedSecurityState,
  captureSecurityState,
  restoreSecurityState,
} from '../utils/security-helpers';

// ADD to AuthFixtures interface
interface AuthFixtures {
  // ... existing fixtures ...

  /**
   * Security state manager for tests that need to toggle security modules.
   * Automatically captures and restores state.
   */
  securityState: {
    enable: (options: SecurityModuleOptions) => Promise<void>;
    captured: CapturedSecurityState | null;
  };
}

// ADD fixture definition in test.extend
securityState: async ({ request }, use) => {
  let capturedState: CapturedSecurityState | null = null;

  const manager = {
    enable: async (options: SecurityModuleOptions) => {
      capturedState = await captureSecurityState(request);
      const cleanup = await withSecurityEnabled(request, options);
      // Store cleanup for afterAll
      manager._cleanup = cleanup;
    },
    captured: capturedState,
    _cleanup: null as (() => Promise<void>) | null,
  };

  await use(manager);

  // Cleanup after test
  if (manager._cleanup) {
    await manager._cleanup();
  }
},
```

---

## Execution Checklist

### Phase 1: Create Helper Module

- [ ] **1.1** Create `tests/utils/security-helpers.ts` with exact code from File 1 above
- [ ] **1.2** Run TypeScript check: `npx tsc --noEmit`
- [ ] **1.3** Verify helper imports correctly in a test file

### Phase 2: Update Security Dashboard Tests

- [ ] **2.1** Add imports to `tests/security/security-dashboard.spec.ts`
- [ ] **2.2** Replace 'Module Toggle Actions' describe block with new implementation
- [ ] **2.3** Run affected tests: `npx playwright test security-dashboard --project=chromium`
- [ ] **2.4** Verify tests pass AND cleanup happens (check security status after)

### Phase 3: Add Global Safety Net

- [ ] **3.1** Update `tests/global-setup.ts` with emergency security reset
- [ ] **3.2** Run full test suite: `npx playwright test --project=chromium`
- [ ] **3.3** Verify no ACL deadlock occurs across multiple runs

### Phase 4: Validation

- [ ] **4.1** Force a test failure (e.g., add `throw new Error()`) and verify cleanup still runs
- [ ] **4.2** Check security status after failed test: `curl localhost:8080/api/v1/security/status`
- [ ] **4.3** Confirm ACL is disabled after cleanup
- [ ] **4.4** Run full E2E suite 3 times consecutively to verify stability

---

## Benefits

1. **No deadlock**: Tests can safely enable/disable ACL with guaranteed cleanup
2. **Cleanup guaranteed**: `test.afterAll` runs even on failure
3. **Realistic testing**: ACL tests use the same toggle mechanism as users
4. **Isolation**: Other tests unaffected by ACL state
5. **Global safety net**: Even if individual cleanup fails, global setup resets state

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Cleanup fails due to API error | Emergency fallback disables ACL specifically |
| Global setup can't reset state | Auth state file check prevents errors |
| Tests run in parallel | Each describe block has its own captured state |
| API changes break helpers | Settings keys are centralized in one const |

## Files Summary

| File | Action | Priority |
|------|--------|----------|
| `tests/utils/security-helpers.ts` | **CREATE** | Critical |
| `tests/security/security-dashboard.spec.ts` | **MODIFY** | Critical |
| `tests/global-setup.ts` | **MODIFY** | High |
| `tests/fixtures/auth-fixtures.ts` | **MODIFY** (Optional) | Low |
