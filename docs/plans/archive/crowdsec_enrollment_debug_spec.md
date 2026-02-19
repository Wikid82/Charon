# CrowdSec Enrollment & Console Connectivity Debug Plan

**Issue Reference:** #586
**Problem:** CrowdSec engine showing as offline since 12/19/25 in web console
**Date Created:** 2026-02-03
**Status:** Research Complete - Ready for Implementation

---

## Executive Summary

This document provides a comprehensive debugging and testing strategy for diagnosing and resolving CrowdSec console enrollment and connectivity issues. The issue manifests as the CrowdSec engine appearing offline in the crowdsec.net web console despite being enrolled locally.

### Key Findings from Research

**Architecture Components Identified:**
1. **CrowdSec Handler** (`backend/internal/api/handlers/crowdsec_handler.go`) - Manages lifecycle, enrollment, status
2. **Console Enrollment Service** (`backend/internal/crowdsec/console_enroll.go`) - Handles enrollment with retry logic
3. **Startup Service** (`backend/internal/services/crowdsec_startup.go`) - Auto-starts CrowdSec on container boot
4. **Docker Entrypoint** (`.docker/docker-entrypoint.sh`) - Initializes CrowdSec configuration
5. **Database Model** (`backend/internal/models/crowdsec_console_enrollment.go`) - Stores enrollment state
6. **LAPI** - Runs on port 8085, health checks via `cscli lapi status`
7. **Feature Flag** - `feature.crowdsec.console_enrollment` controls console enrollment UI visibility

**Current Test Coverage:**
- ✅ Integration tests for CrowdSec decisions (`backend/integration/crowdsec_decisions_integration_test.go`)
- ✅ Integration tests for CrowdSec startup (`backend/integration/crowdsec_integration_test.go`)
- ✅ E2E tests for CrowdSec configuration page (`tests/security/crowdsec-config.spec.ts`)
- ✅ Unit tests for startup service (`backend/internal/services/crowdsec_startup_test.go`)
- ❌ **No E2E tests for console enrollment**
- ❌ **No integration tests for LAPI heartbeat/connectivity**
- ❌ **No tests for enrollment token validation**
- ❌ **No tests for console status polling**

---

## Problem Analysis

### Symptom: Engine Offline in Console

When CrowdSec shows as "offline" in the crowdsec.net console, it indicates one or more of the following:

1. **LAPI Not Running** - The Local API process is not active
2. **Enrollment Not Completed** - Token accepted locally but not on crowdsec.net
3. **Heartbeat Failure** - LAPI running but not sending heartbeats to console
4. **Network Connectivity** - Container cannot reach crowdsec.net APIs
5. **Token Expiry** - Enrollment token expired or revoked
6. **CAPI Not Registered** - Central API credentials missing or invalid
7. **Config Corruption** - Missing or corrupt `online_api_credentials.yaml`

### Known Failure Points

#### 1. Enrollment Token Handling
**Location:** `backend/internal/crowdsec/console_enroll.go:124-149`

**Current Implementation:**
```go
token, err := normalizeEnrollmentKey(req.EnrollmentKey)
if err != nil {
    return ConsoleEnrollmentStatus{}, err
}
```

**Issues:**
- Token validation only checks format (alphanumeric, 10-64 chars)
- No check for token expiry before attempting enrollment
- Token is encrypted and stored but never re-validated
- No explicit error for expired tokens (generic failure message)

**Impact:** Users may submit valid-format tokens that are already expired, leading to silent enrollment failures.

#### 2. LAPI Connectivity
**Location:** `backend/internal/crowdsec/console_enroll.go:218-246`

**Current Implementation:**
```go
func (s *ConsoleEnrollmentService) checkLAPIAvailable(ctx context.Context) error {
    maxRetries := 3
    retryDelay := 2 * time.Second

    for i := 0; i < maxRetries; i++ {
        args := []string{"lapi", "status"}
        // ... execute cscli command
        if err == nil {
            return nil // LAPI is available
        }
        time.Sleep(retryDelay)
    }

    return fmt.Errorf("CrowdSec Local API is not running after %d attempts", maxRetries)
}
```

**Issues:**
- Only 3 retries with 2-second delays (6 seconds total)
- LAPI initialization can take 10-15 seconds on slow hardware
- No exponential backoff (fixed 2s delay)
- Timeout per attempt is 3 seconds (may be insufficient for cold start)
- No check for LAPI process vs LAPI readiness

**Impact:** LAPI may still be initializing when enrollment check fails, causing false negatives.

#### 3. Network Configuration
**Location:** `.docker/docker-entrypoint.sh:212-219`, `docker-compose.yml:47-49`

**Current Implementation:**
```yaml
services:
  charon:
    image: wikid82/charon:latest
    container_name: charon
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"
    # LAPI port 8085 NOT exposed by default
```

**Issues:**
- LAPI listens on `127.0.0.1:8085` (localhost only)
- No port mapping for LAPI (intentional for security)
- Console heartbeats must originate from within container
- Network mode defaults to bridge (may block outbound HTTPS to crowdsec.net)
- No explicit DNS resolution configuration

**Impact:** Enrollment may succeed locally but heartbeats fail due to network restrictions.

#### 4. CAPI Registration
**Location:** `backend/internal/crowdsec/console_enroll.go:248-267`

**Current Implementation:**
```go
func (s *ConsoleEnrollmentService) ensureCAPIRegistered(ctx context.Context) error {
    credsPath := filepath.Join(s.dataDir, "config", "online_api_credentials.yaml")
    if _, err := os.Stat(credsPath); err == nil {
        return nil // Assume registered if file exists
    }

    // Register with CAPI
    args := []string{"capi", "register"}
    out, err := s.exec.ExecuteWithEnv(ctx, "cscli", args, nil)
    if err != nil {
        return fmt.Errorf("capi register: %s: %w", string(out), err)
    }
    return nil
}
```

**Issues:**
- Only checks file existence, not validity
- No validation of credentials format
- No retry logic for CAPI registration failures
- No check for CAPI connectivity before registration
- Credentials may be corrupt or revoked

**Impact:** CAPI registration failures block console enrollment but error is generic.

#### 5. Config File Management
**Location:** `.docker/docker-entrypoint.sh:117-156`

**Current Implementation:**
```bash
# Initialize CrowdSec configuration
if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
    echo "Initializing persistent CrowdSec configuration..."
    if [ -d "/etc/crowdsec.dist" ]; then
        cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/"
    fi
fi

# Configure LAPI port
sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
```

**Issues:**
- Config only initialized on first run (not validated on restarts)
- Port replacement uses `sed` (brittle if config format changes)
- No validation of `config.yaml` syntax
- No validation of `acquis.yaml` (required for datasources)
- `online_api_credentials.yaml` assumed valid if present

**Impact:** Config corruption after first run is not detected until CrowdSec fails to start.

#### 6. Enrollment Status Management
**Location:** `backend/internal/models/crowdsec_console_enrollment.go:7-21`

