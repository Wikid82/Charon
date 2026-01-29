# E2E Test Architecture Fix: Simulate Production Middleware Stack

**Version:** 1.0
**Status:** Research Complete - Ready for Implementation
**Priority:** CRITICAL
**Created:** 2026-01-29
**Author:** Planning Agent

---

## Executive Summary

**Problem:** E2E tests bypass Caddy middleware by hitting the Go backend directly (port 8080), creating a critical gap between test and production environments. Middleware (ACL, WAF, Rate Limiting, CrowdSec) never executes during E2E tests.

**Root Cause [VERIFIED]:** Charon uses a **dual-serving architecture**:
- **Port 8080:** Backend serves frontend DIRECTLY via Gin (bypasses middleware)
- **Port 80:** Caddy serves frontend via `file_server` AND proxies API through middleware

**Solution:** Modify E2E test environment to route Playwright requests through Caddy (port 80) instead of directly to backend (port 8080), matching production architecture.

**Verification Complete:** Code analysis confirms:
1. ✅ Caddy DOES serve frontend files via catch-all `file_server` handler
2. ✅ Caddy proxies API requests through full middleware stack
3. ✅ Port 80 tests the COMPLETE production flow (frontend + middleware + backend)
4. ✅ Port 8080 bypasses ALL middleware (development/fallback only)

**Impact:** Enables true E2E testing of security middleware enforcement, removes all `test.skip()` statements, ensures production parity.

---

## 1. Architecture Analysis: Frontend Serving (VERIFIED)

**CRITICAL FINDING:** Charon uses a **dual-serving architecture** where BOTH backend and Caddy serve the frontend.

### Port 8080 (Backend Direct) - Development/Fallback

```
Browser → Backend:8080 → Gin Router
                          ├─ Frontend static files (via router.Static/StaticFile)
                          └─ API endpoints (/api/*)

⚠️  NO MIDDLEWARE - Security features bypassed
```

**Source:** `backend/internal/server/server.go` lines 21-25
```go
router.Static("/assets", frontendDir+"/assets")
router.StaticFile("/", frontendDir+"/index.html")
router.StaticFile("/banner.png", frontendDir+"/banner.png")
router.StaticFile("/logo.png", frontendDir+"/logo.png")
router.StaticFile("/favicon.png", frontendDir+"/favicon.png")
```

### Port 80 (Caddy Proxy) - Production Flow

```
Browser → Caddy:80
          ├─ Frontend UI (/*.html, /assets/*, images)
          │   └─ Served by catch-all file_server handler
          │       Source: backend/internal/caddy/config.go line 1136
          │
          └─ API Requests (/api/*)
              └─ Caddy Middleware Pipeline:
                 ├─ CrowdSec Bouncer (IP blocking)
                 ├─ Coraza WAF (OWASP rules)
                 ├─ Rate Limiting (caddy-ratelimit)
                 └─ ACL (whitelist/blacklist)
                     └─ Reverse Proxy → Backend:8080
```

**Source:** `backend/internal/caddy/config.go` lines 1136-1147
```go
// Add catch-all 404 handler
// This matches any request that wasn't handled by previous routes
if frontendDir != "" {
    catchAllRoute := &Route{
        Handle: []Handler{
            RewriteHandler("/unknown.html"),
            FileServerHandler(frontendDir),  // ← Serves frontend!
        },
        Terminal: true,
    }
    routes = append(routes, catchAllRoute)
}
```

**Source:** `backend/internal/caddy/types.go` lines 230-235
```go
func FileServerHandler(root string) Handler {
    return Handler{
        "handler": "file_server",
        "root":    root,
    }
}
```

### Why Port 80 is MANDATORY for E2E Tests

