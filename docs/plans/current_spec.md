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

# Future Phase: Playwright Security Test Helpers

**Plan ID**: E2E-SEC-001
**Status**: 📋 TODO (Follow-up Task)
**Priority**: Medium
**Created**: 2026-01-25
**Scope**: Add security test helpers to prevent ACL deadlock in E2E tests

---

## Problem Summary

During E2E testing, if ACL is left enabled from a previous test run (e.g., due to test failure), it can create a **deadlock**:
1. ACL blocks API requests → returns 403 Forbidden
2. Global cleanup can't run → API blocked
3. Auth setup fails → tests skip
4. Manual intervention required to reset volumes

## Solution: Security Test Helpers

Create `tests/utils/security-helpers.ts` with API helpers for:

### 1. Get Security Status

```typescript
export async function getSecurityStatus(request: APIRequestContext): Promise<SecurityStatus>
```

### 2. Toggle ACL via API

```typescript
export async function setAclEnabled(request: APIRequestContext, enabled: boolean): Promise<void>
```

### 3. Ensure ACL Enabled with Cleanup

```typescript
export async function ensureAclEnabled(request: APIRequestContext): Promise<OriginalState>
export async function restoreSecurityState(request: APIRequestContext, originalState: OriginalState): Promise<void>
```

## Usage Pattern

```typescript
test.describe('ACL Enforcement Tests', () => {
  let originalState: OriginalState;

  test.beforeAll(async ({ request }) => {
    originalState = await ensureAclEnabled(request);
  });

  test.afterAll(async ({ request }) => {
    await restoreSecurityState(request, originalState);  // Runs even on failure
  });

  // ACL tests here...
});
```

## Benefits

1. **No deadlock**: Tests can safely enable/disable ACL
2. **Cleanup guaranteed**: `test.afterAll` runs even on failure
3. **Realistic testing**: ACL tests use the same toggle mechanism as users
4. **Isolation**: Other tests unaffected by ACL state

## Files to Create/Modify

| File | Action |
|------|--------|
| `tests/utils/security-helpers.ts` | **Create** - New helper module |
| `tests/security/security-dashboard.spec.ts` | **Modify** - Use new helpers |
| `tests/integration/proxy-acl-integration.spec.ts` | **Modify** - Use new helpers |
