# Phase 3: E2E Security Testing Plan

**Document Status:** Active Planning Phase
**Created:** February 10, 2026
**Target Execution:** Following Phase 2.3 Validation (Post-Approval)
**Approval Gate:** Supervisor Review + Security Team Sign-off

---

## 1. Executive Summary

### Objective
Validate that all security middleware components properly enforce their intended security policies during extended E2E test sessions. Phase 3 transforms theoretical security architecture into verified, tested operational security.

### Scope
- **Test Coverage:** Core, Settings, Tasks, Monitoring test suites with security enforcement validation
- **Middleware Stack:** Cerberus ACL, Coraza WAF, Rate Limiting, CrowdSec integration
- **Session Duration:** 60+ minute extended test runs with automatic token refresh
- **User Roles:** Admin, Regular User, Guest (role-based access validation)
- **Attack Vectors:** SQL injection, XSS, CSRF, DDoS, bot patterns, unauthorized access

### Duration & Resources
- **Estimated Execution Time:** 2-3 hours (includes 60-minute session test)
- **Test Count:** 60-90 distributed across 5 test suites
- **Infrastructure:** Docker container with all security modules enabled
- **Team:** Playwright QA Engineers + Security Infrastructure team

### Risk & Success Criteria

#### Success Criteria (PASS)
- ✅ All 60-90 security tests pass at 100% rate
- ✅ 60-minute session test completes without 401/403 errors
- ✅ All middleware properly logging security events
- ✅ No unauthorized access detected
- ✅ All attack vectors properly blocked
- ✅ Rate limiting enforced consistently

#### Failure Criteria (FAIL)
- ❌ Any security test fails (indicates bypass or misconfiguration)
- ❌ 401/403 errors during 60-minute test (session instability)
- ❌ Unauthorized access allowed (ACL bypass)
- ❌ Malicious requests not blocked (WAF bypass)
- ❌ Rate limit not enforced (abuse vulnerability)
- ❌ CrowdSec fails to block blacklisted IPs
- ❌ Data leakage between user roles

### Entry Criteria
- ✅ Phase 2.3 Critical Fixes Completed:
  - CVE-2024-45337 patched (golang.org/x/crypto updated)
  - InviteUser async email refactored (non-blocking)
  - Auth token refresh implemented (60+ min session support)
  - All validation gates passed (100% success rate)
- ✅ Docker environment ready with all security modules enabled
- ✅ Test database seeded with admin, user, and guest accounts
- ✅ Caddy reverse proxy configured with all plugins
- ✅ Cerberus ACL rules loaded
- ✅ Coraza WAF signatures up-to-date
- ✅ CrowdSec running with decision list synced

### Exit Criteria
- Phase 3 Go/No-Go decision made
- All test results documented in validation report
- Security middleware configurations logged
- Recommendations for Phase 4 (UAT/Integration) prepared

---

## 2. Test Environment Setup

### Pre-Execution Verification Checklist

#### Container & Infrastructure Readiness
- [ ] Docker image rebuilt with latest security modules
  - Golang base image with latest security patches
  - Caddy with Cerberus plugin
  - Coraza WAF signatures updated
- [ ] `charon-e2e` container running and healthy
- [ ] All required ports exposed:
  - `8080` - Application UI/API
  - `2019` - Caddy admin API
  - `2020` - Emergency server
- [ ] Health check passes for all services

#### Security Module Configuration
- [ ] **Cerberus ACL Module:** ENABLED
  - Admin role configured with full permissions
  - User role configured with limited permissions
  - Guest role configured with read-only permissions
  - All policies loaded and active
- [ ] **Coraza WAF Module:** ENABLED
  - OWASP ModSecurity Core Rule Set (CRS) loaded
  - Paranoia level configured (default: 2)
  - Log engine active
- [ ] **Rate Limiting Module:** ENABLED
  - Rate limit thresholds configured per endpoint
  - Storage backend (Redis) active if distributed
  - Headers configured for response
- [ ] **CrowdSec Integration:** ENABLED
  - CrowdSec service running
  - Decision list synced and populated
  - Bouncer middleware active in Caddy
  - Support plans activated (Community minimum)
  - **Verify with:**
    ```bash
    # Check CrowdSec decisions are populated (should show >0 decisions)
    docker exec charon-e2e cscli decisions list | head -20
    # Expected: List of IP/scenario decisions with counts > 0

    # Alternative if cscli not available:
    docker exec charon-e2e curl -s http://127.0.0.1:8081/v1/decisions \
      -H "X-Api-Key: $BOUNCER_KEY" | jq '.decisions | length'
    # Expected: Integer > 0 (number of decisions in database)

    # Whitelist test container IP in CrowdSec bouncer (avoids blocking test traffic)
    # Add to .docker/compose/.env or runtime:
    CROWDSEC_BOUNCER_WHITELIST="127.0.0.1/32,172.17.0.0/16"
    # Then verify bouncer config:
    docker exec charon-e2e grep -A5 "whitelist:" /etc/crowdsec/bouncers/caddy.yaml || echo "Config check complete"
    ```

#### Application Configuration
- [ ] Emergency token configured and validated
  - Format: Bearer token, 32+ characters
  - Used only for bootstrap/recovery operations
- [ ] Database state confirmed
  - Test users exist with correct roles
  - No test contamination from previous runs
  - Backup created for recovery
- [ ] Environment variables loaded
  - `.env` file configured for test environment
  - Security headers enabled
  - HTTPS/TLS properly configured
  - CORS rules appropriate for testing

#### Test User Configuration
```
TEST USERS REQUIRED:

1. Admin User
   - Username: admin@test.local
   - Password: [Securely stored in .env]
   - Role: Administrator
   - Permissions: Full access to all endpoints

2. Regular User
   - Username: user@test.local
   - Password: [Securely stored in .env]
   - Role: User
   - Permissions: Limited to personal data + read proxy hosts

3. Guest User
   - Username: guest@test.local
   - Password: [Securely stored in .env]
   - Role: Guest
   - Permissions: Read-only dashboard access

4. Rate Limit Test User
   - Username: ratelimit@test.local
   - Password: [Securely stored in .env]
   - Role: User
   - Purpose: Dedicated account for rate limit testing
```

#### Caddy Configuration Verification
```bash
# Verify all modules loaded
curl -s http://localhost:2019/config/apps/http/servers/default/routes

# Verify ACL policies are set
curl -s http://localhost:2019/config/apps/http/middleware/access_control_lists

# Verify WAF rules are set
curl -s http://localhost:2019/config/apps/http/middleware/waf

# Verify rate limits are set
curl -s http://localhost:2019/config/apps/http/middleware/rate_limit
```

#### Log Access & Monitoring
- [ ] Caddy logs accessible at `/var/log/caddy/` (or mounted volume)
- [ ] Application logs accessible at `/var/log/charon/`
- [ ] Security event logs separate and monitored
- [ ] Real-time log tailing available:
  ```bash
  docker logs -f charon-e2e
  docker exec charon-e2e tail -f /var/log/caddy/access.log
  ```

---

## 3. Cerberus ACL Testing (Access Control)

### Overview
Cerberus ACL module enforces role-based access control (RBAC) at the middleware layer. All API endpoints and protected resources require role verification before processing.

### Test Strategy

#### Admin Access Enforcement
- Verify admin users can access all protected endpoints
- Confirm admin portal loads with full permissions
- Validate admin can modify security settings
- Ensure admin operations logged

#### User Access Restrictions
- Verify regular users cannot access admin-only endpoints
- Confirm regular users receive 403 Forbidden for blocked endpoints
- Validate regular users can only access personal/assigned resources
- Ensure user can read but not modify advanced settings

#### Guest User Capabilities
- Verify guest users can view dashboard (read-only)
- Confirm guest cannot access settings or admin panels
- Validate guest cannot perform any write operations
- Ensure guest access properly logged

#### Role Transition Testing
- Test permission changes when role is updated
- Verify session updates reflect new permissions
- Confirm re-login required for permission elevation (admin check)

### Required Tests

#### API Endpoint Tests

**Admin-Only Endpoints** (Should return 200 for admin, 403 for others)
- [ ] `GET /api/v1/users` - List all users (admin only)
- [ ] `POST /api/v1/users` - Create user (admin only)
- [ ] `DELETE /api/v1/users/{id}` - Delete user (admin only)
- [ ] `GET /api/v1/access-lists` - View ACL policies (admin only)
- [ ] `POST /api/v1/access-lists` - Create ACL (admin only)
- [ ] `PUT /api/v1/settings/advanced` - Modify advanced settings (admin only)

**User-Accessible Endpoints** (Should return 200 for authenticated users)
- [ ] `GET /api/v1/users/me` - Get current user info
- [ ] `PUT /api/v1/users/me` - Update own profile
- [ ] `GET /api/v1/proxy-hosts` - List proxy hosts (read)
- [ ] `GET /api/v1/dashboard/stats` - View personal stats
- [ ] `GET /api/v1/logs?filter=personal` - View personal logs

**Guest-Readonly Endpoints** (Should return 200 for guest, data filtered)
- [ ] `GET /api/v1/dashboard` - View dashboard (limited data)
- [ ] `GET /api/v1/proxy-hosts` - List proxy hosts (read-only labels)
- [ ] Cannot perform: POST, PUT, DELETE operations

#### UI Navigation Tests

**Dashboard Access by Role**
- [ ] Admin dashboard loads with all widgets
- [ ] User dashboard loads with limited widgets
- [ ] Guest dashboard loads with read-only UI
- [ ] Settings page shows/hides fields by role
- [ ] Admin panel only accessible to admins
- [ ] Unauthorized access attempts redirected to 403 page

**Permission-Based UI Elements**
- [ ] Edit buttons hidden for read-only users
- [ ] Delete buttons hidden for non-admin users
- [ ] Advanced settings only visible to admins
- [ ] User invitation form only visible to admins
- [ ] Security settings only accessible to admins

#### Cross-Role Isolation Tests

**Data Visibility Boundaries**
- [ ] Admin user cannot access other admin's private logs
- [ ] User A cannot see User B's data
- [ ] Guest cannot see any user data
- [ ] API filters enforce role boundaries (not just UI)
- [ ] Database queries include role-based WHERE clauses

**Permission Elevation Prevention**
- [ ] User cannot elevate own role via API
- [ ] User cannot modify admin flag in API calls
- [ ] Guest cannot bypass to user via token manipulation
- [ ] Role changes require logout/re-login
- [ ] Token refresh does not grant elevated permissions

### Success Criteria

| Criterion | Expected | Test Count |
|-----------|----------|-----------|
| Admin access to protected endpoints | ✅ 200 OK for all 5 admin endpoints | 5 |
| User receives 403 for admin endpoints | ✅ 0 unauthorized access | 5 |
| Guest can view dashboard | ✅ Dashboard loads with filtered data | 3 |
| Role-based UI elements | ✅ Buttons/fields show/hide correctly | 8 |
| Cross-role data isolation | ✅ No data leakage in API responses | 6 |
| Permission elevation prevented | ✅ All attempts blocked | 4 |
| **Total Cerberus Tests** | | **31** |

### Phase 3A: Cerberus ACL Validation
**File:** `tests/phase3/cerberus-acl.spec.ts`
**Test Count:** 15-20 tests
**Risk Level:** MEDIUM (affects usability)
**Execution Duration:** ~10 minutes

---

## 4. Coraza WAF Testing (Web Application Firewall)

### Overview
Coraza WAF protects against malicious requests including SQL injection, XSS, CSRF, and other OWASP Top 10 vulnerabilities. Uses OWASP ModSecurity Core Rule Set (CRS).

### Test Strategy

#### SQL Injection Detection & Blocking
- Inject common SQL patterns into API parameters
- Attempt database enumeration via UNION SELECT
- Test time-based boolean blindness patterns
- Verify requests blocked with 403 Forbidden
- Confirm attack logged and attributed

#### Cross-Site Scripting (XSS) Prevention
- Submit JavaScript payload in form fields
- Test HTML entity encoding
- Attempt DOM-based XSS via API
- Verify malicious scripts blocked
- Confirm sanitization logs

#### CSRF Token Validation
- Attempt POST requests without CSRF token
- Verify token presence required for state-changing operations
- Test token expiration handling
- Confirm token rotation after use
- Validate mismatched tokens rejected

#### Malformed Request Handling
- Submit oversized payloads (>100MB)
- Send invalid Content-Type headers
- Test null byte injection
- Submit double-encoded payloads
- Verify safe error responses

#### WAF Rule Enforcement
- Verify OWASP CRS rules active
- Test anomaly score evaluation
- Confirm rule exceptions configured
- Validate phase-based rule execution
- Test logging of matched rules

### Required Tests

#### SQL Injection Tests
```
TEST CASES:
1. POST /api/v1/proxy-hosts with param:
   ?id=1' OR '1'='1
   EXPECTED: 403 Forbidden, logged as SQL_INJECTION

2. GET /api/v1/users?search=admin' UNION SELECT...
   EXPECTED: 403 Forbidden, mode=BLOCK

3. POST /api/v1/users name="'; DROP TABLE users; --"
   EXPECTED: 403 Forbidden, rules.matched logged

4. Malformed URL encoding: %2527 patterns
   EXPECTED: 403 Forbidden or 400 Bad Request (configurable)
```

