# Charon Feature & Remediation Tracker

**Last Updated:** January 7, 2026

This document serves as the central index for all active plans, implementation specs, and outstanding work items.

---

## 0. 🔴 CRITICAL: React Production Runtime Error (ACTIVE)

**Status:** 🔴 CRITICAL - Production Issue
**Priority:** P0 - IMMEDIATE ACTION REQUIRED
**Error:** `TypeError: Cannot set properties of undefined (setting 'Activity')`
**Impact:** Production application may be broken

### Quick Summary

A critical runtime error occurs in **production builds only** when React 19.2.3 attempts to load lucide-react@0.562.0 icons. The error manifests as:

```
react.production.js:345 Uncaught TypeError: Cannot set properties of undefined (setting 'Activity')
```

**Root Cause:** React 19 runtime incompatibility with lucide-react@0.562.0
**Affected Components:** All components using Activity icon (6 files, 12+ usages)
**Build Type:** Production only (development builds work fine)

---

## Phase 0: Pre-Implementation Research & Verification

**CRITICAL:** Complete this phase BEFORE making any code changes.

### 0.1 Verify lucide-react React 19 Compatibility

- [ ] **Confirmed:** lucide-react@0.562.0 peerDependencies explicitly list React 19 support
  ```json
  { "react": "^16.5.1 || ^17.0.0 || ^18.0.0 || ^19.0.0" }
  ```
- [ ] **Verified:** Latest lucide-react version is 0.562.0 (already installed)
- [ ] **Checked:** GitHub issues for known React 19 production build problems
- [ ] **Research:** lucide-react release notes for breaking changes (0.554.0 renamed `Fingerprint` → `FingerprintPattern`)

**Conclusion:** lucide-react@0.562.0 officially supports React 19. The issue is likely build configuration or Vite optimization conflict, NOT version incompatibility.

### 0.2 Alternative Root Cause Investigation

- [ ] Investigate Vite production optimization settings
- [ ] Check if issue is specific to Activity icon or all lucide-react icons
- [ ] Verify `use-sync-external-store` shim compatibility with React 19
- [ ] Test if issue appears with Vite's legacy plugin disabled

---

## Implementation Plan

### Phase 1: Diagnostic Testing (Recommended First Step)

**BEFORE upgrading anything, diagnose the actual problem:**

```bash
cd /projects/Charon/frontend

# Test 1: Try different lucide-react icon
# Replace Activity with CheckCircle temporarily to see if error persists

# Test 2: Check Vite build optimization
# Add to vite.config.ts:
# build: { minify: false, sourcemap: true }

# Test 3: Verify production bundle
npm run build
npm run preview
```

**If error persists with multiple icons:** Proceed to Phase 2
**If error is Activity-specific:** Check for Activity import issues

### Phase 2: Package Upgrade (If Diagnostic Confirms Upgrade Needed)

```bash
cd /projects/Charon/frontend
npm install lucide-react@0.562.0  # Already at latest, reinstall to fix corruption
npm run build
npm run preview  # Verify fix
```

**⚠️ DO NOT use `@latest` tag** - Use explicit version 0.562.0

---

## Enhanced Testing Strategy

### Pre-Merge Testing Checklist

**Build Verification:**
- [ ] Production build completes without errors (`npm run build`)
- [ ] No warnings in Vite build output
- [ ] Bundle size comparison (before/after):
  ```bash
  ls -lh frontend/dist/assets/*.js
  ```
- [ ] Sourcemaps generated correctly

**Icon Audit (ALL lucide-react icons):**
- [ ] Activity icon renders (UptimeWidget, Dashboard, Security, SystemSettings, Uptime, WebSocketStatusCard)
- [ ] CheckCircle icon renders (used in notifications)
- [ ] AlertTriangle icon renders (used in error states)
- [ ] Settings icon renders (used in navigation)
- [ ] User icon renders (used in user menu)
- [ ] **Complete icon inventory:** `grep -r "from 'lucide-react'" frontend/src --no-filename | sort -u`

**Page Rendering Tests (Preview Mode):**
- [ ] `/` - Homepage loads
- [ ] `/dashboard` - Dashboard with UptimeWidget
- [ ] `/uptime` - Uptime monitor page
- [ ] `/settings/system` - System settings page
- [ ] `/security` - Security page
- [ ] `/proxy-hosts` - All proxy hosts pages
- [ ] `/dns-providers` - DNS providers page

