# Security Coverage QA Test Plan

## Overview

This document outlines the comprehensive test plan to achieve **100% code coverage** for all security-related functionality in Charon. Security is a critical selling point for a reverse proxy, and this plan ensures all security code paths are thoroughly tested.

**Target Coverage:** 100% for all security-related code
**Minimum Threshold:** 85% overall (enforced by pre-commit)

---

## Current Coverage Status (Updated 2025-12-12)

### Coverage Summary

| Package | Coverage | Status |
|---------|----------|--------|
| `cerberus` | **100.0%** | ✅ Complete |
| `middleware` | **98.1%** | ✅ Complete |
| `handlers` | **84.1%** | ✅ Above threshold |
| `crowdsec` | **82.1%** | 🟡 Below threshold |
| `services` | **82.0%** | 🟡 Below threshold |

### Tests Added This Session

#### CrowdSec Handler Tests (`crowdsec_handler_test.go`)

- ✅ Console enrollment tests (disabled, service unavailable, invalid payload, success, missing agent name)
- ✅ Console status tests (disabled, unavailable, success, after enrollment)
- ✅ `isConsoleEnrollmentEnabled` tests (DB variants, env variants, defaults)
- ✅ `actorFromContext` tests (with userID, numeric userID, no user)
- ✅ `ttlRemainingSeconds` tests (normal, expired, zero time, zero TTL)
- ✅ `hubEndpoints` tests (nil, deduplicates, multiple, skips empty)

#### CrowdSec Package Tests

- ✅ `ExecuteWithEnv` - 100% coverage
- ✅ `formatEnv` - 100% coverage
- ✅ `hubHTTPError.Error()` - 100% coverage
- ✅ `hubHTTPError.Unwrap()` - 100% coverage
- ✅ `hubHTTPError.CanFallback()` - 100% coverage
- ✅ `deriveKey` - 100% coverage
- ✅ `normalizeEnrollmentKey` - 100% coverage
- ✅ `redactSecret` - 100% coverage
- ✅ Encryption round-trip tests

#### Cerberus Middleware Tests (`cerberus_test.go`)

- ✅ `IsEnabled` - All branches covered (config, DB setting, legacy setting, modes)
- ✅ `Middleware` - 100% coverage achieved
  - Disabled state (skip checks)
  - WAF enabled (metrics tracking)
  - ACL enabled - no lists
  - ACL enabled - disabled list
  - ACL enabled - blocked by blacklist
  - CrowdSec local mode (metrics tracking)

#### Access List Handler Tests

- ✅ `SetGeoIPService` - 100% coverage (was 0%)
- ✅ `TestIP` - 100% coverage (was 89.5%)
- ✅ Internal error path covered

#### Security Service Tests

- ✅ `DeleteRuleSet` not found case
- ✅ `ListDecisions` unlimited and limited variants
- ✅ `LogDecision` nil and prefilled UUID
- ✅ `ListRuleSets` empty database
- ✅ `Upsert` invalid CrowdSec mode variants

### Backend Security Handlers

| File | Function | Current | Target | Priority |
|------|----------|---------|--------|----------|
| `console_enroll.go` | `ExecuteWithEnv` | **0.0%** | 100% | 🔴 CRITICAL |
| `console_enroll.go` | `formatEnv` | **0.0%** | 100% | 🔴 CRITICAL |
| `console_enroll.go` | `Status` | **0.0%** | 100% | 🔴 CRITICAL |
| `console_enroll.go` | `TTL` | **0.0%** | 100% | 🔴 CRITICAL |
| `console_enroll.go` | `Unwrap` | **0.0%** | 100% | 🔴 CRITICAL |
| `hub_sync.go` | `emptyDir` | **30.8%** | 100% | 🔴 CRITICAL |
| `hub_sync.go` | `backupExisting` | **36.4%** | 100% | 🔴 CRITICAL |
| `hub_sync.go` | `extractTarGz` | **65.9%** | 100% | 🟡 HIGH |
| `hub_sync.go` | `Pull` | **65.4%** | 100% | 🟡 HIGH |

### Backend Services Package