#### XSS Payload Tests
```
TEST CASES:
1. POST /api/v1/proxy-hosts with body:
   {"description": "<script>alert('xss')</script>"}
   EXPECTED: 403 Forbidden, XSS rule matched

2. GET /api/v1/dashboard?filter=<img src=x onerror='alert()'>
   EXPECTED: 403 Forbidden, HTML attack pattern

3. Form field: <svg onload=fetch('http://attacker')>
   EXPECTED: 403 Forbidden, event handler detected

4. Attribute escape: '" onmouseover="alert()"
   EXPECTED: 403 Forbidden, quote escape patterns
```

#### CSRF & State-Changing Operations
```
TEST CASES:
1. DELETE /api/v1/users/1 without CSRF token
   EXPECTED: 403 Forbidden, CSRF_NO_TOKEN or 422

2. POST /api/v1/proxy-hosts with expired CSRF token
   EXPECTED: 403 Forbidden, token verification failed

3. PUT /api/v1/settings with invalid CSRF signature
   EXPECTED: 403 Forbidden, signature mismatch

4. Cross-origin OPTIONS preflight handling
   EXPECTED: 200 OK with proper CORS headers, CSRF exempt
```

#### Malformed Request Tests
```
TEST CASES:
1. POST 10MB payload (oversized)
   EXPECTED: 413 Payload Too Large

2. Content-Type: application/xml with JSON body
   EXPECTED: 415 Unsupported Media Type or 400

3. URL encoding with null bytes: %00
   EXPECTED: 403 Forbidden, null byte injection rule

4. Double-encoded: %252527 (%%27 = ')
   EXPECTED: 403 Forbidden or 400, depends on rules
```

#### Rate Limit + WAF Interaction
```
TEST CASE:
Rapid SQL injection attempts (10 in 1 second)
EXPECTED:
  - First 2-3: 403 by WAF (SQL detection)
  - Subsequent: 429 by Rate Limit (abuse pattern)
  - All logged separately (WAF vs Rate Limit)
```

### Success Criteria

| Criterion | Expected | Count |
|-----------|----------|-------|
| SQL injection attempts blocked | ✅ 100% blocked | 4 |
| XSS payloads rejected | ✅ 100% rejected | 4 |
| CSRF validation enforced | ✅ 0 CSRF bypasses | 4 |
| Malformed requests handled | ✅ Safe error responses | 4 |
| WAF logs capture all blocked requests | ✅ 100% logged | 5 |
| **Total Coraza WAF Tests** | | **21** |

### Phase 3B: Coraza WAF Validation
**File:** `tests/phase3/coraza-waf.spec.ts`
**Test Count:** 10-15 tests
**Risk Level:** CRITICAL (security)
**Execution Duration:** ~10 minutes

---

## 5. Rate Limiting Testing (Abuse Prevention)

### Overview
Rate limiting prevents brute force attacks, API abuse, and DoS by throttling requests per user/IP. Separate thresholds apply to different endpoints.

### Rate Limiting Configuration Reference

**Rate limiting uses GLOBAL per-user buckets** (not per-endpoint). Configuration is environment-driven and applied uniformly across all endpoints.

#### Global Rate Limit Configuration (Caddy-level)

| Parameter | Default | Environment Variables | Source Code |
|-----------|---------|----------------------|-------------|
| Requests per Window | 100 | `CERBERUS_SECURITY_RATELIMIT_REQUESTS` | `backend/internal/config/config.go:123` |
| Window Duration | 60s | `CERBERUS_SECURITY_RATELIMIT_WINDOW_SEC` | `backend/internal/config/config.go:124` |
| Burst Size | 10 | `CERBERUS_SECURITY_RATELIMIT_BURST` | `backend/internal/config/config.go:125` |
| Rate Limit Mode | `disabled` | `CERBERUS_SECURITY_RATELIMIT_MODE` | `backend/internal/config/config.go:122` |

**Implementation Details:**
- **Per-User Buckets:** Each authenticated user gets separate token bucket (JWT-based)
- **Global Ceiling:** All endpoints subject to SAME limit (not endpoint-specific)
- **Blocking Response:** HTTP 429 Too Many Requests with `Retry-After` header
- **Window Reset:** Bucket resets after window duration (default 60 seconds)
- **Bypass:** Admin whitelist defined in SecurityConfig (whitelisted IPs bypass all limits)

**Code Locations:**
- Configuration definition: `backend/internal/models/security_config.go:23-28`
- Configuration loading: `backend/internal/config/config.go:38-41, 122-123`
- Rate limit handler implementation: `backend/internal/cerberus/rate_limit.go:136-143`
- Unit tests with threshold values: `backend/internal/cerberus/rate_limit_test.go:198-200, 226-228, 287-289`
- Integration test script: `scripts/rate_limit_integration.sh:44-46`

#### Test Configuration Values
Integration tests use custom values for faster validation:
```
RATE_LIMIT_REQUESTS = 3 requests
RATE_LIMIT_WINDOW_SEC = 10 seconds
RATE_LIMIT_BURST = 1
Expected Behavior: Requests 1-3 return HTTP 200, Request 4 returns HTTP 429
```
Source: `scripts/rate_limit_integration.sh` (lines 44-46)

### Test Strategy

#### Login Rate Limiting
- Verify 5 failed login attempts allowed
- Confirm 6th attempt rate limited (429)
- Test account lockout vs throttling
- Verify exponential backoff if implemented
- Confirm rate limit reset after window expires

#### API Endpoint Rate Limiting
- Identify per-endpoint thresholds
- Exceed threshold and verify 429 response
- Confirm rate limit headers present
  - `X-RateLimit-Limit`
  - `X-RateLimit-Remaining`
  - `X-RateLimit-Reset`
- Test rate limit bucket reset
- Verify different users have separate counters

#### Resource-Intensive Operation Limiting
- Test POST /api/v1/backup (max 2 per hour)
- Verify 3rd backup request rejected
- Confirm recovery operations not limited (emergency token)
- Test concurrent backup attempts serialized
- Validate backup completion clears slot

#### Rate Limit Bypass Prevention
- Verify cannot bypass with different HTTP headers
- Test IP spoofing (X-Forwarded-For) detected
- Confirm rate limit enforced at proxy layer
- Test cannot reset limit via logout/re-login
- Verify distributed rate limiting (if multi-instance)

### Required Tests

#### Login Brute Force Prevention
```
SCENARIO: Prevent password guessing via repeated login attempts
TEST STEPS:
1. Attempt login 5 times with wrong password
   - Attempts 1-5: 401 Unauthorized
   - Each shows "Invalid credentials"
2. Attempt login 6th time
   - EXPECTED: 429 Too Many Requests
   - Message: "Rate limited, try again in 15 minutes"
3. Wait 15 minutes or reset
4. Login succeeds with correct password
   - EXPECTED: 200 OK + token
ACCEPTANCE: All 6 attempts logged, no system errors
```

#### API Endpoint Abuse Prevention
```
SCENARIO: Prevent API scraping via excessive requests
TEST STEPS:
1. GET /api/v1/users?limit=100 (60 times in 60 seconds)
   - Requests 1-60: 200 OK JSON response
   - Request 61: 429 Too Many Requests
   - Response includes: Retry-After header
2. GET /api/v1/proxy-hosts (different endpoint, 30 times)
   - Requests 1-30: 200 OK
   - Request 31: 429 Too Many Requests
3. Wait for window reset (varies by endpoint)
4. Requests succeed again
   - EXPECTED: 200 OK, counter reset
ACCEPTANCE: Separate counters per endpoint confirmed
```

#### Resource Creation Limiting
```
SCENARIO: Prevent rapid resource creation (backup spam)
TEST STEPS:
1. POST /api/v1/backup (request 1)
   - EXPECTED: 202 Accepted, backup started
2. POST /api/v1/backup (request 2, within 1 hour)
   - EXPECTED: 202 Accepted, second backup started
3. POST /api/v1/backup (request 3, within 1 hour)
   - EXPECTED: 429 Too Many Requests
   - Message: "Max 2 backups per hour, limit resets at [time]"
4. After 1 hour window:
5. POST /api/v1/backup (request 4)
   - EXPECTED: 202 Accepted, counter reset
ACCEPTANCE: Backup limit enforced, recovery time accurate
```

#### Multi-User Rate Limit Isolation
```
SCENARIO: Different users have separate rate limits
TEST STEPS:
1. User A: GET /api/v1/users (60 times in 1 minute)
   - Requests 1-60: 200 OK
   - Request 61: 429 Too Many Requests
2. User B: GET /api/v1/users (10 times, same minute)
   - EXPECTED: All 10 return 200 OK
   - Rate limit is PER USER, not GLOBAL
3. Verify: User A still rate limited, User B continues
ACCEPTANCE: Rate limits properly isolated per user/IP
```

#### Rate Limit Header Validation
```
SCENARIO: Clients receive rate limit information in headers
TEST STEPS:
1. GET /api/v1/users (request 1 of 60)
2. Check response headers:
   - X-RateLimit-Limit: 60
   - X-RateLimit-Remaining: 59 (or 59/60 depending on server)
   - X-RateLimit-Reset: [timestamp when resets]
   - Retry-After: Not present (not rate limited yet)
3. GET /api/v1/users (request 61 of 60)
4. Check response headers:
   - 429 Too Many Requests
   - X-RateLimit-Remaining: 0
   - Retry-After: [seconds until reset]
ACCEPTANCE: All headers populated correctly
```

### Success Criteria

| Criterion | Expected | Count |
|-----------|----------|-------|
| Login rate limited after 5 attempts | ✅ 6th returns 429 | 1 |
| API endpoints rate limited per threshold | ✅ Limits enforced | 4 |
| Resource creation limited (backups) | ✅ 3rd rejected | 1 |
| Multi-user rate limits isolated | ✅ Separate counters | 1 |
| Rate limit headers present & accurate | ✅ All headers valid | 3 |
| Limit resets after time window | ✅ Counter resets | 2 |
| **Total Rate Limiting Tests** | | **12** |

### Phase 3C: Rate Limiting Validation
**File:** `tests/phase3/rate-limiting.spec.ts`
**Test Count:** 10-15 tests
**Risk Level:** MEDIUM (abuse prevention)
**Execution Duration:** ~10 minutes
**Note:** Tests must run serially to avoid cross-test interference

---

## 6. CrowdSec Integration Testing (DDoS/Bot Mitigation)

### Overview
CrowdSec provides real-time threat detection and mitigation against DDoS, bot attacks, and malicious IP addresses. Integration via Caddy bouncer plugin.

### CrowdSec Architecture

#### Decision List
```
CrowdSec continuously syncs decision list containing:
- IP bans (from CrowdSec feed, community lists, custom)
- Country-based restrictions (if enabled)
- Behavioral rules (rapid requests, suspicious patterns)
```

#### Bouncer Integration
```
Caddy → Crowdsec Bouncer Plugin → Local decision cache
         (checks each request)
If IP in decisions:
  → 403 Forbidden (standard block)
  → 429 Too Many Requests (rate limit)
  → Other (custom decisions)
```

### Test Strategy

#### Blacklisted IP Blocking
- Identify IPs in CrowdSec decisions list
- Attempt access from blacklisted IP
- Verify all requests blocked (403/429)
- Confirm blocking is transparent (no error logs)
- Test whitelist bypass (whitelisted IPs bypass blocks)

#### Bot Pattern Detection
- Generate bot-like traffic patterns
  - Rapid requests from same IP
  - User-Agent containing bot patterns
  - Missing standard headers (no Referer, User-Agent)
- Verify behavior triggers CrowdSec detection
- Test automatic IP addition to decision list
- Confirm new requests from bot IP are blocked

#### Decision Cache Behavior
- Test decision propagation time (<1s)
- Verify cached decisions prevent repeated lookups
- Confirm cache invalidation on decision update
- Test cache doesn't bypass security

#### Legitimate Traffic Bypass
- Test whitelisted IPs/networks bypass blocks
- Verify health check IPs/endpoints bypass WAF
- Confirm emergency token bypasses rate limiting
- Test known good patterns allowed through

### Required Tests

#### Blacklist Enforcement
```
SCENARIO: Blocked IP cannot access application
SETUP:
1. Identify blacklisted IP from CrowdSec (or simulate)
2. Simulate request from that IP to application
TEST STEPS:
1. GET http://localhost:8080/api/v1/users from blacklisted IP
   - No valid auth token
   - EXPECTED: 403 Forbidden (before auth check)
2. GET with valid admin token from blacklisted IP
   - EXPECTED: Still 403 Forbidden (before endpoint reaches)
3. Verify all endpoints blocked (not just API)
   - GET / (root)
   - GET /dashboard
   - WebSocket connections
   EXPECTED: All blocked
ACCEPTANCE: Blacklist enforced at proxy layer
```

#### Bot Detection Patterns
```
SCENARIO: CrowdSec detects bot-like behavior and blocks
TEST STEPS:
1. Simulate bot behavior (rapid requests without Human headers)
   - Request 50 times in 10 seconds (5/sec)
   - No Referer header
   - No Accept-Language header
   - User-Agent: "python-requests" or "curl"
2. After X requests, check if CrowdSec triggers
   - EXPECTED: Requests start returning 429 or 403
3. Check decision list updated:
   - Query CrowdSec API: Verify IP added
   - Query Caddy logs: Verify bounce block logged
4. Continue requests from same IP
   - EXPECTED: All subsequent requests blocked
5. Requests from different IP
   - EXPECTED: Not affected by bot IP block
ACCEPTANCE: Bot behavior detected and mitigated
```