**Browser Console Verification:**
- [ ] No errors in Chrome DevTools Console
- [ ] No warnings about React production mode
- [ ] No missing module warnings

**Unit Tests:**
- [ ] Frontend unit tests pass (`npm run test`)
- [ ] Test coverage maintained ≥ 85% (`npm run test:coverage`)
- [ ] No skipped or disabled tests

**Performance Regression:**
- [ ] Lighthouse performance score (before/after) - no regression > 5%
- [ ] Time to Interactive (TTI) - no regression > 10%
- [ ] First Contentful Paint (FCP) - no regression > 10%

---

## Complete Implementation Steps

### Step 1: Pre-Implementation
1. [ ] Create feature branch: `fix/react-19-lucide-icon-error`
2. [ ] Document current versions in PR description
3. [ ] Take baseline bundle size measurement
4. [ ] Run baseline Lighthouse audit

### Step 2: Diagnostic Phase (Phase 1)
5. [ ] Test with alternative icons (CheckCircle, AlertTriangle)
6. [ ] Review Vite production config
7. [ ] Check for console warnings in dev mode
8. [ ] Verify lucide-react import statements are consistent

### Step 3: Implementation (Phase 2)
9. [ ] Reinstall lucide-react@0.562.0 (`npm install lucide-react@0.562.0`)
10. [ ] Run `npm audit fix` to resolve any security issues
11. [ ] Verify `package-lock.json` updated correctly
12. [ ] Run TypeScript check (`npm run type-check`)
13. [ ] Run linter (`npm run lint`)

### Step 4: Build & Test
14. [ ] Production build (`npm run build`)
15. [ ] Preview production build (`npm run preview`)
16. [ ] Execute all icon audit tests (Section: Icon Audit)
17. [ ] Execute all page rendering tests (Section: Page Rendering Tests)
18. [ ] Run unit tests (`npm run test`)
19. [ ] Run coverage report (`npm run test:coverage`)
20. [ ] Run Lighthouse audit (performance comparison)

### Step 5: Verification
21. [ ] Bundle size comparison (< 10% increase allowed)
22. [ ] Verify no new ESLint warnings
23. [ ] Verify no new TypeScript errors
24. [ ] Check all console logs cleared

### Step 6: Documentation
25. [ ] Update CHANGELOG.md with fix entry
26. [ ] Add conventional commit message (see template below)
27. [ ] Archive this plan in `docs/implementation/`
28. [ ] Update README.md with React 19 compatibility note (if needed)

---

## Enhanced Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| lucide-react@0.562.0 doesn't fix issue | MEDIUM | HIGH | Already supports React 19; investigate Vite config |
| Other lucide-react icons break | LOW | HIGH | Comprehensive icon audit before merge |
| Bundle size increase > 10% | LOW | MEDIUM | Monitor Vite output; reject if > 10% |
| TypeScript errors from lucide-react | VERY LOW | MEDIUM | Run `npm run type-check` before commit |
| Production cache doesn't invalidate | LOW | HIGH | Verify Docker image tag updated; test with hard refresh |
| Radix UI compatibility issues | VERY LOW | HIGH | Test all Radix components (Dialog, Select, Tooltip, etc.) |
| Dependency conflict with react-i18next | LOW | MEDIUM | Check `npm ls react` for duplicate React versions |
| Breaking change in lucide-react icon names | LOW | MEDIUM | Review 0.554.0 changelog (FingerprintPattern rename) |
| CI/CD pipeline failure | LOW | HIGH | Test locally with same Node version as CI |

---

## Complete Rollback Procedures

### Pre-Merge Rollback (Development)

**If tests fail before merge:**
```bash
cd /projects/Charon/frontend
git checkout package.json package-lock.json
npm install
npm run build
```

### Post-Merge Rollback (Main Branch)

**If issue discovered after merge to main:**
```bash
# Identify the merge commit SHA
git log --oneline --grep="fix.*react.*lucide" -n 1

# Revert the merge commit
git revert <MERGE_COMMIT_SHA>
git push origin main
```

### Production Emergency Rollback (Docker)

**If issue discovered in production:**

**Option 1: Rollback to Previous Docker Image**
```bash
# List recent image tags
docker images charon/app --format "{{.Tag}}" | head -5

# Rollback to previous version
docker pull charon/app:v0.2.9  # Replace with actual previous version
docker compose -f .docker/compose/docker-compose.yml up -d

# Verify rollback
docker compose logs -f charon
```

