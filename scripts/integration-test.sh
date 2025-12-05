#!/bin/bash
set -e

# Configuration
API_URL="http://localhost:8080/api/v1"
ADMIN_EMAIL="admin@example.com"
ADMIN_PASSWORD="changeme"

echo "Waiting for Charon to be ready..."
for i in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w "%{http_code}" $API_URL/health || echo "000")
  if [ "$code" = "200" ]; then
    echo "✅ Charon is ready!"
    break
  fi
  echo "Attempt $i/30: health not ready (code=$code); waiting..."
  sleep 2
done

if [ "$code" != "200" ]; then
  echo "❌ Charon failed to start"
  exit 1
fi

echo "Checking setup status..."
SETUP_RESPONSE=$(curl -s $API_URL/setup)
echo "Setup response: $SETUP_RESPONSE"

# Validate response is JSON before parsing
if ! echo "$SETUP_RESPONSE" | jq -e . >/dev/null 2>&1; then
  echo "❌ Setup endpoint did not return valid JSON"
  echo "Raw response: $SETUP_RESPONSE"
  exit 1
fi

SETUP_REQUIRED=$(echo "$SETUP_RESPONSE" | jq -r .setupRequired)
if [ "$SETUP_REQUIRED" = "true" ]; then
  echo "Setup is required; attempting to create initial admin..."
  SETUP_RESPONSE=$(curl -s -X POST $API_URL/setup \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Administrator\",\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")
  echo "Setup response: $SETUP_RESPONSE"
  if echo "$SETUP_RESPONSE" | jq -e .user >/dev/null 2>&1; then
    echo "✅ Setup completed"
  else
    echo "⚠️ Setup request returned unexpected response; continuing to login attempt"
  fi
fi

echo "Logging in..."
TOKEN=$(curl -s -X POST $API_URL/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .token)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Login failed"
  exit 1
fi
echo "✅ Login successful"

echo "Creating Proxy Host..."
# Remove existing proxy host for the domain to make the test idempotent
EXISTING_ID=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/proxy-hosts | jq -r --arg domain "test.localhost" '.[] | select(.domain_names == $domain) | .uuid' | head -n1)
if [ -n "$EXISTING_ID" ]; then
  echo "Found existing proxy host (ID: $EXISTING_ID), deleting..."
  curl -s -X DELETE $API_URL/proxy-hosts/$EXISTING_ID -H "Authorization: Bearer $TOKEN"
  # Wait until the host is removed and Caddy has reloaded
  for i in $(seq 1 10); do
    sleep 1
    STILL_EXISTS=$(curl -s -H "Authorization: Bearer $TOKEN" $API_URL/proxy-hosts | jq -r --arg domain "test.localhost" '.[] | select(.domain_names == $domain) | .uuid' | head -n1)
    if [ -z "$STILL_EXISTS" ]; then
      break
    fi
    echo "Waiting for API to delete existing proxy host..."
  done
fi
# Start a lightweight test upstream server to ensure proxy has a target (local-only). If a
# whoami container is already running on the Docker network, prefer using that.
USE_HOST_WHOAMI=false
if command -v docker >/dev/null 2>&1; then
  if docker ps --format '{{.Names}}' | grep -q '^whoami$'; then
    USE_HOST_WHOAMI=true
  fi
fi
if [ "$USE_HOST_WHOAMI" = "false" ]; then
  python3 -c "import http.server, socketserver
class Handler(http.server.BaseHTTPRequestHandler):
  def do_GET(self):
    self.send_response(200)
    self.end_headers()
    self.wfile.write(b'Hostname: local-test')
  def log_message(self, format, *args):
    pass
httpd=socketserver.TCPServer(('0.0.0.0', 8081), Handler)
import threading
threading.Thread(target=httpd.serve_forever, daemon=True).start()
" &
else
  echo "Using existing whoami container for upstream tests"
fi