#### Decision Cache Validation
```
SCENARIO: CrowdSec decisions are cached locally for performance
TEST STEPS:
1. Request from blacklisted IP (forces cache lookup)
   - EXPECTED: Blocked in <10ms
2. Request again from same IP (uses cache)
   - EXPECTED: Blocked in <5ms (cache hit faster)
3. Update decision (e.g., remove from blacklist)
4. Wait for cache refresh (typically <30s)
5. Request again from previously-blacklisted IP
   - EXPECTED: Now allowed (cache refreshed)
ACCEPTANCE: Cache working, decisions update timely
```

#### Whitelist Bypass
```
SCENARIO: Whitelisted IPs bypass CrowdSec blocks
SETUP:
1. Assume health check IP is whitelisted
2. Assume localhost is whitelisted
TEST STEPS:
1. Blacklist an IP via CrowdSec
2. Request from blacklisted IP
   - EXPECTED: 403 Forbidden
3. Same request from whitelisted IP (if simulated)
   - EXPECTED: 200 OK (bypasses block)
4. Health check endpoint from any IP
   - EXPECTED: 200 OK (health checks whitelisted)
ACCEPTANCE: Whitelists work as configured
```

#### Request Pattern Variations
```
SCENARIO: Different request types trigger/bypass detection
TEST STEPS:
1. Rapid GET requests (50/min from one IP)
   - EXPECTED: Blocks after threshold
2. Mixed GET/POST/OPTIONS requests (50/min)
   - EXPECTED: Blocks after threshold
3. Varied User-Agents (rotate every request)
   - EXPECTED: Still detected as bot (IP-based blocking)
4. Varied request paths (different endpoints)
   - EXPECTED: Still detected as bot (aggregate pattern)
ACCEPTANCE: Detection based on aggregate behavior, not just patterns
```

### Success Criteria

| Criterion | Expected | Count |
|-----------|----------|-------|
| Blacklisted IPs blocked at all endpoints | ✅ 403 for all | 3 |
| Bot pattern detection triggers | ✅ Behavior detected | 2 |
| Decisions cached locally | ✅ Cache working | 2 |
| Whitelisted IPs bypass blocks | ✅ Allowed through | 2 |
| Decision updates propagate | ✅ <30s refresh | 1 |
| **Total CrowdSec Tests** | | **10** |

### Phase 3D: CrowdSec Integration Validation
**File:** `tests/phase3/crowdsec-integration.spec.ts`
**Test Count:** 8-12 tests
**Risk Level:** MEDIUM (DDoS mitigation)
**Execution Duration:** ~10 minutes
**Note:** May require special network setup or IP spoofing (use caution)

---

## 7. Authentication & Long-Session Testing

### Overview
Authentication flow including login, token refresh, session persistence over 60+ minute periods. Validates Phase 2.3c token refresh implementation.

### Token Refresh Architecture

#### Phase 2.3c Implementation
```
Access Token (JWT):
- Lifespan: 20 minutes
- Stored: Memory/localStorage (frontend)
- Refresh: Automatic every 18 minutes
- Contains: user_id, role, permissions, exp

Refresh Token (Secure HTTP-only Cookie):
- Lifespan: 60 days
- Stored: HttpOnly cookie (browser-managed)
- Transmission: Automatic with every request
- Contains: session_id, user_id, exp

Refresh Flow:
1. Access token expires in 18-20 minutes
2. Frontend detects expiration (via exp claim)
3. Frontend calls POST /api/v1/auth/refresh
4. Include refresh token (automatic from cookie)
5. Server validates refresh token, issues new access token
6. Frontend updates in-memory token
7. Continue operations (no 401 errors)
```

### Test Strategy

#### Login Flow Validation
- Verify username/password acceptance
- Confirm token generation (access + refresh)
- Validate token format and claims
- Test refresh token stored as HttpOnly cookie
- Verify access token stored securely (memory)

#### Token Refresh Mechanism
- Trigger automatic refresh at 18-minute mark
- Verify new token issued without user action
- Confirm refresh token rotates (for security)
- Test refresh on API call (lazy refresh)
- Validate refresh doesn't interrupt user activity

#### Session Persistence
- Verify session survives page navigation
- Test concurrent API calls with same session
- Confirm role-based permissions persist
- Validate data in localStorage/IndexedDB
- Test browser refresh doesn't logout

#### Long-Running Session (60+ minutes)
- Execute continuous test for 60+ minutes
- Perform API calls every 5-10 minutes
- Navigate UI elements periodically
- Verify no 401 errors during entire session
- Confirm all operations complete successfully
- Check token refresh logs for transparency

#### Logout & Cleanup
- Verify logout clears session
- Test refresh token invalidated
- Confirm re-login required after logout
- Validate logout from one tab affects others
- Test refresh token expiration honored

### Required Tests

#### Login & Token Generation
```
SCENARIO: User successfully logs in and receives tokens
TEST STEPS:
1. POST /api/v1/auth/login
   {
     "username": "admin@test.local",
     "password": "SecurePass123!"
   }
2. Verify response:
   {
     "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
     "token_type": "Bearer",
     "expires_in": 1200,  // 20 minutes in seconds
     "refresh_token": "[not in response, in cookie]"
   }
3. Check response headers:
   - Set-Cookie: refresh_token=[...];
     HttpOnly; Secure; SameSite=Strict;
     Max-Age=5184000
4. Decode JWT and verify claims:
   {
     "sub": "user_id_123",
     "name": "admin@test.local",
     "role": "Administrator",
     "exp": 1707563400,
     "iat": 1707562200
   }
ACCEPTANCE: Tokens correctly generated, secure storage
```

#### Token Refresh Mechanism
```
SCENARIO: Token refresh occurs automatically without user action
TEST STEPS:
1. Login successfully (get access token)
2. Store token and timestamp
3. Wait 18 minutes (or simulate time)
4. Make API call before token expires:
   GET /api/v1/users
   Header: Authorization: Bearer [old_token]
5. On 401 (if token is stale):
   a. Automatically call POST /api/v1/auth/refresh
   b. Include refresh token (automatic from cookie)
   c. Receive new access_token
6. Retry original API call with new token:
   GET /api/v1/users
   Header: Authorization: Bearer [new_token]
   - EXPECTED: 200 OK
7. Verify new token different from old:
   - old_token != new_token
   - Both valid, new has later exp claim
ACCEPTANCE: Auto-refresh transparent to user
```

#### 60-Minute Long Session
```
SCENARIO: User session remains active for 60+ minutes without errors
DURATION: 60 minutes minimum
TEST STEPS:

[Minute 0-5]
1. Login as admin@test.local
2. GET /api/v1/users (verify auth works)
   EXPECTED: 200 OK, user list returned

[Minute 10]
3. Navigate to dashboard in UI
4. Check dashboard loads (no 401 errors)
   EXPECTED: Dashboard renders correctly

[Minute 15-20]
5. API call before token refresh needed:
   GET /api/v1/proxy-hosts
   EXPECTED: 200 OK (token still valid)

[Minute 20-25] ← AUTO-REFRESH TRIGGER
6. Token automatically refreshes (no user action)
7. Continue API calls:
   POST /api/v1/proxy-hosts (create new)
   EXPECTED: 200/201 (refresh transparent)

[Minute 30]
8. Navigate to Settings in UI
9. Load advanced settings (admin-only)
   EXPECTED: 200 OK (permissions still valid)

[Minute 35-40]
10. Another API call (another refresh cycle):
    GET /api/v1/logs
    EXPECTED: 200 OK

[Minute 40-50]
11. Rapid API calls (simulate heavy usage):
    - 10 GET requests to different endpoints
    - No delay between requests
    EXPECTED: All 200 OK (no rate limit/auth issues)

[Minute 50-55]
12. UI navigation and page reload:
    - Click settings → dashboard → proxy hosts
    - Refresh page (F5/cmd+R)
    - Verify session persists
    EXPECTED: No logout, session intact

[Minute 55-60]
13. Final API calls:
    - GET /api/v1/users/me (verify identity)
    - PUT /api/v1/users/me (update profile)
    EXPECTED: Both succeed with correct user data

[Minute 60+]
14. Logout
    POST /api/v1/auth/logout
    EXPECTED: 200 OK, session cleared

15. Attempt to reuse old token:
    GET /api/v1/users with old token
    EXPECTED: 401 Unauthorized (token invalidated)

ACCEPTANCE CRITERIA:
✅ 0 x 401 errors during 60-minute session
✅ 0 x 403 errors (permissions maintained)
✅ 0 x token expiration errors (refresh silent)
✅ 100% of API calls successful
✅ UI remains responsive throughout
✅ Token refresh logs show 3+ refresh cycles
✅ **NEW:** Heartbeat logs generated every 10 minutes showing:
  - `✓ [Heartbeat N] Min X: Context. Token expires: TIMESTAMP`
  - No ✗ failures in heartbeat log
  - Token expiry time advances every ~20 minutes (auto-refresh working)

**Heartbeat Monitoring:** See Section 10 "60-Minute Session Test Heartbeat Monitoring" for:
  - TypeScript code snippet for periodic health checks
  - Bash command to monitor progress in real-time
  - Expected log format and success criteria
```

#### Token Validity & Expiration
```
SCENARIO: Expired tokens are properly rejected
TEST STEPS:
1. Let access token expire naturally (20+ minutes)
2. Attempt API call with expired token:
   GET /api/v1/users
   Authorization: Bearer [expired_token]
   EXPECTED: 401 Unauthorized
3. Check error details:
   {
     "error": "invalid_or_expired_token",
     "error_description": "Token has expired",
     "error_code": 1003
   }
4. Attempt refresh with expired token:
   (Refresh token still valid)
   EXPECTED: 200 OK, new token issued
ACCEPTANCE: Expiration properly enforced
```

### Success Criteria

| Criterion | Expected | Count |
|-----------|----------|-------|
| Login generates both token types | ✅ Access + Refresh tokens | 1 |
| Token claims valid & include role | ✅ Claims verified | 1 |
| Refresh token secure (HttpOnly) | ✅ Cookie configured | 1 |
| Auto-refresh at ~18 minutes | ✅ New token issued | 1 |
| 60-minute session zero 401 errors | ✅ 0 x 401 | 1 |
| Session persists across page reloads | ✅ Session intact | 1 |
| Token expiration rejected | ✅ 401 response | 1 |
| Logout invalidates tokens | ✅ Refresh token revoked | 1 |
| Concurrent sessions isolated | ✅ Separate sessions | 2 |
| **Total Auth & Session Tests** | | **10** |

### Phase 3E: Authentication & Session Validation
**File:** `tests/phase3/security-enforcement.spec.ts` (core suite)
**Test Count:** 20-30 tests (includes long-session test)
**Risk Level:** HIGH (critical for uptime)
**Execution Duration:** 60+ minutes (includes long session)

---

## 8. Authorization Testing (Role-Based Access)

### Overview
Authorization applies role-based access rules to ensure users can only perform actions and view data their role permits. Complements ACL testing with data visibility and permission cascading.

### Authorization Matrix

#### Admin Role
```
Permissions:
- Users:           Create, Read, Update, Delete
- Proxy Hosts:     Create, Read, Update, Delete
- Access Lists:    Create, Read, Update, Delete
- Settings:        Read, Update (all fields)
- Logs:            Create, Read (all users' logs)
- Backups:         Create, Read, Delete
- Dashboard:       Full access all metrics
- Reports:         Generate, Read, Delete (all)
- Audit Trail:     Read (all)

API Endpoints:    All endpoints accessible
UI Pages:         All pages accessible
Advanced Settings: Full visibility and edit
```

#### User Role
```
Permissions:
- Users:           Read (own profile), Update (own profile only)
- Proxy Hosts:     Create, Read, Update (own only), Delete (own only)
- Access Lists:    Read (view existing, cannot create)
- Settings:        Read (cannot modify)
- Logs:            Read (own logs only)
- Backups:         Read (full list)
- Dashboard:       Limited metrics (only relevant proxies)
- Reports:         Read (own reports)
- Audit Trail:     Read (limited to own actions)

API Endpoints:    Limited set accessible
UI Pages:         Dashboard, Proxy Hosts, Logs (personal), Profile
Advanced Settings: Not visible
```

#### Guest Role
```
Permissions:
- Users:           None
- Proxy Hosts:     Read (labels only, no config)
- Access Lists:    None
- Settings:        None
- Logs:            None
- Backups:         None
- Dashboard:       Read-only metrics
- Reports:         None
- Audit Trail:     None

API Endpoints:    None (GET / only)
UI Pages:         Dashboard (read-only)
Advanced Settings: Not visible
Actions:          View only, no modifications
```

### Test Strategy

#### Dashboard Access by Role
- Admin dashboard shows all widgets and metrics
- User dashboard shows only relevant proxies
- Guest dashboard shows minimal metrics
- Disabled widgets for unauthorized roles

#### API Data Filtering
- Admin sees all data in API responses
- User sees only owned/assigned data
- Guest sees only public/read-only data
- Queries include role-based WHERE filters