**Current Implementation:**
```go
type CrowdsecConsoleEnrollment struct {
    UUID               string     `json:"uuid"`
    Status             string     `json:"status"` // not_enrolled, enrolling, pending_acceptance, enrolled, failed
    Tenant             string     `json:"tenant"`
    AgentName          string     `json:"agent_name"`
    EncryptedEnrollKey string     `json:"-"`
    LastError          string     `json:"last_error"`
    LastAttemptAt      *time.Time `json:"last_attempt_at"`
    EnrolledAt         *time.Time `json:"enrolled_at"`
    LastHeartbeatAt    *time.Time `json:"last_heartbeat_at"` // NOT USED
    // ...
}
```

**Issues:**
- `LastHeartbeatAt` field exists but is never updated
- No automatic polling of console status
- Status is set to `pending_acceptance` after local enrollment
- No mechanism to detect when user accepts enrollment on crowdsec.net
- No mechanism to detect when engine goes offline
- Status remains `pending_acceptance` indefinitely unless user manually checks

**Impact:** Database shows "pending" but user has already accepted on console (status never updates).

#### 7. Health Check Limitations
**Location:** `backend/internal/api/handlers/crowdsec_handler.go:327-367`

**Current Implementation:**
```go
func (h *CrowdsecHandler) Status(c *gin.Context) {
    running, pid, err := h.Executor.Status(ctx, h.DataDir)
    lapiReady := false
    if running {
        args := []string{"lapi", "status"}
        _, checkErr := h.CmdExec.Execute(checkCtx, "cscli", args...)
        lapiReady = (checkErr == nil)
    }
    c.JSON(http.StatusOK, gin.H{
        "running": running,
        "pid": pid,
        "lapi_ready": lapiReady,
    })
}
```

**Issues:**
- Only checks if LAPI responds to `cscli lapi status`
- Does not check if LAPI can reach crowdsec.net
- Does not verify CAPI credentials are valid
- Does not check if console enrollment is active
- No check for heartbeat status

**Impact:** Status endpoint shows "running" but doesn't detect console connectivity issues.

---

## Root Cause Analysis Framework

### Diagnostic Decision Tree

```
CrowdSec shows as offline in console
    │
    ├─> Is CrowdSec process running?
    │   └─> NO → Check startup logs, fix daemon start
    │   └─> YES ↓
    │
    ├─> Is LAPI responding (`cscli lapi status`)?
    │   └─> NO → Check LAPI initialization logs, increase startup wait time
    │   └─> YES ↓
    │
    ├─> Is CAPI registered (`online_api_credentials.yaml` exists)?
    │   └─> NO → Run `cscli capi register`, check network connectivity
    │   └─> YES ↓
    │
    ├─> Is console enrolled (`cscli console status` shows enrolled)?
    │   └─> NO → Check enrollment token validity, re-enroll
    │   └─> YES ↓
    │
    ├─> Can container reach crowdsec.net?
    │   └─> NO → Check DNS, firewall, proxy settings
    │   └─> YES ↓
    │
    ├─> Are heartbeats being sent?
    │   └─> NO → Check LAPI logs for heartbeat failures
    │   └─> YES ↓
    │
    └─> Console shows agent offline?
        └─> Check crowdsec.net for enrollment acceptance
        └─> Check for token expiry/revocation
        └─> Contact CrowdSec support
```

### Investigation Checklist

#### Phase 1: Local Process Verification
- [ ] Verify CrowdSec process is running: `docker exec charon ps aux | grep crowdsec`
- [ ] Check LAPI is listening: `docker exec charon ss -tlnp | grep 8085`
- [ ] Verify LAPI responds: `docker exec charon cscli lapi status`
- [ ] Check CrowdSec version: `docker exec charon cscli version`
- [ ] Review CrowdSec logs: `docker exec charon tail -100 /var/log/crowdsec/crowdsec.log`

#### Phase 2: Configuration Validation
- [ ] Verify `config.yaml` exists: `docker exec charon test -f /etc/crowdsec/config.yaml`
- [ ] Validate `config.yaml` syntax: `docker exec charon cscli config check`
- [ ] Check LAPI port: `docker exec charon grep listen_uri /etc/crowdsec/config.yaml`
- [ ] Verify `acquis.yaml` exists: `docker exec charon test -f /etc/crowdsec/acquis.yaml`
- [ ] Check datasource config: `docker exec charon cat /etc/crowdsec/acquis.yaml`
- [ ] Validate CAPI credentials: `docker exec charon test -f /etc/crowdsec/config/online_api_credentials.yaml`
- [ ] Check machines list: `docker exec charon cscli machines list`

#### Phase 3: Enrollment State Verification
- [ ] Check enrollment status in DB:
  ```sql
  docker exec charon sqlite3 /app/data/charon.db "SELECT * FROM crowdsec_console_enrollments;"
  ```
- [ ] Verify console enrollment status: `docker exec charon cscli console status`
- [ ] Check feature flag: Query `/api/v1/settings?key=feature.crowdsec.console_enrollment`
- [ ] Review enrollment logs in Charon: Search logs for "crowdsec console enrollment"
- [ ] Check last enrollment attempt timestamp in DB

#### Phase 4: Network Connectivity
- [ ] Test DNS resolution: `docker exec charon nslookup crowdsec.net`
- [ ] Test HTTPS connectivity: `docker exec charon curl -I https://api.crowdsec.net/health`
- [ ] Check container network mode: `docker inspect charon | grep NetworkMode`
- [ ] Verify outbound firewall rules: Check host firewall, corporate proxy
- [ ] Test CAPI connectivity: `docker exec charon cscli capi status`

#### Phase 5: Console Status
- [ ] Check crowdsec.net console for agent enrollment
- [ ] Verify enrollment was accepted on crowdsec.net
- [ ] Check agent last seen timestamp in console
- [ ] Review console activity logs for heartbeat failures
- [ ] Verify enrollment token hasn't expired

---

## Testing Strategy

### Phase 1: Unit Tests for Enrollment Logic

**Objective:** Validate enrollment service behavior in isolation

**Test File:** `backend/internal/crowdsec/console_enroll_test.go` (NEW)

**Test Cases:**

#### 1.1 Token Validation
```go
func TestConsoleEnrollmentService_TokenValidation(t *testing.T) {
    tests := []struct {
        name    string
        token   string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "Valid token",
            token:   "abc123xyz789",
            wantErr: false,
        },
        {
            name:    "Token too short",
            token:   "abc123",
            wantErr: true,
            errMsg:  "invalid enrollment key",
        },
        {
            name:    "Token with special chars",
            token:   "abc-123_xyz",
            wantErr: true,
            errMsg:  "invalid enrollment key",
        },
        {
            name:    "Empty token",
            token:   "",
            wantErr: true,
            errMsg:  "enrollment_key required",
        },
        {
            name:    "Token from cscli command",
            token:   "sudo cscli console enroll abc123xyz789",
            wantErr: false, // Should extract token
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test normalizeEnrollmentKey function
            token, err := normalizeEnrollmentKey(tt.token)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
                assert.NotEmpty(t, token)
            }
        })
    }
}
```

