# Bug #1 Fix: CrowdSec LAPI Authentication Regression - Code Review Summary

**Date**: 2026-02-04
**Developer**: Backend Dev Agent
**Reviewer**: (Awaiting Supervisor Review)
**Status**: ✅ Implementation Complete, ⏳ Awaiting Review

---

## Executive Summary

Successfully implemented Bug #1 fix per investigation report `docs/issues/crowdsec_auth_regression.md`. The root cause was that `ensureBouncerRegistration()` validated whether a bouncer NAME existed instead of testing if the API KEY actually works against LAPI. This caused silent failures when users set invalid environment variable keys.

**Solution**: Created new `testKeyAgainstLAPI()` method that performs real authentication tests against `/v1/decisions/stream` endpoint with exponential backoff retry logic for LAPI startup delays. Updated `ensureBouncerRegistration()` to use actual authentication instead of name-based validation.

---

## Changes Overview

### Modified Files

| File | Lines Changed | Description |
|------|--------------|-------------|
| `backend/internal/api/handlers/crowdsec_handler.go` | +173 lines | Core authentication fix |
| `backend/internal/api/handlers/crowdsec_handler_test.go` | +324 lines | Comprehensive unit tests |
| `backend/integration/crowdsec_lapi_integration_test.go` | +380 lines | End-to-end integration tests |

### New Methods/Functions

#### `testKeyAgainstLAPI()` (lines 1548-1638)

**Purpose**: Validates API key by making authenticated request to LAPI `/v1/decisions/stream` endpoint.

**Behavior**:

- **Connection Refused** → Retry with exponential backoff (500ms → 750ms → 1125ms → ..., max 5s per attempt)
- **403 Forbidden** → Fail immediately (indicates invalid key, no retry)
- **200 OK** → Key valid
- **Timeout**: 30 seconds total, 5 seconds per HTTP request

**Example Log Output**:

```
time="..." level=info msg="LAPI not ready, retrying with backoff" attempt=1 error="connection refused" next_attempt_ms=500
time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="abcd...wxyz" source=file
```

#### `ensureBouncerRegistration()` (lines 1641-1686)

**Purpose**: Ensures valid bouncer authentication using environment variable → file → auto-generation priority.

**Updated Logic**:

1. Check `CROWDSEC_API_KEY` environment variable → **Test against LAPI**
2. Check `CHARON_SECURITY_CROWDSEC_API_KEY` environment variable → **Test against LAPI**
3. Check file `/app/data/crowdsec/bouncer_key` → **Test against LAPI**
4. If all fail, auto-register new bouncer and save key to file

**Breaking Changes**: None. Old `validateBouncerKey()` preserved for backward compatibility.

#### `saveKeyToFile()` (lines 1830-1856)

**Updated**: Atomic write pattern using temp file + rename.

**Security Improvements**:

- Directory created with `0700` permissions (owner only)
- Key file created with `0600` permissions (owner read/write only)
- Atomic write prevents corruption if process killed mid-write

---

## Test Coverage Metrics

### Unit Tests (10 New Tests)

**File**: `backend/internal/api/handlers/crowdsec_handler_test.go`

| Test Name | Coverage | Purpose |
|-----------|----------|---------|
| `TestTestKeyAgainstLAPI_ValidKey` | ✅ | Verifies 200 OK accepted as valid |
| `TestTestKeyAgainstLAPI_InvalidKey` | ✅ | Verifies 403 rejected immediately |
| `TestTestKeyAgainstLAPI_EmptyKey` | ✅ | Verifies empty key rejected |
| `TestTestKeyAgainstLAPI_Timeout` | ✅ | Verifies 5s timeout handling |
| `TestTestKeyAgainstLAPI_NonOKStatus` | ✅ | Verifies non-200/403 handling |
| `TestEnsureBouncerRegistration_ValidEnvKey` | ✅ | Verifies env var priority |
| `TestEnsureBouncerRegistration_InvalidEnvKeyFallback` | ✅ | Verifies auto-registration fallback |
| `TestSaveKeyToFile_AtomicWrite` | ✅ | Verifies atomic write pattern |
| `TestReadKeyFromFile_Trimming` | ✅ | Verifies whitespace handling |
| `TestGetBouncerAPIKeyFromEnv_Priority` | ✅ | Verifies env var precedence |

**Coverage Results**:

```
crowdsec_handler.go:1548: testKeyAgainstLAPI        75.0%
crowdsec_handler.go:1641: ensureBouncerRegistration 83.3%
crowdsec_handler.go:1720: registerAndSaveBouncer    78.6%
crowdsec_handler.go:1752: maskAPIKey                80.0%
crowdsec_handler.go:1802: getBouncerAPIKeyFromEnv   80.0%
crowdsec_handler.go:1819: readKeyFromFile           75.0%
crowdsec_handler.go:1830: saveKeyToFile             58.3%
```

**Overall Coverage**: 85.1% (meets 85% minimum requirement)

### Integration Tests (3 New Tests)

**File**: `backend/integration/crowdsec_lapi_integration_test.go`

| Test Name | Purpose | Docker Required |
|-----------|---------|----------------|
| `TestBouncerAuth_InvalidEnvKeyAutoRecovers` | Verifies auto-recovery from invalid env var | Yes |
| `TestBouncerAuth_ValidEnvKeyPreserved` | Verifies valid env var used without re-registration | Yes |
| `TestBouncerAuth_FileKeyPersistsAcrossRestarts` | Verifies key persistence across container restarts | Yes |

**Execution**:

```bash
cd backend
go test -tags=integration ./integration/ -run "TestBouncerAuth"
```

**Note**: Integration tests require Docker container running with CrowdSec installed.

---

## Demo: Before vs After Fix

### Before Fix (Bug Behavior)

```bash
# User sets invalid env var in docker-compose.yml
CHARON_SECURITY_CROWDSEC_API_KEY=fakeinvalidkey

# Charon starts, CrowdSec enabled
docker logs charon

# OUTPUT (BEFORE FIX):
time="..." level=info msg="Bouncer caddy-bouncer found in registry" ← ❌ WRONG: validates NAME, not KEY
time="..." level=error msg="LAPI request failed" error="access forbidden (403)" ← ❌ RUNTIME ERROR
time="..." level=error msg="CrowdSec bouncer connection failed - check API key"
```

**Result**: Persistent errors, user must manually fix env var or delete it.

---

### After Fix (Expected Behavior)

```bash
# User sets invalid env var in docker-compose.yml
CHARON_SECURITY_CROWDSEC_API_KEY=fakeinvalidkey

# Charon starts, CrowdSec enabled
docker logs charon

# OUTPUT (AFTER FIX):
time="..." level=warning msg="Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but invalid. Either remove it from docker-compose.yml or update it to match the auto-generated key. A new valid key will be generated and saved." masked_key=fake...key ← ✅ CLEAR WARNING

time="..." level=info msg="Registering new CrowdSec bouncer: caddy-bouncer" ← ✅ AUTO-RECOVERY
time="..." level=info msg="CrowdSec bouncer registration successful" masked_key="abcd...wxyz" source=auto_generated ← ✅ NEW KEY GENERATED

time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="abcd...wxyz" source=file ← ✅ SUCCESS
```

**Result**: Auto-recovery, user sees clear warning message, system continues working.

---

## Security Enhancements

### API Key Masking (CWE-312 Mitigation)

**Function**: `maskAPIKey()` (line 1752)

**Behavior**:

- Keys < 8 chars: Return `[REDACTED]`
- Keys >= 8 chars: Return `first4...last4` (e.g., `abcd...wxyz`)

**Example**:

```go
maskAPIKey("valid-api-key-12345678")
// Returns: "vali...5678"
```

**Rationale**: Prevents full key exposure in logs while allowing users to verify which key is active.

### File Permissions

| Object | Permission | Rationale |
|--------|-----------|-----------|
| `/app/data/crowdsec/` | `0700` | Owner-only directory access |
| `/app/data/crowdsec/bouncer_key` | `0600` | Owner read/write only |

**Code**:

```go
os.MkdirAll(filepath.Dir(keyFile), 0700)
os.WriteFile(tempPath, []byte(apiKey), 0600)
```

### Atomic File Writes

**Pattern**: Temp file + rename (POSIX atomic operation)

```go
tempPath := keyFile + ".tmp"
os.WriteFile(tempPath, []byte(apiKey), 0600)  // Write to temp
os.Rename(tempPath, keyFile)                   // Atomic rename
```

**Rationale**: Prevents partial writes if process killed mid-operation.

---

## Breaking Changes

**None**. All changes are backward compatible:

- Old `validateBouncerKey()` method preserved but unused
- Environment variable names unchanged (`CROWDSEC_API_KEY` and `CHARON_SECURITY_CROWDSEC_API_KEY`)
- File path unchanged (`/app/data/crowdsec/bouncer_key`)
- API endpoints unchanged

---

## Manual Verification Guide

**Document**: `docs/testing/crowdsec_auth_manual_verification.md`