| File | Function | Current | Target | Priority |
|------|----------|---------|--------|----------|
| `access_list_service.go` | `testGeoIP` | **9.1%** | 100% | 🔴 CRITICAL |
| `security_service.go` | `GenerateBreakGlassToken` | **73.7%** | 100% | 🟡 HIGH |
| `security_service.go` | `DeleteRuleSet` | **75.0%** | 100% | 🟡 HIGH |
| `security_service.go` | `ListRuleSets` | **75.0%** | 100% | 🟡 HIGH |

### Backend Cerberus Package

| File | Function | Current | Target | Priority |
|------|----------|---------|--------|----------|
| `cerberus.go` | `Middleware` | **81.8%** | 100% | 🟢 MEDIUM |

### Frontend Security Components

| File | Current | Target | Priority |
|------|---------|--------|----------|
| `api/consoleEnrollment.ts` | **0.0%** | 100% | 🔴 CRITICAL |
| `crowdsec.ts` | **81.81%** | 100% | 🟢 MEDIUM |
| `pages/Security.tsx` | **82.35%** | 100% | 🟢 MEDIUM |
| `pages/WafConfig.tsx` | **89.47%** | 100% | 🟢 MEDIUM |
| `pages/CrowdSecConfig.tsx` | **85.96%** | 100% | 🟢 MEDIUM |
| `pages/RateLimiting.tsx` | **90.32%** | 100% | 🟢 MEDIUM |

---

## Test Cases by Component

### 1. CrowdSec Console Enrollment Tests (Handler)

#### `ConsoleEnroll` (0% → 100%)

```go
// TEST CASE 1.1: Successful enrollment
// Input: Valid enrollment key, valid accept_tos
// Expected: 200 OK, enrollment initiated, correct response body
// Validates: Happy path enrollment flow

// TEST CASE 1.2: Missing enrollment key
// Input: Empty enrollment_key field
// Expected: 400 Bad Request with validation error
// Validates: Input validation

// TEST CASE 1.3: TOS not accepted
// Input: Valid key, accept_tos=false
// Expected: 400 Bad Request with TOS error
// Validates: Business rule enforcement

// TEST CASE 1.4: Feature disabled in config
// Input: Valid request but console enrollment disabled
// Expected: 400 Bad Request with feature disabled error
// Validates: Feature flag checking

// TEST CASE 1.5: Enrollment execution error
// Input: Valid request, execution fails
// Expected: 500 Internal Server Error with appropriate message
// Validates: Error handling
```

#### `ConsoleStatus` (0% → 100%)

```go
// TEST CASE 1.6: Get status when enrolled
// Input: GET request when console is enrolled
// Expected: 200 OK with enrolled=true, expiry info
// Validates: Enrolled state reporting

// TEST CASE 1.7: Get status when not enrolled
// Input: GET request when console is not enrolled
// Expected: 200 OK with enrolled=false
// Validates: Unenrolled state reporting

// TEST CASE 1.8: Feature disabled
// Input: GET request when feature disabled
// Expected: 400 Bad Request
// Validates: Feature flag checking
```

#### `isConsoleEnrollmentEnabled` (0% → 100%)

```go
// TEST CASE 1.9: Enabled in config
// Input: Config with CROWDSEC_CONSOLE_ENROLLMENT_KEY set
// Expected: Returns true
// Validates: Positive flag detection

// TEST CASE 1.10: Disabled in config
// Input: Config without enrollment key
// Expected: Returns false
// Validates: Negative flag detection
```

#### `actorFromContext` (0% → 100%)

```go
// TEST CASE 1.11: User in context
// Input: Gin context with authenticated user
// Expected: Returns username
// Validates: User extraction from context

// TEST CASE 1.12: No user in context
// Input: Gin context without user
// Expected: Returns "system" or empty string
// Validates: Fallback behavior
```

### 2. CrowdSec LAPI Tests (Handler)

#### `GetLAPIDecisions` (40% → 100%)

```go
// TEST CASE 2.1: Successful decisions retrieval
// Input: Valid LAPI connection
// Expected: 200 OK with decisions list
// Validates: Happy path

// TEST CASE 2.2: LAPI unavailable
// Input: LAPI service not running
// Expected: 503 Service Unavailable
// Validates: Service dependency handling

// TEST CASE 2.3: Empty decisions list
// Input: Valid LAPI with no active decisions
// Expected: 200 OK with empty array
// Validates: Empty state handling

// TEST CASE 2.4: Connection timeout
// Input: LAPI slow to respond
// Expected: 504 Gateway Timeout
// Validates: Timeout handling

// TEST CASE 2.5: Invalid API key
// Input: Invalid LAPI key
// Expected: 401 Unauthorized
// Validates: Authentication error handling
```

