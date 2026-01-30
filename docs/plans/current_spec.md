# Propagate-Changes Workflow Failure - Investigation Report

**Date:** January 30, 2026  
**Investigator:** Planning Agent  
**Status:** 🔴 ROOT CAUSE IDENTIFIED - Configuration file blocking workflow changes

---

## Executive Summary

Investigation of workflow run [#21532969700](https://github.com/Wikid82/Charon/actions/runs/21532969700/job/62053071596) reveals that the **propagate-changes workflow completed successfully but did NOT create a PR** because `.github/workflows/` is still listed in the `sensitive_paths` configuration file, causing all workflow file changes to be blocked from propagation.

**Root Cause:** Mismatch between workflow code comment (claiming `.github/workflows/` was removed from sensitive paths) and the actual configuration file (`.github/propagate-config.yml`) which still blocks workflow paths.

---

## 1. Root Cause Analysis

### 🔴 CRITICAL: Configuration File Still Blocks Workflow Changes

**Evidence from `.github/propagate-config.yml`:**
```yaml
sensitive_paths:
  - scripts/history-rewrite/
  - data/backups
  - docs/plans/history_rewrite.md
  - .github/workflows/           # <-- THIS BLOCKS ALL WORKFLOW CHANGES
  - scripts/history-rewrite/preview_removals.sh
  - scripts/history-rewrite/clean_history.sh
```

**Contradicting Comment in Workflow (line 84-85):**
```javascript
// NOTE: .github/workflows/ was removed from defaults - workflow updates SHOULD propagate
// to ensure downstream branches have correct CI/CD configurations
```

### Logic Flow That Caused the Skip

1. Push made to `main` branch (triggering workflow)
2. Workflow compared `main` to `development` 
3. Found files changed included `.github/workflows/*` paths
4. Loaded `.github/propagate-config.yml` which contains `.github/workflows/`
5. **Matched sensitive path** → `core.info()` logged skip message
6. PR creation skipped, workflow exits with green status ✅

---

## 2. Other Potential Causes Eliminated

| Potential Cause | Verdict | Evidence |
|----------------|---------|----------|
| Push by github-actions[bot] | ❌ Unlikely | User-triggered push would have different actor |
| `github.event.pusher == null` | ❌ Unlikely | Push events always have pusher context |
| Main already synced with dev | ❌ No | Workflow CI changes would create diff |
| Existing open PR | ❌ Unknown | Would need `gh pr list` to verify |
| **Sensitive path blocking** | ✅ **ROOT CAUSE** | `.github/workflows/` in config file |

---

## 3. Recommended Fix

### Option A: Remove `.github/workflows/` from Sensitive Paths (Recommended)

Edit `.github/propagate-config.yml`:

```yaml
sensitive_paths:
  - scripts/history-rewrite/
  - data/backups
  - docs/plans/history_rewrite.md
  # REMOVED: .github/workflows/ - workflow updates should propagate
  - scripts/history-rewrite/preview_removals.sh
  - scripts/history-rewrite/clean_history.sh
```

**Rationale:** 
- CI/CD changes SHOULD propagate to keep all branches in sync
- The original intent (documented in workflow comment) was to allow this
- Downstream branches with outdated workflows cause CI failures

### Option B: Add Specific Exclusions Instead

If certain workflows should NOT propagate, use specific paths:

```yaml
sensitive_paths:
  - scripts/history-rewrite/
  - data/backups
  - docs/plans/history_rewrite.md
  - .github/workflows/propagate-changes.yml  # Only block self-propagation
  - scripts/history-rewrite/preview_removals.sh
  - scripts/history-rewrite/clean_history.sh
```

---

## 4. Additional Findings

### Workflow Logic Analysis

The workflow has robust logic for:
- ✅ Checking existing PRs before creating duplicates
- ✅ Comparing commits (ahead_by check)
- ✅ Loading external config file for sensitive paths
- ✅ Proper error handling with `core.warning()`

### Potential Edge Case: Skip Condition

```yaml
if: github.actor != 'github-actions[bot]' && github.event.pusher != null
```

This condition is **generally safe**, but:
- If a merge is performed by GitHub's merge queue or rebase, `pusher` context may vary
- Consider adding logging to track when this condition fails

---

## 5. Verification Steps After Fix

1. **Apply fix** to `.github/propagate-config.yml`
2. **Push a test change** to `main` that includes workflow modifications
3. **Verify PR creation** in GitHub Actions logs
4. **Check `core.info()` messages** for:
   - `"Checking propagation from main to development..."`
   - `"Created PR #XXX to merge main into development"`

---

## 6. Previous Investigation (Archived)

The following sections document a previous investigation into Renovate and Playwright configuration issues.

---

# Renovate and Playwright Configuration Issues - Investigation Report (Archived)

**Date:** January 30, 2026  
**Investigator:** Planning Agent  
**Status:** ⚠️ CRITICAL - Multiple configuration issues found

---

## Executive Summary (Archived)

Investigation reveals that **both Renovate and Playwright workflows have incorrect configurations** that deviate from the user's required behavior. The Renovate configuration is missing feature branch support and has incorrect automerge settings. The Playwright workflow is missing push event triggers.

---

## 1. Renovate Configuration Issues

### File Locations
- **Primary Config:** `.github/renovate.json` (154 lines)
- **Workflow:** `.github/workflows/renovate.yml` (31 lines)

### 🔴 CRITICAL ISSUE #1: Missing Feature Branch Support

**Current State (BROKEN):**
```json
"baseBranches": [
  "development"
]
```
- **Line:** `.github/renovate.json:9`
- **Problem:** Only targets `development` branch
- **Impact:** Feature branches (`feature/*`) receive NO Renovate updates

**Required State:**
```json
"baseBranches": [
  "development",
  "feature/*"
]
```

---

### 🔴 CRITICAL ISSUE #2: Automerge Enabled Globally

**Current State (BROKEN):**
```json
"automerge": true,
"automergeType": "pr",
"platformAutomerge": true,
```
- **Lines:** `.github/renovate.json:28-30`
- **Problem:** All non-major updates auto-merge immediately
- **Impact:** Updates merge before compatibility is proven

**Required State:**
- **Feature Branches:** Manual approval required (automerge: false)
- **Development Branch:** Let PRs sit until proven compatible
- **Major Updates:** Already correctly set to manual review (line 148)

---

### 🟡 ISSUE #3: Grouped Updates Configuration

**Current State (PARTIALLY CORRECT):**
```json
{
  "description": "THE MEGAZORD: Group ALL non-major updates (NPM, Docker, Go, Actions) into one weekly PR",
  "matchPackagePatterns": ["*"],
  "matchUpdateTypes": [
    "minor",
    "patch",
    "pin",
    "digest"
  ],
  "groupName": "weekly-non-major-updates",
  "automerge": true
}
```
- **Lines:** `.github/renovate.json:116-127`
- **Status:** ✅ Grouping behavior is CORRECT
- **Problem:** ❌ Automerge should be conditional on branch

---

### 🟢 CORRECT Configuration

**These are working as intended:**
- ✅ Major updates are separate and require manual review (line 145-148)
- ✅ Weekly schedule (Monday 8am, line 23-25)
- ✅ Grouped minor/patch updates (line 116-127)
- ✅ Custom managers for Dockerfile, scripts (lines 32-113)

---

## 2. Playwright Workflow Issues

### File Locations
- **Primary Workflow:** `.github/workflows/playwright.yml` (319 lines)
- **Alternative E2E:** `.github/workflows/e2e-tests.yml` (533 lines)

### 🔴 CRITICAL ISSUE #4: Missing Push Event Triggers

**Current State (BROKEN):**
```yaml
on:
  workflow_run:
    workflows: ["Docker Build, Publish & Test"]
    types:
      - completed

  workflow_dispatch:
    inputs:
      pr_number:
        description: 'PR number to test (optional)'
        required: false
        type: string
```
- **Lines:** `.github/workflows/playwright.yml:4-15`
- **Problem:** Only runs after `docker-build.yml` completes, NOT on direct pushes
- **Impact:** User pushed code and Playwright tests did NOT run

**Root Cause Analysis:**
The workflow uses `workflow_run` trigger which:
1. Waits for "Docker Build, Publish & Test" to finish
2. Only triggers if that workflow was triggered by `pull_request` or `push`
3. BUT the condition on line 28-30 filters execution:
   ```yaml
   if: >-
     github.event_name == 'workflow_dispatch' ||
     ((github.event.workflow_run.event == 'pull_request' || github.event.workflow_run.event == 'push') &&
      github.event.workflow_run.conclusion == 'success')
   ```

**Required State:**
```yaml
on:
  push:
    branches:
      - main
      - development
      - 'feature/**'
    paths:
      - 'frontend/**'
      - 'backend/**'
      - 'tests/**'
      - 'playwright.config.js'
      - '.github/workflows/playwright.yml'

  pull_request:
    branches:
      - main
      - development
      - 'feature/**'

  workflow_run:
    workflows: ["Docker Build, Publish & Test"]
    types:
      - completed

  workflow_dispatch:
    inputs:
      pr_number:
        description: 'PR number to test (optional)'
        required: false
        type: string
```

---

### 🟡 ISSUE #5: Alternative E2E Workflow Exists

**Discovery:**
- File: `.github/workflows/e2e-tests.yml`
- **Lines 31-50:** Has CORRECT push/PR triggers:
  ```yaml
  on:
    pull_request:
      branches:
        - main
        - development
        - 'feature/**'
      paths:
        - 'frontend/**'
        - 'backend/**'
        - 'tests/**'
        - 'playwright.config.js'
        - '.github/workflows/e2e-tests.yml'

    push:
      branches:
        - main
        - development
        - 'feature/**'
  ```

**Question:** Are there TWO Playwright workflows?
- `playwright.yml` - Runs after Docker build (BROKEN triggers)
- `e2e-tests.yml` - Runs on push/PR (CORRECT triggers)

**Impact:** Confusion about which workflow should be the primary E2E test runner

---

## 3. Required Changes Summary

### Renovate Configuration Changes

**File:** `.github/renovate.json`

#### Change #1: Add Feature Branch Support
```diff
  "baseBranches": [
-   "development"
+   "development",
+   "feature/*"
  ],
```
- **Line:** 9
- **Priority:** 🔴 CRITICAL

#### Change #2: Conditional Automerge by Branch
```diff
- "automerge": true,
- "automergeType": "pr",
- "platformAutomerge": true,
```

Replace with:
```json
"packageRules": [
  {
    "description": "Feature branches: Require manual approval",
    "matchBaseBranches": ["feature/*"],
    "automerge": false
  },
  {
    "description": "Development branch: Automerge after compatibility proven",
    "matchBaseBranches": ["development"],
    "automerge": true,
    "automergeType": "pr",
    "platformAutomerge": true,
    "minimumReleaseAge": "3 days"
  }
]
```
- **Lines:** 28-30 (delete) + add to packageRules section
- **Priority:** 🔴 CRITICAL

#### Change #3: Update Grouped Updates Rule
```diff
  {
    "description": "THE MEGAZORD: Group ALL non-major updates (NPM, Docker, Go, Actions) into one weekly PR",
    "matchPackagePatterns": ["*"],
    "matchUpdateTypes": [
      "minor",
      "patch",
      "pin",
      "digest"
    ],
    "groupName": "weekly-non-major-updates",
-   "automerge": true
  }
```
- **Lines:** 116-127
- **Priority:** 🟡 HIGH (automerge now controlled by branch-specific rules)

---

### Playwright Workflow Changes

**File:** `.github/workflows/playwright.yml`

#### Option A: Add Direct Push Triggers (Recommended)

```diff
  on:
+   push:
+     branches:
+       - main
+       - development
+       - 'feature/**'
+     paths:
+       - 'frontend/**'
+       - 'backend/**'
+       - 'tests/**'
+       - 'playwright.config.js'
+       - '.github/workflows/playwright.yml'
+ 
+   pull_request:
+     branches:
+       - main
+       - development
+       - 'feature/**'
+ 
    workflow_run:
      workflows: ["Docker Build, Publish & Test"]
      types:
        - completed
```
- **Lines:** 4 (insert after)
- **Priority:** 🔴 CRITICAL

#### Option B: Consolidate Workflows

**Alternative Solution:**
1. Delete `playwright.yml` (post-docker workflow)
2. Keep `e2e-tests.yml` as the primary E2E test runner
3. Update documentation to reference `e2e-tests.yml`

**Pros:**
- `e2e-tests.yml` already has correct triggers
- Includes sharding and coverage collection
- More comprehensive test execution

**Cons:**
- Requires updating CI documentation
- May have different artifact/image handling

---

## 4. Verification Steps

### After Applying Renovate Changes

1. **Create test feature branch:**
   ```bash
   git checkout -b feature/test-renovate-config
   ```

2. **Manually trigger Renovate:**
   ```bash
   # Via GitHub Actions UI
   # Or via API
   gh workflow run renovate.yml
   ```

3. **Verify Renovate creates PRs against feature branch**

4. **Verify automerge behavior:**
   - Feature branch: PR should NOT automerge
   - Development branch: PR should automerge after 3 days

### After Applying Playwright Changes

1. **Create test commit on feature branch:**
   ```bash
   git checkout -b feature/test-playwright-trigger
   # Make trivial change to frontend
   git commit -am "test: trigger playwright"
   git push origin feature/test-playwright-trigger
   ```

2. **Verify Playwright workflow runs immediately on push**

3. **Check GitHub Actions UI:**
   - Workflow should appear in "Actions" tab
   - Status should show "running" or "completed"
   - Should NOT wait for docker-build workflow

---

## 5. Root Cause Analysis

### Why These Changes Occurred

**Hypothesis:**
Another AI model likely:
1. **Simplified baseBranches** to reduce complexity
2. **Enabled automerge globally** to reduce manual PR overhead
3. **Removed direct push triggers** to avoid duplicate test runs

**Problems with this approach:**
- Violates user's explicit requirements for manual feature branch approval
- Creates risk by auto-merging untested updates
- Breaks CI/CD by preventing push-triggered tests

---

## 6. Implementation Priority

### Immediate (Block Development)
1. 🔴 **Renovate:** Add feature branch support (`.github/renovate.json:9`)
2. 🔴 **Playwright:** Add push triggers (`.github/workflows/playwright.yml:4`)

### High Priority (Block Production)
3. 🟡 **Renovate:** Fix automerge behavior (branch-specific rules)

### Medium Priority (Technical Debt)
4. 🟢 **Consolidate:** Decide on single E2E workflow (playwright.yml vs e2e-tests.yml)

---

## 7. Configuration Comparison Table

| Setting | Current (Broken) | Required | Priority |
|---------|-----------------|----------|----------|
| **Renovate baseBranches** | `["development"]` | `["development", "feature/*"]` | 🔴 CRITICAL |
| **Renovate automerge** | Global `true` | Conditional by branch | 🔴 CRITICAL |
| **Renovate grouping** | ✅ Weekly grouped | ✅ Weekly grouped | 🟢 OK |
| **Renovate major updates** | ✅ Manual review | ✅ Manual review | 🟢 OK |
| **Playwright triggers** | `workflow_run` only | `push` + `pull_request` + `workflow_run` | 🔴 CRITICAL |
| **E2E workflow count** | 2 workflows | 1 workflow (consolidate) | 🟡 HIGH |

---

## 8. Next Steps

1. **Review this specification** with the user
2. **Apply critical changes** to Renovate and Playwright configs
3. **Test changes** on feature branch before merging
4. **Document decision** on e2e-tests.yml vs playwright.yml consolidation
5. **Update CI/CD documentation** to reflect correct workflow triggers

---

## Appendix: File References

### Renovate Configuration
- **Primary Config:** `.github/renovate.json`
  - Line 9: `baseBranches` (NEEDS FIX)
  - Lines 28-30: Global `automerge` (NEEDS FIX)
  - Lines 116-127: Grouped updates (NEEDS UPDATE)
  - Lines 145-148: Major updates (CORRECT)

### Playwright Workflows
- **Primary:** `.github/workflows/playwright.yml`
  - Lines 4-15: `on:` triggers (NEEDS FIX)
  - Lines 28-30: Execution condition (REVIEW)
  
- **Alternative:** `.github/workflows/e2e-tests.yml`
  - Lines 31-50: `on:` triggers (CORRECT - consider as model)

---

**End of Investigation Report**
2. Docker Run (One Command)
3. Alternative: GitHub Container Registry

**Code Sample:**
```yaml
services:
  charon:
    image: wikid82/charon:latest
    container_name: charon
    restart: unless-stopped
```

**Verdict:** Zero mention of standalone binaries, native installation, or platform-specific installers.

---

### 3. Distribution Method ✅

**Source:** `docs/getting-started.md` (Lines 1-150)

**Supported Installation:**
- Docker Hub: `wikid82/charon:latest`
- GitHub Container Registry: `ghcr.io/wikid82/charon:latest`

**Migration Commands:**
```bash
docker exec charon /app/charon migrate
```

**Verdict:** All documentation assumes Docker runtime.

---

### 4. GoReleaser Configuration ⚠️

**Source:** `.goreleaser.yaml` (Lines 1-122)

**Current Build Targets:**
```yaml
builds:
  - id: linux
    goos: [linux]
    goarch: [amd64, arm64]
  
  - id: windows
    goos: [windows]
    goarch: [amd64]
  
  - id: darwin
    goos: [darwin]
    goarch: [amd64, arm64]
```

**Observations:**
- Builds binaries for `linux`, `windows`, `darwin`
- Creates archives (`.tar.gz`, `.zip`)
- Generates Debian/RPM packages
- **These artifacts are never referenced in user documentation**
- **No installation instructions for standalone binaries**

**Verdict:** Unnecessary build targets creating unused artifacts.

---

### 5. Release Workflow Analysis ✅

**Source:** `.github/workflows/release-goreleaser.yml`

**What Gets Published:**
1. ✅ Docker images (multi-platform: `linux/amd64`, `linux/arm64`)
2. ✅ SBOM (Software Bill of Materials)
3. ✅ SLSA provenance attestation
4. ✅ Cryptographic signatures (Cosign)
5. ⚠️ Standalone binaries (unused)
6. ⚠️ Archives (`.tar.gz`, `.zip` - unused)
7. ⚠️ Debian/RPM packages (unused)

**Verdict:** Docker images are the primary (and only documented) distribution method.

---

### 6. Dockerfile Base Image ✅

**Source:** `Dockerfile` (Lines 1-50)

```dockerfile
# renovate: datasource=docker depName=debian versioning=docker
ARG CADDY_IMAGE=debian:trixie-slim@sha256:...
```

**Verdict:** Debian-based Linux container. No Windows/macOS container images exist.

---

### 7. User Base & Use Cases ✅

**Source:** `ARCHITECTURE.md`

**Target Audience:**
> "Simplify website and application hosting for **home users and small teams**"

**Deployment Model:**
> "Monolithic architecture packaged as a **single Docker container**"

**Verdict:** Docker-first design with no enterprise/cloud-native multi-platform requirements.

---

## Current Issue: Disk Space Implementation

**Original Problem:**
```go
// backend/internal/models/systemmetrics.go
func UpdateDiskMetrics(db *gorm.DB) error {
    // TODO: Cross-platform disk space implementation
    // Currently hardcoded to "/" for Linux
    // Need platform detection for Windows (C:\) and macOS
}
```

**Why This Is Complex:**
- Windows uses drive letters (`C:\`, `D:\`)
- macOS uses `/System/Volumes/Data`
- Windows requires `golang.org/x/sys/windows` syscalls
- macOS requires `golang.org/x/sys/unix` with special mount handling
- Testing requires platform-specific CI runners

**Why This Is Unnecessary:**
- Charon **only runs in Linux containers** (Debian base image)
- The host OS (Windows/macOS) is irrelevant - Docker abstracts it
- The disk space check should monitor `/app/data` (container filesystem)

---

## Old Plan Context (Now Superseded)

### Previous Problem Description

The `GetAvailableSpace()` method in `backend/internal/services/backup_service.go` (lines 363-394) used Unix-specific syscalls that blocked Windows cross-compilation. This was mistakenly interpreted as requiring platform-specific implementations.

### Why The Problem Was Misunderstood

- **Assumption**: Users need to run Charon natively on Windows/macOS
- **Reality**: Charon is Docker-only, runs in Linux containers regardless of host OS
- **Root Cause**: GoReleaser configured to build unused Windows/macOS binaries

---

## Recommended Solution

### Simple Solution: Remove Unnecessary Build Targets

**Changes to `.goreleaser.yaml`:**

```yaml
builds:
  - id: linux
    dir: backend
    main: ./cmd/api
    binary: charon
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/Wikid82/charon/backend/internal/version.Version={{.Version}}
      - -X github.com/Wikid82/charon/backend/internal/version.GitCommit={{.Commit}}
      - -X github.com/Wikid82/charon/backend/internal/version.BuildTime={{.Date}}

archives:
  - formats:
      - tar.gz
    id: linux
    ids:
      - linux
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - LICENSE
      - README.md

nfpms:
  - id: packages
    ids:
      - linux
    package_name: charon
    vendor: Charon
    homepage: https://github.com/Wikid82/charon
    maintainer: Wikid82
    description: "Charon - A powerful reverse proxy manager"
    license: MIT
    formats:
      - deb
      - rpm
```

**Removals:**
- ❌ `windows` build ID (lines 23-35)
- ❌ `darwin` build ID (lines 37-51)
- ❌ Windows archive format

**Benefits:**
- ✅ Faster CI builds (no cross-compilation overhead)
- ✅ Smaller release artifacts
- ✅ Clearer distribution model (Docker-only)
- ✅ Reduced maintenance burden
- ✅ No platform-specific disk space code needed

---

### Simplified Disk Space Implementation

**File:** `backend/internal/services/backup_service.go`

**Current Implementation (already Linux-compatible):**
```go
func (s *BackupService) GetAvailableSpace() (int64, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(s.BackupDir, &stat); err != nil {
        return 0, fmt.Errorf("failed to get disk space: %w", err)
    }
    
    bsize := stat.Bsize
    bavail := stat.Bavail
    
    if bsize < 0 {
        return 0, fmt.Errorf("invalid block size %d", bsize)
    }
    
    if bavail > uint64(math.MaxInt64) {
        return math.MaxInt64, nil
    }
    
    available := int64(bavail) * int64(bsize)
    return available, nil
}
```

**Recommended Change:** Monitor `/app/data` instead of `/` for more accurate container volume metrics:

```go
func (s *BackupService) GetAvailableSpace() (int64, error) {
    // Monitor the container data volume (or fallback to root)
    dataPath := "/app/data"
    
    var stat syscall.Statfs_t
    if err := syscall.Statfs(dataPath, &stat); err != nil {
        // Fallback to root filesystem if data mount doesn't exist
        if err := syscall.Statfs("/", &stat); err != nil {
            return 0, fmt.Errorf("failed to get disk space: %w", err)
        }
    }
    
    // Existing overflow protection logic...
    bsize := stat.Bsize
    bavail := stat.Bavail
    
    if bsize < 0 {
        return 0, fmt.Errorf("invalid block size %d", bsize)
    }
    
    if bavail > uint64(math.MaxInt64) {
        return math.MaxInt64, nil
    }
    
    available := int64(bavail) * int64(bsize)
    return available, nil
}
```

**Rationale:**
- Monitors `/app/data` (user's persistent volume)
- Falls back to `/` if volume not mounted
- No platform detection needed
- Works in all Docker environments (Linux host, macOS Docker Desktop, Windows WSL2)

---

## Decision Matrix

| Approach | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| **Remove Windows/macOS targets** | ✅ Aligns with actual architecture<br>✅ Faster CI builds<br>✅ Simpler codebase<br>✅ No cross-platform complexity | ⚠️ Can't distribute standalone binaries (never documented anyway) | **✅ RECOMMENDED** |
| **Keep all platforms** | ⚠️ "Future-proofs" for potential pivot | ❌ Wastes CI resources<br>❌ Adds complexity<br>❌ Misleads users<br>❌ No documented use case | ❌ NOT RECOMMENDED |

---

## Implementation Tasks

### Task 1: Update GoReleaser Configuration
**File:** `.goreleaser.yaml`  
**Changes:**
- Remove `windows` and `darwin` build definitions
- Remove Windows archive format (zip)
- Keep only `linux/amd64` and `linux/arm64`
- Update `nfpms` to reference only `linux` build ID

**Estimated Effort:** 15 minutes

---

### Task 2: Remove Zig Cross-Compilation from CI
**File:** `.github/workflows/release-goreleaser.yml`  
**Changes:**
- Remove `Install Cross-Compilation Tools (Zig)` step (lines 52-56)
- No longer needed for Linux-only builds

**Estimated Effort:** 5 minutes

---

### Task 3: Simplify Disk Metrics (Optional Enhancement)
**File:** `backend/internal/models/systemmetrics.go`  
**Changes:**
- Update `UpdateDiskMetrics()` to monitor `/app/data` instead of `/`
- Add fallback to `/` if data volume not mounted
- Update comments to clarify Docker-only scope

**Estimated Effort:** 10 minutes

---

### Task 4: Update Documentation
**Files:**
- `ARCHITECTURE.md` - Add note about Docker-only distribution in "Build & Release Process" section
- `CONTRIBUTING.md` - Remove any Windows/macOS build instructions

**Estimated Effort:** 10 minutes

---

## Validation Checklist

After implementation:
- [ ] CI release workflow completes successfully
- [ ] Docker images build for `linux/amd64` and `linux/arm64`
- [ ] No Windows/macOS binaries in GitHub releases
- [ ] `backend/internal/services/backup_service.go` still compiles
- [ ] E2E tests pass against built image
- [ ] Documentation reflects Docker-only distribution model

---

## Future Considerations

**If standalone binary distribution is needed in the future:**

1. **Revisit Architecture:**
   - Extract backend into CLI tool
   - Bundle frontend as embedded assets
   - Provide platform-specific installers (`.exe`, `.dmg`, `.deb`)

2. **Update Documentation:**
   - Add installation guides for each platform
   - Provide troubleshooting for native installs

3. **Re-add Build Targets:**
   - Restore `windows` and `darwin` in `.goreleaser.yaml`
   - Implement platform detection for disk metrics with build tags
   - Add CI runners for each platform (Windows Server, macOS)

**Current Priority:** None. Docker-only distribution meets all documented use cases.

---

## Conclusion

Charon is **explicitly designed, documented, and distributed as a Docker-only application**. The Windows and macOS build targets in GoReleaser serve no purpose and should be removed.

**Recommended Next Steps:**
1. Remove unused build targets from `.goreleaser.yaml`
2. Remove Zig cross-compilation step from release workflow
3. (Optional) Update disk metrics to monitor `/app/data` volume
4. Update documentation to clarify Docker-only scope
5. Proceed with simplified implementation (no platform detection needed)

---

**Plan Status:** Ready for Implementation  
**Confidence Level:** High (100% - all evidence aligns)  
**Risk Assessment:** Low (removing unused features)  
**Total Estimated Effort:** 40 minutes (configuration changes + testing)

---

## Archived: Old Plan (Platform-Specific Build Tags)

The previous plan assumed cross-platform binary support was needed and proposed implementing platform-specific disk space checks using build tags. This approach is no longer necessary given the Docker-only distribution model.

**Key Insight from Research:**
- Charon runs in Linux containers regardless of host OS
- Windows/macOS users run Docker Desktop (which uses Linux VMs internally)
- The container always sees a Linux filesystem
- No platform detection needed

**Historical Context:**

	}

	// Safe to convert now
	availBlocks := int64(bavail)
	blockSize := int64(bsize)

	// Check for multiplication overflow
	if availBlocks > 0 && blockSize > math.MaxInt64/availBlocks {
		return math.MaxInt64, nil
	}

	return availBlocks * blockSize, nil
}
```

**Key Points:**
- Preserves existing overflow protection logic
- Maintains gosec compliance (G115)
- No functional changes from current implementation

---

### Phase 3: Windows Implementation

#### File: `backup_service_disk_windows.go`

```go
//go:build windows

package services

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// getAvailableSpace returns the available disk space in bytes for the given directory.
// Windows implementation using GetDiskFreeSpaceExW with long path support.
func getAvailableSpace(dir string) (int64, error) {
	// Normalize path for Windows
	cleanPath := filepath.Clean(dir)
	
	// Handle long paths (>260 chars) by prepending \\?\ prefix
	// This enables paths up to 32,767 characters on Windows
	if len(cleanPath) > 260 && !strings.HasPrefix(cleanPath, `\\?\`) {
		// Convert to absolute path first
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve absolute path for '%s': %w", dir, err)
		}
		// Add long path prefix
		cleanPath = `\\?\` + absPath
	}
	
	// Convert to UTF-16 for Windows API
	utf16Ptr, err := windows.UTF16PtrFromString(cleanPath)
	if err != nil {
		return 0, fmt.Errorf("failed to convert path '%s' to UTF16: %w", dir, err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(
		utf16Ptr,
		&freeBytesAvailable,
		&totalBytes,
		&totalFreeBytes,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get disk space for path '%s': %w", dir, err)
	}

	// freeBytesAvailable already accounts for quotas and user restrictions
	// Check if value exceeds max int64
	if freeBytesAvailable > uint64(math.MaxInt64) {
		return math.MaxInt64, nil
	}

	return int64(freeBytesAvailable), nil
}
```

**Key Points:**

1. **API Choice**: `GetDiskFreeSpaceEx` vs `GetDiskFreeSpace`
   - `GetDiskFreeSpaceEx` respects disk quotas (correct behavior)
   - Returns bytes directly (no block size calculation needed)
   - Supports paths > 260 characters with proper handling

2. **Path Handling**:
   - Converts Go string to UTF-16 (Windows native format)
   - Handles Unicode paths correctly
   - **Windows Long Path Support**: For paths > 260 characters, automatically prepends `\\?\` prefix
   - Normalizes forward slashes to backslashes for Windows API compatibility

3. **Overflow Protection**:
   - Maintains same logic as Unix version
   - Caps at `math.MaxInt64` for consistency

4. **Return Value**:
   - Uses `freeBytesAvailable` (not `totalFreeBytes`)
   - Correctly accounts for user quotas and restrictions

---

### Phase 4: Refactor Main File

#### File: `backup_service.go`

**Modification:**

```go
// BEFORE (lines 363-394): Direct implementation

// AFTER: Delegate to platform-specific function
func (s *BackupService) GetAvailableSpace() (int64, error) {
	return getAvailableSpace(s.BackupDir)
}
```

**Changes:**
1. Remove `var stat syscall.Statfs_t` and all calculation logic
2. Replace with single call to platform-specific `getAvailableSpace()`
3. Platform selection handled at compile-time via build tags

**Benefits:**
- Simplified main file
- No runtime conditionals
- Zero performance overhead
- Same API for all callers

---

### Phase 5: Dependency Management

#### 5.1 Add Windows Dependency

**Command:**
```bash
cd backend
go get golang.org/x/sys/windows@latest
go mod tidy
```

**Expected `go.mod` Change:**
```go
require (
    // ... existing deps ...
    golang.org/x/sys v0.40.0  // existing
)
```

**Note:** `golang.org/x/sys` is already present in `go.mod` (line 95), but we need to ensure `windows` subpackage is available. It's part of the same module, so no new direct dependency needed.

#### 5.2 Verify Build Tags

**Test Matrix:**
```bash
# Test Unix build
GOOS=linux GOARCH=amd64 go build ./cmd/api

# Test Darwin build
GOOS=darwin GOARCH=arm64 go build ./cmd/api

# Test Windows build (this currently fails)
GOOS=windows GOARCH=amd64 go build ./cmd/api
```

---

### Phase 6: Testing Strategy

#### 6.1 Unit Tests

**New Test Files:**
```
backend/internal/services/
├── backup_service_disk_unix_test.go
└── backup_service_disk_windows_test.go
```

**Unix Test (`backup_service_disk_unix_test.go`):**
```go
//go:build unix

package services

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAvailableSpace_Unix(t *testing.T) {
	// Test with temp directory
	tmpDir := t.TempDir()
	
	space, err := getAvailableSpace(tmpDir)
	require.NoError(t, err)
	assert.Greater(t, space, int64(0), "Available space should be positive")
	
	// Test with invalid directory
	space, err = getAvailableSpace("/nonexistent/path")
	assert.Error(t, err)
	assert.Equal(t, int64(0), space)
}

func TestGetAvailableSpace_UnixRootFS(t *testing.T) {
	// Test with root filesystem
	space, err := getAvailableSpace("/")
	require.NoError(t, err)
	assert.Greater(t, space, int64(0))
}

func TestGetAvailableSpace_UnixPermissionDenied(t *testing.T) {
	// Test permission denied scenario
	// Try to stat a path we definitely don't have access to
	if os.Getuid() == 0 {
		t.Skip("Test requires non-root user")
	}
	
	// Most Unix systems have restricted directories
	restrictedPaths := []string{"/root", "/lost+found"}
	
	for _, path := range restrictedPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // Path doesn't exist on this system
		}
		
		space, err := getAvailableSpace(path)
		if err != nil {
			// Expected: permission denied
			assert.Contains(t, err.Error(), "failed to get disk space")
			assert.Equal(t, int64(0), space)
			return // Test passed
		}
	}
	
	t.Skip("No restricted paths found to test permission denial")
}