**Test Scenarios**:

1. Invalid Environment Variable Auto-Recovery
2. LAPI Startup Delay Handling (30s retry window)
3. No More "Access Forbidden" Errors in Production
4. Key Source Visibility in Logs (env var vs file vs auto-generated)

**How to Test**:

```bash
# Scenario 1: Invalid env var
echo "CHARON_SECURITY_CROWDSEC_API_KEY=fakeinvalidkey" >> docker-compose.yml
docker compose up -d
docker logs -f charon | grep -i "invalid"

# Expected: Warning message, new key generated, authentication successful
```

---

## Code Quality Checklist

- ✅ **Linting**: `go vet ./...` and `staticcheck ./...` passed
- ✅ **Tests**: All 10 unit tests passing with 85.1% coverage
- ✅ **Race Detector**: `go test -race ./...` found no data races
- ✅ **Error Handling**: All error paths wrapped with `fmt.Errorf` for context
- ✅ **Logging**: Structured logging with masked sensitive data
- ✅ **Documentation**: Comments explain "why" not "what"
- ✅ **Security**: API keys masked, file permissions secured, atomic writes
- ✅ **Integration Tests**: 3 Docker-based tests added for end-to-end validation

---

## Performance Considerations

### Retry Backoff Strategy

**Formula**: `nextBackoff = currentBackoff * 1.5` (exponential)

**Timings**:

- Attempt 1: 500ms delay
- Attempt 2: 750ms delay
- Attempt 3: 1.125s delay
- Attempt 4: 1.687s delay (capped at 5s)
- ...continues until 30s total timeout

**Cap**: 5 seconds per HTTP request (prevents indefinite hangs)

**Rationale**: Allows LAPI up to 30 seconds to start while avoiding aggressive polling.

### HTTP Client Configuration

```go
httpClient := network.NewInternalServiceHTTPClient()
httpClient.Timeout = 5 * time.Second // Per-request timeout

req, _ := http.NewRequestWithContext(ctx, "GET", lapiURL+"/v1/decisions/stream", nil)
```

**Context**: 5-second timeout per attempt, separate from total 30-second retry window.

---

## Known Limitations

1. **Docker-Only Integration Tests**: Integration tests require Docker container with CrowdSec installed. Cannot run in pure unit test environment.

2. **Manual Environment Variable Setup**: For `TestBouncerAuth_ValidEnvKeyPreserved`, user must manually set `CHARON_SECURITY_CROWDSEC_API_KEY` to a pre-registered key before starting container. Test cannot set env vars dynamically for running containers.

3. **LAPI Binary Availability**: Tests skip if CrowdSec binary not found in container. This is expected behavior for minimal development images.

---

## Deployment Checklist

Before merging to main:

- [ ] All unit tests passing (10/10)
- [ ] Coverage ≥ 85% (currently 85.1%)
- [ ] Integration tests passing (3/3 when Docker available)
- [ ] Manual verification scenarios tested
- [ ] No "access forbidden" errors in production logs after fix
- [ ] Backward compatibility verified (old containers work with new code)
- [ ] Security review completed (API key masking, file permissions)
- [ ] Documentation updated (this summary, manual verification guide)

---

## Next Steps

1. **Supervisor Code Review**: Review implementation for correctness, security, and maintainability
2. **QA Testing**: Execute manual verification scenarios in staging environment
3. **Integration Test Execution**: Run Docker-based integration tests in CI/CD
4. **Deployment**: Merge to main after approval
5. **Monitor Production**: Watch for "access forbidden" errors post-deployment

---

## Questions for Reviewer

1. Should we add telemetry/metrics for retry counts and authentication failures?
2. Is 30-second LAPI startup window acceptable, or should we make it configurable?
3. Should we add a health check endpoint specifically for CrowdSec/LAPI status?
4. Do we need user-facing documentation for environment variable best practices?

---

## Related Files

- **Investigation Report**: `docs/issues/crowdsec_auth_regression.md`
- **Implementation**: `backend/internal/api/handlers/crowdsec_handler.go` (lines 1548-1720)
- **Unit Tests**: `backend/internal/api/handlers/crowdsec_handler_test.go` (lines 3970-4294)
- **Integration Tests**: `backend/integration/crowdsec_lapi_integration_test.go`
- **Manual Verification**: `docs/testing/crowdsec_auth_manual_verification.md`

---

## Contact

**Developer**: Backend Dev Agent (GitHub Copilot)
**Date Completed**: 2026-02-04
**Estimated Review Time**: 15-20 minutes
