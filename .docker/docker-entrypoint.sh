#!/bin/sh
set -e

# Entrypoint script to run both Caddy and Charon in a single container
# This simplifies deployment for home users

echo "Starting Charon with integrated Caddy..."

is_root() {
    [ "$(id -u)" -eq 0 ]
}

run_as_charon() {
    if is_root; then
        gosu charon "$@"
    else
        "$@"
    fi
}

get_group_by_gid() {
    if command -v getent >/dev/null 2>&1; then
        getent group "$1" 2>/dev/null || true
    else
        awk -F: -v gid="$1" '$3==gid {print $0}' /etc/group 2>/dev/null || true
    fi
}

create_group_with_gid() {
    if command -v addgroup >/dev/null 2>&1; then
        addgroup -g "$1" "$2" 2>/dev/null || true
        return
    fi

    if command -v groupadd >/dev/null 2>&1; then
        groupadd -g "$1" "$2" 2>/dev/null || true
    fi
}

add_user_to_group() {
    if command -v addgroup >/dev/null 2>&1; then
        addgroup "$1" "$2" 2>/dev/null || true
        return
    fi

    if command -v usermod >/dev/null 2>&1; then
        usermod -aG "$2" "$1" 2>/dev/null || true
    fi
}

# ============================================================================
# Volume Permission Handling for Non-Root User
# ============================================================================
# When running as non-root user (charon), mounted volumes may have incorrect
# permissions. This section ensures the application can write to required paths.
# Note: This runs as the charon user, so we can only fix owned directories.

# Ensure /app/data exists and is writable (primary data volume)
if [ ! -w "/app/data" ] 2>/dev/null; then
    echo "Warning: /app/data is not writable. Please ensure volume permissions are correct."
    echo "  Run: docker run ... -v charon_data:/app/data ..."
    echo "  Or fix permissions: chown -R 1000:1000 /path/to/volume"
fi

# Ensure /config exists and is writable (Caddy config volume)
if [ ! -w "/config" ] 2>/dev/null; then
    echo "Warning: /config is not writable. Please ensure volume permissions are correct."
fi

# Create required subdirectories in writable volumes
mkdir -p /app/data/caddy 2>/dev/null || true
mkdir -p /app/data/crowdsec 2>/dev/null || true
mkdir -p /app/data/geoip 2>/dev/null || true

# Fix ownership for the data volume and required subdirectories when running as root.
# This handles rootless Docker environments where the host volume may be owned by the
# host user (mapped to container UID 0), making it inaccessible to the charon user.
if is_root; then
    chown charon:charon /app/data 2>/dev/null || true
    chown charon:charon /config 2>/dev/null || true
    chown -R charon:charon /app/data/caddy 2>/dev/null || true
    chown -R charon:charon /app/data/crowdsec 2>/dev/null || true
    chown -R charon:charon /app/data/geoip 2>/dev/null || true
fi

# ============================================================================
# Plugin Directory Permission Verification
# ============================================================================
# The PluginLoaderService requires the plugin directory to NOT be world-writable
# (mode 0002 bit must not be set). This is a security requirement to prevent
# malicious plugin injection.
PLUGINS_DIR="${CHARON_PLUGINS_DIR:-/app/plugins}"
if [ -d "$PLUGINS_DIR" ]; then
    # Check if directory is world-writable (security risk)
    # Using find -perm -0002 is more robust than stat regex - handles sticky/setgid bits correctly
    if find "$PLUGINS_DIR" -maxdepth 0 -perm -0002 -print -quit 2>/dev/null | grep -q .; then
        echo "⚠️  WARNING: Plugin directory $PLUGINS_DIR is world-writable!"
        echo "   This is a security risk - plugins could be injected by any user."
        echo "   Attempting to fix permissions (removing world-writable bit)..."
        # Use chmod o-w to only remove world-writable, preserving sticky/setgid bits
        if chmod o-w "$PLUGINS_DIR" 2>/dev/null; then
            echo "   ✓ Fixed: Plugin directory world-writable permission removed"
        else
            echo "   ✗ ERROR: Cannot fix permissions. Please run: chmod o-w $PLUGINS_DIR"
            echo "   Plugin loading may fail due to insecure permissions."
        fi
    else
        echo "✓ Plugin directory permissions OK: $PLUGINS_DIR"
    fi
