# CrowdSec Non-Root Migration Fix Specification

## Executive Summary

The Charon container was migrated from root to non-root user (UID/GID 1000, username `charon`). This broke CrowdSec because of permission issues and config path mismatches. This document outlines the exact changes needed to fix CrowdSec operation under non-root.

**Status:** Research Complete - Ready for Implementation
**Last Updated:** 2024-12-22
**Priority:** CRITICAL

---

## Root Cause Analysis

### What Happened

1. **Container User Change**: Root → `charon` (UID 1000)
2. **Log File Permission Issue**: CrowdSec config points to `/var/log/crowdsec.log` but non-root cannot create files in `/var/log/`
3. **Config Path Mismatch**: CrowdSec expects configs at `/etc/crowdsec/` but they're stored in `/app/data/crowdsec/config/`
4. **Missing Symlink**: Entrypoint doesn't create `/etc/crowdsec` → `/app/data/crowdsec/config` symlink

### Current Container State

**User**: `charon` (UID 1000, GID 1000)

**Directory Permissions**:

```
✓ /var/log/crowdsec/      - charon:charon (correct)
✓ /var/log/caddy/         - charon:charon (correct)
✓ /app/data/crowdsec/     - charon:charon (correct)
✗ /etc/crowdsec/          - exists but NOT symlinked to persistent storage
```

**CrowdSec Config Issues** (`/app/data/crowdsec/config/config.yaml`):

```yaml
common:
  log_media: file
  log_dir: /var/log/           # ✗ Wrong - should be /var/log/crowdsec/

config_paths:
  config_dir: /etc/crowdsec/   # ✗ Not symlinked to persistent storage
  data_dir: /var/lib/crowdsec/data/  # ✗ Wrong - should be /app/data/crowdsec/data
  simulation_path: /etc/crowdsec/simulation.yaml  # ✗ File doesn't exist
  hub_dir: /etc/crowdsec/hub/  # ✓ Works via actual directory
```

---

## Fix Implementation Plan

### File 1: `.docker/docker-entrypoint.sh`

**Priority**: CRITICAL
**Lines to Modify**: 48-120 (CrowdSec initialization section)

#### Changes Required

1. **Fix log directory path** in config.yaml
2. **Create symlink** `/etc/crowdsec` → `/app/data/crowdsec/config`
3. **Update envsubst variables** to use correct paths
4. **Update data_dir path** in config.yaml

#### Implementation

Replace the section from line 48 onwards with:

```bash
# ============================================================================
# CrowdSec Initialization
# ============================================================================
# Note: CrowdSec agent is not auto-started. Lifecycle is GUI-controlled via backend handlers.

# Initialize CrowdSec configuration if cscli is present
if command -v cscli >/dev/null; then
    echo "Initializing CrowdSec configuration..."

    # Define persistent paths
    CS_PERSIST_DIR="/app/data/crowdsec"
    CS_CONFIG_DIR="$CS_PERSIST_DIR/config"
    CS_DATA_DIR="$CS_PERSIST_DIR/data"
    CS_LOG_DIR="/var/log/crowdsec"

    # Ensure persistent directories exist (within writable volume)
    mkdir -p "$CS_CONFIG_DIR" 2>/dev/null || echo "Warning: Cannot create $CS_CONFIG_DIR"
    mkdir -p "$CS_DATA_DIR" 2>/dev/null || echo "Warning: Cannot create $CS_DATA_DIR"
    # Log directories are created at build time with correct ownership
    mkdir -p "$CS_LOG_DIR" 2>/dev/null || true
    mkdir -p /var/log/caddy 2>/dev/null || true

    # Initialize persistent config if key files are missing
    if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
        echo "Initializing persistent CrowdSec configuration..."
        if [ -d "/etc/crowdsec.dist" ]; then
            cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/" 2>/dev/null || echo "Warning: Could not copy dist config"
        fi
    fi

    # Create symlink from /etc/crowdsec to persistent config BEFORE envsubst
    # This ensures cscli and other tools can find configs at the standard path
    if [ ! -L "/etc/crowdsec" ]; then
        echo "Creating symlink: /etc/crowdsec -> $CS_CONFIG_DIR"
        # Remove directory if it exists (from Dockerfile)
        if [ -d "/etc/crowdsec" ] && [ ! -L "/etc/crowdsec" ]; then
            # Move any existing configs to persistent storage first
            if [ -n "$(ls -A /etc/crowdsec 2>/dev/null)" ]; then
                echo "Migrating existing /etc/crowdsec files to persistent storage..."
                cp -rn /etc/crowdsec/* "$CS_CONFIG_DIR/" 2>/dev/null || true
            fi
            rm -rf /etc/crowdsec
        fi
        ln -sf "$CS_CONFIG_DIR" /etc/crowdsec
        echo "Symlink created successfully"
    else
        echo "Symlink already exists: /etc/crowdsec -> $(readlink /etc/crowdsec)"
    fi

    # Create/update acquisition config for Caddy logs
    if [ ! -f "/etc/crowdsec/acquis.yaml" ] || [ ! -s "/etc/crowdsec/acquis.yaml" ]; then
        echo "Creating acquisition configuration for Caddy logs..."
        cat > /etc/crowdsec/acquis.yaml << 'ACQUIS_EOF'
# Caddy access logs acquisition
# CrowdSec will monitor these files for security events
source: file
filenames:
  - /var/log/caddy/access.log
  - /var/log/caddy/*.log
labels:
  type: caddy
ACQUIS_EOF
    fi

    # Ensure hub directory exists in persistent storage
    mkdir -p /etc/crowdsec/hub

    # Perform variable substitution with CORRECT paths
    export CFG="$CS_CONFIG_DIR"
    export DATA="$CS_DATA_DIR"
    export PID="$CS_PERSIST_DIR/crowdsec.pid"
    export LOG="$CS_LOG_DIR/crowdsec.log"

    # Process config.yaml and user.yaml with envsubst
    for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
        if [ -f "$file" ]; then
            envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
        fi
    done

    # Fix log_dir path in config.yaml (must be /var/log/crowdsec/ not /var/log/)
    if [ -f "/etc/crowdsec/config.yaml" ]; then
        echo "Updating config.yaml paths for non-root operation..."
        sed -i 's|log_dir: /var/log/|log_dir: /var/log/crowdsec/|g' /etc/crowdsec/config.yaml
        sed -i 's|log_dir: /var/log$|log_dir: /var/log/crowdsec|g' /etc/crowdsec/config.yaml
        sed -i 's|data_dir: /var/lib/crowdsec/data/|data_dir: /app/data/crowdsec/data/|g' /etc/crowdsec/config.yaml
        sed -i 's|data_dir: /var/lib/crowdsec/data$|data_dir: /app/data/crowdsec/data|g' /etc/crowdsec/config.yaml
    fi

    # Configure CrowdSec LAPI to use port 8085 to avoid conflict with Charon (port 8080)
    if [ -f "/etc/crowdsec/config.yaml" ]; then
        sed -i 's|listen_uri: 127.0.0.1:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
        sed -i 's|listen_uri: 0.0.0.0:8080|listen_uri: 127.0.0.1:8085|g' /etc/crowdsec/config.yaml
    fi

    # Update local_api_credentials.yaml to use correct port
    if [ -f "/etc/crowdsec/local_api_credentials.yaml" ]; then
        sed -i 's|url: http://127.0.0.1:8080|url: http://127.0.0.1:8085|g' /etc/crowdsec/local_api_credentials.yaml
        sed -i 's|url: http://localhost:8080|url: http://127.0.0.1:8085|g' /etc/crowdsec/local_api_credentials.yaml
    fi

    # Update hub index to ensure CrowdSec can start
    if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
        echo "Updating CrowdSec hub index..."
        timeout 60s cscli hub update 2>/dev/null || echo "⚠️ Hub update timed out or failed, continuing..."
    fi

    # Ensure local machine is registered (auto-heal for volume/config mismatch)
    echo "Registering local machine..."
    cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"

    # Install hub items (parsers, scenarios, collections) if local mode enabled
    if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
        echo "Installing CrowdSec hub items..."
        if [ -x /usr/local/bin/install_hub_items.sh ]; then
            /usr/local/bin/install_hub_items.sh 2>/dev/null || echo "Warning: Some hub items may not have installed"
        fi
    fi
fi

# CrowdSec Lifecycle Management:
# CrowdSec configuration is initialized above (symlinks, directories, hub updates)
# However, the CrowdSec agent is NOT auto-started in the entrypoint.
# Instead, CrowdSec lifecycle is managed by the backend handlers via GUI controls.
echo "CrowdSec configuration initialized. Agent lifecycle is GUI-controlled."
```

---

### File 2: `Dockerfile`

**Priority**: HIGH
**Lines to Modify**: 286-294, 305-309

#### Changes Required

1. **Don't create `/etc/crowdsec` as a directory** - will be symlink at runtime
2. **Keep `/etc/crowdsec.dist` for template storage**
3. **Update ownership commands** to not assume `/etc/crowdsec` is a directory

#### Implementation

**Replace lines 286-294:**

```dockerfile
# Create required CrowdSec directories in runtime image
# Note: /etc/crowdsec will be a SYMLINK to /app/data/crowdsec/config (created at runtime)
# We keep /etc/crowdsec.dist as the source template
RUN mkdir -p /etc/crowdsec.dist /etc/crowdsec.dist/acquis.d /etc/crowdsec.dist/bouncers \
             /etc/crowdsec.dist/hub /etc/crowdsec.dist/notifications \
             /var/lib/crowdsec/data /var/log/crowdsec /var/log/caddy \
             /app/data/crowdsec/config /app/data/crowdsec/data
```

**Replace lines 305-309 (ownership section):**

