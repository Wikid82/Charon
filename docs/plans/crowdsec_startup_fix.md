# CrowdSec Startup Fix Plan

**Date:** 2025-12-22
**Status:** Draft
**Priority:** High

## Executive Summary

CrowdSec is not starting automatically when the container starts. Manual start attempts via the GUI succeed in launching the CrowdSec process, but it immediately fails because the LAPI (Local API) cannot bind to port 8085. The error logs show the Caddy CrowdSec bouncer continuously retrying to connect to LAPI on 127.0.0.1:8085, but getting "connection refused" errors.

## Root Cause Analysis

### 1. **ReconcileCrowdSecOnStartup Not Called**

**Finding:** The `ReconcileCrowdSecOnStartup` function exists in [backend/internal/services/crowdsec_startup.go](backend/internal/services/crowdsec_startup.go) but is called in [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go) as a goroutine **AFTER** route registration completes. This means:
- The function is never called during container startup phase (before HTTP server starts)
- It only executes after the HTTP server is running
- There's no coordination with the entrypoint script's initialization phase

**Evidence:**
```go
// From routes.go line ~466
// Reconcile CrowdSec state on startup (handles container restarts)
go services.ReconcileCrowdSecOnStartup(db, crowdsecExec, crowdsecBinPath, crowdsecDataDir)
```

This goroutine starts AFTER the routes are registered, which happens AFTER the main database migrations and all other initialization. The entrypoint script comments explicitly state:

```bash
# From .docker/docker-entrypoint.sh line 66
# Note: CrowdSec agent is not auto-started. Lifecycle is GUI-controlled via backend handlers.
```

### 2. **CrowdSec Process Starts But LAPI Fails to Bind**

**Finding:** When CrowdSec is manually started via `/api/v1/admin/crowdsec/start`, the process launches successfully (PID is returned, process appears in process list), but the LAPI server component fails to start.

**Evidence from logs:**
```
{"level":"error","ts":1766442959.4174962,"logger":"crowdsec","msg":"failed to connect to LAPI, retrying in 10s: Get \"http://127.0.0.1:8085/v1/decisions/stream?startup=true\": dial tcp 127.0.0.1:8085: connect: connection refused"}
```

The Caddy bouncer (which runs as part of Caddy) is trying to connect to the CrowdSec LAPI on port 8085 but repeatedly fails with "connection refused". This indicates the LAPI listener never binds to the port.

### 3. **Permission Issues with CrowdSec Data Directory**

**Finding:** The CrowdSec data directory `/var/lib/crowdsec/data/` is owned by `root:root` but the application runs as user `charon` (UID 1000).

**Evidence:**
```bash
$ docker compose -f docker-compose.test.yml exec charon ls -la /var/lib/crowdsec/data/
total 192
drwxr-xr-x    1 charon   charon        4096 Dec 22 17:38 .
drwxr-xr-x    1 charon   charon        4096 Dec 22 17:18 ..
-rw-r-----    1 root     root        131072 Dec 22 17:38 crowdsec.db
-rw-r-----    1 root     root         32768 Dec 22 17:38 crowdsec.db-shm
-rw-r-----    1 root     root         12392 Dec 22 17:38 crowdsec.db-wal
```

The database files are owned by `root` with `rw-r-----` (640) permissions. When the CrowdSec process is started by the `charon` user via `exec.Command`, it cannot write to these files or bind to the LAPI socket.

### 4. **Process Group Detachment Issue**

**Finding:** In [backend/internal/api/handlers/crowdsec_exec.go](backend/internal/api/handlers/crowdsec_exec.go), the `Start` method uses `Setpgid: true` to detach the CrowdSec process from the parent process group:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true, // Create new process group
}
```

However, this doesn't address the issue that CrowdSec needs to run with elevated privileges to bind to ports and access system resources. The `charon` user cannot start CrowdSec with the necessary permissions.

### 5. **Config File Path Mismatch**

**Finding:** The entrypoint script creates a symlink from `/etc/crowdsec` → `/app/data/crowdsec/config`, but the CrowdSec binary is started with `-c /app/data/crowdsec/config/config.yaml`. The config file references `/etc/crowdsec` paths:

```yaml
# From /app/data/crowdsec/config/config.yaml
config_paths:
  config_dir: /etc/crowdsec/
  data_dir: /var/lib/crowdsec/data/
  # ... other paths under /etc/crowdsec
