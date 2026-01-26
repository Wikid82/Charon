# CrowdSec Integration Issues - Hotfix Plan

**Date:** December 14, 2025
**Priority:** HOTFIX - Critical
**Status:** Investigation Complete, Ready for Implementation

## Executive Summary

Three critical issues have been identified in the CrowdSec integration that prevent proper operation:

1. **CrowdSec process not actually running** - Message displays but process isn't started
2. **Toggle state management broken** - CrowdSec toggle on Cerberus Dashboard won't turn off
3. **Security log viewer shows wrong logs** - Displays Plex/application logs instead of security logs

## Investigation Findings

### Container Status

```bash
Container: charon (1cc717562976)
Status: Up 4 hours (healthy)
Processes Running:
  - PID 1: /bin/sh /docker-entrypoint.sh
  - PID 31: caddy run --config /config/caddy.json
  - PID 43: /usr/local/bin/dlv exec /app/charon (debugger)
  - PID 52: /app/charon (main process)

CrowdSec Process: NOT RUNNING ❌
No PID file found at: /app/data/crowdsec/crowdsec.pid
```

### Issue #1: CrowdSec Not Running

**Root Cause:**

- The error message "CrowdSec is not running" is **accurate**
- `crowdsec` binary process is not executing in the container
- PID file `/app/data/crowdsec/crowdsec.pid` does not exist
- Process detection in `crowdsec_exec.go:Status()` correctly returns `running=false`

**Code Path:**

```
backend/internal/api/handlers/crowdsec_exec.go:85
├── Status() checks PID file at: filepath.Join(configDir, "crowdsec.pid")
├── PID file missing → returns (running=false, pid=0, err=nil)
└── Frontend displays: "CrowdSec is not running"
```

**Why CrowdSec Isn't Starting:**

1. `ReconcileCrowdSecOnStartup()` runs at container boot (routes.go:360)
2. Checks `SecurityConfig` table for `crowdsec_mode = "local"`
3. **BUT**: The mode might not be set to "local" or the process start is failing silently
4. No error logs visible in container logs about CrowdSec startup failures

**Files Involved:**

- `backend/internal/services/crowdsec_startup.go` - Reconciliation logic
- `backend/internal/api/handlers/crowdsec_exec.go` - Process executor
- `backend/internal/api/handlers/crowdsec_handler.go` - Status endpoint

---

### Issue #2: Toggle Won't Turn Off

**Root Cause:**
Frontend state management has optimistic updates that don't properly reconcile with backend state.

**Code Path:**

```typescript
frontend/src/pages/Security.tsx:94-113 (crowdsecPowerMutation)
├── onMutate: Optimistically sets crowdsec.enabled = new value
├── mutationFn: Calls updateSetting() then startCrowdsec() or stopCrowdsec()
├── onError: Reverts optimistic update but may not fully sync
└── onSuccess: Calls fetchCrowdsecStatus() but state may be stale
```

**The Problem:**

```typescript
// Optimistic update sets enabled immediately
queryClient.setQueryData(['security-status'], (old) => {
  copy.crowdsec = { ...copy.crowdsec, enabled } // ← State updated BEFORE API call
})

// If API fails or times out, toggle appears stuck
```

**Why Toggle Appears Stuck:**

