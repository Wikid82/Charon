# CrowdSec Authentication Fix - Manual Verification Guide

This document provides step-by-step procedures for manually verifying the Bug #1 fix (CrowdSec LAPI authentication regression).

## Prerequisites

- Docker and docker-compose installed
- Charon container running (either `charon-e2e` for testing or production container)
- Access to container logs
- Basic understanding of CrowdSec bouncer authentication

## Test Scenarios

### Scenario 1: Invalid Environment Variable Auto-Recovery

**Objective**: Verify that when `CHARON_SECURITY_CROWDSEC_API_KEY` or `CROWDSEC_API_KEY` is set to an invalid key, Charon detects the failure and auto-generates a new valid key.

**Steps**:

1. **Set Invalid Environment Variable**

   Edit your `docker-compose.yml` or `.env` file:

   ```yaml
   environment:
     CHARON_SECURITY_CROWDSEC_API_KEY: fakeinvalidkey12345
   ```

2. **Start/Restart Container**

   ```bash
   docker compose up -d charon
   # OR
   docker restart charon
   ```

3. **Enable CrowdSec via API**

   ```bash
   # Login first (adjust credentials as needed)
   curl -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"admin@example.com","password":"yourpassword"}'

   # Enable CrowdSec
   curl -b cookies.txt -X POST http://localhost:8080/api/v1/admin/crowdsec/start
   ```

4. **Verify Logs Show Validation Failure**

   ```bash
   docker logs charon --tail 100 | grep -i "invalid"
   ```

   **Expected Output**:
   ```
   time="..." level=warning msg="Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but invalid. Either remove it from docker-compose.yml or update it to match the auto-generated key. A new valid key will be generated and saved." masked_key=fake...345
   ```

5. **Verify New Key Auto-Generated**

   ```bash
   docker exec charon cat /app/data/crowdsec/bouncer_key
   ```

   **Expected**: A valid CrowdSec API key (NOT `fakeinvalidkey12345`)

6. **Verify Caddy Bouncer Connects Successfully**

   ```bash
   # Test authentication with new key
   NEW_KEY=$(docker exec charon cat /app/data/crowdsec/bouncer_key)
   curl -H "X-Api-Key: $NEW_KEY" http://localhost:8080/v1/decisions/stream
   ```

   **Expected**: HTTP 200 OK (may return empty `{"new":null,"deleted":null}`)

7. **Verify Logs Show Success**

   ```bash
   docker logs charon --tail 50 | grep -i "authentication successful"
   ```

   **Expected Output**:
   ```
   time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="abcd...wxyz" source=file
   ```

**Success Criteria**:
- ✅ Warning logged about invalid env var
- ✅ New key auto-generated and saved to `/app/data/crowdsec/bouncer_key`
- ✅ Bouncer authenticates successfully with new key
- ✅ No "access forbidden" errors in logs

---

### Scenario 2: LAPI Startup Delay Handling

**Objective**: Verify that when LAPI starts 5+ seconds after Charon, the retry logic succeeds instead of immediately failing.

**Steps**:

1. **Stop Any Running CrowdSec Instance**

   ```bash
   docker exec charon pkill -9 crowdsec || true
   ```

2. **Enable CrowdSec via API** (while LAPI is down)

   ```bash
   curl -b cookies.txt -X POST http://localhost:8080/api/v1/admin/crowdsec/start
   ```

3. **Monitor Logs for Retry Messages**

   ```bash
   docker logs -f charon 2>&1 | grep -i "lapi not ready"
   ```

   **Expected Output**:
   ```
   time="..." level=info msg="LAPI not ready, retrying with backoff" attempt=1 error="connection refused" next_attempt_ms=500
   time="..." level=info msg="LAPI not ready, retrying with backoff" attempt=2 error="connection refused" next_attempt_ms=750
   time="..." level=info msg="LAPI not ready, retrying with backoff" attempt=3 error="connection refused" next_attempt_ms=1125
   ```

4. **Wait for LAPI to Start** (up to 30 seconds)

   Look for success message:
   ```
   time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="abcd...wxyz" source=file
   ```

5. **Verify Bouncer Connection**

   ```bash
   KEY=$(docker exec charon cat /app/data/crowdsec/bouncer_key)
   curl -H "X-Api-Key: $KEY" http://localhost:8080/v1/decisions/stream
   ```

   **Expected**: HTTP 200 OK

**Success Criteria**:
- ✅ Logs show retry attempts with exponential backoff (500ms → 750ms → 1125ms → ...)
- ✅ Connection succeeds after LAPI starts (within 30s max)
- ✅ No immediate failure on first connection refused error

---

### Scenario 3: No More "Access Forbidden" Errors in Production

**Objective**: Verify that setting an invalid environment variable no longer causes persistent "access forbidden" errors after the fix.

**Steps**:

1. **Reproduce Pre-Fix Behavior** (for comparison - requires reverting to old code)

   With old code, setting invalid env var would cause:
   ```
   time="..." level=error msg="LAPI authentication failed" error="access forbidden (403)" key="[REDACTED]"
   ```

2. **Apply Fix and Repeat Scenario 1**

   With new code, same invalid env var should produce:
   ```
   time="..." level=warning msg="Environment variable CHARON_SECURITY_CROWDSEC_API_KEY is set but invalid..."
   time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="abcd...wxyz" source=file
   ```