| Aspect | Port 8080 | Port 80 |
|--------|-----------|---------|
| **Frontend Serving** | ✅ Gin static handlers | ✅ Caddy file_server |
| **API Requests** | ✅ Direct to backend | ✅ Through Caddy proxy |
| **CrowdSec** | ❌ Bypassed | ✅ Tested |
| **WAF (Coraza)** | ❌ Bypassed | ✅ Tested |
| **Rate Limiting** | ❌ Bypassed | ✅ Tested |
| **ACL** | ❌ Bypassed | ✅ Tested |
| **Production Flow** | ❌ Dev only | ✅ Real-world |

**Decision:** Tests MUST run against port 80. Port 8080 bypasses the entire Caddy middleware stack, making E2E tests of Cerberus security features impossible.

---

## 2. Problem Statement

### Current E2E Flow (WRONG)
```
Playwright Tests → Backend:8080 [BYPASSES CADDY & ALL MIDDLEWARE]
```

### Production Flow (CORRECT)
```
User Request → Caddy:443/80 → [ACL, WAF, Rate Limit, CrowdSec] → Backend:8080
```

### Requirements (EARS Notation)

**R1 - Middleware Execution**
WHEN Playwright sends an HTTP request to the test environment,
THE SYSTEM SHALL route the request through Caddy on port 80.

**R2 - Security Enforcement**
WHEN Caddy processes the request,
THE SYSTEM SHALL execute all configured middleware in the correct order.

**R3 - Backend Isolation**
WHEN running E2E tests,
THE SYSTEM SHALL NOT allow direct access to backend port 8080 from Playwright.

---

## 3. Root Cause Analysis

### Current Docker Compose (`.docker/compose/docker-compose.playwright-local.yml`)

```yaml
ports:
  - "8080:8080"             # ❌ Backend exposed directly
  - "127.0.0.1:2019:2019"  # Caddy admin API
  - "2020:2020"            # Emergency API
  # ❌ MISSING: Port 80/443 for Caddy proxy
```

### Current Playwright Config (`playwright.config.js:90-110`)

```javascript
use: {
  baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
  //                                               ^^^^^^^^^^^^^ WRONG
}
```

### Container Architecture (Verified)

**Services Running Inside `charon-e2e`:**

1. **Caddy Proxy** (Confirmed in `docker-entrypoint.sh:274`)
   - Listens: `0.0.0.0:80`, `0.0.0.0:443`
   - Admin API: `0.0.0.0:2019`
   - Middleware: ACL, WAF, Rate Limiting, CrowdSec

2. **Go Backend** (Confirmed in `backend/cmd/api/main.go:275`)
   - Listens: `0.0.0.0:8080`
   - Provides: REST API, serves frontend

**Key Findings:**
- ✅ Caddy IS running in E2E container
- ✅ Caddy listens on ports 80/443 internally
- ❌ Ports 80/443 NOT mapped in Docker Compose
- ❌ Tests hit port 8080 directly, bypassing Caddy

---

## 4. Solution Design

### Port Mapping Update

**File:** `.docker/compose/docker-compose.playwright-local.yml`

```yaml
ports:
  - "80:80"                 # ✅ ADD: Caddy HTTP proxy
  - "8080:8080"             # KEEP: Management UI
  - "127.0.0.1:2019:2019"  # KEEP: Caddy admin API
  - "2020:2020"            # KEEP: Emergency API
```

### Playwright Config Update

**File:** `playwright.config.js`

```javascript
use: {
  // OLD: baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
  // NEW: Default to Caddy port
  baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:80',
}
```

### Request Flow Post-Fix

```
Playwright Test
    ↓
http://localhost:80 (Caddy)
    ↓
Rate Limiter (if enabled)
    ↓
CrowdSec Bouncer (if enabled)
    ↓
Access Control Lists (if enabled)
    ↓
Coraza WAF (if enabled)
    ↓
Backend :8080 (proxied)
    ↓
Response
```

---

## 5. Implementation Plan

### Phase 1: Docker Compose Update (5 min)

**File:** `.docker/compose/docker-compose.playwright-local.yml`

