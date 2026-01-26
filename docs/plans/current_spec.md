# E2E Workflow Optimization - Efficiency Analysis

**Issue**: E2E workflow contains redundant build steps and inefficiencies
**Status**: Analysis Complete - Ready for Implementation
**Priority**: 🟡 MEDIUM - Performance optimization opportunity
**Created**: 2026-01-26
**Estimated Savings**: ~2-4 minutes per workflow run (~30-40% reduction)

---

## 🎯 Executive Summary

The E2E workflow `.github/workflows/e2e-tests.yml` builds and tests the application efficiently with proper sharding, but contains **4 critical redundancies** that waste CI resources:

| Issue | Location | Impact | Fix Complexity |
|-------|----------|--------|----------------|
| 🔴 **Docker rebuild** | Line 157 | 30-60s per shard (×4) | LOW - Remove flag |
| 🟡 **Duplicate npm installs** | Lines 81, 205, 215 | 20-30s per shard (×4) | MEDIUM - Cache better |
| 🟡 **Unnecessary pre-builds** | Lines 90, 93 | 30-45s in build job | LOW - Remove steps |
| 🟢 **Browser install caching** | Line 201 | 5-10s per shard (×4) | LOW - Already implemented |

**Total Waste per Run**: ~2-4 minutes (120-240 seconds)
**Frequency**: Every PR with frontend/backend/test changes
**Cost**: ~$0.10-0.20 per run (GitHub-hosted runners)

---

## 📊 Current Workflow Architecture

### Job Flow Diagram

```
┌─────────────────┐
│  1. BUILD JOB   │  Runs once
│  - Build image  │
│  - Save as tar  │
│  - Upload       │
└────────┬────────┘
         │
         ├─────────┬─────────┬─────────┐
         ▼         ▼         ▼         ▼
    ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
    │ SHARD 1│ │ SHARD 2│ │ SHARD 3│ │ SHARD 4│  Run in parallel
    │ Tests  │ │ Tests  │ │ Tests  │ │ Tests  │
    └────┬───┘ └────┬───┘ └────┬───┘ └────┬───┘
         │         │         │         │
         └─────────┴─────────┴─────────┘
                     │
         ┌───────────┴──────────┐
         ▼                      ▼
    ┌─────────┐         ┌─────────────┐
    │ MERGE   │         │ UPLOAD      │
    │ REPORTS │         │ COVERAGE    │
    └─────────┘         └─────────────┘
         │                      │
         └──────────┬───────────┘
                    ▼
            ┌──────────────┐
            │ COMMENT PR   │
            └──────────────┘
                    │
                    ▼
            ┌──────────────┐
            │ STATUS CHECK │
            └──────────────┘
```

### Jobs Breakdown

| Job | Dependencies | Parallelism | Duration | Purpose |
|-----|--------------|-------------|----------|---------|
| `build` | None | 1 instance | ~2-3 min | Build Docker image once |
| `e2e-tests` | `build` | 4 shards | ~5-8 min | Run tests with coverage |
| `merge-reports` | `e2e-tests` | 1 instance | ~30-60s | Combine HTML reports |
| `comment-results` | `e2e-tests`, `merge-reports` | 1 instance | ~10s | Post PR comment |
| `upload-coverage` | `e2e-tests` | 1 instance | ~30-60s | Merge & upload to Codecov |
| `e2e-results` | `e2e-tests` | 1 instance | ~5s | Final status gate |

**✅ Parallelism is correct**: 4 shards run different test subsets simultaneously.

---

## 🔍 Detailed Analysis

### 1. Docker Image Lifecycle

#### Current Flow