```

When CrowdSec starts, it follows the symlink correctly, but there's no validation that the symlink is intact or that the paths are accessible by the `charon` user.

## Impact Assessment

- **Critical:** CrowdSec does not start automatically on container startup
- **High:** Manual start via GUI times out after 30 seconds (HTTP handler timeout)
- **Medium:** LAPI is unavailable, so Caddy bouncer cannot function
- **Medium:** Security features (ban decisions, threat detection) are non-functional
- **Low:** Error logs spam the container logs every 10 seconds

## Proposed Solution

### Phase 1: Fix Reconciliation Timing (Immediate)

**Goal:** Ensure `ReconcileCrowdSecOnStartup` is called DURING the entrypoint script phase, not after HTTP server startup.

**Changes Required:**

1. **Move reconciliation to entrypoint script**
   - **File:** [.docker/docker-entrypoint.sh](/.docker/docker-entrypoint.sh)
   - **Action:** Add a call to start CrowdSec directly from the entrypoint script when `SECURITY_CROWDSEC_MODE=local` is set
   - **Location:** After line 180 (after "CrowdSec configuration initialized" message)
   - **Logic:**
     ```bash
     # Check if CrowdSec should auto-start based on environment variable
     if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
         echo "Starting CrowdSec in local mode..."
         # Start as background daemon
         /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml &
         CROWDSEC_PID=$!
         echo "CrowdSec started (PID: $CROWDSEC_PID)"

         # Wait for LAPI to be ready (max 30 seconds)
         LAPI_READY=0
         for i in $(seq 1 30); do
             if cscli lapi status -c /app/data/crowdsec/config/config.yaml 2>/dev/null; then
                 LAPI_READY=1
                 echo "CrowdSec LAPI is ready!"
                 break
             fi
             sleep 1
         done

         if [ "$LAPI_READY" -eq 0 ]; then
             echo "WARNING: CrowdSec LAPI not ready after 30 seconds"
         fi
     fi
     ```

2. **Remove goroutine call from routes.go**
   - **File:** [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go)
   - **Action:** Comment out or remove the goroutine call to `ReconcileCrowdSecOnStartup` (around line 466)
   - **Reason:** Reconciliation should happen in entrypoint, not after HTTP server starts

3. **Add environment variable to docker-compose**
   - **File:** [docker-compose.test.yml](docker-compose.test.yml) and other compose files
   - **Action:** Add `SECURITY_CROWDSEC_MODE: local` to the environment variables
   - **Purpose:** Control automatic startup behavior

### Phase 2: Fix Permission Issues (Critical)

**Goal:** Ensure CrowdSec can write to its data directory and bind to LAPI port.

**Changes Required:**

1. **Fix data directory ownership in Dockerfile**
   - **File:** [Dockerfile](Dockerfile)
   - **Action:** Add ownership fix for CrowdSec directories during build phase
   - **Location:** After line 270 (after GeoIP setup, before final chown)
   - **Change:**
     ```dockerfile
     # Create CrowdSec directories with correct ownership
     RUN mkdir -p /var/lib/crowdsec/data /var/log/crowdsec /etc/crowdsec && \
         chown -R charon:charon /var/lib/crowdsec /var/log/crowdsec /etc/crowdsec
     ```

2. **Update entrypoint script to fix permissions**
   - **File:** [.docker/docker-entrypoint.sh](/.docker/docker-entrypoint.sh)
   - **Action:** Add permission fix before starting CrowdSec
   - **Location:** In the CrowdSec startup block (after config initialization)
   - **Change:**
     ```bash
     # Ensure correct ownership of CrowdSec directories
     # Note: This must run as root, so place before su-exec to charon user
     chown -R charon:charon /var/lib/crowdsec /var/log/crowdsec 2>/dev/null || true
     ```

3. **Run CrowdSec as charon user**
   - **File:** [.docker/docker-entrypoint.sh](/.docker/docker-entrypoint.sh)
   - **Action:** Use `su-exec charon` to start CrowdSec
   - **Change:**
     ```bash
     # Start CrowdSec as charon user (not root)
     su-exec charon /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml &
     CROWDSEC_PID=$!
     ```

### Phase 3: Fix LAPI Binding Issue (Critical)

**Goal:** Ensure LAPI can bind to port 8085 without permission errors.

**Root Cause:** Port 8085 doesn't require elevated privileges (only ports <1024 do), so this should work. However, we need to verify the LAPI configuration is correct and the process can actually bind.

**Changes Required:**

1. **Verify LAPI port configuration**
   - **File:** None (configuration check)
   - **Action:** Confirm the entrypoint script's `sed` commands correctly set port 8085 in config.yaml
   - **Current:** Lines 151-155 in docker-entrypoint.sh already do this
   - **Verification:** Add debug logging to confirm sed operations succeeded

2. **Add startup validation**
   - **File:** [.docker/docker-entrypoint.sh](/.docker/docker-entrypoint.sh)
   - **Action:** After starting CrowdSec, verify LAPI is listening
   - **Change:**
     ```bash
     # Verify LAPI is listening on port 8085
     if netstat -tuln | grep -q ':8085 '; then
         echo "✓ CrowdSec LAPI is listening on port 8085"
     else
         echo "✗ WARNING: CrowdSec LAPI is NOT listening on port 8085"
         echo "  Check /var/log/crowdsec/crowdsec.log for errors"
     fi
     ```

3. **Add netstat to Dockerfile if not present**
   - **File:** [Dockerfile](Dockerfile)
   - **Action:** Add `net-tools` or `netstat` to apk packages
   - **Location:** Line ~257 (where runtime dependencies are installed)
   - **Change:**
     ```dockerfile
     RUN apk --no-cache add bash ca-certificates sqlite-libs sqlite tzdata curl gettext su-exec net-tools \
     ```

### Phase 4: Improve Handler Timeout Handling (Medium Priority)

**Goal:** Provide better feedback when CrowdSec start takes longer than expected.

**Changes Required:**

1. **Increase start timeout in handler**
   - **File:** [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)
   - **Action:** Increase LAPI readiness timeout from 30s to 60s
   - **Location:** Line ~223 (in `Start` method)
   - **Current:** `maxWait := 30 * time.Second`
   - **Change:** `maxWait := 60 * time.Second`
   - **Reason:** LAPI startup can take 45+ seconds on slow systems

2. **Add progress updates to handler**
   - **File:** [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)
   - **Action:** Return intermediate status updates instead of blocking for 30+ seconds
   - **Option 1:** Use streaming JSON response with periodic updates
   - **Option 2:** Return 202 Accepted with a separate status endpoint
   - **Recommendation:** Option 2 (cleaner, follows REST patterns)

3. **Add dedicated status check endpoint**
   - **File:** [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)
   - **Action:** Add `/api/v1/admin/crowdsec/startup-status` endpoint
   - **Purpose:** Allow frontend to poll for startup completion
   - **Response:**
     ```json
     {
       "status": "starting|ready|failed",
       "pid": 12345,
       "lapi_ready": false,
       "elapsed_seconds": 15,
       "message": "Waiting for LAPI to bind to port 8085..."
     }
     ```

### Phase 5: Enhance Logging and Debugging (Low Priority)

**Goal:** Make it easier to diagnose CrowdSec startup issues in the future.

**Changes Required:**

1. **Add structured logging to reconciliation**
   - **File:** [backend/internal/services/crowdsec_startup.go](backend/internal/services/crowdsec_startup.go)
   - **Action:** Add more detailed logs at each decision point
   - **Examples:**
     - Log when SecurityConfig check is performed
     - Log the actual mode and enabled status values
     - Log when binary/config validation succeeds/fails
     - Log the exact command being executed to start CrowdSec

2. **Add health check script**
   - **File:** New file `scripts/crowdsec_health_check.sh`
   - **Purpose:** Standalone script to diagnose CrowdSec issues
   - **Checks:**
     - Binary exists and is executable
     - Config files exist and are valid
     - Data directory is writable
     - LAPI port is not already in use
     - Process is running and responding

3. **Add recovery mechanism**
   - **File:** [backend/internal/services/crowdsec_startup.go](backend/internal/services/crowdsec_startup.go)
   - **Action:** If verification fails after start, attempt to retrieve error logs
   - **Logic:**
     ```go
     if !verifyRunning {
         // Read last 50 lines of crowdsec.log for debugging
         logPath := filepath.Join(dataDir, "logs", "crowdsec.log")
         if logData, err := exec.Command("tail", "-n", "50", logPath).Output(); err == nil {
             logger.Log().WithField("log_tail", string(logData)).Error("CrowdSec failed to start - log excerpt")
         }
     }
     ```

## Implementation Order

1. **Phase 2 (Permissions)** - Must be done first, as this is the actual blocker
2. **Phase 3 (LAPI)** - Immediately after Phase 2, to verify binding works
3. **Phase 1 (Timing)** - Once CrowdSec can actually start, fix when it starts
4. **Phase 4 (Timeouts)** - Improve user experience after core functionality works
5. **Phase 5 (Logging)** - Nice to have for future debugging

## Testing Strategy

### Unit Tests

- [ ] Test `ReconcileCrowdSecOnStartup` with various permission scenarios
- [ ] Test `DefaultCrowdsecExecutor.Start` with non-root user
- [ ] Test LAPI readiness check with unreachable server

### Integration Tests

- [ ] Test automatic startup with `SECURITY_CROWDSEC_MODE=local`
- [ ] Test manual start via `/api/v1/admin/crowdsec/start`
- [ ] Test LAPI connectivity from Caddy bouncer
- [ ] Test container restart preserves CrowdSec state

### Manual Verification Steps

1. **Build and run test container:**
   ```bash
   docker build -t charon:test .
   docker compose -f docker-compose.test.yml up -d
   ```

2. **Verify CrowdSec auto-started:**
   ```bash
   docker compose -f docker-compose.test.yml exec charon ps aux | grep crowdsec
   ```

3. **Verify LAPI is listening:**
   ```bash
   docker compose -f docker-compose.test.yml exec charon netstat -tuln | grep 8085
   ```

4. **Verify Caddy bouncer can connect:**
   ```bash
   docker compose -f docker-compose.test.yml logs charon | grep -i "crowdsec.*ready"
   ```

5. **Test manual stop/start:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/admin/crowdsec/stop
   curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
   ```

