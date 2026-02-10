# Phase 4 Integration Test Suite

Integration testing for multi-component workflows and system interactions in Charon.

## Overview

**Test Count**: 40 tests across 7 test files
**Framework**: Playwright Test (Firefox)
**Base URL**: `http://127.0.0.1:8080` (Docker container)
**Focus**: Component interactions, middleware enforcement, data consistency, security layer validation

## Test Files

### 01-admin-user-e2e-workflow.spec.ts (7 tests)
- **Purpose**: Complete workflows from admin and user perspectives
- **Scenarios**:
  - User creation → role assignment → login → access resources
  - Admin role modification triggers immediate permission updates
  - Admin user deletion → deleted user login fails
  - Complete workflow appears in audit log (action, user, timestamp)
  - User self-promotion blocked (role escalation prevention)
  - Multi-user data isolation verified
  - User logout → different user login succeeds
- **Key Assertions**:
  - Visibility changes after role modification
  - 401 Unauthorized when accessing restricted resources
  - Audit trail entries created for each action
  - Data isolation between user accounts

### 02-waf-ratelimit-interaction.spec.ts (5 tests)
- **Purpose**: WAF and rate limiting working together and independently
- **Scenarios**:
  - Malicious payload blocked by WAF (403 Forbidden)
  - Excessive requests blocked by rate limiter (429 Too Many Requests)
  - Both enforced independently (WAF ≠ rate limit)
  - Malicious request within rate limit still blocked (WAF priority → 403)
  - Clean request exceeding limit blocked (rate limit → 429)
- **Payloads Tested**:
  - SQL Injection: `'; DROP TABLE users--`
  - Path Traversal: `../../../etc/passwd`
  - Command Injection: `` `cat /etc/passwd` ``
- **Assertions**:
  - response.status() === 403 for WAF blocks
  - response.status() === 429 for rate limit blocks
  - Headers present in denied responses

### 03-acl-waf-layering.spec.ts (4 tests)
- **Purpose**: Defense-in-depth validation (ACL and WAF as separate mandatory layers)
- **Scenarios**:
  - Non-admin insufficient to bypass WAF (403 even with permission)
  - WAF enforced regardless of user role (admin ≠ immune)
  - Admin user also subject to WAF enforcement (mandatory layer)
  - ACL provides additional filtering beyond WAF (two-layer security)
- **Key Tests**:
  - Create guest user, attempt malicious request, verify 403
  - Login as admin, send malicious request, verify 403
  - Verify ACL blocks before WAF (deny-by-default)
- **Assertions**:
  - 403 Forbidden regardless of user role
  - WAF module enforces on all requests
  - ACL filtering applied independently

### 04-auth-middleware-cascade.spec.ts (6 tests)
- **Purpose**: Authentication flows through middleware stack correctly
- **Middleware Chain**:
  1. Auth (token validation)
  2. ACL (role/permission check)
  3. WAF (malicious payload detection)
  4. Rate Limiting (request throttling)
- **Scenarios**:
  - Missing auth token → 401 Unauthorized (auth first)
  - Invalid/malformed token → 401 (auth first)
  - Valid token passes through ACL (200 OK)
  - Valid token passes through WAF (200 OK)
  - Valid token passes through rate limiting (200 OK)
  - Valid token flows through complete stack (end-to-end)
- **Assertions**:
  - 401 for missing/invalid tokens (rejected early)
  - 200 for valid token through each layer

### 05-data-consistency.spec.ts (8 tests)
- **Purpose**: UI operations sync correctly with API representation
- **Scenarios**:
  - Create entity via UI → API returns identical data
  - Modify entity via API → UI refreshes and shows updated data
  - Delete entity via UI → API no longer returns item
  - Concurrent modifications handled correctly (no data loss)
  - Transaction rollback prevents partial updates
  - Database constraints enforced (unique, foreign key)
  - UI form validation matches backend validation
  - Large dataset pagination consistent between loads
- **Test Entities**:
  - Users (email, role, permissions)
  - Proxy hosts (domain, target, SSL)
  - Domains (nameserver, DNS records)
- **Assertions**:
  - API response data matches UI displayed data
  - Constraint violations return expected errors
  - Pagination offsets consistent across requests

### 06-long-running-operations.spec.ts (5 tests)
- **Purpose**: Background tasks don't block system responsiveness
- **Scenarios**:
  - Backup creation doesn't block other operations
  - System responsive during backup (UI updates work)
  - Proxy creation succeeds while backup running
  - User login succeeds during long operation
  - Task completion verified after operation finishes