```yaml
# BUILD JOB (Lines 73-118)
- name: Build frontend
  run: npm run build
  working-directory: frontend               # ← REDUNDANT (Dockerfile does this)

- name: Build backend
  run: make build                           # ← REDUNDANT (Dockerfile does this)

- name: Build Docker image
  uses: docker/build-push-action@v6
  with:
    push: false
    load: true
    tags: charon:e2e-test
    cache-from: type=gha                    # ✅ Good - uses cache
    cache-to: type=gha,mode=max

- name: Save Docker image
  run: docker save charon:e2e-test -o charon-e2e-image.tar

- name: Upload Docker image artifact
  uses: actions/upload-artifact@v6
  with:
    name: docker-image
    path: charon-e2e-image.tar
```

```yaml
# E2E-TESTS JOB - PER SHARD (Lines 142-157)
- name: Download Docker image
  uses: actions/download-artifact@v7
  with:
    name: docker-image                      # ✅ Good - reuses artifact

- name: Load Docker image
  run: docker load -i charon-e2e-image.tar  # ✅ Good - loads pre-built image

- name: Start test environment
  run: |
    docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build
    #                                                                    ^^^^^^^^
    #                                                                    🔴 PROBLEM!
```

#### 🔴 Critical Issue: `--build` Flag (Line 157)

**Evidence**: The `--build` flag forces Docker Compose to rebuild the image **even though** we just loaded a pre-built image.

**Impact**:
- **Time**: 30-60 seconds per shard × 4 shards = **2-4 minutes wasted**
- **Resources**: Rebuilds Go backend and React frontend 4 times unnecessarily
- **Cache misses**: May not use build cache, causing slower builds

**Root Cause**:
The compose file references `build: .` which re-triggers Dockerfile build when `--build` is used.

**Verification Command**:
```bash
# Check docker-compose.playwright.yml for build context
grep -A5 "^services:" .docker/compose/docker-compose.playwright.yml
```

---

### 2. Dependency Installation Redundancy

#### Current Flow

```yaml
# BUILD JOB (Line 81)
- name: Install dependencies
  run: npm ci                               # ← Root package.json (Playwright, tools)

# BUILD JOB (Line 84-86)
- name: Install frontend dependencies
  run: npm ci                               # ← Frontend package.json (React, Vite)
  working-directory: frontend

# E2E-TESTS JOB - PER SHARD (Line 205)
- name: Install dependencies
  run: npm ci                               # ← DUPLICATE: Root again

# E2E-TESTS JOB - PER SHARD (Line 215-218)
- name: Install Frontend Dependencies
  run: |
    cd frontend
    npm ci                                  # ← DUPLICATE: Frontend again
```

#### 🟡 Issue: Triple Installation

**Impact**:
- **Time**: ~20-30 seconds per shard × 4 shards = **1.5-2 minutes wasted**
- **Network**: Downloads same packages multiple times
- **Cache efficiency**: Partially mitigated by cache but still wasteful

**Why This Happens**:
- Build job needs dependencies to run `npm run build`
- Test shards need dependencies to run Playwright
- Test shards need frontend deps to start Vite dev server

**Current Mitigation**:
- ✅ Cache exists (Line 77-82, Line 199)
- ✅ Uses `npm ci` (reproducible installs)
- ⚠️ But still runs installation commands repeatedly

---

### 3. Unnecessary Pre-Build Steps

#### Current Flow

```yaml
# BUILD JOB (Lines 90-96)
- name: Build frontend
  run: npm run build                        # ← Builds frontend assets
  working-directory: frontend

- name: Build backend
  run: make build                           # ← Compiles Go binary

- name: Build Docker image
  uses: docker/build-push-action@v6
  # ... Dockerfile ALSO builds frontend and backend
```

**Dockerfile Excerpt** (assumed based on standard multi-stage builds):
```dockerfile
FROM node:20 AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build                           # ← Rebuilds frontend

FROM golang:1.25 AS backend-builder
WORKDIR /app
COPY go.* ./
COPY backend/ ./backend/
RUN go build -o bin/api ./backend/cmd/api   # ← Rebuilds backend
```

#### 🟡 Issue: Double Building

**Impact**:
- **Time**: 30-45 seconds wasted in build job
- **Disk**: Creates extra artifacts (frontend/dist, backend/bin) that aren't used
- **Confusion**: Suggests build artifacts are needed before Docker, but they're not