#### 1.2 LAPI Availability Check with Retries
```go
func TestConsoleEnrollmentService_CheckLAPIAvailable(t *testing.T) {
    tests := []struct {
        name          string
        execResponses []execResponse // Mock responses for each retry
        wantErr       bool
        errMsg        string
    }{
        {
            name: "LAPI available on first try",
            execResponses: []execResponse{
                {output: "", err: nil},
            },
            wantErr: false,
        },
        {
            name: "LAPI available on second try",
            execResponses: []execResponse{
                {output: "connection refused", err: fmt.Errorf("exit status 1")},
                {output: "", err: nil},
            },
            wantErr: false,
        },
        {
            name: "LAPI never becomes available",
            execResponses: []execResponse{
                {output: "connection refused", err: fmt.Errorf("exit status 1")},
                {output: "connection refused", err: fmt.Errorf("exit status 1")},
                {output: "connection refused", err: fmt.Errorf("exit status 1")},
            },
            wantErr: true,
            errMsg:  "Local API is not running after 3 attempts",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create mock executor that returns responses in sequence
            mockExec := &mockEnvCommandExecutor{
                responses: tt.execResponses,
            }

            svc := &ConsoleEnrollmentService{
                exec:    mockExec,
                dataDir: "/tmp/test",
            }

            err := svc.checkLAPIAvailable(context.Background())
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

#### 1.3 CAPI Registration
```go
func TestConsoleEnrollmentService_EnsureCAPIRegistered(t *testing.T) {
    tests := []struct {
        name          string
        credsExist    bool
        registerErr   error
        wantErr       bool
        wantRegister  bool
    }{
        {
            name:         "CAPI already registered",
            credsExist:   true,
            wantErr:      false,
            wantRegister: false,
        },
        {
            name:         "CAPI not registered, success",
            credsExist:   false,
            registerErr:  nil,
            wantErr:      false,
            wantRegister: true,
        },
        {
            name:         "CAPI registration fails",
            credsExist:   false,
            registerErr:  fmt.Errorf("network error"),
            wantErr:      true,
            wantRegister: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test CAPI registration logic
        })
    }
}
```

#### 1.4 Enrollment Status Transitions
```go
func TestConsoleEnrollmentService_StatusTransitions(t *testing.T) {
    // Test: not_enrolled → enrolling → pending_acceptance
    // Test: pending_acceptance → enrolled (manual transition)
    // Test: enrolling → failed (on error)
    // Test: failed → enrolling (retry)
}
```

#### 1.5 Concurrent Enrollment Prevention
```go
func TestConsoleEnrollmentService_ConcurrentEnrollment(t *testing.T) {
    // Test: Multiple simultaneous enrollment attempts should be blocked
    // Test: Mutex prevents race conditions
}
```

#### 1.6 Token Encryption/Decryption
```go
func TestConsoleEnrollmentService_TokenEncryption(t *testing.T) {
    // Test: Token is encrypted before storage
    // Test: Decryption works correctly
    // Test: Encryption key derivation
}
```

### Phase 2: Integration Tests for LAPI Connectivity

**Objective:** Verify LAPI health checks and connectivity in real environment

**Test File:** `backend/integration/crowdsec_lapi_integration_test.go` (NEW)

**Test Cases:**

#### 2.1 LAPI Startup and Health
```go
//go:build integration
func TestCrowdSecLAPIStartup(t *testing.T) {
    // 1. Start CrowdSec via API: POST /api/v1/admin/crowdsec/start
    // 2. Wait for LAPI to initialize (up to 30s)
    // 3. Verify: GET /api/v1/admin/crowdsec/status returns lapi_ready: true
    // 4. Verify: docker exec cscli lapi status returns 0
    // 5. Verify: LAPI health endpoint responds: curl http://localhost:8085/health
}
```

#### 2.2 LAPI Readiness After Restart
```go
//go:build integration
func TestCrowdSecLAPIRestartPersistence(t *testing.T) {
    // 1. Enroll CrowdSec
    // 2. Stop CrowdSec
    // 3. Restart container
    // 4. Verify LAPI comes back online
    // 5. Verify enrollment status persists
}
```

#### 2.3 LAPI Port Configuration
```go
//go:build integration
func TestCrowdSecLAPIPortConfiguration(t *testing.T) {
    // 1. Verify LAPI listens on 8085 (not 8080)
    // 2. Verify Charon can reach LAPI at 127.0.0.1:8085
    // 3. Verify LAPI is NOT exposed to host
}
```

#### 2.4 CAPI Connectivity
```go
//go:build integration
func TestCrowdSecCAPIConnectivity(t *testing.T) {
    // 1. Verify CAPI can be reached from container
    // 2. Verify CAPI registration succeeds
    // 3. Verify online_api_credentials.yaml is created
    // 4. Verify credentials are valid (cscli capi status)
}
```

### Phase 3: E2E Tests for Console Enrollment

**Objective:** Verify complete enrollment flow from UI

**Test File:** `tests/security/crowdsec-console-enrollment.spec.ts` (NEW)

**Test Cases:**

#### 3.1 Enrollment Flow (Happy Path)
```typescript
test('should complete console enrollment successfully', async ({ page, request }) => {
  // Prerequisite: Ensure CrowdSec is enabled
  await enableCrowdSec(request);

  // Step 1: Navigate to CrowdSec configuration
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);

  // Step 2: Verify console enrollment section is visible
  const enrollmentSection = page.getByTestId('console-enrollment-section');
  await expect(enrollmentSection).toBeVisible();

  // Step 3: Enter enrollment token
  const tokenInput = page.getByTestId('enrollment-token-input');
  await tokenInput.fill(process.env.TEST_CROWDSEC_ENROLLMENT_TOKEN || 'test-token-123');

  // Step 4: Enter agent name
  const agentNameInput = page.getByTestId('agent-name-input');
  await agentNameInput.fill('test-agent-e2e');

  // Step 5: Submit enrollment
  const enrollButton = page.getByRole('button', { name: /enroll/i });
  await enrollButton.click();

  // Step 6: Wait for enrollment request to complete
  const enrollResponse = await page.waitForResponse(
    resp => resp.url().includes('/api/v1/admin/crowdsec/console/enrollment') && resp.request().method() === 'POST'
  );
  expect(enrollResponse.ok()).toBeTruthy();

  // Step 7: Verify status changes to pending_acceptance
  await expect(page.getByText(/pending acceptance/i)).toBeVisible({ timeout: 10000 });

  // Step 8: Verify enrollment record was created
  const statusResponse = await request.get('/api/v1/admin/crowdsec/console/enrollment');
  expect(statusResponse.ok()).toBeTruthy();
  const status = await statusResponse.json();
  expect(status.status).toBe('pending_acceptance');
  expect(status.agent_name).toBe('test-agent-e2e');
});
```

#### 3.2 Enrollment Validation Errors
```typescript
test('should show validation errors for invalid enrollment data', async ({ page }) => {
  await page.goto('/security/crowdsec');

  // Test: Empty token
  await page.getByRole('button', { name: /enroll/i }).click();
  await expect(page.getByText(/enrollment.*required/i)).toBeVisible();

  // Test: Invalid token format
  await page.getByTestId('enrollment-token-input').fill('invalid@token!');
  await page.getByRole('button', { name: /enroll/i }).click();
  await expect(page.getByText(/invalid.*enrollment/i)).toBeVisible();

  // Test: Empty agent name
  await page.getByTestId('enrollment-token-input').fill('validtoken123');
  await page.getByRole('button', { name: /enroll/i }).click();
  await expect(page.getByText(/agent.*required/i)).toBeVisible();
});
```

#### 3.3 Enrollment When LAPI Not Running
```typescript
test('should show error when LAPI is not running', async ({ page, request }) => {
  // Prerequisite: Stop CrowdSec
  await request.post('/api/v1/admin/crowdsec/stop');
  await page.waitForTimeout(2000);

  // Attempt enrollment
  await page.goto('/security/crowdsec');
  const tokenInput = page.getByTestId('enrollment-token-input');
  await tokenInput.fill('validtoken123');
  await page.getByRole('button', { name: /enroll/i }).click();

  // Verify error message
  await expect(page.getByText(/Local API is not running/i)).toBeVisible({ timeout: 10000 });
});
```

#### 3.4 Re-enrollment (Force)
```typescript
test('should allow re-enrollment with force flag', async ({ page, request }) => {
  // Prerequisite: Complete initial enrollment
  // ... (same as happy path)

  // Attempt enrollment again (should fail without force)
  await page.getByTestId('enrollment-token-input').fill('newtoken456');
  await page.getByRole('button', { name: /enroll/i }).click();
  await expect(page.getByText(/already enrolled/i)).toBeVisible();

  // Enable force re-enrollment
  await page.getByTestId('force-reenroll-checkbox').click();
  await page.getByRole('button', { name: /enroll/i }).click();

  // Verify re-enrollment succeeds
  await waitForToast(page, /enrollment.*sent/i);
});
```

#### 3.5 Enrollment Status Display
```typescript
test('should display current enrollment status correctly', async ({ page, request }) => {
  // Test: Not enrolled
  await page.goto('/security/crowdsec');
  await expect(page.getByText(/not enrolled/i)).toBeVisible();

  // Test: Enrolling (in progress)
  // (Mock or trigger enrollment)

  // Test: Pending acceptance
  // (Mock pending state)
  await expect(page.getByText(/pending acceptance/i)).toBeVisible();
  await expect(page.getByText(/accept.*crowdsec\.net/i)).toBeVisible();

  // Test: Enrolled
  // (Mock enrolled state)
  await expect(page.getByText(/enrolled/i)).toBeVisible();
  await expect(page.getByTestId('enrollment-success-badge')).toBeVisible();
});
```

#### 3.6 Clear Enrollment State
```typescript
test('should clear enrollment state to allow fresh enrollment', async ({ page, request }) => {
  // Prerequisite: Complete enrollment
  // ...

  // Click "Clear Enrollment" button
  const clearButton = page.getByRole('button', { name: /clear.*enrollment/i });
  await clearButton.click();

  // Confirm in dialog
  await page.getByRole('button', { name: /confirm/i }).click();

  // Wait for DELETE request
  const deleteResponse = await page.waitForResponse(
    resp => resp.url().includes('/api/v1/admin/crowdsec/console/enrollment') && resp.request().method() === 'DELETE'
  );
  expect(deleteResponse.ok()).toBeTruthy();

  // Verify status resets to not_enrolled
  await expect(page.getByText(/not enrolled/i)).toBeVisible();
});
```

### Phase 4: E2E Tests for Console Status Monitoring

**Objective:** Verify console connectivity monitoring and heartbeat tracking

**Test File:** `tests/security/crowdsec-console-monitoring.spec.ts` (NEW)

**Test Cases:**

#### 4.1 Console Status Endpoint
```typescript
test('should fetch console enrollment status', async ({ request }) => {
  const response = await request.get('/api/v1/admin/crowdsec/console/enrollment');
  expect(response.ok()).toBeTruthy();

  const status = await response.json();
  expect(status).toHaveProperty('status');
  expect(status).toHaveProperty('agent_name');
  expect(status).toHaveProperty('tenant');
  expect(status).toHaveProperty('last_attempt_at');
  expect(status).toHaveProperty('key_present');
});
```

#### 4.2 Heartbeat Tracking (Future Enhancement)
```typescript
test.skip('should track console heartbeats', async ({ request }) => {
  // NOTE: This test is skipped because LastHeartbeatAt is not currently implemented
  // Once implemented, this test should:
  // 1. Enroll with console
  // 2. Wait for heartbeat to be sent (typically every 10s)
  // 3. Verify last_heartbeat_at is updated in database
  // 4. Verify status endpoint returns heartbeat timestamp
});
```

### Phase 5: Diagnostic and Monitoring Tests

**Objective:** Verify diagnostic endpoints and log collection

**Test File:** `tests/security/crowdsec-diagnostics.spec.ts` (NEW)

**Test Cases:**

#### 5.1 Config File Validation
```typescript
test('should validate CrowdSec configuration files', async ({ request }) => {
  // GET /api/v1/admin/crowdsec/files (list config files)
  const filesResponse = await request.get('/api/v1/admin/crowdsec/files');
  expect(filesResponse.ok()).toBeTruthy();

  const files = await filesResponse.json();
  expect(files.files).toContain('config/config.yaml');
  expect(files.files).toContain('config/acquis.yaml');

  // GET /api/v1/admin/crowdsec/files?path=config/config.yaml
  const configResponse = await request.get('/api/v1/admin/crowdsec/files?path=config/config.yaml');
  expect(configResponse.ok()).toBeTruthy();

  const config = await configResponse.json();
  expect(config.content).toContain('listen_uri: 127.0.0.1:8085');
});
```

#### 5.2 LAPI Health Endpoint
```typescript
test('should verify LAPI health endpoint responds', async ({ request }) => {
  // Prerequisite: Ensure CrowdSec is running
  await enableCrowdSec(request);
  await waitForLAPIReady(request, 30000);

  // Test LAPI health endpoint directly
  // NOTE: This requires port 8085 to be exposed or curl from within container
  const statusResponse = await request.get('/api/v1/admin/crowdsec/status');
  expect(statusResponse.ok()).toBeTruthy();

  const status = await statusResponse.json();
  expect(status.lapi_ready).toBe(true);
});
```

#### 5.3 Export Configuration
```typescript
test('should export CrowdSec configuration', async ({ request }) => {
  const response = await request.get('/api/v1/admin/crowdsec/export');
  expect(response.ok()).toBeTruthy();

  // Verify response is tar.gz file
  const contentType = response.headers()['content-type'];
  expect(contentType).toContain('application/gzip');

  const contentDisposition = response.headers()['content-disposition'];
  expect(contentDisposition).toMatch(/attachment.*crowdsec-config.*\.tar\.gz/);
});
```

---

## Implementation Plan

### Phase 1: Diagnostic Tools (Week 1)

**Goal:** Implement comprehensive diagnostic endpoints to aid troubleshooting

#### 1.1 Add Console Connectivity Check Endpoint
**File:** `backend/internal/api/handlers/crowdsec_handler.go`

```go
// ConsoleConnectivityCheck verifies connectivity to crowdsec.net APIs
func (h *CrowdsecHandler) ConsoleConnectivityCheck(c *gin.Context) {
    ctx := c.Request.Context()

    checks := map[string]interface{}{
        "lapi_running": false,
        "capi_registered": false,
        "console_enrolled": false,
        "capi_reachable": false,
        "console_reachable": false,
    }

    // Check 1: LAPI running
    running, _, _ := h.Executor.Status(ctx, h.DataDir)
    checks["lapi_running"] = running

    // Check 2: LAPI health
    if running {
        args := []string{"lapi", "status"}
        _, err := h.CmdExec.Execute(ctx, "cscli", args...)
        checks["lapi_ready"] = (err == nil)
    }

    // Check 3: CAPI registered
    credsPath := filepath.Join(h.DataDir, "config", "online_api_credentials.yaml")
    checks["capi_registered"] = fileExists(credsPath)

    // Check 4: CAPI reachable
    if checks["capi_registered"].(bool) {
        args := []string{"capi", "status"}
        out, err := h.CmdExec.Execute(ctx, "cscli", args...)
        checks["capi_reachable"] = (err == nil)
        checks["capi_status_output"] = string(out)
    }

    // Check 5: Console enrolled
    if h.Console != nil {
        status, err := h.Console.Status(ctx)
        if err == nil {
            checks["console_enrolled"] = (status.Status == "enrolled" || status.Status == "pending_acceptance")
            checks["console_status"] = status
        }
    }

    // Check 6: Console API reachable (ping crowdsec.net)
    consoleURL := "https://api.crowdsec.net/health"
    resp, err := http.Get(consoleURL)
    if err == nil {
        defer resp.Body.Close()
        checks["console_reachable"] = (resp.StatusCode == 200)
    } else {
        checks["console_reachable"] = false
        checks["console_error"] = err.Error()
    }

    c.JSON(http.StatusOK, checks)
}
```

**Route:** `GET /api/v1/admin/crowdsec/diagnostics/connectivity`

#### 1.2 Add Config Validation Endpoint
**File:** `backend/internal/api/handlers/crowdsec_handler.go`

```go
// ValidateConfig checks CrowdSec configuration files for common issues
func (h *CrowdsecHandler) ValidateConfig(c *gin.Context) {
    ctx := c.Request.Context()

    validation := map[string]interface{}{
        "config_exists": false,
        "config_valid": false,
        "acquis_exists": false,
        "acquis_valid": false,
        "lapi_port": "",
        "errors": []string{},
    }

    // Check config.yaml
    configPath := filepath.Join(h.DataDir, "config", "config.yaml")
    if fileExists(configPath) {
        validation["config_exists"] = true

        // Read config and check LAPI port
        content, err := os.ReadFile(configPath)
        if err == nil {
            configStr := string(content)
            if strings.Contains(configStr, "listen_uri") {
                // Extract port
                re := regexp.MustCompile(`listen_uri:\s*127\.0\.0\.1:(\d+)`)
                matches := re.FindStringSubmatch(configStr)
                if len(matches) > 1 {
                    validation["lapi_port"] = matches[1]
                }
            }
        }

        // Validate using cscli
        args := []string{"-c", configPath, "config", "check"}
        out, err := h.CmdExec.Execute(ctx, "cscli", args...)
        if err == nil {
            validation["config_valid"] = true
        } else {
            validation["config_valid"] = false
            validation["errors"] = append(validation["errors"].([]string), string(out))
        }
    }

    // Check acquis.yaml
    acquisPath := filepath.Join(h.DataDir, "config", "acquis.yaml")
    if fileExists(acquisPath) {
        validation["acquis_exists"] = true

        // Check if it has datasources
        content, err := os.ReadFile(acquisPath)
        if err == nil {
            acquisStr := string(content)
            if strings.Contains(acquisStr, "source:") && strings.Contains(acquisStr, "filenames:") {
                validation["acquis_valid"] = true
            } else {
                validation["acquis_valid"] = false
                validation["errors"] = append(validation["errors"].([]string), "acquis.yaml missing datasource configuration")
            }
        }
    } else {
        validation["errors"] = append(validation["errors"].([]string), "acquis.yaml not found")
    }

    c.JSON(http.StatusOK, validation)
}
```

**Route:** `GET /api/v1/admin/crowdsec/diagnostics/config`

#### 1.3 Add Heartbeat Status Endpoint (Future)
**File:** `backend/internal/api/handlers/crowdsec_handler.go`

```go
// GetHeartbeatStatus returns console heartbeat status
// NOTE: This is a placeholder for future implementation
// Currently, LastHeartbeatAt is not tracked in the database
func (h *CrowdsecHandler) GetHeartbeatStatus(c *gin.Context) {
    if h.Console == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console service unavailable"})
        return
    }

    ctx := c.Request.Context()
    status, err := h.Console.Status(ctx)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // TODO: Implement heartbeat tracking
    // For now, return placeholder
    c.JSON(http.StatusOK, gin.H{
        "status": status.Status,
        "last_heartbeat_at": status.LastHeartbeatAt,
        "heartbeat_tracking_implemented": false,
        "note": "Heartbeat tracking not yet implemented",
    })
}
```

**Route:** `GET /api/v1/admin/crowdsec/console/heartbeat`

### Phase 2: Enhanced Enrollment Validation (Week 2)

**Goal:** Improve enrollment validation and error messaging

#### 2.1 Increase LAPI Check Retries
**File:** `backend/internal/crowdsec/console_enroll.go`

**Change:**
```go
// Before:
maxRetries := 3
retryDelay := 2 * time.Second

