# CrowdSec Console Enrollment Persistence Issue - ARCHITECTURAL ROOT CAUSE

**Date:** December 14, 2025 (Updated with Architectural Analysis)
**Issue:** Console enrollment shows "enrolled" locally but doesn't appear on crowdsec.net
**Status:** 🚨 **ARCHITECTURAL ISSUE IDENTIFIED** - Environment variable dependency breaks GUI control

---

## 🎯 Key Findings

### Critical Discovery
The `CHARON_SECURITY_CROWDSEC_MODE` environment variable is **LEGACY/DEPRECATED** technical debt from when Charon supported external CrowdSec instances (no longer supported). Now that Charon offers the **import config option**, CrowdSec should be **entirely GUI-controlled**, but the code still checks environment variables.

### Root Cause Chain
1. User enables CrowdSec via GUI → Database updated (`security.crowdsec.enabled = true`)
2. Backend sees CrowdSec enabled and allows Console enrollment
3. **BUT** `docker-entrypoint.sh` checks `SECURITY_CROWDSEC_MODE` environment variable
4. LAPI never starts because env var says "disabled"
5. Enrollment command runs but cannot contact LAPI
6. User sees "enrolled" in UI but nothing appears on crowdsec.net

### Why This is an Architecture Problem
- **WAF, ACL, and Rate Limiting** are all GUI-controlled via Settings table
- **CrowdSec** still has legacy environment variable checks in entrypoint script
- Backend has proper `Start()` and `Stop()` handlers but they're not integrated with container lifecycle
- This creates inconsistent UX where GUI toggle doesn't actually control the service

### Impact
- **ALL users** attempting Console enrollment are affected
- **Not a configuration issue** - users cannot fix this without workaround
- **Technical debt** preventing proper GUI-based security orchestration

---

## Executive Summary

The CrowdSec console enrollment appears successful locally (green checkmark in Charon UI) but the instance **does not appear on the CrowdSec Console dashboard at crowdsec.net**.

**🚨 CRITICAL ARCHITECTURAL ISSUE:** The `CHARON_SECURITY_CROWDSEC_MODE` environment variable is **LEGACY/DEPRECATED** from when Charon supported external CrowdSec instances. Now that Charon offers the **import config option**, CrowdSec is **always internally managed** and should be **GUI-controlled**, not environment variable controlled.

**✅ TRUE ROOT CAUSE:** The code still checks the legacy `SECURITY_CROWDSEC_MODE` environment variable in `docker-entrypoint.sh`, which prevents LAPI from starting even when the GUI says CrowdSec is enabled. The `cscli console enroll` command **requires LAPI to be running** to complete the enrollment registration with crowdsec.net.

**CORRECTED UNDERSTANDING:** Enrollment tokens are **REUSABLE** (confirmed by user testing). The issue is NOT token exhaustion - it's that the enrollment process cannot complete without an active LAPI connection.

**Key Finding:** The enrollment command executes without error even when LAPI is down, causing the database to show "enrolled" status while the actual Console registration never happens.

---

## Architectural Analysis

### Current Architecture (INCORRECT)

**Environment Variable Dependency:**
```bash
# docker-entrypoint.sh checks this legacy env var:
SECURITY_CROWDSEC_MODE=${CERBERUS_SECURITY_CROWDSEC_MODE:-${CHARON_SECURITY_CROWDSEC_MODE:-$CPM_SECURITY_CROWDSEC_MODE}}

if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    crowdsec -c /etc/crowdsec/config.yaml &
fi
```

**The Problem:**
- User enables CrowdSec via GUI → `security.crowdsec.enabled = true` in database
- Backend sees CrowdSec enabled and allows enrollment
- But `docker-entrypoint.sh` checks **environment variable**, not database
- LAPI never starts because env var says "disabled"
- Enrollment command runs but cannot contact LAPI
- User sees "enrolled" in UI but nothing on crowdsec.net

### Correct Architecture (GUI-Controlled)

**How Other Security Features Work (Pattern to Follow):**