else
    echo "Note: Plugin directory $PLUGINS_DIR does not exist (plugins disabled)"
fi

# ============================================================================
# Docker Socket Permission Handling
# ============================================================================
# The Docker integration feature requires access to the Docker socket.
# If the container runs as root, we can auto-align group membership with the
# socket GID. If running non-root (default), we cannot modify groups; users
# can enable Docker integration by using a compatible GID / --group-add.

if [ -S "/var/run/docker.sock" ] && is_root; then
    DOCKER_SOCK_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "")
    if [ -n "$DOCKER_SOCK_GID" ] && [ "$DOCKER_SOCK_GID" != "0" ]; then
        # Check if a group with this GID exists
        GROUP_ENTRY=$(get_group_by_gid "$DOCKER_SOCK_GID")
        if [ -z "$GROUP_ENTRY" ]; then
            echo "Docker socket detected (gid=$DOCKER_SOCK_GID) - creating docker group and adding charon user..."
            # Create docker group with the socket's GID
            create_group_with_gid "$DOCKER_SOCK_GID" docker
            # Add charon user to the docker group
            add_user_to_group charon docker
            echo "Docker integration enabled for charon user"
        else
            # Group exists, just add charon to it
            GROUP_NAME=$(echo "$GROUP_ENTRY" | cut -d: -f1)
            echo "Docker socket detected (gid=$DOCKER_SOCK_GID, group=$GROUP_NAME) - adding charon user..."
            add_user_to_group charon "$GROUP_NAME"
            echo "Docker integration enabled for charon user"
        fi
    fi
elif [ -S "/var/run/docker.sock" ]; then
    DOCKER_SOCK_GID=$(stat -c '%g' /var/run/docker.sock 2>/dev/null || echo "unknown")
    echo "Note: Docker socket mounted (GID=$DOCKER_SOCK_GID) but container is running non-root; skipping docker.sock group setup."
    echo "      If Docker discovery is needed, add 'group_add: [\"$DOCKER_SOCK_GID\"]' to your compose service."
    if [ "$DOCKER_SOCK_GID" = "0" ]; then
        if [ "${ALLOW_DOCKER_SOCK_GID_0:-false}" != "true" ]; then
            echo "⚠️  WARNING: Docker socket GID is 0 (root group). group_add: [\"0\"] grants root-group access."
            echo "   Set ALLOW_DOCKER_SOCK_GID_0=true to acknowledge this risk."
        fi
    fi
else
    echo "Note: Docker socket not found. Docker container discovery will be unavailable."