#### `CheckLAPIHealth` (41.4% → 100%)

```go
// TEST CASE 2.6: LAPI healthy
// Input: Healthy LAPI endpoint
// Expected: 200 OK with healthy=true
// Validates: Health check success

// TEST CASE 2.7: LAPI unhealthy
// Input: Unhealthy LAPI endpoint
// Expected: 200 OK with healthy=false, error details
// Validates: Health check failure reporting

// TEST CASE 2.8: Connection refused
// Input: LAPI not listening
// Expected: Appropriate error response
// Validates: Connection error handling
```

### 3. Security Handler Tests

#### `LookupGeoIP` (38.9% → 100%)

```go
// TEST CASE 3.1: Successful IP lookup
// Input: Valid public IP address
// Expected: 200 OK with country, city, coordinates
// Validates: GeoIP resolution

// TEST CASE 3.2: Private IP address
// Input: 192.168.1.1 or 10.0.0.1
// Expected: 200 OK with "Private Address" response
// Validates: Private IP handling

// TEST CASE 3.3: Invalid IP format
// Input: "not-an-ip"
// Expected: 400 Bad Request
// Validates: Input validation

// TEST CASE 3.4: GeoIP database not loaded
// Input: Valid IP, GeoIP service unavailable
// Expected: 503 Service Unavailable
// Validates: Dependency handling

// TEST CASE 3.5: Localhost/loopback
// Input: 127.0.0.1
// Expected: 200 OK with "Localhost" response
// Validates: Loopback handling
```

#### `ReloadGeoIP` (58.3% → 100%)

```go
// TEST CASE 3.6: Successful reload
// Input: POST to reload with valid database path
// Expected: 200 OK with success message
// Validates: Hot reload functionality

// TEST CASE 3.7: Database file not found
// Input: POST with invalid database path
// Expected: 500 Internal Server Error
// Validates: File error handling

// TEST CASE 3.8: Corrupt database file
// Input: POST with corrupt MaxMind database
// Expected: 500 Internal Server Error with parse error
// Validates: Database validation
```

#### `UpdateConfig` (69.2% → 100%)

```go
// TEST CASE 3.9: Update all security settings
// Input: Complete SecurityConfig object
// Expected: 200 OK with updated config
// Validates: Full config update

// TEST CASE 3.10: Partial update
// Input: Partial config (only rate_limit changes)
// Expected: 200 OK with merged config
// Validates: Partial update handling

// TEST CASE 3.11: Invalid config values
// Input: Negative rate limit values
// Expected: 400 Bad Request
// Validates: Config validation

// TEST CASE 3.12: Enable WAF
// Input: waf.enabled = true
// Expected: 200 OK, WAF activated
// Validates: WAF toggle

// TEST CASE 3.13: Update ACL settings
// Input: acl.default_action = "deny"
// Expected: 200 OK, ACL updated
// Validates: ACL config update
```

### 4. CrowdSec Package Tests

#### `ExecuteWithEnv` (0% → 100%)

```go
// TEST CASE 4.1: Execute command successfully
// Input: Valid command with env vars
// Expected: Success, command executed
// Validates: Environment variable passing

// TEST CASE 4.2: Execute with missing env
// Input: Command requiring env vars not set
// Expected: Error with clear message
// Validates: Missing env handling

// TEST CASE 4.3: Command execution failure
// Input: Command that returns non-zero exit
// Expected: Error with exit code and stderr
// Validates: Error propagation
```

#### `backupExisting` (36.4% → 100%)

```go
// TEST CASE 4.4: Backup when directory exists
// Input: Existing directory with files
// Expected: Backup created with timestamp
// Validates: Backup creation

// TEST CASE 4.5: Backup when directory doesn't exist
// Input: Non-existent directory
// Expected: No error, no backup created
// Validates: Missing directory handling

// TEST CASE 4.6: Permission denied during backup
// Input: Read-only filesystem
// Expected: Error with permission message
// Validates: Permission error handling
```

#### `emptyDir` (30.8% → 100%)