// After:
maxRetries := 5 // Increased from 3
retryDelay := 3 * time.Second // Increased from 2s
// Add exponential backoff
for i := 0; i < maxRetries; i++ {
    // ... existing code ...
    if i < maxRetries-1 {
        // Exponential backoff: 3s, 6s, 12s, 24s
        delay := time.Duration(retryDelay.Seconds() * math.Pow(2, float64(i))) * time.Second
        time.Sleep(delay)
    }
}
```

#### 2.2 Add Token Expiry Detection
**File:** `backend/internal/crowdsec/console_enroll.go`

**Add new function:**
```go
// checkTokenExpiry attempts to detect if enrollment token is expired
// Returns true if token appears expired, false otherwise
func (s *ConsoleEnrollmentService) checkTokenExpiry(token string) (bool, error) {
    // Tokens from crowdsec.net have a limited lifetime (typically 24-48 hours)
    // We can't validate this client-side without calling the API
    // Best we can do is check the error message from cscli

    // For now, this is a placeholder
    // Future: Could call crowdsec.net API to validate token before enrollment
    return false, nil
}
```

#### 2.3 Improve Error Messages
**File:** `backend/internal/crowdsec/console_enroll.go`

**Enhance `extractCscliErrorMessage`:**
```go
func extractCscliErrorMessage(output string) string {
    output = strings.TrimSpace(output)
    if output == "" {
        return ""
    }

    // Check for specific error patterns
    errorPatterns := map[string]string{
        "token is expired":           "Enrollment token has expired. Please generate a new token from crowdsec.net",
        "token is invalid":           "Enrollment token is invalid. Please verify the token from crowdsec.net",
        "agent is already enrolled":  "Agent is already enrolled. Use force=true to re-enroll",
        "LAPI is not reachable":      "Cannot reach Local API. Ensure CrowdSec is running",
        "CAPI is not reachable":      "Cannot reach Central API. Check network connectivity",
    }

    for pattern, message := range errorPatterns {
        if strings.Contains(strings.ToLower(output), pattern) {
            return message
        }
    }

    // Fall back to existing extraction logic
    // ...
}
```

### Phase 3: Heartbeat Monitoring Implementation (Week 3)

**Goal:** Implement console heartbeat tracking and status polling

#### 3.1 Add Heartbeat Polling Service
**File:** `backend/internal/crowdsec/heartbeat_poller.go` (NEW)

```go
package crowdsec

