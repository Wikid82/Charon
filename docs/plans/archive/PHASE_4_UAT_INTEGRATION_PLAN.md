# Phase 4: UAT & Integration Testing Plan

**Date:** February 10, 2026
**Status:** READY FOR EXECUTION ✅
**Confidence Level:** 85% (based on Phase 2-3 baseline + test creation complete)
**Estimated Duration:** 2-3 hours (execution only; test writing completed)
**Test Coverage:** 110 comprehensive test cases (70 UAT + 40 integration)
**Test Files Status:** ✅ All test suites created and ready in `/tests/phase4-*/`
**Note:** Regression tests (CVE/upstream dependency tracking) handled by CI security jobs, not Phase 4 UAT

---

## Executive Summary

Phase 4 is the final validation milestone before production beta release. This plan provides:

1. **User Acceptance Testing (UAT)** - 70 real-world workflow tests validating that end-users can perform all major operations
2. **Integration Testing** - 40 multi-component tests ensuring system components work together correctly
3. **Production Readiness** - Final checklists and go/no-go gates before beta launch

**Note on Security Regression Testing:** CVE tracking and upstream dependency regression testing is handled by dedicated CI jobs (Trivy scans, security integration tests run with modules enabled). Phase 4 UAT focuses on feature validation with security modules disabled for isolated testing.

### Success Criteria

- ✅ All 70 UAT tests passing (core workflows functional)
- ✅ All 40 integration tests passing (components working together)
- ✅ Zero CRITICAL/HIGH security vulnerabilities (Trivy scan)
- ✅ Production readiness checklist 100% complete
- ✅ Performance metrics within acceptable ranges
- ✅ Documentation updated and complete

### Go/No-Go Decision

| Outcome | Criteria | Action |
|---------|----------|--------|
| **GO** | All tests passing, zero CRITICAL/HIGH vulns, checklist complete | Proceed to beta release |
| **CONDITIONAL** | Minor issues found (non-blocking tests), easy remediation | Fix issues, retest, then GO |
| **NO-GO** | Critical security issues, major test failures, architectural problems | Stop, remediate, restart Phase 4 |

---

## Risk Assessment & Mitigation

### TOP 3 IDENTIFIED RISKS

#### Risk #1: Test Implementation Delay → **RESOLVED ✅**
- **Status:** All 110 test files created and ready (70 UAT + 40 integration)
- **Previous Impact:** Could extend Phase 4 from 3-5 hours → 18-25 hours
- **Mitigation Applied:** Pre-write all test suites before Phase 4 execution
- **Current Impact:** ZERO - Tests ready to run

#### Risk #2: Security Module Regression (Handled by CI)
- **Status:** MITIGATED - Delegated to CI security jobs
- **Impact:** Phase 3 security modules could be broken by future changes
- **Mitigation:**
  1. Dedicated CI jobs test security with modules **enabled**: `cerberus-integration.yml`, `waf-integration.yml`, `rate-limit-integration.yml`, `crowdsec-integration.yml`
  2. CVE tracking and upstream dependency changes monitored by Trivy + security scanning
  3. Phase 4 UAT runs with security modules **disabled** for isolated feature testing
  4. Security regression detection: Automated by CI pipeline, not Phase 4 responsibility
- **Monitoring:** CI security jobs run on each commit; Phase 4 focuses on feature validation

#### Risk #3: Concurrency Issues Missed by Low Test Concurrency
- **Status:** MITIGATED - Increased concurrency levels
- **Impact:** Production race conditions not caught in testing
- **Mitigation Applied:** Updated INT-407, INT-306 to use 20+ and 50+ concurrent operations respectively
- **Current Status:** Updated in integration test files (INT-407: 2→20, INT-306: 5→50)

### STOP-AND-INVESTIGATE TRIGGERS

