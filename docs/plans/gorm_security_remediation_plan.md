# GORM Security Issues Remediation Plan

**Status:** 🟡 **READY TO START**
**Created:** 2026-01-28
**Scanner Report:** 60 issues detected (28 CRITICAL, 2 HIGH, 33 MEDIUM)
**Estimated Total Time:** 8-12 hours
**Target Completion:** 2026-01-29

---

## Executive Summary

The GORM Security Scanner detected **60 pre-existing security issues** in the codebase. This plan provides a systematic approach to fix all issues, organized by priority and type.

**Issue Breakdown:**
- 🔴 **28 CRITICAL**: 22 ID leaks + 3 exposed secrets + 2 DTO issues + 1 emergency field
- 🔵 **33 MEDIUM**: Missing primary key tags (informational)

**Approach:**
1. Fix all CRITICAL issues (models, secrets, DTOs)
2. Optionally address MEDIUM issues (missing tags)
3. Verify with scanner
4. Run full test suite
5. Update CI to blocking mode

---

## Phase 1: Fix ID Leaks in Models (6-8 hours)

### Priority: 🔴 CRITICAL
### Estimated Time: 6-8 hours (22 models × 15-20 min each)

**Pattern:**
```go
// BEFORE (Vulnerable):
ID uint `json:"id" gorm:"primaryKey"`

// AFTER (Secure):
ID uint `json:"-" gorm:"primaryKey"`
```

### Task Checklist

#### Core Models (6 models)
- [ ] **User** (`internal/models/user.go:23`)
- [ ] **ProxyHost** (`internal/models/proxy_host.go:9`)
- [ ] **Domain** (`internal/models/domain.go:11`)
- [ ] **DNSProvider** (`internal/models/dns_provider.go:11`)
- [ ] **SSLCertificate** (`internal/models/ssl_certificate.go:10`)
- [ ] **AccessList** (`internal/models/access_list.go:10`)

#### Security Models (5 models)
- [ ] **SecurityConfig** (`internal/models/security_config.go:10`)
- [ ] **SecurityAudit** (`internal/models/security_audit.go:9`)
- [ ] **SecurityDecision** (`internal/models/security_decision.go:10`)
- [ ] **SecurityHeaderProfile** (`internal/models/security_header_profile.go:10`)
- [ ] **SecurityRuleset** (`internal/models/security_ruleset.go:9`)

#### Infrastructure Models (5 models)
- [ ] **Location** (`internal/models/location.go:9`)
- [ ] **Plugin** (`internal/models/plugin.go:8`)
- [ ] **RemoteServer** (`internal/models/remote_server.go:10`)
- [ ] **ImportSession** (`internal/models/import_session.go:10`)
- [ ] **Setting** (`internal/models/setting.go:10`)

#### Integration Models (3 models)
- [ ] **CrowdsecConsoleEnrollment** (`internal/models/crowdsec_console_enrollment.go:7`)
- [ ] **CrowdsecPresetEvent** (`internal/models/crowdsec_preset_event.go:7`)
- [ ] **CaddyConfig** (`internal/models/caddy_config.go:9`)

#### Provider & Monitoring Models (3 models)
- [ ] **DNSProviderCredential** (`internal/models/dns_provider_credential.go:11`)
- [ ] **EmergencyToken** (`internal/models/emergency_token.go:10`)
- [ ] **UptimeHeartbeat** (`internal/models/uptime.go:39`)

### Post-Model-Update Tasks

After changing each model:

1. **Update API handlers** that reference `.ID`:
   ```bash
   # Find usages:
   grep -r "\.ID" backend/internal/api/handlers/
   grep -r "\"id\":" backend/internal/api/handlers/
   ```

2. **Update service layer** queries using `.ID`:
   ```bash
   # Find usages:
   grep -r "\.ID" backend/internal/services/
   ```

3. **Verify UUID field exists and is exposed**:
   ```go
   UUID string `json:"uuid" gorm:"uniqueIndex"`
   ```

4. **Update tests** referencing `.ID`:
   ```bash
   # Find test failures:
   go test ./... -run TestModel
   ```

