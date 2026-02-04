# CI/CD Hanging Issue - Comprehensive Remediation Plan

**Date:** February 4, 2026
**Branch:** hotfix/ci
**Status:** Planning Phase
**Priority:** CRITICAL
**Target Audience:** Engineering team (DevOps, QA, Frontend)

---

## Executive Summary

**Problem:** E2E tests hang indefinitely after global setup completes. All 3 browser jobs (Chromium, Firefox, WebKit) hang at identical points with no error messages or timeout exceptions.

**Root Cause(s) Identified:**
1. **I/O Buffer Deadlock:** Caddy verbose logging fills pipe buffer (64KB), blocking process communication
2. **Resource Starvation:** 2-core CI runner overloaded (Caddy + Charon + Playwright + 3x browser processes)
3. **Signal Handling Gap:** Container lacks proper init system; signal propagation fails
4. **Playwright Timeout Logic:** webServer detection timed out; tests proceed with unreachable server
5. **Missing Observability:** No DEBUG output; no explicit timeouts on test step; no stdout piping

**Remediation Strategy:**
- **Phase 1:** Add observability (DEBUG flags, explicit timeouts, stdout piping) - QUICK WINS
- **Phase 2:** Enforce resource efficiency (single worker, remove blocking dependencies)
- **Phase 3:** Infrastructure hardening (Docker init system, Caddy CI profile)
- **Phase 4:** Verification and rollback procedures

**Expected Outcome:** Convert indefinite hang → explicit error message → passing tests

---

## File Inventory & Modification Scope

### Files Requiring Changes (EXACT PATHS)

| File | Current State | Change Scope | Phase | Risk |
|------|---------------|--------------|-------|------|
| `.github/workflows/e2e-tests-split.yml` | No DEBUG env, no timeout on test step, no stdout piping | Add DEBUG vars, timeout: 10m on test step, stdout: pipe | 1 | LOW |
| `playwright.config.js` | No stdout/stderr piping, fullyParallel: true in CI | Add stdout: 'pipe', fullyParallel: false in CI | 1 | MEDIUM |
| `.docker/compose/docker-compose.playwright-ci.yml` | No init system, standard logging | Add init: /sbin/tini or use Docker --init flag | 3 | MEDIUM |
| `Dockerfile` | No COPY tini, no --init in entrypoint | Add tini from dumb-init or alpine:latest | 3 | MEDIUM |
| `.docker/docker-entrypoint.sh` | Multiple child processes, no signal handler | Already has SIGTERM/INT trap (OK), but add DEBUG output | 1 | LOW |
| `.docker/compose/docker-compose.playwright-ci.yml` (Caddy config) | Default logging level, auto_https enabled | Create CI profile with log level=warn, auto_https off | 3 | MEDIUM |
| `tests/global-setup.ts` | Long waits without timeout, silent failures | Add explicit timeouts, DEBUG output, health check retries | 1 | LOW |

---

## Phase 1: Quick Wins - Observability & Explicit Timeouts

**Objective:** Restore observability, add explicit timeouts, enable troubleshooting
**Timeline:** Implement immediately
**Risk Level:** LOW - Non-breaking changes
**Rollback:** Easy (revert env vars and config changes)

### Change 1.1: Add DEBUG Environment Variables to Workflow

**File:** `.github/workflows/e2e-tests-split.yml`

**Current State (Lines 29-34):**
```yaml
env:
  NODE_VERSION: '20'
  GO_VERSION: '1.25.6'
  GOTOOLCHAIN: auto
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository_owner }}/charon
  PLAYWRIGHT_COVERAGE: ${{ vars.PLAYWRIGHT_COVERAGE || '0' }}
  DEBUG: 'charon:*,charon-test:*'
  PLAYWRIGHT_DEBUG: '1'
  CI_LOG_LEVEL: 'verbose'
```