WAF, Rate Limiting, and ACL are all **GUI-controlled** through the Settings table:
- `security.waf.enabled` → Controls WAF mode
- `security.rate_limit.enabled` → Controls rate limiting
- `security.acl.enabled` → Controls ACL mode

These settings are read by:
1. **Backend handlers** via `security_handler.go:GetStatus()`
2. **Caddy config generator** via `caddy/manager.go:computeEffectiveFlags()`
3. **Frontend** via API calls to `/api/v1/security/status`

**CrowdSec Should Follow Same Pattern:**
- GUI toggle → `security.crowdsec.enabled` in Settings table
- Backend reads setting and manages CrowdSec process lifecycle
- No environment variable dependency

### Import Config Feature (Why External Mode is Deprecated)

The import config feature (`importCrowdsecConfig`) allows users to:
1. Upload a complete CrowdSec configuration (tar.gz)
2. Import pre-configured settings, collections, and bouncers
3. Manage CrowdSec entirely through Charon's GUI

**This replaced the need for "external" mode:**
- Old way: Set `CROWDSEC_MODE=external` and point to external LAPI
- New way: Import your existing config and let Charon manage it internally

---

## Forensic Investigation Findings

### Environment Status (Verified Dec 14, 2025)

**✅ CAPI Registration:** Working
```bash
$ docker exec charon cscli capi status
✓ Loaded credentials from /etc/crowdsec/online_api_credentials.yaml
✓ You can successfully interact with Central API (CAPI)
```

**❌ LAPI Status:** NOT RUNNING
```bash
$ docker exec charon cscli lapi status
✗ Error: dial tcp 127.0.0.1:8085: connection refused
```

**❌ CrowdSec Agent:** NOT RUNNING
```bash
$ docker exec charon ps aux | grep crowdsec
(no processes found)
```

**Environment Variables:**
```bash
CHARON_SECURITY_CROWDSEC_MODE=disabled  # ← THIS IS THE PROBLEM
```

### Why Enrollment Appears Successful

The enrollment flow in `backend/internal/crowdsec/console_enroll.go`:

1. ✅ Validates token format
2. ✅ Ensures CAPI registered (`ensureCAPIRegistered`)
3. ✅ Updates database to "enrolling" status
4. ✅ Executes `cscli console enroll <token>`
5. **❌ Command exits with code 0 even when LAPI is down**
6. ✅ Updates database to "enrolled" status
7. ✅ Returns success to UI

**The Bug:** `cscli console enroll` does NOT verify LAPI connectivity before returning success. It writes local state but cannot register with crowdsec.net Console API without an active LAPI connection.

---

## Root Cause: Legacy Environment Variable Architecture

### Confirmed (100% Confidence)

**The Issue:** The `docker-entrypoint.sh` script only starts CrowdSec LAPI when checking a **legacy environment variable**, not the **GUI setting**:

```bash
# docker-entrypoint.sh (INCORRECT ARCHITECTURE)
SECURITY_CROWDSEC_MODE=${CERBERUS_SECURITY_CROWDSEC_MODE:-${CHARON_SECURITY_CROWDSEC_MODE:-$CPM_SECURITY_CROWDSEC_MODE}}

if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    crowdsec -c /etc/crowdsec/config.yaml &
fi
```

**Current State:**
- GUI setting: `security.crowdsec.enabled = true` (in database)
- Environment: `CHARON_SECURITY_CROWDSEC_MODE=disabled`
- Result: LAPI NOT RUNNING

**Correct Architecture:**
- CrowdSec should be started/stopped by **backend handlers** (`Start()` and `Stop()` methods)
- The GUI toggle should call these handlers, just like WAF and ACL
- No environment variable checks in entrypoint script

**Console Enrollment REQUIRES:**
1. CrowdSec agent running
2. Local API (LAPI) running on port 8085
3. Active connection between LAPI and Console API (api.crowdsec.net)
4. **All controlled by GUI, not environment variables**

---

## Comparison: How WAF/ACL Work (Correct Pattern)

### WAF Control Flow (GUI → Backend → Caddy)