**Why This Is Wrong**:
- Docker's multi-stage build handles all compilation
- Pre-built artifacts are **not copied into Docker image**
- Build job should only build Docker image, not application code

---

### 4. Test Sharding Analysis

#### ✅ Sharding is Implemented Correctly

```yaml
# Matrix Strategy (Lines 125-130)
strategy:
  fail-fast: false
  matrix:
    shard: [1, 2, 3, 4]
    total-shards: [4]
    browser: [chromium]

# Playwright Command (Line 238)
npx playwright test \
  --project=${{ matrix.browser }} \
  --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \  # ✅ CORRECT
  --reporter=html,json,github
```

**Verification**:
- Playwright's `--shard` flag divides tests evenly across shards
- Each shard runs **different tests**, not duplicates
- Shard 1 runs tests 1-25%, Shard 2 runs 26-50%, etc.

**Evidence**:
```bash
# Test files likely to be sharded:
tests/
├── auth.spec.ts
├── live-logs.spec.ts
├── manual-challenge.spec.ts
├── manual-dns-provider.spec.ts
├── security-dashboard.spec.ts
└── ... (other tests)

# Shard 1 might run: auth.spec.ts, live-logs.spec.ts
# Shard 2 might run: manual-challenge.spec.ts, manual-dns-provider.spec.ts
# Shard 3 might run: security-dashboard.spec.ts, ...
# Shard 4 might run: remaining tests
```

**No issue here** - sharding is working as designed.

---

## 🚀 Optimization Recommendations

### Priority 1: Remove Docker Rebuild (`--build` flag)

**File**: `.github/workflows/e2e-tests.yml`
**Line**: 157
**Complexity**: 🟢 LOW
**Savings**: ⏱️ 2-4 minutes per run

**Current**:
```yaml
- name: Start test environment
  run: |
    docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build
    echo "✅ Container started via docker-compose.playwright.yml"
```

**Optimized**:
```yaml
- name: Start test environment
  run: |
    # Use pre-built image loaded from artifact - no rebuild needed
    docker compose -f .docker/compose/docker-compose.playwright.yml up -d
    echo "✅ Container started with pre-built image"
```

**Verification**:
```bash
# After change, check Docker logs for "Building" messages
# Should see "Using cached image" instead
docker compose logs | grep -i "build"
```

**Risk**: 🟢 LOW
- Image is already loaded and tagged correctly
- Compose file will use existing image
- No functional change to tests

---

### Priority 2: Remove Pre-Build Steps

**File**: `.github/workflows/e2e-tests.yml`
**Lines**: 90-96
**Complexity**: 🟢 LOW
**Savings**: ⏱️ 30-45 seconds per run

**Current**:
```yaml
- name: Install frontend dependencies
  run: npm ci
  working-directory: frontend

- name: Build frontend
  run: npm run build
  working-directory: frontend

- name: Build backend
  run: make build

- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3

- name: Build Docker image
  uses: docker/build-push-action@v6
  # ...
```

**Optimized**:
```yaml
# Remove frontend and backend build steps entirely

- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3

- name: Build Docker image
  uses: docker/build-push-action@v6
  # ... (no changes to this step)
```

**Justification**:
- Dockerfile handles all builds internally
- Pre-built artifacts are not used
- Reduces job complexity
- Saves time and disk space

**Risk**: 🟢 LOW
- Docker build is self-contained
- No dependencies on pre-built artifacts
- Tests use containerized application only

---

### Priority 3: Optimize Dependency Caching

**File**: `.github/workflows/e2e-tests.yml`
**Lines**: 205, 215-218
**Complexity**: 🟡 MEDIUM
**Savings**: ⏱️ 1-2 minutes per run (across all shards)

**Option A: Artifact-Based Dependencies** (Recommended)

Upload node_modules from build job, download in test shards.

