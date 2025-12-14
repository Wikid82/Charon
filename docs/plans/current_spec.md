# Fix Plan: Critical Issues After Docker Rebuild

**Date:** December 14, 2025
**Status:** Planning Phase
**Priority:** P0 - Urgent

---

## Issue Summary

After a Docker container rebuild, three critical issues were identified:

1. **500 error on CrowdSec Stop()** - Toggling CrowdSec OFF returns "Failed to stop CrowdSec: Request failed with status code 500"
2. **CrowdSec shows "not running"** - Despite database setting being enabled, the process isn't running after container restart
3. **Live Logs Disconnected** - WebSocket shows disconnected state even when logs are being generated

---

## Root Cause Analysis

### Issue 1: 500 Error on Stop()

**Location:** [crowdsec_exec.go](../../backend/internal/api/handlers/crowdsec_exec.go#L36-L51)

**Root Cause:** The `Stop()` method in `DefaultCrowdsecExecutor` fails when there's no PID file.

```go
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        return fmt.Errorf("pid file read: %w", err)  // ← FAILS HERE with 500
    }
    // ...
}
```

**Problem:** When the container restarts:
1. The PID file (`/app/data/crowdsec/crowdsec.pid`) is deleted (ephemeral or process cleanup removes it)
2. The database still has CrowdSec "enabled" = true
3. User clicks "Disable" (which calls Stop())
4. Stop() tries to read the PID file → fails → returns error → 500 response

**Why it fails:** The code assumes a PID file always exists when stopping. But after container restart, there's no PID file because the CrowdSec process wasn't running (GUI-controlled lifecycle, NOT auto-started).

### Issue 2: CrowdSec Shows "Not Running" After Restart

**Location:** [docker-entrypoint.sh](../../docker-entrypoint.sh#L91-L97)

**Root Cause:** CrowdSec is **intentionally NOT auto-started** in the entrypoint. The design is "GUI-controlled lifecycle."

From the entrypoint:
```bash
# CrowdSec Lifecycle Management:
# CrowdSec configuration is initialized above (symlinks, directories, hub updates)
# However, the CrowdSec agent is NOT auto-started in the entrypoint.
# Instead, CrowdSec lifecycle is managed by the backend handlers via GUI controls.
```

**Problem:** The database stores the user's "enabled" preference, but there's no reconciliation at startup:
1. Container restarts
2. Database says CrowdSec `enabled = true` (user's previous preference)
3. But CrowdSec process is NOT started (by design)
4. UI shows "enabled" but status shows "not running" → confusing state mismatch

**Missing Logic:** No startup reconciliation to check "if DB says enabled, start CrowdSec process."

### Issue 3: Live Logs Disconnected

**Location:** [logs_ws.go](../../backend/internal/api/handlers/logs_ws.go) and [log_watcher.go](../../backend/internal/services/log_watcher.go)

**Root Cause:** There are **two separate WebSocket log systems** that may be misconfigured:

1. **`/api/v1/logs/live`** (logs_ws.go) - Streams application logs via `logger.GetBroadcastHook()`
2. **`/api/v1/cerberus/logs/ws`** (cerberus_logs_ws.go) - Streams Caddy access logs via `LogWatcher`

**Potential Issues:**

a) **LogWatcher not started:** The `LogWatcher` must be explicitly started with `Start(ctx)`. If the watcher isn't started during server initialization, no logs are broadcast.

b) **Log file doesn't exist:** The LogWatcher waits for `/var/log/caddy/access.log` to exist. After container restart with no traffic, this file may not exist yet.

c) **WebSocket connection path mismatch:** Frontend might connect to wrong endpoint or with invalid token.

d) **CSP blocking WebSocket:** Security middleware's Content-Security-Policy must allow `ws:` and `wss:` protocols.

---

## Detailed Code Analysis

### Stop() Method - Full Code Review

```go
// File: backend/internal/api/handlers/crowdsec_exec.go
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        return fmt.Errorf("pid file read: %w", err)  // ← CRITICAL: Returns error on ENOENT
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
        return err
    }
    _ = os.Remove(e.pidFile(configDir))
    return nil
}
```

