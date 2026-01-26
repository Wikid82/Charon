# Diagnostic & Fix Plan: CrowdSec and Live Logs Issues Post Docker Rebuild

**Date:** December 14, 2025
**Investigator:** Planning Agent
**Scope:** Three user-reported issues after Docker rebuild
**Status:** ✅ **COMPLETE - Root causes identified with fixes ready**

---

## Executive Summary

After thorough investigation of the backend handlers, executor implementation, entrypoint script, and frontend code, I've identified the root causes for all three reported issues:

1. **CrowdSec shows "not running"** - Process detection via PID file is failing
2. **500 error when stopping CrowdSec** - PID file doesn't exist when CrowdSec wasn't started via handlers
3. **Live log viewer disconnected** - LogWatcher can't find the access log file

---

## Issue 1: CrowdSec Shows "Not Running" Even Though Enabled in UI

### Root Cause Analysis

The mismatch occurs because:

1. **Database Setting vs Process State**: The UI toggle updates the setting `security.crowdsec.enabled` in the database, but **does not actually start the CrowdSec process**.

2. **Process Lifecycle Design**: Per [docker-entrypoint.sh](../../docker-entrypoint.sh) (line 56-65), CrowdSec is explicitly **NOT auto-started** in the container entrypoint:

   ```bash
   # CrowdSec Lifecycle Management:
   # CrowdSec agent is NOT auto-started in the entrypoint.
   # Instead, CrowdSec lifecycle is managed by the backend handlers via GUI controls.
   ```

