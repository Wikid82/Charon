# E2E CI Failure Diagnosis - 100% Failure vs 90% Pass Local

**Date**: February 4, 2026
**Status**: 🔴 CRITICAL - 100% CI failure rate vs 90% local pass rate
**Urgency**: HIGH - Blocking all PRs and CI/CD pipeline

---

## Executive Summary

**Problem**: E2E tests exhibit a critical environmental discrepancy:
- **Local Environment**: 90% of E2E tests PASS when running via `skill-runner.sh test-e2e-playwright`
- **CI Environment**: 100% of E2E jobs FAIL in GitHub Actions workflow (`e2e-tests-split.yml`)

**Root Cause Hypothesis**: Multiple critical configuration differences between local and CI environments create an inconsistent test execution environment, leading to systematic failures in CI.

**Impact**:
- ❌ All PRs blocked due to failing E2E checks
- ❌ Cannot merge to `main` or `development`
- ❌ CI/CD pipeline completely stalled
- ⚠️ Development velocity severely impacted

---

## Configuration Comparison Matrix

### Docker Compose Configuration Differences

| Configuration | Local (`docker-compose.playwright-local.yml`) | CI (`docker-compose.playwright-ci.yml`) | Impact |
|---------------|----------------------------------------------|----------------------------------------|---------|
| **Environment** | `CHARON_ENV=e2e` | `CHARON_ENV=test` | 🔴 **HIGH** - Different runtime behavior |
| **Credential Source** | `env_file: ../../.env` | Environment variables from `$GITHUB_ENV` | 🟡 **MEDIUM** - Potential missing vars |
| **Encryption Key** | Loaded from `.env` file | Generated ephemeral: `openssl rand -base64 32` | 🟢 **LOW** - Both valid |
| **Emergency Token** | Loaded from `.env` file | From GitHub Secrets (`CHARON_EMERGENCY_TOKEN`) | 🟡 **MEDIUM** - Potential missing/invalid token |
| **Security Tests Flag** | ❌ **NOT SET** | ✅ `CHARON_SECURITY_TESTS_ENABLED=true` | 🔴 **CRITICAL** - May enable security modules |
| **Data Storage** | `tmpfs: /app/data` (in-memory, ephemeral) | Named volumes (`playwright_data`, etc.) | 🟡 **MEDIUM** - Different persistence behavior |
| **Security Profile** | ❌ Not enabled by default | ✅ `--profile security-tests` (enables CrowdSec) | 🔴 **CRITICAL** - Different security modules active |
| **Image Source** | `charon:local` (fresh local build) | `charon:e2e-test` (loaded from artifact) | 🟢 **LOW** - Both should be identical builds |
| **Container Name** | `charon-e2e` | `charon-playwright` | 🟢 **LOW** - Cosmetic difference |

### GitHub Actions Workflow Environment

| Variable | CI Value | Local Equivalent | Impact |
|----------|----------|------------------|--------|
| `CI` | `true` | Not set | 🟡 **MEDIUM** - Playwright retries, workers, etc. |
| `PLAYWRIGHT_BASE_URL` | `http://localhost:8080` | `http://localhost:8080` | 🟢 **LOW** - Identical |
| `PLAYWRIGHT_COVERAGE` | `0` (disabled by default) | `0` | 🟢 **LOW** - Identical |
| `CHARON_EMERGENCY_SERVER_ENABLED` | `true` | `true` | 🟢 **LOW** - Identical |
| `CHARON_EMERGENCY_BIND` | `0.0.0.0:2020` | `0.0.0.0:2020` | 🟢 **LOW** - Identical |
| `NODE_VERSION` | `20` | User-dependent | 🟡 **MEDIUM** - May differ |
| `GO_VERSION` | `1.25.6` | User-dependent | 🟡 **MEDIUM** - May differ |

### Local Test Execution Flow

**User runs E2E tests locally:**

