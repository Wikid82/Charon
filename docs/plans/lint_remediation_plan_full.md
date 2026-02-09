# Lint Remediation & Monitoring Plan

**Status:** Planning
**Created:** 2026-02-02
**Target Completion:** 2026-02-03

---

## Executive Summary

This plan addresses 40 Go linting issues (18 errcheck, 22 gosec warnings from `full_lint_output.txt`), 6 TypeScript warnings, and establishes monitoring for retry attempt frequency to ensure it remains below 5%.

### Goals

1. **Go Linting:** Fix all 40 reported issues (18 errcheck, 22 gosec)
2. **TypeScript:** Resolve 6 ESLint warnings (no-explicit-any, no-unused-vars)
3. **Monitoring:** Implement retry attempt frequency tracking (<5% threshold)

---

## Research Findings

### 1. Go Linting Issues (40 total from full_lint_output.txt)

**Source Files:**
- `backend/final_lint.txt` (34 issues - subset)
- `backend/full_lint_output.txt` (40 issues - complete list)

#### 1.1 Errcheck Issues (18 total)

**Category A: Unchecked json.Unmarshal in Tests (6)**

| File | Line | Issue |
|------|------|-------|
| `internal/api/handlers/security_handler_audit_test.go` | 581 | `json.Unmarshal(w.Body.Bytes(), &resp)` |
| `internal/api/handlers/security_handler_coverage_test.go` | 525, 589 | `json.Unmarshal(w.Body.Bytes(), &resp)` (2 locations) |
| `internal/api/handlers/settings_handler_test.go` | 895, 923, 1081 | `json.Unmarshal(w.Body.Bytes(), &resp)` (3 locations) |

**Root Cause:** Test code not checking JSON unmarshaling errors
**Impact:** Tests may pass with invalid JSON responses, false positives
**Fix:** Add error checking: `require.NoError(t, json.Unmarshal(...))`

**Category B: Unchecked Environment Variable Operations (11)**

| File | Line | Issue |
|------|------|-------|
| `internal/caddy/config_test.go` | 1794 | `os.Unsetenv(v)` |
| `internal/config/config_test.go` | 56, 57, 72, 74, 75, 82 | `os.Setenv(...)` (6 instances) |
| `internal/config/config_test.go` | 157, 158, 159, 175, 196 | `os.Unsetenv(...)` (5 instances total) |

**Root Cause:** Environment variable setup/cleanup without error handling
**Impact:** Test isolation failures, flaky tests
**Fix:** Wrap with `require.NoError(t, os.Setenv/Unsetenv(...))`

**Category C: Unchecked Database Close Operations (4)**

| File | Line | Issue |
|------|------|-------|
| `internal/services/dns_provider_service_test.go` | 1446, 1466, 1493, 1531, 1549 | `sqlDB.Close()` (4 locations) |
| `internal/database/errors_test.go` | 230 | `sqlDB.Close()` |

**Root Cause:** Resource cleanup without error handling
**Impact:** Resource leaks in tests
**Fix:** `defer func() { _ = sqlDB.Close() }()` or explicit error check

**Category D: Unchecked w.Write in Tests (3)**

| File | Line | Issue |
|------|------|-------|
| `internal/caddy/manager_additional_test.go` | 1467, 1522 | `w.Write([]byte(...))` (2 locations) |
| `internal/caddy/manager_test.go` | 133 | `w.Write([]byte(...))` |

**Root Cause:** HTTP response writing without error handling
**Impact:** Silent failures in mock HTTP servers
**Fix:** `_, _ = w.Write(...)` or check error if critical

**Category E: Unchecked db.AutoMigrate in Tests (3)**

| File | Line | Issue |
|------|------|-------|
| `internal/api/handlers/notification_coverage_test.go` | 22 | `db.AutoMigrate(...)` |
| `internal/api/handlers/pr_coverage_test.go` | 404, 438 | `db.AutoMigrate(...)` (2 locations) |

**Root Cause:** Database schema migration without error handling
**Impact:** Tests may run with incorrect schema
**Fix:** `require.NoError(t, db.AutoMigrate(...))`