The problem is clear: `os.ReadFile()` returns `os.ErrNotExist` when the PID file doesn't exist, and this is propagated as a 500 error.

### Status() Method - Already Handles Missing PID Gracefully

```go
func (e *DefaultCrowdsecExecutor) Status(ctx context.Context, configDir string) (running bool, pid int, err error) {
    b, err := os.ReadFile(e.pidFile(configDir))
    if err != nil {
        // Missing pid file is treated as not running ← GOOD PATTERN
        return false, 0, nil
    }
    // ...
}
```

**Key Insight:** `Status()` already handles missing PID file gracefully by returning `(false, 0, nil)`. The `Stop()` method should follow the same pattern.

---

## Fix Plan

### Fix 1: Make Stop() Idempotent (No Error When Already Stopped)

**File:** `backend/internal/api/handlers/crowdsec_exec.go`

**Current Code (lines 36-51):**
```go
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	b, err := os.ReadFile(e.pidFile(configDir))
	if err != nil {
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
		return err
	}
	// best-effort remove pid file
	_ = os.Remove(e.pidFile(configDir))
	return nil
}
```

**Fixed Code:**
```go
func (e *DefaultCrowdsecExecutor) Stop(ctx context.Context, configDir string) error {
	b, err := os.ReadFile(e.pidFile(configDir))
	if err != nil {
		if os.IsNotExist(err) {
			// No PID file means process isn't running - that's OK for Stop()
			// This makes Stop() idempotent (safe to call multiple times)
			return nil
		}
		return fmt.Errorf("pid file read: %w", err)
	}
	pid, err := strconv.Atoi(string(b))
	if err != nil {
		// Malformed PID file - remove it and treat as not running
		_ = os.Remove(e.pidFile(configDir))
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process lookup failed - clean up PID file
		_ = os.Remove(e.pidFile(configDir))
		return nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Process may already be dead - clean up PID file
		_ = os.Remove(e.pidFile(configDir))
		// Only return error if it's not "process doesn't exist"
		if !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	// best-effort remove pid file
	_ = os.Remove(e.pidFile(configDir))
	return nil
}
```

**Rationale:** Stop() should be idempotent. Stopping an already-stopped service shouldn't error.

### Fix 2: Add CrowdSec Startup Reconciliation

**File:** `backend/internal/api/routes/routes.go` (or create `backend/internal/services/crowdsec_startup.go`)

**New Function:**
```go
// ReconcileCrowdSecOnStartup checks if CrowdSec should be running based on DB settings
// and starts it if necessary. This handles the case where the container restarts
// but the user's preference was to have CrowdSec enabled.
func ReconcileCrowdSecOnStartup(db *gorm.DB, executor handlers.CrowdsecExecutor, binPath, dataDir string) {
    if db == nil {
        return
    }

    var secCfg models.SecurityConfig
    if err := db.First(&secCfg).Error; err != nil {
        // No config yet or error - nothing to reconcile
        logger.Log().WithError(err).Debug("No security config found for CrowdSec reconciliation")
        return
    }

    // Check if CrowdSec should be running based on mode
    if secCfg.CrowdSecMode != "local" {
        logger.Log().WithField("mode", secCfg.CrowdSecMode).Debug("CrowdSec mode is not 'local', skipping auto-start")
        return
    }

    // Check if already running
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    running, _, _ := executor.Status(ctx, dataDir)
    if running {
        logger.Log().Info("CrowdSec already running, no action needed")
        return
    }

    // Start CrowdSec since DB says it should be enabled
    logger.Log().Info("CrowdSec mode is 'local' but process not running, starting...")
    _, err := executor.Start(ctx, binPath, dataDir)
    if err != nil {
        logger.Log().WithError(err).Warn("Failed to auto-start CrowdSec on startup reconciliation")
    } else {
        logger.Log().Info("CrowdSec started successfully via startup reconciliation")
    }
}
```