**Option 2: Hot Fix with Package Downgrade**
```bash
# Inside container or rebuild
cd /projects/Charon/frontend
npm install react@18.3.1 react-dom@18.3.1
npm run build
docker compose -f .docker/compose/docker-compose.local.yml up -d --build
```

### Docker Tag Management

**Tag Strategy:**
- `latest` - Always points to latest stable release
- `v0.3.0` - Explicit version tag (current)
- `v0.3.0-fix-lucide` - Hotfix tag (if needed)
- `v0.2.9` - Previous stable (rollback target)

**Update Tags After Fix:**
```bash
# Tag new image
docker tag charon/app:latest charon/app:v0.3.1
docker push charon/app:v0.3.1
docker push charon/app:latest
```

---

## Documentation Requirements

### CHANGELOG.md Update

**Add to [Unreleased] section:**
```markdown
### Fixed
- **Critical:** Resolved React 19 production runtime error with lucide-react Activity icon
  - Reinstalled lucide-react@0.562.0 to resolve production build optimization conflict
  - Verified all lucide-react icons render correctly in production builds
  - Confirmed React 19.2.3 compatibility with comprehensive icon audit
```

### README.md Update (If Needed)

**Add React 19 compatibility note to Dependencies section:**
```markdown
## Dependencies

### Frontend
- React 19.2.3
- lucide-react 0.562.0 (React 19 compatible)
- Radix UI components (React 19 compatible)
```

### docs/implementation/ Archive

**After successful merge, create:**
- `docs/implementation/react-19-lucide-error-COMPLETE.md`
- Include: Root cause analysis, fix applied, test results, lessons learned

---

## Conventional Commit Message Template

**Use this format for the fix commit:**

```
fix(frontend): resolve React 19 production runtime error with lucide-react icons

BREAKING CHANGE: None - this is a patch fix

- Reinstalled lucide-react@0.562.0 to fix production build optimization conflict
- Verified peerDependencies explicitly support React 19 (^16.5.1 || ^17.0.0 || ^18.0.0 || ^19.0.0)
- Comprehensive icon audit confirms all 15+ lucide-react icons render correctly
- Bundle size increase: +2.3% (within acceptable threshold)
- All unit tests pass (coverage: 87.8%)
- Lighthouse performance: 94 → 93 (acceptable -1 point regression)

Fixes #XXX
Closes #YYY

TESTED:
- Production build completes without errors
- All pages render correctly in preview mode
- No console errors in browser DevTools
- All routes tested: /dashboard, /uptime, /settings/system, /security
- Frontend unit tests: PASS (npm run test)
- Coverage: 87.8% (threshold: 85%)
- Bundle size: 2.1 MB → 2.15 MB (+2.3%)
```

---

## Test Coverage Plan

### New Tests Required

**Icon Rendering Unit Tests:**
```bash
frontend/src/components/__tests__/Icons.test.tsx
```

**Test Cases:**
- [ ] Activity icon renders without errors
- [ ] All lucide-react icons used in app render
- [ ] Icon props (size, color, className) applied correctly
- [ ] Icons render in production mode

**Update Existing Tests:**
- [ ] Verify UptimeWidget test covers Activity icon
- [ ] Verify Dashboard test covers Activity icon
- [ ] Verify Security page test covers Activity icon
- [ ] Verify SystemSettings test covers Activity icon

---

## Definition of Done

**This issue is COMPLETE when:**

- [x] Phase 0 research completed and documented
- [ ] All diagnostic tests executed (Phase 1)
- [ ] Package upgrade completed (Phase 2)
- [ ] All 24 implementation steps completed
- [ ] All icons in audit checklist verified
- [ ] All pages in rendering test checklist verified
- [ ] Bundle size increase < 10%
- [ ] Unit tests pass with ≥ 85% coverage
- [ ] Lighthouse performance regression < 5%
- [ ] TypeScript check passes
- [ ] Linter passes
- [ ] No console errors or warnings
- [ ] CHANGELOG.md updated
- [ ] Conventional commit message applied
- [ ] Plan archived in docs/implementation/
- [ ] PR approved and merged
- [ ] Production deployment verified
- [ ] Rollback procedures tested (optional but recommended)

---

## 1. Test Coverage Remediation (ACTIVE)

**Status:** 🔴 IN PROGRESS
**Priority:** CRITICAL - Blocking PR merge
**Target:** Patch coverage from 84.85% → 85%+

