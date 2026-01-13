# Migration Guide: CrowdSec Auto-Start Behavior

**Effective Version:** v0.9.0+
**Last Updated:** December 23, 2025

---

## Overview

Starting in version 0.9.0, CrowdSec now **automatically starts** when the container restarts, if it was previously enabled. This eliminates the need for manual intervention after server reboots or container updates.

**Key Behavioral Changes:**

| Scenario | Before (v0.8.x) | After (v0.9.0+) |
|----------|-----------------|-----------------|
| **Container Restart** | CrowdSec stays offline | CrowdSec auto-starts if enabled |
| **Server Reboot** | Manual start required | CrowdSec auto-starts if enabled |
| **Docker Compose Up** | CrowdSec offline | CrowdSec auto-starts if enabled |
| **Container Update** | Manual start required | CrowdSec auto-starts if enabled |

---

## What Changed?

### 1. Reconciliation Moved to Startup Phase

**Before (v0.8.x):**

```
Container Start → HTTP Server → Routes Registered → Reconciliation (too late)
```

**After (v0.9.0+):**

```
Container Start → Database Migrations → Reconciliation → HTTP Server
```

**Impact:** CrowdSec now starts within 10-15 seconds of container boot, before the HTTP server accepts requests.

### 2. Mutex Protection Added

**Before (v0.8.x):** No protection against concurrent reconciliation calls (race condition risk)

**After (v0.9.0+):** Mutex prevents multiple reconciliation attempts from interfering

**Impact:** Safer, more predictable startup behavior

### 3. Permission Fix

**Before (v0.8.x):** CrowdSec directories owned by `root:root` (permission errors)

**After (v0.9.0+):** CrowdSec directories owned by `charon:charon` (correct permissions)

**Impact:** CrowdSec can write to its database and log files without permission errors

### 4. Timeout Increased

**Before (v0.8.x):** 30-second timeout for LAPI readiness

**After (v0.9.0+):** 60-second timeout for LAPI readiness

**Impact:** Slower systems (Raspberry Pi, HDD) have enough time for LAPI to initialize

---

## Migration Paths

### Path A: Fresh Installation (v0.9.0+)

**No action required.** CrowdSec is disabled by default. Enable via Security dashboard when ready.

**Steps:**

1. Deploy Charon v0.9.0+
2. Navigate to Security dashboard
3. Toggle CrowdSec ON
4. Wait 10-15 seconds for LAPI to initialize
5. Verify status shows "Active"

**Result:** CrowdSec will auto-start on future container restarts

---

### Path B: Upgrade from v0.8.x (CrowdSec Disabled)

**No action required.** Your current state (disabled) will be preserved.

**What Happens:**

1. Container starts with new reconciliation logic
2. Reconciliation checks SecurityConfig and Settings tables
3. Both indicate CrowdSec disabled
4. CrowdSec stays offline (as expected)

**Verification:**

```bash
# After upgrade, verify CrowdSec is still disabled
docker exec charon cscli lapi status

# Expected output:
# Error: can't init client: no credentials or machine found
```

**Enable When Ready:**

- Navigate to Security dashboard
- Toggle CrowdSec ON
- Auto-start will work on future restarts

---

### Path C: Upgrade from v0.8.x (CrowdSec Enabled)

**Recommended action:** Restart container after upgrade to trigger auto-start.

**Migration Steps:**

1. **Before Upgrade:** Note CrowdSec status

   ```bash
   docker exec charon cscli lapi status
   # Expected: ✓ You can successfully interact with Local API (LAPI)
   ```

2. **Upgrade to v0.9.0+:**

   ```bash
   docker compose pull
   docker compose up -d
   ```

3. **Wait 15 seconds** for reconciliation to complete

4. **Verify CrowdSec auto-started:**

   ```bash
   docker exec charon cscli lapi status
   ```

   Expected output:

   ```
   ✓ You can successfully interact with Local API (LAPI)
   ```

5. **Check reconciliation logs:**

   ```bash
   docker logs charon 2>&1 | grep "CrowdSec reconciliation"
   ```

   Expected output:

   ```json
   {"level":"info","msg":"CrowdSec reconciliation: starting startup check"}
   {"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'"}
   {"level":"info","msg":"CrowdSec reconciliation: successfully started and verified CrowdSec","pid":123}
   ```