```bash
# Step 1: Rebuild E2E container (CRITICAL: user must do this)
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# Default behavior: NO security profile enabled
# Result: CrowdSec NOT running
# CHARON_SECURITY_TESTS_ENABLED: NOT SET

# Step 2: Run tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

**What's missing locally:**
1. ❌ No `--profile security-tests` (CrowdSec not running)
2. ❌ No `CHARON_SECURITY_TESTS_ENABLED` environment variable
3. ❌ `CHARON_ENV=e2e` instead of `CHARON_ENV=test`
4. ✅ Uses `.env` file (requires user to have created it)

### CI Test Execution Flow

**GitHub Actions runs E2E tests:**

```yaml
# Step 1: Generate ephemeral encryption key
- name: Generate ephemeral encryption key
  run: echo "CHARON_ENCRYPTION_KEY=$(openssl rand -base64 32)" >> $GITHUB_ENV

# Step 2: Validate emergency token
- name: Validate Emergency Token Configuration
  # Checks CHARON_EMERGENCY_TOKEN from secrets

# Step 3: Start with security-tests profile
- name: Start test environment
  run: |
    docker compose -f .docker/compose/docker-compose.playwright-ci.yml --profile security-tests up -d

# Environment variables in workflow:
env:
  CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
  CHARON_EMERGENCY_SERVER_ENABLED: "true"
  CHARON_SECURITY_TESTS_ENABLED: "true"  # ← SET IN CI
  CHARON_E2E_IMAGE_TAG: charon:e2e-test

# Step 4: Wait for health check (30 attempts, 2s interval)

# Step 5: Run tests with sharding
npx playwright test --project=chromium --shard=1/4
```

**What's different in CI:**
1. ✅ `--profile security-tests` enabled (CrowdSec running)
2. ✅ `CHARON_SECURITY_TESTS_ENABLED=true` explicitly set
3. ✅ `CHARON_ENV=test` (not `e2e`)
4. ✅ Named volumes (persistent data within workflow run)
5. ✅ Sharding enabled (4 shards per browser)

---

## Root Cause Analysis

### Critical Difference #1: CHARON_ENV (e2e vs test)

**Evidence**: Local uses `CHARON_ENV=e2e`, CI uses `CHARON_ENV=test`

**Behavior Difference**:
Looking at `backend/internal/caddy/config.go:92`:
```go
isE2E := os.Getenv("CHARON_ENV") == "e2e"

if acmeEmail != "" || isE2E {
    // E2E environment allows certificate generation without email
}
```

**Impact**: The application may behave differently in rate limiting, certificate generation, or other environment-specific logic depending on this variable.

**Severity**: 🔴 **HIGH** - Fundamental environment difference

**Hypothesis**: If there's rate limiting logic checking for `CHARON_ENV == "e2e"` to provide lenient limits, the CI environment with `CHARON_ENV=test` may enforce stricter limits, causing test failures.

### Critical Difference #2: CHARON_SECURITY_TESTS_ENABLED

**Evidence**: NOT set locally, explicitly set to `"true"` in CI

**Where it's set**:
- CI Workflow: `CHARON_SECURITY_TESTS_ENABLED: "true"` in env block
- CI Compose: `CHARON_SECURITY_TESTS_ENABLED=${CHARON_SECURITY_TESTS_ENABLED:-true}`
- Local Compose: ❌ **NOT PRESENT**

**Impact**: **UNKNOWN** - This variable is NOT used anywhere in the backend Go code (confirmed by grep search). However, it may:
1. Be checked in the frontend TypeScript code
2. Control test fixture behavior
3. Be a vestigial variable that was removed from code but left in compose files

**Severity**: 🟡 **MEDIUM** - Present in CI but not local, unexplained purpose

**Action Required**: Search frontend and test fixtures for usage of this variable.

### Critical Difference #3: Security Profile (CrowdSec)

**Evidence**: CI runs with `--profile security-tests`, local does NOT (unless manually specified)

**Impact**:
- **CI**: CrowdSec container running alongside `charon-app`
- **Local**: No CrowdSec (unless user runs `docker-rebuild-e2e --profile=security-tests`)

**CrowdSec Service Configuration**:
```yaml
crowdsec:
  image: crowdsecurity/crowdsec:latest
  profiles:
    - security-tests
  environment:
    - COLLECTIONS=crowdsecurity/nginx crowdsecurity/http-cve
    - BOUNCER_KEY_charon=test-bouncer-key-for-e2e
    - DISABLE_ONLINE_API=true