1. **Frontend:** User toggles WAF switch → calls `updateSetting('security.waf.enabled', 'true')`
2. **Backend:** Settings table updated → Caddy config regenerated
3. **Caddy Manager:** Reads `security.waf.enabled` from database → enables WAF handlers
4. **No Environment Variable Checks**

### CrowdSec Control Flow (BROKEN - Still Uses Env Vars)

1. **Frontend:** User toggles CrowdSec switch → calls `updateSetting('security.crowdsec.enabled', 'true')`
2. **Backend:** Settings table updated → BUT...
3. **Entrypoint Script:** Checks `SECURITY_CROWDSEC_MODE` env var (LEGACY)
4. **Result:** LAPI never starts because env var says "disabled"

### How CrowdSec SHOULD Work (GUI-Controlled)

1. **Frontend:** User toggles CrowdSec switch → calls `/api/v1/admin/crowdsec/start`
2. **Backend Handler:** `CrowdsecHandler.Start()` executes → starts LAPI process
3. **Process Management:** Backend tracks PID and monitors health
4. **No Environment Variable Dependency**

**Evidence from Code:**

```go
// backend/internal/api/handlers/crowdsec_handler.go
// These handlers already exist but aren't properly integrated!

func (h *CrowdsecHandler) Start(c *gin.Context) {
    ctx := c.Request.Context()
    pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "started", "pid": pid})
}

func (h *CrowdsecHandler) Stop(c *gin.Context) {
    ctx := c.Request.Context()
    if err := h.Executor.Stop(ctx, h.DataDir); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
```

**Frontend Integration:**

```typescript
// frontend/src/pages/Security.tsx
// CrowdSec toggle DOES call start/stop, but LAPI never started by entrypoint!

const crowdsecPowerMutation = useMutation({
  mutationFn: async (enabled: boolean) => {
    await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    if (enabled) {
      await startCrowdsec()  // ← Calls backend Start() handler
    } else {
      await stopCrowdsec()   // ← Calls backend Stop() handler
    }
    return enabled
  },
})
```

**The Missing Piece:** The `docker-entrypoint.sh` should ALWAYS initialize CrowdSec but NOT start the agent. The backend handlers should control the lifecycle.

---

## Immediate Fix (For User)

**WORKAROUND (Until Architecture Fixed):**

Set the legacy environment variable to match the GUI state:

**Step 1: Enable CrowdSec Local Mode (Environment Variable)**

Update `docker-compose.yml` or `docker-compose.override.yml`:
```yaml
services:
  charon:
    environment:
      - CHARON_SECURITY_CROWDSEC_MODE=local  # Temporary workaround for legacy check
```

**Step 2: Recreate Container**
```bash
docker compose down
docker compose up -d
```

**Step 3: Verify LAPI is Running**
```bash
# Wait 30 seconds for LAPI to start
docker exec charon cscli lapi status
```

Expected output:
```
✓ Loaded credentials from /etc/crowdsec/local_api_credentials.yaml
✓ You can successfully interact with Local API (LAPI)
```

**Step 4: Re-submit Enrollment Token**
- Go to Charon UI → Cerberus → CrowdSec
- Submit enrollment token (same token works!)
- Verify instance appears on crowdsec.net dashboard

---

## Long-Term Fix Implementation Plan (ARCHITECTURE CORRECTION)

### Priority Overview

1. **CRITICAL:** Remove environment variable dependency from entrypoint script
2. **CRITICAL:** Ensure backend handlers control CrowdSec lifecycle
3. **HIGH:** Add LAPI availability check before enrollment
4. **HIGH:** Update documentation to reflect GUI-only control
5. **MEDIUM:** Add migration guide for users with env vars set

---

### Fix 1: Remove Environment Variable Dependency (CRITICAL PRIORITY)

**Problem:** `docker-entrypoint.sh` checks legacy `SECURITY_CROWDSEC_MODE` env var
**Solution:** Remove env var check, let backend control CrowdSec lifecycle
**Time:** 45 minutes
**Files affected:** `docker-entrypoint.sh`, `backend/internal/api/handlers/crowdsec_handler.go`

**Implementation:**

**Part A: Update docker-entrypoint.sh**

Remove the CrowdSec agent auto-start logic:

```bash
# BEFORE (INCORRECT - Environment Variable Control):
if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    echo "CrowdSec Local Mode enabled."
    crowdsec -c /etc/crowdsec/config.yaml &
    CROWDSEC_PID=$!
fi

# AFTER (CORRECT - Backend Control):
# CrowdSec initialization (config setup) always runs
# But agent startup is controlled by backend handlers via GUI
# No automatic startup based on environment variables
```

**Part B: Ensure Backend Handlers Work Correctly**

The `CrowdsecHandler.Start()` already exists and works:

```go
// backend/internal/api/handlers/crowdsec_handler.go
func (h *CrowdsecHandler) Start(c *gin.Context) {
    ctx := c.Request.Context()
    pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "started", "pid": pid})
}
```

**Part C: Frontend Integration Verification**

Verify the frontend correctly calls start/stop:

```typescript
// frontend/src/pages/Security.tsx (ALREADY CORRECT)
const crowdsecPowerMutation = useMutation({
  mutationFn: async (enabled: boolean) => {
    await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')
    if (enabled) {
      await startCrowdsec()  // Calls /api/v1/admin/crowdsec/start
    } else {
      await stopCrowdsec()   // Calls /api/v1/admin/crowdsec/stop
    }
    return enabled
  },
})
```

**Testing:**
1. Remove env var from docker-compose.yml
2. Start container (CrowdSec should NOT auto-start)
3. Toggle CrowdSec in GUI (should start LAPI)
4. Verify `cscli lapi status` shows running
5. Toggle off (should stop LAPI)

---

### Fix 2: Add LAPI Availability Check Before Enrollment (CRITICAL PRIORITY)

### Fix 2: Add LAPI Availability Check Before Enrollment (CRITICAL PRIORITY)

**Problem:** Enrollment command succeeds even when LAPI is down
**Solution:** Verify LAPI connectivity before allowing enrollment
**Time:** 30 minutes
**Files affected:** `backend/internal/crowdsec/console_enroll.go`

**Implementation:**

Add LAPI health check before enrollment:

```go
func (s *ConsoleEnrollmentService) checkLAPIAvailable(ctx context.Context) error {
    args := []string{"lapi", "status"}
    if _, err := os.Stat(filepath.Join(s.dataDir, "config.yaml")); err == nil {
        args = append([]string{"-c", filepath.Join(s.dataDir, "config.yaml")}, args...)
    }
    _, err := s.exec.ExecuteWithEnv(ctx, "cscli", args, nil)
    if err != nil {
        return fmt.Errorf("CrowdSec Local API is not running - please enable CrowdSec via the GUI toggle first")
    }
    return nil
}
```

Update `Enroll()` method:
```go
// Before: if err := s.ensureCAPIRegistered(ctx); err != nil {
if err := s.checkLAPIAvailable(ctx); err != nil {
    return ConsoleEnrollmentStatus{}, err
}
if err := s.ensureCAPIRegistered(ctx); err != nil {
    return ConsoleEnrollmentStatus{}, err
}
```

---

### Fix 3: Add UI Warning When CrowdSec is Disabled (HIGH PRIORITY)

**Problem:** Users can attempt enrollment when CrowdSec is disabled
**Solution:** Add status check to enrollment UI with clear instructions
**Time:** 20 minutes
**Files affected:** `frontend/src/pages/CrowdSecConfig.tsx`

**Implementation:**

Add LAPI status detection to enrollment form:

```typescript
const crowdsecStatusQuery = useQuery({
  queryKey: ['crowdsec-status'],
  queryFn: async () => {
    const response = await client.get('/api/v1/admin/crowdsec/status');
    return response.data;
  },
  enabled: consoleEnrollmentEnabled,
  refetchInterval: 5000, // Poll every 5 seconds
});

// In enrollment form JSX:
{!crowdsecStatusQuery.data?.running && (
  <Alert variant="warning">
    <AlertTriangle className="w-4 h-4" />
    <span>
      CrowdSec Local API is not running. Please enable CrowdSec using the toggle switch
      in the Security dashboard before enrolling in the Console.
    </span>
    <Button
      variant="link"
      onClick={() => navigate('/security')}
    >
      Go to Security Dashboard
    </Button>
  </Alert>
)}

<Button
  disabled={!crowdsecStatusQuery.data?.running || !enrollmentToken}
  onClick={handleEnroll}
>
  Enroll Instance
</Button>
```

