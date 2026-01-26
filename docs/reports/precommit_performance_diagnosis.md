# Pre-commit Performance Diagnosis Report

**Date:** December 17, 2025
**Issue:** Pre-commit hooks hanging or taking extremely long time to run
**Status:** ROOT CAUSE IDENTIFIED

---

## Executive Summary

The pre-commit hooks are **hanging indefinitely** due to the `go-test-coverage` hook timing out during test execution. This hook runs the full Go test suite with race detection enabled (`go test -race -v -mod=readonly -coverprofile=... ./...`), which is an extremely expensive operation to run on every commit.

**Critical Finding:** The hook times out after 5+ minutes and never completes, causing pre-commit to hang indefinitely.

---

## Pre-commit Configuration Analysis

### All Configured Hooks

Based on `.pre-commit-config.yaml`, the following hooks are configured:

#### Standard Hooks (pre-commit/pre-commit-hooks)

1. **end-of-file-fixer** - Fast (< 1 second)
2. **trailing-whitespace** - Fast (< 1 second)
3. **check-yaml** - Fast (< 1 second)
4. **check-added-large-files** (max 2500 KB) - Fast (< 1 second)

#### Local Hooks - Active (run on every commit)

1. **dockerfile-check** - Fast (only on Dockerfile changes)
2. **go-test-coverage** - **⚠️ CULPRIT - HANGS INDEFINITELY**
3. **go-vet** - Moderate (~1-2 seconds)
4. **check-version-match** - Fast (only on .version changes)
5. **check-lfs-large-files** - Fast (< 1 second)
6. **block-codeql-db-commits** - Fast (< 1 second)
7. **block-data-backups-commit** - Fast (< 1 second)
8. **frontend-type-check** - Slow (~21 seconds)
9. **frontend-lint** - Moderate (~5 seconds)

#### Local Hooks - Manual Stage (only run explicitly)

1. **go-test-race** - Manual only
2. **golangci-lint** - Manual only
3. **hadolint** - Manual only
4. **frontend-test-coverage** - Manual only
5. **security-scan** - Manual only

#### Third-party Hooks - Manual Stage

1. **markdownlint** - Manual only

---

## Root Cause Identification

### PRIMARY CULPRIT: `go-test-coverage` Hook

**Evidence:**

- Hook configuration: `entry: scripts/go-test-coverage.sh`
- Runs on: All `.go` file changes (`files: '\.go$'`)
- Pass filenames: `false` (always runs full test suite)
- Command executed: `go test -race -v -mod=readonly -coverprofile=... ./...`

**Why It Hangs:**

1. **Full Test Suite Execution:** Runs ALL backend tests (155 test files across 20 packages)
2. **Race Detector Enabled:** The `-race` flag adds significant overhead (5-10x slower)
3. **Verbose Output:** The `-v` flag generates extensive output
4. **No Timeout:** The hook has no timeout configured
5. **Test Complexity:** Some tests include `time.Sleep()` calls (36 instances found)
6. **Test Coverage Calculation:** After tests complete, coverage is calculated and filtered

**Measured Performance:**

- Timeout after 300 seconds (5 minutes) - never completes
- Even on successful runs (without timeout), would take 2-5 minutes minimum

### SECONDARY SLOW HOOK: `frontend-type-check`

**Evidence:**

- Measured time: ~21 seconds
- Runs TypeScript type checking on entire frontend
- Resource intensive: 516 MB peak memory usage

**Impact:** While slow, this hook completes successfully. However, it contributes to overall pre-commit slowness.

---

## Environment Analysis

### File Count

- **Total files in workspace:** 59,967 files
- **Git-tracked files:** 776 files
- **Test files (*.go):** 155 files
- **Markdown files:** 1,241 files
- **Backend Go packages:** 20 packages

### Large Untracked Directories (Correctly Excluded)

- `codeql-db/` - 187 MB (4,546 files)
- `data/` - 46 MB
- `.venv/` - 47 MB (2,348 files)
- These are properly excluded via `.gitignore`

### Problematic Files in Workspace (Not Tracked)

The following files exist but are correctly ignored:

- Multiple `*.cover` files in `backend/` (coverage artifacts)
- Multiple `*.sarif` files (CodeQL scan results)
- Multiple `*.db` files (SQLite databases)
- `codeql-*.sarif` files in root

**Status:** These files are properly excluded from git and should not affect pre-commit performance.

---

## Detailed Hook Performance Benchmarks

| Hook | Status | Time | Notes |
|------|--------|------|-------|
| end-of-file-fixer | ✅ Pass | < 1s | Fast |
| trailing-whitespace | ✅ Pass | < 1s | Fast |
| check-yaml | ✅ Pass | < 1s | Fast |
| check-added-large-files | ✅ Pass | < 1s | Fast |
| dockerfile-check | ✅ Pass | < 1s | Conditional |
| **go-test-coverage** | ⛔ **HANGS** | **> 300s** | **NEVER COMPLETES** |
| go-vet | ✅ Pass | 1.16s | Acceptable |
| check-version-match | ✅ Pass | < 1s | Conditional |
| check-lfs-large-files | ✅ Pass | < 1s | Fast |
| block-codeql-db-commits | ✅ Pass | < 1s | Fast |
| block-data-backups-commit | ✅ Pass | < 1s | Fast |
| frontend-type-check | ⚠️ Slow | 20.99s | Works but slow |
| frontend-lint | ✅ Pass | 5.09s | Acceptable |