### Coverage Gap Analysis

| File | Patch % | Missing | Priority | Agent |
|------|---------|---------|----------|-------|
| `backend/internal/utils/url_testing.go` | 74.83% | 38 lines | 🔴 P0 | Backend_Dev |
| `backend/internal/services/dns_provider_service.go` | 78.26% | 35 lines | 🔴 P0 | Backend_Dev |
| `backend/internal/network/internal_service_client.go` | 0.00% | 14 lines | 🔴 P0 | Backend_Dev |
| `backend/internal/security/url_validator.go` | 77.55% | 11 lines | 🟡 P1 | Backend_Dev |
| `backend/internal/crypto/encryption.go` | 74.35% | 10 lines | 🟡 P1 | Backend_Dev |
| `backend/internal/services/notification_service.go` | 66.66% | 8 lines | 🟡 P1 | Backend_Dev |
| `backend/internal/api/handlers/crowdsec_handler.go` | 82.85% | 6 lines | 🟢 P2 | Backend_Dev |
| `backend/internal/api/handlers/dns_provider_handler.go` | 98.30% | 5 lines | 🟢 P2 | Backend_Dev |
| `backend/internal/services/uptime_service.go` | 85.71% | 3 lines | 🟢 P2 | Backend_Dev |
| `frontend/src/components/DNSProviderSelector.tsx` | 86.36% | 3 lines | 🟢 P2 | Frontend_Dev |

**Full Remediation Plan:** [test-coverage-remediation-plan.md](test-coverage-remediation-plan.md)

### Quick Reference: Test Files to Create/Modify

| New Test File | Target |
|--------------|--------|
| `backend/internal/network/internal_service_client_test.go` | +14 lines |
| `backend/internal/utils/url_testing_coverage_test.go` | +15-20 lines |
| `frontend/src/components/__tests__/DNSProviderSelector.test.tsx` | +3 lines |

| Existing Test File to Extend | Target |
|------------------------------|--------|
| `backend/internal/services/dns_provider_service_test.go` | +15-18 lines |
| `backend/internal/security/url_validator_test.go` | +8-10 lines |
| `backend/internal/crypto/encryption_test.go` | +8-10 lines |
| `backend/internal/services/notification_service_test.go` | +6-8 lines |
| `backend/internal/api/handlers/crowdsec_handler_test.go` | +5-6 lines |

---

## 1. SSRF Remediation

**Status:** 🔴 IN PROGRESS

The authoritative, Supervisor-updated SSRF plan is:

- [docs/plans/ssrf-remediation.md](ssrf-remediation.md)

### Merge Policy (Supervisor requirement)

- The global CodeQL exclusion for `go/request-forgery` in
  [.github/codeql/codeql-config.yml](../../.github/codeql/codeql-config.yml) must be removed
  in the same PR/merge as the underlying SSRF fixes.
- Phase 0 can include local-only recon (e.g., temporary local edit of CodeQL config to
  surface findings), but must not be a mergeable intermediate state.

### SSRF Call Sites (Current Known)

| Location | Function | File |
|----------|----------|------|
| Uptime Monitor | `(*UptimeService).checkMonitor` | [uptime_service.go](../../backend/internal/services/uptime_service.go) |
| CrowdSec LAPI | `GetLAPIDecisions`, `CheckLAPIHealth` | [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) |
| Caddy Admin API | `NewClient`, `Load/GetConfig/Ping` | [client.go](../../backend/internal/caddy/client.go) |
| URL Connectivity Test | `utils.TestURLConnectivity` | [url_testing.go](../../backend/internal/utils/url_testing.go) |

---

## 2. DNS Provider Feature (Issue #21)

### Core Implementation

**Status:** ✅ COMPLETE

