#!/bin/sh
set -e

# Entrypoint script to run both Caddy and Charon in a single container
# This simplifies deployment for home users

echo "Starting Charon with integrated Caddy..."

# ============================================================================
# CrowdSec Initialization
# ============================================================================
CROWDSEC_PID=""
SECURITY_CROWDSEC_MODE=${CERBERUS_SECURITY_CROWDSEC_MODE:-${CHARON_SECURITY_CROWDSEC_MODE:-$CPM_SECURITY_CROWDSEC_MODE}}

# Initialize CrowdSec configuration if cscli is present
if command -v cscli >/dev/null; then
    echo "Initializing CrowdSec configuration..."

    # Define persistent paths
    CS_PERSIST_DIR="/app/data/crowdsec"
    CS_CONFIG_DIR="$CS_PERSIST_DIR/config"
    CS_DATA_DIR="$CS_PERSIST_DIR/data"

    # Ensure persistent directories exist
    mkdir -p "$CS_CONFIG_DIR"
    mkdir -p "$CS_DATA_DIR"
    mkdir -p /var/log/crowdsec
    mkdir -p /var/log/caddy

    # Initialize persistent config if key files are missing
    if [ ! -f "$CS_CONFIG_DIR/config.yaml" ]; then
        echo "Initializing persistent CrowdSec configuration..."
        if [ -d "/etc/crowdsec.dist" ]; then
            cp -r /etc/crowdsec.dist/* "$CS_CONFIG_DIR/"
        elif [ -d "/etc/crowdsec" ]; then
            # Fallback if .dist is missing
            cp -r /etc/crowdsec/* "$CS_CONFIG_DIR/"
        fi
    fi

    # Link /etc/crowdsec to persistent config for runtime compatibility
    if [ ! -L "/etc/crowdsec" ]; then
        echo "Relinking /etc/crowdsec to persistent storage..."
        rm -rf /etc/crowdsec
        ln -s "$CS_CONFIG_DIR" /etc/crowdsec
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
    export LOG=/var/log/crowdsec.log

    # Process config.yaml and user.yaml with envsubst
    # We use a temp file to avoid issues with reading/writing same file
    for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
        if [ -f "$file" ]; then
            envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
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

    # Update hub index to ensure CrowdSec can start
    if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
        echo "Updating CrowdSec hub index..."
        cscli hub update 2>/dev/null || echo "Warning: Failed to update hub index (network issue?)"
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
fi

# Start CrowdSec agent if local mode is enabled
if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    echo "CrowdSec Local Mode enabled."

    if command -v crowdsec >/dev/null; then
        # Create an empty access log so CrowdSec doesn't fail on missing file
        touch /var/log/caddy/access.log

        echo "Starting CrowdSec agent..."
        crowdsec -c /etc/crowdsec/config.yaml &
        CROWDSEC_PID=$!
        echo "CrowdSec started (PID: $CROWDSEC_PID)"

        # Wait for LAPI to be ready
        echo "Waiting for CrowdSec LAPI..."
        lapi_ready=0
        for i in $(seq 1 30); do
            if wget -q -O- http://127.0.0.1:8085/health >/dev/null 2>&1; then
                echo "CrowdSec LAPI is ready!"
                lapi_ready=1
                break
            fi
            sleep 1
        done

        if [ "$lapi_ready" = "1" ]; then
            # Register bouncer for Caddy
            if [ -x /usr/local/bin/register_bouncer.sh ]; then
                echo "Registering Caddy bouncer..."
                BOUNCER_API_KEY=$(/usr/local/bin/register_bouncer.sh 2>/dev/null | tail -1)
                if [ -n "$BOUNCER_API_KEY" ]; then
                    export CROWDSEC_BOUNCER_API_KEY="$BOUNCER_API_KEY"
                    echo "Bouncer registered with API key"
                fi
            fi
        else
            echo "Warning: CrowdSec LAPI not ready after 30 seconds"
        fi
    else
        echo "CrowdSec binary not found - skipping agent startup"
    fi
fi

# Start Caddy in the background with initial empty config
echo '{"admin":{"listen":"0.0.0.0:2019"},"apps":{}}' > /config/caddy.json
# Use JSON config directly; no adapter needed
caddy run --config /config/caddy.json &
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
echo "Starting Charon management application..."
DEBUG_FLAG=${CHARON_DEBUG:-$CPMP_DEBUG}
DEBUG_PORT=${CHARON_DEBUG_PORT:-$CPMP_DEBUG_PORT}
if [ "$DEBUG_FLAG" = "1" ]; then
    echo "Running Charon under Delve (port $DEBUG_PORT)"
    bin_path=/app/charon
    if [ ! -f "$bin_path" ]; then
        bin_path=/app/cpmp
    fi
    /usr/local/bin/dlv exec "$bin_path" --headless --listen=":$DEBUG_PORT" --api-version=2 --accept-multiclient --continue --log -- &
else
    bin_path=/app/charon
    if [ ! -f "$bin_path" ]; then
        bin_path=/app/cpmp
    fi
    "$bin_path" &
fi
APP_PID=$!
echo "Charon started (PID: $APP_PID)"
shutdown() {
    echo "Shutting down..."
    kill -TERM "$APP_PID" 2>/dev/null || true
    kill -TERM "$CADDY_PID" 2>/dev/null || true
    if [ -n "$CROWDSEC_PID" ]; then
        echo "Stopping CrowdSec..."
        kill -TERM "$CROWDSEC_PID" 2>/dev/null || true
        wait "$CROWDSEC_PID" 2>/dev/null || true
    fi
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
