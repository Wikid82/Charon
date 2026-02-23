# E2E Test Failures - Emergency Token & API Endpoints Fix Plan

**Status**: Ready for Implementation
**Priority**: Critical
**Created**: 2026-01-27
**Test Results**: 129/162 passing (80%) - 6 failures, 27 skipped

## Executive Summary

All 6 E2E test failures trace back to **emergency token server not being configured** despite the environment variable being set correctly in the container. This is a **blocking issue** that must be fixed first, as other test failures may be false positives caused by this misconfiguration.

## Problem Statement

### Critical Issue: Emergency Token Server Returns 501

The backend emergency token endpoint returns:
```json
{
  "error": "not configured",
  "message": "Emergency reset is not configured. Set CHARON_EMERGENCY_TOKEN environment variable."
}
```

**But the environment variable IS set:**
```bash
$ docker exec charon-e2e env | grep CHARON_EMERGENCY_TOKEN
CHARON_EMERGENCY_TOKEN=f51dedd6a4f2eaa200dcbf4feecae78ff926e06d9094d726f3613729b66d346b
```

**Impact**:
- 4 emergency reset tests fail with 501 errors
- 2 tests fail with 404 errors (API endpoints missing)
- Global setup warns about failed emergency reset
- Cannot validate admin whitelist fixes

## Requirements (EARS Notation)

### R1: Emergency Token Server Configuration
**WHEN** the emergency token server starts, **THE SYSTEM SHALL** successfully read the emergency token (from database or environment variable) and initialize the emergency reset endpoint.

**Acceptance Criteria**:
- Emergency endpoint returns 200 OK when called with valid token
- Emergency endpoint returns 401 Unauthorized for invalid/missing token
- Emergency endpoint returns 501 ONLY if no token is configured
- Global setup emergency reset succeeds with no warnings
- Server checks database first, then falls back to CHARON_EMERGENCY_TOKEN env var for backward compatibility

### R2: Emergency Reset API Functionality
**WHEN** emergency reset is called with a valid token via Basic Auth, **THE SYSTEM SHALL** disable all security modules and return success response.

**Acceptance Criteria**:
- POST `/emergency/security-reset` with valid Basic Auth returns 200
- Response contains `{"success": true, "disabled_modules": [...]}`
- ACL, WAF, CrowdSec, and rate limiting are all disabled
- Caddy configuration is reloaded

### R3: UI-Based Emergency Token Management
**WHEN** an admin user accesses the Emergency Token settings, **THE SYSTEM SHALL** provide a UI to generate, view metadata, and regenerate the emergency token.

**Acceptance Criteria**:
- Admin can generate new token via UI (requires authentication)
- Token is generated with cryptographically secure randomness (64 bytes minimum)
- Token is displayed in plaintext ONCE during generation
- Prominent warning: "Save this token immediately - you will not see it again"
- Token stored as bcrypt hash in database (NEVER plaintext)
- UI shows token status: "Configured - Last generated: [date] - Expires: [date]"
- Admin can regenerate token (invalidates old token immediately)

### R4: Emergency Token Expiration Policy
**WHEN** an admin generates an emergency token, **THE SYSTEM SHALL** allow selection of expiration policy similar to GitHub PATs.

**Acceptance Criteria**:
- Expiration options: 30 days, 60 days, 90 days (default), Custom (1-365 days), Never
- Token expiration is enforced at validation time (401 if expired)
- Expired tokens cannot be used for emergency reset
- Admin can view expiration date in UI
- Admin can change expiration policy for existing token

### R5: Emergency Token Expiration Notifications
**WHEN** an emergency token is within 14 days of expiration, **THE SYSTEM SHALL** notify the admin through the notification system.

**Acceptance Criteria**:
- Internal notification (mandatory): Banner in admin UI showing days until expiration
- External notification (optional): Email/webhook if configured
- Notifications sent at 14 days, 7 days, 3 days, and 1 day before expiration
- Notification includes direct link to token regeneration page
- After expiration, notification changes to "Emergency token expired - regenerate immediately"

### R3: Configuration API Endpoint
**WHEN** PATCH `/api/v1/config` is called with authentication, **THE SYSTEM SHALL** update the specified configuration settings.

**Acceptance Criteria**:
- Endpoint exists and returns 200/204 on success
- Can update `security.admin_whitelist` configuration
- Changes are persisted to configuration store
- Caddy configuration is reloaded if security settings change

## Root Cause Analysis

### Hypothesis 1: Environment Variable Name Mismatch
Backend code may be checking for a different env var name (e.g., `EMERGENCY_TOKEN` instead of `CHARON_EMERGENCY_TOKEN`).

**Evidence Needed**: Search backend code for emergency token env var loading

### Hypothesis 2: Initialization Timing Issue
Emergency server may be initializing before env vars are loaded, or using a stale config.

**Evidence Needed**: Check emergency server initialization sequence

### Hypothesis 3: Different Binary/Build
The `charon:e2e-test` image may be using a different build than expected.

**Evidence Needed**: Verify Docker image build includes emergency token support

### Hypothesis 4: Emergency Server Not Enabled
Despite `CHARON_EMERGENCY_SERVER_ENABLED=true`, the server may not be starting.

**Evidence Needed**: Check container logs for emergency server startup messages

### Hypothesis 5: Build Cache Issue
The `charon:e2e-test` image may be using a cached build with old code, despite environment variables being set correctly.

**Evidence Needed**: Verify Docker image build timestamp and binary version inside container

### Hypothesis 6: Response Code Bug
The emergency endpoint may be correctly reading the token but returning wrong status code (501 instead of 401/403) due to error handling logic.

**Evidence Needed**: Examine error handling in emergency endpoint code

## Phased Implementation Plan

---

## 📍 PHASE 0: Environment Verification & Clean Rebuild
**Priority**: CRITICAL - MUST COMPLETE FIRST
**Estimated Time**: 30 minutes
**Assignee**: DevOps