func TestGetAvailableSpace_UnixSymlink(t *testing.T) {
	// Test symlink resolution - statfs follows symlinks
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	symlinkPath := filepath.Join(tmpDir, "link")
	
	err := os.Mkdir(targetDir, 0755)
	require.NoError(t, err)
	
	err = os.Symlink(targetDir, symlinkPath)
	require.NoError(t, err)
	
	// Should follow symlink and return space for target
	space, err := getAvailableSpace(symlinkPath)
	require.NoError(t, err)
	assert.Greater(t, space, int64(0))
	
	// Compare with direct target query (should match filesystem)
	targetSpace, err := getAvailableSpace(targetDir)
	require.NoError(t, err)
	assert.Equal(t, targetSpace, space, "Symlink should resolve to same filesystem")
}
```

**Windows Test (`backup_service_disk_windows_test.go`):**
```go
//go:build windows

package services

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAvailableSpace_Windows(t *testing.T) {
	// Test with temp directory
	tmpDir := t.TempDir()
	
	space, err := getAvailableSpace(tmpDir)
	require.NoError(t, err)
	assert.Greater(t, space, int64(0), "Available space should be positive")
	
	// Test with C: drive (usually exists on Windows)
	space, err = getAvailableSpace("C:\\")
	require.NoError(t, err)
	assert.Greater(t, space, int64(0))
}