---

## Recommendations

### CRITICAL: Fix go-test-coverage Hook

**Option 1: Move to Manual Stage (RECOMMENDED)**

```yaml
- id: go-test-coverage
  name: Go Test Coverage
  entry: scripts/go-test-coverage.sh
  language: script
  files: '\.go$'
  pass_filenames: false
  verbose: true
  stages: [manual]  # ⬅️ ADD THIS LINE
```

**Rationale:**

- Running full test suite on every commit is excessive
- Race detection is very slow and better suited for CI
- Coverage checks should be run before PR submission, not every commit
- Developers can run manually when needed: `pre-commit run go-test-coverage --all-files`

**Option 2: Disable the Hook Entirely**

```yaml
# Comment out or remove the entire go-test-coverage hook
```

**Option 3: Run Tests Without Race Detector in Pre-commit**

```yaml
- id: go-test-coverage
  name: Go Test Coverage (Fast)
  entry: bash -c 'cd backend && go test -short -coverprofile=coverage.txt ./...'
  language: system
  files: '\.go$'
  pass_filenames: false
```

- Remove `-race` flag
- Add `-short` flag to skip long-running tests
- This would reduce time from 300s+ to ~30s

### SECONDARY: Optimize frontend-type-check (Optional)

**Option 1: Move to Manual Stage**

```yaml
- id: frontend-type-check
  name: Frontend TypeScript Check
  entry: bash -c 'cd frontend && npm run type-check'
  language: system
  files: '^frontend/.*\.(ts|tsx)$'
  pass_filenames: false
  stages: [manual]  # ⬅️ ADD THIS
```

**Option 2: Add Incremental Type Checking**
Modify `frontend/tsconfig.json` to enable incremental compilation:

```json
{
  "compilerOptions": {
    "incremental": true,
    "tsBuildInfoFile": "./node_modules/.cache/.tsbuildinfo"
  }
}
```

### TERTIARY: General Optimizations

1. **Add Timeout to All Long-Running Hooks**
   - Add timeout wrapper to prevent infinite hangs
   - Example: `entry: timeout 60 scripts/go-test-coverage.sh`

2. **Exclude More Patterns**
   - Add `*.cover` to pre-commit excludes
   - Add `*.sarif` to pre-commit excludes

3. **Consider CI/CD Strategy**
   - Run expensive checks (coverage, linting, type checks) in CI only
   - Keep pre-commit fast (<10 seconds total) for better developer experience
   - Use git hooks for critical checks only (syntax, formatting)

---

## Proposed Configuration Changes

### Immediate Fix (Move Slow Hooks to Manual Stage)

```yaml
# In .pre-commit-config.yaml

repos:
  - repo: local
    hooks:
      # ... other hooks ...

      - id: go-test-coverage
        name: Go Test Coverage (Manual)
        entry: scripts/go-test-coverage.sh
        language: script
        files: '\.go$'
        pass_filenames: false
        verbose: true
        stages: [manual]  # ⬅️ ADD THIS

      # ... other hooks ...

      - id: frontend-type-check
        name: Frontend TypeScript Check (Manual)
        entry: bash -c 'cd frontend && npm run type-check'
        language: system
        files: '^frontend/.*\.(ts|tsx)$'
        pass_filenames: false
        stages: [manual]  # ⬅️ ADD THIS
```

### Alternative: Fast Pre-commit Configuration

```yaml
      - id: go-test-coverage
        name: Go Test Coverage (Fast - No Race)
        entry: bash -c 'cd backend && go test -short -timeout=30s -coverprofile=coverage.txt ./... && go tool cover -func=coverage.txt | tail -n 1'
        language: system
        files: '\.go$'
        pass_filenames: false
```

---

## Impact Assessment

### Current State

- **Total pre-commit time:** INFINITE (hangs)
- **Developer experience:** BROKEN
- **CI/CD reliability:** Blocked

### After Fix (Manual Stage)

- **Total pre-commit time:** ~30 seconds
- **Hooks remaining:**
  - Standard hooks: ~2s
  - go-vet: ~1s
  - frontend-lint: ~5s
  - Security checks: ~1s
  - Other: ~1s
- **Developer experience:** Acceptable

### After Fix (Fast Go Tests)

- **Total pre-commit time:** ~60 seconds
- **Includes fast Go tests:** Yes
- **Developer experience:** Acceptable but slower

---

## Testing Verification

To verify the fix:

```bash
# 1. Apply the configuration change (move hooks to manual stage)

# 2. Test pre-commit without slow hooks
time pre-commit run --all-files

# Expected: Completes in < 30 seconds

# 3. Test slow hooks manually
time pre-commit run go-test-coverage --all-files
time pre-commit run frontend-type-check --all-files

# Expected: These run when explicitly called
```

---

## Conclusion

**Root Cause:** The `go-test-coverage` hook runs the entire Go test suite with race detection on every commit, which takes 5+ minutes and often times out, causing pre-commit to hang indefinitely.

**Solution:** Move the `go-test-coverage` hook to the `manual` stage so it only runs when explicitly invoked, not on every commit. Optionally move `frontend-type-check` to manual stage as well for faster commits.

**Expected Outcome:** Pre-commit will complete in ~30 seconds instead of hanging indefinitely.

**Action Required:** Update `.pre-commit-config.yaml` with the recommended changes and re-test.