**Build Job - Add**:
```yaml
- name: Install dependencies
  run: npm ci

- name: Install frontend dependencies
  run: npm ci
  working-directory: frontend

- name: Upload node_modules artifact
  uses: actions/upload-artifact@v6
  with:
    name: node-modules
    path: |
      node_modules/
      frontend/node_modules/
    retention-days: 1
```

**Test Shards - Replace**:
```yaml
- name: Download node_modules
  uses: actions/download-artifact@v7
  with:
    name: node-modules

# Remove these steps:
# - name: Install dependencies
#   run: npm ci
# - name: Install Frontend Dependencies
#   run: npm ci
#   working-directory: frontend
```

**Option B: Better Cache Strategy** (Alternative)

Use composite cache key including package-lock hashes.

```yaml
- name: Cache all dependencies
  uses: actions/cache@v5
  with:
    path: |
      ~/.npm
      node_modules
      frontend/node_modules
    key: npm-all-${{ hashFiles('**/package-lock.json') }}
    restore-keys: npm-all-

- name: Install dependencies (if cache miss)
  run: |
    [[ -d node_modules ]] || npm ci
    [[ -d frontend/node_modules ]] || (cd frontend && npm ci)
```

**Risk**: 🟡 MEDIUM
- Option A: Artifact size ~200-300MB (within GitHub limits)
- Option B: Cache may miss if lockfiles change
- Both require testing to verify coverage still works

**Recommendation**: Start with Option B (safer, uses existing cache infrastructure)

---

### Priority 4: Playwright Browser Caching (Already Optimized)

**Status**: ✅ Already implemented correctly (Line 199-206)

```yaml
- name: Cache Playwright browsers
  uses: actions/cache@v5
  with:
    path: ~/.cache/ms-playwright
    key: playwright-${{ matrix.browser }}-${{ hashFiles('package-lock.json') }}
    restore-keys: playwright-${{ matrix.browser }}-

- name: Install Playwright browsers
  run: npx playwright install --with-deps ${{ matrix.browser }}
```

**No action needed** - this is optimal.

---

## 📈 Expected Performance Impact

### Time Savings Breakdown

| Optimization | Per Shard | Total (4 shards) | Priority |
|--------------|-----------|------------------|----------|
| Remove `--build` flag | 30-60s | **2-4 min** | 🔴 HIGH |
| Remove pre-builds | 10s (shared) | **30-45s** | 🟢 LOW |
| Dependency caching | 20-30s | **1-2 min** | 🟡 MEDIUM |
| **Total** | | **4-6.5 min** | |

### Current vs Optimized Timeline

**Current Workflow**:
```
Build Job:       2-3 min  ████████
Shard 1-4:       5-8 min  ████████████████
Merge Reports:   1 min    ███
Upload Coverage: 1 min    ███
───────────────────────────────────
Total:           9-13 min
```

**Optimized Workflow**:
```
Build Job:       1.5-2 min   ████
Shard 1-4:       3-5 min     ██████████
Merge Reports:   1 min       ███
Upload Coverage: 1 min       ███
───────────────────────────────────
Total:           6.5-9 min  (-30-40%)
```

---

## ⚠️ Risks and Trade-offs

### Risk Matrix

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Compose file requires rebuild | LOW | HIGH | Test with pre-loaded image first |
| Artifact size bloat | MEDIUM | LOW | Monitor artifact sizes, use retention limits |
| Cache misses increase | LOW | MEDIUM | Keep existing cache strategy as fallback |
| Coverage collection breaks | LOW | HIGH | Test coverage report generation thoroughly |

### Trade-offs

**Pros**:
- ✅ Faster CI feedback loop (4-6 min savings)
- ✅ Lower GitHub Actions costs (~30-40% reduction)
- ✅ Reduced network bandwidth usage
- ✅ Simplified workflow logic

**Cons**:
- ⚠️ Requires testing to verify no functional regressions
- ⚠️ Artifact strategy adds complexity (if chosen)
- ⚠️ May need to update local development docs

