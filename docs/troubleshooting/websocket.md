---
title: Troubleshooting WebSocket Issues
description: Resolve WebSocket connection problems in Charon. Proxy configuration, timeouts, and connection stability.
---

## Troubleshooting WebSocket Issues

WebSocket connections are used in Charon for real-time features like live log streaming. If you're experiencing issues with WebSocket connections (e.g., logs not updating in real-time), this guide will help you diagnose and resolve the problem.

### Quick Diagnostics

### Check WebSocket Connection Status

1. Go to **System Settings** in the Charon UI
2. Scroll to the **WebSocket Connections** card
3. Check if there are active connections displayed

The WebSocket status card shows:

- Total number of active WebSocket connections
- Breakdown by type (General Logs vs Security Logs)
- Oldest connection age
- Detailed connection info (when expanded)

### Browser Console Check

Open your browser's Developer Tools (F12) and check the Console tab for:

- WebSocket connection errors
- Connection refused messages
- Authentication failures
- CORS errors

## Common Issues and Solutions

### 1. Proxy/Load Balancer Configuration

**Symptom:** WebSocket connections fail to establish or disconnect immediately.

**Cause:** If running Charon behind a reverse proxy (Nginx, Apache, HAProxy, or load balancer), the proxy might be terminating WebSocket connections or not forwarding the upgrade request properly.

**Solution:**

#### Nginx Configuration

```nginx
location /api/v1/logs/live {
    proxy_pass http://charon:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # Increase timeouts for long-lived connections
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

location /api/v1/cerberus/logs/ws {
    proxy_pass http://charon:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # Increase timeouts for long-lived connections
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

Key requirements:

- `proxy_http_version 1.1` — Required for WebSocket support
- `Upgrade` and `Connection` headers — Required for WebSocket upgrade
- Long `proxy_read_timeout` — Prevents connection from timing out

#### Apache Configuration

```apache
<VirtualHost *:443>
    ServerName charon.example.com

    # Enable WebSocket proxy
    ProxyRequests Off
    ProxyPreserveHost On

    # WebSocket endpoints
    ProxyPass /api/v1/logs/live ws://localhost:8080/api/v1/logs/live retry=0 timeout=3600
    ProxyPassReverse /api/v1/logs/live ws://localhost:8080/api/v1/logs/live

    ProxyPass /api/v1/cerberus/logs/ws ws://localhost:8080/api/v1/cerberus/logs/ws retry=0 timeout=3600
    ProxyPassReverse /api/v1/cerberus/logs/ws ws://localhost:8080/api/v1/cerberus/logs/ws

    # Regular HTTP endpoints
    ProxyPass / http://localhost:8080/
    ProxyPassReverse / http://localhost:8080/
</VirtualHost>
```

Required modules:

```bash
a2enmod proxy proxy_http proxy_wstunnel
```

### 2. Network Timeouts

**Symptom:** WebSocket connections work initially but disconnect after some idle time.

**Cause:** Intermediate network infrastructure (firewalls, load balancers, NAT devices) may have idle timeout settings shorter than the WebSocket keepalive interval.

**Solution:**

Charon sends WebSocket ping frames every 30 seconds to keep connections alive. If you're still experiencing timeouts:

1. **Check proxy timeout settings** (see above)
2. **Check firewall idle timeout:**

   ```bash
   # Linux iptables
   iptables -L -v -n | grep ESTABLISHED

   # If timeout is too short, increase it:
   iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
   echo 3600 > /proc/sys/net/netfilter/nf_conntrack_tcp_timeout_established
   ```

3. **Check load balancer settings:**
   - AWS ALB/ELB: Set idle timeout to 3600 seconds
   - GCP Load Balancer: Set timeout to 1 hour
   - Azure Load Balancer: Set idle timeout to maximum

### 3. HTTPS Certificate Errors (Docker)

**Symptom:** WebSocket connections fail with TLS/certificate errors, especially in Docker environments.

**Cause:** Missing CA certificates in the Docker container, or self-signed certificates not trusted by the browser.

**Solution:**

#### Install CA Certificates (Docker)

Add to your Dockerfile:

```dockerfile
RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates
```

Or for existing containers:

```bash
docker exec -it charon apt-get update && apt-get install -y ca-certificates
```

#### For Self-Signed Certificates (Development Only)

**Warning:** This compromises security. Only use in development environments.

Set environment variable:

```bash
docker run -e FF_IGNORE_CERT_ERRORS=1 charon:latest
```

Or in docker-compose.yml:

```yaml
services:
  charon:
    environment:
      - FF_IGNORE_CERT_ERRORS=1