```go
// TEST CASE 4.7: Empty directory with files
// Input: Directory with files and subdirectories
// Expected: All contents removed, directory remains
// Validates: Recursive deletion

// TEST CASE 4.8: Empty non-existent directory
// Input: Directory that doesn't exist
// Expected: Error or no-op (depending on design)
// Validates: Missing directory handling

// TEST CASE 4.9: Empty directory with symlinks
// Input: Directory containing symlinks
// Expected: Symlinks removed, targets untouched
// Validates: Symlink handling
```

#### `extractTarGz` (65.9% → 100%)

```go
// TEST CASE 4.10: Extract valid tarball
// Input: Valid .tar.gz file
// Expected: Contents extracted to destination
// Validates: Basic extraction

// TEST CASE 4.11: Extract corrupted tarball
// Input: Corrupted .tar.gz file
// Expected: Error with corruption message
// Validates: Corruption detection

// TEST CASE 4.12: Extract with path traversal attempt
// Input: Malicious tarball with "../" paths
// Expected: Error or sanitized extraction
// Validates: Security - path traversal prevention

// TEST CASE 4.13: Extract to non-existent directory
// Input: Destination doesn't exist
// Expected: Directory created, contents extracted
// Validates: Auto-directory creation
```

### 5. Services Package Tests

#### `testGeoIP` (9.1% → 100%)

```go
// TEST CASE 5.1: Test with valid GeoIP database
// Input: Valid MaxMind database path
// Expected: Test passes, returns success
// Validates: Database validation

// TEST CASE 5.2: Test with missing database
// Input: Non-existent database path
// Expected: Error indicating file not found
// Validates: Missing file handling

// TEST CASE 5.3: Test with wrong file format
// Input: Non-MaxMind file (e.g., text file)
// Expected: Error indicating invalid format
// Validates: Format validation

// TEST CASE 5.4: Test with empty path
// Input: Empty string for database path
// Expected: Appropriate error or default behavior
// Validates: Empty input handling
```

#### `GenerateBreakGlassToken` (73.7% → 100%)

```go
// TEST CASE 5.5: Generate valid token
// Input: Request to generate token
// Expected: Token with proper expiry, stored in DB
// Validates: Token generation

// TEST CASE 5.6: Generate when existing token exists
// Input: Request when unexpired token exists
// Expected: Existing token returned or new one generated
// Validates: Token reuse policy

// TEST CASE 5.7: Token expiry configuration
// Input: Custom expiry duration
// Expected: Token has correct expiry time
// Validates: Expiry configuration
```

### 6. Cerberus Package Tests

#### `Middleware` (81.8% → 100%)

```go
// TEST CASE 6.1: Request when Cerberus disabled
// Input: Request with Cerberus disabled in config
// Expected: Request passes through unmodified
// Validates: Bypass when disabled

// TEST CASE 6.2: Request blocked by WAF
// Input: Request matching WAF rule
// Expected: 403 Forbidden with WAF block reason
// Validates: WAF integration

// TEST CASE 6.3: Request blocked by ACL
// Input: Request from blocked IP
// Expected: 403 Forbidden with ACL block reason
// Validates: ACL integration

// TEST CASE 6.4: Request blocked by rate limit
// Input: Request exceeding rate limit
// Expected: 429 Too Many Requests
// Validates: Rate limiting integration

// TEST CASE 6.5: Request allowed through all checks
// Input: Legitimate request passing all checks
// Expected: Request proceeds to handler
// Validates: Pass-through behavior

// TEST CASE 6.6: Break glass token bypass
// Input: Request with valid break-glass token
// Expected: Request bypasses security checks
// Validates: Emergency bypass

// TEST CASE 6.7: GeoIP blocking
// Input: Request from blocked country
// Expected: 403 Forbidden with geo block reason
// Validates: GeoIP integration
```

---

## Frontend Test Cases

### 7. Console Enrollment API Tests

#### `api/consoleEnrollment.ts` (0% → 100%)

```typescript
// TEST CASE 7.1: enrollConsole - successful enrollment
// Input: Valid enrollment key, accepted TOS
// Expected: Success response with enrollment status
// Validates: API call construction and response handling

// TEST CASE 7.2: enrollConsole - network error
// Input: Network failure during enrollment
// Expected: Error thrown with appropriate message
// Validates: Error handling

// TEST CASE 7.3: getConsoleStatus - enrolled state
// Input: GET request when enrolled
// Expected: Returns enrolled status with details
// Validates: Status parsing

// TEST CASE 7.4: getConsoleStatus - not enrolled
// Input: GET request when not enrolled
// Expected: Returns not enrolled status
// Validates: Negative state handling
```