import (
    "context"
    "sync"
    "time"

    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/Wikid82/charon/backend/internal/models"
    "gorm.io/gorm"
)

type HeartbeatPoller struct {
    db       *gorm.DB
    exec     EnvCommandExecutor
    dataDir  string
    interval time.Duration
    stopCh   chan struct{}
    wg       sync.WaitGroup
}

func NewHeartbeatPoller(db *gorm.DB, exec EnvCommandExecutor, dataDir string) *HeartbeatPoller {
    return &HeartbeatPoller{
        db:       db,
        exec:     exec,
        dataDir:  dataDir,
        interval: 60 * time.Second, // Check every 60 seconds
        stopCh:   make(chan struct{}),
    }
}

func (p *HeartbeatPoller) Start() {
    p.wg.Add(1)
    go p.poll()
}

func (p *HeartbeatPoller) Stop() {
    close(p.stopCh)
    p.wg.Wait()
}

func (p *HeartbeatPoller) poll() {
    defer p.wg.Done()

    ticker := time.NewTicker(p.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            p.checkHeartbeat()
        case <-p.stopCh:
            return
        }
    }
}

func (p *HeartbeatPoller) checkHeartbeat() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Check if console is enrolled
    var enrollment models.CrowdsecConsoleEnrollment
    if err := p.db.WithContext(ctx).First(&enrollment).Error; err != nil {
        return // Not enrolled, skip
    }

    if enrollment.Status != "enrolled" && enrollment.Status != "pending_acceptance" {
        return // Not enrolled, skip
    }

    // Check console status via cscli
    args := []string{"console", "status"}
    configPath := filepath.Join(p.dataDir, "config", "config.yaml")
    if _, err := os.Stat(configPath); err == nil {
        args = append([]string{"-c", configPath}, args...)
    }

    out, err := p.exec.ExecuteWithEnv(ctx, "cscli", args, nil)
    if err != nil {
        logger.Log().WithError(err).WithField("output", string(out)).Warn("Failed to check console status")
        return
    }

    // Parse output to detect status
    output := string(out)
    now := time.Now().UTC()

    if strings.Contains(output, "enabled") && strings.Contains(output, "enrolled") {
        // Update heartbeat timestamp
        enrollment.LastHeartbeatAt = &now
        if enrollment.Status == "pending_acceptance" {
            // User has accepted enrollment on console
            enrollment.Status = "enrolled"
            enrollment.EnrolledAt = &now
        }
        if err := p.db.WithContext(ctx).Save(&enrollment).Error; err != nil {
            logger.Log().WithError(err).Warn("Failed to update heartbeat timestamp")
        } else {
            logger.Log().Debug("Console heartbeat updated")
        }
    }
}
```

#### 3.2 Integrate Heartbeat Poller in Main
**File:** `backend/cmd/api/main.go`

```go
// Initialize heartbeat poller
if consoleEnrollmentEnabled {
    heartbeatPoller := crowdsec.NewHeartbeatPoller(db, &crowdsec.SecureCommandExecutor{}, crowdsecDataDir)
    heartbeatPoller.Start()
    defer heartbeatPoller.Stop()
}
```

### Phase 4: Enhanced Logging and Monitoring (Week 3-4)

**Goal:** Improve observability for troubleshooting

#### 4.1 Add Structured Logging for Enrollment
**File:** `backend/internal/crowdsec/console_enroll.go`

**Enhance logging throughout enrollment process:**
```go
// Log at each critical step
logger.Log().WithFields(map[string]any{
    "correlation_id": rec.LastCorrelationID,
    "agent_name": agent,
    "tenant": tenant,
    "step": "lapi_check",
}).Info("Checking LAPI availability")