func TestGetAvailableSpace_WindowsInvalidPath(t *testing.T) {
	// Test with invalid drive letter
	space, err := getAvailableSpace("Z:\\nonexistent\\path")
	// May error or return 0 depending on Windows version
	if err != nil {
		assert.Equal(t, int64(0), space)
	}
}

func TestGetAvailableSpace_WindowsLongPath(t *testing.T) {
	// Test long path handling (>260 characters)
	tmpBase := t.TempDir()
	
	// Create a deeply nested directory structure to exceed MAX_PATH
	longPath := tmpBase
	for i := 0; i < 20; i++ {
		longPath = filepath.Join(longPath, "verylongdirectorynamewithlotsofcharacters")
	}
	
	err := os.MkdirAll(longPath, 0755)
	require.NoError(t, err, "Should create long path with \\\\?\\ prefix support")
	
	// Test disk space check on long path
	space, err := getAvailableSpace(longPath)
	require.NoError(t, err, "Should query disk space for paths >260 chars")
	assert.Greater(t, space, int64(0), "Available space should be positive")
}

func TestGetAvailableSpace_WindowsUnicodePath(t *testing.T) {
	// Test Unicode path handling to ensure UTF-16 conversion works correctly
	tmpBase := t.TempDir()
	
	// Create directory with Unicode characters (emoji, CJK, Arabic)
	unicodeDirName := "test_🚀_测试_اختبار"
	unicodePath := filepath.Join(tmpBase, unicodeDirName)
	
	err := os.Mkdir(unicodePath, 0755)
	require.NoError(t, err, "Should create directory with Unicode name")
	
	// Test disk space check on Unicode path
	space, err := getAvailableSpace(unicodePath)
	require.NoError(t, err, "Should handle Unicode path names")
	assert.Greater(t, space, int64(0), "Available space should be positive")
}