- **Long-Running Operations Tested**:
  - Database backup (2-5 second duration)
  - File encryption during backup
  - APK signature generation
- **Assertions**:
  - Proxy creation returns 200 during backup
  - Login token generated and valid during backup
  - Backup completion verified with file hash

### 07-multi-component-workflows.spec.ts (5 tests)
- **Purpose**: Complex real-world workflows involving multiple system features
- **Workflows**:
  1. Create proxy → Enable WAF → Send request → WAF enforces
  2. Create user → Assign role → User creates proxy → ACL enforces
  3. Create backup → Delete user → Restore → Data restored
  4. Enable security module → Create user → User subject to rate limits
  5. Admin: Create user → Enable security → User cannot bypass limits
- **Key Assertions**:
  - Malicious request blocked (403) from WAF assigned to proxy
  - New user inherits security configuration immediately
  - Deleted user data restored from backup
  - Rate limiting applied to new users after security enablement
  - User-created resources respect admin-level security settings

## Execution

### Run all integration tests:
```bash
cd /projects/Charon
npx playwright test tests/phase4-integration/ --project=firefox
```

### Run specific integration test:
```bash
npx playwright test tests/phase4-integration/04-auth-middleware-cascade.spec.ts --project=firefox
```

### Run with timing output:
```bash
npx playwright test tests/phase4-integration/ --project=firefox --reporter=line
```

## Prerequisites

1. **Docker environment running**:
   ```bash
   .github/skills/scripts/skill-runner.sh docker-rebuild-e2e
   ```

2. **Playwright installed**:
   ```bash
   npm install && npx playwright install firefox
   ```

3. **Admin user creation** (handled by global-setup.ts)

## Test Characteristics

### Performance Expectations
- Small operations (create/modify/delete): <5 seconds
- Long-running operations (backup): 2-5 seconds (non-blocking)
- Concurrent operations: All complete within 30 seconds total

### Error Handling
- Tests use try/catch for error scenarios
- Soft assertions for optional features (`.catch(() => false)`)
- Multiple selector strategies for component resilience

### Data Management
- `beforeEach`: Preps test data in known state
- `afterEach`: Cleans up test data via UI operations
- No cross-test data persistence

## Success Criteria

✅ All 7 files compile without syntax errors
✅ All 40 tests execute and pass
✅ WAF blocks malicious payloads (403)
✅ Rate limiting enforces thresholds (429)
✅ ACL prevents unauthorized access (401/403)
✅ UI↔API data consistency verified
✅ Concurrent operations complete successfully
✅ Long-running operations don't block system
✅ Multi-component workflows function end-to-end
✅ Middleware stack enforces in correct order

## Troubleshooting

### Test timeouts on operations
- Check if Docker container is healthy: `docker ps | grep charon-e2e`
- Long-running tests might need: `--timeout=120000`
- Verify network latency to container

### Rate limit tests failing
- Verify rate limiting enabled in admin UI
- Check configured threshold (default: 100 req/60s)
- Clear any previous rate limit state: `docker exec charon-e2e redis-cli FLUSHALL`

### WAF tests not blocking payloads
- Verify Coraza WAF enabled in admin UI
- Check WAF sensitivity level (default: medium)
- Verify malicious payload pattern matches WAF rules

### Data consistency issues
- Run in `--debug` mode to inspect API responses
- Check browser console for validation errors
- Verify test data cleanup (afterEach should delete created items)

## Integration with CI/CD

These tests run as Phase 4 validation after UAT passes:

```yaml
# .github/workflows/phase4-integration.yml
- runs: npx playwright test tests/phase4-integration/ --project=firefox
  timeout: 45 minutes
  screenshots: retain-on-failure
```

## Key Differences from UAT

| Aspect | UAT | Integration |
|--------|-----|-------------|
| **Scope** | Single component | Multiple components |
| **Focus** | Feature completeness | Interaction correctness |
| **Assertions** | Visual/UI state | API response + UI state |
| **Workflows** | Single operation | Multi-step processes |
| **Security** | Individual module | Middleware stack |

## Notes

- **Concurrency testing**: Tests verify true concurrent execution, not sequential
- **Soft assertions**: Optional features handled gracefully if missing
- **Error injection**: Some tests intentionally send invalid data
- **Performance baseline**: No hard limits, but used as regression detection
- **Isolation**: Each test creates and cleans up its own data

## Additional Resources

- [Playwright Documentation](https://playwright.dev/)
- [Charon Architecture](../../ARCHITECTURE.md)
- [Phase 4 Plan](../../design.md)
- [Integration Test Patterns](../phase4-uat/README.md)