**Integration Point:** Call this function after database migration in server initialization:
```go
// In routes.go or server.go, after DB is ready and handlers are created
if crowdsecHandler != nil {
    ReconcileCrowdSecOnStartup(db, crowdsecHandler.Executor, crowdsecHandler.BinPath, crowdsecHandler.DataDir)
}
```

### Fix 3: Ensure LogWatcher is Started and Log File Exists

**File:** `backend/internal/api/routes/routes.go`

**Check that LogWatcher.Start() is called:**
```go
// Ensure LogWatcher is started with proper log path
logPath := "/var/log/caddy/access.log"

// Ensure log directory exists
if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
    logger.Log().WithError(err).Warn("Failed to create log directory")
}

// Create empty log file if it doesn't exist (allows LogWatcher to start tailing immediately)
if _, err := os.Stat(logPath); os.IsNotExist(err) {
    if f, err := os.Create(logPath); err == nil {
        f.Close()
        logger.Log().WithField("path", logPath).Info("Created empty log file for LogWatcher")
    }
}

// Create and start the LogWatcher
watcher := services.NewLogWatcher(logPath)
if err := watcher.Start(context.Background()); err != nil {
    logger.Log().WithError(err).Error("Failed to start LogWatcher")
}
```

**Additionally, verify CSP allows WebSocket:**
The security middleware in `backend/internal/api/middleware/security.go` already has:
```go
directives["connect-src"] = "'self' ws: wss:" // WebSocket for HMR
```

This should allow WebSocket connections.

---

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/api/handlers/crowdsec_exec.go` | Make `Stop()` idempotent for missing PID file |
| `backend/internal/api/routes/routes.go` | Add CrowdSec startup reconciliation call |
| `backend/internal/services/crowdsec_startup.go` | (NEW) Create startup reconciliation function |
| `backend/internal/api/handlers/crowdsec_exec_test.go` | Add tests for Stop() idempotency |

---

## Testing Plan

### Test 1: Stop() Idempotency
```bash
# Start container fresh (no CrowdSec running)
docker compose down -v && docker compose up -d

# Call Stop() without starting CrowdSec first
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/stop \
  -H "Authorization: Bearer $TOKEN"

# Expected: 200 OK {"status": "stopped"}
# NOT: 500 Internal Server Error
```

### Test 2: Startup Reconciliation
```bash
# Enable CrowdSec via API
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start \
  -H "Authorization: Bearer $TOKEN"

# Verify running
curl http://localhost:8080/api/v1/admin/crowdsec/status \
  -H "Authorization: Bearer $TOKEN"
# Expected: {"running": true, "pid": xxx, "lapi_ready": true}

# Restart container
docker compose restart charon

# Wait for startup
sleep 10

# Verify CrowdSec auto-started
curl http://localhost:8080/api/v1/admin/crowdsec/status \
  -H "Authorization: Bearer $TOKEN"
# Expected: {"running": true, "pid": xxx, "lapi_ready": true}
```

### Test 3: Live Logs
```bash
# Connect to WebSocket
websocat "ws://localhost:8080/api/v1/logs/live?token=$TOKEN"

# In another terminal, generate traffic
curl http://localhost:8080/api/v1/health

# Verify log entries appear in WebSocket connection
```

---

## Implementation Order

1. **Fix 1 (Stop Idempotency)** - Quick fix, high impact, unblocks users immediately
2. **Fix 2 (Startup Reconciliation)** - Core fix for state mismatch after restart
3. **Fix 3 (Live Logs)** - May need more investigation; likely already working if LogWatcher is started

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Stop() idempotency | Very Low | Makes existing code more robust |
| Startup reconciliation | Low | Only runs once at startup, respects DB state |
| Log file creation | Very Low | Standard file operations with error handling |

---

## Notes

- The CrowdSec PID file path is `${dataDir}/crowdsec.pid` (e.g., `/app/data/crowdsec/crowdsec.pid`)
- The LogWatcher monitors `/var/log/caddy/access.log` by default
- WebSocket ping interval is 30 seconds for keep-alive
- CrowdSec LAPI runs on port 8085 (not 8080) to avoid conflict with Charon
- The `Status()` handler already includes `lapi_ready` field (from previous fix)