logger.Log().WithFields(map[string]any{
    "correlation_id": rec.LastCorrelationID,
    "step": "capi_registration",
}).Info("Ensuring CAPI registration")

logger.Log().WithFields(map[string]any{
    "correlation_id": rec.LastCorrelationID,
    "step": "enrollment_submit",
    "force": req.Force,
}).Info("Submitting enrollment request to CrowdSec console")
```

#### 4.2 Add Prometheus Metrics
**File:** `backend/internal/metrics/crowdsec_metrics.go` (NEW)

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    CrowdSecEnrollmentAttempts = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "charon_crowdsec_enrollment_attempts_total",
            Help: "Total number of console enrollment attempts",
        },
        []string{"status"}, // success, failed, pending
    )

    CrowdSecEnrollmentDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "charon_crowdsec_enrollment_duration_seconds",
            Help: "Duration of console enrollment attempts",
            Buckets: []float64{1, 5, 10, 30, 60},
        },
        []string{"status"},
    )

    CrowdSecHeartbeatLatency = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "charon_crowdsec_heartbeat_latency_seconds",
            Help: "Latency of console heartbeat checks",
            Buckets: []float64{0.1, 0.5, 1, 2, 5},
        },
    )

    CrowdSecLAPIHealth = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "charon_crowdsec_lapi_healthy",
            Help: "1 if LAPI is healthy, 0 otherwise",
        },
    )
)
```

**Integrate metrics in handlers:**
```go
func (h *CrowdsecHandler) ConsoleEnroll(c *gin.Context) {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        // Record metrics based on result
    }()

    // ... enrollment logic ...

    metrics.CrowdSecEnrollmentAttempts.WithLabelValues("success").Inc()
    metrics.CrowdSecEnrollmentDuration.WithLabelValues("success").Observe(duration)
}
```

### Phase 5: Documentation and User Guidance (Week 4)

**Goal:** Provide clear troubleshooting documentation for users

#### 5.1 Update Cerberus Documentation
**File:** `docs/cerberus.md`

**Add new section:**
```markdown
### Troubleshooting Console Enrollment

#### Symptom: Enrollment shows "pending_acceptance" but I already accepted on crowdsec.net

**Cause:** Status polling not implemented (manual refresh required)

**Solution:**
1. Refresh the Charon UI page
2. Check `/api/v1/admin/crowdsec/console/enrollment` endpoint
3. If still pending, check `cscli console status` inside container:
   ```bash
   docker exec charon cscli console status
   ```

#### Symptom: "Local API is not running after X attempts"

**Cause:** LAPI is still initializing or failed to start

**Solution:**
1. Check CrowdSec process:
   ```bash
   docker exec charon ps aux | grep crowdsec
   ```
2. Check LAPI logs:
   ```bash
   docker exec charon tail -100 /var/log/crowdsec/crowdsec.log
   ```
3. Verify LAPI config:
   ```bash
   docker exec charon grep listen_uri /etc/crowdsec/config.yaml
   ```
4. Test LAPI manually:
   ```bash
   docker exec charon cscli lapi status
   ```

#### Symptom: Enrollment fails with "CAPI is not reachable"

**Cause:** Network connectivity issue or CAPI credentials invalid

**Solution:**
1. Test connectivity to crowdsec.net:
   ```bash
   docker exec charon curl -I https://api.crowdsec.net/health
   ```
2. Check CAPI credentials:
   ```bash
   docker exec charon test -f /etc/crowdsec/config/online_api_credentials.yaml
   ```
3. Re-register with CAPI:
   ```bash
   docker exec charon cscli capi register
   ```

#### Symptom: Engine shows offline in console despite successful enrollment

**Possible Causes:**
1. Heartbeats not being sent
2. Network/firewall blocking outbound HTTPS
3. Token expired or revoked
4. LAPI process crashed after enrollment

**Solution:**
1. Verify LAPI is running:
   ```bash
   docker exec charon ps aux | grep crowdsec
   ```
2. Check console status:
   ```bash
   docker exec charon cscli console status
   ```
3. Check CrowdSec logs for heartbeat errors:
   ```bash
   docker exec charon tail -100 /var/log/crowdsec/crowdsec.log | grep -i heartbeat
   ```
4. Test outbound HTTPS:
   ```bash
   docker exec charon curl -v https://api.crowdsec.net/health
   ```
5. Check crowdsec.net console for agent last seen timestamp
```