```
IF UAT tests <80% passing THEN
  PAUSE and categorize failures:
    - CRITICAL (auth, login, core ops): Fix immediately, re-test
    - IMPORTANT (features): Documented as known issue, proceed
    - MINOR (UI, formatting): Noted for Phase 5, proceed
END IF

IF Integration tests have race condition failures THEN
  Increase concurrency further, re-run
  May indicate data consistency issue
END IF

IF ANY CRITICAL or HIGH security vulnerability discovered THEN
  STOP Phase 4
  Remediate vulnerability
  Re-run security scan
  Do NOT proceed until 0 CRITICAL/HIGH
END IF

IF Response time >2× baseline (e.g., Login >300ms vs <150ms baseline) THEN
  INVESTIGATE:
    - Database performance issue
    - Memory leak during test
    - Authentication bottleneck
  Optimize and re-test
END IF
```

---

## Performance Baselines

### Target Response Times (Critical Paths)

| Endpoint | Operation | Target (P99) | Measurement Method | Alert Threshold |
|----------|-----------|--------------|-------------------|------------------|
| `/api/v1/auth/login` | Authentication | <150ms | Measure request → response | >250ms (fail) |
| `/api/v1/users` | List users | <250ms | GET with pagination | >400ms (investigate) |
| `/api/v1/proxy-hosts` | List proxies | <250ms | GET with pagination | >400ms (investigate) |
| `/api/v1/users/invite` | Create user invite | <200ms | Was 5-30s; verify async | >300ms (regressed) |
| `/api/v1/domains` | List domains | <300ms | GET full list | >500ms (investigate) |
| `/api/v1/auth/refresh` | Token refresh | <50ms | POST to refresh endpoint | >150ms (investigate) |
| Backup Operation | Create full backup | <5min | Time from start → complete | >8min (investigate) |
| Restore Operation | Restore from backup | <10min | Time from restore → operational | >15min (investigate) |

### Resource Usage Baselines

| Metric | Target | Max Alert | Measurement |
|--------|--------|-----------|-------------|
| Memory Usage | <500MB steady | >750MB | `docker stats charon-e2e` during tests |
| CPU Usage (peak) | <70% | >90% | `docker stats` or `top` in container |
| Database Size | <1GB | >2GB | `du -sh /var/lib/postgresql` |
| Disk I/O | Normal patterns | >80% I/O wait | `iostat` during test |

### Measurement Implementation

```typescript
// In each test, measure response time:
const start = performance.now();
const response = await page.request.get('/api/v1/users');
const duration = performance.now() - start;
console.log(`API Response time: ${duration.toFixed(2)}ms`);

// Expected: ~100-250ms
// Alert if: >400ms
```

### Performance Regression Detection

**If any metric exceeds baseline by 2×:**
1. Run again to rule out transient slowdown
2. Check system load (other containers running?)
3. Profile: Database query slowness? API endpoint issue? Network latency?
4. If confirmed regression: Stop Phase 4, optimize, re-test

---

## Browser Testing Strategy

### Scope & Rationale

**Phase 4 Browser Coverage: Firefox ONLY (Primary QA Browser)**

```
Browser Testing Strategy:
├── Regression Tests (Phase 2.3 + 3)
│   └── Firefox only (API-focused, not browser-specific)
│       └── Rationale: Security/performance tests, browser variant not critical
│
├── UAT Tests (User workflows)
│   └── Firefox only (primary user flow validation)
│       └── Rationale: Test coverage in hours; multi-browser testing deferred to Phase 5
│       └── Exception: Top 5 critical UAT tests (login, user create, proxy create, backup, security)
│           may spot-check on Chrome if time permits
│
├── Integration Tests (Component interactions)
│   └── Firefox only (API-focused, data consistency tests)
│       └── Rationale: Not visual/rendering tests; browser independence assumed
│
└── NOT in Phase 4 scope (defer to Phase 5):
    ├── Chrome (Chromium-based compliance)
    ├── Safari/WebKit (edge cases, rendering)
    ├── Mobile browser testing
    └── Low-bandwidth scenarios
```

### Execution Command

**Phase 4 Test Execution (Firefox only):**
```bash
cd /projects/Charon
echo "Step 1: Rebuild E2E environment"
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

echo "Step 2: Run regression tests"
npx playwright test tests/phase4-regression/ --project=firefox

echo "Step 3: Run UAT tests"
npx playwright test tests/phase4-uat/ --project=firefox

echo "Step 4: Run integration tests (serial for concurrency tests)"
npx playwright test tests/phase4-integration/ --project=firefox --workers=1
```