**If CrowdSec Didn't Auto-Start:**

See [Troubleshooting](#troubleshooting) section below.

---

### Path D: Upgrade with Environment Variables (DEPRECATED)

**⚠️ Action Required:** Remove environment variables and use GUI toggle instead.

**Old Configuration (v0.8.x):**

```yaml
services:
  charon:
    environment:
      - SECURITY_CROWDSEC_MODE=local
      - CHARON_SECURITY_CROWDSEC_MODE=local
```

**New Configuration (v0.9.0+):**

```yaml
services:
  charon:
    # Remove environment variables - CrowdSec is now GUI-controlled
    environment:
      - CHARON_ENV=production
```

**Migration Steps:**

1. **Note current CrowdSec state:**

   ```bash
   docker exec charon cscli lapi status
   ```

2. **Edit docker-compose.yml:**
   - Remove `SECURITY_CROWDSEC_MODE` lines
   - Remove `CHARON_SECURITY_CROWDSEC_MODE` lines

3. **Restart container:**

   ```bash
   docker compose down
   docker compose up -d
   ```

4. **If CrowdSec was previously enabled:**
   - Navigate to Security dashboard
   - Toggle CrowdSec ON
   - Verify auto-start on next restart:

     ```bash
     docker restart charon
     sleep 15
     docker exec charon cscli lapi status
     ```

**Why Remove Environment Variables:**

- Consistent behavior with other security features (WAF, ACL, Rate Limiting)
- Single source of truth (database, not environment)
- Easier to manage via GUI
- No need to edit docker-compose.yml for security settings

---

## Auto-Start Behavior Explained

### Decision Logic

CrowdSec auto-starts on container boot if **ANY** of these conditions are true:

1. **SecurityConfig table:** `crowdsec_mode = "local"`
2. **Settings table:** `security.crowdsec.enabled = "true"`

**Pseudocode:**

```
IF SecurityConfig.crowdsec_mode == "local" THEN
    LOG "Starting based on SecurityConfig mode='local'"
    START CrowdSec
ELSE IF Settings["security.crowdsec.enabled"] == "true" THEN
    LOG "Starting based on Settings table override"
    START CrowdSec
ELSE
    LOG "Both SecurityConfig and Settings indicate disabled"
    SKIP (CrowdSec stays offline)
END IF
```

### Two-Source Priority

**Why two sources?**

- **SecurityConfig (primary):** New, structured, strongly typed
- **Settings (fallback):** Legacy support, runtime toggles

**Initialization Flow:**

```
Container Boot
    ↓
Database Migrations (ensures SecurityConfig table exists)
    ↓
Reconciliation Checks SecurityConfig
    ↓
    ├─ SecurityConfig exists?
    │   ├─ Yes: Use SecurityConfig.crowdsec_mode
    │   └─ No: Check Settings table
    │       ├─ Settings["security.crowdsec.enabled"] == "true"?
    │       │   ├─ Yes: Create SecurityConfig with mode="local"
    │       │   └─ No: Create SecurityConfig with mode="disabled"
    │       └─ Use newly created SecurityConfig
    └─ Start CrowdSec if mode == "local"
```

### Persistence Guarantees

| Action | Persists Across Restart? |
|--------|--------------------------|
| **Toggle ON via GUI** | ✅ Yes (stored in database) |
| **Toggle OFF via GUI** | ✅ Yes (stored in database) |
| **Environment variable** | ❌ No (deprecated, not used) |
| **Volume deletion** | ❌ No (database reset) |
| **Container recreation** | ✅ Yes (if volume preserved) |

---

## Timing Expectations

### Container Boot Sequence

| Phase | Duration | Cumulative | Status |
|-------|----------|------------|--------|
| **Container Start** | 1-2s | 1-2s | Entrypoint script running |
| **Database Migrations** | 1-2s | 2-4s | Security tables created/updated |
| **CrowdSec Reconciliation** | 2-5s | 4-9s | Process started, verifying |
| **HTTP Server Start** | 1s | 5-10s | API ready for requests |
| **LAPI Initialization** | 5-10s | 10-20s | CrowdSec fully operational |

**Total Time to CrowdSec Ready:** 10-20 seconds on average systems

### LAPI Initialization Phases

| Phase | Duration | Description |
|-------|----------|-------------|
| **Process Start** | 1-2s | CrowdSec binary launches |
| **Config Loading** | 2-3s | Parsers, scenarios loaded |
| **Database Init** | 1-2s | SQLite connection established |
| **Hub Update** | 3-8s | Security rule index updated |
| **LAPI Binding** | 1s | HTTP server starts on :8085 |
| **Health Check** | 1s | First successful LAPI query |

**Slowest Systems:** Up to 45 seconds (Raspberry Pi with slow SD card)

---

## Verification Steps

### Step 1: Verify Auto-Start Worked

```bash
# After container restart
docker restart charon

# Wait for startup to complete
sleep 20

# Check CrowdSec status
docker exec charon cscli lapi status
```

**Expected Output (Success):**

```
✓ You can successfully interact with Local API (LAPI)
```

**Expected Output (Failure):**

```
Error: can't init client: no credentials or machine found
```

### Step 2: Check Reconciliation Logs

```bash
docker logs charon 2>&1 | grep "CrowdSec reconciliation"
```

**Expected Output (Auto-Started):**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check","bin_path":"/usr/local/bin/crowdsec","data_dir":"/app/data/crowdsec"}
{"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'","mode":"local"}
{"level":"info","msg":"CrowdSec reconciliation: successfully started and verified CrowdSec","pid":123,"verified":true}
```

**Expected Output (Skipped - Disabled):**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting startup check","bin_path":"/usr/local/bin/crowdsec","data_dir":"/app/data/crowdsec"}
{"level":"info","msg":"CrowdSec reconciliation skipped: both SecurityConfig and Settings indicate disabled","db_mode":"disabled","setting_enabled":false}
```

### Step 3: Verify Database State

```bash
# Check SecurityConfig table
docker exec charon sqlite3 /app/data/charon.db \
  "SELECT uuid, crowdsec_mode, enabled FROM security_configs LIMIT 1;"
```

**Expected Output (Enabled):**

```
default|local|1
```

**Expected Output (Disabled):**

```
default|disabled|0
```

### Step 4: Verify Process Running

```bash
# Check CrowdSec process
docker exec charon ps aux | grep crowdsec | grep -v grep
```

**Expected Output:**

```
charon     123  0.5  1.2  50000 12000 ?        Sl   10:30   0:01 /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml
```

### Step 5: Verify LAPI Listening

```bash
# Check port 8085
docker exec charon netstat -tuln | grep 8085
```

**Expected Output:**

```
tcp        0      0 127.0.0.1:8085          0.0.0.0:*               LISTEN
```

---

## Troubleshooting

### Issue: CrowdSec Not Auto-Starting

**Symptoms:**

- Container restarts successfully
- CrowdSec status shows "Offline"
- `cscli lapi status` returns error

**Diagnosis:**

1. **Check reconciliation logs:**

   ```bash
   docker logs charon 2>&1 | grep "CrowdSec reconciliation"
   ```

2. **Check SecurityConfig mode:**

   ```bash
   docker exec charon sqlite3 /app/data/charon.db \
     "SELECT crowdsec_mode FROM security_configs LIMIT 1;"
   ```

   Expected: `local`
   Actual: `disabled` → **Root Cause: User disabled CrowdSec**

3. **Check Settings table:**

   ```bash
   docker exec charon sqlite3 /app/data/charon.db \
     "SELECT value FROM settings WHERE key='security.crowdsec.enabled';"
   ```

   Expected: `true`
   Actual: `false` or empty → **Root Cause: Setting not configured**

**Resolution:**

**If mode is disabled:**

```bash
# Enable via GUI (recommended)
# OR manually update database:
docker exec charon sqlite3 /app/data/charon.db \
  "UPDATE security_configs SET crowdsec_mode='local', enabled=1;"
docker restart charon
```

**If table missing:**

```bash
# Run migrations
docker exec charon /app/charon migrate
docker restart charon
```

### Issue: Permission Denied Errors

**Symptoms:**

- CrowdSec process starts but immediately exits
- Logs show: "permission denied: /var/lib/crowdsec/data/crowdsec.db"

**Diagnosis:**

```bash
# Check directory ownership
docker exec charon ls -la /var/lib/crowdsec/data/
```

Expected: `charon:charon`
Actual: `root:root` → **Root Cause: Old Dockerfile (pre-v0.9.0)**

**Resolution:**

```bash
# Rebuild container with new Dockerfile
docker compose down
docker compose build --no-cache
docker compose up -d
```

### Issue: LAPI Timeout

**Symptoms:**

- CrowdSec starts but LAPI never becomes ready
- Timeout after 60 seconds

**Diagnosis:**

```bash
# Check LAPI logs
docker exec charon tail -50 /var/log/crowdsec/crowdsec.log

# Check system resources
docker stats charon
```

**Common Causes:**

- Low memory (< 512MB)
- Slow disk I/O
- Network timeout (hub update)

**Resolution:**

```bash
# Increase memory allocation in docker-compose.yml
services:
  charon:
    deploy:
      resources:
        limits:
          memory: 1G

# Restart container
docker compose restart
```

### Issue: Multiple CrowdSec Processes

**Symptoms:**

- Multiple `crowdsec` processes running
- Error: "address already in use: 127.0.0.1:8085"

**Diagnosis:**

```bash
docker exec charon ps aux | grep crowdsec | grep -v grep
```

Expected: 1 process
Actual: 2+ processes → **Root Cause: Race condition (should not happen in v0.9.0+ due to mutex)**

**Resolution:**

```bash
# Kill all CrowdSec processes
docker exec charon pkill crowdsec

# Start cleanly via GUI
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
```

---

## Rollback Procedure

If you encounter issues with v0.9.0+ and need to rollback:

### Step 1: Stop Current Container

```bash
docker compose down
```

### Step 2: Rollback to v0.8.x

```yaml
# docker-compose.yml
services:
  charon:
    image: ghcr.io/wikid82/charon:v0.8.5  # or your previous version
```

### Step 3: Restart Container

```bash
docker compose up -d
```

### Step 4: Manual CrowdSec Start (if needed)

```bash
# If CrowdSec was previously enabled
curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
```

### Step 5: Report Issue

Please report rollback necessity on [GitHub Issues](https://github.com/Wikid82/charon/issues) with:

- Container logs: `docker logs charon`
- System info: `docker info`
- CrowdSec logs: `docker exec charon tail -50 /var/log/crowdsec/crowdsec.log`

---

## FAQ

### Q: Will CrowdSec auto-start after a fresh install?

**A:** No. CrowdSec is disabled by default. You must enable it via the Security dashboard. After enabling, it will auto-start on future restarts.

### Q: Can I disable auto-start behavior?

**A:** Yes. Toggle CrowdSec OFF in the Security dashboard. It will stay disabled until you re-enable it.

### Q: What if I delete my persistent volume?

**A:** Database is reset, CrowdSec reverts to disabled state. You'll need to enable it again via the GUI.

### Q: Do environment variables still work?

**A:** No, they are deprecated and ignored in v0.9.0+. Use the GUI toggle instead.

### Q: What happens if reconciliation fails?

**A:** Container continues to start normally. CrowdSec stays offline, but the API and proxy features work. Check logs for failure reason.

### Q: Is there a performance impact?

**A:** Minimal. Reconciliation adds 2-5 seconds to container startup time. CrowdSec adds ~50MB memory and 5-10% CPU (idle).

### Q: Can I force a manual reconciliation?

**A:** Not directly. Restart the container to trigger reconciliation, or toggle CrowdSec OFF/ON via GUI.

---

## Additional Resources

- **Implementation Details:** [CrowdSec Startup Fix Documentation](implementation/crowdsec_startup_fix_COMPLETE.md)
- **User Guide:** [Getting Started - CrowdSec Setup](getting-started.md#step-15-database-migrations-if-upgrading)
- **Security Documentation:** [CrowdSec Features](security.md#crowdsec-block-bad-ips)
- **GitHub Issues:** [Report Problems](https://github.com/Wikid82/charon/issues)

---

## Change History

| Date | Version | Change |
|------|---------|--------|
| 2025-12-23 | v0.9.0 | Auto-start behavior implemented |
| 2025-12-23 | v0.9.0 | Environment variables deprecated |
| 2025-12-23 | v0.9.0 | Mutex protection added |
| 2025-12-23 | v0.9.0 | Timeout increased to 60s |

---

*For technical questions or issues, please open a [GitHub Issue](https://github.com/Wikid82/charon/issues).*