### Task 0.1: Clean Environment Rebuild
**Actions**:
```bash
# Stop and remove all containers, volumes, networks
docker compose -f .docker/compose/docker-compose.playwright-local.yml down -v

# Clean build with no cache
docker build --no-cache -t charon:e2e-test .

# Start fresh environment
docker compose -f .docker/compose/docker-compose.playwright-local.yml up -d
```

**Deliverable**: Clean environment with verified fresh build

### Task 0.2: Verify Build Integrity
**Actions**:
```bash
# Check image build timestamp (should be within last hour)
docker inspect charon:e2e-test --format='{{.Created}}'

# Verify running container matches expected image
docker ps --filter "name=charon-e2e" --format '{{.Image}} {{.CreatedAt}}'

# Check binary version inside container
docker exec charon-e2e /app/charon -version || echo "Version check failed"

# Verify build info in binary
docker exec charon-e2e strings /app/charon | grep -i "emergency\|version\|built" | head -20
```

**Expected Results**:
- Image created within last hour
- Container running correct image tag
- Binary contains recent build timestamp

**Deliverable**: Build integrity verification report

### Task 0.3: Baseline Capture
**Actions**:
```bash
# Capture baseline logs
docker logs charon-e2e > test-results/logs/baseline_logs.txt 2>&1

# Quick smoke test
curl -f http://localhost:8080/health || echo "Health check failed"

# Capture environment variables
docker exec charon-e2e env | grep CHARON_ | sort > test-results/logs/baseline_env.txt
```

**Deliverable**: Baseline logs and environment snapshot

---

## 📍 PHASE 1: Emergency Token Investigation & Fix
**Priority**: CRITICAL - BLOCKING ALL OTHER WORK
**Estimated Time**: 2-4 hours
**Assignee**: Backend_Dev

### Task 1.1: Investigate Backend Token Loading
**File Locations**:
- Search: `backend/**/*emergency*.go`
- Search: `backend/**/config*.go` for env var loading
- Check: Emergency server initialization code

**Actions**:
1. Find where `CHARON_EMERGENCY_TOKEN` is read from environment
2. Check for typos, case sensitivity, or name mismatches
3. Verify initialization order (is config loaded before server starts?)
4. Check if token validation happens at startup or per-request

**Deliverable**: Root cause identified with specific file/line numbers

### Task 1.2: Verify Container Logs
**Actions**:
```bash
# Check if emergency server actually starts
docker compose -f .docker/compose/docker-compose.playwright-local.yml logs charon-e2e | grep -i emergency

# Check for any startup errors
docker compose -f .docker/compose/docker-compose.playwright-local.yml logs charon-e2e | grep -i error

# Verify env vars are loaded
docker exec charon-e2e env | grep CHARON_
```

**Deliverable**: Log analysis confirming emergency server status

### Task 1.3: Fix Emergency Token Loading
**Based on findings from 1.1 and 1.2**

**Decision Tree**:
- **IF** env var name mismatch → Correct variable name in code
- **ELSE IF** initialization timing issue → Move token load to earlier stage
- **ELSE IF** token validation logic wrong → Fix validation + add unit tests
- **ELSE IF** build cache issue → Already fixed in Phase 0
- **ELSE** → Escalate to senior engineer with full diagnostic report

**Possible Fixes**:
- Correct environment variable name if mismatched
- Move token loading earlier in initialization sequence
- Add debug logging to confirm token is read (with redaction)
- Ensure emergency server only starts if token is valid

**Required Code Changes**:
1. **Add startup validation**:
   ```go
   // Fail fast if misconfigured
   if emergencyServerEnabled && emergencyToken == "" {
       log.Fatal("CHARON_EMERGENCY_SERVER_ENABLED=true but CHARON_EMERGENCY_TOKEN is empty")
   }
   ```

2. **Add startup log** (with token redaction):
   ```go
   log.Info("Emergency server initialized with token: [REDACTED]")
   ```

3. **Add unit tests**:
   ```go
   // backend/internal/emergency/server_test.go
   func TestEmergencyServerStartupValidation(t *testing.T) {
       // Test that server fails if token empty but server enabled
   }

   func TestEmergencyTokenLoadedFromEnv(t *testing.T) {
       // Test env var is read correctly
   }
   ```

**Security Requirements**:
- ✅ All logging must redact emergency token
- ✅ Replace full token with: `[EMERGENCY_TOKEN:xxxx...xxxx]` (first/last 4 chars only)
- ✅ Test: `docker logs charon-e2e | grep -i emergency` should NOT show full token
- ✅ Add rate limiting: max 3 attempts per minute per IP
- ✅ Add audit logging: timestamp, source IP, result for every call

**Test Validation**:
```bash
# Should return 200 OK
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \
  -H "X-Emergency-Token: f51dedd6a4f2eaa200dcbf4feecae78ff926e06d9094d726f3613729b66d346b"

# Should return 401 Unauthorized
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \
  -H "X-Emergency-Token: invalid-token"

# Should return 501 Not Configured (empty token)
CHARON_EMERGENCY_TOKEN="" docker compose ... up -d
curl -X POST http://localhost:2020/emergency/security-reset ...

# Should return 501 Not Configured (whitespace token)
CHARON_EMERGENCY_TOKEN="   " docker compose ... up -d
curl -X POST http://localhost:2020/emergency/security-reset ...
```

**Edge Case Tests**:
```typescript
// Add to tests/security-enforcement/emergency-reset.spec.ts

test('empty token env var returns 501', async () => {
  // Restart container with CHARON_EMERGENCY_TOKEN=""
  // Expect 501 Not Configured
});

test('whitespace-only token is rejected', async () => {
  // Restart container with CHARON_EMERGENCY_TOKEN="   "
  // Expect 501 Not Configured
});

test('concurrent emergency reset calls succeed', async () => {
  // Call emergency reset from 2 tests simultaneously
  // Both should succeed OR second should gracefully handle "already disabled"
});

test('emergency reset idempotency', async () => {
  // Call emergency reset twice in a row
  // Second call should succeed with "already disabled" message
});

test('Caddy reload failure handling', async () => {
  // Simulate Caddy reload failure (stop Caddy)
  // Emergency endpoint should return 500 with error details
});

test('token logged as redacted', async () => {
  // Check docker logs for emergency token
  // Should only show [EMERGENCY_TOKEN:f51d...346b]
});
```