# Prefer "whoami" when running inside CI/docker (it resolves on the docker network).
# For local runs, default to 127.0.0.1 since we start the test upstream on the host —
# but if charon runs inside Docker and the upstream is bound to the host, we must
# use host.docker.internal so Caddy inside the container can reach the host service.
FORWARD_HOST="127.0.0.1"
FORWARD_PORT="8081"
if [ "$USE_HOST_WHOAMI" = "true" ]; then
  FORWARD_HOST="whoami"
  FORWARD_PORT="80"
fi
if [ -n "$CI" ] || [ -n "$GITHUB_ACTIONS" ]; then
  FORWARD_HOST="whoami"
  # whoami image listens on port 80 inside its container
  FORWARD_PORT="80"
fi

# If we're running charon in Docker locally and we didn't choose whoami, prefer
# host.docker.internal so that the containerized Caddy can reach a host-bound upstream.
if command -v docker >/dev/null 2>&1; then
  if docker ps --format '{{.Names}}' | grep -q '^charon-debug$' || docker ps --format '{{.Image}}' | grep -q 'charon:local'; then
    if [ "$FORWARD_HOST" = "127.0.0.1" ]; then
      FORWARD_HOST="host.docker.internal"
    fi
  fi
fi
echo "Using forward host: $FORWARD_HOST:$FORWARD_PORT"

# Adjust the Caddy/Caddy proxy test port for local runs to avoid conflicts with
# host services on port 80.
CADDY_PORT="80"
if [ -z "$CI" ] && [ -z "$GITHUB_ACTIONS" ]; then
  # Use a non-privileged port locally when binding to host: 8082
  CADDY_PORT="8082"
fi
echo "Using Caddy host port: $CADDY_PORT"
# Retry creation up to 5 times if the apply config call fails due to Caddy reloads
RESPONSE=""
for attempt in 1 2 3 4 5; do
  RESPONSE=$(curl -s -X POST $API_URL/proxy-hosts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_names": "test.localhost",
    "forward_scheme": "http",
    "forward_host": "'"$FORWARD_HOST"'",
    "forward_port": '"$FORWARD_PORT"',
    "access_list_id": null,
    "certificate_id": null,
    "ssl_forced": false,
    "caching_enabled": false,
    "block_exploits": false,
    "allow_websocket_upgrade": true,
    "http2_support": true,
    "hsts_enabled": false,
    "hsts_subdomains": false,
    "locations": []
  }')
  # If Response contains a failure message indicating caddy apply failed, retry
  if echo "$RESPONSE" | grep -q "Failed to apply configuration"; then
    echo "Warning: failed to apply config on attempt $attempt, retrying..."
    # Wait for Caddy admin API on host to respond to /config to reduce collisions
    for i in $(seq 1 10); do
      if curl -s -o /dev/null -w "%{http_code}" http://localhost:${CADDY_ADMIN_PORT:-20194}/config/ >/dev/null 2>&1; then
        break
      fi
      sleep 1
    done
    sleep $attempt
    continue
  fi
  break
done

ID=$(echo $RESPONSE | jq -r .uuid)
if [ -z "$ID" ] || [ "$ID" = "null" ]; then
  echo "❌ Failed to create proxy host: $RESPONSE"
  exit 1
fi
echo "✅ Proxy Host created (ID: $ID)"

echo "Testing Proxy..."
# We use Host header to route to the correct proxy host
# We hit localhost:80 (Caddy) which should route to whoami
HTTP_CODE=0
CONTENT=""
# Retry probing Caddy for the new route for up to 10 seconds
for i in $(seq 1 10); do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: test.localhost" http://localhost:${CADDY_PORT} || true)
  CONTENT=$(curl -s -H "Host: test.localhost" http://localhost:${CADDY_PORT} || true)
  if [ "$HTTP_CODE" = "200" ] && echo "$CONTENT" | grep -q "Hostname:"; then
    break
  fi
  echo "Waiting for Caddy to pick up new route ($i/10)..."
  sleep 1
done

if [ "$HTTP_CODE" = "200" ] && echo "$CONTENT" | grep -q "Hostname:"; then
  echo "✅ Proxy test passed! Content received from whoami."
else
  echo "❌ Proxy test failed (Code: $HTTP_CODE)"
  echo "Content: $CONTENT"
  exit 1
fi
