#!/usr/bin/env bash
set -euo pipefail

# Debug script to check rate limit configuration

echo "=== Starting debug container ==="
docker rm -f charon-debug 2>/dev/null || true
docker run -d --name charon-debug \
    --network containers_default \
    -p 8180:80 -p 8280:8080 -p 2119:2019 \
    -e CHARON_ENV=development \
    charon:local

sleep 10

echo ""
echo "=== Registering user ==="
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"debug@test.local","password":"pass123","name":"Debug"}' \
    http://localhost:8280/api/v1/auth/register >/dev/null || true

echo "=== Logging in ==="
TOKEN=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"debug@test.local","password":"pass123"}' \
    -c /tmp/debug-cookie \
    http://localhost:8280/api/v1/auth/login | jq -r '.token // empty')

echo ""
echo "=== Current security status (before config) ==="
curl -s -b /tmp/debug-cookie http://localhost:8280/api/v1/security/status | jq .

echo ""
echo "=== Setting security config ==="
curl -s -X POST -H "Content-Type: application/json" \
    -d '{
        "name": "default",
        "enabled": true,
        "rate_limit_enable": true,
        "rate_limit_requests": 3,
        "rate_limit_window_sec": 10,
        "rate_limit_burst": 1,
        "admin_whitelist": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
    }' \
    -b /tmp/debug-cookie \
    http://localhost:8280/api/v1/security/config | jq .

echo ""
echo "=== Waiting for config to apply ==="
sleep 5

echo ""
echo "=== Security status (after config) ==="
curl -s -b /tmp/debug-cookie http://localhost:8280/api/v1/security/status | jq .

echo ""
echo "=== Security config from DB ==="
curl -s -b /tmp/debug-cookie http://localhost:8280/api/v1/security/config | jq .

echo ""
echo "=== Caddy config (checking for rate_limit handler) ==="
curl -s http://localhost:2119/config/ | jq '.apps.http.servers.charon_server.routes[0].handle // []' | grep -i rate_limit || echo "No rate_limit handler found"

echo ""
echo "=== Full Caddy route handlers ==="
curl -s http://localhost:2119/config/ | jq '.apps.http.servers.charon_server.routes[0].handle // []'

echo ""
echo "=== Container logs (last 50 lines) ==="
docker logs charon-debug 2>&1 | tail -50

echo ""
echo "=== Cleanup ==="
docker rm -f charon-debug
rm -f /tmp/debug-cookie