**Deliverable**: Emergency endpoint returns correct status codes for all edge cases

### Task 1.4: Rebuild & Validate
**Actions**:
1. Rebuild Docker image: `docker build -t charon:e2e-test .`
2. Restart container: `docker compose -f .docker/compose/docker-compose.playwright-local.yml up -d --force-recreate`
3. Run emergency reset tests: `npx playwright test tests/security-enforcement/emergency-reset.spec.ts`

**Expected Results**:
- 4/4 emergency reset tests should pass (currently 0/4)
- Global setup should complete without warnings
- Emergency endpoint accessible at localhost:2020

**Deliverable**: Emergency reset tests passing

---

## 📍 PHASE 2: API Endpoints & UI-Based Token Management
**Priority**: HIGH - Blocking 2 test failures + Long-term security improvement
**Estimated Time**: 5-8 hours (includes UI token management)
**Assignee**: Backend_Dev + Frontend_Dev (parallel after Task 2.1)
**Depends On**: Phase 1 complete

### Task 2.1: Implement Emergency Token API Endpoints (Backend)

**New Endpoints**:

```go
// POST /api/v1/emergency/token/generate
// Generates new emergency token with expiration policy
// Requires admin authentication
// Request: {"expiration_days": 90}  // or 30, 60, 0 (never), custom
// Response: {
//   "token": "abc123...xyz789",  // plaintext, shown ONCE
//   "created_at": "2026-01-27T10:00:00Z",
//   "expires_at": "2026-04-27T10:00:00Z",
//   "expiration_policy": "90_days"
// }

// GET /api/v1/emergency/token/status
// Returns token metadata (NOT the token itself)
// Requires admin authentication
// Response: {
//   "configured": true,
//   "created_at": "2026-01-27T10:00:00Z",
//   "expires_at": "2026-04-27T10:00:00Z",
//   "expiration_policy": "90_days",
//   "days_until_expiration": 89,
//   "is_expired": false
// }

// DELETE /api/v1/emergency/token
// Revokes current emergency token
// Requires admin authentication
// Response: {"success": true, "message": "Emergency token revoked"}

// PATCH /api/v1/emergency/token/expiration
// Updates expiration policy for existing token
// Requires admin authentication
// Request: {"expiration_days": 60}
// Response: {"success": true, "new_expires_at": "..."}
```

**Database Schema**:
```sql
CREATE TABLE emergency_tokens (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL,  -- bcrypt hash
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,  -- NULL for never expire
    expiration_policy TEXT NOT NULL,  -- "30_days", "90_days", "never", etc.
    created_by_user_id INTEGER,
    last_used_at TIMESTAMP,
    use_count INTEGER DEFAULT 0,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id)
);

CREATE INDEX idx_emergency_token_expires ON emergency_tokens(expires_at);
```

**Security Requirements**:
- Generate token with `crypto/rand` - minimum 64 bytes
- Store only bcrypt hash (cost factor 12+)
- Validate expiration on every emergency reset call
- Log all generate/regenerate/revoke events
- Return 401 if token expired
- Backward compatibility: Check database first, fall back to CHARON_EMERGENCY_TOKEN env var

**Test Cases**:
```go
func TestGenerateEmergencyToken(t *testing.T) {
    // Test token generation with different expiration policies
    // Test token is 64+ bytes
    // Test hash is stored, not plaintext
    // Test expiration is calculated correctly
}

func TestEmergencyTokenExpiration(t *testing.T) {
    // Test expired token returns 401
    // Test "never" policy never expires
    // Test token validation checks expiration
}

func TestEmergencyTokenBackwardCompatibility(t *testing.T) {
    // Test env var still works if no DB token
    // Test DB token takes precedence over env var
}
```

**Deliverable**: Emergency token API endpoints functional with database storage

### Task 2.2: Implement PATCH /api/v1/config Endpoint (Backend)

**Requirements**:
```go
// PATCH /api/v1/config
// Updates configuration settings
// Requires authentication
// Request body: {"security": {"admin_whitelist": "127.0.0.1/32,..."}}
// Response: 200 OK or 204 No Content
```

**Test Cases**:
```typescript
// Should update admin whitelist
const response = await request.patch('/api/v1/config', {
  data: { security: { admin_whitelist: '127.0.0.1/32' } }
});
expect(response.ok()).toBeTruthy();

// Should persist changes
const getResponse = await request.get('/api/v1/config');
expect(getResponse.json()).toContain('127.0.0.1/32');
```

**Deliverable**: PATCH /api/v1/config endpoint functional

### Task 2.3: Verify Security Enable Endpoints (Backend)

**Check if these exist**:
- `POST /api/v1/security/acl/enable` (or similar)
- `POST /api/v1/security/cerberus/enable` (or similar)

**If missing, implement**:
```go
// POST /api/v1/security/{module}/enable
// Enables the specified security module
// Requires authentication
// Response: 200 OK with status
```

**Test**:
```bash
curl -X POST http://localhost:8080/api/v1/security/acl/enable \
  -H "Cookie: session=..." \
  -H "Content-Type: application/json"
```

**Deliverable**: Security module enable endpoints functional

### Task 2.4: Emergency Token UI Implementation (Frontend)
**Assignee**: Frontend_Dev
**Depends On**: Task 2.1 complete
**Can run in parallel with**: Task 2.2, 2.3

**New Admin Settings Page**: `/admin/emergency-token`

**UI Components**:

1. **Token Status Card**:
   ```typescript
   // Shows when token is configured
   <Card>
     <Badge status="success">Emergency Token Configured</Badge>
     <Metadata>
       - Created: 2026-01-27 10:00:00
       - Expires: 2026-04-27 10:00:00 (89 days)
       - Policy: 90 days
       - Last Used: Never / 2026-01-27 15:30:00
       - Use Count: 0
     </Metadata>

     <Collapsible title="Usage Instructions (How to Use Your Token)">
       <Alert variant="info">
         Use these commands with your saved emergency token when you need to disable all security.
       </Alert>
       <Tabs>
         <Tab label="Docker">
           <Code copyable language="bash">
             {`docker exec charon curl -X POST http://localhost:2020/emergency/security-reset \\
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \\
  -H "X-Emergency-Token: YOUR_SAVED_TOKEN"`}
           </Code>
         </Tab>
         <Tab label="cURL">
           <Code copyable language="bash">
             {`curl -X POST http://localhost:2020/emergency/security-reset \\
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \\
  -H "X-Emergency-Token: YOUR_SAVED_TOKEN"`}
           </Code>
         </Tab>
         <Tab label="CLI">
           <Code copyable language="bash">
             {`charon emergency reset \\
  --token "YOUR_SAVED_TOKEN" \\
  --admin-user admin \\
  --admin-pass changeme`}
           </Code>
         </Tab>
       </Tabs>
     </Collapsible>

     <Actions>
       <Button variant="primary">Regenerate Token</Button>
       <Button variant="secondary">Change Expiration</Button>
       <Button variant="danger">Revoke Token</Button>
     </Actions>
   </Card>
   ```

2. **Token Generation Modal**:
   ```typescript
   <Modal title="Generate Emergency Token">
     <Alert variant="warning">
       ⚠️ This token provides unrestricted access to disable all security.
       Store it securely in a password manager.
     </Alert>

     <Select label="Expiration Policy">
       <Option value={30}>30 days</Option>
       <Option value={60}>60 days</Option>
       <Option value={90} selected>90 days (Recommended)</Option>
       <Option value="custom">Custom (1-365 days)</Option>
       <Option value={0}>Never expire</Option>
     </Select>

     {policy === 'custom' && (
       <Input type="number" label="Custom Days" min={1} max={365} />
     )}

     <Button onClick={generateToken}>Generate Token</Button>
   </Modal>
   ```

3. **Token Display Modal** (shows ONCE after generation):
   ```typescript
   <Modal title="Save Your Emergency Token" closable={false}>
     <Alert variant="critical">
       🔒 SAVE THIS TOKEN NOW - You will not see it again!
     </Alert>

     <Section>
       <Label>Emergency Token</Label>
       <TokenDisplay>
         <Code copyable>{generatedToken}</Code>
       </TokenDisplay>
     </Section>

     <Section>
       <Label>How to Use (Copy & Save with Token)</Label>
       <Tabs>
         <Tab label="Docker (Recommended)">
           <Code copyable language="bash">
             {`# Emergency reset via Docker
docker exec charon curl -X POST http://localhost:2020/emergency/security-reset \\
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \\
  -H "X-Emergency-Token: ${generatedToken}"`}
           </Code>
         </Tab>

         <Tab label="cURL (Direct Access)">
           <Code copyable language="bash">
             {`# Emergency reset via cURL (from host with access to container)
curl -X POST http://localhost:2020/emergency/security-reset \\
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \\
  -H "X-Emergency-Token: ${generatedToken}"`}
           </Code>
         </Tab>

         <Tab label="CLI (If Installed)">
           <Code copyable language="bash">
             {`# Emergency reset via Charon CLI
charon emergency reset \\
  --token "${generatedToken}" \\
  --admin-user admin \\
  --admin-pass changeme`}
           </Code>
         </Tab>
       </Tabs>

       <Alert variant="info">
         💡 <strong>Tip:</strong> Save these commands in your password manager along with the token.
         When needed, just copy and paste the appropriate command for your setup.
       </Alert>
     </Section>

     <Metadata>
       - Expires: 2026-04-27 10:00:00 (90 days)
       - Created: Just now
     </Metadata>

     <Checklist>
       <Checkbox required>
         I have saved this token AND usage commands in a secure location (password manager)
       </Checkbox>
       <Checkbox required>
         I understand this token cannot be recovered if lost
       </Checkbox>
       <Checkbox required>
         I have tested the command works (optional but recommended)
       </Checkbox>
     </Checklist>

     <Button disabled={!allChecked} onClick={closeModal}>
       I've Saved Everything
     </Button>
   </Modal>
   ```

4. **Expiration Warning Banner**:
   ```typescript
   // Shows when token is within 14 days of expiration
   <Banner variant="warning" dismissible={false}>
     <Icon name="clock" />
     Your emergency token expires in {daysUntilExpiration} days.
     <Link to="/admin/emergency-token">Regenerate now</Link>
   </Banner>
   ```

5. **Expired Token Banner**:
   ```typescript
   // Shows when token is expired
   <Banner variant="danger" dismissible={false}>
     <Icon name="alert" />
     Your emergency token has expired! Emergency reset will not work.
     <Link to="/admin/emergency-token">Generate new token</Link>
   </Banner>
   ```

**Notification Integration**:
```typescript
// Add to notification system
interface EmergencyTokenNotification {
  type: 'emergency_token_expiring' | 'emergency_token_expired';
  severity: 'warning' | 'critical';
  days_until_expiration: number;
  action_url: '/admin/emergency-token';
  mandatory: true;  // Cannot be dismissed
}

// Notification preferences
interface NotificationPreferences {
  emergency_token_expiration: {
    internal: true;  // Always enabled, cannot disable
    external_email: boolean;  // Optional
    external_webhook: boolean;  // Optional
  };
}
```

**Accessibility Requirements**:
- All form inputs have proper labels
- Error messages are announced to screen readers
- Keyboard navigation works for all modals
- Color is not the only indicator (icons + text for warnings)
- Token display has high contrast
- Copy button has proper ARIA label