---

## Phase 2: Fix Exposed Secrets (30 minutes)

### Priority: 🔴 CRITICAL
### Estimated Time: 30 minutes (3 fields)

**Pattern:**
```go
// BEFORE (Vulnerable):
APIKey string `json:"api_key"`

// AFTER (Secure):
APIKey string `json:"-"`
```

### Task Checklist

- [ ] **User.APIKey** - Change to `json:"-"`
  - Location: `internal/models/user.go`
  - Verify: API key is never serialized to JSON

- [ ] **ManualChallenge.Token** - Change to `json:"-"`
  - Location: `internal/services/manual_challenge_service.go:337`
  - Verify: Challenge token is never exposed

- [ ] **CaddyConfig.ConfigHash** - Change to `json:"-"`
  - Location: `internal/models/caddy_config.go`
  - Verify: Config hash is never exposed

### Verification

```bash
# Ensure no secrets are exposed:
./scripts/scan-gorm-security.sh --report | grep "CRITICAL.*Secret"
# Should return: 0 results
```

---

## Phase 3: Fix Response DTO Embedding (1-2 hours)

### Priority: 🟡 HIGH
### Estimated Time: 1-2 hours (2 structs)

**Problem:** Response structs embed models, inheriting exposed IDs

**Pattern:**
```go
// BEFORE (Inherits ID exposure):
type ProxyHostResponse struct {
    models.ProxyHost  // Embeds entire model
    Warnings []ProxyHostWarning `json:"warnings,omitempty"`
}

// AFTER (Explicit fields):
type ProxyHostResponse struct {
    UUID         string    `json:"uuid"`
    DomainNames  []string  `json:"domain_names"`
    ForwardHost  string    `json:"forward_host"`
    // ... other fields explicitly defined
    Warnings     []ProxyHostWarning `json:"warnings,omitempty"`
}
```

### Task Checklist

- [ ] **ProxyHostResponse** (`internal/api/handlers/proxy_host_handler.go:31`)
  - [ ] Replace `models.ProxyHost` embedding with explicit fields
  - [ ] Include `UUID string json:"uuid"` (expose external ID)
  - [ ] Exclude `ID uint` (hide internal ID)
  - [ ] Update all handler functions creating ProxyHostResponse
  - [ ] Update tests

- [ ] **DNSProviderResponse** (`internal/services/dns_provider_service.go:56`)
  - [ ] Replace `models.DNSProvider` embedding with explicit fields
  - [ ] Include `UUID string json:"uuid"` (expose external ID)
  - [ ] Exclude `ID uint` (hide internal ID)
  - [ ] Keep `HasCredentials bool json:"has_credentials"`
  - [ ] Update all service functions creating DNSProviderResponse
  - [ ] Update tests

### Post-DTO-Update Tasks

1. **Update handler logic**:
   ```go
   // Map model to response:
   response := ProxyHostResponse{
       UUID:        model.UUID,
       DomainNames: model.DomainNames,
       // ... explicit field mapping
   }
   ```

2. **Frontend coordination** (if needed):
   - Frontend likely uses `uuid` already, not `id`
   - Verify API client expectations
   - Update TypeScript types if needed

---

## Phase 4: Fix Emergency Break Glass Field (15 minutes)

### Priority: 🔴 CRITICAL
### Estimated Time: 15 minutes (1 field)

**Issue:** `EmergencyConfig.BreakGlassEnabled` exposed

### Task Checklist

- [ ] **EmergencyConfig.BreakGlassEnabled**
  - Location: Find in security models
  - Change: Add `json:"-"` or verify it's informational only
  - Verify: Emergency status not leaking sensitive info

---

## Phase 5: Optional - Fix Missing Primary Key Tags (1 hour)

### Priority: 🔵 MEDIUM (Informational)
### Estimated Time: 1 hour (33 fields)
### Decision: **OPTIONAL** - Can defer to separate issue

**Pattern:**
```go
// BEFORE (Missing tag):
ID uint `json:"-"`

// AFTER (Explicit tag):
ID uint `json:"-" gorm:"primaryKey"`
```