```dockerfile
# Security: Set ownership of all application directories to non-root charon user
# Note: /etc/crowdsec will be created as symlink at runtime by entrypoint
RUN chown -R charon:charon /app /config /var/log/crowdsec /var/log/caddy && \
    chown -R charon:charon /etc/crowdsec.dist 2>/dev/null || true && \
    chown -R charon:charon /var/lib/crowdsec 2>/dev/null || true
```

---

## Verification Checklist

After implementation, verify these conditions:

### 1. Container Startup

```bash
docker logs charon 2>&1 | grep -i crowdsec
# Expected: "CrowdSec configuration initialized"
# Expected: "Created symlink: /etc/crowdsec -> /app/data/crowdsec/config"
# No errors about permissions or missing files
```

### 2. Symlink Creation

```bash
docker exec charon ls -la /etc/crowdsec
# Expected: lrwxrwxrwx ... /etc/crowdsec -> /app/data/crowdsec/config
```

### 3. Config File Paths

```bash
docker exec charon grep -E "log_dir|data_dir|config_dir" /app/data/crowdsec/config/config.yaml
# Expected:
#   log_dir: /var/log/crowdsec/
#   data_dir: /app/data/crowdsec/data/
#   config_dir: /etc/crowdsec/  (resolves via symlink)
```

### 4. Log Directory Writability

```bash
docker exec charon test -w /var/log/crowdsec/ && echo "writable" || echo "not writable"
# Expected: writable
```

### 5. CrowdSec Start via API

```bash
# Enable CrowdSec via API
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/crowdsec/start

# Check status
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/crowdsec/status
# Expected: {"running":true,"pid":XXXX,"lapi_ready":true}
```

### 6. Manual Process Start (Direct Test)

```bash
docker exec charon /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml
# Should start without permission errors
# Check logs: docker exec charon cat /var/log/crowdsec/crowdsec.log
```

### 7. LAPI Connectivity

```bash
docker exec charon cscli lapi status
# Expected: "You can successfully interact with Local API (LAPI)"
```

---

## Testing Strategy

### Phase 1: Clean Start Test

1. Remove existing volume: `docker volume rm charon_data`
2. Start fresh container: `docker compose up -d`
3. Verify symlink and config paths
4. Enable CrowdSec via UI
5. Verify process starts successfully

### Phase 2: Upgrade Test (Migration Scenario)

1. Use existing volume with old directory structure
2. Start updated container
3. Verify entrypoint migrates old configs
4. Verify symlink creation
5. Enable CrowdSec via UI

### Phase 3: Lifecycle Test

1. Start CrowdSec via API
2. Verify LAPI becomes ready
3. Stop CrowdSec via API
4. Restart container
5. Verify CrowdSec auto-starts if enabled

### Phase 4: Hub Operations Test

1. Run `cscli hub update`
2. Install test preset via API
3. Verify files stored in correct locations
4. Check cache permissions

---

## Rollback Plan

If issues occur after implementation:

1. **Immediate Rollback**: Revert to previous container image
2. **Config Recovery**: Backup script creates timestamped copies
3. **Manual Fix**: Mount volume and fix symlink/paths manually

---

## Implementation Priority

| Priority | Task | Impact | Complexity |
|----------|------|--------|------------|
| CRITICAL | Fix `.docker/docker-entrypoint.sh` | CrowdSec won't start | Medium |
| HIGH | Update `Dockerfile` directory creation | Prevents symlink creation | Low |
| MEDIUM | Add verification tests | CI/CD coverage | Medium |
| LOW | Document in migration guide | User awareness | Low |

---

## Related Files (No Changes Needed)

✓ `backend/internal/api/handlers/crowdsec_exec.go` - Uses correct paths
✓ `backend/internal/config/config.go` - Default config is correct
✓ `backend/internal/services/crowdsec_startup.go` - Logic is correct
✓ `configs/crowdsec/acquis.yaml` - Already correct
✓ `configs/crowdsec/install_hub_items.sh` - Already correct
✓ `configs/crowdsec/register_bouncer.sh` - Already correct

---

## Additional Notes

- **CrowdSec LAPI Port**: 8085 (correctly configured to avoid port conflict with Charon on 8080)
- **Acquisition Config**: Correctly points to `/var/log/caddy/*.log`
- **Hub Cache**: Stored in `/app/data/crowdsec/hub_cache/` (writable by charon user)
- **Bouncer API Key**: Expected at `/etc/crowdsec/bouncers/caddy-bouncer.key` (will resolve via symlink)
- **PID File**: Stored at `/app/data/crowdsec/crowdsec.pid` (correct location)

---

## Success Criteria

Implementation is complete when:

1. ✅ Container starts without CrowdSec errors
2. ✅ `/etc/crowdsec` symlink exists and points to persistent storage
3. ✅ Config files use correct paths (`/var/log/crowdsec/`, `/app/data/crowdsec/data/`)
4. ✅ CrowdSec can be started via UI without permission errors
5. ✅ LAPI becomes ready within 30 seconds
6. ✅ `cscli` commands work correctly (hub update, preset install, etc.)
7. ✅ Process survives container restarts when enabled

---

*Research completed: December 22, 2024*
*Ready for implementation*