**Security Requirements**:
- Token display uses monospace font to prevent confusion
- Copy button uses Clipboard API (secure context only)
- No token in URL parameters or localStorage
- Token only visible during generation modal
- All API calls use HTTPS

**Test Cases**:
```typescript
test('generates token with selected expiration policy', async () => {
  // Select 60 days policy
  // Click Generate
  // Verify token displayed
  // Verify expiration date calculated correctly
});

test('token display requires confirmation checkboxes', async () => {
  // Generate token
  // Try to close modal without checking boxes
  // Should be disabled
  // Check both boxes
  // Button should be enabled
});

test('shows expiration warning banner when < 14 days', async () => {
  // Mock token with 10 days until expiration
  // Verify warning banner appears
  // Verify link to regenerate page
});

test('cannot dismiss mandatory expiration notifications', async () => {
  // Verify warning banner has no dismiss button
  // Verify banner persists across page loads
});

test('usage commands include actual token during generation', async () => {
  // Generate token
  // Verify Docker/cURL/CLI commands contain the actual token
  // Verify commands are properly formatted and executable
});

test('usage instructions available in status card', async () => {
  // Navigate to emergency token page with configured token
  // Expand usage instructions collapsible
  // Verify commands are shown (without actual token)
  // Verify copy buttons work
});

test('copy button works for token and commands', async () => {
  // Generate token
  // Click copy button on token
  // Verify clipboard contains token
  // Click copy button on Docker command
  // Verify clipboard contains full command with token
});
```

**Deliverable**: Emergency token UI fully functional with expiration management

### Task 2.5: Integration Test
**Actions**:
1. Run security enforcement tests: `npx playwright test tests/security-enforcement/`
2. Verify configureAdminWhitelist() no longer returns 404
3. Verify emergency-token test setup succeeds

**Expected Results**:
- Emergency token tests pass (7 tests, currently 1 fail + 6 skipped)
- Admin whitelist test passes (3 tests, currently 1 fail + 2 skipped)
- No more "Failed to configure admin whitelist: 404" warnings

**Deliverable**: All security enforcement tests passing except CrowdSec-dependent ones

---

## 📍 PHASE 3: Validation & Regression Testing
**Priority**: MEDIUM - Ensure no regressions
**Estimated Time**: 1-2 hours
**Assignee**: QA_Security
**Depends On**: Phase 1 & 2 complete

### Task 3.1: Full E2E Test Suite
**Actions**:
```bash
# Run complete suite
npx playwright test

# Generate coverage report
npx playwright test --coverage
```

**Success Criteria**:
- **Target**: ≥145/162 tests passing (90%+)
- **Emergency tests**: 4/4 passing (was 0/4)
- **Emergency token protocol**: 7/7 passing (was 1/7)
- **Admin whitelist**: 3/3 passing (was 1/3)
- **Overall**: 6 failures fixed, ~14 tests recovered from skipped

**Deliverable**: Test results report with comparison

### Task 3.2: Manual Verification
**Test Scenarios**:

1. **Emergency Reset via curl**:
   ```bash
   # Enable ACL
   # Try to access API (blocked)
   # Use emergency reset
   # Verify ACL disabled
   ```

2. **Admin Whitelist Configuration**:
   ```bash
   # Login to dashboard
   # Navigate to Security > Admin Whitelist
   # Add IP range: 192.168.1.0/24
   # Save and verify in UI
   ```

3. **Container Restart Persistence**:
   ```bash
   # Configure admin whitelist
   # Restart container
   # Verify whitelist persists (should be in tmpfs, so it won't)
   ```

**Deliverable**: Manual test checklist completed

### Task 3.3: Update Documentation
**Files to Update**:
- `docs/troubleshooting/e2e-tests.md` - Add emergency token troubleshooting
- `docs/getting-started.md` - Clarify emergency token setup
- `docs/security.md` - **ADD WARNING**: Emergency server port 2020 is localhost/internal-only
- `docs/emergency-reset.md` - **NEW**: Add FAQ with ready-to-use commands
- `README.md` - Update E2E test status
- `tests/security-enforcement/README.md` - Document admin whitelist setup

**New Documentation: docs/emergency-reset.md**:
```markdown
# Emergency Reset Guide

## What is Emergency Reset?

Emergency reset allows administrators to disable ALL security modules when locked out.

## When to Use

⚠️ **Only use in genuine emergencies:**
- Locked out of admin dashboard due to ACL misconfiguration
- WAF blocking legitimate requests
- CrowdSec banning your IP incorrectly
- Rate limiting preventing access

## How to Get Your Token

1. Login to Charon admin dashboard
2. Navigate to **Settings > Emergency Token**
3. Click **Generate Emergency Token**
4. **IMMEDIATELY save the token and commands** in your password manager
5. You will NOT see the token again

## How to Use Your Token

### Docker Deployment (Most Common)

```bash
docker exec charon curl -X POST http://localhost:2020/emergency/security-reset \
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \
  -H "X-Emergency-Token: YOUR_TOKEN_HERE"
```

### Direct Access (Non-Docker)

```bash
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "Authorization: Basic YWRtaW46Y2hhbmdlbWU=" \
  -H "X-Emergency-Token: YOUR_TOKEN_HERE"
```

### CLI (If Installed)

```bash
charon emergency reset \
  --token "YOUR_TOKEN_HERE" \
  --admin-user admin \
  --admin-pass changeme