#### Permission Cascading
- Parent resource permission controls child access
- Permission inheritance follows object hierarchy
- Cascading deletions respect permission checks
- Bulk operations verify all items authorized

#### Real-Time Permission Updates
- Permission changes immediate on next request
- No caching hiding permission changes
- Role changes require revalidation
- Token refresh doesn't grant new permissions

### Required Tests

#### Dashboard Widget Visibility
```
SCENARIO: Different roles see different dashboard widgets
TEST STEPS:

AS ADMIN:
1. Login as admin@test.local
2. Navigate to dashboard
3. Verify all widgets visible:
   - User Count Card ✓
   - Active Proxy Hosts Card ✓
   - System Stats (CPU, Memory, Disk) ✓
   - Recent Activities ✓
   - Security Events ✓
   - Backup Status ✓
   - Alert/Warning Messages ✓
4. Click each widget:
   - All load without 403 errors
   - Data shows aggregated/all users
   EXPECTED: 100% widget visibility

AS USER:
1. Login as user@test.local
2. Navigate to dashboard
3. Verify limited widgets visible:
   - User's Proxy Hosts ✓
   - Personal Stats ✓
   - Recent Logs (own only) ✓
4. Verify hidden widgets:
   - User Count Card ✗ (hidden)
   - System Stats ✗ (hidden)
   - Security Events ✗ (hidden)
   - Backup Status ✗ (hidden)
5. Click hidden widget areas:
   - Either not clickable or return 403
   EXPECTED: Limited visibility, no data leakage

AS GUEST:
1. Login as guest@test.local
2. Navigate to dashboard
3. Verify minimal widgets:
   - Proxy Hosts List (read-only labels) ✓
   - General Help/Info ✓
4. All action buttons disabled:
   - Edit buttons ✗
   - Delete buttons ✗
   - Create buttons ✗
   EXPECTED: Read-only experience
```

#### API Data Filtering
```
SCENARIO: API responses filtered by user role
TEST STEPS:

AS ADMIN:
1. GET /api/v1/proxy-hosts
2. Response includes:
   [
     {"id": 1, "name": "Proxy1", "owner_id": 1},
     {"id": 2, "name": "Proxy2", "owner_id": 2},
     {"id": 3, "name": "Proxy3", "owner_id": 1}
   ]
   → All proxies visible (owned or not)

AS USER (with id=2):
1. GET /api/v1/proxy-hosts
2. Response includes:
   [
     {"id": 2, "name": "Proxy2", "owner_id": 2}
   ]
   → Only user's own proxies
   → No other users' proxies visible

3. Attempt GET /api/v1/proxy-hosts/1 (owner_id=1)
   EXPECTED: 403 Forbidden (no access)

AS GUEST:
1. GET /api/v1/proxy-hosts
2. Response includes:
   [
     {"id": 1, "name": "Proxy1"},  // only name, no detail
     {"id": 2, "name": "Proxy2"}
   ]
   → Labels only, no secrets/config visible
```

#### Write Permission Enforcement
```
SCENARIO: Only authorized roles can modify resources
TEST STEPS:

AS ADMIN:
1. POST /api/v1/proxy-hosts { new proxy config }
   EXPECTED: 201 Created

2. PUT /api/v1/proxy-hosts/1 { update config }
   EXPECTED: 200 OK (update succeeds)

3. DELETE /api/v1/proxy-hosts/1
   EXPECTED: 204 No Content (delete succeeds)

AS USER (with id=2, owner of proxy 2):
1. POST /api/v1/proxy-hosts { new proxy }
   EXPECTED: 201 Created (can create own)

2. PUT /api/v1/proxy-hosts/2 { update own }
   EXPECTED: 200 OK (can update own)

3. PUT /api/v1/proxy-hosts/1 { update other's }
   EXPECTED: 403 Forbidden (cannot modify)

4. DELETE /api/v1/proxy-hosts/1
   EXPECTED: 403 Forbidden (cannot delete)

AS GUEST:
1. POST /api/v1/proxy-hosts { new proxy }
   EXPECTED: 403 Forbidden (no create permission)

2. PUT /api/v1/proxy-hosts/1 { any update }
   EXPECTED: 403 Forbidden (no update permission)

3. DELETE /api/v1/proxy-hosts/1
   EXPECTED: 403 Forbidden (no delete permission)
```

#### Settings Access Control
```
SCENARIO: Role determines settings visibility and editability
TEST STEPS:

AS ADMIN (in UI):
1. Navigate to Settings
2. Tabs visible:
   - General ✓
   - Security ✓
   - Advanced ✓
   - API Keys ✓
3. All fields editable:
   - Save button activates after change
   - API call succeeds

AS USER (in UI):
1. Navigate to Settings
2. Tabs visible:
   - General ✓
   - User Account ✓
3. Tabs NOT visible:
   - Security ✗
   - Advanced ✗
   - API Keys ✗
4. Editable fields limited:
   - Name: editable
   - Email: editable
   - Permission fields: read-only (no save option)

AS GUEST:
1. Navigate to Settings
2. Either:
   a. Page does not load (403)
   b. Page loads but all read-only
   EXPECTED: Cannot modify anything
```

#### Permission Escalation Prevention
```
SCENARIO: Users cannot elevate own permissions
TEST STEPS:

AS USER:
1. Attempt to modify own role via API:
   PUT /api/v1/users/me
   {
     "name": "user@test.local",
     "role": "Administrator"
   }
   EXPECTED: 403 Forbidden or role ignored (reverted to User)

2. Attempt to create new admin:
   POST /api/v1/users
   {
     "email": "neward@test.local",
     "role": "Administrator"
   }
   EXPECTED: 403 Forbidden (cannot create admins)

3. Attempt to grant permission via API key:
   POST /api/v1/users/me/api-keys
   {
     "name": "admin_key",
     "role": "Administrator"
   }
   EXPECTED: 403 Forbidden or role limited to User

VERIFY: Database confirms role unchanged
```

### Success Criteria

| Criterion | Expected | Count |
|-----------|----------|-------|
| Admin sees all dashboard widgets | ✅ 100% visible | 1 |
| User sees limited widgets | ✅ ~50% hidden | 1 |
| Guest sees read-only dashboard | ✅ All buttons disabled | 1 |
| API filters data by role | ✅ Correct filtering | 3 |
| Write operations role-checked | ✅ 0 unauthorized writes | 4 |
| Settings visibility matches role | ✅ Proper tabs shown | 3 |
| Permission escalation blocked | ✅ All attempts rejected | 3 |
| **Total Authorization Tests** | | **16** |

---

## 9. Test Suite Organization

### Test File Structure

#### Phase 3 Test Directory Layout
```
/projects/Charon/tests/phase3/
├── security-enforcement.spec.ts      # Core auth & 60-min session
├── cerberus-acl.spec.ts              # Role-based access control
├── coraza-waf.spec.ts                # Attack prevention
├── rate-limiting.spec.ts             # Abuse prevention
├── crowdsec-integration.spec.ts      # DDoS/Bot mitigation
├── fixtures/
│   ├── test-users.ts                 # User creation & management
│   ├── security-payloads.ts          # Attack patterns
│   ├── test-data.ts                  # API test data
│   └── helpers.ts                    # Common utilities
└── README.md                          # Phase 3 test documentation
```

### Test Suite Breakdown

#### Phase 3A: Security Enforcement (Core)
**File:** `tests/phase3/security-enforcement.spec.ts`

| Category | Test Count | Priority | Risk |
|----------|-----------|----------|------|
| Login & Token Generation | 3 | P0 | HIGH |
| Token Refresh Mechanism | 3 | P0 | HIGH |
| 60-Minute Long Session | 1 | P0 | CRITICAL |
| Logout & Cleanup | 2 | P1 | MEDIUM |
| Concurrent Sessions | 2 | P1 | MEDIUM |
| **Subtotal** | **11** | — | — |

**Test Organization:**
```typescript
describe('Phase 3: Security Enforcement (Core Suite)', () => {
  describe('Authentication > Login & Token Generation', () => {
    test('Admin login receives access + refresh tokens', { slow: true })
    test('Login returns proper JWT claims', { slow: true })
    test('Refresh token stored as secure HttpOnly cookie', { slow: true })
  })

  describe('Authentication > Token Refresh', () => {
    test('Auto-refresh triggered at 18-minute mark', { slow: true })
    test('Refresh transparent to user (no 401 errors)', { slow: true })
    test('New token issued with updated exp claim', { slow: true })
  })

  describe('Session > 60-Minute Long-Running', () => {
    test('60-minute session completes without 401 errors', { slow: true, timeout: '70m' })
  })

  describe('Session > Logout & Cleanup', () => {
    test('Logout invalidates refresh token', { slow: true })
    test('Re-login after logout works correctly', { slow: true })
  })

  describe('Session > Concurrent Sessions', () => {
    test('Multiple users have isolated sessions', { slow: true })
    test('Logout in one session does not affect others', { slow: true })
  })
})
```

#### Phase 3B: Cerberus ACL Testing
**File:** `tests/phase3/cerberus-acl.spec.ts`

| Category | Test Count | Priority | Risk |
|----------|-----------|----------|------|
| Admin Access | 5 | P0 | HIGH |
| User Restrictions | 5 | P0 | HIGH |
| Guest Capabilities | 3 | P1 | MEDIUM |
| Role Transitions | 2 | P1 | MEDIUM |
| Cross-Role Isolation | 6 | P0 | CRITICAL |
| Permission Elevation Prevention | 4 | P0 | CRITICAL |
| **Subtotal** | **25** | — | — |

**Execution Duration:** ~10 minutes
**Risk Level:** MEDIUM

#### Phase 3C: Coraza WAF Protection
**File:** `tests/phase3/coraza-waf.spec.ts`

| Category | Test Count | Priority | Risk |
|----------|-----------|----------|------|
| SQL Injection Detection | 4 | P0 | CRITICAL |
| XSS Prevention | 4 | P0 | CRITICAL |
| CSRF Validation | 4 | P1 | HIGH |
| Malformed Requests | 4 | P1 | MEDIUM |
| WAF Logging | 5 | P2 | LOW |
| **Subtotal** | **21** | — | — |

**Execution Duration:** ~10 minutes
**Risk Level:** CRITICAL (security)

#### Phase 3D: Rate Limiting Enforcement
**File:** `tests/phase3/rate-limiting.spec.ts`

| Category | Test Count | Priority | Risk |
|----------|-----------|----------|------|
| Login Brute Force | 1 | P0 | HIGH |
| API Endpoint Limits | 4 | P0 | HIGH |
| Resource Creation Limits | 1 | P1 | MEDIUM |
| Multi-User Isolation | 1 | P0 | HIGH |
| Rate Limit Headers | 3 | P1 | MEDIUM |
| Limit Reset Behavior | 2 | P1 | MEDIUM |
| **Subtotal** | **12** | — | — |

**Execution Duration:** ~10 minutes
**Risk Level:** MEDIUM
**Constraint:** Must run serially (rate limits interfere with parallel execution)

#### Phase 3E: CrowdSec Integration
**File:** `tests/phase3/crowdsec-integration.spec.ts`

| Category | Test Count | Priority | Risk |
|----------|-----------|----------|------|
| Blacklist Enforcement | 3 | P0 | CRITICAL |
| Bot Detection | 2 | P0 | HIGH |
| Decision Caching | 2 | P1 | MEDIUM |
| Whitelist Bypass | 2 | P1 | MEDIUM |
| Pattern Variations | 1 | P2 | LOW |
| **Subtotal** | **10** | — | — |

**Execution Duration:** ~10 minutes
**Risk Level:** MEDIUM
**Note:** May require network-level testing or IP spoofing

### Test Count Summary

| Suite | File | Tests | Duration | Priority |
|-------|------|-------|----------|----------|
| Core Security | security-enforcement.spec.ts | 11 | 60+ min | P0/P1 |
| Cerberus ACL | cerberus-acl.spec.ts | 25 | 10 min | P0/P1 |
| Coraza WAF | coraza-waf.spec.ts | 21 | 10 min | P0/P1 |
| Rate Limiting | rate-limiting.spec.ts | 12 | 10 min | P0/P1 |
| CrowdSec | crowdsec-integration.spec.ts | 10 | 10 min | P0/P1 |
| **TOTAL** | | **79** | **~100 min** | — |

### Test Execution Order

```bash
# Phase 3 Test Execution Sequence:

# 1. Start E2E Environment (one-time setup)
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e

# 2. Run Core Security Tests (CRITICAL - foundation for others)
npx playwright test tests/phase3/security-enforcement.spec.ts \
  --project=firefox --reporter=html

# 3. Run Cerberus ACL Tests (MEDIUM - authorization checks)
npx playwright test tests/phase3/cerberus-acl.spec.ts \
  --project=firefox --reporter=html

# 4. Run Coraza WAF Tests (CRITICAL - attack prevention)
npx playwright test tests/phase3/coraza-waf.spec.ts \
  --project=firefox --reporter=html

# 5. Run Rate Limiting Tests (MEDIUM - run serially)
npx playwright test tests/phase3/rate-limiting.spec.ts \
  --project=firefox --reporter=html \
  --workers=1  # Serial execution required

# 6. Run CrowdSec Tests (MEDIUM - DDoS mitigation)
npx playwright test tests/phase3/crowdsec-integration.spec.ts \
  --project=firefox --reporter=html

# 7. Generate Combined Report
npx playwright show-report

# Total Execution Time: ~100 minutes (includes 60-min session test)
```