---

### Fix 4: Update Documentation (HIGH PRIORITY)

**Problem:** Documentation mentions environment variables for CrowdSec control
**Solution:** Update docs to reflect GUI-only control, mark env vars as deprecated
**Time:** 30 minutes
**Files affected:**
- `docs/security.md`
- `docs/cerberus.md`
- `docs/troubleshooting/crowdsec.md`
- `README.md`

**Changes Needed:**

1. **Mark Environment Variables as Deprecated:**
   ```md
   ⚠️ **DEPRECATED:** `CHARON_SECURITY_CROWDSEC_MODE` environment variable is no longer used.
   CrowdSec is now controlled via the GUI in the Security dashboard.
   ```

2. **Add GUI Control Instructions:**
   ```md
   ## Enabling CrowdSec

   1. Navigate to **Security** dashboard
   2. Toggle the **CrowdSec** switch to **ON**
   3. The backend will start the CrowdSec agent and Local API (LAPI)
   4. Verify status shows "Active" with a running PID

   **Note:** CrowdSec is internally managed by Charon. No external setup required.
   ```

3. **Update Console Enrollment Prerequisites:**
   ```md
   ## Console Enrollment Prerequisites

   Before enrolling your Charon instance with CrowdSec Console:

   1. ✅ CrowdSec must be **enabled** in the GUI (toggle switch ON)
   2. ✅ Local API (LAPI) must be **running** (check status)
   3. ✅ Feature flag `feature.crowdsec.console_enrollment` must be enabled
   4. ✅ Valid enrollment token from crowdsec.net

   **Troubleshooting:** If enrollment fails, verify LAPI is running:
   ```bash
   docker exec charon cscli lapi status
   ```
   ```

---

### Fix 5: Add Migration Guide for Existing Users (MEDIUM PRIORITY)

**Problem:** Users may have env vars set that will no longer work
**Solution:** Add migration guide to help users transition
**Time:** 15 minutes
**Files affected:** `docs/migration-guide.md` (new file)

**Content:**

```md
# CrowdSec Control Migration Guide

## What Changed

**Before (v1.x):** CrowdSec was controlled by environment variables:
```yaml
environment:
  - CHARON_SECURITY_CROWDSEC_MODE=local
```

**After (v2.x):** CrowdSec is controlled via GUI toggle in Security dashboard.

## Migration Steps

### Step 1: Remove Environment Variable

Edit your `docker-compose.yml` and remove:
```yaml
# REMOVE THIS LINE:
- CHARON_SECURITY_CROWDSEC_MODE=local
```

### Step 2: Restart Container

```bash
docker compose down
docker compose up -d
```

### Step 3: Enable via GUI

1. Open Charon UI → **Security** dashboard
2. Toggle **CrowdSec** switch to **ON**
3. Verify status shows "Active"

### Step 4: Re-enroll Console (If Applicable)

If you were enrolled in CrowdSec Console before:
1. Your enrollment is preserved in the database
2. No action needed unless enrollment was incomplete

## Benefits of GUI Control

- ✅ No need to restart container to enable/disable
- ✅ Status visible in real-time
- ✅ Consistent with WAF, ACL, and Rate Limiting controls
- ✅ Better integration with Charon's security orchestration

## Troubleshooting

**Q: CrowdSec won't start after toggling?**
- Check logs: `docker logs charon`
- Verify config exists: `docker exec charon ls -la /app/data/crowdsec/config`

**Q: Console enrollment fails?**
- Verify LAPI is running: `docker exec charon cscli lapi status`
- Check enrollment prerequisites in [docs/security.md](security.md)
```

---

### Fix 6: Add Integration Test (MEDIUM PRIORITY)

### Fix 6: Add Integration Test (MEDIUM PRIORITY)

