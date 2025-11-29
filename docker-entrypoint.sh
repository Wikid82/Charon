#!/bin/sh
set -e

# Entrypoint script to run both Caddy and CPM+ in a single container
# This simplifies deployment for home users

echo "Starting Charon with integrated Caddy..."

# Optional: Install and start CrowdSec (Local Mode)
CROWDSEC_PID=""
SECURITY_CROWDSEC_MODE=${CERBERUS_SECURITY_CROWDSEC_MODE:-${CHARON_SECURITY_CROWDSEC_MODE:-$CPM_SECURITY_CROWDSEC_MODE}}
if [ "$SECURITY_CROWDSEC_MODE" = "local" ]; then
    echo "CrowdSec Local Mode enabled. Installing CrowdSec agent..."
    # Install crowdsec from community repository if needed
    apk add --no-cache crowdsec --repository=http://dl-cdn.alpinelinux.org/alpine/edge/community || \
    apk add --no-cache crowdsec || \
    echo "Failed to install crowdsec. Check repositories."

    if command -v crowdsec >/dev/null; then
        echo "Starting CrowdSec agent..."
        # Ensure configuration exists or is generated (basic check)
        if [ ! -d "/etc/crowdsec" ]; then
             echo "Warning: /etc/crowdsec not found. CrowdSec might fail to start."
        fi
        crowdsec &
        CROWDSEC_PID=$!
        echo "CrowdSec started (PID: $CROWDSEC_PID)"
    else
        echo "CrowdSec binary not found after installation attempt."
    fi
fi

# Start Caddy in the background with initial empty config
echo '{"apps":{}}' > /config/caddy.json
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

# Start CPM+ management application
echo "Starting Charon management application..."
DEBUG_FLAG=${CHARON_DEBUG:-$CPMP_DEBUG}
DEBUG_PORT=${CHARON_DEBUG_PORT:-$CPMP_DEBUG_PORT}
if [ "$DEBUG_FLAG" = "1" ]; then
    echo "Running Charon under Delve (port $DEBUG_PORT)"
    bin_path=/app/charon
    if [ ! -f "$bin_path" ]; then
        bin_path=/app/cpmp
    fi
    /usr/local/bin/dlv exec "$bin_path" --headless --listen=":"$DEBUG_PORT" --api-version=2 --accept-multiclient --continue --log -- &
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