```

#### Better Solution: Use Valid Certificates

1. Use Let's Encrypt (free, automated)
2. Use a trusted CA certificate
3. Import your self-signed cert into the browser's trust store

### 4. Firewall Settings

**Symptom:** WebSocket connections fail or time out.

**Cause:** Firewall blocking WebSocket traffic on ports 80/443.

**Solution:**

#### Linux (iptables)

Allow WebSocket traffic:

```bash
# Allow HTTP/HTTPS
iptables -A INPUT -p tcp --dport 80 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -j ACCEPT

# Allow established connections (for WebSocket)
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Save rules
iptables-save > /etc/iptables/rules.v4
```

#### Docker

Ensure ports are exposed:

```yaml
services:
  charon:
    ports:
      - "8080:8080"
      - "443:443"
```

#### Cloud Providers

- **AWS:** Add inbound rules to Security Group for ports 80/443
- **GCP:** Add firewall rules for ports 80/443
- **Azure:** Add Network Security Group rules for ports 80/443

### 5. Connection Stability / Packet Loss

**Symptom:** Frequent WebSocket disconnections and reconnections.

**Cause:** Unstable network with packet loss prevents WebSocket connections from staying open.

**Solution:**

#### Check Network Stability

```bash
# Ping test
ping -c 100 charon.example.com

# Check packet loss (should be < 1%)
mtr charon.example.com
```

#### Enable Connection Retry (Client-Side)

The Charon frontend automatically handles reconnection for security logs but not general logs. If you need more robust reconnection:

1. Monitor the WebSocket status in System Settings
2. Refresh the page if connections are frequently dropping
3. Consider using a more stable network connection
4. Check if VPN or proxy is causing issues

### 6. Browser Compatibility

**Symptom:** WebSocket connections don't work in certain browsers.

**Cause:** Very old browsers don't support WebSocket protocol.

**Supported Browsers:**

- Chrome 16+ ✅
- Firefox 11+ ✅
- Safari 7+ ✅
- Edge (all versions) ✅
- IE 10+ ⚠️ (deprecated, use Edge)

**Solution:** Update to a modern browser.

### 7. CORS Issues

**Symptom:** Browser console shows CORS errors with WebSocket connections.

**Cause:** Cross-origin WebSocket connection blocked by browser security policy.

**Solution:**

WebSocket connections should be same-origin (from the same domain as the Charon UI). If you're accessing Charon from a different domain:

1. **Preferred:** Access Charon UI from the same domain
2. **Alternative:** Configure CORS in Charon (if supported)
3. **Development Only:** Use browser extension to disable CORS (NOT for production)

### 8. Authentication Issues

**Symptom:** WebSocket connection fails with 401 Unauthorized.

**Cause:** Authentication token not being sent with WebSocket connection.

**Solution:**

Charon WebSocket endpoints support three authentication methods:

1. **HttpOnly Cookie** (automatic) — Used by default when accessing UI from browser
2. **Query Parameter** — `?token=<your-token>`
3. **Authorization Header** — Not supported for browser WebSocket connections

If you're accessing WebSocket from a script or tool:

```javascript
const ws = new WebSocket('wss://charon.example.com/api/v1/logs/live?token=YOUR_TOKEN');
```

## Monitoring WebSocket Connections

### Using the System Settings UI

1. Navigate to **System Settings** in Charon
2. View the **WebSocket Connections** card
3. Expand details to see:
   - Connection ID
   - Connection type (General/Security)
   - Remote address
   - Active filters
   - Connection duration

### Using the API

Check WebSocket statistics programmatically:

```bash
# Get connection statistics
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://charon.example.com/api/v1/websocket/stats

# Get detailed connection list
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://charon.example.com/api/v1/websocket/connections
```

Response example:

```json
{
  "total_active": 2,
  "logs_connections": 1,
  "cerberus_connections": 1,
  "oldest_connection": "2024-01-15T10:30:00Z",
  "last_updated": "2024-01-15T11:00:00Z"
}
```

### Using Browser DevTools

1. Open DevTools (F12)
2. Go to **Network** tab
3. Filter by **WS** (WebSocket)
4. Look for connections to:
   - `/api/v1/logs/live`
   - `/api/v1/cerberus/logs/ws`

Check:

- Status should be `101 Switching Protocols`
- Messages tab shows incoming log entries
- No errors in Frames tab

## Still Having Issues?

If none of the above solutions work:

1. **Check Charon logs:**

   ```bash
   docker logs charon | grep -i websocket
   ```

2. **Enable debug logging** (if available)

3. **Report an issue on GitHub:**
   - [Charon Issues](https://github.com/Wikid82/charon/issues)
   - Include:
     - Charon version
     - Browser and version
     - Proxy/load balancer configuration
     - Error messages from browser console
     - Charon server logs

## See Also

- [Live Logs Guide](../live-logs-guide.md)
- [Security Documentation](../security.md)
- [API Documentation](../api.md)