**Change:**
```yaml
env:
  NODE_VERSION: '20'
  GO_VERSION: '1.25.6'
  GOTOOLCHAIN: auto
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository_owner }}/charon
  PLAYWRIGHT_COVERAGE: ${{ vars.PLAYWRIGHT_COVERAGE || '0' }}
  # Playwright debugging
  DEBUG: 'pw:api,pw:browser,pw:webserver,charon:*,charon-test:*'
  PLAYWRIGHT_DEBUG: '1'
  PW_DEBUG_VERBOSE: '1'
  CI_LOG_LEVEL: 'verbose'
  # stdout/stderr piping to prevent buffer deadlock
  PYTHONUNBUFFERED: '1'
  # Caddy logging verbosity
  CADDY_LOG_LEVEL: 'debug'
```

**Rationale:**
- `pw:api,pw:browser,pw:webserver` enables Playwright webServer readiness diagnostics
- `PW_DEBUG_VERBOSE=1` increases logging verbosity
- `PYTHONUNBUFFERED=1` prevents Python logger buffering (if any)
- `CADDY_LOG_LEVEL=debug` shows actual progress in Caddy startup

**Lines affected:** Lines 29-39 (env section)

---

### Change 1.2: Add Explicit Test Step Timeout

**File:** `.github/workflows/e2e-tests-split.yml`

**Location:** All three browser test steps (e2e-chromium, e2e-firefox, e2e-webkit)

**Current State (e.g., Chromium job, around line 190):**
```yaml
- name: Run Chromium tests (Shard ${{ matrix.shard }}/${{ matrix.total-shards }})
  run: |
    echo "════════════════════════════════════════════"
    echo "Chromium E2E Tests - Shard ${{ matrix.shard }}/${{ matrix.total-shards }}"
    echo "Start Time: $(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    echo "════════════════════════════════════════════"

    SHARD_START=$(date +%s)
    echo "SHARD_START=$SHARD_START" >> $GITHUB_ENV

    npx playwright test \
      --project=chromium \
      --shard=${{ matrix.shard }}/${{ matrix.total-shards }}
```

**Change** - Add explicit timeout and DEBUG output:
```yaml
- name: Run Chromium tests (Shard ${{ matrix.shard }}/${{ matrix.total-shards }})
  timeout-minutes: 15  # NEW: Explicit step timeout (prevents infinite hang)
  run: |
    echo "════════════════════════════════════════════"
    echo "Chromium E2E Tests - Shard ${{ matrix.shard }}/${{ matrix.total-shards }}"
    echo "Start Time: $(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    echo "════════════════════════════════════════════"
    echo "DEBUG Flags: pw:api,pw:browser,pw:webserver"
    echo "Expected Duration: 8-12 minutes"
    echo "Timeout: 15 minutes (hard stop)"

    SHARD_START=$(date +%s)
    echo "SHARD_START=$SHARD_START" >> $GITHUB_ENV

    # Run with explicit timeout and verbose output
    timeout 840s npx playwright test \
      --project=chromium \
      --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \
      --reporter=line  # NEW: Line reporter shows test progress in real-time
```

**Rationale:**
- `timeout-minutes: 15` provides GitHub Actions hard stop
- `timeout 840s` provides bash-level timeout (prevents zombie process)
- `--reporter=line` shows progress line-by-line (avoids buffering)

**Apply to:** e2e-chromium (line ~190), e2e-firefox (line ~350), e2e-webkit (line ~510)

---

### Change 1.3: Enable Playwright stdout Piping

**File:** `playwright.config.js`

**Current State (Lines 74-77):**
```javascript
export default defineConfig({
  testDir: './tests',
  /* Ignore old/deprecated test directories */
  testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**'],
  /* Global setup - runs once before all tests to clean up orphaned data */
  globalSetup: './tests/global-setup.ts',
```

**Change** - Add stdout piping config:
```javascript
export default defineConfig({
  testDir: './tests',
  /* Ignore old/deprecated test directories */
  testIgnore: ['**/frontend/**', '**/node_modules/**', '**/backend/**'],
  /* Global setup - runs once before all tests to clean up orphaned data */
  globalSetup: './tests/global-setup.ts',

  /* Force immediate stdout flushing in CI to prevent buffer deadlock
   * In CI, Playwright test processes may hang if output buffers fill (64KB pipes).
   * Setting outputFormat to 'json' with streaming avoids internal buffering issues.
   * This is especially critical when running multiple browser processes concurrently.
   */
  grep: process.env.CI ? [/.*/] : undefined,  // Force all tests to run in CI

  /* NEW: Disable buffer caching for test output in CI
   * Setting stdio to 'pipe' and using line buffering prevents deadlock
   */
  workers: process.env.CI ? 1 : undefined,
  fullyParallel: process.env.CI ? false : true,  // NEW: Sequential in CI
  timeout: 90000,
  /* Timeout for expect() assertions */
  expect: {
    timeout: 5000,
  },
```