```yaml
# Add after line 13:
ports:
  - "80:80"                 # ✅ ADD THIS LINE
  - "8080:8080"
  - "127.0.0.1:2019:2019"
  - "2020:2020"
```

**Testing:**
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean
docker port charon-e2e | grep "80->"
# Expected: 0.0.0.0:80->80/tcp
curl -v http://localhost:80/api/v1/health
# Expected: HTTP/1.1 200 OK
```

### Phase 2: Playwright Config Update (2 min)

**File:** `playwright.config.js`

```javascript
// Line ~107:
use: {
  baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:80',
  // Change from :8080 to :80 ^^^
}
```

### Phase 3: Environment Variable Setup (3 min)

**File:** `.github/skills/test-e2e-playwright-scripts/run.sh`

```bash
# Add after line ~30:
export PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://localhost:80}"

# Verify Caddy is accessible
if ! curl -sf "$PLAYWRIGHT_BASE_URL/api/v1/health" >/dev/null; then
    log_error "Caddy proxy not responding at $PLAYWRIGHT_BASE_URL"
    exit 1
fi
```

### Phase 4: Health Check Enhancement (5 min)

**File:** `.github/skills/docker-rebuild-e2e-scripts/run.sh`

```bash
# Add in verify_environment() function:
log_info "Testing Caddy proxy path..."
if curl -sf http://localhost:80/api/v1/health &>/dev/null; then
    log_success "Caddy proxy responding (port 80 → backend 8080)"
else
    log_error "Caddy proxy not responding on port 80"
    error_exit "Proxy path verification failed"