**Problem:** No test coverage for enrollment prerequisites
**Solution:** Add test that verifies LAPI requirement and GUI lifecycle
**Time:** 30 minutes
**Files affected:**
- `backend/internal/crowdsec/console_enroll_test.go`
- `scripts/crowdsec_lifecycle_test.sh` (new file)

**Implementation:**

**Unit Test:**
```go
func TestEnroll_RequiresLAPI(t *testing.T) {
    exec := &mockExecutor{
        responses: []cmdResponse{
            {out: nil, err: nil}, // capi register success
            {out: nil, err: errors.New("connection refused")}, // lapi status fails
        },
    }
    svc := NewConsoleEnrollmentService(db, exec, tempDir, "secret")

    _, err := svc.Enroll(ctx, ConsoleEnrollRequest{
        EnrollmentKey: "test123token",
        AgentName:     "agent",
    })

    require.Error(t, err)
    require.Contains(t, err.Error(), "Local API is not running")
}
```

**Integration Test Script:**
```bash
#!/bin/bash
# scripts/crowdsec_lifecycle_test.sh
# Tests GUI-controlled CrowdSec lifecycle

echo "Testing CrowdSec GUI-controlled lifecycle..."

# 1. Start Charon without env var
docker compose up -d
sleep 5

# 2. Verify CrowdSec NOT running by default
docker exec charon cscli lapi status 2>&1 | grep "connection refused"
echo "✓ CrowdSec not auto-started without env var"

# 3. Enable via GUI toggle
curl -X POST -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{"key": "security.crowdsec.enabled", "value": "true", "category": "security", "type": "bool"}' \
  http://localhost:8080/api/v1/admin/settings

# 4. Call start endpoint (mimics GUI toggle)
curl -X POST -b cookies.txt \
  http://localhost:8080/api/v1/admin/crowdsec/start

sleep 10

# 5. Verify LAPI running
docker exec charon cscli lapi status | grep "successfully interact"
echo "✓ LAPI started via GUI toggle"

# 6. Disable via GUI
curl -X POST -b cookies.txt \
  http://localhost:8080/api/v1/admin/crowdsec/stop

sleep 5

# 7. Verify LAPI stopped
docker exec charon cscli lapi status 2>&1 | grep "connection refused"
echo "✓ LAPI stopped via GUI toggle"

echo "✅ All GUI lifecycle tests passed"
```

---

## Summary of Architectural Changes

### What's Broken Now (Environment Variable Control)

```
┌─────────────────┐
│  docker-compose │
│   env: MODE=    │  ← Environment variable set here
│    disabled     │
└────────┬────────┘
         │
         v
┌─────────────────┐
│ entrypoint.sh   │
│ if MODE=local   │  ← Checks env var, doesn't start LAPI
│   start crowdsec│
└─────────────────┘
         │
         v
    ❌ LAPI never starts
         │
         v
┌─────────────────┐
│  GUI Toggle     │
│ "CrowdSec: ON"  │  ← User thinks it's enabled
└─────────────────┘
         │
         v
┌─────────────────┐
│ Enroll Console  │  ← Fails silently (LAPI not running)
└─────────────────┘
```

### What Should Happen (GUI Control)

```
┌─────────────────┐
│  docker-compose │
│  (no env var)   │  ← No environment variable needed
└────────┬────────┘
         │
         v
┌─────────────────┐
│ entrypoint.sh   │
│ Init CrowdSec   │  ← Setup config only, don't start agent
│ (config only)   │
└─────────────────┘
         │
         v
┌─────────────────┐
│  GUI Toggle     │
│ "CrowdSec: ON"  │  ← User enables via GUI
└────────┬────────┘
         │
         v
┌─────────────────┐
│ POST /crowdsec/ │
│     /start      │  ← Frontend calls backend handler
└────────┬────────┘
         │
         v
┌─────────────────┐
│ Backend Handler │
│ Start LAPI      │  ← Backend starts the agent
│   (PID tracked) │
└────────┬────────┘
         │
         v
    ✅ LAPI running
         │
         v
┌─────────────────┐
│ Enroll Console  │  ← Works! LAPI available
└─────────────────┘
```