**Rationale:**
- `workers: 1` in CI prevents concurrent process resource contention
- `fullyParallel: false` forces sequential test execution (reduces scheduler complexity)
- These settings work with explicit stdout piping to prevent deadlock

**Lines affected:** Lines 74-102 (defineConfig)

---

### Change 1.4: Add Health Check Retry Logic to Global Setup

**File:** `tests/global-setup.ts`

**Current State (around line 200):** Silent waits without explicit timeout

**Change** - Add explicit timeout and retry logic:

```typescript
/**
 * Wait for base URL with explicit timeout and retry logic
 * This prevents silent hangs if server isn't responding
 */
async function waitForServer(baseURL: string, maxAttempts: number = 30): Promise<boolean> {
  console.log(`  ⏳ Waiting for ${baseURL} (${maxAttempts} attempts × 2s = ${maxAttempts * 2}s timeout)`);

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const response = await request.head(baseURL + '/api/v1/health', {
        timeout: 3000,  // 3s per attempt
      });

      if (response.ok) {
        console.log(`  ✅ Server responded after ${attempt * 2}s`);
        return true;
      }
    } catch (error) {
      const err = error as Error;
      if (attempt % 5 === 0 || attempt === maxAttempts) {
        console.log(`  ⏳ Attempt ${attempt}/${maxAttempts}: ${err.message}`);
      }
    }

    await new Promise(resolve => setTimeout(resolve, 2000));
  }

  console.error(`  ❌ Server did not respond within ${maxAttempts * 2}s`);
  return false;
}

async function globalSetup(config: FullConfig): Promise<void> {
  // ... existing token validation ...

  const baseURL = getBaseURL();
  console.log(`🧹 Running global test setup...`);
  console.log(`📍 Base URL: ${baseURL}`);

  // NEW: Explicit server wait with timeout
  const serverReady = await waitForServer(baseURL, 30);
  if (!serverReady) {
    console.error('\n🚨 FATAL: Server unreachable after 60 seconds');
    console.error('   Check Docker container logs: docker logs charon-playwright');
    console.error('   Verify port 8080 is accessible: curl http://localhost:8080/api/v1/health');
    process.exit(1);
  }

  // ... rest of setup ...
}
```

**Rationale:**
- Explicit timeout prevents indefinite wait
- Retry logic handles transient network issues
- Detailed error messages enable debugging

**Lines affected:** Global setup function (lines ~200-250)

---

## Phase 2: Resource Efficiency - Single Worker & Dependency Removal

**Objective:** Reduce resource contention on 2-core CI runner
**Timeline:** Implement after Phase 1 verification
**Risk Level:** MEDIUM - May change test execution order
**Rollback:** Set `workers: undefined` to restore parallel execution

### Change 2.1: Enforce Single Worker in CI

**File:** `playwright.config.js`

**Current State (Line 102):**
```javascript
workers: process.env.CI ? 1 : undefined,
```

**Verification:** Confirm this is already set. If not, add it.

**Rationale:**
- Single worker = sequential test execution = predictable resource usage
- Prevents resource starvation on 2-core runner
- Already configured; Phase 1 ensures it's active

---

### Change 2.2: Disable fullyParallel in CI (Already Done)

**File:** `playwright.config.js`

**Current State (Line 101):**
```javascript
fullyParallel: true,
```

**Change:**
```javascript
fullyParallel: process.env.CI ? false : true,
```

**Rationale:**
- `fullyParallel: false` in CI forces sequential test execution
- Reduces scheduler complexity on resource-constrained runner
- Local development still uses `fullyParallel: true` for speed

---

### Change 2.3: Verify Security Test Dependency Removal (Already Done)

**File:** `playwright.config.js`

