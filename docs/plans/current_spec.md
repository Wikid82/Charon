# Security Module Testing Plan: Toggle-On-Test-Toggle-Off Pattern

**Plan ID**: SEC-TEST-2026-001
**Status**: ✅ APPROVED (Supervisor Review: 2026-01-25)
**Priority**: HIGH
**Created**: 2026-01-25
**Updated**: 2026-01-25 (Added Phase -1: Container Startup Fix)
**Branch**: feature/beta-release
**Scope**: Complete security module testing with toggle-on-test-toggle-off pattern

---

## Executive Summary

This plan provides a **definitive testing strategy** for ALL security modules in Charon. Each module will be tested with the **toggle-on-test-toggle-off** pattern to:

1. Verify security features work when enabled
2. Ensure tests don't leave security features in a state that blocks other tests
3. Provide comprehensive coverage of security blocking behavior

---

## Security Module Inventory

### Complete Module List

| Layer | Module | Toggle Key | Implementation | Blocks Requests? |
|-------|--------|------------|----------------|------------------|
| **Master** | Cerberus | `feature.cerberus.enabled` | Backend middleware + Caddy | Controls all layers |
| **Layer 1** | CrowdSec | `security.crowdsec.enabled` | Caddy bouncer plugin | ✅ Yes (IP bans) |
| **Layer 2** | ACL | `security.acl.enabled` | Cerberus middleware | ✅ Yes (IP whitelist/blacklist) |
| **Layer 3** | WAF (Coraza) | `security.waf.enabled` | Caddy Coraza plugin | ✅ Yes (malicious requests) |
| **Layer 4** | Rate Limiting | `security.rate_limit.enabled` | Caddy rate limiter | ✅ Yes (threshold exceeded) |
| **Layer 5** | Security Headers | N/A (per-host) | Caddy headers | ❌ No (affects behavior) |

---

## 1. API Endpoints for Each Module

### 1.1 Master Toggle (Cerberus)

```http
POST /api/v1/settings
Content-Type: application/json

{ "key": "feature.cerberus.enabled", "value": "true" | "false" }
```