1. User clicks toggle → Frontend immediately updates UI to "enabled"
2. Backend API is called to start CrowdSec
3. CrowdSec process fails to start (see Issue #1)
4. API returns success (because the *setting* was updated)
5. Frontend thinks CrowdSec is enabled, but `Status()` API says `running=false`
6. Toggle now in inconsistent state - shows "on" but status says "not running"

**Files Involved:**

- `frontend/src/pages/Security.tsx:94-136` - Toggle mutation logic
- `frontend/src/pages/CrowdSecConfig.tsx:105` - Status check
- `backend/internal/api/handlers/security_handler.go:60-175` - GetStatus priority chain

---

### Issue #3: Security Log Viewer Shows Wrong Logs

**Root Cause:**
The `LiveLogViewer` component connects to the correct `/api/v1/cerberus/logs/ws` endpoint, but the `LogWatcher` service is reading from `/var/log/caddy/access.log` which may not exist or may contain the wrong logs.

**Code Path:**

```
frontend/src/pages/Security.tsx:411
├── <LiveLogViewer mode="security" securityFilters={{}} />
└── Connects to: ws://localhost:8080/api/v1/cerberus/logs/ws

backend/internal/api/routes/routes.go:362-390
├── LogWatcher initialized with: accessLogPath = "/var/log/caddy/access.log"
├── File exists check: Creates empty file if missing
└── Starts tailing: services.LogWatcher.tailFile()

backend/internal/services/log_watcher.go:139-186
├── Opens /var/log/caddy/access.log
├── Seeks to end of file
└── Reads new lines, parses as Caddy JSON logs
```

**The Problem:**
The log file path `/var/log/caddy/access.log` is hardcoded and may not match where Caddy is actually writing logs. The user reports seeing Plex logs, which suggests:

1. **Wrong log file** - The LogWatcher might be reading an old/wrong log file
2. **Parsing issue** - Caddy logs aren't properly formatted as expected
3. **Source detection broken** - Logs are being classified as "normal" instead of security events

**Verification Needed:**

```bash
# Check where Caddy is actually logging
docker exec charon cat /config/caddy.json | jq '.logging'

# Check if the access.log file exists and contains recent entries
docker exec charon tail -50 /var/log/caddy/access.log

# Check Caddy data directory
docker exec charon ls -la /app/data/caddy/
```

**Files Involved:**

- `backend/internal/api/routes/routes.go:366` - accessLogPath definition
- `backend/internal/services/log_watcher.go` - File tailing and parsing
- `backend/internal/api/handlers/cerberus_logs_ws.go` - WebSocket handler
- `frontend/src/components/LiveLogViewer.tsx` - Frontend component

---

## Root Cause Summary

| Issue | Root Cause | Impact |
|-------|------------|--------|
| CrowdSec not running | Process start fails silently OR mode not set to "local" in DB | User cannot use CrowdSec features |
| Toggle stuck | Optimistic UI updates + API success despite process failure | Confusing UX, user can't disable |
| Wrong logs displayed | LogWatcher reading wrong file OR parsing application logs | User can't monitor security events |

---

## Proposed Fixes

### Fix #1: CrowdSec Process Start Issues

**Change X → Y Impact:**

```diff
File: backend/internal/services/crowdsec_startup.go

IF Change: Add detailed logging + retry mechanism
THEN Impact:
  ✓ Startup failures become visible in logs
  ✓ Transient failures (DB not ready) are retried
  ✓ CrowdSec has better chance of starting on boot
  ⚠ Retry logic could delay boot by a few seconds

IF Change: Validate binPath exists before calling Start()
THEN Impact:
  ✓ Prevent calling Start() if crowdsec binary missing
  ✓ Clear error message to user
  ⚠ Additional filesystem check on every reconcile
```

**Implementation:**

```go
// backend/internal/services/crowdsec_startup.go

func ReconcileCrowdSecOnStartup(db *gorm.DB, executor CrowdsecProcessManager, binPath, dataDir string) {
 logger.Log().Info("Starting CrowdSec reconciliation on startup")

 // ... existing checks ...

 // VALIDATE: Ensure binary exists
 if _, err := os.Stat(binPath); os.IsNotExist(err) {
  logger.Log().WithField("path", binPath).Error("CrowdSec binary not found, cannot start")
  return
 }

 // VALIDATE: Ensure config directory exists
 if _, err := os.Stat(dataDir); os.IsNotExist(err) {
  logger.Log().WithField("path", dataDir).Error("CrowdSec config directory not found, cannot start")
  return
 }

 // ... existing status check ...

 // START with better error handling
 logger.Log().WithFields(logrus.Fields{
  "bin_path":  binPath,
  "data_dir":  dataDir,
 }).Info("Attempting to start CrowdSec process")

 startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
 defer startCancel()

 newPid, err := executor.Start(startCtx, binPath, dataDir)
 if err != nil {
  logger.Log().WithError(err).WithFields(logrus.Fields{
   "bin_path": binPath,
   "data_dir": dataDir,
  }).Error("CrowdSec reconciliation: FAILED to start CrowdSec - check binary path and config")
  return
 }

 // VERIFY: Wait for PID file to be written
 time.Sleep(2 * time.Second)
 running, pid, err := executor.Status(ctx, dataDir)
 if err != nil || !running {
  logger.Log().WithFields(logrus.Fields{
   "expected_pid": newPid,
   "actual_pid":   pid,
   "running":      running,
  }).Error("CrowdSec process started but not running - process may have crashed")
  return
 }

 logger.Log().WithField("pid", newPid).Info("CrowdSec reconciliation: successfully started and verified CrowdSec")
}
```

---

### Fix #2: Toggle State Management

**Change X → Y Impact:**

```diff
File: frontend/src/pages/Security.tsx

IF Change: Remove optimistic updates, wait for API confirmation
THEN Impact:
  ✓ Toggle always reflects actual backend state
  ✓ No "stuck toggle" UX issue
  ⚠ Toggle feels slightly slower (100-200ms delay)
  ⚠ User must wait for API response before seeing change

IF Change: Add explicit error handling + status reconciliation
THEN Impact:
  ✓ Errors are clearly shown to user
  ✓ Toggle reverts on failure
  ✓ Status check after mutation ensures consistency
  ⚠ Additional API call overhead
```

**Implementation:**

```typescript
// frontend/src/pages/Security.tsx

const crowdsecPowerMutation = useMutation({
  mutationFn: async (enabled: boolean) => {
    // Update setting first
    await updateSetting('security.crowdsec.enabled', enabled ? 'true' : 'false', 'security', 'bool')

    if (enabled) {
      toast.info('Starting CrowdSec... This may take up to 30 seconds')
      const result = await startCrowdsec()

      // VERIFY: Check if it actually started
      const status = await statusCrowdsec()
      if (!status.running) {
        throw new Error('CrowdSec setting enabled but process failed to start. Check server logs.')
      }

      return result
    } else {
      await stopCrowdsec()

      // VERIFY: Check if it actually stopped
      const status = await statusCrowdsec()
      if (status.running) {
        throw new Error('CrowdSec setting disabled but process still running. Check server logs.')
      }

      return { enabled: false }
    }
  },

  // REMOVE OPTIMISTIC UPDATES
  onMutate: undefined,

  onError: (err: unknown, enabled: boolean) => {
    const msg = err instanceof Error ? err.message : String(err)
    toast.error(enabled ? `Failed to start CrowdSec: ${msg}` : `Failed to stop CrowdSec: ${msg}`)

    // Force refresh status from backend
    queryClient.invalidateQueries({ queryKey: ['security-status'] })
    fetchCrowdsecStatus()
  },

  onSuccess: async () => {
    // Refresh all related queries to ensure consistency
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['security-status'] }),
      queryClient.invalidateQueries({ queryKey: ['settings'] }),
      fetchCrowdsecStatus(),
    ])

    toast.success('CrowdSec status updated successfully')
  },
})
```

---

### Fix #3: Security Log Viewer

**Change X → Y Impact:**

```diff
File: backend/internal/api/routes/routes.go + backend/internal/services/log_watcher.go

IF Change: Make log path configurable + validate it exists
THEN Impact:
  ✓ Can specify correct log file via env var
  ✓ Graceful fallback if file doesn't exist
  ✓ Clear error logging about file path issues
  ⚠ Requires updating deployment/env vars

IF Change: Improve log parsing + source detection
THEN Impact:
  ✓ Better classification of security events
  ✓ Clearer distinction between app logs and security logs
  ⚠ More CPU overhead for regex matching
```

**Implementation Plan:**

1. **Verify Current Log Configuration:**

```bash
# Check Caddy config for logging directive
docker exec charon cat /config/caddy.json | jq '.logging.logs'

# Find where Caddy is actually writing logs
docker exec charon find /app/data /var/log -name "*.log" -type f 2>/dev/null

# Check if access.log has recent entries
docker exec charon tail -20 /var/log/caddy/access.log
```

1. **Add Log Path Validation:**

```go
// backend/internal/api/routes/routes.go:366

accessLogPath := os.Getenv("CHARON_CADDY_ACCESS_LOG")
if accessLogPath == "" {
 // Try multiple paths in order of preference
 candidatePaths := []string{
  "/var/log/caddy/access.log",
  filepath.Join(cfg.CaddyConfigDir, "logs", "access.log"),
  filepath.Join(dataDir, "logs", "access.log"),
 }

 for _, path := range candidatePaths {
  if _, err := os.Stat(path); err == nil {
   accessLogPath = path
   logger.Log().WithField("path", path).Info("Found existing Caddy access log")
   break
  }
 }

 // If none exist, use default and create it
 if accessLogPath == "" {
  accessLogPath = "/var/log/caddy/access.log"
  logger.Log().WithField("path", accessLogPath).Warn("No existing access log found, will create at default path")
 }
}

logger.Log().WithField("path", accessLogPath).Info("Initializing LogWatcher with access log path")
```

1. **Improve Source Detection:**

```go
// backend/internal/services/log_watcher.go:221

func (w *LogWatcher) detectSecurityEvent(entry *models.SecurityLogEntry, caddyLog *models.CaddyAccessLog) {
 // Enhanced logger name checking
 loggerLower := strings.ToLower(caddyLog.Logger)

 // Check for WAF/Coraza
 if caddyLog.Status == 403 && (
  strings.Contains(loggerLower, "waf") ||
  strings.Contains(loggerLower, "coraza") ||
  hasHeader(caddyLog.RespHeaders, "X-Coraza-Id")) {
  entry.Blocked = true
  entry.Source = "waf"
  entry.Level = "warn"
  entry.BlockReason = "WAF rule triggered"
  // ... extract rule ID ...
  return
 }

 // Check for CrowdSec
 if caddyLog.Status == 403 && (
  strings.Contains(loggerLower, "crowdsec") ||
  strings.Contains(loggerLower, "bouncer") ||
  hasHeader(caddyLog.RespHeaders, "X-Crowdsec-Decision")) {
  entry.Blocked = true
  entry.Source = "crowdsec"
  entry.Level = "warn"
  entry.BlockReason = "CrowdSec decision"
  return
 }

 // Check for ACL
 if caddyLog.Status == 403 && (
  strings.Contains(loggerLower, "acl") ||
  hasHeader(caddyLog.RespHeaders, "X-Acl-Denied")) {
  entry.Blocked = true
  entry.Source = "acl"
  entry.Level = "warn"
  entry.BlockReason = "Access list denied"
  return
 }

 // Check for rate limiting
 if caddyLog.Status == 429 {
  entry.Blocked = true
  entry.Source = "ratelimit"
  entry.Level = "warn"
  entry.BlockReason = "Rate limit exceeded"
  // ... extract rate limit headers ...
  return
 }

 // If it's a proxy log (reverse_proxy logger), mark as normal traffic
 if strings.Contains(loggerLower, "reverse_proxy") ||
    strings.Contains(loggerLower, "access_log") {
  entry.Source = "normal"
  entry.Blocked = false
  // Don't set level to warn for successful requests
  if caddyLog.Status < 400 {
   entry.Level = "info"
  }
  return
 }

 // Default for unclassified 403s
 if caddyLog.Status == 403 {
  entry.Blocked = true
  entry.Source = "cerberus"
  entry.Level = "warn"
  entry.BlockReason = "Access denied"
 }
}
```

---

## Testing Plan

### Pre-Checks

```bash
# 1. Verify container is running
docker ps | grep charon

# 2. Check if crowdsec binary exists
docker exec charon which crowdsec
docker exec charon ls -la /usr/bin/crowdsec  # Or wherever it's installed

# 3. Check database config
docker exec charon cat /app/data/charon.db  # Would need sqlite3 or Go query

# 4. Check Caddy log configuration
docker exec charon cat /config/caddy.json | jq '.logging'

# 5. Find actual log files
docker exec charon find /var/log /app/data -name "*.log" -type f 2>/dev/null
```

### Test Scenario 1: CrowdSec Startup

```bash
# Given: Container restarts
docker restart charon

# When: Container boots
# Then:
#   - Check logs for CrowdSec reconciliation messages
#   - Verify PID file created: /app/data/crowdsec/crowdsec.pid
#   - Verify process running: docker exec charon ps aux | grep crowdsec
#   - Verify status API returns running=true

docker logs charon --tail 100 | grep -i "crowdsec"
docker exec charon ps aux | grep crowdsec
docker exec charon ls -la /app/data/crowdsec/crowdsec.pid
```

### Test Scenario 2: Toggle Behavior

```bash
# Given: CrowdSec is running
# When: User clicks toggle to disable
# Then:
#   - Frontend shows loading state
#   - API call succeeds
#   - Process stops (no crowdsec in ps)
#   - PID file removed
#   - Toggle reflects OFF state
#   - Status API returns running=false

# When: User clicks toggle to enable
# Then:
#   - Frontend shows loading state
#   - API call succeeds
#   - Process starts
#   - PID file created
#   - Toggle reflects ON state
#   - Status API returns running=true
```

### Test Scenario 3: Security Log Viewer

```bash
# Given: CrowdSec is enabled and blocking traffic
# When: User opens Cerberus Dashboard
# Then:
#   - WebSocket connects successfully (check browser console)
#   - Logs appear in real-time
#   - Blocked requests show with red indicator
#   - Source badges show correct module (crowdsec, waf, etc.)

# Test blocked request:
curl -H "User-Agent: BadBot" https://your-charon-instance.com
# Should see blocked log entry in dashboard
```

---

## Implementation Order

1. **Phase 1: Diagnostics** (15 minutes)
   - Run all pre-checks
   - Document actual state of system
   - Identify which issue is the primary blocker

2. **Phase 2: CrowdSec Startup** (30 minutes)
   - Implement enhanced logging in `crowdsec_startup.go`
   - Add binary/config validation
   - Test container restart

3. **Phase 3: Toggle Fix** (20 minutes)
   - Remove optimistic updates from `Security.tsx`
   - Add status verification
   - Test toggle on/off cycle

4. **Phase 4: Log Viewer** (30 minutes)
   - Verify log file path
   - Implement log path detection
   - Improve source detection
   - Test with actual traffic

5. **Phase 5: Integration Testing** (30 minutes)
   - Full end-to-end test
   - Verify all three issues resolved
   - Check for regressions

**Total Estimated Time:** 2 hours

---

## Success Criteria

✅ **CrowdSec Running:**

- `docker exec charon ps aux | grep crowdsec` shows running process
- PID file exists at `/app/data/crowdsec/crowdsec.pid`
- `/api/v1/admin/crowdsec/status` returns `{"running": true, "pid": <number>}`

✅ **Toggle Working:**

- Toggle can be turned on and off without getting stuck
- UI state matches backend process state
- Clear error messages if operations fail

✅ **Logs Correct:**

- Security log viewer shows Caddy access logs
- Blocked requests appear with proper indicators
- Source badges correctly identify security module
- WebSocket stays connected

---

## Rollback Plan

If hotfix causes issues:

1. **Revert Commits:**

```bash
git revert HEAD~3..HEAD  # Revert last 3 commits
git push origin feature/beta-release
```

1. **Restart Container:**

```bash
docker restart charon
```

1. **Verify Basic Functionality:**

- Proxy hosts still work
- SSL still works
- No new errors in logs

---

## Notes for QA

- Test on clean container (no previous CrowdSec state)
- Test with existing CrowdSec config
- Test rapid toggle on/off cycles
- Monitor container logs during testing
- Check browser console for WebSocket errors
- Verify memory usage doesn't spike (log file tailing)

---

## QA Testing Results (December 15, 2025)

**Tester:** QA_Security
**Build:** charon:local (post-migration implementation)
**Test Date:** 2025-12-15 03:24 UTC

### Phase 1: Migration Implementation Testing

#### Test 1.1: Migration Command Execution

- **Status:** ✅ **PASSED**
- **Command:** `docker exec charon /app/charon migrate`
- **Result:** All 6 security tables created successfully
- **Evidence:** See [crowdsec_migration_qa_report.md](crowdsec_migration_qa_report.md)

#### Test 1.2: CrowdSec Auto-Start Behavior

- **Status:** ⚠️ **EXPECTED BEHAVIOR** (Not a Bug)
- **Observation:** CrowdSec did NOT auto-start after restart
- **Reason:** Fresh database has no SecurityConfig **record**, only table structure
- **Resolution:** This is correct first-boot behavior

### Phase 2: Code Quality Validation

- **Pre-commit:** ✅ All hooks passed
- **Backend Tests:** ✅ 9/9 packages passed (including 3 new migration tests)
- **Frontend Tests:** ✅ 772 tests passed | 2 skipped
- **Code Cleanliness:** ✅ No debug statements, zero linter issues

### Phase 3: Regression Testing

- **Schema Impact:** ✅ No changes to existing tables
- **Feature Validation:** ✅ All 772 tests passed, no regressions

### Summary

**QA Sign-Off:** ✅ **APPROVED FOR PRODUCTION**

**Detailed Report:** [crowdsec_migration_qa_report.md](crowdsec_migration_qa_report.md)