**Impact:** Missing tags don't cause security issues, but explicit tags improve:
- Query optimizer performance
- Code clarity
- GORM auto-migration accuracy

**Recommendation:** Create separate issue for this backlog item.

---

## Phase 6: Verification & Testing (1-2 hours)

### Task Checklist

#### 1. Scanner Verification (5 minutes)
- [ ] Run scanner in report mode:
  ```bash
  ./scripts/scan-gorm-security.sh --report
  ```
- [ ] Verify: **0 CRITICAL** and **0 HIGH** issues remain
- [ ] Optional: Verify **0 MEDIUM** if Phase 5 completed

#### 2. Backend Tests (30 minutes)
- [ ] Run full backend test suite:
  ```bash
  cd backend && go test ./... -v
  ```
- [ ] Verify: All tests pass
- [ ] Fix any test failures related to ID → UUID changes

#### 3. Backend Coverage (15 minutes)
- [ ] Run coverage tests:
  ```bash
  .github/skills/scripts/skill-runner.sh test-backend-coverage
  ```
- [ ] Verify: Coverage ≥85%
- [ ] Address any coverage drops

#### 4. Frontend Tests (if API changes) (30 minutes)
- [ ] TypeScript type check:
  ```bash
  cd frontend && npm run type-check
  ```
- [ ] Run frontend tests:
  ```bash
  .github/skills/scripts/skill-runner.sh test-frontend-coverage
  ```
- [ ] Verify: All tests pass

#### 5. Integration Tests (15 minutes)
- [ ] Start Docker environment:
  ```bash
  docker-compose up -d
  ```
- [ ] Test affected endpoints:
  - GET /api/proxy-hosts (verify UUID, no ID)
  - GET /api/dns-providers (verify UUID, no ID)
  - GET /api/users/me (verify no APIKey exposed)

#### 6. Final Scanner Check (5 minutes)
- [ ] Run scanner in check mode:
  ```bash
  ./scripts/scan-gorm-security.sh --check
  echo "Exit code: $?"  # Must be 0
  ```
- [ ] Verify: Exit code **0** (no issues)

---

## Phase 7: Enable Blocking in Pre-commit (5 minutes)

### Task Checklist

- [ ] Update `.pre-commit-config.yaml`:
  ```yaml
  # CHANGE FROM:
  stages: [manual]

  # CHANGE TO:
  stages: [commit]
  ```

- [ ] Test pre-commit hook:
  ```bash
  pre-commit run --all-files
  ```

- [ ] Verify: Scanner runs on commit and passes

---

## Phase 8: Update Documentation (15 minutes)

### Task Checklist

- [ ] Update `docs/plans/gorm_security_remediation_plan.md`:
  - Mark all tasks as complete
  - Add "COMPLETED" status and completion date

- [ ] Update `docs/implementation/gorm_security_scanner_complete.md`:
  - Update "Current Findings" section to show 0 issues
  - Update pre-commit status to "Blocking"

- [ ] Update `docs/reports/gorm_scanner_qa_report.md`:
  - Add remediation completion note

- [ ] Update `CHANGELOG.md`:
  - Add entry: "Fixed 60 GORM security issues (ID leaks, exposed secrets)"

---

## Success Criteria

### Technical Success ✅
- [ ] Scanner reports **0 CRITICAL** and **0 HIGH** issues
- [ ] All backend tests pass
- [ ] Coverage maintained (≥85%)
- [ ] Pre-commit hook in blocking mode
- [ ] CI pipeline passes

### Security Success ✅
- [ ] No internal database IDs exposed via JSON
- [ ] No API keys/tokens/secrets exposed
- [ ] Response DTOs use explicit fields
- [ ] UUID fields used for all external references

### Documentation Success ✅
- [ ] Remediation plan marked complete
- [ ] Scanner documentation updated
- [ ] CHANGELOG updated
- [ ] Blocking mode documented

---

## Risk Mitigation

### Risk: Breaking API Changes

**Mitigation:**
- Frontend likely already uses `uuid` field, not `id`
- Test all API endpoints after changes
- Check frontend API client for `id` references
- Coordinate with frontend team if breaking changes needed