**Implementation**: [settings_handler.go](../../backend/internal/api/handlers/settings_handler.go#L73-L108)

**Effect**: When disabled, ALL security modules are disabled regardless of individual settings.

### 1.2 ACL (Access Control Lists)

```http
POST /api/v1/settings
{ "key": "security.acl.enabled", "value": "true" | "false" }
```

**Get Status**:

```http
GET /api/v1/security/status
Returns: { "acl": { "mode": "enabled", "enabled": true } }
```

**Implementation**:

- [cerberus.go](../../backend/internal/cerberus/cerberus.go#L135-L160) - Middleware blocks requests
- [access_list_handler.go](../../backend/internal/api/handlers/access_list_handler.go) - CRUD operations

**Blocking Logic** (from cerberus.go):

```go
for _, acl := range acls {
    allowed, _, err := c.accessSvc.TestIP(acl.ID, clientIP)
    if err == nil && !allowed {
        ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Blocked by access control list"})
        return
    }
}
```

### 1.3 CrowdSec

```http
POST /api/v1/settings
{ "key": "security.crowdsec.enabled", "value": "true" | "false" }
```

**Mode setting**:

```http
POST /api/v1/settings
{ "key": "security.crowdsec.mode", "value": "local" | "disabled" }
```

**Implementation**:

- [crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go) - API handlers
- Caddy crowdsec-bouncer plugin - Actual blocking at proxy layer

### 1.4 WAF (Coraza)

```http
POST /api/v1/settings
{ "key": "security.waf.enabled", "value": "true" | "false" }
```

**Implementation**:

- [security_handler.go](../../backend/internal/api/handlers/security_handler.go#L51-L130) - Status and config
- Caddy Coraza plugin - Actual blocking (SQL injection, XSS, etc.)

### 1.5 Rate Limiting

```http
POST /api/v1/settings
{ "key": "security.rate_limit.enabled", "value": "true" | "false" }
```

**Implementation**:

- [security_handler.go](../../backend/internal/api/handlers/security_handler.go#L425-L460) - Presets
- Caddy rate limiter directive - Actual blocking

### 1.6 Security Headers

**No global toggle** - Applied per proxy host via:

```http
POST /api/v1/proxy-hosts/:id
{ "securityHeaders": { "hsts": true, "csp": "...", ... } }
```

---

## 2. Existing Test Inventory

### 2.1 Test Files by Security Module

| Module | E2E Test Files | Backend Unit Test Files |
|--------|----------------|-------------------------|
| **ACL** | [access-lists-crud.spec.ts](../../tests/core/access-lists-crud.spec.ts) (35+ tests), [proxy-acl-integration.spec.ts](../../tests/integration/proxy-acl-integration.spec.ts) (18 tests) | access_list_handler_test.go, access_list_service_test.go |
| **CrowdSec** | [crowdsec-config.spec.ts](../../tests/security/crowdsec-config.spec.ts) (12 tests), [crowdsec-decisions.spec.ts](../../tests/security/crowdsec-decisions.spec.ts) | crowdsec_handler_test.go (20+ tests) |
| **WAF** | [waf-config.spec.ts](../../tests/security/waf-config.spec.ts) (15 tests) | security_handler_waf_test.go |
| **Rate Limiting** | [rate-limiting.spec.ts](../../tests/security/rate-limiting.spec.ts) (14 tests) | security_ratelimit_test.go |
| **Security Headers** | [security-headers.spec.ts](../../tests/security/security-headers.spec.ts) (16 tests) | security_headers_handler_test.go |
| **Dashboard** | [security-dashboard.spec.ts](../../tests/security/security-dashboard.spec.ts) (20 tests) | N/A |
| **Integration** | [security-suite-integration.spec.ts](../../tests/integration/security-suite-integration.spec.ts) (23 tests) | N/A |

### 2.2 Coverage Gaps (Blocking Tests Needed)

| Module | What's Tested | What's Missing |
|--------|---------------|----------------|
| **ACL** | CRUD, UI toggles, API TestIP | ❌ E2E blocking verification (real HTTP blocked) |
| **CrowdSec** | UI config, decisions display | ❌ E2E IP ban blocking verification |
| **WAF** | UI config, mode toggle | ❌ E2E SQL injection/XSS blocking verification |
| **Rate Limiting** | UI config, settings | ❌ E2E threshold exceeded blocking |
| **Security Headers** | UI config, profiles | ⚠️ Headers present but not enforcement |

---

## 3. Proposed Playwright Project Structure

### 3.1 Test Execution Flow

```text
┌──────────────────┐
│   global-setup   │  ← Disable ALL security (clean slate)
└────────┬─────────┘
         │
┌────────▼─────────┐
│      setup       │  ← auth.setup.ts (login, save state)
└────────┬─────────┘
         │
┌────────▼─────────────────────────────────────────────────────┐
│              security-tests (sequential)                      │
│                                                               │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│   │ acl-tests   │→ │ waf-tests   │→ │crowdsec-tests│         │
│   └─────────────┘  └─────────────┘  └─────────────┘          │
│          │                │                │                  │
│          ▼                ▼                ▼                  │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│   │ rate-limit  │→ │sec-headers  │→ │ combined    │          │
│   │   -tests    │  │   -tests    │  │   -tests    │          │
│   └─────────────┘  └─────────────┘  └─────────────┘          │
└────────────────────────────┬─────────────────────────────────┘
                             │
┌────────────────────────────▼─────────────────────────────────┐
│              security-teardown                                │
│                                                               │
│   Disable: ACL, CrowdSec, WAF, Rate Limiting                 │
│   Restore: Cerberus to disabled state                        │
└────────────────────────────┬─────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
    ┌────▼────┐        ┌─────▼────┐        ┌─────▼────┐
    │chromium │        │ firefox  │        │  webkit  │
    └─────────┘        └──────────┘        └──────────┘
         All run with security modules DISABLED
```

### 3.2 Why Sequential for Security Tests?

Security tests must run **sequentially** (not parallel) because:

1. **Shared state**: All modules share the Cerberus master toggle
2. **Port conflicts**: Tests may use the same proxy hosts
3. **Blocking cascade**: One module enabled can block another's test requests
4. **Cleanup dependencies**: Each module must be disabled before the next runs

### 3.3 Updated `playwright.config.js`

```javascript
projects: [
  // 1. Setup project - authentication (runs FIRST)
  {
    name: 'setup',
    testMatch: /auth\.setup\.ts/,
  },

  // 2. Security Tests - Run WITH security enabled (SEQUENTIAL, headless Chromium)
  {
    name: 'security-tests',
    testDir: './tests/security-enforcement',
    dependencies: ['setup'],
    teardown: 'security-teardown',
    fullyParallel: false, // Force sequential - modules share state
    use: {
      ...devices['Desktop Chrome'],
      headless: true, // Security tests are API-level, don't need headed
    },
  },

  // 3. Security Teardown - Disable ALL security modules
  {
    name: 'security-teardown',
    testMatch: /security-teardown\.setup\.ts/,
  },

  // 4. Browser projects - Depend on TEARDOWN to ensure security is disabled
  {
    name: 'chromium',
    use: { ...devices['Desktop Chrome'], storageState: STORAGE_STATE },
    dependencies: ['setup', 'security-teardown'], // Explicit teardown dependency
  },

  {
    name: 'firefox',
    use: { ...devices['Desktop Firefox'], storageState: STORAGE_STATE },
    dependencies: ['setup', 'security-teardown'],
  },

  {
    name: 'webkit',
    use: { ...devices['Desktop Safari'], storageState: STORAGE_STATE },
    dependencies: ['setup', 'security-teardown'],
  },
],
```

---

## 4. New Test Files Needed

### 4.1 Directory Structure

```text
tests/
├── security-enforcement/           ← NEW FOLDER (no numeric prefixes - order via project config)
│   ├── acl-enforcement.spec.ts
│   ├── waf-enforcement.spec.ts          ← Requires Caddy proxy running
│   ├── crowdsec-enforcement.spec.ts
│   ├── rate-limit-enforcement.spec.ts   ← Requires Caddy proxy running
│   ├── security-headers-enforcement.spec.ts
│   └── combined-enforcement.spec.ts
├── security-teardown.setup.ts      ← NEW FILE
├── security/                       ← EXISTING (UI config tests)
│   ├── security-dashboard.spec.ts
│   ├── waf-config.spec.ts
│   ├── rate-limiting.spec.ts
│   ├── crowdsec-config.spec.ts
│   ├── crowdsec-decisions.spec.ts
│   ├── security-headers.spec.ts
│   └── audit-logs.spec.ts
└── utils/
    └── security-helpers.ts         ← EXISTING (to enhance)
```

### 4.2 Test File Specifications

#### `acl-enforcement.spec.ts` (5 tests)

| Test | Description |
|------|-------------|
| `should verify ACL is enabled` | Check security status returns acl.enabled=true |
| `should block IP not in whitelist` | Create whitelist ACL, verify 403 for excluded IP |
| `should allow IP in whitelist` | Add test IP to whitelist, verify 200 |
| `should block IP in blacklist` | Create blacklist with test IP, verify 403 |
| `should show correct error message` | Verify "Blocked by access control list" message |

#### `waf-enforcement.spec.ts` (4 tests) — Requires Caddy Proxy

| Test | Description |
|------|-------------|
| `should verify WAF is enabled` | Check security status returns waf.enabled=true |
| `should block SQL injection attempt` | Send `' OR 1=1--` in query, verify 403/418 |
| `should block XSS attempt` | Send `<script>alert()</script>`, verify 403/418 |
| `should allow legitimate requests` | Verify normal requests pass through |

#### `crowdsec-enforcement.spec.ts` (3 tests)

| Test | Description |
|------|-------------|
| `should verify CrowdSec is enabled` | Check crowdsec.enabled=true, mode="local" |
| `should create manual ban decision` | POST to /api/v1/security/decisions |
| `should list ban decisions` | GET /api/v1/security/decisions |

#### `rate-limit-enforcement.spec.ts` (3 tests) — Requires Caddy Proxy

| Test | Description |
|------|-------------|
| `should verify rate limiting is enabled` | Check rate_limit.enabled=true |
| `should return rate limit presets` | GET /api/v1/security/rate-limit-presets |
| `should document threshold behavior` | Describe expected 429 behavior |

#### `security-headers-enforcement.spec.ts` (4 tests)

| Test | Description |
|------|-------------|
| `should return X-Content-Type-Options` | Check header = 'nosniff' |
| `should return X-Frame-Options` | Check header = 'DENY' or 'SAMEORIGIN' |
| `should return HSTS on HTTPS` | Check Strict-Transport-Security |
| `should return CSP when configured` | Check Content-Security-Policy |

#### `combined-enforcement.spec.ts` (5 tests)

| Test | Description |
|------|-------------|
| `should enable all modules simultaneously` | Enable all, verify all status=true |
| `should log security events to audit log` | Verify audit entries created |
| `should handle rapid module toggle without race conditions` | Toggle on/off quickly, verify stable state |
| `should persist settings across page reload` | Toggle, refresh, verify settings retained |
| `should enforce priority when multiple modules conflict` | ACL + WAF both enabled, verify correct behavior |

#### `security-teardown.setup.ts`

Disables all security modules with error handling (continue-on-error pattern):

```typescript
import { test as teardown } from '@bgotink/playwright-coverage';
import { request } from '@playwright/test';

teardown('disable-all-security-modules', async () => {
  const modules = [
    { key: 'security.acl.enabled', value: 'false' },
    { key: 'security.waf.enabled', value: 'false' },
    { key: 'security.crowdsec.enabled', value: 'false' },
    { key: 'security.rate_limit.enabled', value: 'false' },
    { key: 'feature.cerberus.enabled', value: 'false' },
  ];

  const requestContext = await request.newContext({
    baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
    storageState: 'playwright/.auth/user.json',
  });

  const errors: string[] = [];

  for (const { key, value } of modules) {
    try {
      await requestContext.post('/api/v1/settings', { data: { key, value } });
      console.log(`✓ Disabled: ${key}`);
    } catch (e) {
      errors.push(`Failed to disable ${key}: ${e}`);
    }
  }

  await requestContext.dispose();

  // Stabilization delay - wait for Caddy config reload
  await new Promise(resolve => setTimeout(resolve, 1000));

  if (errors.length > 0) {
    console.error('Security teardown had errors (continuing anyway):', errors.join('\n'));
    // Don't throw - let other tests run even if teardown partially failed
  }
});
```

---

## 5. Questions Answered

### Q1: What's the API to toggle each module?

| Module | Setting Key | Values |
|--------|-------------|--------|
| Cerberus (Master) | `feature.cerberus.enabled` | `"true"` / `"false"` |
| ACL | `security.acl.enabled` | `"true"` / `"false"` |
| CrowdSec | `security.crowdsec.enabled` | `"true"` / `"false"` |
| WAF | `security.waf.enabled` | `"true"` / `"false"` |
| Rate Limiting | `security.rate_limit.enabled` | `"true"` / `"false"` |

All via: `POST /api/v1/settings` with `{ "key": "<key>", "value": "<value>" }`

### Q2: Should security tests run sequentially or parallel?

**SEQUENTIAL** - Because:

- Modules share Cerberus master toggle
- Enabling one module can block other tests
- Race conditions in security state
- Cleanup dependencies between modules

### Q3: One teardown or separate per module?

**ONE TEARDOWN** - Using Playwright's `teardown` project relationship:

- Runs after ALL security tests complete
- Disables ALL modules in one sweep
- Guaranteed to run even if tests fail
- Simpler maintenance

### Q4: Minimum tests per module?

| Module | Minimum Tests | Requires Caddy? |
|--------|---------------|----------------|
| ACL | 5 | No (Backend) |
| WAF | 4 | Yes |
| CrowdSec | 3 | No (API) |
| Rate Limiting | 3 | Yes |
| Security Headers | 4 | No |
| Combined | 5 | Partial |
| **Total** | **24** | |

---

## 6. Implementation Checklist

### Phase -1: Container Startup Fix (URGENT BLOCKER - 15 min)

**STATUS**: 🔴 BLOCKING — E2E tests cannot run until this is fixed

**Problem**: Docker entrypoint creates directories as root before dropping privileges to `charon` user, causing Caddy permission errors:

```
{"error":"save snapshot: write snapshot: open /app/data/caddy/config-1769363949.json: permission denied"}
```

**Evidence** (from `docker exec charon-e2e ls -la /app/data/`):

```
drwxr-xr-x 2 root   root        40 Jan 25 17:59 caddy   <-- WRONG: root ownership
drwxr-xr-x 2 root   root        40 Jan 25 17:59 geoip   <-- WRONG: root ownership
drwxr-xr-x 2 charon charon     100 Jan 25 17:59 crowdsec <-- CORRECT
```

**Required Fix** in `.docker/docker-entrypoint.sh`:

After the mkdir block (around line 35), add ownership fix:

```bash
# Fix ownership for directories created as root
if is_root; then
    chown -R charon:charon /app/data/caddy 2>/dev/null || true
    chown -R charon:charon /app/data/crowdsec 2>/dev/null || true
    chown -R charon:charon /app/data/geoip 2>/dev/null || true
fi
```

- [ ] **Fix docker-entrypoint.sh**: Add chown commands after mkdir block
- [ ] **Rebuild E2E container**: Run `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
- [ ] **Verify fix**: Confirm `ls -la /app/data/` shows `charon:charon` ownership

---

### Phase 0: Critical Fixes (Blocking - 30 min)

**From Supervisor Review — MUST FIX BEFORE PROCEEDING:**

- [ ] **Fix hardcoded IP**: Change `tests/global-setup.ts` line 17 from `100.98.12.109` to `localhost`
- [ ] **Expand emergency reset**: Update `emergencySecurityReset()` in `global-setup.ts` to disable ALL security modules (not just ACL)
- [ ] **Add failsafe**: Global-setup should attempt to disable all security modules BEFORE auth (crash protection)

### Phase 1: Infrastructure (1 hour)

- [ ] Create `tests/security-enforcement/` directory
- [ ] Create `tests/security-teardown.setup.ts` (with error handling + stabilization delay)
- [ ] Update `playwright.config.js` with security-tests and security-teardown projects
- [ ] Enhance `tests/utils/security-helpers.ts`

### Phase 2: Enforcement Tests (3 hours)

- [ ] Create `acl-enforcement.spec.ts` (5 tests)
- [ ] Create `waf-enforcement.spec.ts` (4 tests) — requires Caddy
- [ ] Create `crowdsec-enforcement.spec.ts` (3 tests)
- [ ] Create `rate-limit-enforcement.spec.ts` (3 tests) — requires Caddy
- [ ] Create `security-headers-enforcement.spec.ts` (4 tests)
- [ ] Create `combined-enforcement.spec.ts` (5 tests)

### Phase 3: Verification (1 hour)

- [ ] Run: `npx playwright test --project=security-tests`
- [ ] Verify teardown disables all modules
- [ ] Run full suite: `npx playwright test`
- [ ] Verify < 10 failures (only genuine issues)

---

## 7. Success Criteria

| Metric | Before | Target |
|--------|--------|--------|
| Security enforcement tests | 0 | 24 |
| Test failures from ACL blocking | 222 | 0 |
| Security module toggle coverage | Partial | 100% |
| CI security test job | N/A | Passing |

---

## References

- [Playwright Project Dependencies](https://playwright.dev/docs/test-projects#dependencies)
- [Playwright Teardown](https://playwright.dev/docs/test-global-setup-teardown#teardown)
- [Security Helpers](../../tests/utils/security-helpers.ts)
- [Cerberus Middleware](../../backend/internal/cerberus/cerberus.go)
- [Security Handler](../../backend/internal/api/handlers/security_handler.go)

---

## 8. Known Pre-existing Test Failures (Not Blocking)

**Analysis Date**: 2026-01-25
**Status**: ⚠️ DOCUMENTED — Fix separately from security testing work

These 5 failures pre-date the Docker Hub, break-glass, and security testing infrastructure changes. Git history confirms no settings test files were modified in the current work.

### Failure Summary

| Test File | Line | Failure | Root Cause | Type |
|-----------|------|---------|------------|------|
| `account-settings.spec.ts` | 289 | `getByText(/invalid.*email|email.*invalid/i)` not found | Frontend email validation error text doesn't match test regex | Locator mismatch |
| `system-settings.spec.ts` | 412 | `data-testid="toast-success"` or `/success|saved/i` not found | Success toast implementation doesn't match test expectations | Locator mismatch |
| `user-management.spec.ts` | 277 | Strict mode: 2 elements match `/send.*invite/i` | Commit `0492c1be` added "Resend Invite" button conflicting with "Send Invite" | UI change without test update |
| `user-management.spec.ts` | 436 | Strict mode: 2 elements match `/send.*invite/i` | Same as above | UI change without test update |
| `user-management.spec.ts` | 948 | Strict mode: 2 elements match `/send.*invite/i` | Same as above | UI change without test update |

### Evidence

**Last modification to settings test files**: Commit `0492c1be` (Jan 24, 2026) — "fix: implement user management UI"

This commit added:
- "Resend Invite" button for pending users in the users table
- Email format validation with error display
- But did not update the test locators to distinguish between buttons

### Recommended Fix (Future PR)

```typescript
// CURRENT (fails strict mode):
const sendButton = page.getByRole('button', { name: /send.*invite/i });

// FIX: Be more specific to match only modal button
const sendButton = page
  .locator('.invite-modal')  // or modal dialog locator
  .getByRole('button', { name: /send.*invite/i });

// OR use exact name:
const sendButton = page.getByRole('button', { name: 'Send Invite' });
```

### Tracking

These should be fixed in a separate PR after the security testing implementation is complete. They do not block the current work.

---

## 10. Supervisor Review Summary

**Review Date**: 2026-01-25
**Verdict**: ✅ APPROVED with Recommendations

### Grades

| Criteria | Grade | Notes |
|----------|-------|-------|
| Test Structure | B+ → A | Fixed with explicit teardown dependencies |
| API Correctness | A | Verified against settings_handler.go |
| Coverage | B → A- | Expanded from 21 to 24 tests |
| Pitfall Handling | B- → A | Added error handling + stabilization delay |
| Best Practices | A- | Removed numeric prefixes |

### Key Changes Incorporated

1. **Browser dependencies fixed**: Now depend on `['setup', 'security-teardown']` not just `['security-tests']`
2. **Teardown error handling**: Continue-on-error pattern with logging
3. **Stabilization delay**: 1-second wait after teardown for Caddy reload
4. **Test count increased**: 21 → 24 tests (3 new combined tests)
5. **Numeric prefixes removed**: Playwright ignores them; rely on project config
6. **Headless enforcement**: Security tests run headless Chromium (API-level tests)
7. **Caddy requirements documented**: WAF and Rate Limiting tests need Caddy proxy

### Critical Pre-Implementation Fixes (Phase 0)

These MUST be completed before Phase 1:

1. ❌ `tests/global-setup.ts:17` — Change `100.98.12.109` → `localhost`
2. ❌ `emergencySecurityReset()` — Expand to disable ALL modules, not just ACL
3. ❌ Add pre-auth security disable attempt (crash protection)

---