#### 1.2 Gosec Security Issues (22 total - unchanged from final_lint.txt)

*(Same 22 gosec issues as documented in final_lint.txt)*

### 2. TypeScript Linting Issues (6 warnings - unchanged)

*(Same 6 ESLint warnings as documented earlier)*

### 3. Retry Monitoring Analysis

**Current State:**

**Retry Logic Location:** `backend/internal/services/uptime_service.go`

**Configuration:**
- `MaxRetries` in `UptimeServiceConfig` (default: 2)
- `MaxRetries` in `models.UptimeMonitor` (default: 3)

**Current Behavior:**
```go
for retry := 0; retry <= s.config.MaxRetries && !success; retry++ {
    if retry > 0 {
        logger.Log().Info("Retrying TCP check")
    }
    // Try connection...
}
```

**Metrics Gaps:**
- No retry frequency tracking
- No alerting on excessive retries
- No historical data for analysis

**Requirements:**
- Track retry attempts vs first-try successes
- Alert if retry rate >5% over rolling 1000 checks
- Expose Prometheus metrics for dashboarding

---

## Technical Specifications

### Phase 1: Backend Go Linting Fixes

#### 1.1 Errcheck Fixes (18 issues)

**JSON Unmarshal (6 fixes):**

```go
// Pattern to apply across 6 locations
// BEFORE:
json.Unmarshal(w.Body.Bytes(), &resp)

// AFTER:
err := json.Unmarshal(w.Body.Bytes(), &resp)
require.NoError(t, err, "Failed to unmarshal response")
```

**Files:**
- `internal/api/handlers/security_handler_audit_test.go:581`
- `internal/api/handlers/security_handler_coverage_test.go:525, 589`
- `internal/api/handlers/settings_handler_test.go:895, 923, 1081`

**Environment Variables (11 fixes):**

```go
// BEFORE:
os.Setenv("VAR_NAME", "value")

// AFTER:
require.NoError(t, os.Setenv("VAR_NAME", "value"))
```

**Files:**
- `internal/config/config_test.go:56, 57, 72, 74, 75, 82, 157, 158, 159, 175, 196`
- `internal/caddy/config_test.go:1794`

**Database Close (4 fixes):**

```go
// BEFORE:
sqlDB.Close()

// AFTER:
defer func() { _ = sqlDB.Close() }()
```

**Files:**
- `internal/services/dns_provider_service_test.go:1446, 1466, 1493, 1531, 1549`
- `internal/database/errors_test.go:230`

**HTTP Write (3 fixes):**

```go
// BEFORE:
w.Write([]byte(`{"data": "value"}`))

// AFTER:
_, _ = w.Write([]byte(`{"data": "value"}`))
```

**Files:**
- `internal/caddy/manager_additional_test.go:1467, 1522`
- `internal/caddy/manager_test.go:133`

**AutoMigrate (3 fixes):**

```go
// BEFORE:
db.AutoMigrate(&models.Model{})

// AFTER:
require.NoError(t, db.AutoMigrate(&models.Model{}))
```

**Files:**
- `internal/api/handlers/notification_coverage_test.go:22`
- `internal/api/handlers/pr_coverage_test.go:404, 438`

#### 1.2 Gosec Security Fixes (22 issues)

*(Apply the same 22 gosec fixes as documented in the original plan)*

### Phase 2: Frontend TypeScript Linting Fixes (6 warnings)

*(Apply the same 6 TypeScript fixes as documented in the original plan)*

### Phase 3: Retry Monitoring Implementation

*(Same implementation as documented in the original plan)*

---

## Implementation Plan

### Phase 1: Backend Go Linting Fixes

**Estimated Time:** 3-4 hours

**Tasks:**

1. **Errcheck Fixes** (60 min)
   - [ ] Fix 6 JSON unmarshal errors
   - [ ] Fix 11 environment variable operations
   - [ ] Fix 4 database close operations
   - [ ] Fix 3 HTTP write operations
   - [ ] Fix 3 AutoMigrate calls