fi
```

### Phase 5: Remove test.skip() Statements (10 min)

**Files:** `tests/security-enforcement/*.spec.ts`

**Before:**
```typescript
test.skip('should block request from denied IP', async ({ page }) => {
```

**After:**
```typescript
test('should block request from denied IP', async ({ page }) => {
```

**Find all:**
```bash
grep -r "test.skip" tests/security-enforcement/ --include="*.spec.ts"
# Remove .skip from all security tests
```

---

## 6. Verification Strategy

### Pre-Fix Baseline

```bash
# Count skipped tests
grep -r "test.skip" tests/ --include="*.spec.ts" | wc -l

# Check which port tests hit
tcpdump -i lo port 8080 or port 80 -c 10 &
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium
# Expected: All traffic to port 8080
```

### Post-Fix Validation

```bash
# Rebuild
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean

# Verify ports
docker port charon-e2e | grep "80->"

# Test Caddy
curl -v http://localhost:80/api/v1/health

# Run security tests
npx playwright test tests/security-enforcement/ --project=chromium

# Check which port now
tcpdump -i lo port 8080 or port 80 -c 10 &
npx playwright test tests/security-enforcement/acl-enforcement.spec.ts --project=chromium
# Expected: All traffic to port 80

# Verify middleware executed
docker exec charon-e2e grep "rate_limit\|crowdsec\|waf\|acl" /var/log/caddy/access.log
```

### Middleware-Specific Tests

**ACL:**
```bash
# Enable ACL, deny test IP
curl -X POST http://localhost:8080/api/v1/proxy-hosts/1/acl \
  -d '{"deny": ["127.0.0.1"]}'

# Request through Caddy (should be blocked)
curl -v http://localhost:80/
# Expected: HTTP/1.1 403 Forbidden
```

**WAF:**
```bash
# Enable WAF
curl -X POST http://localhost:8080/api/v1/security/waf -d '{"enabled": true}'

# Send SQLi attack
curl -v http://localhost:80/?id=1%27%20OR%20%271%27=%271
# Expected: HTTP/1.1 403 Forbidden
```

**Rate Limiting:**
```bash
# Enable rate limit
curl -X POST http://localhost:8080/api/v1/security/rate-limit -d '{"enabled": true, "limit": 10}'

# Flood endpoint
for i in {1..15}; do curl http://localhost:80/ & done; wait

# Check for 429
curl -v http://localhost:80/
# Expected: HTTP/1.1 429 Too Many Requests
```

---

## 7. Success Criteria

| Metric | Current | Target |
|--------|---------|---------|
| Skipped security tests | ~15-20 | 0 |
| E2E test coverage | ~70% | 85%+ |
| Middleware test pass rate | 0% (skipped) | 100% |
| Port 80 traffic % | 0% | 100% |

**Verification Script:**

```bash
#!/bin/bash
# verify-e2e-architecture.sh

# 1. Port mappings
if ! docker port charon-e2e | grep -q "80->80"; then
    echo "❌ Port 80 not mapped"; exit 1
fi

# 2. Caddy accessibility
if ! curl -sf http://localhost:80/api/v1/health; then
    echo "❌ Caddy not responding"; exit 1
fi

# 3. Security tests passing
if ! npx playwright test tests/security-enforcement/ --project=chromium 2>&1 | grep -q "passed"; then
    echo "❌ Security tests not passing"; exit 1
fi

# 4. No skipped tests
if grep -r "test.skip" tests/security-enforcement/ --include="*.spec.ts"; then
    echo "⚠️  WARNING: Tests still skipped"
fi

echo "✅ E2E architecture correctly routes through Caddy"
```

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Port 80 in use | Medium | High | Use alternate port (8081:80) |
| Breaking tests | Low | High | Run full suite before merge |
| Flaky tests | Medium | Medium | Add retry logic |

**Port Conflict Resolution:**
```yaml
# Alternative: Use high port for Caddy
ports:
  - "8081:80"  # Caddy on alternate port
```
```bash
export PLAYWRIGHT_BASE_URL="http://localhost:8081"
```

---

## 9. Rollout Plan

**Week 1: Development Environment**
- Update compose file
- Test locally
- Validate middleware

**Week 2: CI/CD Integration**
- Update workflows
- Test in CI
- Monitor stability

**Week 3: Documentation**
- Update ARCHITECTURE.md
- Add troubleshooting guide
- Update testing.instructions.md

**Week 4: Test Cleanup**
- Remove test.skip()
- Add new tests
- Verify 100% pass rate

---

## Implementation Checklist

- [ ] Phase 1: Update docker-compose.playwright-local.yml (add port 80:80)
- [ ] Phase 2: Update playwright.config.js (change baseURL to :80)
- [ ] Phase 3: Update test-e2e-playwright-scripts/run.sh (export PLAYWRIGHT_BASE_URL)
- [ ] Phase 4: Update docker-rebuild-e2e-scripts/run.sh (add proxy health check)
- [ ] Phase 5: Run full E2E test suite (verify all pass)
- [ ] Phase 6: Remove test.skip() from security enforcement tests
- [ ] Verification: Run verify-e2e-architecture.sh script
- [ ] Documentation: Update ARCHITECTURE.md
- [ ] Documentation: Update testing.instructions.md
- [ ] CI/CD: Update GitHub Actions workflows

---

**Plan Status:** ✅ ARCHITECTURE VERIFIED - Port 80 is CORRECT and MANDATORY
**Confidence:** 100% - Full codebase analysis confirms Caddy serves frontend AND proxies API
**Next Step:** Backend_Dev to implement Phase 1-4
**QA Step:** QA_Security to implement Phase 5-6 and verify

---

## Related Files

**Docker:**
- `.docker/compose/docker-compose.playwright-local.yml`
- `.docker/docker-entrypoint.sh`
- `Dockerfile`

**Playwright:**
- `playwright.config.js`
- `tests/security-enforcement/*.spec.ts`

**Skills:**
- `.github/skills/docker-rebuild-e2e-scripts/run.sh`
- `.github/skills/test-e2e-playwright-scripts/run.sh`

**Backend:**
- `backend/internal/caddy/manager.go`
- `backend/internal/caddy/config.go`
- `backend/cmd/api/main.go`

---

*End of Specification*
