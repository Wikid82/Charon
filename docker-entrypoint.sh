#!/bin/bash
set -e

# Entrypoint script to run both Caddy and CPM+ in a single container
# This simplifies deployment for home users

echo "Starting CaddyProxyManager+ with integrated Caddy..."

# Start Caddy in the background with initial empty config
echo '{"apps":{}}' > /config/caddy.json
caddy run --config /config/caddy.json --adapter json &
CADDY_PID=$!
echo "Caddy started (PID: $CADDY_PID)"

# Wait for Caddy to be ready
echo "Waiting for Caddy admin API..."
for i in {1..30}; do
    if wget -q -O- http://localhost:2019/config/ > /dev/null 2>&1; then
        echo "Caddy is ready!"
        break
    fi
    sleep 1
done

# Start CPM+ management application
echo "Starting CPM+ management application..."
/app/api &
APP_PID=$!
echo "CPM+ started (PID: $APP_PID)"

# Function to handle shutdown gracefully
shutdown() {
    echo "Shutting down..."
    kill -TERM $APP_PID 2>/dev/null || true
    kill -TERM $CADDY_PID 2>/dev/null || true
    wait $APP_PID 2>/dev/null || true
    wait $CADDY_PID 2>/dev/null || true
    exit 0
}

# Trap signals for graceful shutdown
trap shutdown SIGTERM SIGINT

echo "CaddyProxyManager+ is running!"
echo "  - Management UI: http://localhost:8080"
echo "  - Caddy Proxy: http://localhost:80, https://localhost:443"
echo "  - Caddy Admin API: http://localhost:2019"

# Wait for either process to exit
wait -n $APP_PID $CADDY_PID

# If one process exits, shut down the other
EXIT_CODE=$?
echo "A process exited with code $EXIT_CODE, shutting down..."
shutdown