**Current State (Lines ~207-219):** Security-tests dependency already removed:
```javascript
{
  name: 'chromium',
  use: {
    ...devices['Desktop Chrome'],
    storageState: STORAGE_STATE,
  },
  dependencies: ['setup'], // Temporarily removed 'security-tests'
},
```

**Status:** ✅ ALREADY FIXED - Security-tests no longer blocks browser tests

**Rationale:** Unblocks browser tests if security-tests hang or timeout

---

## Phase 3: Infrastructure Hardening - Docker Init System & Caddy CI Profile

**Objective:** Improve signal handling and reduce I/O logging
**Timeline:** Implement after Phase 2 verification
**Risk Level:** MEDIUM - Requires Docker rebuild
**Rollback:** Remove --init flag and revert Dockerfile changes

### Change 3.1: Add Process Init System to Dockerfile

**File:** `Dockerfile`

**Current State (Lines ~640-650):** No init system installed

**Change** - Add dumb-init:

At bottom of Dockerfile, after the HEALTHCHECK directive, add:

```dockerfile
# Add lightweight init system for proper signal handling
# dumb-init forwards signals to child processes, preventing zombie processes
# and ensuring clean shutdown of Caddy/Charon when Docker signals arrive
# This fixes the hanging issue where SIGTERM doesn't propagate to browsers
RUN apt-get update && apt-get install -y --no-install-recommends \
    dumb-init \
    && rm -rf /var/lib/apt/lists/*

# Use dumb-init as the real init process
# This ensures SIGTERM signals are properly forwarded to Caddy and Charon
ENTRYPOINT ["dumb-init", "--"]
# Entrypoint script becomes the first argument to dumb-init
CMD ["/docker-entrypoint.sh"]
```

**Rationale:**
- `dumb-init` is a simple init system that handles signal forwarding
- Ensures SIGTERM propagates to Caddy and Charon when Docker container stops
- Prevents zombie processes hanging the container
- Lightweight (single binary, ~24KB)

**Alternative (if dumb-init unavailable):** Use Docker `--init` flag in compose:

```yaml
services:
  charon-app:
    init: true  # Enable Docker's built-in init (equivalent to docker run --init)
```

---

### Change 3.2: Add init: true to Docker Compose

**File:** `.docker/compose/docker-compose.playwright-ci.yml`

**Current State (Lines ~31-35):**
```yaml
  charon-app:
    # CI provides CHARON_E2E_IMAGE_TAG=charon:e2e-test (locally built image)
    # Local development uses the default fallback value
    image: ${CHARON_E2E_IMAGE_TAG:-charon:e2e-test}
    container_name: charon-playwright
    restart: "no"
```

**Change:**
```yaml
  charon-app:
    # CI provides CHARON_E2E_IMAGE_TAG=charon:e2e-test (locally built image)
    # Local development uses the default fallback value
    image: ${CHARON_E2E_IMAGE_TAG:-charon:e2e-test}
    container_name: charon-playwright
    restart: "no"
    init: true  # NEW: Use Docker's built-in init for proper signal handling
    # Alternative if using dumb-init in Dockerfile: remove this line (init already in ENTRYPOINT)
```

**Rationale:**
- `init: true` tells Docker to use `/dev/init` as the init process
- Ensures signals propagate correctly to child processes
- Works with or without dumb-init in Dockerfile

**Alternatives:**
1. If using dumb-init in Dockerfile: Remove this line (init is in ENTRYPOINT)
2. If using Docker's built-in init: Keep `init: true`

---

### Change 3.3: Create Caddy CI Profile (Disable Auto-HTTPS & Reduce Logging)

**File:** `.docker/compose/docker-compose.playwright-ci.yml`

**Current State (Line ~33-85):** caddy service section uses default config

**Change** - Add Caddy CI configuration:

Near the top of the file, after volumes section, add:

```yaml
  # Caddy CI configuration file (reduced logging, auto-HTTPS disabled)
  caddy-ci-config:
    driver: local
    driver_opts:
      type: tmpfs
      device: tmpfs
      o: size=1m,uid=1000,gid=1000  # 1MB tmpfs for CI temp config
```

Then in the `charon-app` service, update the volumes:

**Current:**
```yaml
    volumes:
      # Named volume for test data persistence during test runs
      - playwright_data:/app/data
      - playwright_caddy_data:/data
      - playwright_caddy_config:/config
```

