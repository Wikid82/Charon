# Phase 4 UAT Test Suite

Comprehensive User Acceptance Testing for Charon reverse proxy system before production beta release.

## Overview

**Test Count**: 70 tests across 8 feature areas
**Framework**: Playwright Test (Firefox)
**Base URL**: `http://127.0.0.1:8080` (Docker container)
**Coverage**: Admin onboarding, user management, proxy hosts, security configuration, domain/DNS, monitoring, backup/recovery, emergency operations

## Test Files

### 01-admin-onboarding.spec.ts (8 tests)
- **Purpose**: Validate first-time admin setup and dashboard experience
- **Tests**:
  - Admin login with performance measurement (<5s)
  - Dashboard widget display and functionality
  - Settings page navigation and access
  - Emergency token generation (modal and inline display)
  - Encryption key setup and storage
  - Navigation menu item visibility and navigation
  - Logout and session cleanup
  - Re-login validation and session restoration

### 02-user-management.spec.ts (10 tests)
- **Purpose**: User CRUD operations and role-based access control
- **Tests**:
  - Create user (all fields, minimal fields)
  - Assign and modify user roles
  - Delete user with confirmation
  - Login as user with restricted permissions
  - Unauthorized API access from guest role
  - Guest role minimal permissions
  - Email address modification
  - Password reset workflow with login validation
  - Search users by email address
  - Pagination with large user count (>25 users)

### 03-proxy-host-management.spec.ts (12 tests)
- **Purpose**: Reverse proxy lifecycle and configuration
- **Tests**:
  - Create proxy with domain and target validation
  - Edit proxy configuration
  - Delete proxy with cleanup
  - SSL/TLS certificate setup
  - Traffic routing and verification
  - Access list configuration and enforcement
  - WAF integration with proxy
  - Rate limiting application to proxy
  - Domain regex pattern validation
  - Proxy statistics display
  - Disable/enable proxy toggle
  - Form validation error handling

### 04-security-configuration.spec.ts (10 tests)
- **Purpose**: Security module enablement and configuration
- **Tests**:
  - Enable Cerberus ACL module
  - Enable Coraza WAF module
  - Enable rate limiting
  - Enable CrowdSec integration
  - Configure ACL rules (IP whitelist)
  - Adjust WAF sensitivity levels
  - Set rate limiting thresholds (100 req/60s example)
  - CrowdSec API key field verification
  - Malicious payload blocking via API call
  - Security dashboard status display

### 05-domain-dns-management.spec.ts (8 tests)
- **Purpose**: Domain and DNS provider lifecycle
- **Tests**:
  - Add domain (test.example.com)
  - View DNS records (A, AAAA, CNAME)
  - Add DNS provider with credentials
  - Verify domain ownership (DNS TXT/CNAME)
  - Renew SSL certificate with confirmation
  - View domain statistics (cert expiry, uptime, DNS status)
  - Disable domain toggle
  - Export domains as JSON file

### 06-monitoring-audit.spec.ts (8 tests)
- **Purpose**: Logging, monitoring, and audit trail functionality
- **Tests**:
  - Real-time log stream display
  - Filter logs by severity level (error, info, etc.)
  - Search logs by keyword
  - Export logs to CSV file with download handling
  - Pagination with 100+ log entries
  - Audit trail showing user actions with timestamps
  - Security events logged and displayed
  - Log retention policy enforcement

### 07-backup-recovery.spec.ts (9 tests)
- **Purpose**: Backup and disaster recovery
- **Tests**:
  - Create manual backup through UI
  - Schedule automatic backups (daily)
  - Download backup file
  - Restore from backup with confirmation
  - Verify data integrity post-restore (users, proxies)
  - Delete backup with confirmation
  - Enable encryption for backups
  - Restore with password protection field
  - Retention policy (keep 7 backups max)

### 08-emergency-operations.spec.ts (5 tests)
- **Purpose**: Break-glass recovery and emergency procedures
- **Tests**:
  - Emergency token availability and access
  - Break-glass recovery procedures (navigation)
  - Disable WAF in emergency mode (no auth required)
  - Reset encryption key (availability verification)
  - Emergency token usage logging in audit trail

## Execution

### Run all UAT tests:
```bash
cd /projects/Charon
npx playwright test tests/phase4-uat/ --project=firefox
```

### Run specific feature tests:
```bash
npx playwright test tests/phase4-uat/02-user-management.spec.ts --project=firefox
```

### Run with debugging:
```bash
npx playwright test tests/phase4-uat/ --project=firefox --debug
```

### Run with headed browser (visible):
```bash
npx playwright test tests/phase4-uat/ --project=firefox --headed
```

### View test report:
```bash
npx playwright show-report
```

## Prerequisites

1. **Docker environment running**:
   ```bash
   .github/skills/scripts/skill-runner.sh docker-rebuild-e2e
   ```
   This starts the Charon application on `http://127.0.0.1:8080`

2. **Playwright dependencies installed**:
   ```bash
   npm install
   npx playwright install firefox
   ```

3. **Valid admin credentials** for initial login (from environment or `.env` file)

## Test Data Management

- **Test users**: Created with unique emails (`test-FEATURE@test.local`)
- **Test proxies**: Domains like `feature-test.local`
- **Cleanup**: `afterEach` hooks delete all created test data via UI operations
- **No data persistence**: Each test run is isolated, no test data leaks

## Success Criteria

✅ All 8 test files compile without syntax errors
✅ All 70 tests execute and pass against staging environment
✅ Dashboard loads within 5 seconds
✅ User creation completes within 10 seconds
✅ Proxy creation completes within 10 seconds
✅ Emergency procedures accessible and documented
✅ Backup/restore workflow functional
✅ Security modules configurable and togglable
✅ Audit logging captures all user actions
✅ Data cleanup runs successfully (no orphaned test data)

## Troubleshooting

### Container not running
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

### Tests timeout
- Increase timeout: `--timeout=120000`
- Check container health: `docker ps | grep charon-e2e`

### Locator failures (element not found)
- Run in headed mode: `--headed`
- Use `--debug` to pause and inspect
- Check selector patterns in test file (getByRole, getByLabel, getByText)

### Port already in use
- Kill existing container: `docker kill charon-e2e`
- Rebuild fresh: `docker-rebuild-e2e`

## Notes

- **Firefox only**: Phase 4 tests run Firefox to save time (tests are feature-focused, not browser-specific)
- **Performance measurements**: Login, user creation, proxy creation are timed for baseline metrics
- **Soft assertions**: Optional features use `.isVisible().catch(() => false)` to handle deployment variations
- **Test organization**: Tests group by functional feature area, not by technical layer
- **Accessibility**: Uses semantic selectors (getByRole, getByLabel) for better resilience

## Integration with CI/CD

These tests run as part of the Phase 4 validation gate before production beta release:

```yaml
# .github/workflows/phase4-uat.yml
- runs: npx playwright test tests/phase4-uat/ --project=firefox
  timeout: 30 minutes
  screenshots: retain-on-failure
```

## Contact & Support

For issues or questions about the test suite:
1. Check test output for specific failure messages
2. Run individual test in debug mode
3. Verify Docker container is healthy and responsive
4. Check application logs: `docker logs charon-e2e`
