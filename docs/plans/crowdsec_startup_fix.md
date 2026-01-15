# CrowdSec Startup Fix Plan

**Date:** 2025-12-22
**Updated:** 2025-12-23 (Post-Implementation Investigation)
**Status:** FAILED - Requires Additional Fixes
**Priority:** CRITICAL

## Current State (2025-12-23 Investigation)

**CrowdSec is NOT starting due to PERMISSION ERRORS.** The initial fix was implemented but did NOT address the actual root causes.

### Actual Error Messages from Container Logs

```
Failed to write to log, can't open new logfile: open /var/log/crowdsec.log: permission denied

FATAL unable to create database client: unable to set perms on /var/lib/crowdsec/data/crowdsec.db: chmod /var/lib/crowdsec/data/crowdsec.db: operation not permitted

{"level":"warning","msg":"CrowdSec started but LAPI not ready within timeout","pid":316,"time":"2025-12-22T21:04:00-05:00"}
```

### File Ownership Issues (VERIFIED)

```bash
# Database file owned by root - CrowdSec can't chmod it
$ stat -c '%U:%G %n' /var/lib/crowdsec/data/crowdsec.db
root:root /var/lib/crowdsec/data/crowdsec.db

# Config files owned by root - created by entrypoint running as root
$ stat -c '%U:%G %n' /app/data/crowdsec/config/config.yaml /app/data/crowdsec/config/user.yaml
root:root /app/data/crowdsec/config/config.yaml
root:root /app/data/crowdsec/config/user.yaml
```

### CrowdSec Config Problem (CRITICAL)

The `config.yaml` has `log_dir: /var/log/` (wrong path):

```yaml
common:
  log_dir: /var/log/          # <-- WRONG: Should be /var/log/crowdsec/
  log_media: file
```

CrowdSec is trying to write to `/var/log/crowdsec.log` but `/var/log/` is owned by root. The correct path should be `/var/log/crowdsec/` which is owned by charon.

## Root Cause Analysis (UPDATED)

### 1. **Entrypoint Script Runs CrowdSec Commands as Root**

**Finding:** The entrypoint script runs `cscli machines add -a --force` and `envsubst` on config files **while still running as root**. These operations:

- Create `/var/lib/crowdsec/data/crowdsec.db` owned by root
- Overwrite `config.yaml` and `user.yaml` with root ownership

**Evidence from entrypoint:**

```bash
# These run as root BEFORE `su-exec charon` is used
cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"
envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
```

### 2. **CrowdSec Log Path Configuration Error**

**Finding:** The distributed `config.yaml` has `log_dir: /var/log/` instead of `log_dir: /var/log/crowdsec/`.

**Evidence:**

```yaml
# Current (WRONG):
log_dir: /var/log/

# Should be:
log_dir: /var/log/crowdsec/
```

### 3. **ReconcileCrowdSecOnStartup IS Being Called (VERIFIED)**