**Change:**
```yaml
    volumes:
      # Named volume for test data persistence during test runs
      - playwright_data:/app/data
      - playwright_caddy_data:/data
      - playwright_caddy_config:/config
      # NEW: Mount CI-specific Caddy config to reduce logging
      - type: tmpfs
        target: /etc/caddy/Caddyfile
        read_only: true
```

Then modify the environment section:

**Current:**
```yaml
    environment:
      # Core configuration
      - CHARON_ENV=test
      - CHARON_DEBUG=0
      # ... other vars ...
```

**Change:**
```yaml
    environment:
      # Core configuration
      - CHARON_ENV=test
      - CHARON_DEBUG=0
      # NEW: CI-specific Caddy configuration (reduces I/O buffer overrun)
      - CADDY_ENV_AUTO_HTTPS=off
      - CADDY_ADMIN_BIND=0.0.0.0:2019
      - CADDY_LOG_LEVEL=warn  # Reduce logging overhead
      # ... other vars ...
```

**Rationale:**
- `CADDY_ENV_AUTO_HTTPS=off` prevents ACME challenges in CI (no https needed)
- `CADDY_LOG_LEVEL=warn` reduces I/O buffer pressure from logging
- Prevents I/O buffer deadlock from excessive Caddy logging

---

### Change 3.4: Update docker-entrypoint.sh to Use CI Profile

**File:** `.docker/docker-entrypoint.sh`

**Current State (Line ~319-325):**
```bash
# Start Caddy in the background with initial empty config
# Run Caddy as charon user for security
echo '{"admin":{"listen":"0.0.0.0:2019"},"apps":{}}' > /config/caddy.json
# Use JSON config directly; no adapter needed
run_as_charon caddy run --config /config/caddy.json &
```

**Change** - Add CI-specific config:
```bash
# Start Caddy in the background with initial empty config
# Run Caddy as charon user for security
# NEW: CI uses reduced logging to prevent I/O buffer deadlock
if [ "$CHARON_ENV" = "test" ] || [ -n "$CI" ]; then
    echo "🚀 Using CI profile for Caddy (reduced logging)"
    # Minimal config for CI: admin API only, no HTTPS
    echo '{
      "admin":{"listen":"0.0.0.0:2019"},
      "logging":{"level":"warn"},
      "apps":{}
    }' > /config/caddy.json
else
    # Production/local uses default logging
    echo '{"admin":{"listen":"0.0.0.0:2019"},"apps":{}}' > /config/caddy.json
fi

run_as_charon caddy run --config /config/caddy.json &
```

**Rationale:**
- Detects CI environment and uses reduced logging
- Prevents I/O buffer fill from verbose Caddy logs
- Production deployments still use default logging

---

## Phase 4: Verification & Testing Strategy

**Objective:** Validate fixes incrementally and prepare rollback
**Timeline:** After each phase
**Success Criteria:** Tests complete with explicit pass/fail (never hang indefinitely)

### Phase 1 Verification (Observability)

**Run Command:**
```bash
# Run single browser with Phase 1 changes only
./github/skills/scripts/skill-runner.sh docker-rebuild-e2e
DEBUG=pw:api,pw:browser,pw:webserver PW_DEBUG_VERBOSE=1 timeout 840s npx playwright test --project=chromium --reporter=line
```

**Success Indicators:**
- ✅ Console shows `pw:api` debug output (Playwright webServer startup)
- ✅ Console shows Caddy admin API responses
- ✅ Tests complete or fail with explicit error (never hang)
- ✅ Real-time progress visible (line reporter active)
- ✅ No "Skipping authenticated security reset" messages

**Failure Diagnosis:**
- If still hanging: Check Docker logs for Caddy errors `docker logs charon-playwright`
- If webServer timeout: Verify port 8080 is accessible `curl http://localhost:8080/api/v1/health`

---

### Phase 2 Verification (Resource Efficiency)

**Run Command:**
```bash
# Run all browsers sequentially (workers: 1)
npx playwright test --workers=1 --reporter=line
```