#### 5.2 Add Diagnostic Script
**File:** `scripts/diagnose-crowdsec.sh` (NEW)

```bash
#!/bin/bash
set -e

echo "CrowdSec Console Enrollment Diagnostic Script"
echo "=============================================="
echo ""

# Check 1: CrowdSec process
echo "[1/8] Checking CrowdSec process..."
if docker exec charon ps aux | grep -q '[c]rowdsec'; then
    echo "✓ CrowdSec process is running"
    CROWDSEC_PID=$(docker exec charon ps aux | grep '[c]rowdsec' | awk '{print $2}')
    echo "  PID: $CROWDSEC_PID"
else
    echo "✗ CrowdSec process is NOT running"
    echo "  Run: docker exec charon charon crowdsec start"
    exit 1
fi

# Check 2: LAPI health
echo "[2/8] Checking LAPI health..."
if docker exec charon cscli lapi status >/dev/null 2>&1; then
    echo "✓ LAPI is responding"
else
    echo "✗ LAPI is NOT responding"
    echo "  Check logs: docker exec charon tail -100 /var/log/crowdsec/crowdsec.log"
fi

# Check 3: LAPI port
echo "[3/8] Checking LAPI configuration..."
LAPI_PORT=$(docker exec charon grep listen_uri /etc/crowdsec/config.yaml | sed 's/.*:\([0-9]*\)/\1/')
if [ "$LAPI_PORT" = "8085" ]; then
    echo "✓ LAPI configured on port 8085"
else
    echo "✗ LAPI port is $LAPI_PORT (expected 8085)"
fi

# Check 4: CAPI registration
echo "[4/8] Checking CAPI registration..."
if docker exec charon test -f /etc/crowdsec/config/online_api_credentials.yaml; then
    echo "✓ CAPI credentials file exists"
    if docker exec charon cscli capi status >/dev/null 2>&1; then
        echo "✓ CAPI is reachable"
    else
        echo "✗ CAPI is NOT reachable (network issue or invalid credentials)"
    fi
else
    echo "✗ CAPI credentials file missing"
    echo "  Run: docker exec charon cscli capi register"
fi

# Check 5: Console enrollment status
echo "[5/8] Checking console enrollment status..."
CONSOLE_STATUS=$(docker exec charon cscli console status 2>&1 || echo "error")
if echo "$CONSOLE_STATUS" | grep -qi "enrolled"; then
    echo "✓ Console enrollment detected"
    echo "  $CONSOLE_STATUS"
else
    echo "✗ Console not enrolled"
    echo "  Enroll via Charon UI: /security/crowdsec"
fi

# Check 6: Config validation
echo "[6/8] Validating configuration files..."
if docker exec charon test -f /etc/crowdsec/config.yaml; then
    echo "✓ config.yaml exists"
else
    echo "✗ config.yaml missing"
fi

if docker exec charon test -f /etc/crowdsec/acquis.yaml; then
    echo "✓ acquis.yaml exists"
    if docker exec charon grep -q "source:" /etc/crowdsec/acquis.yaml; then
        echo "✓ acquis.yaml has datasource configuration"
    else
        echo "✗ acquis.yaml missing datasource"
    fi
else
    echo "✗ acquis.yaml missing"
fi

# Check 7: Network connectivity
echo "[7/8] Checking network connectivity..."
if docker exec charon curl -fsS --connect-timeout 5 https://api.crowdsec.net/health >/dev/null 2>&1; then
    echo "✓ Can reach crowdsec.net API"
else
    echo "✗ Cannot reach crowdsec.net API"
    echo "  Check firewall, proxy, DNS configuration"
fi

# Check 8: Database enrollment state
echo "[8/8] Checking database enrollment state..."
ENROLLMENT_STATUS=$(docker exec charon sqlite3 /app/data/charon.db \
    "SELECT status FROM crowdsec_console_enrollments LIMIT 1;" 2>/dev/null || echo "")
if [ -n "$ENROLLMENT_STATUS" ]; then
    echo "✓ Enrollment record found: $ENROLLMENT_STATUS"
else
    echo "✓ No enrollment record (not enrolled)"
fi

echo ""
echo "Diagnostic complete!"
echo ""
echo "If CrowdSec shows as offline in console:"
echo "  1. Verify all checks above passed"
echo "  2. Check crowdsec.net console for agent last seen"
echo "  3. Review CrowdSec logs: docker exec charon tail -100 /var/log/crowdsec/crowdsec.log"
echo "  4. Contact CrowdSec support if issue persists"
```

---

## Test Execution Plan

### Phase 1: Unit Tests (Day 1-2)
```bash
# Run enrollment service unit tests
cd backend
go test -v ./internal/crowdsec/... -run TestConsoleEnrollment

# Expected: All new token validation, LAPI check, and CAPI registration tests pass
```

### Phase 2: Integration Tests (Day 3-4)
```bash
# Run LAPI integration tests
cd backend
go test -v -tags=integration ./integration/... -run TestCrowdSecLAPI

# Expected: LAPI startup, health checks, and CAPI connectivity tests pass
```

### Phase 3: E2E Tests (Day 5-6)
```bash
# Rebuild E2E container
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e --clean

# Run console enrollment E2E tests
npx playwright test tests/security/crowdsec-console-enrollment.spec.ts

# Run console monitoring E2E tests
npx playwright test tests/security/crowdsec-console-monitoring.spec.ts

# Run diagnostic E2E tests
npx playwright test tests/security/crowdsec-diagnostics.spec.ts

# Expected: All enrollment flow, validation, and diagnostic tests pass
```

### Phase 4: Manual Verification (Day 7)
```bash
# Run diagnostic script
./scripts/diagnose-crowdsec.sh

# Expected: All checks pass, detailed status report generated
```

### Phase 5: Production Validation (Day 8-10)
1. Deploy to staging environment
2. Complete enrollment with real crowdsec.net console
3. Monitor heartbeat status over 24-48 hours
4. Verify engine stays online in console
5. Document any additional issues discovered

---

## Success Criteria

### Short-term (Week 1-2)
- ✅ All diagnostic endpoints implemented and functional
- ✅ Enrollment validation enhanced with better error messages
- ✅ Config validation endpoint reports accurate status
- ✅ Connectivity check endpoint identifies network issues

