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
        su-exec charon "$@"
    else
        "$@"
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
        if ! getent group "$DOCKER_SOCK_GID" >/dev/null 2>&1; then
            echo "Docker socket detected (gid=$DOCKER_SOCK_GID) - creating docker group and adding charon user..."
            # Create docker group with the socket's GID
            addgroup -g "$DOCKER_SOCK_GID" docker 2>/dev/null || true
            # Add charon user to the docker group
            addgroup charon docker 2>/dev/null || true
            echo "Docker integration enabled for charon user"
        else
            # Group exists, just add charon to it
            GROUP_NAME=$(getent group "$DOCKER_SOCK_GID" | cut -d: -f1)
            echo "Docker socket detected (gid=$DOCKER_SOCK_GID, group=$GROUP_NAME) - adding charon user..."
            addgroup charon "$GROUP_NAME" 2>/dev/null || true
            echo "Docker integration enabled for charon user"
        fi
    fi
elif [ -S "/var/run/docker.sock" ]; then
    echo "Note: Docker socket mounted but container is running non-root; skipping docker.sock group setup."
    echo "      If Docker discovery is needed, run with matching group permissions (e.g., --group-add)"
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
    # Log directories are created at build time with correct ownership
    # Only attempt to create if they don't exist (first run scenarios)
    mkdir -p /var/log/crowdsec 2>/dev/null || true
    mkdir -p /var/log/caddy 2>/dev/null || true

    # Initialize persistent config if key files are missing
    if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
        echo "Initializing persistent CrowdSec configuration..."
        if [ -d "/etc/crowdsec.dist" ] && [ -n "$(ls -A /etc/crowdsec.dist 2>/dev/null)" ]; then
            cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/" || {
                echo "ERROR: Failed to copy config from /etc/crowdsec.dist"
                exit 1
            }
            echo "Successfully initialized config from .dist directory"
        elif [ -d "/etc/crowdsec" ] && [ ! -L "/etc/crowdsec" ] && [ -n "$(ls -A /etc/crowdsec 2>/dev/null)" ]; then
            cp -r /etc/crowdsec/* "$CS_CONFIG_DIR/" || {
                echo "ERROR: Failed to copy config from /etc/crowdsec"
                exit 1
            }
            echo "Successfully initialized config from /etc/crowdsec"
        else
            echo "ERROR: No config source found (neither .dist nor /etc/crowdsec available)"
            exit 1
        fi
    fi

    # Verify symlink exists (created at build time)
    # Note: Symlink is created in Dockerfile as root before switching to non-root user
    # Non-root users cannot create symlinks in /etc, so this must be done at build time
    if [ -L "/etc/crowdsec" ]; then
        echo "CrowdSec config symlink verified: /etc/crowdsec -> $CS_CONFIG_DIR"
    else
        echo "WARNING: /etc/crowdsec symlink not found. This may indicate a build issue."
        echo "Expected: /etc/crowdsec -> /app/data/crowdsec/config"
        # Try to continue anyway - config may still work if CrowdSec uses CFG env var
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

    # Verify LAPI configuration was applied correctly
    if grep -q "listen_uri:.*:8085" "$CS_CONFIG_DIR/config.yaml"; then
        echo "✓ CrowdSec LAPI configured for port 8085"
    else
        echo "✗ WARNING: LAPI port configuration may be incorrect"
    fi

    # Update hub index to ensure CrowdSec can start
    if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
        echo "Updating CrowdSec hub index..."
        timeout 60s cscli hub update 2>/dev/null || echo "⚠️ Hub update timed out or failed, continuing..."
    fi

    # Ensure local machine is registered (auto-heal for volume/config mismatch)
    # We force registration because we just restored configuration (and likely credentials)
    echo "Registering local machine..."
    cscli machines add -a --force 2>/dev/null || echo "Warning: Machine registration may have failed"

    # Install hub items (parsers, scenarios, collections) if local mode enabled
    if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
        echo "Installing CrowdSec hub items..."
        if [ -x /usr/local/bin/install_hub_items.sh ]; then
            /usr/local/bin/install_hub_items.sh 2>/dev/null || echo "Warning: Some hub items may not have installed"
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
    if wget -q -O- http://127.0.0.1:2019/config/ > /dev/null 2>&1; then
        echo "Caddy is ready!"
        break
    fi
    i=$((i+1))
    sleep 1
done

# Start Charon management application
# Drop privileges to charon user before starting the application
# This maintains security while allowing Docker socket access via group membership
# Note: When running as root, we use su-exec; otherwise we run directly.
echo "Starting Charon management application..."
DEBUG_FLAG=${CHARON_DEBUG:-$CPMP_DEBUG}
DEBUG_PORT=${CHARON_DEBUG_PORT:-$CPMP_DEBUG_PORT}
if [ "$DEBUG_FLAG" = "1" ]; then
    echo "Running Charon under Delve (port $DEBUG_PORT)"
    bin_path=/app/charon
    if [ ! -f "$bin_path" ]; then
        bin_path=/app/cpmp
    fi
    run_as_charon /usr/local/bin/dlv exec "$bin_path" --headless --listen=":$DEBUG_PORT" --api-version=2 --accept-multiclient --continue --log -- &
else
    bin_path=/app/charon
    if [ ! -f "$bin_path" ]; then
        bin_path=/app/cpmp
    fi
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