### Why Serial Execution?

Rate limiting tests and WAF tests must run serially because:
1. **Rate Limiting:** Rapid parallel requests from the same test session hit rate limits
   - Solution: Execute rate-limiting suite with `--workers=1`
2. **WAF Testing:** Multiple rapid attack pattern submissions from same IP/session may trigger CrowdSec
   - Mitigation: Space out WAF tests or use different test users per request
3. **CrowdSec Decisions:** Decisions may take time to propagate
   - Solution: Add delays between related tests

**Other suites (Security Enforcement, Cerberus, CrowdSec) can run in parallel** if needed to accelerate execution.

### Parallel Execution Alternative (Faster, Less Safe)

```bash
# Fast execution (all tests parallel) - NOT RECOMMENDED
npx playwright test tests/phase3/ \
  --project=firefox --reporter=html \
  --grep="Phase3"  # Tag all Phase 3 tests

# This may cause:
# ⚠️ False failures due to rate limits crossing tests
# ⚠️ WAF blocking benign requests as DDoS
# ⚠️ CrowdSec blocking test IP across multiple tests
```

---

## 10. Execution Strategy

### Pre-Test Verification Checklist

#### Infrastructure Readiness (10 minutes)

- [ ] **Docker Container:**
  ```bash
  docker ps | grep charon-e2e
  # Output: charon-e2e container running

  docker exec charon-e2e curl -s http://localhost:8080/health
  # Output: {"status":"ok"} or {"healthy":true}
  ```

- [ ] **Security Modules Status:**
  ```bash
  # Check Cerberus ACL
  curl -s http://localhost:2019/config/apps/http/middleware | grep -i acl

  # Check Coraza WAF
  curl -s http://localhost:2019/config/apps/http/middleware | grep -i waf

  # Check Rate Limiting
  curl -s http://localhost:2019/config/apps/http/middleware | grep -i rate

  # Check CrowdSec Bouncer
  curl -s http://localhost:2019/config/apps/http/middleware | grep -i crowdsec
  ```

- [ ] **Emergency Token Configuration:**
  ```bash
  # Verify token exists in environment
  docker exec charon-e2e grep EMERGENCY_TOKEN .env
  # Output: EMERGENCY_TOKEN=<token>...

  # Test token validity
  curl -s -X POST http://localhost:8080/api/v1/auth/validate \
    -H "Authorization: Bearer $EMERGENCY_TOKEN"
  # Output: {"valid":true}
  ```

- [ ] **Database State:**
  ```bash
  # Verify test users exist
  docker exec charon-e2e sqlite3 data/charon.db \
    "SELECT email, role FROM users ORDER BY email;"

  # Output should include:
  # admin@test.local|Administrator
  # user@test.local|User
  # guest@test.local|Guest
  # ratelimit@test.local|User
  ```

- [ ] **Test User Credentials Verified:**
  ```bash
  # Test admin login
  curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@test.local","password":"AdminPass123!"}'
  # Output: {"access_token":"...", "token_type":"Bearer"}
  ```

- [ ] **Log Directories Accessible:**
  ```bash
  docker exec charon-e2e ls -la /var/log/caddy/ /var/log/charon/ 2>/dev/null
  # All log directories exist and writable
  docker exec charon-e2e find /var/log -name "*.log" -type f
  ```

### Serial Execution Plan

#### Execution Phase 1: Core Security Tests (60+ minutes)
```bash
# Start time: [Record]

# 1. Run Core Security Test Suite
npx playwright test tests/phase3/security-enforcement.spec.ts \
  --project=firefox \
  --reporter=html \
  --output-folder="test-results/phase3-core" \
  2>&1 | tee logs/phase3-core-execution.log

# Expected: High test count, long duration (60-min session test)
# Completion time: [Record]
```

**Monitoring During Execution:**
```bash
# In separate terminal - Monitor API calls
docker exec charon-e2e tail -f /var/log/caddy/access.log | \
  grep -E "(401|403|500)" | \
  tee logs/phase3-core-errors.log

# In separate terminal - Monitor token refresh
docker exec charon-e2e tail -f /var/log/charon/security.log | \
  grep -i "token\|refresh\|session" | \
  tee logs/phase3-core-tokens.log
```

**60-Minute Session Test Heartbeat Monitoring (NEW):**

For the 60+ minute long-running session test, implement periodic health checks to ensure the session remains active and token refresh is working. This allows QA to monitor progress in real-time.

*TypeScript Code Snippet for Playwright Test:*
```typescript
// tests/phase3/security-enforcement.spec.ts - 60-minute session test heartbeat

const SESSION_DURATION_MS = 60 * 60 * 1000;  // 60 minutes
const HEARTBEAT_INTERVAL_MS = 10 * 60 * 1000; // 10 minutes
const startTime = Date.now();
let heartbeatCount = 0;

// Helper: Get token expiration from JWT
function getTokenExpiry(token: string): number {
  const parts = token.split('.');
  if (parts.length !== 3) return 0;
  try {
    const payload = JSON.parse(atob(parts[1]));
    return payload.exp ? payload.exp * 1000 : 0; // Convert to milliseconds
  } catch {
    return 0;
  }
}

// Helper: Log heartbeat every 10 minutes
async function logHeartbeat(page: Page, context: string): Promise<void> {
  heartbeatCount++;
  const elapsed = Math.floor((Date.now() - startTime) / 1000 / 60); // minutes
  const token = await page.evaluate(() => localStorage.getItem('access_token'));
  const expiryMs = token ? getTokenExpiry(token) : 0;
  const expiryTime = expiryMs > 0
    ? new Date(expiryMs).toISOString()
    : 'unknown';

  const heartbeatMsg = `✓ [Heartbeat ${heartbeatCount}] Min ${elapsed}: ${context}. Token expires: ${expiryTime}`;
  console.log(heartbeatMsg);

  // Write to file for log analysis
  const fs = require('fs');
  const logDir = 'logs';
  if (!fs.existsSync(logDir)) fs.mkdirSync(logDir);
  fs.appendFileSync(`${logDir}/session-heartbeat.log`, heartbeatMsg + '\n');
}

// In test: Log heartbeat every 10 minutes during 60-minute session
test('60-minute session with automatic token refresh and heartbeat',
  { timeout: '70m' }, // 70-minute timeout to allow for test overhead
  async ({ page, browser }) => {
    const testStartTime = Date.now();

    // Initial login
    await page.goto('http://localhost:8080');
    await page.fill('[name="email"]', 'admin@test.local');
    await page.fill('[name="password"]', 'AdminPass123!');
    await page.click('button:has-text("Login")');
    await page.waitForNavigation();

    // Initial heartbeat
    await logHeartbeat(page, 'Initial login successful');

    // Run heartbeat loop every 10 minutes
    const heartbeatTimer = setInterval(async () => {
      try {
        // Verify session still active via API call
        const response = await page.evaluate(async () => {
          const token = localStorage.getItem('access_token');
          return fetch('/api/v1/users/me', {
            headers: { Authorization: `Bearer ${token}` }
          }).then(r => ({ status: r.status, ok: r.ok }));
        });

        if (response.ok) {
          await logHeartbeat(page, 'API health check OK');
        } else {
          console.warn(`⚠ [Heartbeat ${heartbeatCount}] API returned ${response.status}`);
        }
      } catch (err) {
        console.error(`✗ [Heartbeat ${heartbeatCount}] Error: ${err.message}`);
      }
    }, HEARTBEAT_INTERVAL_MS);

    try {
      // Run session activities for 60 minutes
      // (navigate, make API calls, interact with UI)
      const endTime = testStartTime + SESSION_DURATION_MS;
      let iteration = 0;

      while (Date.now() < endTime) {
        iteration++;

        // Periodically navigate and make API calls
        if (iteration % 2 === 0) {
          // Navigate to different pages
          await page.goto('http://localhost:8080/dashboard');
          await page.waitForLoadState('networkidle');
        } else {
          // Make API call
          const token = await page.evaluate(() => localStorage.getItem('access_token'));
          const response = await page.evaluate(async (token) => {
            return fetch('/api/v1/users', {
              headers: { 'Authorization': `Bearer ${token}` }
            }).then(r => r.status);
          }, token);
        }

        // Wait 5 minutes between iterations
        await page.waitForTimeout(5 * 60 * 1000);
      }

      // Final heartbeat
      await logHeartbeat(page, 'Session completed successfully');

    } finally {
      clearInterval(heartbeatTimer);
    }

    // Verify session integrity at end
    const finalToken = await page.evaluate(() => localStorage.getItem('access_token'));
    expect(finalToken).toBeTruthy();
    expect(finalToken?.length).toBeGreaterThan(100); // JWT sanity check
  }
);
```

*Bash Command for Real-Time Monitoring During Test Execution:*
```bash
#!/usr/bin/env bash
# Run in separate terminal while 60-minute test is executing

# Monitor heartbeat log in real-time
echo "=== Monitoring 60-Minute Session Test Progress ==="
echo "Press Ctrl+C to stop monitoring"
echo ""

# Create log directory if needed
mkdir -p logs

# Monitor heartbeat log with timestamps
(tail -f logs/session-heartbeat.log 2>/dev/null &) | while IFS= read -r line; do
  echo "[$(date +'%H:%M:%S')] $line"
done

# Alternative: Watch for errors in real-time
echo ""
echo "=== Errors/Warnings (if any) ==="
grep -E "✗|⚠|error|failed" logs/session-heartbeat.log 2>/dev/null || echo "No errors detected"

# Alternative: Show token refresh frequency
echo ""
echo "=== Token Refresh Count ==="
grep -c "Token expires:" logs/session-heartbeat.log 2>/dev/null || echo "0 refreshes detected"
```

**Integration into Test Execution:**
1. Add heartbeat code to `tests/phase3/security-enforcement.spec.ts` (60-minute test)
2. Run test: `npx playwright test tests/phase3/security-enforcement.spec.ts -g "60-minute"`
3. **In separate terminal:** Run bash monitoring command above
4. **Expected log output example:**
   ```
   ✓ [Heartbeat 1] Min 10: Initial login successful. Token expires: 2026-02-10T08:35:42Z
   ✓ [Heartbeat 2] Min 20: API health check OK. Token expires: 2026-02-10T08:45:12Z
   ✓ [Heartbeat 3] Min 30: API health check OK. Token expires: 2026-02-10T08:55:18Z
   ⚠ [Heartbeat 4] Min 40: API returned 401 (indicates token refresh failure)
   ✓ [Heartbeat 5] Min 50: API health check OK. Token expires: 2026-02-10T09:15:30Z
   ✓ [Heartbeat 6] Min 60: Session completed successfully. Token expires: 2026-02-10T09:25:44Z
   ```
5. **Success criteria:** 0 errors (✗), token expires time advances every ~20 minutes (auto-refresh working), all heartbeats logged

#### Execution Phase 2: Cerberus ACL Tests (10 minutes)
```bash
# Wait for Phase 1 completion before starting Phase 2
# This prevents test interference

npx playwright test tests/phase3/cerberus-acl.spec.ts \
  --project=firefox \
  --reporter=html \
  --output-folder="test-results/phase3-acl" \
  2>&1 | tee logs/phase3-acl-execution.log

# Expected: Access control enforcement verification
# Completion time: [Record]
```

**Monitoring During Execution:**
```bash
# Monitor ACL log
docker exec charon-e2e tail -f /var/log/caddy/access.log | \
  grep -E "(403|DENY)" | \
  tee logs/phase3-acl-blocks.log
```

#### Execution Phase 3: Coraza WAF Tests (10 minutes)
```bash
# After Phase 2 completion

npx playwright test tests/phase3/coraza-waf.spec.ts \
  --project=firefox \
  --reporter=html \
  --output-folder="test-results/phase3-waf" \
  2>&1 | tee logs/phase3-waf-execution.log

# Expected: Malicious request blocking verification
# Completion time: [Record]
```

**Monitoring During Execution:**
```bash
# Monitor WAF blocks
docker exec charon-e2e tail -f /var/log/caddy/access.log | \
  grep -E "(403|WAF|injection|xss)" | \
  tee logs/phase3-waf-blocks.log
```

#### Execution Phase 4: Rate Limiting Tests (10 minutes)
```bash
# After Phase 3 completion
# CRITICAL: Run with --workers=1 (serial execution)

npx playwright test tests/phase3/rate-limiting.spec.ts \
  --project=firefox \
  --reporter=html \
  --output-folder="test-results/phase3-ratelimit" \
  --workers=1 \
  2>&1 | tee logs/phase3-ratelimit-execution.log

# Expected: Rate limit enforcement verification
# Completion time: [Record]
```

**Monitoring During Execution:**
```bash
# Monitor rate limit headers
docker exec charon-e2e tail -f /var/log/caddy/access.log | \
  grep -E "(429|X-RateLimit)" | \
  tee logs/phase3-ratelimit-events.log
```

#### Execution Phase 5: CrowdSec Tests (10 minutes)
```bash
# After Phase 4 completion

npx playwright test tests/phase3/crowdsec-integration.spec.ts \
  --project=firefox \
  --reporter=html \
  --output-folder="test-results/phase3-crowdsec" \
  2>&1 | tee logs/phase3-crowdsec-execution.log

# Expected: DDoS/bot mitigation verification
# Completion time: [Record]
```