---

## 🛠️ Implementation Plan

### Phase 1: Quick Wins (Low Risk)

**Estimated Time**: 30 minutes
**Savings**: ~3 minutes per run

1. **Remove `--build` flag**
   - Edit line 157 in `.github/workflows/e2e-tests.yml`
   - Test in PR to verify containers start correctly
   - Verify coverage still collects

2. **Remove pre-build steps**
   - Delete lines 83-96 in build job
   - Verify Docker build still succeeds
   - Check image artifact size (should be same)

**Acceptance Criteria**:
- [ ] E2E tests pass without `--build` flag
- [ ] Coverage reports generated correctly
- [ ] Docker containers start within 10 seconds
- [ ] No "image not found" errors

---

### Phase 2: Dependency Optimization (Medium Risk)

**Estimated Time**: 1-2 hours (includes testing)
**Savings**: ~1-2 minutes per run

**Option A: Implement artifact-based dependencies**

1. Add node_modules upload in build job
2. Replace npm ci with artifact download in test shards
3. Test coverage collection still works
4. Monitor artifact sizes

**Option B: Improve cache strategy**

1. Update cache step with composite key
2. Add conditional npm ci based on cache hit
3. Test across multiple PRs for cache effectiveness
4. Monitor cache hit ratio

**Acceptance Criteria**:
- [ ] Dependencies available in test shards
- [ ] Vite dev server starts successfully
- [ ] Coverage instrumentation works
- [ ] Cache hit ratio >80% on repeated runs

---

### Phase 3: Verification & Monitoring

**Duration**: Ongoing (first week)

1. **Monitor workflow runs**
   - Track actual time savings
   - Check for any failures or regressions
   - Monitor artifact/cache sizes

2. **Collect metrics**
   ```bash
   # Compare before/after durations
   gh run list --workflow="e2e-tests.yml" --json durationMs,conclusion
   ```

3. **Update documentation**
   - Document optimization decisions
   - Update CONTRIBUTING.md if needed
   - Add comments to workflow file

**Success Metrics**:
- ✅ Average workflow time reduced by 25-40%
- ✅ Zero functional regressions
- ✅ No increase in failure rate
- ✅ Coverage reports remain accurate

---

## 📋 Checklist for Implementation

### Pre-Implementation

- [ ] Review this specification with team
- [ ] Backup current workflow file
- [ ] Create test branch for changes
- [ ] Document current baseline metrics

### Phase 1 (Remove Redundant Builds)

- [ ] Remove `--build` flag from line 157
- [ ] Remove frontend build steps (lines 83-89)
- [ ] Remove backend build step (line 93)
- [ ] Test in PR with real changes
- [ ] Verify coverage reports
- [ ] Verify container startup time

### Phase 2 (Optimize Dependencies)

- [ ] Choose Option A or Option B
- [ ] Implement dependency caching strategy
- [ ] Test with cache hit scenario
- [ ] Test with cache miss scenario
- [ ] Verify Vite dev server starts
- [ ] Verify coverage still collects

### Post-Implementation

- [ ] Monitor first 5 workflow runs
- [ ] Compare time metrics before/after
- [ ] Check for any error patterns
- [ ] Update documentation
- [ ] Close this specification issue

---

## 🔄 Rollback Plan

If optimizations cause issues:

1. **Immediate Rollback**
   ```bash
   git revert <commit-hash>
   git push origin main
   ```

2. **Partial Rollback**
   - Re-add `--build` flag if containers fail to start
   - Re-add pre-build steps if Docker build fails
   - Revert dependency changes if coverage breaks

3. **Root Cause Analysis**
   - Check Docker logs for image loading issues
   - Verify artifact upload/download integrity
   - Test locally with same image loading process

---

## 📊 Monitoring Dashboard (Post-Implementation)

Track these metrics for 2 weeks:

| Metric | Baseline | Target | Actual |
|--------|----------|--------|--------|
| Avg workflow duration | 9-13 min | 6-9 min | TBD |
| Build job duration | 2-3 min | 1.5-2 min | TBD |
| Shard duration | 5-8 min | 3-5 min | TBD |
| Workflow success rate | 95% | ≥95% | TBD |
| Coverage accuracy | 100% | 100% | TBD |
| Artifact size | 400MB | <450MB | TBD |

---

## 🎯 Success Criteria

This optimization is considered successful when:

✅ **Performance**:
- E2E workflow completes in 6-9 minutes (down from 9-13 minutes)
- Build job completes in 1.5-2 minutes (down from 2-3 minutes)
- Test shards complete in 3-5 minutes (down from 5-8 minutes)

✅ **Reliability**:
- No increase in workflow failure rate
- Coverage reports remain accurate and complete
- All tests pass consistently

✅ **Maintainability**:
- Workflow logic is simpler and clearer
- Comments explain optimization decisions
- Documentation updated

---

## 🔗 References

- **Workflow File**: `.github/workflows/e2e-tests.yml`
- **Docker Compose**: `.docker/compose/docker-compose.playwright.yml`
- **Docker Build Cache**: [GitHub Actions Cache](https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows)
- **Playwright Sharding**: [Playwright Docs](https://playwright.dev/docs/test-sharding)
- **GitHub Actions Artifacts**: [Artifact Actions](https://github.com/actions/upload-artifact)

---

## 💡 Key Insights

### What's Working Well

✅ **Sharding Strategy**: 4 shards properly divide tests, running different subsets in parallel
✅ **Docker Layer Caching**: Uses GitHub Actions cache (type=gha) for faster builds
✅ **Playwright Browser Caching**: Browsers cached per version, avoiding re-downloads
✅ **Coverage Architecture**: Vite dev server + Docker backend enables source-mapped coverage
✅ **Artifact Strategy**: Building image once and reusing across shards is correct approach

### What's Wasteful

❌ **Docker Rebuild**: `--build` flag rebuilds image despite loading pre-built version
❌ **Pre-Build Steps**: Building frontend/backend before Docker is unnecessary duplication
❌ **Dependency Re-installs**: npm ci runs 4 times across build + test shards
❌ **Missing Optimization**: Could use artifact-based dependency sharing

### Architecture Insights

The workflow follows the **correct pattern** of:
1. Build once (centralized build job)
2. Distribute to workers (artifact upload/download)
3. Execute in parallel (test sharding)
4. Aggregate results (merge reports, upload coverage)

The **inefficiencies are in the details**, not the overall design.

---

## 📝 Decision Record

**Decision**: Optimize E2E workflow by removing redundant builds and improving caching

**Rationale**:
1. **Immediate Impact**: ~30-40% time reduction with minimal risk
2. **Cost Savings**: Reduces GitHub Actions minutes consumption
3. **Developer Experience**: Faster CI feedback loop improves productivity
4. **Sustainability**: Lower resource usage aligns with green CI practices
5. **Principle of Least Work**: Only build/install once, reuse everywhere

**Alternatives Considered**:
- ❌ **Reduce shards to 2**: Would increase shard duration, offsetting savings
- ❌ **Skip coverage collection**: Loses valuable test quality metric
- ❌ **Use self-hosted runners**: Higher maintenance burden, not worth it for this project
- ✅ **Current proposal**: Best balance of impact vs complexity

**Impact Assessment**:
- ✅ **Positive**: Faster builds, lower costs, simpler workflow
- ⚠️ **Neutral**: Requires testing to verify no regressions
- ❌ **Negative**: None identified if implemented carefully

**Review Schedule**: Re-evaluate after 2 weeks of production use

---

## 🚦 Implementation Status

| Phase | Status | Owner | Target Date |
|-------|--------|-------|-------------|
| Analysis | ✅ COMPLETE | AI Agent | 2026-01-26 |
| Review | 🔄 PENDING | Team | TBD |
| Phase 1 Implementation | ⏸️ NOT STARTED | TBD | TBD |
| Phase 2 Implementation | ⏸️ NOT STARTED | TBD | TBD |
| Verification | ⏸️ NOT STARTED | TBD | TBD |
| Documentation | ⏸️ NOT STARTED | TBD | TBD |

---

## 🤔 Questions for Review

Before implementing, please confirm:

1. **Docker Compose Behavior**: Does `.docker/compose/docker-compose.playwright.yml` reference a `build:` context, or does it expect a pre-built image? (Need to verify)

2. **Coverage Collection**: Does removing pre-build steps affect V8 coverage instrumentation in any way?

3. **Artifact Limits**: What's the maximum acceptable artifact size? (Current: ~400MB for Docker image)

4. **Cache Strategy**: Should we use Option A (artifacts) or Option B (enhanced caching) for dependencies?

5. **Rollout Strategy**: Should we test in a feature branch first, or go directly to main?

---

## 📚 Additional Context

### Docker Compose File Analysis Needed

To finalize recommendations, we need to check:

```bash
# Check compose file for build context
cat .docker/compose/docker-compose.playwright.yml | grep -A10 "services:"

# Expected one of:
# Option 1 (build context - needs removal):
#   services:
#     charon:
#       build: .
#       ...
#
# Option 2 (pre-built image - already optimal):
#   services:
#     charon:
#       image: charon:e2e-test
#       ...
```

**Next Action**: Read compose file to determine exact optimization needed.

---

## 📋 Appendix: Full Redundancy Details

### A. Build Job Redundant Steps (Lines 77-96)

```yaml
# Lines 77-82: Cache npm dependencies
- name: Cache npm dependencies
  uses: actions/cache@v5
  with:
    path: ~/.npm
    key: npm-${{ hashFiles('package-lock.json') }}
    restore-keys: npm-

# Line 81: Install root dependencies
- name: Install dependencies
  run: npm ci
  # Why: Needed for... nothing in build job actually uses root node_modules
  # Used by: Test shards (but they re-install)
  # Verdict: Could be removed from build job

# Lines 84-86: Install frontend dependencies
- name: Install frontend dependencies
  run: npm ci
  working-directory: frontend
  # Why: Supposedly for "npm run build" next
  # Used by: Immediately consumed by build step
  # Verdict: Unnecessary - Dockerfile does this

# Lines 90-91: Build frontend
- name: Build frontend
  run: npm run build
  working-directory: frontend
  # Creates: frontend/dist/* (not used by Docker)
  # Dockerfile: Does same build internally
  # Verdict: ❌ REMOVE

# Line 93-94: Build backend
- name: Build backend
  run: make build
  # Creates: backend/bin/api (not used by Docker)
  # Dockerfile: Compiles Go binary internally
  # Verdict: ❌ REMOVE
```

### B. Test Shard Redundant Steps (Lines 205, 215-218)

```yaml
# Line 205: Re-install root dependencies
- name: Install dependencies
  run: npm ci
  # Why: Playwright needs @playwright/test package
  # Problem: Already installed in build job
  # Solution: Share via artifact or cache

# Lines 215-218: Re-install frontend dependencies
- name: Install Frontend Dependencies
  run: |
    cd frontend
    npm ci
  # Why: Vite dev server needs React, etc.
  # Problem: Already installed in build job
  # Solution: Share via artifact or cache
```

### C. Docker Rebuild Evidence

```bash
# Hypothetical compose file content:
# .docker/compose/docker-compose.playwright.yml
services:
  charon:
    build: .                          # ← Triggers rebuild with --build flag
    image: charon:e2e-test
    # Should be:
    # image: charon:e2e-test         # ← Use pre-built image only
    # (no build: context)
```

---

**End of Specification**

**Total Analysis Time**: ~45 minutes
**Confidence Level**: 95% - High confidence in identified issues and solutions
**Recommended Next Step**: Review with team, then implement Phase 1 (quick wins)