**Finding:** The reconciliation function is now correctly called in [backend/cmd/api/main.go#L144](backend/cmd/api/main.go#L144) BEFORE the HTTP server starts:

```go
crowdsecExec := handlers.NewDefaultCrowdsecExecutor()
services.ReconcileCrowdSecOnStartup(db, crowdsecExec, crowdsecBinPath, crowdsecDataDir)
```

This is CORRECT but CrowdSec still fails due to permission issues.

### 4. **CrowdSec Start Method is Correct (VERIFIED)**

**Finding:** The executor's `Start` method correctly uses `os/exec` without context cancellation:

```go
cmd := exec.Command(binPath, "-c", configFile)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```

The binary starts but immediately crashes due to permission denied errors.

## What Was Implemented vs What Actually Happened

| Item | Expected | Actual |
|------|----------|--------|
| Reconciliation in main.go | ✅ Added | ✅ Called on startup |
| Dockerfile chown for CrowdSec dirs | ✅ Added | ❌ Overwritten at runtime by entrypoint |
| Goroutine removed from routes.go | ✅ Removed | ✅ Confirmed removed |
| Entrypoint permission fix | ❌ Not implemented | ❌ Root operations create root-owned files |
| Config log_dir fix | ❌ Not implemented | ❌ Still pointing to /var/log/ |

## REQUIRED FIXES (Specific Code Changes)

### FIX 1: Change CrowdSec log_dir in Entrypoint (CRITICAL)

**File:** `.docker/docker-entrypoint.sh`
**Location:** After line 155 (after `sed -i 's|listen_uri.*|listen_uri: 127.0.0.1:8085|g'`)
**Add:**

```bash
# Fix log_dir path - must point to /var/log/crowdsec/ not /var/log/
if [ -f "/etc/crowdsec/config.yaml" ]; then
    sed -i 's|log_dir: /var/log/$|log_dir: /var/log/crowdsec/|g' /etc/crowdsec/config.yaml
    sed -i 's|log_dir: /var/log/\s*$|log_dir: /var/log/crowdsec/|g' /etc/crowdsec/config.yaml
fi
```

### FIX 2: Run cscli Commands as charon User (CRITICAL)

**File:** `.docker/docker-entrypoint.sh`
**Change:** All `cscli` commands must run as `charon` user, not root.

**Current (WRONG):**

```bash
cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"
```

**Required (CORRECT):**

```bash
su-exec charon cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"
```

### FIX 3: Run envsubst as charon User (CRITICAL)

**File:** `.docker/docker-entrypoint.sh`
**Change:** The envsubst operations must preserve charon ownership.

**Current (WRONG):**

```bash
for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
    if [ -f "$file" ]; then
        envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
    fi
done
```

**Required (CORRECT):**

```bash
for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
    if [ -f "$file" ]; then
        envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
        chown charon:charon "$file" 2>/dev/null || true
    fi
done
```

### FIX 4: Fix Ownership AFTER cscli Operations (CRITICAL)

**File:** `.docker/docker-entrypoint.sh`
**Location:** After all cscli operations, before "CrowdSec configuration initialized" message
**Add:**

```bash
# Fix ownership of files created by cscli (runs as root, creates root-owned files)
# The database and config files must be owned by charon for CrowdSec to start
chown -R charon:charon /var/lib/crowdsec 2>/dev/null || true
chown -R charon:charon /app/data/crowdsec 2>/dev/null || true
chown -R charon:charon /var/log/crowdsec 2>/dev/null || true
```

### FIX 5: Update Default config.yaml in configs/crowdsec/ (PREVENTIVE)

**File:** `configs/crowdsec/config.yaml` (if exists) or modify the distributed template
**Change:** Ensure log_dir is correct in the source template:

```yaml
common:
  daemonize: true
  log_media: file
  log_level: info
  log_dir: /var/log/crowdsec/   # <-- CORRECT PATH
```

## Complete Entrypoint Script Fix

Here's the corrected CrowdSec section for `.docker/docker-entrypoint.sh`:

```bash
# ============================================================================
# CrowdSec Initialization
# ============================================================================

if command -v cscli >/dev/null; then
    echo "Initializing CrowdSec configuration..."

    # Define persistent paths
    CS_PERSIST_DIR="/app/data/crowdsec"
    CS_CONFIG_DIR="$CS_PERSIST_DIR/config"
    CS_DATA_DIR="$CS_PERSIST_DIR/data"
    CS_LOG_DIR="/var/log/crowdsec"

    # Ensure persistent directories exist
    mkdir -p "$CS_CONFIG_DIR" "$CS_DATA_DIR" "$CS_LOG_DIR" 2>/dev/null || true
    mkdir -p /var/lib/crowdsec/data 2>/dev/null || true

    # Initialize persistent config if key files are missing
    if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
        echo "Initializing persistent CrowdSec configuration..."
        if [ -d "/etc/crowdsec.dist" ] && [ -n "$(ls -A /etc/crowdsec.dist 2>/dev/null)" ]; then
            cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/" || exit 1
            echo "Successfully initialized config from .dist directory"
        fi
    fi

    # Create acquisition config
    if [ ! -f "/etc/crowdsec/acquis.yaml" ] || [ ! -s "/etc/crowdsec/acquis.yaml" ]; then
        cat > /etc/crowdsec/acquis.yaml << 'ACQUIS_EOF'
source: file
filenames:
  - /var/log/caddy/access.log
  - /var/log/caddy/*.log
labels:
  type: caddy
ACQUIS_EOF
    fi

    # Environment substitution (preserving ownership after)
    export CFG=/etc/crowdsec
    export DATA="$CS_DATA_DIR"
    export PID=/var/run/crowdsec.pid
    export LOG="$CS_LOG_DIR/crowdsec.log"

    for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
        if [ -f "$file" ]; then
            envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
            chown charon:charon "$file" 2>/dev/null || true
        fi
    done

    # Configure LAPI port (8085 instead of 8080)
    if [ -f "/etc/crowdsec/config.yaml" ]; then
        sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
        sed -i 's|listen_uri: 0.0.0.0:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
        # FIX: Correct log_dir path
        sed -i 's|log_dir: /var/log/$|log_dir: /var/log/crowdsec/|g' /etc/crowdsec/config.yaml
    fi

    # Update local_api_credentials.yaml to use correct port
    if [ -f "/etc/crowdsec/local_api_credentials.yaml" ]; then
        sed -i 's|url: http://127.0.0.1:8080|url: http://127.0.0.1:8085|g' /etc/crowdsec/local_api_credentials.yaml
        sed -i 's|url: http://localhost:8080|url: http://127.0.0.1:8085|g' /etc/crowdsec/local_api_credentials.yaml
    fi

    # Update hub index
    if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
        echo "Updating CrowdSec hub index..."
        timeout 60s cscli hub update 2>/dev/null || echo "⚠️ Hub update timed out"
    fi

    # Register local machine (run as charon or fix ownership after)
    echo "Registering local machine..."
    cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration failed"

    # *** CRITICAL FIX: Fix ownership of ALL CrowdSec files after cscli operations ***
    # cscli runs as root and creates root-owned files (crowdsec.db, config files)
    # CrowdSec process runs as charon and needs write access
    echo "Fixing CrowdSec file ownership..."
    chown -R charon:charon /var/lib/crowdsec 2>/dev/null || true
    chown -R charon:charon /app/data/crowdsec 2>/dev/null || true
    chown -R charon:charon /var/log/crowdsec 2>/dev/null || true

    echo "CrowdSec configuration initialized. Agent lifecycle is GUI-controlled."
fi
```

## Testing After Fix

1. **Rebuild container:**

   ```bash
   docker build -t charon:local . && docker compose -f docker-compose.test.yml up -d
   ```

2. **Verify ownership is correct:**

   ```bash
   docker compose -f docker-compose.test.yml exec charon ls -la /var/lib/crowdsec/data/
   # Expected: all files owned by charon:charon
   ```

3. **Check CrowdSec logs for permission errors:**

   ```bash
   docker compose -f docker-compose.test.yml logs charon 2>&1 | grep -i "permission\|denied\|FATAL"
   # Expected: no permission errors
   ```

4. **Verify LAPI is listening after manual start:**

   ```bash
   curl -X POST http://localhost:8080/api/v1/admin/crowdsec/start
   docker compose -f docker-compose.test.yml exec charon ss -tuln | grep 8085
   # Expected: LISTEN on :8085
   ```

## Success Criteria (Updated)

- [ ] All files in `/var/lib/crowdsec/` owned by `charon:charon`
- [ ] All files in `/app/data/crowdsec/` owned by `charon:charon`
- [ ] `config.yaml` has `log_dir: /var/log/crowdsec/`
- [ ] No "permission denied" errors in container logs
- [ ] CrowdSec LAPI binds to port 8085 successfully
- [ ] Manual start via GUI completes without timeout
- [ ] Reconciliation on startup works when mode=local

## References

- [CrowdSec Documentation](https://docs.crowdsec.net/)
- [CrowdSec LAPI Reference](https://docs.crowdsec.net/docs/local_api/intro)
- [Caddy CrowdSec Bouncer Plugin](https://github.com/hslatman/caddy-crowdsec-bouncer)
- [Issue #16: ACL Implementation](ISSUE_16_ACL_IMPLEMENTATION.md) (related security feature)

## Changelog

### 2025-12-23 - Investigation Update

- **Status:** FAILED - Previous implementation did not fix root cause
- **Finding:** Permission errors due to entrypoint running cscli as root
- **Finding:** log_dir config points to wrong path (/var/log/ vs /var/log/crowdsec/)
- **Action:** Updated plan with specific entrypoint script fixes
- **Priority:** Escalated to CRITICAL

### 2025-12-22 - Initial Plan

- Created initial plan based on code review
- Identified timing issue with goroutine call
- Proposed moving reconciliation to main.go (implemented)
