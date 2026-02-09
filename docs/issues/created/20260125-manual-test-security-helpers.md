# Manual Testing: Security Test Helpers

**Created**: June 2025
**Priority**: Medium
**Status**: Open

## Objective

Verify the security test helpers implementation prevents ACL deadlock during E2E test execution.

## Test Scenarios

### Scenario 1: ACL Toggle Isolation

1. Run security dashboard tests that toggle ACL on
2. Intentionally cancel mid-test (Ctrl+C)
3. Run any other E2E test (e.g., manual-dns-provider)
4. **Expected**: Tests should pass - global-setup.ts should reset ACL

### Scenario 2: State Restoration After Failure

1. Modify a security dashboard toggle test to throw an error after enabling ACL
2. Run the test (it will fail)
3. Run a different test file
4. **Expected**: ACL should be disabled, other tests should pass

### Scenario 3: Concurrent Test Runs

1. Run full E2E suite: `npx playwright test --project=chromium`
2. **Expected**: No tests fail due to ACL blocking (@api-tagged requests)
3. **Expected**: Security dashboard toggle tests complete without deadlock

### Scenario 4: Fresh Container State

1. Stop all containers: `docker compose -f .docker/compose/docker-compose.yml down -v`
2. Start fresh: `docker compose -f .docker/compose/docker-compose.ci.yml up -d`
3. Run security dashboard tests
4. **Expected**: Tests pass, ACL state properly managed

## Verification Commands

```bash
# Full E2E suite
npx playwright test --project=chromium

# Security-specific tests
npx playwright test tests/security/*.spec.ts --project=chromium

# Check ACL is disabled after tests
curl -s http://localhost:8080/api/v1/security/status | jq '.acl_enabled'
```

## Acceptance Criteria

- [ ] Security dashboard toggle tests pass consistently
- [ ] No "403 Forbidden" errors in unrelated tests after security tests run
- [ ] global-setup.ts emergency reset works when ACL is stuck enabled
- [ ] afterAll cleanup creates fresh request context (no fixture reuse errors)