**Success Criteria**:
- ✅ No "access forbidden" errors after auto-recovery
- ✅ Bouncer connects successfully with auto-generated key

---

### Scenario 4: Key Source Visibility in Logs

**Objective**: Verify that logs clearly indicate which key source is used (environment variable vs file vs auto-generated).

**Test Cases**:

#### 4a. Valid Environment Variable

```bash
# Set valid key in env
export CHARON_SECURITY_CROWDSEC_API_KEY=<valid_key_from_cscli>
docker restart charon
```

**Expected Log**:
```
time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="vali...test" source=environment_variable
```

#### 4b. File-Based Key

```bash
# Clear env var, restart with existing file
unset CHARON_SECURITY_CROWDSEC_API_KEY
docker restart charon
```

**Expected Log**:
```
time="..." level=info msg="CrowdSec bouncer authentication successful" masked_key="abcd...wxyz" source=file
```

#### 4c. Auto-Generated Key

```bash
# Clear env var and file, start fresh
docker exec charon rm -f /app/data/crowdsec/bouncer_key
docker restart charon
```

**Expected Log**:
```
time="..." level=info msg="Registering new CrowdSec bouncer: caddy-bouncer"
time="..." level=info msg="CrowdSec bouncer registration successful" masked_key="new-...123" source=auto_generated
```

**Success Criteria**:
- ✅ Logs clearly show `source=environment_variable`, `source=file`, or `source=auto_generated`
- ✅ User can determine which key is active without reading code

---

## Troubleshooting

### Issue: "failed to execute cscli" Errors

**Cause**: CrowdSec binary not installed in container

**Resolution**: Ensure CrowdSec is installed via Dockerfile or skip test if binary is intentionally excluded.

### Issue: LAPI Timeout After 30 Seconds

**Cause**: CrowdSec process failed to start or crashed

**Debug Steps**:
1. Check LAPI process: `docker exec charon ps aux | grep crowdsec`
2. Check LAPI logs: `docker exec charon cat /var/log/crowdsec/crowdsec.log`
3. Verify config: `docker exec charon cat /etc/crowdsec/config.yaml`

### Issue: "access forbidden" Despite New Key

**Cause**: Key not properly registered with LAPI

**Resolution**:
```bash
# List registered bouncers
docker exec charon cscli bouncers list

# If caddy-bouncer missing, re-register
docker exec charon cscli bouncers delete caddy-bouncer || true
docker restart charon
```

---

## Verification Checklist

Before considering the fix complete, verify all scenarios pass:

- [ ] **Scenario 1**: Invalid env var triggers auto-recovery
- [ ] **Scenario 2**: LAPI startup delay handled with retry logic
- [ ] **Scenario 3**: No "access forbidden" errors in production logs
- [ ] **Scenario 4a**: Env var source logged correctly
- [ ] **Scenario 4b**: File source logged correctly
- [ ] **Scenario 4c**: Auto-generated source logged correctly
- [ ] **Integration Tests**: All 3 tests in `backend/integration/crowdsec_lapi_integration_test.go` pass
- [ ] **Unit Tests**: All 10 tests in `backend/internal/api/handlers/crowdsec_handler_test.go` pass

---

## Additional Validation

### Docker Logs Monitoring (Real-Time)

```bash
# Watch logs in real-time for auth-related messages
docker logs -f charon 2>&1 | grep -iE "crowdsec|bouncer|lapi|authentication"
```

### LAPI Health Check

```bash
# Check if LAPI is responding
curl http://localhost:8080/v1/health
```

**Expected**: HTTP 200 OK

### Bouncer Registration Status

```bash
# Verify bouncer is registered via cscli
docker exec charon cscli bouncers list

# Expected output should include:
# Name             │ IP Address │ Valid │ Last API Key │ Last API Pull
# ─────────────────┼────────────┼───────┼──────────────┼───────────────
# caddy-bouncer    │            │ ✔️    │ <timestamp>  │ <timestamp>
```

---

## Notes for QA and Code Review

- **Backward Compatibility**: Old behavior (name-based validation) is preserved in `validateBouncerKey()` for backward compatibility. New authentication logic is in `testKeyAgainstLAPI()`.
- **Security**: API keys are masked in logs (first 4 + last 4 chars only) to prevent exposure via CWE-312.
- **File Permissions**: Bouncer key file created with 0600 permissions (read/write owner only), directory with 0700.
- **Atomic Writes**: `saveKeyToFile()` uses temp file + rename pattern to prevent corruption.
- **Retry Logic**: Connection refused errors trigger exponential backoff (500ms → 750ms → 1125ms → ..., capped at 5s per attempt, 30s total).
- **Fast Fail**: 403 Forbidden errors fail immediately without retries (indicates invalid key, not LAPI startup issue).

---

## Related Documentation

- **Investigation Report**: `docs/issues/crowdsec_auth_regression.md`
- **Unit Tests**: `backend/internal/api/handlers/crowdsec_handler_test.go` (lines 3970-4294)
- **Integration Tests**: `backend/integration/crowdsec_lapi_integration_test.go`
- **Implementation**: `backend/internal/api/handlers/crowdsec_handler.go` (lines 1548-1720)