**Monitoring During Execution:**
```bash
# Monitor CrowdSec decisions
docker exec charon-e2e tail -f /var/log/caddy/access.log | \
  grep -E "(403|blocked|crowdsec)" | \
  tee logs/phase3-crowdsec-blocks.log
```

### Retry Strategy

#### If Individual Test Fails:

1. **Capture Failure Details:**
   ```bash
   # Check HTML report
   npx playwright show-report test-results/phase3-*/

   # Review error logs
   tail -100 logs/phase3-*-execution.log

   # Review security logs
   docker logs charon-e2e | tail -200
   ```

2. **Determine Root Cause:**
   - Is it a test logic error? (Rewrite test)
   - Is it a security configuration issue? (Fix config)
   - Is it a flaky test dependent on timing? (Add waits/retries)
   - Is it an application bug? (Escalate to dev team)

3. **Re-Run Only Failed Test:**
   ```bash
   # Re-run single failed test
   npx playwright test tests/phase3/cerberus-acl.spec.ts \
     -g "should receive 403 for admin endpoint" \
     --project=firefox
   ```

4. **No Automatic Retries:**
   - Phase 3 security tests run once only
   - If test fails, it indicates a real security issue
   - Retrying masks the problem
   - All failures require investigation before retry

#### If Entire Suite Fails:

1. **Check System State:**
   ```bash
   # Verify container still healthy
   docker exec charon-e2e curl http://localhost:8080/health

   # Verify database still accessible
   docker exec charon-e2e sqlite3 data/charon.db "SELECT 1;"

   # Check for rate limit escalation
   docker exec charon-e2e tail /var/log/caddy/access.log | \
     grep -c 429
   ```

2. **Reset Environment (if contaminated):**
   ```bash
   # Option 1: Restore database from backup
   docker exec charon-e2e cp data/charon.db.backup data/charon.db

   # Option 2: Rebuild entire environment (if needed)
   .github/skills/scripts/skill-runner.sh docker-rebuild-e2e

   # Re-verify pre-test checklist
   # Restart test suite
   ```

3. **Escalate if Unresolved:**
   - Document all logs and error messages
   - Create GitHub issue with "Phase 3 Test Failure" label
   - Notify security team + dev lead
   - Phase 3 cannot proceed until resolved

### Test Report Generation

```bash
# After all suites complete, generate combined report

# 1. Copy all test results to reports directory
mkdir -p docs/reports/phase3
cp -r test-results/phase3-* docs/reports/phase3/

# 2. Collect all logs
mkdir -p docs/reports/phase3/logs
cp logs/phase3-* docs/reports/phase3/logs/

# 3. Copy HTML reports
for suite in core acl waf ratelimit crowdsec; do
  cp test-results/phase3-${suite}/index.html \
    docs/reports/phase3/report-${suite}.html
done

# 4. Generate Markdown summary (see section below)
```

---

## 11. Expected Challenges & Mitigations

### Challenge: Rate Limiting Blocks Rapid Test Execution

**Impact:** Tests timeout, false failures
**Scenario:** WAF + Rate Limiting tests run in parallel, both generate rapid requests

**Mitigation Strategies:**
1. **Run tests serially** (recommended)
   - Use `--workers=1` for rate limiting suite
   - Sequential execution avoids cross-test interference
2. **Whitelist test traffic**
   - Add test container IP to rate limit whitelist
   - Configure WAF to exempt test endpoints
3. **Space out requests**
   - Add delays between rapid test steps
   - Distribute requests across multiple test users

**Implementation:**
```bash
# Recommended approach
npx playwright test tests/phase3/rate-limiting.spec.ts \
  --workers=1  # Serial execution

# Alternative: Whitelist test traffic
# In Caddy config: rate_limiter { skip 127.0.0.1 }
```

### Challenge: WAF Blocks Legitimate Test Payloads

**Impact:** False positives, test failures
**Scenario:** Test submits valid-but-suspicious data (JSON with quotes, etc.)

**Mitigation Strategies:**
1. **Tune WAF paranoia level**
   - Default: Level 2 (balanced)
   - For testing: Level 1 (less strict) in test environment
   - Production: Level 3-4 (strict)

2. **Configure WAF exceptions**
   ```
   # Caddy coraza config
   exclude_rules:
     - 942200  # Exclude overly-broad SQL detection
     - 941320  # Exclude strict XSS detection
   ```

3. **Use crafted test payloads**
   - Test with payloads that clearly violate rules
   - Avoid ambiguous data that might be legitimate

**Implementation:**
```go
// Test with clear attack pattern
const sqlInjection = "1' OR '1'='1"  // Unambiguous
// Not: "admin" which might legitimately contain quote char
```

### Challenge: CrowdSec Blocks Test IP for Duration

**Impact:** All tests fail for the session, cannot recover
**Scenario:** Rate limiting or WAF test triggers CrowdSec detection, test IP is blocked

**Mitigation Strategies:**
1. **Whitelist test IP/container**
   ```
   # In CrowdSec bouncer config
   whitelist:
     enabled: true
     ips:
       - 127.0.0.1    # localhost
       - 172.17.0.0/16  # Docker internal network
   ```

2. **Use separate container IP per suite**
   - Run each test suite in different container
   - Avoids cross-test IP contamination

3. **Monitor CrowdSec decisions**
   ```bash
   # Check if test IP is blocked
   docker exec charon-e2e cscli decisions list

   # Remove test IP if needed
   docker exec charon-e2e cscli decisions delete \
     --ip 127.0.0.1
   ```

**Implementation:**
```bash
# Pre-test verification
docker exec charon-e2e cscli decisions list | grep BLOCKED
# If test IP appears, whitelist it before proceeding
```

### Challenge: Token Expires During 60-Minute Session

**Impact:** 401 errors, session test fails
**Scenario:** Token refresh fails or becomes stale

**Mitigation Strategies:**
1. **Verify refresh implementation (Phase 2.3c)**
   - Confirm auto-refresh active and working
   - Check refresh token storage and transmission
   - Validate token expiration claims

2. **Monitor token lifecycle**
   ```bash
   # Log token refresh events
   docker exec charon-e2e tail -f /var/log/charon/security.log | \
     grep -i "token\|refresh"
   ```

3. **Add explicit refresh checks**
   ```typescript
   // In test: verify token refreshed at expected times
   test('60-minute session with token refresh', { timeout: '70m' }, async ({ page }) => {
     const tokenBefore = getStoredToken()

     // Wait 18 minutes
     await page.waitForTimeout(18 * 60 * 1000)

     // Force API call to trigger refresh
     const response = await makeAPICall()
     expect(response.status).toBe(200)  // Not 401

     const tokenAfter = getStoredToken()
     expect(tokenAfter).not.toEqual(tokenBefore)  // Token refreshed
   })
   ```

**Implementation:**
```typescript
// Utility function to verify token freshness
async function verifyTokenNotExpired(page: Page): Promise<boolean> {
  const token = await page.evaluate(() =>
    localStorage.getItem('access_token')
  )
  const { exp } = parseJWT(token)
  return exp > Date.now() / 1000
}
```

### Challenge: Security Logs Flood Output, Hard to Debug

**Impact:** Lost in noise, cannot identify root causes
**Scenario:** 60-minute test generates thousands of log entries

**Mitigation Strategies:**
1. **Redirect logs to file**
   ```bash
   docker logs charon-e2e 2>&1 | tee logs/phase3-full.log
   # Logs saved for offline analysis
   ```

2. **Filter logs during execution**
   ```bash
   # Only show errors and security events
   tail -f /var/log/caddy/access.log | \
     grep -E "(401|403|429|500|deny|block|xss|sql)"
   ```

3. **Post-processing analysis**
   ```bash
   # After test completion, analyze logs
   grep -c "401" logs/phase3-full.log  # Count auth errors
   grep -c "403" logs/phase3-full.log  # Count access denied
   grep -c "429" logs/phase3-full.log  # Count rate limits

   # Extract unique errors
   grep "error\|failed" logs/phase3-full.log | sort | uniq -c
   ```

**Implementation:**
```bash
# Comprehensive logging setup
PHASE3_LOGS="logs/phase3-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$PHASE3_LOGS"

# 1. Run tests with output redirect
npx playwright test tests/phase3/ ... 2>&1 | \
  tee "$PHASE3_LOGS/test-execution.log"

# 2. Capture container logs
docker logs charon-e2e > "$PHASE3_LOGS/container.log" 2>&1

# 3. Export security logs
docker exec charon-e2e cp /var/log/caddy/access.log \
  "$PHASE3_LOGS/caddy-access.log"
docker exec charon-e2e cp /var/log/charon/security.log \
  "$PHASE3_LOGS/charon-security.log"

echo "All logs exported to: $PHASE3_LOGS"
```

### Challenge: Multiple Roles Interfere with Each Other

**Impact:** Data leakage, permission bypasses
**Scenario:** Admin test user accidentally accesses user test user's data

**Mitigation Strategies:**
1. **Separate test user accounts**
   - Create dedicated test users per role
   - Never reuse test accounts across different role tests
   - Clear session between role transitions

2. **Fresh login for each role test**
   ```typescript
   // Bad: Reusing same browser context
   const adminToken = await login('admin@test.local')
   // ... admin tests ...
   const userToken = await login('user@test.local')  // Risk! Admin context still active

   // Good: Separate browser contexts
   const adminContext = await browser.newContext()
   const adminToken = await login('admin@test.local', adminContext)
   // ... admin tests ...
   await adminContext.close()

   const userContext = await browser.newContext()
   const userToken = await login('user@test.local', userContext)
   // ... user tests ...
   await userContext.close()
   ```

3. **Data isolation verification**
   ```typescript
   test('User cannot see other user data', async ({ page }) => {
     // Login as User A
     const dataA = await fetchUserData('user-a@test.local')

     // Logout and login as User B
     await logout()
     const dataB = await fetchUserData('user-b@test.local')

     // Verify cross-user data not visible
     expect(dataA.email).toBe('user-a@test.local')
     expect(dataB.email).toBe('user-b@test.local')
     expect(dataA.proxies).not.toContain(dataB.proxies)
   })
   ```

**Implementation:**
```typescript
// Test fixture for role isolation
export const createRoleTestContext = async (role: 'admin' | 'user' | 'guest') => {
  const browser = await chromium.launch()
  const context = await browser.newContext()

  // Fresh login for this role
  const tokens = await authenticateAs(role, context)

  return {
    context,
    tokens,
    cleanup: async () => {
      await context.close()
      await browser.close()
    }
  }
}

// Usage in tests
test('Admin can access all endpoints', async () => {
  const { context, tokens, cleanup } = await createRoleTestContext('admin')
  // ... admin-specific tests ...
  await cleanup()
})
```

### Challenge Summary Table

| Challenge | Impact | Likelihood | Mitigation |
|-----------|--------|------------|-----------|
| Rate limit blocks tests | Tests timeout | HIGH | Run serially, whitelist |
| WAF blocks valid data | False positives | MEDIUM | Tune rules, use clear payloads |
| CrowdSec blocks IP | All tests fail | MEDIUM | Whitelist test IP, monitor decisions |
| Token expires mid-session | 401 errors | LOW | Verify Phase 2.3c, monitor refresh |
| Security logs flood output | Debug difficult | MEDIUM | Redirect to files, filter, analyze |
| Multi-role interference | Data leakage | MEDIUM | Separate contexts, fresh logins |

---

## 12. Success Criteria & Go/No-Go Gate

### Phase 3 Pass Criteria (ALL Required)

#### ✅ Test Execution Success
- [x] Core Security tests: **100% pass rate** (11/11 tests pass)
  - Login & token generation working
  - Token refresh operating correctly
  - 60-minute session completes without 401 errors
  - Logout properly clears session
- [x] Cerberus ACL tests: **100% pass rate** (25/25 tests pass)
  - Admin access to protected endpoints verified
  - User restrictions enforced
  - Guest read-only capabilities confirmed
  - Role-based access properly enforced
- [x] Coraza WAF tests: **100% pass rate** (21/21 tests pass)
  - SQL injection attempts blocked
  - XSS payloads rejected
  - CSRF validation enforced
  - Malformed requests handled safely
- [x] Rate Limiting tests: **100% pass rate** (12/12 tests pass)
  - Login throttled after threshold
  - API endpoints rate limited appropriately
  - Rate limit headers present
  - Limits reset after time window
- [x] CrowdSec tests: **100% pass rate** (10/10 tests pass)
  - Blacklisted IPs blocked
  - Bot behavior detected
  - Decisions cached and updated
  - Whitelist bypass working

#### ✅ Session Stability
- [x] **60-minute session test** completes successfully
  - Zero 401 errors during entire test
  - Zero 403 errors (permissions maintained)
  - Token refresh occurs transparently (3+ times)
  - All API calls succeed (200/201/204 responses)
  - UI remains responsive throughout
  - Logout properly clears session

#### ✅ Middleware Enforcement
- [x] **Cerberus ACL** properly enforcing roles
  - Admin endpoints return 200 for admin, 403 for others
  - User endpoints filtered by ownership
  - Guest endpoints read-only
  - Permission bypass attempts rejected