### Medium-term (Week 3-4)
- ✅ Heartbeat polling service implemented and running
- ✅ LastHeartbeatAt field populated correctly
- ✅ Console status transitions from pending_acceptance to enrolled automatically
- ✅ Prometheus metrics tracking enrollment success/failure rates
- ✅ All unit tests pass with 100% coverage for new code
- ✅ All integration tests pass consistently

### Long-term (Week 4+)
- ✅ All E2E tests pass on Chromium, Firefox, Webkit
- ✅ Diagnostic script catches 90%+ of common issues
- ✅ Documentation updated with troubleshooting guides
- ✅ Zero false positives in offline detection
- ✅ Engine consistently shows online in console for enrolled instances
- ✅ User-reported enrollment issues reduced by 80%+

---

## Risk Mitigation

### Risk 1: LAPI Initialization Timing
**Mitigation:**
- Increase retry attempts from 3 to 5
- Implement exponential backoff (3s, 6s, 12s, 24s)
- Add detailed logging for each retry attempt
- Document expected initialization time (10-15s on slow hardware)

### Risk 2: Network Connectivity Variability
**Mitigation:**
- Add explicit connectivity checks before enrollment
- Test against both api.crowdsec.net and main crowdsec.net domains
- Document firewall/proxy requirements clearly
- Provide fallback diagnostic commands

### Risk 3: Token Expiry Edge Cases
**Mitigation:**
- Improve error message extraction to detect expiry
- Document token lifetime (24-48 hours)
- Add warning in UI when token is >24 hours old
- Provide clear instructions to regenerate token

### Risk 4: Database State Corruption
**Mitigation:**
- Add validation for enrollment state transitions
- Implement database migration to ensure schema consistency
- Add repair mechanism for corrupted enrollment records
- Document manual DB cleanup procedures

### Risk 5: Test Flakiness
**Mitigation:**
- Use deterministic wait strategies (not arbitrary sleeps)
- Implement retry logic for network-dependent tests
- Mock external dependencies where possible
- Run tests in isolated containers to prevent interference

---

## Monitoring and Alerting

### Metrics to Track
1. **Enrollment Success Rate:** `charon_crowdsec_enrollment_attempts_total{status="success"}`
2. **Enrollment Failure Rate:** `charon_crowdsec_enrollment_attempts_total{status="failed"}`
3. **LAPI Health:** `charon_crowdsec_lapi_healthy` (1 = healthy, 0 = unhealthy)
4. **Heartbeat Latency:** `charon_crowdsec_heartbeat_latency_seconds`
5. **Enrollment Duration:** `charon_crowdsec_enrollment_duration_seconds`

### Alerts to Create
1. **LAPI Down:** Alert if `charon_crowdsec_lapi_healthy == 0` for >5 minutes
2. **Enrollment Failures:** Alert if enrollment failure rate >20% over 1 hour
3. **Heartbeat Timeout:** Alert if no heartbeat received for >10 minutes (when enrolled)
4. **Console Offline:** Alert if engine shows offline in console for >30 minutes

### Log Queries
```bash
# Enrollment attempts in last hour
grep "crowdsec console enrollment" /var/log/charon.log | tail -100

# LAPI health check failures
grep "LAPI check" /var/log/charon.log | grep -i error

# Heartbeat status
grep "heartbeat" /var/log/crowdsec/crowdsec.log | tail -50
```

---

## Appendix A: File Manifest

### New Files to Create
1. `backend/internal/crowdsec/console_enroll_test.go` - Unit tests for enrollment service
2. `backend/integration/crowdsec_lapi_integration_test.go` - LAPI integration tests
3. `tests/security/crowdsec-console-enrollment.spec.ts` - E2E enrollment tests
4. `tests/security/crowdsec-console-monitoring.spec.ts` - E2E monitoring tests
5. `tests/security/crowdsec-diagnostics.spec.ts` - E2E diagnostic tests
6. `backend/internal/crowdsec/heartbeat_poller.go` - Heartbeat polling service
7. `backend/internal/metrics/crowdsec_metrics.go` - Prometheus metrics
8. `scripts/diagnose-crowdsec.sh` - Diagnostic script

### Files to Modify
1. `backend/internal/api/handlers/crowdsec_handler.go` - Add diagnostic endpoints
2. `backend/internal/crowdsec/console_enroll.go` - Enhance retry logic, error messages
3. `backend/cmd/api/main.go` - Integrate heartbeat poller
4. `docs/cerberus.md` - Add troubleshooting section

### Files to Review (No Changes)
1. `backend/internal/models/crowdsec_console_enrollment.go` - Model is adequate
2. `.docker/docker-entrypoint.sh` - Config initialization is adequate
3. `docker-compose.yml` - Network configuration is adequate

---

## Appendix B: API Endpoint Reference

### New Diagnostic Endpoints
| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/api/v1/admin/crowdsec/diagnostics/connectivity` | GET | Check connectivity to crowdsec.net APIs | Admin |
| `/api/v1/admin/crowdsec/diagnostics/config` | GET | Validate CrowdSec configuration files | Admin |
| `/api/v1/admin/crowdsec/console/heartbeat` | GET | Get console heartbeat status | Admin |

### Existing Enrollment Endpoints
| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/api/v1/admin/crowdsec/console/enrollment` | GET | Get current enrollment status | Admin |
| `/api/v1/admin/crowdsec/console/enrollment` | POST | Enroll with CrowdSec console | Admin |
| `/api/v1/admin/crowdsec/console/enrollment` | DELETE | Clear enrollment state | Admin |
| `/api/v1/admin/crowdsec/status` | GET | Get CrowdSec running status | Admin |
| `/api/v1/admin/crowdsec/start` | POST | Start CrowdSec process | Admin |
| `/api/v1/admin/crowdsec/stop` | POST | Stop CrowdSec process | Admin |

---

## Appendix C: Database Schema

### CrowdsecConsoleEnrollment Table
```sql
CREATE TABLE crowdsec_console_enrollments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL, -- not_enrolled, enrolling, pending_acceptance, enrolled, failed
    tenant TEXT,
    agent_name TEXT,
    encrypted_enroll_key TEXT, -- AES-256 encrypted
    last_error TEXT,
    last_correlation_id TEXT,
    last_attempt_at DATETIME,
    enrolled_at DATETIME,
    last_heartbeat_at DATETIME, -- NEW: Updated by heartbeat poller
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_crowdsec_enrollments_status ON crowdsec_console_enrollments(status);
CREATE INDEX idx_crowdsec_enrollments_correlation_id ON crowdsec_console_enrollments(last_correlation_id);
```

---

## Appendix D: References

### Internal Documentation
- [Cerberus Security Documentation](../cerberus.md)
- [Security Architecture](../security.md)
- [Testing Instructions](../../.github/instructions/testing.instructions.md)

### External Resources
- [CrowdSec Documentation](https://docs.crowdsec.net/)
- [CrowdSec Console Enrollment](https://docs.crowdsec.net/docs/next/console/enrollment/)
- [CrowdSec LAPI](https://docs.crowdsec.net/docs/next/local_api/intro/)
- [CrowdSec CAPI](https://docs.crowdsec.net/docs/next/capi/)

### GitHub Issues
- [Issue #586](https://github.com/Wikid82/charon/issues/586) - CrowdSec offline since 12/19/25

---

## Document Version Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-02-03 | GitHub Copilot | Initial comprehensive plan |

---

**End of Document**