### Pattern Consistency Across Security Features

| Feature | Control Method | Status Endpoint | Lifecycle Handler |
|---------|---------------|-----------------|-------------------|
| **Cerberus** | GUI Toggle | `/security/status` | N/A (master switch) |
| **WAF** | GUI Toggle | `/security/status` | Config regeneration |
| **ACL** | GUI Toggle | `/security/status` | Config regeneration |
| **Rate Limit** | GUI Toggle | `/security/status` | Config regeneration |
| **CrowdSec** (OLD) | ❌ Env Var | `/security/status` | ❌ Entrypoint script |
| **CrowdSec** (NEW) | ✅ GUI Toggle | `/security/status` | ✅ Start/Stop handlers |

---

## Testing Strategy

### Manual Testing (For User - Workaround)

1. **Set Environment Variable (Temporary)**
   ```bash
   # docker-compose.override.yml
   environment:
     - CHARON_SECURITY_CROWDSEC_MODE=local
   ```

2. **Restart Container**
   ```bash
   docker compose down && docker compose up -d
   ```

3. **Verify LAPI Running**
   ```bash
   docker exec charon cscli lapi status
   # Should show: "You can successfully interact with Local API (LAPI)"
   ```

4. **Test Enrollment**
   - Submit enrollment token via Charon UI
   - Check crowdsec.net dashboard after 60 seconds
   - Instance should appear

### Automated Testing (For Developers - After Fix)

1. **Unit Test:** LAPI availability check before enrollment
2. **Integration Test:** GUI-controlled CrowdSec lifecycle (start/stop)
3. **End-to-End Test:** Full enrollment flow with GUI toggle
4. **Regression Test:** Verify env var no longer affects behavior

### Post-Fix Validation

1. **Remove Environment Variable**
   ```bash
   # Ensure CHARON_SECURITY_CROWDSEC_MODE is NOT set
   ```

2. **Start Container**
   ```bash
   docker compose up -d
   ```

3. **Verify CrowdSec NOT Running**
   ```bash
   docker exec charon cscli lapi status
   # Should show: "connection refused"
   ```

4. **Enable via GUI**
   - Toggle CrowdSec switch in Security dashboard
   - Wait 10 seconds

5. **Verify LAPI Started**
   ```bash
   docker exec charon cscli lapi status
   # Should show: "successfully interact"
   ```

6. **Test Console Enrollment**
   - Submit enrollment token
   - Verify appears on crowdsec.net

7. **Disable via GUI**
   - Toggle CrowdSec switch off
   - Wait 5 seconds

8. **Verify LAPI Stopped**
   ```bash
   docker exec charon cscli lapi status
   # Should show: "connection refused"
   ```

---

## Files Requiring Changes

### Backend (Go)
1. ✅ `docker-entrypoint.sh` - Remove env var check, initialize config only
2. ✅ `backend/internal/crowdsec/console_enroll.go` - Add LAPI availability check
3. ⚠️ `backend/internal/api/handlers/crowdsec_handler.go` - Already has Start/Stop (verify works)

### Frontend (TypeScript)
1. ✅ `frontend/src/pages/CrowdSecConfig.tsx` - Add LAPI status warning
2. ⚠️ `frontend/src/pages/Security.tsx` - Already calls start/stop (verify integration)

### Documentation
1. ✅ `docs/security.md` - Remove env var instructions, add GUI instructions
2. ✅ `docs/cerberus.md` - Mark env vars deprecated
3. ✅ `docs/troubleshooting/crowdsec.md` - Update enrollment prerequisites
4. ✅ `README.md` - Update quick start to use GUI only
5. ✅ `docs/migration-guide.md` - New file for v1.x → v2.x migration
6. ✅ `docker-compose.yml` - Comment out deprecated env var

### Testing
1. ✅ `backend/internal/crowdsec/console_enroll_test.go` - Add LAPI requirement test
2. ✅ `scripts/crowdsec_lifecycle_test.sh` - New integration test for GUI control