2. **Gosec Fixes** (2-3 hours)
   - [ ] Fix 8 permission issues
   - [ ] Fix 3 integer overflow issues
   - [ ] Fix 3 file inclusion issues
   - [ ] Fix 1 slice bounds issue
   - [ ] Fix 2 decompression bomb issues
   - [ ] Fix 1 file traversal issue
   - [ ] Fix 2 Slowloris issues
   - [ ] Fix 1 hardcoded credential (add #nosec comment)

**Verification:**
```bash
cd backend && golangci-lint run ./...
# Expected: 0 issues
```

### Phase 2: Frontend TypeScript Linting Fixes

**Estimated Time:** 1-2 hours

*(Same as original plan)*

### Phase 3: Retry Monitoring Implementation

**Estimated Time:** 4-5 hours

*(Same as original plan)*

---

## Acceptance Criteria

**Phase 1 Complete:**
- [ ] All 40 Go linting issues resolved (18 errcheck + 22 gosec)
- [ ] `golangci-lint run ./...` exits with code 0
- [ ] All unit tests pass
- [ ] Code coverage ≥85%

**Phase 2 Complete:**
- [ ] All 6 TypeScript warnings resolved
- [ ] `npm run lint` shows 0 warnings
- [ ] All unit tests pass
- [ ] Code coverage ≥85%

**Phase 3 Complete:**
- [ ] Retry rate metric exposed at `/metrics`
- [ ] API endpoint `/api/v1/uptime/stats` returns correct data
- [ ] Dashboard displays retry rate widget
- [ ] Alert logged when retry rate >5%
- [ ] E2E test validates monitoring flow

---

## File Changes Summary

### Backend Files (21 total)

#### Errcheck (14 files):
1. `internal/api/handlers/security_handler_audit_test.go` (1)
2. `internal/api/handlers/security_handler_coverage_test.go` (2)
3. `internal/api/handlers/settings_handler_test.go` (3)
4. `internal/config/config_test.go` (13)
5. `internal/caddy/config_test.go` (1)
6. `internal/services/dns_provider_service_test.go` (5)
7. `internal/database/errors_test.go` (1)
8. `internal/caddy/manager_additional_test.go` (2)
9. `internal/caddy/manager_test.go` (1)
10. `internal/api/handlers/notification_coverage_test.go` (1)
11. `internal/api/handlers/pr_coverage_test.go` (2)

#### Gosec (18 files):
12. `cmd/seed/seed_smoke_test.go`
13. `internal/api/handlers/manual_challenge_handler.go`
14. `internal/api/handlers/security_handler_rules_decisions_test.go`
15. `internal/caddy/config.go`
16. `internal/config/config.go`
17. `internal/crowdsec/hub_cache.go`
18. `internal/crowdsec/hub_sync.go`
19. `internal/database/database_test.go`
20. `internal/services/backup_service.go`
21. `internal/services/backup_service_test.go`
22. `internal/services/uptime_service_test.go`
23. `internal/util/crypto_test.go`

### Frontend Files (5 total):
1. `src/components/ImportSitesModal.test.tsx`
2. `src/components/ImportSitesModal.tsx`
3. `src/components/__tests__/DNSProviderForm.test.tsx`
4. `src/context/AuthContext.tsx`
5. `src/hooks/__tests__/useImport.test.ts`

### New Files (Phase 3):
1. `backend/internal/metrics/uptime_metrics.go` (if needed)
2. `frontend/src/components/RetryStatsCard.tsx`
3. `tests/uptime-retry-stats.spec.ts`
4. `docs/monitoring.md`

---

## References

- **Go Lint Output:** `backend/final_lint.txt` (34 issues), `backend/full_lint_output.txt` (40 issues)
- **TypeScript Lint Output:** `npm run lint` output (6 warnings)
- **Gosec:** https://github.com/securego/gosec
- **golangci-lint:** https://golangci-lint.run/
- **Prometheus Best Practices:** https://prometheus.io/docs/practices/naming/

---

**Plan Status:** ✅ Ready for Implementation
**Next Step:** Begin Phase 1 - Backend Go Linting Fixes (Errcheck first, then Gosec)