```

**Severity**: 🔴 **CRITICAL** - Entire security module missing locally

**Hypothesis**: Tests may be failing in CI because:
1. CrowdSec is blocking requests that should pass
2. CrowdSec has configuration issues in CI environment
3. Tests are written assuming CrowdSec is NOT running
4. Network routing through CrowdSec causes latency or timeouts

### Critical Difference #4: Data Storage (tmpfs vs named volumes)

**Evidence**:
- Local: `tmpfs: /app/data:size=100M,mode=1777` (in-memory, cleared on restart)
- CI: Named volumes `playwright_data`, `playwright_caddy_data`, `playwright_caddy_config`

**Impact**:
- **Local**: True ephemeral storage - every restart is 100% fresh
- **CI**: Volumes persist across container restarts within the same workflow run

**Severity**: 🟡 **MEDIUM** - Could cause state pollution in CI

**Hypothesis**: If CI containers are restarted mid-workflow (e.g., between shards), the volumes retain data, potentially causing state pollution that doesn't exist locally.

### Critical Difference #5: Credential Management

**Evidence**:
- Local: Uses `env_file: ../../.env` to load all credentials
- CI: Passes credentials explicitly via `$GITHUB_ENV` and secrets

**Failure Scenario**:
1. User creates `.env` file with `CHARON_ENCRYPTION_KEY` and `CHARON_EMERGENCY_TOKEN`
2. Local tests pass because both variables are loaded from `.env`
3. CI generates ephemeral `CHARON_ENCRYPTION_KEY` (always fresh)
4. CI loads `CHARON_EMERGENCY_TOKEN` from GitHub Secrets

**Potential Issues**:
- ❓ Is `CHARON_EMERGENCY_TOKEN` correctly configured in GitHub Secrets?
- ❓ Is the token length validation passing in CI? (requires ≥64 characters)
- ❓ Are there any other variables loaded from `.env` locally that are missing in CI?

**Severity**: 🔴 **HIGH** - Credential mismatches can cause authentication failures

---

## Suspected Failure Scenarios

### Scenario A: CrowdSec Blocking Legitimate Test Requests

**Hypothesis**: CrowdSec in CI is blocking test requests that would pass locally without CrowdSec.

**Evidence Needed**:
1. Docker logs from CrowdSec container in failed CI runs
2. Charon application logs showing blocked requests
3. Test failure patterns (are they authentication/authorization related?)

**Test**:
Run locally with security-tests profile:
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --profile=security-tests
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

**Expected**: If this is the root cause, tests will fail locally with the profile enabled.

### Scenario B: CHARON_ENV=test Enforces Stricter Limits

**Hypothesis**: The `test` environment enforces production-like limits (rate limiting, timeouts) that break tests designed for lenient `e2e` environment.

**Evidence Needed**:
1. Search backend code for all uses of `CHARON_ENV`
2. Identify rate limiting, timeout, or other behavior differences
3. Check if tests make rapid API calls that would hit rate limits

**Test**:
Modify local compose to use `CHARON_ENV=test`:
```yaml
# .docker/compose/docker-compose.playwright-local.yml
environment:
  - CHARON_ENV=test  # Change from e2e
```

**Expected**: If this is the root cause, tests will fail locally with `CHARON_ENV=test`.

### Scenario C: Missing Environment Variable in CI

**Hypothesis**: The CI environment is missing a critical environment variable that's loaded from `.env` locally but not set in CI compose/workflow.

**Evidence Needed**:
1. Compare `.env.example` with all variables explicitly set in `docker-compose.playwright-ci.yml` and the workflow
2. Check application startup logs for warnings about missing environment variables
3. Review test failure messages for configuration errors

**Test**:
Audit all environment variables:
```bash
# Local container
docker exec charon-e2e env | sort > local-env.txt