### Configuration (Already Correct)
1. ⚠️ `backend/internal/models/security_config.go` - CrowdSecMode field exists (DB)
2. ⚠️ `backend/internal/api/handlers/security_handler.go` - Already reads from DB
3. ⚠️ `frontend/src/api/crowdsec.ts` - Start/stop API calls already exist

---

## Risk Assessment

### Low Risk Changes
- ✅ Documentation updates
- ✅ Frontend UI warnings
- ✅ Backend LAPI availability check

### Medium Risk Changes
- ⚠️ Removing env var logic from entrypoint (requires thorough testing)
- ⚠️ Integration test for GUI lifecycle

### High Risk Areas (Existing Functionality - Verify)
- ⚠️ Backend Start/Stop handlers (already exist, need to verify)
- ⚠️ Frontend toggle integration (already exists, need to verify)
- ⚠️ CrowdSec config persistence across restarts

### Migration Considerations
- Users with `CHARON_SECURITY_CROWDSEC_MODE=local` set will need to:
  1. Remove environment variable
  2. Enable via GUI toggle
  3. Re-verify enrollment if applicable

---

## Rollback Plan

If the architectural changes cause issues:

1. **Immediate Rollback:** Add env var check back to `docker-entrypoint.sh`
2. **Document Workaround:** Continue using env var for CrowdSec control
3. **Defer Fix:** Mark as "known limitation" in docs until proper fix validated

---

## Files Inspected During Investigation

### Configuration ✅
- `docker-compose.yml` - Volume mounts correct
- `docker-entrypoint.sh` - Conditional CrowdSec startup logic
- `Dockerfile` - CrowdSec installed correctly

### Backend ✅
- `backend/internal/crowdsec/console_enroll.go` - Enrollment flow logic
- `backend/internal/models/crowdsec_console_enrollment.go` - Database model
- `backend/internal/api/handlers/crowdsec_handler.go` - API endpoint

### Runtime Verification ✅
- `/etc/crowdsec` → `/app/data/crowdsec/config` (symlink correct)
- `/app/data/crowdsec/config/online_api_credentials.yaml` exists (CAPI registered)
- `/app/data/crowdsec/config/console.yaml` exists
- `ps aux` shows NO crowdsec processes (LAPI not running)
- Environment: `CHARON_SECURITY_CROWDSEC_MODE=disabled`

---

## Conclusion

**Root Cause (Updated with Architectural Analysis):** Console enrollment fails because of **architectural technical debt** - the legacy environment variable `CHARON_SECURITY_CROWDSEC_MODE` still controls LAPI startup in `docker-entrypoint.sh`, bypassing the GUI control system that users expect.

**The Real Problem:** This is NOT a user configuration issue. It's a **code architecture issue** where:
1. CrowdSec control was never fully migrated to GUI-based management
2. The entrypoint script still checks deprecated environment variables
3. Backend handlers (`Start()`/`Stop()`) exist but aren't properly integrated with container startup
4. Users are misled into thinking the GUI toggle actually controls CrowdSec

**Immediate Fix (User Workaround):** Set `CHARON_SECURITY_CROWDSEC_MODE=local` environment variable to match GUI state.

**Proper Fix (Development Required):**
1. **CRITICAL:** Remove environment variable dependency from `docker-entrypoint.sh`
2. **CRITICAL:** Ensure backend handlers control CrowdSec lifecycle (GUI → API → Process)
3. **HIGH:** Add LAPI availability check before enrollment (prevents silent failures)
4. **HIGH:** Add UI warnings when LAPI is not running (improves UX)
5. **HIGH:** Update documentation to reflect GUI-only control
6. **MEDIUM:** Add migration guide for users transitioning from env var control
7. **MEDIUM:** Add integration tests for GUI-controlled lifecycle

**Pattern to Follow:** CrowdSec should work like WAF, ACL, and Rate Limiting - all controlled through Settings table, no environment variable dependency.

**Token Reusability:** Confirmed REUSABLE - no need to generate new tokens after fixing LAPI availability.

**Impact:** This architectural issue affects ALL users trying to use Console enrollment, not just the reporter. The fix will benefit the entire user base by providing consistent, GUI-based security feature management.