3. **Status() Handler Behavior** ([crowdsec_handler.go#L238-L266](../../backend/internal/api/handlers/crowdsec_handler.go)):
   - Calls `h.Executor.Status()` which reads from PID file at `{configDir}/crowdsec.pid`
   - If PID file doesn't exist (CrowdSec never started), returns `running: false`
   - The frontend correctly shows "Stopped" even when setting is "enabled"

4. **The Disconnect**:
   - Setting `security.crowdsec.enabled = true` ≠ Process running
   - The setting tells Cerberus middleware to "use CrowdSec for protection" IF running
   - The actual start requires clicking the toggle which calls `crowdsecPowerMutation.mutate(true)`

### Why It Appears Broken

After Docker rebuild:

- Fresh container has `security.crowdsec.enabled` potentially still `true` in DB (persisted volume)
- But PID file is gone (container restart)
- CrowdSec process not running
- UI shows "enabled" setting but status shows "not running"

### Status() Handler Already Fixed

Looking at the current implementation in [crowdsec_handler.go#L238-L266](../../backend/internal/api/handlers/crowdsec_handler.go), the `Status()` handler **already includes LAPI readiness check**:

```go
func (h *CrowdsecHandler) Status(c *gin.Context) {
    ctx := c.Request.Context()
    running, pid, err := h.Executor.Status(ctx, h.DataDir)
    // ...
    // Check LAPI connectivity if process is running
    lapiReady := false
    if running {
        args := []string{"lapi", "status"}
        // ... LAPI check implementation ...
        lapiReady = (checkErr == nil)
    }

    c.JSON(http.StatusOK, gin.H{
        "running":    running,
        "pid":        pid,
        "lapi_ready": lapiReady,
    })
}
```

### Additional Enhancement Required

Add `setting_enabled` and `needs_start` fields to help frontend show correct state:

**File:** [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go)

```go
func (h *CrowdsecHandler) Status(c *gin.Context) {
    ctx := c.Request.Context()
    running, pid, err := h.Executor.Status(ctx, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Check setting state
    settingEnabled := false
    if h.DB != nil {
        var setting models.Setting
        if err := h.DB.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error; err == nil {
            settingEnabled = strings.EqualFold(strings.TrimSpace(setting.Value), "true")
        }
    }

    // Check LAPI connectivity if process is running
    lapiReady := false
    if running {
        // ... existing LAPI check ...
    }

    c.JSON(http.StatusOK, gin.H{
        "running":         running,
        "pid":             pid,
        "lapi_ready":      lapiReady,
        "setting_enabled": settingEnabled,
        "needs_start":     settingEnabled && !running,  // NEW: hint for frontend
    })
}
```

---

## Issue 2: 500 Error When Stopping CrowdSec

### Root Cause Analysis

The 500 error occurs in [crowdsec_exec.go#L37-L53](../../backend/internal/api/handlers/crowdsec_exec.go):

```go
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        return fmt.Errorf("pid file read: %w", err)  // <-- 500 error here
    }
    // ...
}
```

**The Problem:**

1. PID file at `/app/data/crowdsec/crowdsec.pid` doesn't exist
2. This happens when:
   - CrowdSec was never started via the handlers
   - Container was restarted (PID file lost)
   - CrowdSec was started externally but not via Charon handlers

### Fix Required

Modify `Stop()` in [crowdsec_exec.go](../../backend/internal/api/handlers/crowdsec_exec.go) to handle missing PID gracefully:

```go
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        if os.IsNotExist(err) {
            // PID file doesn't exist - process likely not running or was started externally
            // Try to find and stop any running crowdsec process
            return e.stopByProcessName(ctx)
        }
        return fmt.Errorf("pid file read: %w", err)
    }
    pid, err := strconv.Atoi(string(b))
    if err != nil {
        return fmt.Errorf("invalid pid: %w", err)
    }
    proc, err := os.FindProcess(pid)
    if err != nil {
        return err
    }
    if err := proc.Signal(syscall.SIGTERM); err != nil {
        // Process might already be dead
        if errors.Is(err, os.ErrProcessDone) {
            _ = os.Remove(e.pidFile(configDir))
            return nil
        }
        return err
    }
    _ = os.Remove(e.pidFile(configDir))
    return nil
}

// stopByProcessName attempts to stop CrowdSec by finding it via process name
func (e *DefaultCrowdsecExecutor) stopByProcessName(ctx context.Context) error {
    // Use pkill or pgrep to find crowdsec process
    cmd := exec.CommandContext(ctx, "pkill", "-TERM", "crowdsec")
    err := cmd.Run()
    if err != nil {
        // pkill returns exit code 1 if no processes matched - that's OK
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
            return nil // No process to kill, already stopped
        }
        return fmt.Errorf("failed to stop crowdsec by process name: %w", err)
    }
    return nil
}
```

**File:** [backend/internal/api/handlers/crowdsec_exec.go](../../backend/internal/api/handlers/crowdsec_exec.go)

---

## Issue 3: Live Log Viewer Disconnected on Cerberus Dashboard

### Root Cause Analysis

The Live Log Viewer uses two WebSocket endpoints:

1. **Application Logs** (`/api/v1/logs/live`) - Works via `BroadcastHook` in logger
2. **Security Logs** (`/api/v1/cerberus/logs/ws`) - Requires `LogWatcher` to tail access log file

The Cerberus Security Logs WebSocket ([cerberus_logs_ws.go](../../backend/internal/api/handlers/cerberus_logs_ws.go)) depends on `LogWatcher` which tails `/var/log/caddy/access.log`.

**The Problem:**

In [log_watcher.go#L102-L117](../../backend/internal/services/log_watcher.go):

```go
func (w *LogWatcher) tailFile() {
    for {
        // Wait for file to exist
        if _, err := os.Stat(w.logPath); os.IsNotExist(err) {
            logger.Log().WithField("path", w.logPath).Debug("Log file not found, waiting...")
            time.Sleep(time.Second)
            continue
        }
        // ...
    }
}
```

After Docker rebuild:

1. Caddy may not have written any logs yet
2. `/var/log/caddy/access.log` doesn't exist
3. `LogWatcher` enters infinite "waiting" loop
4. No log entries are ever sent to WebSocket clients
5. Frontend shows "disconnected" because no heartbeat/data received

### Why "Disconnected" Appears

From [cerberus_logs_ws.go#L79-L83](../../backend/internal/api/handlers/cerberus_logs_ws.go):

```go
case <-ticker.C:
    // Send ping to keep connection alive
    if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
        return
    }
```

The ping is sent every 30 seconds, but if the frontend's WebSocket connection times out or encounters an error before receiving any message, it shows "disconnected".

### Fix Required

**Fix 1:** Create log file if missing in `LogWatcher.Start()`:

**File:** [backend/internal/services/log_watcher.go](../../backend/internal/services/log_watcher.go)

```go
import "path/filepath"

func (w *LogWatcher) Start(ctx context.Context) error {
    w.mu.Lock()
    if w.started {
        w.mu.Unlock()
        return nil
    }
    w.started = true
    w.mu.Unlock()

    // Ensure log file exists
    logDir := filepath.Dir(w.logPath)
    if err := os.MkdirAll(logDir, 0755); err != nil {
        logger.Log().WithError(err).Warn("Failed to create log directory")
    }
    if _, err := os.Stat(w.logPath); os.IsNotExist(err) {
        if f, err := os.Create(w.logPath); err == nil {
            f.Close()
            logger.Log().WithField("path", w.logPath).Info("Created empty log file for tailing")
        }
    }

    go w.tailFile()
    logger.Log().WithField("path", w.logPath).Info("LogWatcher started")
    return nil
}
```

**Fix 2:** Send initial heartbeat message on WebSocket connect:

**File:** [backend/internal/api/handlers/cerberus_logs_ws.go](../../backend/internal/api/handlers/cerberus_logs_ws.go)

```go
func (h *CerberusLogsHandler) LiveLogs(c *gin.Context) {
    // ... existing upgrade code ...

    logger.Log().WithField("subscriber_id", subscriberID).Info("Cerberus logs WebSocket connected")

    // Send connection confirmation immediately
    _ = conn.WriteJSON(map[string]interface{}{
        "type":      "connected",
        "timestamp": time.Now().Format(time.RFC3339),
    })

    // ... rest unchanged ...
}
```

---

## Summary of Required Changes

### File 1: [backend/internal/api/handlers/crowdsec_exec.go](../../backend/internal/api/handlers/crowdsec_exec.go)

**Change:** Make `Stop()` handle missing PID file gracefully

```go
// Add import for exec
import "os/exec"

// Add this method
func (e *DefaultCrowdsecExecutor) stopByProcessName(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "pkill", "-TERM", "crowdsec")
    err := cmd.Run()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
            return nil
        }
        return fmt.Errorf("failed to stop crowdsec by process name: %w", err)
    }
    return nil
}

// Modify Stop()
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        if os.IsNotExist(err) {
            return e.stopByProcessName(ctx)
        }
        return fmt.Errorf("pid file read: %w", err)
    }
    // ... rest unchanged ...
}
```

### File 2: [backend/internal/services/log_watcher.go](../../backend/internal/services/log_watcher.go)

**Change:** Ensure log file exists before starting tail

```go
import "path/filepath"

func (w *LogWatcher) Start(ctx context.Context) error {
    w.mu.Lock()
    if w.started {
        w.mu.Unlock()
        return nil
    }
    w.started = true
    w.mu.Unlock()

    // Ensure log file exists
    logDir := filepath.Dir(w.logPath)
    if err := os.MkdirAll(logDir, 0755); err != nil {
        logger.Log().WithError(err).Warn("Failed to create log directory")
    }
    if _, err := os.Stat(w.logPath); os.IsNotExist(err) {
        if f, err := os.Create(w.logPath); err == nil {
            f.Close()
        }
    }

    go w.tailFile()
    logger.Log().WithField("path", w.logPath).Info("LogWatcher started")
    return nil
}
```

### File 3: [backend/internal/api/handlers/cerberus_logs_ws.go](../../backend/internal/api/handlers/cerberus_logs_ws.go)

**Change:** Send connection confirmation on WebSocket connect

```go
func (h *CerberusLogsHandler) LiveLogs(c *gin.Context) {
    // ... existing upgrade code ...

    logger.Log().WithField("subscriber_id", subscriberID).Info("Cerberus logs WebSocket connected")

    // Send connection confirmation immediately
    _ = conn.WriteJSON(map[string]interface{}{
        "type":      "connected",
        "timestamp": time.Now().Format(time.RFC3339),
    })

    // ... rest unchanged ...
}
```

### File 4: [backend/internal/api/handlers/crowdsec_handler.go](../../backend/internal/api/handlers/crowdsec_handler.go)

**Change:** Add setting reconciliation hint in Status response

```go
func (h *CrowdsecHandler) Status(c *gin.Context) {
    ctx := c.Request.Context()
    running, pid, err := h.Executor.Status(ctx, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Check setting state
    settingEnabled := false
    if h.DB != nil {
        var setting models.Setting
        if err := h.DB.Where("key = ?", "security.crowdsec.enabled").First(&setting).Error; err == nil {
            settingEnabled = strings.EqualFold(strings.TrimSpace(setting.Value), "true")
        }
    }

    // Check LAPI connectivity if process is running
    lapiReady := false
    if running {
        // ... existing LAPI check ...
    }

    c.JSON(http.StatusOK, gin.H{
        "running":         running,
        "pid":             pid,
        "lapi_ready":      lapiReady,
        "setting_enabled": settingEnabled,
        "needs_start":     settingEnabled && !running,
    })
}
```

---

## Testing Steps

### Test Issue 1: CrowdSec Status Consistency

1. Start container fresh
2. Check Security dashboard - should show CrowdSec as "Disabled"
3. Toggle CrowdSec on - should start process and show "Running"
4. Restart container
5. Check Security dashboard - should show "needs restart" or auto-start

### Test Issue 2: Stop CrowdSec Without Error

1. With CrowdSec not running, try to stop via UI toggle
2. Should NOT return 500 error
3. Should return success or "already stopped"
4. Check logs for graceful handling

### Test Issue 3: Live Logs Connection

1. Start container fresh
2. Navigate to Cerberus Dashboard
3. Live Log Viewer should show "Connected" status
4. Make a request to trigger log entry
5. Entry should appear in viewer

### Integration Test

```bash
# Run in container
cd /projects/Charon/backend
go test ./internal/api/handlers/... -run TestCrowdsec -v
```

---

## Debug Commands

```bash
# Check if CrowdSec PID file exists
ls -la /app/data/crowdsec/crowdsec.pid

# Check CrowdSec process status
pgrep -la crowdsec

# Check access log file
ls -la /var/log/caddy/access.log

# Test LAPI health
curl http://127.0.0.1:8085/health

# Check WebSocket endpoint
# In browser console:
# new WebSocket('ws://localhost:8080/api/v1/cerberus/logs/ws')
```

---

## Conclusion

All three issues stem from **state synchronization problems** after container restart:

1. **CrowdSec**: Database setting doesn't match process state
2. **Stop Error**: Handler assumes PID file exists when it may not
3. **Live Logs**: Log file may not exist, causing LogWatcher to wait indefinitely

The fixes are defensive programming patterns:

- Handle missing PID file gracefully
- Create log files if they don't exist
- Add reconciliation hints in status responses
- Send WebSocket heartbeats immediately on connect

---

## Commit Message Template

```
fix: handle container restart edge cases for CrowdSec and Live Logs

Issue 1 - CrowdSec "not running" status:
- Add setting_enabled and needs_start fields to Status() response
- Frontend can now show proper "needs restart" state

Issue 2 - 500 error on Stop:
- Handle missing PID file gracefully in Stop()
- Fallback to pkill if PID file doesn't exist
- Return success if process already stopped

Issue 3 - Live Logs disconnected:
- Create log file if it doesn't exist on LogWatcher.Start()
- Send WebSocket connection confirmation immediately
- Ensure clients know connection is alive before first log entry

All fixes are defensive programming patterns for container restart scenarios.
```