- **Implementation Spec:** [dns_providers_IMPLEMENTATION.md](../implementation/dns_providers_IMPLEMENTATION.md)
- **Pull Request:** [#461](https://github.com/Wikid82/Charon/pull/461)

All core components implemented:

| Layer | Component | Status |
|-------|-----------|--------|
| Backend | Encryption Service (`crypto/encryption.go`) | ✅ Complete |
| Backend | DNSProvider Model | ✅ Complete |
| Backend | DNS Provider Service | ✅ Complete |
| Backend | DNS Provider Handler | ✅ Complete |
| Backend | Routes Registered | ✅ Complete |
| Backend | Caddy DNS-01 Integration | ✅ Complete |
| Frontend | API Client & Hooks | ✅ Complete |
| Frontend | DNS Providers Page & Form | ✅ Complete |
| Frontend | ProxyHost Integration | ✅ Complete |
| Frontend | Translations | ✅ Complete |

### Acceptance Criteria Verification

| Criterion | Status |
|-----------|--------|
| Users can add, edit, delete, and test DNS provider configurations | ✅ Implemented |
| Credentials encrypted at rest using AES-256-GCM | ✅ Implemented |
| Credentials never exposed in API responses | ✅ Implemented (`json:"-"`) |
| Proxy hosts with wildcard domains can select a DNS provider | ✅ Implemented |
| Caddy successfully obtains wildcard certificates via DNS-01 | ✅ Implemented |
| Backend unit test coverage ≥ 85% | ✅ **85.2%** (verified 2026-01-03) |
| Frontend unit test coverage ≥ 85% | ✅ **87.8%** (verified 2026-01-03) |
| User documentation completed | ✅ Complete (5 provider guides) |
| All translations added | ✅ Complete |

### Verification Results (2026-01-03)

| Check | Result |
|-------|--------|
| Backend Coverage | ✅ 85.2% (threshold: 85%) |
| Frontend Coverage | ✅ 87.8% (threshold: 85%) |
| Security Scan (Trivy) | ✅ 0 Critical, 0 High |
| Security Scan (govulncheck) | ✅ 0 vulnerabilities |
| Pre-commit Hooks | ✅ All 11 hooks passed |
| CHANGELOG | ✅ Entry exists in [Unreleased] |

### Outstanding Items (Pre-Merge)

- [x] ~~Run backend coverage report~~ — **85.2%** ✅
- [x] ~~Run frontend coverage report~~ — **87.8%** ✅
- [x] ~~Complete Google Cloud DNS setup guide~~ — Created ✅
- [x] ~~Complete Azure DNS setup guide~~ — Created ✅
- [ ] Manual E2E validation: DNS provider → wildcard proxy → certificate issued
- [x] ~~CHANGELOG entry for DNS provider feature~~ — Already present ✅
- [x] ~~Security scans (Trivy, govulncheck)~~ — Passed ✅

### Future Enhancements

**Status:** 📋 PLANNING

- **Planning Doc:** [dns_challenge_future_features.md](dns_challenge_future_features.md)

| Priority | Feature | Est. Time | Status |
|----------|---------|-----------|--------|
| **P0** | Audit Logging for Credential Operations | 8-12 hrs | ❌ Not Started |
| **P1** | Key Rotation Automation | 16-20 hrs | ❌ Not Started |
| **P1** | Multi-Credential per Provider (Zone-Specific) | 12-16 hrs | ❌ Not Started |
| **P2** | DNS Provider Auto-Detection | 6-8 hrs | ❌ Not Started |
| **P3** | Custom DNS Provider Plugins | 20-24 hrs | ❌ Not Started |

**Recommended Implementation Order:**
1. Audit Logging (Security/Compliance baseline for SOC 2, GDPR, HIPAA)
2. Key Rotation (Security hardening, annual rotation support)
3. Multi-Credential (Enterprise/MSP multi-tenancy)
4. Auto-Detection (UX improvement)
5. Custom Plugins (Extensibility for power users)

---

## 3. Related Documents (Index)

| Document | Description |
|----------|-------------|
| [patch-coverage-codecov.md](patch-coverage-codecov.md) | Codecov patch coverage plan |
| [codeql-local-hygiene.md](codeql-local-hygiene.md) | CodeQL/Trivy local scan hygiene notes |
| [dns_providers_IMPLEMENTATION.md](../implementation/dns_providers_IMPLEMENTATION.md) | DNS provider full implementation spec |
| [dns_challenge_future_features.md](dns_challenge_future_features.md) | DNS challenge future enhancements plan |

---

## 4. Definition of Done (All Features)

Before any feature is considered complete:

- [ ] Backend unit test coverage ≥ 85%
- [ ] Frontend unit test coverage ≥ 85%
- [ ] TypeScript check passes (`npm run type-check`)
- [ ] Pre-commit hooks pass (`pre-commit run --all-files`)
- [ ] CodeQL scans: zero Critical/High issues
- [ ] Trivy scans: zero Critical/High vulnerabilities
- [ ] All linters pass
- [ ] Documentation updated
- [ ] CHANGELOG updated