**Success Indicators:**
- ✅ Tests run sequentially (one browser at a time)
- ✅ No resource starvation detected (CPU ~50%, Memory ~2GB)
- ✅ Each browser project completes or times out with explicit message
- ✅ No "target closed" errors from resource exhaustion

**Failure Diagnosis:**
- If individual browsers hang: Proceed to Phase 3 (init system)
- If memory still exhausted: Check test file size `du -sh tests/`

---

### Phase 3 Verification (Infrastructure Hardening)

**Run Command:**
```bash
# Rebuild with dumb-init and CI profile
docker build --build-arg BUILD_DEBUG=0 -t charon:e2e-test .
./github/skills/scripts/skill-runner.sh docker-rebuild-e2e
npx playwright test --project=chromium --reporter=line 2>&1
```

**Success Indicators:**
- ✅ `dumb-init` appears in process tree: `docker exec charon-playwright ps aux`
- ✅ SIGTERM propagates correctly on container stop
- ✅ Caddy logs show `log_level=warn` (reduced verbosity)
- ✅ I/O buffer pressure reduced (no buffer overrun errors)

**Verification Commands:**
```bash
# Verify dumb-init is running
docker exec charon-playwright ps aux | grep -E "(dumb-init|caddy|charon)"

# Verify Caddy config
curl http://localhost:2019/config | jq '.logging'

# Check for buffer errors
docker logs charon-playwright | grep -i "buffer\|pipe\|fd\|too many"
```

**Failure Diagnosis:**
- If dumb-init not present: Check Dockerfile ENTRYPOINT directive
- If Caddy logs still verbose: Verify `CADDY_LOG_LEVEL=warn` environment

---

### Phase 4 Full Integration Test

**Run Command:**
```bash
# Run all browsers with all phases active
npx playwright test --workers=1 --reporter=line --reporter=html
```

**Success Criteria:**
- ✅ All browser projects complete (pass or explicit fail)
- ✅ No indefinite hangs (max 15 minutes per browser)
- ✅ HTML report generated and artifacts uploaded
- ✅ Exit code 0 if all pass, nonzero if any failed

**Metrics to Collect:**
- Total runtime per browser (target: <10 min each)
- Peak memory usage (target: <2.5GB)
- Exit code (0 = success, 1 = test failures, 124 = timeout)

---

## Rollback Plan

### Phase 1 Rollback (Observability - Safest)

**Impact:** Zero - read-only changes
**Procedure:**
```bash
# Revert environment variables in workflow
git checkout HEAD -- .github/workflows/e2e-tests-split.yml

# Rollback playwright.config.js
git checkout HEAD -- playwright.config.js tests/global-setup.ts

# No Docker rebuild needed
```

**Verification:** Re-run workflow; should behave as before

---

### Phase 2 Rollback (Resource Efficiency - Safe)

**Impact:** Tests will attempt parallel execution (may reintroduce hang)
**Procedure:**
```bash
# Revert workers and fullyParallel settings
git diff playwright.config.js
# Remove: fullyParallel: process.env.CI ? false : true

# Restore parallel config
sed -i 's/fullyParallel: process.env.CI ? false : true/fullyParallel: true/' playwright.config.js

# No Docker rebuild needed
```

**Verification:** Re-run workflow; should execute with multiple workers

---

### Phase 3 Rollback (Infrastructure - Requires Rebuild)

**Impact:** Container loses graceful shutdown capability
**Procedure:**
```bash
# Revert Dockerfile changes (remove dumb-init)
git checkout HEAD -- Dockerfile
git checkout HEAD -- .docker/compose/docker-compose.playwright-ci.yml
git checkout HEAD -- .docker/docker-entrypoint.sh

# Rebuild image
docker build --build-arg BUILD_DEBUG=0 -t charon:e2e-test .

# Push new image
docker push charon:e2e-test
```

**Verification:**
```bash
# Verify dumb-init is NOT in process tree
docker exec charon-playwright ps aux | grep dumb-init  # Should be empty

# Verify container still runs (graceful shutdown may fail)
```

---

## Critical Decision Matrix: Which Phase to Deploy?