```

## Frequently Asked Questions

### Q: I lost my emergency token, what do I do?

**A:** Login to admin dashboard and regenerate a new token. The old token will be invalidated.

### Q: My token expired, how do I get a new one?

**A:** Login to admin dashboard and generate a new token. Expired tokens cannot be used.

### Q: I'm locked out AND my token is expired/lost. Help!

**A:** You'll need to:
1. Stop the Charon container
2. Temporarily disable security in the configuration
3. Restart container and login
4. Generate new emergency token
5. Re-enable security

### Q: What happens when I use emergency reset?

**A:** ALL security modules are immediately disabled:
- ACL (Access Control Lists)
- WAF (Web Application Firewall)
- CrowdSec integration
- Rate limiting
- Admin IP whitelist

You can then re-enable them individually from the dashboard.

### Q: Is emergency reset secure?

**A:** Yes, if used properly:
- Token is cryptographically random (64+ bytes)
- Port 2020 is localhost-only (not exposed to internet)
- All usage is audit logged
- Token can have expiration policy (30/60/90 days)
- Requires both admin credentials AND the token

### Q: How often should I rotate my token?

**A:** We recommend 90 days (default). For high-security environments, use 30 or 60 days.

## Troubleshooting

### "401 Unauthorized"
- Your token is incorrect, expired, or revoked
- Regenerate a new token from admin dashboard

### "Connection refused"
- Emergency server is not running
- Check `CHARON_EMERGENCY_SERVER_ENABLED=true` in config

### "Wrong admin credentials"
- The Basic Auth uses your Charon admin username/password
- Default is `admin:changeme` (change in production!)

## Security Best Practices

1. ✅ Store token in password manager (1Password, Bitwarden, etc.)
2. ✅ Save usage commands WITH the token
3. ✅ Set expiration policy (don't use "Never")
4. ✅ Test token immediately after generation
5. ✅ Enable external notifications for expiration warnings
6. ❌ Never commit token to git
7. ❌ Never share token via email/Slack
8. ❌ Never expose port 2020 externally
```

**Security Documentation**:
```markdown
## docs/security.md additions:

### Emergency Access Port (2020)

⚠️ **CRITICAL**: The emergency server endpoint on port 2020 must NEVER be exposed externally.

**Configuration**:
- Port 2020 is bound to localhost only by default
- Emergency token must be at least 32 bytes of cryptographic randomness
- Token is redacted in all logs as `[EMERGENCY_TOKEN:xxxx...xxxx]`

**Security Controls**:
- Rate limiting: 3 attempts per minute per IP
- Audit logging: All access attempts logged with timestamp and source IP
- Token strength validation at startup

**Verification**:
```bash
# Port should NOT be exposed externally
docker port charon 2020  # Should return nothing in production

# Verify firewall blocks external access
netstat -tuln | grep 2020  # Should show 127.0.0.1:2020 only
```
```

**Deliverable**: Documentation updated with security warnings

### Task 3.4: Regression Prevention
**Priority**: CRITICAL - Prevent future misconfigurations
**Estimated Time**: 1 hour

**Actions**:

1. **Add Backend Startup Health Check**:
   ```go
   // backend/cmd/charon/main.go or equivalent
   func validateEmergencyConfig() {
       emergencyEnabled := os.Getenv("CHARON_EMERGENCY_SERVER_ENABLED") == "true"
       emergencyToken := os.Getenv("CHARON_EMERGENCY_TOKEN")

       if emergencyEnabled {
           if emergencyToken == "" || len(strings.TrimSpace(emergencyToken)) == 0 {
               log.Fatal("FATAL: CHARON_EMERGENCY_SERVER_ENABLED=true but CHARON_EMERGENCY_TOKEN is empty or whitespace")
           }
           if len(emergencyToken) < 32 {
               log.Warn("WARNING: CHARON_EMERGENCY_TOKEN is shorter than 32 bytes (weak security)")
           }
           // Log with redaction
           redacted := fmt.Sprintf("[EMERGENCY_TOKEN:%s...%s]",
               emergencyToken[:4], emergencyToken[len(emergencyToken)-4:])
           log.Info("Emergency server initialized with token: " + redacted)
       }
   }
   ```

2. **Add CI Health Check**:
   ```yaml
   # .github/workflows/e2e-tests.yml
   - name: Verify emergency token loaded
     run: |
       docker logs charon-e2e | grep "Emergency server initialized with token: \[REDACTED\]"
       if [ $? -ne 0 ]; then
         echo "ERROR: Emergency token not loaded!"
         docker logs charon-e2e | tail -50
         exit 1
       fi

       # Verify port 2020 NOT exposed externally
       docker port charon-e2e 2020 && echo "ERROR: Port 2020 exposed!" && exit 1 || true
   ```

3. **Add Integration Test in Backend**:
   ```go
   // backend/internal/emergency/server_test.go
   func TestEmergencyServerStartupValidation(t *testing.T) {
       tests := []struct {
           name          string
           enabled       string
           token         string
           expectPanic   bool
       }{
           {"enabled with valid token", "true", "a1b2c3d4e5f6...", false},
           {"enabled with empty token", "true", "", true},
           {"enabled with whitespace token", "true", "   ", true},
           {"disabled with empty token", "false", "", false},
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               os.Setenv("CHARON_EMERGENCY_SERVER_ENABLED", tt.enabled)
               os.Setenv("CHARON_EMERGENCY_TOKEN", tt.token)

               if tt.expectPanic {
                   defer func() {
                       if r := recover(); r == nil {
                           t.Errorf("Expected panic but got none")
                       }
                   }()
               }

               validateEmergencyConfig()
           })
       }
   }
   ```

4. **Add Playwright Pre-Test Check**:
   ```typescript
   // tests/globalSetup.ts - Add before emergency reset
   async function verifyEmergencyServerReady() {
       const exec = require('child_process').execSync;

       // Check emergency server is listening
       try {
           exec('docker exec charon-e2e netstat -tuln | grep ":2020 "');
       } catch (error) {
           throw new Error('Emergency server not listening on port 2020');
       }

       // Check logs confirm token loaded
       const logs = exec('docker logs charon-e2e 2>&1').toString();
       if (!logs.includes('Emergency server initialized')) {
           throw new Error('Emergency server did not initialize properly');
       }
   }
   ```

**Deliverable**: Fail-fast checks prevent silent misconfiguration in all environments

---