### 8. Security Page Tests

#### `pages/Security.tsx` (82.35% → 100%)

```typescript
// TEST CASE 8.1: Render security dashboard
// Input: Component mount with mock data
// Expected: All security sections displayed
// Validates: Basic rendering

// TEST CASE 8.2: Toggle Cerberus enabled
// Input: Click Cerberus toggle
// Expected: API called, state updated
// Validates: Toggle interaction

// TEST CASE 8.3: Enable rate limiting with presets
// Input: Select rate limit preset
// Expected: Rate limit config applied
// Validates: Preset application

// TEST CASE 8.4: Generate break glass token
// Input: Click generate token button
// Expected: Token displayed, copy functionality works
// Validates: Token generation UI

// TEST CASE 8.5: Error state display
// Input: API returns error
// Expected: Error message displayed to user
// Validates: Error handling UI
```

### 9. WAF Config Tests

#### `pages/WafConfig.tsx` (89.47% → 100%)

```typescript
// TEST CASE 9.1: Render WAF configuration
// Input: Component mount with mock WAF config
// Expected: All WAF settings displayed
// Validates: Basic rendering

// TEST CASE 9.2: Add WAF exclusion
// Input: Enter exclusion pattern and save
// Expected: Exclusion added to list
// Validates: Exclusion creation

// TEST CASE 9.3: Delete WAF exclusion
// Input: Click delete on existing exclusion
// Expected: Exclusion removed after confirmation
// Validates: Exclusion deletion

// TEST CASE 9.4: Rule set management
// Input: Toggle rule set enabled/disabled
// Expected: Rule set state updated
// Validates: Rule set toggling
```

---

## Test Execution Order

### Phase 1: Critical (0% Coverage) - IMMEDIATE

1. CrowdSec Console tests (ConsoleEnroll, ConsoleStatus)
2. `testGeoIP` service tests
3. `SetGeoIPService` handler tests
4. CrowdSec package tests (ExecuteWithEnv, formatEnv, Status)

### Phase 2: High Priority (<70% Coverage) - WEEK 1

1. `LookupGeoIP` handler tests
2. `GetLAPIDecisions` handler tests
3. `CheckLAPIHealth` handler tests
4. `ReloadGeoIP` handler tests
5. `emptyDir` and `backupExisting` tests

### Phase 3: Medium Priority (70-90% Coverage) - WEEK 2

1. Remaining handler coverage gaps
2. Cerberus middleware edge cases
3. Frontend console enrollment tests
4. Security.tsx remaining coverage
5. WafConfig.tsx remaining coverage

---

## QA Workflow

### For Each Test Case

1. **QA writes test** following expected behavior
2. **If test passes**: Mark as ✅ complete
3. **If test fails due to missing code behavior**: Create DEV issue to implement behavior
4. **If test fails due to bug**: Create DEV issue to fix bug

### Dev Handoff Format

```markdown
## Test Failure Report

**Test Case ID:** 3.4
**Test Name:** LookupGeoIP_GeoIPServiceUnavailable
**Expected Behavior:** Return 503 Service Unavailable with error message
**Actual Behavior:** Returns 500 with generic error / panics / returns nil

**Test Code:**
[Include test code]

**Required Fix:**
- Add nil check for GeoIP service
- Return appropriate HTTP status code
- Include helpful error message for debugging
```

---

## Coverage Commands

```bash
# Backend full coverage
cd backend && go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep -E "(security|crowdsec|access_list|cerberus)"

# Backend specific package
go test -coverprofile=handlers.out ./internal/api/handlers/...
go tool cover -func=handlers.out

# Frontend coverage
cd frontend && npm run test:coverage
```

---

## Definition of Done

- [ ] All 0% coverage functions have tests
- [ ] All security handlers ≥95% coverage
- [ ] All CrowdSec package functions ≥95% coverage
- [ ] All security services ≥95% coverage
- [ ] Cerberus middleware 100% coverage
- [ ] Frontend security components ≥95% coverage
- [ ] All tests pass
- [ ] Pre-commit passes with ≥85% overall coverage
- [ ] No TODO comments in security code
- [ ] All error paths tested