### Why NOT Multi-Browser in Phase 4?

| Reason | Justification | Timeline Impact |
|--------|---------------|------------------|
| **Time constraint** | 145 tests × 3 browsers = 435 test runs (would extend 3-5 hr to 9-15 hrs) | Would exceed Phase 4 window |
| **Test focus** | UAT/Integration are functional, not rendering-specific | Browser variance minimal for these tests |
| **CI/CD already validates** | `.github/workflows/` runs multi-browser tests post-Phase4 | Redundant in Phase 4 |
| **MVP scope** | Phase 4 is feature validation, Phase 5 is cross-browser hardening | Proper sequencing |
| **Production**: Phase 5 will include| Chrome/Safari spot-checks + full multi-browser CI | Comprehensive coverage post-Phase 4 |

### If Additional Browsers Needed:

**Chrome spot-check (5 critical tests only):**
```bash
# ONLY if Phase 4 ahead of schedule:
npx playwright test tests/phase4-uat/02-user-management.spec.ts --project=chromium
npx playwright test tests/phase4-uat/03-proxy-host-management.spec.ts --project=chromium
npx playwright test tests/phase4-uat/07-backup-recovery.spec.ts --project=chromium
npx playwright test tests/phase4-integration/01-admin-user-e2e-workflow.spec.ts --project=chromium
npx playwright test tests/phase4-regression/phase-3-security-gates.spec.ts --project=chromium
```

**WebKit (NOT recommended for Phase 4):** Deferred to Phase 5

---

## Updated Test File Locations

### Test Files Created & Ready ✅

**Location:** `/projects/Charon/tests/phase4-*/`

```
tests/
├── phase4-uat/                          (70 UAT tests, 8 feature areas)
│   ├── 01-admin-onboarding.spec.ts      (8 tests)
│   ├── 02-user-management.spec.ts       (10 tests)
│   ├── 03-proxy-host-management.spec.ts (12 tests)
│   ├── 04-security-configuration.spec.ts (10 tests)
│   ├── 05-domain-dns-management.spec.ts (8 tests)
│   ├── 06-monitoring-audit.spec.ts      (8 tests)
│   ├── 07-backup-recovery.spec.ts       (9 tests)
│   ├── 08-emergency-operations.spec.ts  (5 tests)
│   └── README.md
│
├── phase4-integration/                  (40 integration tests, 7 scenarios)
│   ├── 01-admin-user-e2e-workflow.spec.ts      (7 tests)
│   ├── 02-waf-ratelimit-interaction.spec.ts    (5 tests)
│   ├── 03-acl-waf-layering.spec.ts             (4 tests)
│   ├── 04-auth-middleware-cascade.spec.ts      (6 tests)
│   ├── 05-data-consistency.spec.ts             (8 tests)
│   ├── 06-long-running-operations.spec.ts      (5 tests)
│   ├── 07-multi-component-workflows.spec.ts    (5 tests)
│   └── README.md

TOTAL: 110 tests ready to execute
NOTES:
  - Security/CVE regression testing (Phase 2.3 fixes, Phase 3 gates) handled by CI jobs
  - Trivy scans and security integration tests run on each commit with modules enabled
  - Phase 4 focuses on feature UAT and data consistency with security modules disabled
```

---

## Execution Strategy

### Test Execution Order

Phase 4 tests should execute in this order to catch issues early:

1. **UAT Tests** (70 tests) - ~60-90 min
   - ✅ Admin Onboarding (8 tests) - ~10 min
   - ✅ User Management (10 tests) - ~15 min
   - ✅ Proxy Hosts (12 tests) - ~20 min
   - ✅ Security Configuration (10 tests) - ~15 min
   - ✅ Domain Management (8 tests) - ~15 min
   - ✅ Monitoring & Audit (8 tests) - ~10 min
   - ✅ Backup & Recovery (9 tests) - ~15 min
   - ✅ Emergency Operations (5 tests) - ~8 min

