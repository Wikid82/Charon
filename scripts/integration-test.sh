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
# We use 'whoami' as the forward host because they are on the same docker network
RESPONSE=$(curl -s -X POST $API_URL/proxy-hosts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_names": ["test.localhost"],
    "forward_scheme": "http",
    "forward_host": "whoami",
    "forward_port": 80,
    "access_list_id": "",
    "certificate_id": "",
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