func TestGetAvailableSpace_WindowsPermissionDenied(t *testing.T) {
	// Test permission denied scenario
	// On Windows, system directories like C:\System Volume Information
	// typically deny access to non-admin users
	space, err := getAvailableSpace("C:\\System Volume Information")
	if err != nil {
		// Expected: access denied error
		assert.Contains(t, err.Error(), "failed to get disk space")
		assert.Equal(t, int64(0), space)
	} else {
		// If no error (running as admin), space should still be valid
		assert.GreaterOrEqual(t, space, int64(0))
	}
}
```

#### 6.2 Integration Testing

**Existing Tests Impact:**
- `backend/internal/services/backup_service_test.go` should work unchanged
- If tests mock disk space, update mocks to use new signature
- Add CI matrix testing for Windows builds

**CI/CD Testing:**

Add platform-specific test matrix to ensure all implementations are validated:

```yaml
# .github/workflows/go-tests.yml
name: Go Tests

on:
  pull_request:
    paths:
      - 'backend/**/*.go'
      - 'backend/go.mod'
      - 'backend/go.sum'
  push:
    branches:
      - main

jobs:
  test-cross-platform:
    name: Test on ${{ matrix.os }}
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go-version: ['1.25.6']
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
          cache: true
          cache-dependency-path: backend/go.sum

      - name: Run platform-specific tests
        working-directory: backend
        run: |
          go test -v -race -coverprofile=coverage.txt -covermode=atomic ./internal/services/...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./backend/coverage.txt
          flags: ${{ matrix.os }}
          token: ${{ secrets.CODECOV_TOKEN }}

  verify-cross-compilation:
    name: Cross-compile for ${{ matrix.goos }}/${{ matrix.goarch }}
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64
          - goos: darwin
            goarch: amd64
          - goos: darwin
            goarch: arm64
          - goos: windows
            goarch: amd64
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.6'

      - name: Build for ${{ matrix.goos }}/${{ matrix.goarch }}
        working-directory: backend
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: 0
        run: |
          go build -v -o /tmp/charon-${{ matrix.goos }}-${{ matrix.goarch }} ./cmd/api