2. **Integration Tests** (40 tests) - ~45-60 min
   - Multi-component workflows
   - Data consistency verification
   - Long-running operations
   - Error handling across layers

3. **Production Readiness Checklist** - ~30 min
   - Manual verification of 45 checklist items
   - Documentation review
   - Security spot-checks

**Note:** Phase 2.3 fixes and Phase 3 security gates are validated by CI security jobs (run with security modules enabled), not Phase 4 UAT.

### Parallelization Strategy

To reduce total execution time, run independent suites in parallel where possible:

**Can Run Parallel:**
- UAT suites are independent (no data dependencies)
- Different user roles can test simultaneously
- Integration tests can run after UAT
- Regression tests must run first (sequential)

**Must Run Sequential:**
- Phase 2.3 regression (validates state before other tests)
- Phase 3 regression (validates state before UAT)
- Auth/user tests before authorization tests
- Setup operations before dependent tests

**Recommended Parallelization:**
```
Phase 2.3 Regression    [████████████] 15 min (sequential)
Phase 3 Regression      [████████████] 10 min (sequential)
UAT Suite A & B         [████████████] 50 min (parallel, 2 workers)
UAT Suite C & D         [████████████] 50 min (parallel, 2 workers)
Integration Tests       [████████████] 60 min (4 parallel suites)
─────────────────────────────────────────────
Total Time: ~90 min (vs. 225 min if fully sequential)
```

### Real-Time Monitoring

Monitor these metrics during Phase 4 execution:

| Metric | Target | Action if Failed |
|--------|--------|------------------|
| Test Pass Rate | >95% | Stop, investigate failure |
| API Response Time | <200ms p99 | Check performance, DB load |
| Error Rate | <0.1% | Check logs for errors |
| Security Events Blocked | >0 (if attacked) | Verify WAF/ACL working |
| Audit Log Entries | >100 per hour | Check audit logging active |
| Memory Usage | <500MB | Monitor for leaks |
| Database Size | <1GB | Check for unexpected growth |

### Success/Fail Decision Criteria

**PASS (Proceed to Beta):**
- ✅ All Phase 2.3 regression tests passing
- ✅ All Phase 3 regression tests passing
- ✅ ≥95% of UAT tests passing (≥48-76 of 50-80)
- ✅ ≥90% of integration tests passing (≥27-45 of 30-50)
- ✅ Trivy: 0 CRITICAL, 0 HIGH (in app code)
- ✅ Production readiness checklist ≥90% complete
- ✅ No data corruption or data loss incidents

**CONDITIONAL (Review & Remediate):**
- ⚠️ 80-95% of tests passing, but non-blocking issues
- ⚠️ 1-2 MEDIUM vulnerabilities (reviewable, documented)
- ⚠️ Minor documentation gaps (non-essential for core operation)
- **Action:** Fix issues, rerun affected tests, re-evaluate

**FAIL (Do Not Proceed):**
- ❌ <80% of tests passing (indicates major issues)
- ❌ CRITICAL or HIGH security vulnerabilities (in app code)
- ❌ Data loss or corruption incidents
- ❌ Auth/authorization not working
- ❌ WAF/security modules not enforcing
- **Action:** Stop Phase 4, remediate critical issues, restart from appropriate phase

### Escalation Procedure

If tests fail, escalate in this order:

1. **Developer Review** (30 min)
   - Identify root cause in failure logs
   - Determine if fix is quick (code change) or structural

2. **Architecture Review** (if needed)
   - Complex failures affecting multiple components
   - Potential architectural issues

3. **Security Review** (if security-related)
   - WAF/ACL/rate limit failures
   - Authentication/authorization issues
   - Audit trail gaps

4. **Product Review** (if feature-related)
   - Workflow failures that affect user experience
   - Missing features or incorrect behavior

---

## Timeline & Resource Estimate

### Phase 4 Timeline (2-3 hours)