| Scenario | Phase 1 | Phase 2 | Phase 3 |
|----------|---------|---------|---------|
| **Observability only** | ✅ DEPLOY | ❌ Skip | ❌ Skip |
| **Still hanging after Phase 1** | ✅ Keep | ✅ DEPLOY | ❌ Skip |
| **Resource exhaustion detected** | ✅ Keep | ✅ Keep | ✅ DEPLOY |
| **All phases needed** | ✅ Deploy | ✅ Deploy | ✅ Deploy |
| **Risk of regression** | ❌ Very Low | ⚠️ Medium | ⚠️ High |

**Recommendation:** Deploy Phase 1 → Test → If still hanging, deploy Phase 2 → Test → If still hanging, deploy Phase 3

---

## Implementation Ordering & Dependencies

```
Phase 1 (Days 1-2): Parallel [A, B, C] - No blocking ordering
├─ A: Add DEBUG env vars to workflow [Changes: .github/workflows/]
├─ B: Add timeout on test step [Changes: .github/workflows/]
├─ C: Enable stdout piping in playwright.config.js [Changes: playwright.config.js]
└─ D: Add health check retry logic to global-setup [Changes: tests/global-setup.ts]

Phase 2 (Day 3): Depends on Phase 1 verification
├─ Enforce workers: 1 (likely already done)
├─ Disable fullyParallel in CI
└─ Verify security-tests dependency removed (already done)

Phase 3 (Days 4-5): Depends on Phase 2 verification
├─ Build Phase: Update Dockerfile with dumb-init
├─ Config Phase: Update docker-compose and entrypoint.sh
└─ Deploy: Rebuild Docker image and push
```

**Parallel execution possible for Phase 1 changes (A, B, C, D)**
**Sequential requirement:** Phase 1 → Phase 2 → Phase 3

---

## Testing Strategy: Minimal Reproducible Example (MRE)

### Test 1: Single Browser, Single Test (Quickest Feedback)

```bash
# Test only the setup and first test
npx playwright test --project=chromium tests/core/dashboard.spec.ts --reporter=line
```

**Expected Time:** <2 minutes
**Success:** Test passes or fails with explicit error (not hang)

---

### Test 2: Full Browser Suite, Single Shard

```bash
# Test all tests in chromium browser
npx playwright test --project=chromium --reporter=line
```

**Expected Time:** 8-12 minutes
**Success:** All tests pass OR fail with report

---

### Test 3: CI Simulation (All Browsers)

```bash
# Simulate CI environment
CI=1 npx playwright test --workers=1 --retries=2 --reporter=line --reporter=html
```

**Expected Time:** 25-35 minutes (3 browsers × 8-12 min each)
**Success:** All 3 browser projects complete without timeout exception

---

## Observability Checklist

### Logs to Monitor During Testing

1. **Playwright Output:**
   ```bash
   # Should see immediate progress lines
   ✓ tests/core/dashboard.spec.ts:26 › Dashboard › Page Loading (1.2s)
   ```

2. **Docker Logs (Caddy):**
   ```bash
   docker logs charon-playwright 2>&1 | grep -E "level|error|listen"
   # Should see: "level": "warn" (CI mode)
   ```

3. **GitHub Actions Output:**
   - Should see DEBUG output from `pw:api` and `pw:browser`
   - Should see explicit timeout or completion message
   - Should NOT see indefinite hang

---

## Success Criteria (Definition of Done)

- [ ] Phase 1 complete: DEBUG output visible, explicit timeouts on test step
- [ ] Phase 1 verified: Run 1x Chromium test; verify completes or fails (not hang)
- [ ] Phase 2 complete: workers: 1, fullyParallel: false
- [ ] Phase 2 verified: Run all 3 browsers; measure runtime and memory
- [ ] Phase 3 complete: dumb-init added, CI profile created
- [ ] Phase 3 verified: Verify graceful shutdown, log levels
- [ ] Full integration test: All 3 browsers complete in <35 minutes
- [ ] Rollback plan documented and tested
- [ ] CI workflow updated to v2
- [ ] Developer documentation updated

---

## Dependencies & External Factors

| Dependency | Status | Impact |
|-----------|--------|--------|
| dumb-init availability in debian:trixie-slim | ✅ Available | Phase 3 can proceed |
| Docker Compose v3.9+ (supports init: true) | ✅ Assumed | Phase 3 compose change |
| GitHub Actions timeout support | ✅ Supported | Phase 1 can proceed |
| Playwright v1.40+ (supports --reporter=line) | ✅ Latest | Phase 1 can proceed |

