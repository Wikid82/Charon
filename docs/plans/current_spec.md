# Git & Workflow Recovery Plan

**Plan ID**: GIT-2026-001
**Status**: 📋 PENDING
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