# CI container (from failed run logs)
# Download docker logs artifact and extract env vars
```

### Scenario D: Image Build Differences (Local vs CI Artifact)

**Hypothesis**: The Docker image built locally (`charon:local`) differs from the CI artifact (`charon:e2e-test`) in some way that causes test failures.

**Evidence Needed**:
1. Compare Dockerfile build args between local and CI
2. Inspect image layers to identify differences
3. Check if CI cache is corrupted

**Test**:
Load the CI artifact locally and run tests against it:
```bash
# Download artifact from failed CI run
# Load image: docker load -i charon-e2e-image.tar
# Run tests against CI artifact locally
```

---

## Diagnostic Action Plan

### Phase 1: Evidence Collection (Immediate)

**Task 1.1**: Download recent failed CI run artifacts
- [ ] Download Docker logs from latest failed run
- [ ] Download test traces and videos
- [ ] Download HTML test reports

**Task 1.2**: Capture local environment baseline
```bash
# With default settings (passing tests)
docker exec charon-e2e env | sort > local-env-baseline.txt
docker logs charon-e2e > local-logs-baseline.txt
```

**Task 1.3**: Search for CHARON_SECURITY_TESTS_ENABLED usage
```bash
# Frontend
grep -r "CHARON_SECURITY_TESTS_ENABLED" frontend/

# Tests
grep -r "CHARON_SECURITY_TESTS_ENABLED" tests/

# Backend (already confirmed: NOT USED)
```

**Task 1.4**: Document test failure patterns in CI
- [ ] Review last 10 failed CI runs
- [ ] Identify common error messages
- [ ] Check if specific tests always fail
- [ ] Check if failures are random or deterministic

### Phase 2: Controlled Experiments (Next)

**Experiment 2.1**: Enable security-tests profile locally
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --profile=security-tests --clean
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

**Expected Outcome**: If CrowdSec is the root cause, tests will fail locally.

**Experiment 2.2**: Change CHARON_ENV to "test" locally
```bash
# Edit .docker/compose/docker-compose.playwright-local.yml
# Change: CHARON_ENV=e2e → CHARON_ENV=test
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

**Expected Outcome**: If environment-specific behavior differs, tests will fail locally.

**Experiment 2.3**: Add CHARON_SECURITY_TESTS_ENABLED locally
```bash
# Edit .docker/compose/docker-compose.playwright-local.yml
# Add: - CHARON_SECURITY_TESTS_ENABLED=true
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

**Expected Outcome**: If this flag controls critical behavior, tests may fail locally.

**Experiment 2.4**: Use named volumes instead of tmpfs locally
```bash
# Edit .docker/compose/docker-compose.playwright-local.yml
# Replace tmpfs with named volumes matching CI config
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
.github/skills/scripts/skill-runner.sh test-e2e-playwright
```

**Expected Outcome**: If volume persistence causes state pollution, tests may behave differently.

### Phase 3: CI Simplification (Final)

If experiments identify the root cause, apply corresponding fix to CI:

**Fix 3.1**: Remove security-tests profile from CI (if CrowdSec is the culprit)
```yaml
# .github/workflows/e2e-tests-split.yml
- name: Start test environment
  run: |
    docker compose -f .docker/compose/docker-compose.playwright-ci.yml up -d
    # Remove: --profile security-tests
```

**Fix 3.2**: Align CI environment to match local (if CHARON_ENV is the issue)
```yaml
# .docker/compose/docker-compose.playwright-ci.yml
environment:
  - CHARON_ENV=e2e  # Change from test to e2e
