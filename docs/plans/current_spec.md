# CrowdSec 1.7.5 Upgrade Verification Plan

**Document Type**: Verification Plan
**Version**: 1.7.4 → 1.7.5
**Created**: 2026-01-22
**Status**: Ready for Implementation

---

## Executive Summary

This document outlines the verification plan for upgrading CrowdSec from version 1.7.4 to 1.7.5 in the Charon project. Based on analysis of the CrowdSec 1.7.5 release notes and the current integration implementation, this upgrade appears to be a **low-risk maintenance release** focused on internal refactoring, improved error handling, and dependency updates.

---

## 1. CrowdSec 1.7.5 Release Analysis

### 1.1 Key Changes Summary

| Category | Count | Risk Level |
|----------|-------|------------|
| Internal Refactoring | ~25 | Low |
| Bug Fixes | 8 | Low |
| Dependency Updates | ~12 | Low |
| New Features | 2 | Low |

### 1.2 Notable Changes Relevant to Charon Integration

#### New Features/Improvements

1. **`ParseKVLax` for Flexible Key-Value Parsing** ([#4007](https://github.com/crowdsecurity/crowdsec/pull/4007))
   - Adds more flexible parsing capabilities
   - Impact: None - internal parser enhancement

2. **AppSec Transaction ID Header Support** ([#4124](https://github.com/crowdsecurity/crowdsec/pull/4124))
   - Enables request tracing via transaction ID header
   - Impact: Optional feature, no required changes

3. **Docker Datasource Schema** ([#4206](https://github.com/crowdsecurity/crowdsec/pull/4206))
   - Improved Docker acquisition configuration
   - Impact: May benefit container monitoring setups

#### Bug Fixes

1. **PAPI Allowlist Check** ([#4196](https://github.com/crowdsecurity/crowdsec/pull/4196))
   - Checks if decision is allowlisted before adding
   - Impact: Improved decision handling

2. **CAPI Token Reuse** ([#4201](https://github.com/crowdsecurity/crowdsec/pull/4201))
   - Always reuses stored token for CAPI
   - Impact: Better authentication stability

3. **LAPI-Only Container Hub Fix** ([#4169](https://github.com/crowdsecurity/crowdsec/pull/4169))
   - Don't prepare hub in LAPI-only containers
   - Impact: Better for containerized deployments

#### Internal Changes (No External Impact)

- Removed `github.com/pkg/errors` dependency - uses `fmt.Errorf` instead
- Replaced syscall with unix/windows packages
- Various linting improvements (golangci-lint 2.8)
- Refactored acquisition and leakybucket packages
- Removed global variables in favor of dependency injection
- Build improvements for Docker (larger runners)
- Updated expr to 1.17.7 (already patched in Charon Dockerfile)
- Updated modernc.org/sqlite

### 1.3 Breaking Changes Assessment

**No Breaking Changes Identified**

The 1.7.5 release contains no API-breaking changes. All modifications are:
- Internal refactoring
- Bug fixes
- Dependency updates
- CI/CD improvements

---

## 2. Current Charon CrowdSec Integration Analysis

### 2.1 Integration Points

| Component | Location | Description |
|-----------|----------|-------------|
| **Core Package** | [backend/internal/crowdsec/](backend/internal/crowdsec/) | CrowdSec integration library |
| **API Handler** | [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) | REST API endpoints |
| **Startup Service** | [backend/internal/services/crowdsec_startup.go](backend/internal/services/) | Initialization logic |
| **Dockerfile** | [Dockerfile](../../Dockerfile) (lines 199-290) | Source build configuration |

### 2.2 Key Files in crowdsec Package

| File | Purpose | Functions to Verify |
|------|---------|---------------------|
| `registration.go` | Bouncer registration, LAPI health | `EnsureBouncerRegistered`, `CheckLAPIHealth`, `GetLAPIVersion` |
| `hub_sync.go` | Hub index fetching, preset pull/apply | `FetchIndex`, `Pull`, `Apply`, `extractTarGz` |
| `hub_cache.go` | Preset caching with TTL | `Store`, `Load`, `Evict` |
| `console_enroll.go` | Console enrollment | `Enroll`, `Status`, `checkLAPIAvailable` |
| `presets.go` | Curated preset definitions | `ListCuratedPresets`, `FindPreset` |

### 2.3 Handler Functions (crowdsec_handler.go)

| Handler | Line | API Endpoint |
|---------|------|--------------|
| `Start` | 188 | POST /api/crowdsec/start |
| `Stop` | 290 | POST /api/crowdsec/stop |
| `Status` | 317 | GET /api/crowdsec/status |
| `ImportConfig` | 346 | POST /api/crowdsec/import |
| `ExportConfig` | 417 | GET /api/crowdsec/export |
| `ListFiles` | 486 | GET /api/crowdsec/files |
| `ReadFile` | 513 | GET /api/crowdsec/files/:path |
| `WriteFile` | 540 | PUT /api/crowdsec/files/:path |
| `ListPresets` | 580 | GET /api/crowdsec/presets |
| `PullPreset` | 662 | POST /api/crowdsec/presets/:slug/pull |
| `ApplyPreset` | 748 | POST /api/crowdsec/presets/:slug/apply |
| `ConsoleEnroll` | 876 | POST /api/crowdsec/console/enroll |
| `ConsoleStatus` | 932 | GET /api/crowdsec/console/status |
| `DeleteConsoleEnrollment` | 954 | DELETE /api/crowdsec/console/enrollment |
| `GetCachedPreset` | 975 | GET /api/crowdsec/presets/:slug |
| `GetLAPIDecisions` | 1077 | GET /api/crowdsec/lapi/decisions |
| `CheckLAPIHealth` | 1231 | GET /api/crowdsec/lapi/health |

### 2.4 Docker Configuration

**Dockerfile CrowdSec Section** (lines 199-290):
- Current version: `CROWDSEC_VERSION=1.7.4`
- Build method: Source compilation with Go 1.25.6
- Dependency patches applied:
  - `github.com/expr-lang/expr@v1.17.7`
  - `golang.org/x/crypto@v0.46.0`
- Fix for expr-lang v1.17.7 compatibility (sed replacement)

**Docker Compose Files**:
- `.docker/compose/docker-compose.yml` - Production config with crowdsec_data volume
- `.docker/compose/docker-compose.local.yml` - Local development
- `.docker/compose/docker-compose.playwright.yml` - E2E testing (crowdsec disabled)

---

## 3. Verification Checklist

### 3.1 Pre-Upgrade Verification

- [ ] **Backup current state**
  - Export current CrowdSec configuration
  - Document current bouncer registrations
  - Note current LAPI version from `/api/crowdsec/lapi/health`

- [ ] **Review dependency patches**
  - Verify if `expr-lang@v1.17.7` patch is still needed (1.7.5 updates to 1.17.7)
  - Check if `golang.org/x/crypto@v0.46.0` is still required

### 3.2 Dockerfile Update Checklist

- [ ] Update `CROWDSEC_VERSION=1.7.5` on line 213
- [ ] Update `CROWDSEC_VERSION=1.7.5` on line 267 (fallback stage)
- [ ] Verify expr-lang patch compatibility (line 228-235)
- [ ] Test multi-arch build (amd64, arm64)

### 3.3 Build Verification

```bash
# Build CrowdSec builder stage
docker build --target crowdsec-builder -t charon-crowdsec-test:1.7.5 .

# Verify binaries
docker run --rm charon-crowdsec-test:1.7.5 /crowdsec-out/cscli version
docker run --rm charon-crowdsec-test:1.7.5 /crowdsec-out/crowdsec -version

# Full image build
docker build -t charon:1.7.5-test .
```

### 3.4 Unit Test Verification

Run all CrowdSec-related tests:

```bash
# Core package tests
cd backend && go test -v -race ./internal/crowdsec/...

# Handler tests
go test -v -race ./internal/api/handlers/... -run "Crowdsec"

# Startup tests
go test -v -race ./internal/services/... -run "Crowdsec"
```

**Test Files to Execute**:
| Test File | Purpose |
|-----------|---------|
| `hub_sync_test.go` | Hub fetching and preset application |
| `hub_cache_test.go` | Cache TTL and eviction |
| `registration_test.go` | Bouncer registration |
| `console_enroll_test.go` | Console enrollment |
| `presets_test.go` | Curated preset definitions |
| `crowdsec_handler_test.go` | Handler integration |
| `crowdsec_lapi_test.go` | LAPI communication |
| `crowdsec_decisions_test.go` | Decision handling |
| `crowdsec_startup_test.go` | Service startup |

### 3.5 Integration Test Verification

```bash
# Run integration tests
cd backend && go test -v -tags=integration ./integration/... -run "Crowdsec"

# Run via task
make integration-crowdsec
```

### 3.6 E2E Test Verification

**Test Files**:
| Test File | Status | Purpose |
|-----------|--------|---------||
| `tests/security/crowdsec-config.spec.ts` | Active | CrowdSec configuration UI tests |
| `tests/security/crowdsec-decisions.spec.ts` | Skipped | LAPI decisions tests (requires running CrowdSec) |

> **Note**: `crowdsec-decisions.spec.ts` is currently skipped as it requires a running CrowdSec instance with LAPI enabled. These tests run in CI with full infrastructure.

```bash
# Run Playwright E2E tests (via skill runner - recommended)
.github/skills/scripts/skill-runner.sh test-e2e-playwright

# Or run specific test files
npx playwright test --project=chromium tests/security/crowdsec-config.spec.ts
```

### 3.7 Functional Verification Matrix

| Feature | Test Method | Expected Outcome |
|---------|-------------|------------------|
| **LAPI Health** | GET `/api/crowdsec/lapi/health` | Returns version "1.7.5" |
| **Start/Stop** | POST `/api/crowdsec/start`, `/stop` | Process starts/stops cleanly |
| **Status Check** | GET `/api/crowdsec/status` | Returns running state and PID |
| **Hub Index Fetch** | GET `/api/crowdsec/presets` | Returns preset list |
| **Preset Pull** | POST `/api/crowdsec/presets/base-http-scenarios/pull` | Downloads and caches preset |
| **Preset Apply** | POST `/api/crowdsec/presets/base-http-scenarios/apply` | Applies preset configuration |
| **Console Enroll** | POST `/api/crowdsec/console/enroll` | Sends enrollment request |
| **LAPI Decisions** | GET `/api/crowdsec/lapi/decisions` | Returns decision list |
| **Bouncer Registration** | Automatic on start | API key retrieved/generated |

### 3.8 Dependency Patch Verification

CrowdSec 1.7.5 includes `expr-lang/expr@v1.17.7` natively. Test whether the Dockerfile patch can be removed.

**Verification Steps**:

1. [ ] **Test WITHOUT expr-lang patch**:
   ```bash
   # Temporarily comment out expr-lang patch in Dockerfile (lines 225-229)
   # Build and run tests
   docker build --target crowdsec-builder -t charon-crowdsec-no-patch:test .
   docker run --rm charon-crowdsec-no-patch:test /crowdsec-out/cscli version
   ```

2. [ ] **If build succeeds without patch**:
   - Remove `go get github.com/expr-lang/expr@v1.17.7` line
   - Remove the `sed` fix for `program.Source().String()` if not needed
   - Keep `golang.org/x/crypto@v0.46.0` patch for security

3. [ ] **If build fails without patch**:
   - Retain the patch with updated comment noting it's still required
   - Document the specific error for future reference

4. [ ] **Validation**:
   - Run full test suite after patch removal
   - Verify no regression in CrowdSec functionality

---

## 4. Test Scenarios

### 4.1 Upgrade Smoke Test

```
WHEN the Docker image is built with CROWDSEC_VERSION=1.7.5
THEN the cscli version command reports v1.7.5
AND the crowdsec binary starts successfully
AND the LAPI health endpoint responds
```

### 4.2 Hub Sync Compatibility

```
WHEN hub index is fetched after upgrade
THEN the index format is parsed correctly
AND preset pull operations complete successfully
AND preset apply operations complete without errors
```

### 4.3 Console Enrollment Stability

```
WHEN console enrollment is attempted after upgrade
THEN LAPI availability check succeeds
AND CAPI registration works if needed
AND enrollment request is sent successfully
```

### 4.4 Decision API Compatibility

```
WHEN LAPI decisions are queried after upgrade
THEN the response format is unchanged
AND decisions are correctly parsed
AND filtering by scope/type works
```

### 4.5 Bouncer Registration

```
WHEN a new bouncer is registered after upgrade
THEN cscli bouncers add command succeeds
AND the bouncer appears in cscli bouncers list
AND the API key is correctly returned
```

---

## 5. Rollback Plan

### 5.1 Quick Rollback

If issues are encountered after upgrade:

1. **Revert Dockerfile**:
   ```bash
   git checkout HEAD~1 -- Dockerfile
   ```

2. **Rebuild with previous version**:
   ```bash
   docker build --build-arg CROWDSEC_VERSION=1.7.4 -t charon:rollback .
   ```

3. **Redeploy**:
   ```bash
   docker-compose -f .docker/compose/docker-compose.yml down
   docker-compose -f .docker/compose/docker-compose.yml up -d
   ```

### 5.2 Data Preservation

The `crowdsec_data` volume contains:
- Configuration files
- Acquired scenarios and parsers
- Decision database
- Bouncer registrations

This volume persists across container recreations, ensuring data is preserved during rollback.

---

## 6. Files Requiring Updates

### 6.1 Must Update

| File | Line(s) | Change |
|------|---------|--------|
| `Dockerfile` | 213 | `CROWDSEC_VERSION=1.7.4` → `1.7.5` |
| `Dockerfile` | 267 | `CROWDSEC_VERSION=1.7.4` → `1.7.5` |

### 6.2 May Require Review

| File | Reason |
|------|--------|
| `Dockerfile` (lines 228-235) | Verify expr-lang patch still needed |
| `docs/plans/crowdsec_source_build.md` | Update version reference |
| `docs/implementation/QUICK_FIX_SUPPLY_CHAIN.md` | Update version reference |

### 6.3 No Changes Required (Verified Compatible)

| File | Reason |
|------|--------|
| `backend/internal/crowdsec/*.go` | No API changes in 1.7.5 |
| `backend/internal/api/handlers/crowdsec_handler.go` | No API changes |
| `.docker/compose/*.yml` | Volume/env unchanged |

---

## 7. Dependency Analysis

### 7.1 Current Dockerfile Patches

```dockerfile
# Dockerfile lines 225-229
RUN go get github.com/expr-lang/expr@v1.17.7 && \
    go get golang.org/x/crypto@v0.46.0 && \
    go mod tidy
```

**1.7.5 Status**:
- CrowdSec 1.7.5 already includes `expr@v1.17.7` ([#4150](https://github.com/crowdsecurity/crowdsec/pull/4150))
- The `expr-lang` patch **may be removable** - verify during testing
- The `golang.org/x/crypto` patch should remain for security

### 7.2 Compatibility Fix

```dockerfile
# Dockerfile lines 232-235
RUN sed -i 's/string(program\.Source())/program.Source().String()/g' pkg/exprhelpers/debugger.go
```

**Verification Needed**: Check if this fix is still required in 1.7.5

---

## 8. Implementation Steps

### Phase 1: Preparation

1. [ ] Create feature branch: `feature/crowdsec-1.7.5-upgrade`
2. [ ] Run current test suite to establish baseline
3. [ ] Document current LAPI version and status

### Phase 2: Update

4. [ ] Update Dockerfile with version 1.7.5
5. [ ] Test build locally (amd64)
6. [ ] Test build for arm64 (if available)
7. [ ] Verify binaries report correct version

### Phase 3: Verification

8. [ ] Run E2E tests first: `.github/skills/scripts/skill-runner.sh test-e2e-playwright`
9. [ ] Run integration tests: `make integration-crowdsec`
10. [ ] Run unit tests: `make test-backend`
11. [ ] Run coverage verification: `make test-backend-coverage`
12. [ ] Manual API verification using functional matrix

### Phase 4: Documentation

12. [ ] Update version references in documentation
13. [ ] Update CHANGELOG.md
14. [ ] Create PR with test results

### Phase 5: Deployment

15. [ ] Merge to main
16. [ ] Monitor CI/CD pipeline
17. [ ] Verify production deployment
18. [ ] Monitor logs for any CrowdSec errors

---

## 9. Acceptance Criteria

The upgrade is considered successful when:

- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] All E2E tests pass
- [ ] LAPI reports version 1.7.5
- [ ] Start/Stop operations work correctly
- [ ] Hub preset operations complete successfully
- [ ] Console enrollment works (if applicable)
- [ ] No new errors in application logs
- [ ] Docker image builds successfully for amd64 and arm64
- [ ] Coverage report generated
- [ ] Coverage threshold ≥85% maintained
- [ ] Patch coverage 100% for Dockerfile modifications
- [ ] No new CodeQL alerts introduced

---

## 10. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Build failure | Low | Medium | Fallback stage in Dockerfile |
| API incompatibility | Very Low | High | Comprehensive test coverage |
| Performance regression | Low | Medium | Monitor via observability |
| expr-lang patch conflict | Low | Low | Test without patch first |
| Skipped E2E tests miss regression | Medium | Medium | Integration tests cover LAPI; CI runs full suite |

**Overall Risk Level**: **LOW**

The 1.7.5 release is a maintenance update with no breaking changes. The comprehensive test coverage in Charon provides high confidence in upgrade success.

---

## Appendix A: Quick Reference Commands

```bash
# Build and test
make docker-build
make test-backend
make test-crowdsec

# Run specific test files
cd backend && go test -v ./internal/crowdsec/... -run TestHub
cd backend && go test -v ./internal/crowdsec/... -run TestConsole
cd backend && go test -v ./internal/crowdsec/... -run TestRegistration

# Integration tests
.github/skills/scripts/skill-runner.sh integration-crowdsec

# E2E tests
npx playwright test --project=chromium

# Check CrowdSec version in container
docker exec charon cscli version
docker exec charon curl -s http://127.0.0.1:8085/health

# LAPI health check
curl http://localhost:8080/api/crowdsec/lapi/health
```

---

## Appendix B: Related Documentation

- [CrowdSec 1.7.5 Release Notes](https://github.com/crowdsecurity/crowdsec/releases/tag/v1.7.5)
- [CrowdSec Source Build Plan](crowdsec_source_build.md)
- [Supply Chain Remediation](../implementation/SUPPLY_CHAIN_REMEDIATION_PLAN.md)
- [Charon Security Documentation](../../SECURITY.md)