| Phase | Task | Duration | Resources |
|-------|------|----------|-----------|
| 1 | Rebuild E2E environment (if needed) | 5-10 min | 1 QA engineer |
| 2 | Run UAT test suite (70 tests) | 60-90 min | 2 QA engineers (parallel) |
| 3 | Run integration tests (40 tests) | 45-60 min | 2 QA engineers (parallel) |
| 4 | Production readiness review | 30 min | Tech lead + QA |
| 5 | Documentation & final sign-off | 20 min | Tech lead |
| **TOTAL** | **Phase 4 Complete** | **2-3 hours** | **2 QA + 1 Tech Lead** |

### Resource Requirements

- **QA Engineers:** 2 (for parallel test execution)
- **Tech Lead:** 1 (for review and go/no-go decisions)
- **Infrastructure:** Docker environment, CI/CD system
- **Tools:** Playwright, test reporters, monitoring tools

### Pre-Phase 4 Preparation (1 hour)

- [ ] Review Phase 2.3 fix details (crypto, async email, token refresh)
- [ ] Review Phase 3 security test infrastructure
- [ ] Prepare production readiness checklist
- [ ] Set up test monitoring and alerting
- [ ] Create test failure log templates
- [ ] Brief team on Phase 4 plan

---

## Go/No-Go Decision Matrix

### Decision Authority

- **Phase 4 Authorization:** Technical Lead (sole authority)
- **Escalation:** Product Manager (if contested)
- **Final Approval:** Engineering Manager

### Decision Criteria

#### Criteria 1: Test Pass Rates

| Test Category | Phase | Pass Rate | Decision |
|---------------|-------|-----------|----------|
| Regression | 2.3 | 100% | **GO** |
| Regression | 3 | 100% | **GO** |
| UAT | 4 | ≥95% | **GO** |
| Integration | 4 | ≥90% | **GO** |
| All Combined | - | ≥92% | **GO** |
| All Combined | - | 80-92% | **CONDITIONAL** |
| All Combined | - | <80% | **NO-GO** |

#### Criteria 2: Security Vulnerabilities

| Severity | App Code | Dependencies | Decision |
|----------|----------|--------------|----------|
| CRITICAL | 0 allowed | 0 allowed | **REQUIRED** |
| HIGH | 0 allowed | Document & review | **REQUIRED** |
| MEDIUM | Assess risk | Acceptable | **OK** |
| LOW | Acceptable | Acceptable | **OK** |

#### Criteria 3: Production Checklist

| Category | Items Complete | Status |
|----------|-----------------|--------|
| Deployment | ≥13/15 | **GO** |
| Documentation | ≥10/12 | **GO** |
| Security | 10/10 | **REQUIRED** |
| Performance | ≥6/8 | **CONDITIONAL** |
| Release | 10/10 | **REQUIRED** |

#### Criteria 4: Data Integrity

| Aspect | Status | Decision |
|--------|--------|----------|
| No data loss | ✅ Verified | **GO** |
| No data corruption | ✅ Verified | **GO** |
| Backups working | ✅ Verified | **GO** |
| Restore successful | ✅ Verified | **GO** |
| User isolation intact | ✅ Verified | **GO** |

### Sample Decision Scenarios

**Scenario 1: All Tests Pass, No Issues**
```
Phase 2.3 Regression: 20/20 ✅
Phase 3 Regression: 15/15 ✅
UAT: 75/80 (93%) ✅
Integration: 48/50 (96%) ✅
Security Scan: 0 CRITICAL, 0 HIGH ✅
Checklist: 44/45 items ✅
Data Integrity: All verified ✅
───────────────────────────
DECISION: ✅ GO FOR BETA RELEASE
```

**Scenario 2: Few UAT Failures, No Security Issues**
```
Phase 2.3 Regression: 20/20 ✅
Phase 3 Regression: 15/15 ✅
UAT: 68/80 (85%) ⚠️ (7 failures non-blocking)
Integration: 45/50 (90%) ✅
Security Scan: 0 CRITICAL, 0 HIGH ✅
Checklist: 42/45 items ⚠️
Data Integrity: All verified ✅
───────────────────────────
DECISION: 🟡 CONDITIONAL
Action: Fix 7 failing UAT tests, verify no regressions, re-run
Expected: 1-2 hours remediation, then GO
```

