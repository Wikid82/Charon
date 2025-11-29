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
SETUP_REQUIRED=$(curl -s $API_URL/setup | jq -r .setupRequired)
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
fi
# Start a lightweight test upstream server to ensure proxy has a target (local-only)
python3 -c "import http.server, socketserver
class Handler(http.server.BaseHTTPRequestHandler):
  def do_GET(self):
    self.send_response(200)
    self.end_headers()
    self.wfile.write(b'Hostname: local-test')
  def log_message(self, format, *args):
    pass
httpd=socketserver.TCPServer((\"0.0.0.0\", 8081), Handler)
import threading
threading.Thread(target=httpd.serve_forever, daemon=True).start()
" &

# We use 'whoami' as the forward host because they are on the same docker network
RESPONSE=$(curl -s -X POST $API_URL/proxy-hosts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_names": "test.localhost",
    "forward_scheme": "http",
    "forward_host": "127.0.0.1",
    "forward_port": 8081,
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

ID=$(echo $RESPONSE | jq -r .uuid)
if [ -z "$ID" ] || [ "$ID" = "null" ]; then
  echo "❌ Failed to create proxy host: $RESPONSE"
  exit 1
fi
echo "✅ Proxy Host created (ID: $ID)"

echo "Testing Proxy..."
# We use Host header to route to the correct proxy host
# We hit localhost:80 (Caddy) which should route to whoami
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: test.localhost" http://localhost:80)
CONTENT=$(curl -s -H "Host: test.localhost" http://localhost:80)

if [ "$HTTP_CODE" = "200" ] && echo "$CONTENT" | grep -q "Hostname:"; then
  echo "✅ Proxy test passed! Content received from whoami."
else
  echo "❌ Proxy test failed (Code: $HTTP_CODE)"
  echo "Content: $CONTENT"
  exit 1
fi