6. **Verify decisions endpoint works:**
   ```bash
   curl http://localhost:8080/api/v1/admin/crowdsec/decisions
   ```

## Rollback Plan

If changes break the build or runtime:

1. Revert Dockerfile changes:
   ```bash
   git checkout HEAD -- Dockerfile
   ```

2. Revert entrypoint script:
   ```bash
   git checkout HEAD -- .docker/docker-entrypoint.sh
   ```

3. Rebuild and test:
   ```bash
   docker build -t charon:rollback .
   docker compose -f docker-compose.test.yml up -d
   ```

## Success Criteria

- [ ] CrowdSec process starts automatically on container startup
- [ ] LAPI binds to port 8085 successfully
- [ ] Caddy bouncer can connect to LAPI within 30 seconds
- [ ] Manual start via GUI completes within 60 seconds
- [ ] Container logs show "CrowdSec LAPI is ready" message
- [ ] Decisions endpoint returns valid data (empty array is OK)
- [ ] Container restart preserves CrowdSec running state
- [ ] All existing CrowdSec tests pass
- [ ] No permission errors in logs

## Future Improvements

1. **Add CrowdSec metrics to Prometheus endpoint**
   - Expose LAPI status, decision count, parser stats
2. **Add GUI indicators for LAPI health**
   - Show "LAPI Ready" badge in security dashboard
3. **Add automatic restart on crash**
   - Implement watchdog that restarts CrowdSec if it dies
4. **Add configuration validation on save**
   - Use `crowdsec -c <config> -t` before applying changes
5. **Add log streaming for CrowdSec logs**
   - Expose `/var/log/crowdsec/crowdsec.log` via WebSocket

## References

- [CrowdSec Documentation](https://docs.crowdsec.net/)
- [CrowdSec LAPI Reference](https://docs.crowdsec.net/docs/local_api/intro)
- [Caddy CrowdSec Bouncer Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)
- [Issue #16: ACL Implementation](ISSUE_16_ACL_IMPLEMENTATION.md) (related security feature)
- [Integration Test: crowdsec_integration_test.go](backend/integration/crowdsec_integration_test.go)

## Review and Approval

- [ ] Reviewed by: _____________
- [ ] Approved by: _____________
- [ ] Implementation assigned to: _____________
- [ ] Target completion date: _____________