```

**Fix 3.3**: Remove CHARON_SECURITY_TESTS_ENABLED (if unused)
```yaml
# Remove from workflow and compose if truly unused
```

**Fix 3.4**: Use tmpfs in CI (if volume persistence is the issue)
```yaml
# .docker/compose/docker-compose.playwright-ci.yml
tmpfs:
  - /app/data:size=100M,mode=1777
# Remove: playwright_data volume
```

---

## Investigation Priorities

### 🔴 **CRITICAL** - Investigate First

1. **CrowdSec Profile Difference**
   - CI runs with CrowdSec, local does not (by default)
   - Most likely root cause of 100% failure rate
   - **Action**: Run Experiment 2.1 immediately

2. **CHARON_ENV Difference (e2e vs test)**
   - Known to affect application behavior (rate limiting, etc.)
   - **Action**: Run Experiment 2.2 immediately

3. **Emergency Token Validation**
   - CI validates token length (≥64 chars)
   - Local loads from `.env` (unchecked)
   - **Action**: Review CI logs for token validation failures

### 🟡 **MEDIUM** - Investigate Next

4. **CHARON_SECURITY_TESTS_ENABLED Purpose**
   - Set in CI, not in local
   - Not used in backend Go code
   - **Action**: Search frontend/tests for usage

5. **Named Volumes vs tmpfs**
   - CI uses persistent volumes
   - Local uses ephemeral tmpfs
   - **Action**: Run Experiment 2.4 to test state pollution theory

6. **Image Build Differences**
   - Local builds fresh, CI loads from artifact
   - **Action**: Load CI artifact locally and compare

### 🟢 **LOW** - Investigate Last

7. **Node.js/Go Version Differences**
   - Unlikely to cause 100% failure
   - More likely to cause flaky tests, not systematic failures

8. **Sharding Differences**
   - CI uses sharding (4 shards per browser)
   - Local runs all tests in single process
   - **Action**: Test with sharding locally

---

## Success Criteria for Resolution

**Definition of Done**: CI environment matches local environment in all critical configuration aspects, resulting in:

1. ✅ CI E2E tests pass at ≥90% rate (matching local)
2. ✅ Root cause identified and documented
3. ✅ Configuration differences eliminated or explained
4. ✅ Reproducible test environment (local = CI)
5. ✅ All experiments documented with results
6. ✅ Runbook created for future E2E debugging

**Rollback Plan**: If fixes introduce new issues, revert changes and document findings for deeper investigation.

---

## References

**Files to Review**:
- `.github/workflows/e2e-tests-split.yml` - CI workflow configuration
- `.docker/compose/docker-compose.playwright-ci.yml` - CI docker compose
- `.docker/compose/docker-compose.playwright-local.yml` - Local docker compose
- `.github/skills/scripts/skill-runner.sh` - Skill runner orchestration
- `.github/skills/test-e2e-playwright-scripts/run.sh` - Local test execution
- `.github/skills/docker-rebuild-e2e-scripts/run.sh` - Local container rebuild
- `backend/internal/caddy/config.go` - CHARON_ENV usage
- `playwright.config.js` - Playwright test configuration

**Related Documentation**:
- `.github/instructions/testing.instructions.md` - Test protocols
- `.github/instructions/playwright-typescript.instructions.md` - Playwright guidelines
- `docs/reports/gh_actions_diagnostic.md` - Previous CI failure analysis

**GitHub Actions Runs** (recent failures):
- Check Actions tab for latest failed runs on `e2e-tests-split.yml`
- Download artifacts: Docker logs, test reports, traces

---

**Next Action**: Execute Phase 1 evidence collection, focusing on CrowdSec profile and CHARON_ENV differences as primary suspects.

**Assigned To**: Supervisor Agent (for review and approval of diagnostic experiments)

**Timeline**:
- Phase 1 (Evidence): 1-2 hours
- Phase 2 (Experiments): 2-4 hours
- Phase 3 (Fixes): 1-2 hours
- **Total Estimated Time**: 4-8 hours to resolution

---

*Diagnostic Plan Generated: February 4, 2026*
*Author: GitHub Copilot (Planning Mode)*