- [x] **Coraza WAF** blocking attacks
  - 0 SQL injection bypasses
  - 0 XSS bypasses
  - CSRF token validation active
  - All blocked requests logged
- [x] **Rate Limiting** enforced consistently
  - Login rate limited after threshold
  - API endpoints limited appropriately
  - Headers indicate limit status
  - Counters reset correctly
- [x] **CrowdSec** mitigating threats
  - Blacklisted IPs blocked
  - Bot patterns detected
  - Decisions properly cached
  - Legitimate traffic allowed

#### ✅ Logging & Monitoring
- [x] All security events logged
  - Failed auth attempts logged
  - Access denied events recorded
  - Attack attempts logged
  - Rate limit violations recorded
- [x] Logs accessible and parseable
  - Caddy logs complete
  - Application logs complete
  - Security logs separated
  - All logs timestamp accurate

#### ✅ Configuration Verified
- [x] All security modules enabled and active
  - Cerberus ACL policies loaded
  - Coraza WAF rules active
  - Rate limiting configured
  - CrowdSec decisions synced
- [x] Test environment matches production
  - Same security module versions
  - Same middleware configuration
  - Same rate limiting rules
  - Same data access patterns

### Phase 3 Fail Criteria (ANY = FAIL)

#### ❌ Test Failures
- Any security test fails (indicates bypass or misconfiguration)
  - Core Security test fails → Session instability, token issue
  - Cerberus test fails → ACL bypass, permission leakage
  - WAF test fails → Attack not blocked (CRITICAL)
  - Rate Limit test fails → Abuse vulnerability
  - CrowdSec test fails → DDoS vulnerability
- Pass rate < 100% for any suite (0 tolerance for security tests)

#### ❌ Session Instability
- 401 errors during 60-minute session
  - Indicates token refresh failure or validation issue
- 403 errors indicating role change (unexpected)
  - Indicates permission revocation during session
- Session timeout < 60 minutes
  - Indicates configuration error

#### ❌ Middleware Bypass
- Unauthorized access allowed (ACL bypass)
  - User accessing admin endpoint
  - Guest modifying data
  - Permission escalation successful
- Malicious request not blocked (WAF bypass)
  - SQL injection executes
  - XSS script runs
  - CSRF validates without token
- Rate limit not enforced (abuse vulnerability)
  - Unlimited requests after threshold
  - Rate limit headers missing/incorrect
- Blacklisted IP not blocked (CrowdSec bypass)
  - Blacklisted IP accesses API
  - Bot pattern not detected

#### ❌ Data Isolation Failure
- Data leakage between user roles
  - User sees other user's data
  - Guest sees admin-only metrics
  - Cross-role data visible in API response
- Permission inheritance broken
  - Parent resource permission not enforcing children
  - Cascading permissions not applied

#### ❌ Security Configuration Issues
- Security modules not enabled
  - Cerberus ACL not active
  - Coraza WAF not active
  - Rate Limiting not active
  - CrowdSec not synced
- Logs not being captured
  - Security events not logged
  - Attack attempts not recorded
  - Cannot trace security decisions
- Environment not ready
  - Test users not properly seeded
  - Credentials not configured
  - Emergency token not validated

### Phase 3 Go/No-Go Decision Logic

```
Total Tests: 79
Passing Tests: ?
Test Pass Rate: ?/79 = ?%

IF (Pass Rate == 100%) AND (60-min session succeeds) THEN
  RESULT: ✅ GO → Proceed to Phase 4

  ACTIONS:
  1. Document all test results
  2. Archive all logs and reports
  3. Create Phase 3 Validation Report (see Section 14)
  4. Notify Security Team + Stakeholders
  5. Schedule Phase 4 (UAT/Integration)
  6. Prepare for production release

ELSE IF (Pass Rate >= 95%) AND (critical tests pass) THEN
  RESULT: ⚠️ CONDITIONAL PASS → Phase 3 with caveats

  CAVEATS:
  1. Non-critical test failures documented
  2. Risk assessment completed
  3. Security Team approval required
  4. May proceed to Phase 4 with known issues
  5. Remediation planned for post-release

  REQUIRED FOR CONDITIONAL PASS:
  - 0 x critical security test failures
  - 60-minute session test passes
  - All ACL/WAF/CrowdSec tests pass
  - Only non-critical rate limiting or logging issues

ELSE (Pass Rate < 95%)
  RESULT: ❌ FAIL → Stop and remediate

  ACTIONS:
  1. Document all failures with details
  2. Create GitHub issues for each failure
  3. Notify Security Team (critical issues)
  4. Debug and identify root causes
  5. Fix issues or update tests
  6. Rerun failed suites
  7. Do not proceed to Phase 4 until PASS achieved

  ESCALATION (if unresolved):
  - Contact Security Lead
  - Contact Engineering Lead
  - Schedule design review if architecture issue
  - Consider alternative implementation
```

### Decision Matrix

| Scenario | Pass Rate | Session OK? | Critical OK? | Decision | Action |
|----------|-----------|------------|------------|----------|--------|
| Ideal | 100% | ✅ | ✅ | ✅ GO | Phase 4 ready |
| Good | 100% | ✅ | ✅ | ✅ GO | Phase 4 ready |
| Acceptable | 98-99% | ✅ | ✅ | ⚠️ CONDITIONAL | Document caveats |
| At-Risk | 95-97% | ✅ | ⚠️ | ❌ FAIL | Remediate warnings |
| Blocked | <95% | ❌ | ❌ | ❌ FAIL | Escalate + remediate |

---

## 13. Detailed Test Specifications

### Test Spec Template

Each test includes detailed step-by-step execution with expected outcomes.

#### **Test: 60-Minute Long-Running Session with Token Refresh**

**File:** `tests/phase3/security-enforcement.spec.ts`
**Priority:** P0 - CRITICAL
**Risk:** HIGH (blocks Phase 4)
**Duration:** 60+ minutes
**Slow Test Marker:** Yes

```typescript
test.describe('Phase 3: Security Enforcement', () => {
  test('Long-running E2E session (60+ min) with auto token refresh', {
    slow: true,
    timeout: '70m'  // Allow 70 minutes with 10-min buffer
  }, async ({ page }) => {

    /**
     * TEST OBJECTIVE:
     * Verify user session remains active for 60+ minutes with automatic
     * token refresh without generating 401 Unauthorized errors.
     *
     * ENTRY CRITERIA:
     * - Admin user account exists and is active
     * - Token refresh endpoint operational
     * - Application logging configured
     *
     * SUCCESS CRITERIA:
     * - 0 x 401 errors during 60-minute session
     * - Token refresh occurs >= 3 times (every ~18 min)
     * - All API calls return 200/201/204 (success)
     * - Session survives page reloads and navigation
     */

    const startTime = Date.now()
    const SESSION_DURATION = 60 * 60 * 1000  // 60 minutes
    const SESSION_WARNING_TIME = 65 * 60 * 1000  // 65 minutes (abort if hit)
    let tokenRefreshCount = 0
    let apiCallCount = 0
    let errorCount = 0

    // [STEP 1: LOGIN]
    console.log('[00:00] STEP 1: Login as admin user')
    await page.goto('/login')
    await page.fill('input[name="email"]', 'admin@test.local')
    await page.fill('input[name="password"]', 'AdminPass123!')
    await page.click('button[type="submit"]')

    // Wait for dashboard to load (indicates successful auth)
    await page.waitForSelector('[data-testid="dashboard"]', { timeout: 30000 })
    await expect(page).toHaveURL(/\/dashboard/)

    // Extract initial token
    const initialToken = await page.evaluate(() =>
      localStorage.getItem('access_token')
    )
    expect(initialToken).toBeTruthy()
    console.log('[00:05] ✓ Login successful, token acquired')

    // [STEP 2-6: 60-MINUTE SESSION LOOP]
    let lastRefreshTime = Date.now()

    while (Date.now() - startTime < SESSION_DURATION) {
      const elapsedMinutes = Math.round((Date.now() - startTime) / 60000)

      // [Every 10 minutes: API call with auth check]
      if ((Date.now() - startTime) % (10 * 60 * 1000) < 1000) {
        console.log(`[${elapsedMinutes}:00] Making API call: GET /api/v1/users`)

        const response = await page.request.get('/api/v1/users', {
          headers: {
            'Authorization': `Bearer ${
              // Get latest token from localStorage
              await page.evaluate(() => localStorage.getItem('access_token'))
            }`
          }
        })

        apiCallCount++

        if (response.status() === 200) {
          console.log(`[${elapsedMinutes}:00] ✓ API call successful (200)`)
        } else if (response.status() === 401) {
          errorCount++
          console.error(`[${elapsedMinutes}:00] ❌ CRITICAL: Received 401 Unauthorized`)
          // Continue to verify if auto-refresh happens
        } else {
          console.warn(`[${elapsedMinutes}:00] Unexpected status: ${response.status()}`)
        }
      }

      // [Every 15 minutes: UI navigation]
      if ((Date.now() - startTime) % (15 * 60 * 1000) < 1000 && elapsedMinutes > 0) {
        console.log(`[${elapsedMinutes}:00] Navigating UI...`)
        const pagesToVisit = ['/dashboard', '/settings', '/proxy-hosts']
        for (const urlPath of pagesToVisit) {
          await page.goto(urlPath)
          await page.waitForLoadState('networkidle')
          console.log(`[${elapsedMinutes}:00] ✓ ${urlPath} loaded`)
        }
      }

      // [Check token refresh (every 18+ minutes)]
      const currentToken = await page.evaluate(() =>
        localStorage.getItem('access_token')
      )
      if (currentToken !== initialToken &&
          Date.now() - lastRefreshTime > 18 * 60 * 1000) {
        tokenRefreshCount++
        lastRefreshTime = Date.now()
        console.log(`[${elapsedMinutes}:00] ✓ Token refresh detected (refresh #${tokenRefreshCount})`)
      }

      // [Safety: abort if session time exceeded]
      if (Date.now() - startTime > SESSION_WARNING_TIME) {
        console.warn('[70:00] WARNING: Session time exceeded 65 minutes, aborting')
        break
      }

      // Yield to browser (prevent test hang)
      await page.waitForTimeout(1000)
    }

    // [STEP 7: LOGOUT]
    console.log(`[${Math.round((Date.now() - startTime) / 60000)}:00] STEP 7: Logout`)
    await page.goto('/logout')
    await page.waitForURL(/\/login/)
    console.log('✓ Logged out successfully')

    // [STEP 8: VERIFY TOKEN INVALIDATED]
    console.log('STEP 8: Verify token invalidated')
    const logoutToken = await page.evaluate(() =>
      localStorage.getItem('access_token')
    )
    expect(logoutToken).toBeNull()  // Should be cleared
    console.log('✓ Token cleared after logout')

    // [STEP 9: VERIFY CANNOT REUSE OLD TOKEN]
    console.log('STEP 9: Verify cannot reuse old token')
    const reuseResponse = await page.request.get('/api/v1/users', {
      headers: {
        'Authorization': `Bearer ${initialToken}`
      }
    })
    expect(reuseResponse.status()).toBe(401)  // Old token should fail
    console.log('✓ Old token properly invalidated')

    // [RESULTS & ASSERTIONS]
    console.log('\n=== SESSION TEST RESULTS ===')
    console.log(`Duration: ${Math.round((Date.now() - startTime) / 60000)} minutes`)
    console.log(`API Calls: ${apiCallCount}`)
    console.log(`Errors (401): ${errorCount}`)
    console.log(`Token Refreshes: ${tokenRefreshCount}`)
    console.log('========================\n')

    // Assertions (would fail test if not met)
    expect(errorCount).toBe(0)  // CRITICAL: No 401 errors
    expect(tokenRefreshCount).toBeGreaterThanOrEqual(3)  // At least 3 refreshes in 60 min
    expect(apiCallCount).toBeGreaterThan(0)  // At least some API calls made
  })
})
```

### Template for Other Tests

Each test follows this structure:

```typescript
test('descriptive test name', {
  slow: true,  // Add if test is inherently slow
  timeout: 'XXs'  // Specify timeout if not standard
}, async ({ page }) => {
  // [STEP 1: Setup]
  // [STEP 2: Execute]
  // [STEP 3: Verify]
  // [ASSERTIONS: expect() calls]
})
```

*See previous sections for detailed test implementations across all 5 test suites.*

---

## 14. Phase 3 Validation Report Template

### Report Location: `/projects/Charon/docs/reports/PHASE_3_SECURITY_VALIDATION.md`

```markdown
# Phase 3: E2E Security Testing - Validation Report

**Report Date:** [Date filled by QA]
**Execution Duration:** [Test execution time]
**Total Tests Executed:** [Count]
**Overall Pass Rate:** [X/Y = Z%]

---

## Executive Summary

[Summary of all middleware validations and overall findings]

This report documents the completion of Phase 3: E2E Security Testing, verifying that all security middleware components (Cerberus ACL, Coraza WAF, Rate Limiting, CrowdSec) properly enforce their intended security policies during extended test sessions.

### Key Findings:
- **Cerberus ACL:** [Enforcing / Has Issues / Failed]
- **Coraza WAF:** [Blocking / Has Bypasses / Failed]
- **Rate Limiting:** [Enforcing / Has Gaps / Failed]
- **CrowdSec:** [Effective / Has Blindspots / Failed]
- **Session Stability:** [60+ min successful / Issues found]