### Risk: Test Failures

**Mitigation:**
- Tests may hardcode `.ID` references
- Search and replace `.ID` → `.UUID` in tests
- Verify test fixtures use UUIDs
- Run tests incrementally after each model update

### Risk: Handler/Service Breakage

**Mitigation:**
- Use grep to find all `.ID` references before changing models
- Update handlers/services at the same time as models
- Test each endpoint after updating
- Use compiler errors as a guide (`.ID` will fail to serialize)

---

## Time Estimates Summary

| Phase | Priority | Time | Can Defer? |
|-------|----------|------|------------|
| Phase 1: ID Leaks (22 models) | 🔴 CRITICAL | 6-8 hours | ❌ No |
| Phase 2: Secrets (3 fields) | 🔴 CRITICAL | 30 min | ❌ No |
| Phase 3: DTO Embedding (2 structs) | 🟡 HIGH | 1-2 hours | ❌ No |
| Phase 4: Emergency Field (1 field) | 🔴 CRITICAL | 15 min | ❌ No |
| Phase 5: Missing Tags (33 fields) | 🔵 MEDIUM | 1 hour | ✅ Yes |
| Phase 6: Verification & Testing | Required | 1-2 hours | ❌ No |
| Phase 7: Enable Blocking | Required | 5 min | ❌ No |
| Phase 8: Documentation | Required | 15 min | ❌ No |
| **TOTAL (Required)** | | **9.5-13 hours** | |
| **TOTAL (with optional)** | | **10.5-14 hours** | |

---

## Quick Start Guide for Tomorrow

### Morning Session (4-5 hours)

1. **Start Scanner** to see baseline:
   ```bash
   ./scripts/scan-gorm-security.sh --report | tee gorm-before.txt
   ```

2. **Tackle Core Models** (User, ProxyHost, Domain, DNSProvider, SSLCertificate, AccessList):
   - For each model:
     - Change `json:"id"` → `json:"-"`
     - Verify `UUID` field exists
     - Run: `go test ./internal/models/`
     - Fix compilation errors
   - Batch commit: "fix: hide internal IDs in core GORM models"

3. **Coffee Break** ☕

### Afternoon Session (4-5 hours)

4. **Tackle Remaining Models** (Security, Infrastructure, Integration models):
   - Same pattern as morning
   - Batch commit: "fix: hide internal IDs in remaining GORM models"

5. **Fix Exposed Secrets** (Phase 2):
   - Quick 30-minute task
   - Commit: "fix: hide API keys and sensitive fields"

6. **Fix Response DTOs** (Phase 3):
   - ProxyHostResponse
   - DNSProviderResponse
   - Commit: "refactor: use explicit fields in response DTOs"

7. **Final Verification** (Phase 6):
   - Run scanner: `./scripts/scan-gorm-security.sh --check`
   - Run tests: `go test ./...`
   - Run coverage: `.github/skills/scripts/skill-runner.sh test-backend-coverage`

8. **Enable & Document** (Phases 7-8):
   - Update pre-commit config to blocking mode
   - Update all docs
   - Final commit: "chore: enable GORM security scanner blocking mode"

---

## Post-Remediation

After all fixes are complete:

1. **Create PR** with all remediation commits
2. **CI will pass** (scanner finds 0 issues)
3. **Merge to main**
4. **Close chore item** in `docs/plans/chores.md`
5. **Celebrate** 🎉 - Charon is now more secure!

---

## Notes

- **Current Branch:** `feature/beta-release`
- **Scanner Location:** `scripts/scan-gorm-security.sh`
- **Documentation:** `docs/implementation/gorm_security_scanner_complete.md`
- **QA Report:** `docs/reports/gorm_scanner_qa_report.md`

**Tips:**
- Work in small batches (5-6 models at a time)
- Test incrementally (don't fix everything before testing)
- Use grep to find all `.ID` references before each change
- Commit frequently to make rollback easier if needed
- Take breaks - this is tedious but important work!

**Good luck tomorrow! 💪**