**Scenario 3: Security Module Failure**
```
Phase 2.3 Regression: 20/20 ✅
Phase 3 Regression: 12/15 ❌ (ACL tests failing)
UAT: 60/80 (75%) ❌
Integration: 25/50 (50%) ❌
Security Scan: 2 CRITICAL (crypto issue) ❌
───────────────────────────
DECISION: ❌ NO-GO
Action: STOP Phase 4
- Investigate crypto issue (Phase 2.3 regression)
- Fix security module (Phase 3)
- Re-run regression tests
- Potentially restart Phase 4
Timeline: +4-8 hours
```

---

## Appendix

### A. Test Execution Commands

```bash
# Run all tests (sequential)
cd /projects/Charon
npx playwright test tests/phase3/ --project=firefox

# Run specific test category
npx playwright test tests/phase3/security-enforcement.spec.ts --project=firefox

# Run with debug output
npx playwright test --debug

# Generate HTML report
npx playwright show-report

# Run with specific browser
npx playwright test --project=chromium
npx playwright test --project=webkit
```

### B. Key Test Files Locations

- **Phase 2.3 Regression:** `/projects/Charon/tests/phase3/security-enforcement.spec.ts`
- **Phase 3 Regression:** `/projects/Charon/tests/phase3/*.spec.ts`
- **UAT Tests:** `/projects/Charon/tests/phase4-uat/` (to be created)
- **Integration Tests:** `/projects/Charon/tests/phase4-integration/` (to be created)
- **Test Utilities:** `/projects/Charon/tests/utils/`
- **Fixtures:** `/projects/Charon/tests/fixtures/`

### C. Infrastructure Requirements

**Docker Container:**
- Image: `charon:local` (built before Phase 4)
- Ports: 8080 (app), 2019 (Caddy admin), 2020 (emergency)
- Environment: `.env` with required variables

**CI/CD System:**
- GitHub Actions or equivalent
- Docker support
- Test result publishing
- Artifact storage

**Monitoring:**
- Real-time test progress tracking
- Error log aggregation
- Performance metrics collection
- Alert configuration

### D. Failure Investigation Template

When a test fails, use this template to document investigation:

```
Test ID: [e.g., UAT-001]
Test Name: [e.g., "Login page loads"]
Failure Time: [timestamp]
Environment: [docker/local/ci]
Browser: [firefox/chrome/webkit]

Expected Result: [e.g., "Login form displayed"]
Actual Result: [e.g., "404 Not Found"]

Error logs: [relevant logs from playwright reporter]

Root Cause Analysis:
- [ ] Code defect
- [ ] Test environment issue
- [ ] Test flakiness/race condition
- [ ] Environment variable missing
- [ ] Dependency issue (API down, DB locked, etc.)

Proposed Fix: [action to resolve]
Risk Assessment: [impact of fix]
Remediation Time: [estimate]

Sign-off: [investigator] at [time]
```

---

## References

- **Phase 2 Report:** [docs/reports/PHASE_2_FINAL_APPROVAL.md](docs/reports/PHASE_2_FINAL_APPROVAL.md)
- **Phase 3 Report:** [docs/reports/PHASE_3_FINAL_VALIDATION_REPORT.md](docs/reports/PHASE_3_FINAL_VALIDATION_REPORT.md)
- **Current Spec:** [docs/plans/current_spec.md](docs/plans/current_spec.md)
- **Security Instructions:** [.github/instructions/security-and-owasp.instructions.md](.github/instructions/security-and-owasp.instructions.md)
- **Testing Instructions:** [.github/instructions/testing.instructions.md](.github/instructions/testing.instructions.md)

---

## Sign-Off

**Document Status:** READY FOR TEAM REVIEW & APPROVAL

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Technical Lead | [TO BE ASSIGNED] | 2026-02-10 | ☐ |
| QA Lead | [TO BE ASSIGNED] | 2026-02-10 | ☐ |
| Product Manager | [TO BE ASSIGNED] | 2026-02-10 | ☐ |

---

**Version:** 1.0
**Last Updated:** February 10, 2026
**Next Review:** Upon Phase 4 initiation or when significant changes occur
**Document Location:** `/projects/Charon/docs/plans/PHASE_4_UAT_INTEGRATION_PLAN.md`