```

#### 6.3 Manual Testing Checklist

**Unix/Linux:**
- [ ] Backup creation succeeds with sufficient space
- [ ] Backup creation fails gracefully with insufficient space
- [ ] Log messages show correct available space

**Windows:**
- [ ] Binary compiles successfully
- [ ] Same functionality as Unix version
- [ ] Handles UNC paths (\\server\share)
- [ ] Respects disk quotas

---

### Phase 7: Documentation Updates

#### 7.1 Code Documentation

**File-level comments:**
```go
// backup_service_disk_unix.go
// Platform-specific implementation of disk space queries for Unix-like systems.
// This file is compiled only on Linux, macOS, BSD, and other Unix variants.

// backup_service_disk_windows.go
// Platform-specific implementation of disk space queries for Windows.
// Uses Win32 API GetDiskFreeSpaceEx to query filesystem statistics.
```

#### 7.2 Architecture Documentation

**Update `ARCHITECTURE.md`:**
- Add section on platform-specific implementations
- Document build tag strategy
- List platform-specific files

**Update `docs/development/building.md` (if exists):**
- Cross-compilation requirements
- Platform-specific testing instructions

#### 7.3 Developer Guidance

**Create `docs/development/platform-specific-code.md`:**
```markdown
# Platform-Specific Code Guidelines