---

## Test Execution Summary

### Test Suite Results

| Module | File | Tests | Passed | Failed | Pass Rate | Status |
|--------|------|-------|--------|--------|-----------|--------|
| Core Security | security-enforcement.spec.ts | 11 | ? | ? | ?% | ⏳ |
| Cerberus ACL | cerberus-acl.spec.ts | 25 | ? | ? | ?% | ⏳ |
| Coraza WAF | coraza-waf.spec.ts | 21 | ? | ? | ?% | ⏳ |
| Rate Limiting | rate-limiting.spec.ts | 12 | ? | ? | ?% | ⏳ |
| CrowdSec | crowdsec-integration.spec.ts | 10 | ? | ? | ?% | ⏳ |
| **TOTAL** | | **79** | **?** | **?** | **?%** | **?** |

### Execution Timeline
- Start Time: [Time]
- End Time: [Time]
- Total Duration: [Hours:Minutes]
- Environment: Docker container (charon-e2e)

---

## Detailed Test Results

### [Test Suite Name]: [Pass/Fail]

#### Test Results Detail
```
[Detailed results from HTML report]
```

#### Failed Tests (if any)
```
Test Name: [Test]
Error: [Error message]
Root Cause: [Identified cause]
Remediation: [Action taken]
Re-run Result: [Pass/Fail]
```

#### Key Observations
- [Observation 1]
- [Observation 2]
- [Observation 3]

---

## Security Audit Results

### Middleware Enforcement Verification

#### Cerberus ACL Module
- **Status:** ✅ Enforcing / ⚠️ Partially Enforcing / ❌ Not Enforcing
- **Findings:**
  - Admin access properly controlled
  - User restrictions enforced
  - Guest permissions respected
  - Cross-user data isolation verified
- **Issues Found:** [List any issues or "None"]
- **Recommendation:** [Proceed / Remediate before proceeding]

#### Coraza WAF Module
- **Status:** ✅ Blocking / ⚠️ Partial Blocking / ❌ Bypass Detected
- **Findings:**
  - SQL injection attempts blocked
  - XSS payloads rejected
  - CSRF validation active
  - Malformed requests handled
- **Issues Found:** [List any bypasses or "None detected"]
- **Recommendation:** [Proceed / Update rules]

#### Rate Limiting Module
- **Status:** ✅ Enforcing / ⚠️ Inconsistent / ❌ Not Enforcing
- **Findings:**
  - Login throttling: [Threshold verified]
  - API rate limits: [Applied correctly]
  - Headers: [Present/Missing/Incorrect]
  - Reset behavior: [Verified/Issue found]
- **Issues Found:** [Gaps or problems]
- **Recommendation:** [Proceed / Tune thresholds]

#### CrowdSec Integration
- **Status:** ✅ Effective / ⚠️ Limited / ❌ Ineffective
- **Findings:**
  - Blacklist enforcement: [Working]
  - Bot detection: [Active/Inactive]
  - Decision caching: [Functional/Issue]
  - Whitelist bypass: [Working/Issue]
- **Issues Found:** [Any issues or "None"]
- **Recommendation:** [Proceed / Review config]

### Security Event Logging
- **Events Logged:** [Count]
- **Log Coverage:** [Percentage of events captured]
- **Issues:** [Any missing logs or "All captured"]

---

## Performance Metrics

### Session Stability
- **60-Minute Test Duration:** [Passed/Failed]
- **401 Errors:** [Count - should be 0]
- **403 Errors:** [Count - should be 0]
- **Token Refreshes:** [Count - should be 3+]
- **API Call Success Rate:** [X/Y = Z%]
- **UI Responsiveness:** [Good/Degraded/Poor]

### Test Execution Performance
- **Average Test Duration:** [Seconds]
- **Slowest Test:** [Name] ([Seconds])
- **Resource Usage:** CPU [%], Memory [MB]

---

## Issues & Resolutions

### Critical Issues (Block Phase 4)
| Issue | Severity | Status | Resolution |
|-------|----------|--------|-----------|
| [Issue 1] | CRITICAL | [Fixed/Pending] | [Resolution] |

### High Priority Issues (Address in Phase 4)
| Issue | Severity | Status | Resolution |
|-------|----------|--------|-----------|
| [Issue 1] | HIGH | [Fixed/Pending] | [Resolution] |

### Low Priority Issues (Backlog)
| Issue | Severity | Status | Resolution |
|-------|----------|--------|-----------|
| [Issue 1] | LOW | [Fixed/Pending] | [Resolution] |

### Issue Resolution Timeline
- [Date]: Issue X identified
- [Date]: Root cause analysis completed
- [Date]: Fix implemented
- [Date]: Re-test passed

---

## Go/No-Go Assessment

### Phase 3 Success Criteria Checklist

- [ ] Core Security tests: 100% pass rate (11/11)
- [ ] Cerberus ACL tests: 100% pass rate (25/25)
- [ ] Coraza WAF tests: 100% pass rate (21/21)
- [ ] Rate Limiting tests: 100% pass rate (12/12)
- [ ] CrowdSec tests: 100% pass rate (10/10)
- [ ] 60-minute session: Completed successfully
- [ ] Zero 401 errors during session
- [ ] Zero critical security bypasses
- [ ] All middleware properly logging
- [ ] Environment ready for Phase 4

### Final Verdict

**Phase 3 Result:** ✅ PASS / ⚠️ CONDITIONAL PASS / ❌ FAIL

**Rationale:**
[Explanation of why test passed/failed and any caveats]

### Recommendation for Phase 4

**Status:** ✅ Ready for Phase 4 / ⚠️ Proceed with Caveats / ❌ Do Not Proceed

**Next Steps:**
1. [Action 1 - Schedule Phase 4]
2. [Action 2 - notify stakeholders]
3. [Action 3 - prepare for UAT]

---

## Appendices

### A. Detailed Test Logs
- See: `logs/phase3-core-execution.log`
- See: `logs/phase3-acl-execution.log`
- See: `logs/phase3-waf-execution.log`
- See: `logs/phase3-ratelimit-execution.log`
- See: `logs/phase3-crowdsec-execution.log`

### B. Security Event Audit
- Captured from: `/var/log/caddy/access.log`
- Captured from: `/var/log/charon/security.log`
- Events analyzed: [Count]

### C. HTML Test Reports
- `docs/reports/phase3/report-core.html`
- `docs/reports/phase3/report-acl.html`
- `docs/reports/phase3/report-waf.html`
- `docs/reports/phase3/report-ratelimit.html`
- `docs/reports/phase3/report-crowdsec.html`

### D. Environment Configuration
```
Docker Image: [SHA]
Charon Version: [Version]
Go Version: [Version]
Caddy Version: [Version]
Cerberus Version: [Version]
Coraza Version: [Version]
CrowdSec Version: [Version]
Test Database: [Location]
Backup Location: [Location]
```

### E. Known Issues & Future Work
- [Issue 1] - Status: [Backlog/In Progress/Resolved]
- [Issue 2] - Status: [Backlog/In Progress/Resolved]

---

## Sign-Off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| QA Lead | [Name] | [Date] | [Signature] |
| Security Lead | [Name] | [Date] | [Signature] |
| Engineering Lead | [Name] | [Date] | [Signature] |

---

*Report Generated: [Date] at [Time] UTC*
*Phase 3 Execution Duration: [Days/Hours]*
```

---

## 15. Phase 3 Execution Timeline

| Phase | Task | Duration | Owner | Dependencies | Status |
|-------|------|----------|-------|--------------|--------|
| Pre-Test | Environment setup & verification | 10 min | QA | Docker ready | ⏳ |
| | Pre-execution checklist | 5 min | QA | All infra ready | ⏳ |
| Phase 1 | Core Security tests execution | 60+ min | Playwright Dev | Environment ready | ⏳ |
| | Monitor token refresh & session | 5 min | QA | Tests running | ⏳ |
| Phase 2 | Cerberus ACL tests execution | 10 min | Playwright Dev | Phase 1 complete | ⏳ |
| | Analyze ACL logs & results | 5 min | QA | Tests complete | ⏳ |
| Phase 3 | Coraza WAF tests execution | 10 min | Playwright Dev | Phase 2 complete | ⏳ |
| | Review attack blocking logs | 5 min | QA | Tests complete | ⏳ |
| Phase 4 | Rate Limiting tests execution | 10 min | Playwright Dev | Phase 3 complete | ⏳ |
| | (Serial: --workers=1) | | | | ⏳ |
| | Analyze rate limit enforcement | 5 min | QA | Tests complete | ⏳ |
| Phase 5 | CrowdSec tests execution | 10 min | Playwright Dev | Phase 4 complete | ⏳ |
| | Review CrowdSec decisions | 5 min | QA | Tests complete | ⏳ |
| Post-Test | Log collection & analysis | 20 min | QA | All tests done | ⏳ |
| | Generate validation report | 15 min | QA | All logs collected | ⏳ |
| | Supervisor review | 30 min | Supervisor | Report ready | ⏳ |
| | Go/No-Go decision | 30 min | Leadership | Review complete | ⏳ |
| **TOTAL** | | **~180 min** | — | — | **⏳** |

**Total Estimated Time: 2-3 hours** (accounts for serialization requirements)

### Parallel Execution Opportunity (If Needed)

If Phase 3 timeline is compressed:
- Run Cerberus ACL (Phase 2) in parallel with Core Security (Phase 1) `--workers=4`
- Run Coraza WAF (Phase 3) after Phase 1 completes
- Run Rate Limiting (Phase 4) serially after Phase 3
- Run CrowdSec (Phase 5) in parallel with Rate Limiting `--workers=4`

**Fastest Sequential Path (66 min):**
1. Core Security: 60+ min
2. WAF + ACL in parallel: 10 min (share --workers=2)
3. Rate Limit: 10 min (serial)
4. CrowdSec: 10 min

**This reduces total to ~90 minutes** but increases complexity of troubleshooting.

---

## Known Constraints & Limitations

### Test Environment Constraints
- **Single Container Instance:** All tests run in one Docker container (not multi-instance)
  - Implication: CrowdSec decisions list is local, not distributed
  - Workaround: Tests assume single-instance deployment
- **SQLite Database:** Not optimized for concurrent connections
  - Implication: Heavy parallel test loads may lock database
  - Workaround: Run suites serially
- **Memory Limits:** Docker container has limited memory
  - Implication: Long-running tests (60-min) consume memory
  - Monitor: `docker stats charon-e2e` during execution

### WAF Configuration Limitations
- **Paranoia Level 2:** Balanced security vs. usability
  - False Negatives: Some sophisticated attacks may bypass
  - False Positives: Some legitimate requests may block
  - Recommendation: Tune for production use
- **CRS Version:** OWASP ModSecurity Core Rule Set version dependent
  - Recommendation: Verify CRS version matches deployment

### Rate Limiting Constraints
- **In-Memory Storage:** Rate limits reset on container restart
  - Implication: Tests assume persistent container
  - Workaround: Don't restart container between suites
- **Single Container:** Counts are not synchronized across instances
  - Recommendation: Verify distributed rate limiting before scaling

### CrowdSec Limitations
- **Community Decisions:** May have false positives from community
  - Solution: Custom rules or whitelist trusted IPs
- **Cache Update Delay:** Decisions cached for ~30 seconds
  - Implication: Decision updates not instant
  - Mitigation: Add waits for decision propagation

---

## Approval & Sign-Off

### Plan Development Approval

| Role | Approval | Date | Notes |
|------|----------|------|-------|
| Principal Architect | ⏳ Pending | — | Plan author |
| Security Lead | ⏳ Pending | — | Security validation |
| Engineering Lead | ⏳ Pending | — | Technical feasibility |

### Execution Authority

| Role | Authorization | Date | Signature |
|------|---------------|------|-----------|
| QA Manager | ⏳ Pending | — | Test execution approval |
| Product Manager | ⏳ Pending | — | Go/No-Go authority |

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | Feb 10, 2026 | Principal Architect | Initial comprehensive plan |
| — | — | — | — |

---

**END OF PHASE 3 SECURITY TESTING PLAN**

**Next Document:** `PHASE_3_SECURITY_VALIDATION.md` (created during/after execution)

---

## Quick Reference

### Pre-Test Execution Checklist
```bash
# Run this before starting Phase 3 tests
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
# Verify: Health check passes, all modules enabled
# Check: Test users exist, emergency token valid
# Confirm: All logs accessible
```

### Execute Phase 3 Tests
```bash
# Run all Phase 3 suites (with proper serialization)
npx playwright test tests/phase3/ \
  --project=firefox \
  --reporter=html \
  --grep="Phase3" \
  --output-folder="test-results/phase3"
```

### Analyze Results
```bash
# View HTML report
npx playwright show-report test-results/phase3/

# Check pass rate
grep -c "passed" test-results/phase3/index.html

# Extract any failures
grep "failed" test-results/phase3/index.html
```

### Generate Validation Report
```bash
# After all tests complete
cp -r test-results/phase3-* docs/reports/phase3/
# Manually fill: /projects/Charon/docs/reports/PHASE_3_SECURITY_VALIDATION.md
```

---

**Ready for Supervisor review. Plan location: `/projects/Charon/docs/plans/PHASE_3_SECURITY_TESTING_PLAN.md`**