## 📍 PHASE 4: CrowdSec Integration (Optional)
**Priority**: LOW - Nice to have
**Estimated Time**: 4-6 hours
**Assignee**: DevOps + Backend_Dev
**Depends On**: Phase 3 complete

### Task 4.1: Add CrowdSec to Playwright Compose
**Update**: `.docker/compose/docker-compose.playwright-local.yml`

**Add CrowdSec service**:
```yaml
services:
  crowdsec:
    image: crowdsecurity/crowdsec:latest
    container_name: crowdsec-e2e
    environment:
      - COLLECTIONS=crowdsecurity/http-cve crowdsecurity/whitelist-good-actors
    volumes:
      - crowdsec-db:/var/lib/crowdsec/data
      - crowdsec-config:/etc/crowdsec
    networks:
      - default

volumes:
  crowdsec-db:
  crowdsec-config:
```

**Deliverable**: CrowdSec service in local compose file

### Task 4.2: Validate CrowdSec Decision Tests
**Run tests**:
```bash
npx playwright test tests/security/crowdsec-decisions.spec.ts
```

**Expected**: 12/12 tests pass (currently 12 skipped)

**Deliverable**: CrowdSec decision management tests passing

---

## Success Criteria

### Phase 0 (MUST COMPLETE)
- ✅ Clean environment rebuild with no cache
- ✅ Docker image build timestamp within last hour
- ✅ Binary version verified inside container
- ✅ Baseline logs and environment captured

### Phase 1 (MUST COMPLETE)
- ✅ Emergency token endpoint returns 200 with valid token
- ✅ Emergency token endpoint returns 401 with invalid token
- ✅ Emergency token endpoint returns 501 ONLY when env var unset/whitespace
- ✅ 4/4 emergency reset tests passing
- ✅ Emergency reset completes in <500ms (performance check)
- ✅ Token is redacted in all logs (no full token visible)
- ✅ Port 2020 is NOT exposed externally
- ✅ Rate limiting active (3 attempts/minute/IP)
- ✅ Audit logging captures all access attempts
- ✅ Global setup completes without warnings or errors
- ✅ Edge case tests pass (idempotency, concurrent access, Caddy failure)

### Phase 2 (MUST COMPLETE)
- ✅ Emergency token API endpoints functional (generate, status, revoke, update expiration)
- ✅ Emergency token stored as bcrypt hash in database
- ✅ Emergency endpoint validates DB token first, falls back to env var
- ✅ Backend tests for token generation, expiration, validation pass
- ✅ PATCH /api/v1/config endpoint exists and works
- ✅ Admin whitelist can be configured via API
- ✅ Security module enable endpoints functional
- ✅ Emergency token UI page fully functional
- ✅ Token generation shows plaintext ONCE with required confirmations
- ✅ Expiration warning banner appears at 14 days
- ✅ Notification system integrated for expiration alerts
- ✅ 0 "Failed to configure admin whitelist" warnings

### Phase 3 (MUST COMPLETE)
- ✅ ≥145/162 tests passing (90%+)
- ✅ Emergency token protocol: 7/7 passing (was 1/7)
- ✅ Admin whitelist tests: 3/3 passing (was 1/3)
- ✅ Emergency reset tests: 4/4 passing (was 0/4)
- ✅ Backend test coverage for emergency package: ≥85%
- ✅ E2E coverage for emergency flows: ≥80%
- ✅ No regressions in existing passing tests
- ✅ Fail-fast checks implemented (Task 3.4)
- ✅ CI health checks added
- ✅ Documentation updated with security warnings

### Phase 4 (OPTIONAL)
- ✅ CrowdSec service in local compose
- ✅ CrowdSec decision tests: 12/12 passing

---

## Risk Assessment

### CRITICAL SECURITY RISK
**Emergency endpoint on port 2020 must NEVER be exposed externally**

**Threat**: If port 2020 is accessible from the internet, attackers could disable all security modules using a stolen or brute-forced emergency token.

**Mitigation Required**:
1. ✅ Verify port 2020 is NOT in docker-compose port mappings for production
2. ✅ Add firewall rule to block external access to port 2020
3. ✅ Document in security.md: "Emergency server is localhost/internal-only"
4. ✅ Add startup check: Log WARNING if emergency endpoint is externally accessible
5. ✅ Add rate limiting: max 3 attempts per minute per IP
6. ✅ Add audit logging: timestamp, source IP, result for every call
7. ✅ Token must be at least 32 bytes of cryptographic randomness
8. ✅ Ensure test token is NEVER used in production

**Detection**:
```bash
# Check if port 2020 is exposed
docker port charon 2020  # Should return nothing for production

# Verify firewall
iptables -L INPUT -n | grep 2020  # Should show DROP rule for external

# Check in compose file
grep -A 5 "2020" .docker/compose/docker-compose.yml  # Should NOT map to 0.0.0.0
```

### High Risk
**Emergency token fix requires backend code changes**
- Risk: Breaking existing emergency functionality
- Mitigation: Add comprehensive logging, test thoroughly with edge cases
- Rollback: See detailed rollback procedure below

### Medium Risk
**New API endpoints may conflict with existing routes**
- Risk: Route collision or authentication issues
- Mitigation: Follow existing API patterns, use middleware consistently
- Rollback: Remove endpoint, update tests to skip

### Low Risk
**CrowdSec integration adds complexity**
- Risk: CrowdSec not available in all environments
- Mitigation: Keep as optional profile in compose file
- Rollback: Remove CrowdSec service, keep tests skipped

---

## Timeline Estimate

| Phase | Duration | Dependencies | Can Parallelize? |
|-------|----------|--------------|------------------|
| Phase 0 | 0.5 hours | None | No (must verify environment) |
| Phase 1 | 2-4 hours | Phase 0 | No (blocking) |
| Phase 2 | 5-8 hours | Phase 1 | Partially (Task 2.1-2.3 backend, Task 2.4 frontend) |
| Phase 3 | 2-3 hours | Phase 1 & 2 | No (validation + Task 3.4) |
| Phase 4 | 4-6 hours | Phase 3 | Yes (optional) |
| **Total** | **14-23 hours** | Sequential | Phase 4 can be async |