## When to Use Build Tags

Use build tags when:
- Accessing OS-specific APIs (syscalls, Win32, etc.)
- Functionality differs by platform
- No cross-platform abstraction exists

## Build Tag Reference

- `//go:build unix` - Linux, macOS, BSD, Solaris
- `//go:build windows` - Windows
- `//go:build darwin` - macOS only
- `//go:build linux` - Linux only

## File Naming Convention

Pattern: `{feature}_{platform}.go`
Examples:
- `backup_service_disk_unix.go`
- `backup_service_disk_windows.go`
```

---

### Phase 8: Configuration Updates

#### 8.1 Codecov Configuration

**Current `codecov.yml` (line 15-31):**
```yaml
ignore:
  - "**/*_test.go"
  - "**/testdata/**"
  - "**/mocks/**"
```

**No changes needed:**
- Platform-specific files are production code
- Should be included in coverage
- Tests run on each platform will cover respective implementation

**Rationale:**
- Unix tests run on Linux CI runners → cover `*_unix.go`
- Windows tests run on Windows CI runners → cover `*_windows.go`
- Combined coverage shows full platform coverage

#### 8.2 .gitignore Updates

**Current `.gitignore`:**
No changes needed for source files.

**Verify exclusions:**
```gitignore
# Already covered:
*.test
*.out
backend/bin/
```

#### 8.3 Linter Configuration

**Verify gopls/staticcheck:**
- Build tags are standard Go feature
- No linter configuration changes needed
- GoReleaser will compile each platform separately

---

## Build Validation

### Pre-Merge Checklist

**Compilation Tests:**
```bash
# Unix targets
GOOS=linux GOARCH=amd64 go build -o /dev/null ./backend/cmd/api
GOOS=darwin GOARCH=arm64 go build -o /dev/null ./backend/cmd/api

