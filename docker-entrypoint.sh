#!/bin/sh
set -e

# Entrypoint script to run both Caddy and Charon in a single container
# This simplifies deployment for home users

echo "Starting Charon with integrated Caddy..."

# Optional: Install and start CrowdSec (Local Mode)
CROWDSEC_PID=""
SECURITY_CROWDSEC_MODE=${CERBERUS_SECURITY_CROWDSEC_MODE:-${CHARON_SECURITY_CROWDSEC_MODE:-$CPM_SECURITY_CROWDSEC_MODE}}

# Always initialize CrowdSec configuration if missing and cscli is present
# This ensures cscli commands work even if the agent isn't running in background
if command -v cscli >/dev/null && [ ! -f "/etc/crowdsec/config.yaml" ]; then
    echo "Initializing CrowdSec configuration..."
    mkdir -p /etc/crowdsec
    if [ -d "/etc/crowdsec.dist" ]; then
        cp -r /etc/crowdsec.dist/* /etc/crowdsec/
    fi

    # Ensure data directories exist
    mkdir -p /var/lib/crowdsec/data
    mkdir -p /etc/crowdsec/hub

    # Perform variable substitution if needed (standard CrowdSec config uses $CFG, $DATA, etc.)
    # We set standard paths for Alpine/Docker
    export CFG=/etc/crowdsec
    export DATA=/var/lib/crowdsec/data
    export PID=/var/run/crowdsec.pid
    export LOG=/var/log/crowdsec.log

    # Process config.yaml and user.yaml with envsubst
    # We use a temp file to avoid issues with reading/writing same file
    for file in /etc/crowdsec/config.yaml /etc/crowdsec/user.yaml; do
        if [ -f "$file" ]; then
            envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
        fi
    done

    # Update hub index to ensure CrowdSec can start
    if [ ! -f "/etc/crowdsec/hub/.index.json" ]; then
        echo "Updating CrowdSec hub index..."
        cscli hub update || echo "Failed to update hub index (network issue?)"
    fi
    # Ensure local machine is registered (auto-heal for volume/config mismatch)
    # We force registration because we just restored configuration (and likely credentials)
    echo "Registering local machine..."
    cscli machines add -a --force || echo "Failed to register local machine"
fi

if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    echo "CrowdSec Local Mode enabled."

    if command -v crowdsec >/dev/null; then
        echo "Starting CrowdSec agent..."
        crowdsec &
        CROWDSEC_PID=$!
        echo "CrowdSec started (PID: $CROWDSEC_PID)"
    else
        echo "CrowdSec binary not found."
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