**Note**:
- Added 2-3 hours for security hardening (token redaction, rate limiting, audit logging) and regression prevention (Task 3.4)
- Added 2-3 hours for UI-based emergency token management with expiration policies (Task 2.4)

**Recommended Approach**:
- **Session 1** (8-10 hours): Phases 0-2 (environment setup, backend implementation, UI development)
- **Session 2** (2-3 hours): Phase 3 (validation, regression prevention, documentation)
- Defer Phase 4 (CrowdSec) to separate task

---

## Acceptance Test Plan

### Pre-Deployment Checklist
- [ ] All Phase 1 tasks complete
- [ ] Emergency token tests: 4/4 passing
- [ ] Emergency endpoint manual test: PASS
- [ ] All Phase 2 tasks complete
- [ ] API endpoint tests: PASS
- [ ] Security enforcement tests: ≥17/19 passing
- [ ] Full E2E suite: ≥145/162 passing (90%)
- [ ] No regressions in previously passing tests
- [ ] Documentation updated
- [ ] Changes committed to feature branch

### Post-Deployment Validation
- [ ] CI/CD E2E tests pass in GitHub Actions
- [ ] Manual smoke test on staging environment
- [ ] Emergency reset verified in production-like setup
- [ ] Admin whitelist configuration verified in UI

---

## Notes for Implementation

### Backend Code Search Commands
```bash
# Find emergency token environment variable loading
rg "CHARON_EMERGENCY_TOKEN" backend/

# Find emergency reset endpoint handler
rg "emergency.*reset" backend/ -A 10

# Find config API endpoints
rg "api/v1/config" backend/ -A 5

# Find security module enable endpoints
rg "security.*enable" backend/ -A 5
```

### Test Execution Commands
```bash
# Run specific test files
npx playwright test tests/security-enforcement/emergency-reset.spec.ts
npx playwright test tests/security-enforcement/emergency-token.spec.ts
npx playwright test tests/security-enforcement/zzz-admin-whitelist-blocking.spec.ts

# Run all security enforcement tests
npx playwright test tests/security-enforcement/

# Run with debug logging
DEBUG=charon:* npx playwright test tests/security-enforcement/
```

### Container Debug Commands
```bash
# Check emergency server is listening
docker exec charon-e2e netstat -tuln | grep 2020

# Check application logs
docker compose -f .docker/compose/docker-compose.playwright-local.yml logs -f charon-e2e

# Verify environment variables
docker exec charon-e2e env | grep CHARON_ | sort

# Test emergency endpoint directly
docker exec charon-e2e curl -X POST http://localhost:2020/emergency/security-reset \
  -u admin:changeme \
  -H "X-Emergency-Token: $(cat /proc/1/environ | tr '\0' '\n' | grep CHARON_EMERGENCY_TOKEN | cut -d= -f2)"
```

---

## Post-Deployment Monitoring (Phase 3.5)

**Metrics to track for 48 hours after deployment**:
- **Emergency endpoint error rate**: Should be 0% for valid tokens
- **Emergency reset execution time**: Should be <500ms consistently
- **Failed authentication attempts**: Audit log for suspicious activity
- **Test suite stability**: Compare pass rate over 10 consecutive runs
- **Port exposure checks**: Automated scanning for port 2020 external accessibility

**Alerting Configuration**:
```yaml
# Add to monitoring system
Alerts:
  - name: emergency_endpoint_misconfigured
    condition: emergency_endpoint returns 501 in E2E tests
    severity: critical
    action: Page oncall engineer

  - name: emergency_port_exposed
    condition: port 2020 accessible from external IP
    severity: critical
    action: Auto-disable emergency server, page security team

  - name: emergency_token_in_logs
    condition: full emergency token appears in logs (regex match)
    severity: high
    action: Rotate token immediately, alert security team

  - name: excessive_emergency_attempts
    condition: >10 failed auth attempts in 5 minutes
    severity: medium
    action: Log source IP, consider blocking
```

**Dashboard Metrics**:
- Emergency endpoint response time (p50, p95, p99)
- Emergency endpoint status code distribution
- Rate limit hit rate
- Audit log volume

---

## Artifacts to Preserve

**For post-mortem analysis and future reference**:

📁 **`test-results/emergency-fix/`**
- `baseline_logs.txt` - Logs before fix applied
- `baseline_env.txt` - Environment variables before fix
- `code_analysis.md` - Root cause analysis with file/line numbers
- `test_comparison.md` - Before/after test results side-by-side
- `security_audit.md` - Security review of emergency endpoint
- `edge_case_results.txt` - Results from all edge case tests
- `performance_metrics.json` - Emergency reset timing data

📁 **`docs/implementation/emergency_token_fix_COMPLETE.md`**
- Final implementation summary
- Code changes made with rationale
- Test results and coverage reports
- Lessons learned
- Recommendations for future work

---

## Related Documents
- [E2E Troubleshooting Guide](../troubleshooting/e2e-tests.md)
- [Emergency Token Implementation](../implementation/e2e_remediation_complete.md)
- [Admin Whitelist Test](../implementation/admin_whitelist_test_and_fix_COMPLETE.md)
- [Getting Started - Emergency Token Setup](../getting-started.md)
- [Security Documentation](../security.md)
- [Supply Chain Security](../SUPPLY_CHAIN_SECURITY_FIXES.md)

---

**Last Updated**: 2026-01-27 (Updated with UI-based token management)
**Status**: Phase 0 Complete - Ready for Phase 1
**Next Action**: Backend_Dev to begin Task 1.1 (Emergency Token Investigation)
**Estimated Total Time**: 14-23 hours (Phases 0-3 with UI enhancements)
**Major Enhancement**: UI-based emergency token management with GitHub PAT-style expiration policies