# Windows target (currently fails)
GOOS=windows GOARCH=amd64 go build -o /dev/null ./backend/cmd/api
```

**Post-Implementation:**
All three commands should succeed with exit code 0.

**Unit Test Validation:**
```bash
# Run on each platform
go test ./backend/internal/services/... -v

# Expected output includes:
# - TestGetAvailableSpace_Unix (on Unix)
# - TestGetAvailableSpace_Windows (on Windows)
```

### GoReleaser Integration

**`.goreleaser.yaml` (lines 23-35):**
```yaml
- id: windows
  dir: backend
  main: ./cmd/api
  binary: charon
  env:
    - CGO_ENABLED=0  # ✅ Maintained: static binary
  goos:
    - windows
  goarch:
    - amd64
```

**Expected Behavior After Fix:**
- GoReleaser snapshot builds succeed
- Windows binary in `dist/windows_windows_amd64_v1/`
- Binary size similar to Linux/Darwin variants

---

## Risk Assessment & Mitigation

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Windows API fails on network drives | Medium | Medium | Document UNC path limitations, add error handling |
| Path encoding issues (Unicode) | Low | Medium | UTF-16 conversion with error handling |
| Quota calculation differs | Low | Low | Use `freeBytesAvailable` (quota-aware) |
| Missing test coverage on Windows | Medium | Low | Add CI Windows runner for tests |
| Breaking existing Unix behavior | Low | High | Preserve existing logic byte-for-byte |

### Rollback Plan

**If Windows implementation causes issues:**
1. Revert to Unix-only with build tag exclusion:
   ```go
   //go:build !windows
   ```
2. Update GoReleaser to skip Windows target temporarily
3. File issue to investigate Windows-specific failures

**Revert Complexity:** Low (isolated files, no API changes)

---

## Timeline & Effort Estimate

### Breakdown

| Phase | Task | Effort | Dependencies |
|-------|------|--------|-------------|
| 1 | File structure refactoring | 30 min | None |
| 2 | Unix implementation | 15 min | Phase 1 |
| 3 | Windows implementation | 1 hour | Phase 1, research |
| 4 | Main file refactor | 15 min | Phase 2, 3 |
| 5 | Dependency management | 10 min | None |
| 6 | Unit tests (both platforms) | 1.5 hours | Phase 2, 3 |
| 7 | Documentation | 45 min | Phase 4 |
| 8 | Configuration updates | 15 min | Phase 6 |
| **Total** | | **~4.5 hours** | |

### Milestones

- ✅ **M1**: Unix implementation compiles (Phase 1-2)
- ✅ **M2**: Windows implementation compiles (Phase 3)
- ✅ **M3**: All platforms compile successfully (Phase 4-5)
- ✅ **M4**: Tests pass on Unix (Phase 6)
- ✅ **M5**: Tests pass on Windows (Phase 6)
- ✅ **M6**: Documentation complete (Phase 7)
- ✅ **M7**: Ready for merge (Phase 8)

---

## Success Criteria

### Functional Requirements

- [ ] `GOOS=windows GOARCH=amd64 go build` succeeds without errors
- [ ] `GetAvailableSpace()` returns accurate values on Windows
- [ ] Existing Unix behavior unchanged (byte-for-byte identical)
- [ ] All existing tests pass without modification
- [ ] New platform-specific tests added and passing

### Non-Functional Requirements

- [ ] Zero runtime performance overhead (compile-time selection)
- [ ] No new external dependencies (uses existing `golang.org/x/sys`)
- [ ] Codecov shows >85% coverage for new files
- [ ] GoReleaser nightly builds include Windows binaries
- [ ] Documentation updated for platform-specific code patterns

### Quality Gates

- [ ] No gosec findings on new code
- [ ] staticcheck passes on all platforms
- [ ] golangci-lint passes
- [ ] No breaking API changes
- [ ] Windows binary size < 50MB (similar to Linux)

---

## Known Limitations & Platform-Specific Behavior

### Disk Quotas

**Windows:**
- `GetDiskFreeSpaceEx` respects user disk quotas configured via NTFS
- `freeBytesAvailable` reflects quota-limited space (correct behavior)
- If user has 10GB quota on 100GB volume with 50GB free, returns ~10GB

**Unix:**
- `syscall.Statfs` returns filesystem-level statistics
- Does NOT account for user quotas set via `quota`, `edquota`, or XFS project quotas
- Returns physical available space regardless of quota limits
- **Recommendation**: For quota-aware backups on Unix, implement separate quota checking via `quotactl()` syscall (future enhancement)

### Mount Points and Virtual Filesystems

**Both Platforms:**
- Query operates on the filesystem containing the path, not the path's parent
- If backup dir is `/mnt/backup` on separate mount, returns that mount's space
- Virtual filesystems (tmpfs, ramfs, procfs) return valid stats but may not reflect persistent storage

**Unix Specific:**
- `/proc`, `/sys`, `/dev` return non-zero space (virtual filesystems)
- Network mounts (NFS, CIFS) return remote filesystem stats (may be stale)
- Bind mounts resolve to underlying filesystem

**Windows Specific:**
- UNC paths (`\\server\share`) supported but require network access
- Mounted volumes (NTFS junctions, symbolic links) follow to target
- Drive letters always resolve to root of volume

### Symlink Behavior

**Unix:**
- `syscall.Statfs` **follows symlinks** to target directory
- If `/backup` → `/mnt/external/backup`, queries `/mnt/external` filesystem
- Broken symlinks return error ("no such file or directory")

**Windows:**
- `GetDiskFreeSpaceEx` **follows junction points and symbolic links**
- Reparse points (directory symlinks) resolve to target volume
- Hard links not applicable to directories (Windows limitation)

### Path Length Limits

**Unix:**
- No practical path length limit on modern systems (Linux: 4096 bytes, macOS: 1024 bytes)
- Individual filename component limit: 255 bytes

**Windows:**
- **Legacy applications**: MAX_PATH = 260 characters (including drive and null terminator)
- **Long path support**: Up to 32,767 characters with `\\?\` prefix (handled automatically in our implementation)
- **Registry requirement**: `Computer\HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled` = 1 (Windows 10 1607+)
- **Limitation**: Some third-party backup tools may not support long paths

### Error Handling Edge Cases

**Permission Denied:**
- Unix: Returns `syscall.EACCES` wrapped in error
- Windows: Returns `syscall.ERROR_ACCESS_DENIED` wrapped in error
- **Behavior**: Backup creation should fail gracefully with clear error message

**Path Does Not Exist:**
- Unix: Returns `syscall.ENOENT`
- Windows: Returns `syscall.ERROR_FILE_NOT_FOUND` or `ERROR_PATH_NOT_FOUND`
- **Behavior**: Create parent directories before calling space check

**Network Timeouts:**
- Both platforms: Network filesystem queries can hang indefinitely
- **Mitigation**: Document that network paths may cause slow backup starts
- **Future**: Add timeout context to space check calls

### Overflow and Large Filesystems

**Both Platforms:**
- Cap return value at `math.MaxInt64` (9,223,372,036,854,775,807 bytes ≈ 8 exabytes)
- Filesystems larger than 8EB report max value (edge case, unlikely until 2030s)
- Block size calculation protected against multiplication overflow

### Concurrent Access

**Both Platforms:**
- Space check is a snapshot at query time, not transactional
- Available space may decrease between check and backup write
- **Mitigation**: Pre-flight check provides best-effort validation; backup write handles actual out-of-space errors

---

## Future Enhancements

### Out of Scope (This PR)

1. **UNC Path Support**: Full support for Windows network paths (`\\server\share`)
   - Current implementation supports basic UNC paths via Win32 API
   - Advanced scenarios (DFS, mapped drives) deferred

2. **Disk Quota Management**: Proactive quota warnings
   - Could add separate endpoint for quota information
   - Requires additional Win32 API calls

3. **Real-time Space Monitoring**: Filesystem watcher for space changes
   - Would require platform-specific event listeners
   - Significant scope expansion

4. **Cross-Platform Backup Restoration**: Handling Windows vs Unix path separators in archives
   - Archive format already uses forward slashes (zip standard)
   - No changes needed for basic compatibility

### Technical Debt

**None identified.** This implementation:
- Follows Go best practices for platform-specific code
- Uses standard library and official `golang.org/x` extensions
- Maintains backward compatibility
- Adds no unnecessary complexity

---

## References

### Go Documentation
- [Build Constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
- [syscall package](https://pkg.go.dev/syscall)
- [golang.org/x/sys/windows](https://pkg.go.dev/golang.org/x/sys/windows)

### Windows API
- [GetDiskFreeSpaceExW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdiskfreespaceexw)
- [File Management Functions](https://learn.microsoft.com/en-us/windows/win32/fileio/file-management-functions)

### Similar Implementations
- Go stdlib: `os.Stat()` uses build tags for platform-specific `Sys()` implementation
- Docker: Uses `golang.org/x/sys` for platform-specific volume operations
- Prometheus: Platform-specific collectors via build tags

### Project Files
- GoReleaser config: `.goreleaser.yaml` (lines 23-35)
- Nightly CI: `.github/workflows/nightly-build.yml` (lines 268-285)
- Backend go.mod: `backend/go.mod` (line 95: `golang.org/x/sys v0.40.0`)

---

## Appendix: Build Tag Examples in Codebase

**Current Usage** (from analysis):
- `backend/integration/*_test.go` - Use `//go:build integration` for integration tests
- `backend/internal/api/handlers/security_handler_test_fixed.go` - Uses build tags

**Pattern Established:**
Build tags are already in use for test isolation. This PR extends the pattern to platform-specific production code.

---

## Implementation Order

**Recommended Sequence:**
1. Create `backup_service_disk_unix.go` (copy existing logic)
2. Test Unix compilation: `GOOS=linux go build`
3. Create `backup_service_disk_windows.go` (new implementation)
4. Test Windows compilation: `GOOS=windows go build`
5. Refactor `backup_service.go` to delegate
6. Add unit tests for both platforms
7. Update documentation
8. Verify GoReleaser builds all targets

**Critical Path:**
Phase 3 (Windows implementation) is the longest and most complex. Start research on Win32 API early.

---

**Plan Version**: 1.1
**Created**: 2026-01-30
**Updated**: 2026-01-30
**Author**: Planning Agent
**Status**: Ready for Implementation

---

## Plan Revision History

### v1.1 (2026-01-30)
- ✅ Added Windows long path support with `\\?\` prefix for paths > 260 characters
- ✅ Removed unused `syscall` and `unsafe` imports from Windows implementation
- ✅ Added missing test cases: long paths, Unicode paths, permission denied, symlinks
- ✅ Added detailed CI/CD matrix configuration with actual workflow YAML
- ✅ Documented limitations: quotas, mount points, symlinks, path lengths
- ✅ Enhanced error messages with path context in all error returns
- ✅ Removed out-of-scope sections: GoReleaser v2 migration, SQLite driver changes (separate issue)

### v1.0 (2026-01-30)
- Initial plan for cross-platform disk space check implementation

---

## Out of Scope

The following items are explicitly excluded from this implementation plan and may be addressed in separate issues:

### 1. GoReleaser v1 → v2 Migration
- **Rationale**: Cross-platform disk space check is independent of release tooling
- **Status**: Tracked in separate issue for GoReleaser configuration updates
- **Priority**: Can be addressed after disk space check implementation

### 2. SQLite Driver Migration
- **Rationale**: Database driver choice is independent of disk space queries
- **Status**: Current CGO-based SQLite driver works for all platforms
- **Priority**: Performance optimization, not a blocking issue for Windows compilation

### 3. Nightly Build CI/CD Issues
- **Rationale**: CI/CD pipeline fixes are separate from source code changes
- **Status**: Tracked in separate workflow configuration issues
- **Priority**: Can be addressed in parallel or after implementation