fi

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
    mkdir -p "$CS_PERSIST_DIR/hub_cache"

    # ============================================================================
    # CrowdSec Bouncer Key Persistence Directory
    # ============================================================================
    # Create the persistent directory for bouncer key storage.
    # This directory is inside /app/data which is volume-mounted.
    # The bouncer key will be stored at /app/data/crowdsec/bouncer_key
    echo "CrowdSec bouncer key will be stored at: $CS_PERSIST_DIR/bouncer_key"

    # Fix ownership for key directory if running as root
    if is_root; then
        chown charon:charon "$CS_PERSIST_DIR" 2>/dev/null || true
    fi

    # Log directories are created at build time with correct ownership
    # Only attempt to create if they don't exist (first run scenarios)
    mkdir -p /var/log/crowdsec 2>/dev/null || true
    mkdir -p /var/log/caddy 2>/dev/null || true

    # Initialize persistent config if key files are missing
    if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
        echo "Initializing persistent CrowdSec configuration..."

        # Check if .dist has content
        if [ -d "/etc/crowdsec.dist" ] && find /etc/crowdsec.dist -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null | grep -q .; then
            echo "Copying config from /etc/crowdsec.dist..."
            if ! cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/"; then
                echo "ERROR: Failed to copy config from /etc/crowdsec.dist"
                echo "DEBUG: Contents of /etc/crowdsec.dist:"
                ls -la /etc/crowdsec.dist/
                exit 1
            fi

            # Verify critical files were copied
            if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
                echo "ERROR: config.yaml was not copied to $CS_CONFIG_DIR"
                echo "DEBUG: Contents of $CS_CONFIG_DIR after copy:"
                ls -la "$CS_CONFIG_DIR/"
                exit 1
            fi
            echo "✓ Successfully initialized config from .dist directory"
        elif [ -d "/etc/crowdsec" ] && [ ! -L "/etc/crowdsec" ] && find /etc/crowdsec -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null | grep -q .; then
            echo "Copying config from /etc/crowdsec (fallback)..."
            if ! cp -r /etc/crowdsec/* "$CS_CONFIG_DIR/"; then
                echo "ERROR: Failed to copy config from /etc/crowdsec (fallback)"
                exit 1
            fi
            echo "✓ Successfully initialized config from /etc/crowdsec"
        else
            echo "ERROR: No config source found!"
            echo "DEBUG: /etc/crowdsec.dist contents:"
            ls -la /etc/crowdsec.dist/ 2>/dev/null || echo "  (directory not found or empty)"
            echo "DEBUG: /etc/crowdsec contents:"
            ls -la /etc/crowdsec 2>/dev/null || echo "  (directory not found or empty)"
            exit 1
        fi
    else
        echo "✓ Persistent config already exists: $CS_CONFIG_DIR/config.yaml"
    fi

    # Verify symlink exists (created at build time)
    # Note: Symlink is created in Dockerfile as root before switching to non-root user
    # Non-root users cannot create symlinks in /etc, so this must be done at build time
    if [ -L "/etc/crowdsec" ]; then
        echo "CrowdSec config symlink verified: /etc/crowdsec -> $CS_CONFIG_DIR"

        # Verify the symlink target is accessible and has config.yaml
        if [ ! -f "/etc/crowdsec/config.yaml" ]; then
            echo "ERROR: /etc/crowdsec/config.yaml is not accessible via symlink"
            echo "DEBUG: Symlink target verification:"
            ls -la /etc/crowdsec 2>/dev/null || echo "  (symlink broken or missing)"
            echo "DEBUG: Directory contents:"
            ls -la "$CS_CONFIG_DIR/" 2>/dev/null | head -10 || echo "  (directory not found)"
            exit 1
        fi
        echo "✓ /etc/crowdsec/config.yaml is accessible via symlink"
    else
        echo "ERROR: /etc/crowdsec symlink not found"
        echo "Expected: /etc/crowdsec -> /app/data/crowdsec/config"
        echo "This indicates a critical build-time issue. Symlink must be created at build time as root."
        echo "DEBUG: Directory check:"
        find /etc -mindepth 1 -maxdepth 1 -name '*crowdsec*' -exec ls -ld {} \; 2>/dev/null || echo "  (no crowdsec entry found)"
        exit 1
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

    # Perform variable substitution
    export CFG=/etc/crowdsec
    export DATA="$CS_DATA_DIR"
    export PID=/var/run/crowdsec.pid
    export LOG="$CS_LOG_DIR/crowdsec.log"

    # Process config.yaml and user.yaml with envsubst
    # We use a temp file to avoid issues with reading/writing same file
    for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
        if [ -f "$file" ]; then
            envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
            chown charon:charon "$file" 2>/dev/null || true
        fi
    done

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

    # Fix log directory path (ensure it points to /var/log/crowdsec/ not /var/log/)
    sed -i 's|log_dir: /var/log/$|log_dir: /var/log/crowdsec/|g' "$CS_CONFIG_DIR/config.yaml"
    # Also handle case where it might be without trailing slash
    sed -i 's|log_dir: /var/log$|log_dir: /var/log/crowdsec|g' "$CS_CONFIG_DIR/config.yaml"

    # Redirect CrowdSec LAPI database to persistent volume
    # Default path /var/lib/crowdsec/data/crowdsec.db is ephemeral (not volume-mounted),
    # so it is destroyed on every container rebuild. The bouncer API key (stored on the
    # persistent volume at /app/data/crowdsec/) survives rebuilds but the LAPI database
    # that validates it does not — causing perpetual key rejection.
    # Redirecting db_path to the volume-mounted CS_DATA_DIR fixes this.
    sed -i "s|db_path: /var/lib/crowdsec/data/crowdsec.db|db_path: ${CS_DATA_DIR}/crowdsec.db|g" "$CS_CONFIG_DIR/config.yaml"
    if grep -q "db_path:.*${CS_DATA_DIR}" "$CS_CONFIG_DIR/config.yaml"; then
        echo "✓ CrowdSec LAPI database redirected to persistent volume: ${CS_DATA_DIR}/crowdsec.db"
    else
        echo "⚠️  WARNING: Could not verify LAPI db_path redirect — bouncer keys may not survive rebuilds"
    fi

    # Verify LAPI configuration was applied correctly
    if grep -q "listen_uri:.*:8085" "$CS_CONFIG_DIR/config.yaml"; then
        echo "✓ CrowdSec LAPI configured for port 8085"
    else
        echo "✗ WARNING: LAPI port configuration may be incorrect"
    fi

    # Machine registration is fast (local DB write) and required for LAPI auth.
    # Always run regardless of environment.
    echo "Registering local machine..."
    cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"

    # Hub index update and hub item downloads are internet-bound operations (30–120 s).
    # Skip them when CHARON_SECURITY_TESTS_ENABLED=false (non-security CI shards) so the
    # container starts within the health-check window.
    # Production and security-test environments leave this variable unset or true, which
    # preserves the existing behaviour.
    if [ "${CHARON_SECURITY_TESTS_ENABLED}" = "false" ]; then
        echo "⚡ Skipping CrowdSec hub initialization (CHARON_SECURITY_TESTS_ENABLED=false)"
    else
        # Always refresh hub index on startup (stale index causes hash mismatch errors on collection install)
        echo "Updating CrowdSec hub index..."
        if ! timeout 60s cscli hub update 2>&1; then
            echo "⚠️ Hub index update failed (network issue?). Collections may fail to install."
            echo "   CrowdSec will still start with whatever index is cached."
        fi

        # Always ensure required collections are present (idempotent — already-installed items are skipped).
        # Collections are just config files with zero runtime cost when CrowdSec is disabled.
        echo "Ensuring CrowdSec hub items are installed..."
        if [ -x /usr/local/bin/install_hub_items.sh ]; then
            /usr/local/bin/install_hub_items.sh || echo "⚠️ Some hub items may not have installed. CrowdSec can still start."
        fi
    fi

    # Fix ownership AFTER cscli commands (they run as root and create root-owned files)
    echo "Fixing CrowdSec file ownership..."
    if is_root; then
        chown -R charon:charon /var/lib/crowdsec 2>/dev/null || true
        chown -R charon:charon /app/data/crowdsec 2>/dev/null || true
        chown -R charon:charon /var/log/crowdsec 2>/dev/null || true
    fi
fi

# CrowdSec Lifecycle Management:
# CrowdSec configuration is initialized above (symlinks, directories, hub updates)
# However, the CrowdSec agent is NOT auto-started in the entrypoint.
# Instead, CrowdSec lifecycle is managed by the backend handlers via GUI controls.
# This makes CrowdSec consistent with other security features (WAF, ACL, Rate Limiting).
# Users enable/disable CrowdSec using the Security dashboard toggle, which calls:
#   - POST /api/v1/admin/crowdsec/start (to start the agent)
#   - POST /api/v1/admin/crowdsec/stop (to stop the agent)
# This approach provides:
#   - Consistent user experience across all security features
#   - No environment variable dependency
#   - Real-time control without container restart
#   - Proper integration with Charon's security orchestration
echo "CrowdSec configuration initialized. Agent lifecycle is GUI-controlled."

# Start Caddy in the background with initial empty config
# Run Caddy as charon user for security
echo '{"admin":{"listen":"0.0.0.0:2019"},"apps":{}}' > /config/caddy.json
# Use JSON config directly; no adapter needed
run_as_charon caddy run --config /config/caddy.json &
CADDY_PID=$!
echo "Caddy started (PID: $CADDY_PID)"

# Wait for Caddy to be ready
echo "Waiting for Caddy admin API..."
i=1
while [ "$i" -le 30 ]; do
    if wget -qO /dev/null http://127.0.0.1:2019/config/ 2>/dev/null; then
        echo "Caddy is ready!"
        break
    fi
    i=$((i+1))
    sleep 1
done

# Start Charon management application
# Drop privileges to charon user before starting the application
# This maintains security while allowing Docker socket access via group membership
# Note: When running as root, we use gosu; otherwise we run directly.
echo "Starting Charon management application..."
DEBUG_FLAG=${CHARON_DEBUG:-$CPMP_DEBUG}
DEBUG_PORT=${CHARON_DEBUG_PORT:-${CPMP_DEBUG_PORT:-2345}}

# Determine binary path
bin_path=/app/charon
if [ ! -f "$bin_path" ]; then
    bin_path=/app/cpmp
fi

if [ "$DEBUG_FLAG" = "1" ]; then
    # Verify that /usr/local/bin/dlv is a real Delve binary, not the production stub
    # (production images ship a shell stub that exits 1 to satisfy the COPY instruction
    # without embedding the vulnerable golang.org/x/sys < v0.27.0 — GO-2026-5024).
    # Real Delve exits 0 on `dlv version`; the stub exits 1.
    if ! /usr/local/bin/dlv version >/dev/null 2>&1; then
        echo "Note: Delve not available in this image (production build, GO-2026-5024 mitigation)."
        echo "   Running Charon directly. To enable remote debugging, rebuild with:"
        echo "   docker build --build-arg BUILD_DEBUG=1 ..."
        run_as_charon "$bin_path" &
    # Check if binary has debug symbols (required for Delve)
    # objdump -h lists section headers; .debug_info is present if DWARF symbols exist
    elif command -v objdump >/dev/null 2>&1; then
        if ! objdump -h "$bin_path" 2>/dev/null | grep -q '\.debug_info'; then
            echo "⚠️  WARNING: Binary lacks debug symbols (DWARF info stripped)."
            echo "   Delve debugging will NOT work with this binary."
            echo "   To fix, rebuild with: docker build --build-arg BUILD_DEBUG=1 ..."
            echo "   Falling back to normal execution (without debugger)."
            run_as_charon "$bin_path" &
        else
            echo "✓ Debug symbols detected. Running Charon under Delve (port $DEBUG_PORT)"
            run_as_charon /usr/local/bin/dlv exec "$bin_path" --headless --listen=":$DEBUG_PORT" --api-version=2 --accept-multiclient --continue --log -- &
        fi
    else
        # objdump not available, try to run Delve anyway with a warning
        echo "Note: Cannot verify debug symbols (objdump not found). Attempting Delve..."
        run_as_charon /usr/local/bin/dlv exec "$bin_path" --headless --listen=":$DEBUG_PORT" --api-version=2 --accept-multiclient --continue --log -- &
    fi
else
    run_as_charon "$bin_path" &
fi
APP_PID=$!
echo "Charon started (PID: $APP_PID)"
shutdown() {
    echo "Shutting down..."
    kill -TERM "$APP_PID" 2>/dev/null || true
    kill -TERM "$CADDY_PID" 2>/dev/null || true
    # Note: CrowdSec process lifecycle is managed by backend handlers
    # The backend will handle graceful CrowdSec shutdown when the container stops
    wait "$APP_PID" 2>/dev/null || true
    wait "$CADDY_PID" 2>/dev/null || true
    exit 0
}

# Trap signals for graceful shutdown
trap 'shutdown' TERM INT

echo "Charon is running!"
echo "  - Management UI: http://localhost:8080"
echo "  - Caddy Proxy: http://localhost:80, https://localhost:443"
echo "  - Caddy Admin API: http://localhost:2019"

# Wait loop: exit when either process dies, then shutdown the other
while kill -0 "$APP_PID" 2>/dev/null && kill -0 "$CADDY_PID" 2>/dev/null; do
    sleep 1
done

echo "A process exited, initiating shutdown..."
shutdown