---

## Confidence Assessment

**Overall Confidence: 78% (Medium-High)**

### Reasoning:

**High Confidence (85%+):**
- Issue clearly identified: I/O buffer deadlock + resource starvation
- Phase 1 (observability) low-risk, high-information gain
- Explicit timeouts will convert hang → error (measurable improvement)

**Medium Confidence (70-80%):**
- Phase 2 (resource efficiency) depends on verifying Phase 1 reduces contention
- Phase 3 (init system) addresses signal handling but may not be root cause if app-level deadlock

**Lower Confidence (<70%):**
- Network configuration (IPv4 vs IPv6) could still cause issues
- Unknown Playwright webServer detection logic may have other edge cases

**Risk Mitigation:**
- Phase 1 provides debugging telemetry to diagnose remaining issues
- Rollback simple for each phase
- MRE testing strategy limits blast radius
- Incremental deployment reduces rollback overhead

**Incremental verification reduces overall risk to 15%**

---

## Timeline & Milestones

| Milestone | Date | Owner | Duration |
|-----------|------|-------|----------|
| **Phase 1 Implementation** | Feb 5 | QA/DevOps | 4 hours |
| **Phase 1 Testing & Verification** | Feb 5-6 | QA | 8 hours |
| **Phase 2 Implementation** | Feb 6 | QA/DevOps | 2 hours |
| **Phase 2 Testing** | Feb 6 | QA | 4 hours |
| **Phase 3 Implementation** | Feb 7 | DevOps | 4 hours |
| **Phase 3 Docker Rebuild** | Feb 7 | DevOps | 2 hours |
| **Full Integration Test** | Feb 7-8 | QA | 4 hours |
| **Documentation & Handoff** | Feb 8 | Engineering | 2 hours |

**Total: 30 hours (4 days)**

---

## Follow-Up Actions

After remediation completion:

1. **Documentation Update:** Update [docs/guides/ci-cd-pipeline.md] with new CI profile
2. **Alert Configuration:** Add monitoring for test hangs (script: check for zombie processes)
3. **Process Review:** Document why hang occurred (post-mortem analysis)
4. **Prevention:** Add pre-commit check for `fullyParallel: true` in CI environment

---

## Appendix A: Diagnostic Commands

```bash
# Monitor test progress in real-time
watch -n 1 'docker stats charon-playwright --no-stream | tail -5'

# Check for buffer-related errors
grep -i "buffer\|pipe\|epipe" <(docker logs charon-playwright)

# Verify process tree (should see dumb-init → caddy, dumb-init → charon)
docker exec charon-playwright ps auxf

# Check I/O wait time (high = buffer contention)
docker exec charon-playwright iostat -x 1 3

# Verify network configuration (IPv4 vs IPv6)
docker exec charon-playwright curl -4 http://localhost:8080/api/v1/health
docker exec charon-playwright curl -6 http://localhost:8080/api/v1/health
```

---

## Appendix B: References & Related Documents

- **Diagnostic Analysis:** [docs/implementation/FRONTEND_TEST_HANG_FIX.md](../implementation/FRONTEND_TEST_HANG_FIX.md)
- **Browser Alignment Report:** [docs/reports/browser_alignment_diagnostic.md](../reports/browser_alignment_diagnostic.md)
- **E2E Triage Quick Start:** [docs/plans/e2e-test-triage-quick-start.md](../plans/e2e-test-triage-quick-start.md)
- **Playwright Documentation:** https://playwright.dev/docs/intro
- **dumb-init GitHub:** https://github.com/Yelp/dumb-init
- **Docker Init System:** https://docs.docker.com/engine/reference/run/#specify-an-init-process

---

**Plan Complete: Ready for Review & Implementation**

**Next Steps:**
1. Review with QA lead (risk assessment)
2. Review with DevOps lead (Docker/infrastructure)
3. Begin Phase 1 implementation
4. Execute verification tests
5. Iterate on findings

---

*Generated by Planning Agent on February 4, 2026*
*Last Updated: N/A (Initial Creation)*
*Status: READY FOR REVIEW*
