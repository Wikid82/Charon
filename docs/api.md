---
title: API Documentation
description: Complete REST API reference for Charon. Includes endpoints for proxy hosts, certificates, security, and more.
---

## API Documentation

Charon REST API documentation. All endpoints return JSON and use standard HTTP status codes.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

🚧 Authentication is not yet implemented. All endpoints are currently public.

Future authentication will use JWT tokens:

```http
Authorization: Bearer <token>
```

## Response Format

### Success Response

```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Example",
  "created_at": "2025-01-18T10:00:00Z"
}
```

### Error Response

```json
{
  "error": "Resource not found",
  "code": 404
}
```

## Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 204 | No Content (successful deletion) |
| 400 | Bad Request (validation error) |
| 404 | Not Found |
| 500 | Internal Server Error |

## Endpoints

### Metrics (Prometheus)

Expose internal counters for scraping.

```http
GET /metrics
```

No authentication required. Primary WAF metrics:

```text
charon_waf_requests_total
charon_waf_blocked_total
charon_waf_monitored_total
```

---

### Health Check

Check API health status.

```http
GET /health
```

**Response 200:**

```json
{
  "status": "ok"
}
```

---

### Security Suite (Cerberus)

#### Status

```http
GET /security/status
```

Returns enabled flag plus modes for each module.

#### Get Global Security Config

```http
GET /security/config
```

Response 200 (no config yet): `{ "config": null }`

#### Upsert Global Security Config

```http
POST /security/config
Content-Type: application/json
```

Request Body (example):

```json
{
  "name": "default",
  "enabled": true,
  "admin_whitelist": "198.51.100.10,203.0.113.0/24",
  "crowdsec_mode": "local",
  "waf_mode": "monitor",
  "waf_rules_source": "owasp-crs-local"
}
```

Response 200: `{ "config": { ... } }`

**Security Considerations**:

Webhook URLs configured in security settings are validated to prevent Server-Side Request Forgery (SSRF) attacks. The following destinations are blocked:

- Private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Cloud metadata endpoints (169.254.169.254)
- Loopback addresses (127.0.0.0/8)
- Link-local addresses

**Error Response**:

```json
{
  "error": "Invalid webhook URL: URL resolves to a private IP address (blocked for security)"
}
```

**Example Valid URL**:

```json
{
  "webhook_url": "https://webhook.example.com/receive"
}
```

#### Enable Cerberus

```http
POST /security/enable
```

Payload (optional break-glass token):

```json
{ "break_glass_token": "abcd1234" }
```

#### Disable Cerberus

```http
POST /security/disable
```

Payload (required if not localhost):

```json
{ "break_glass_token": "abcd1234" }
```

#### Generate Break-Glass Token

```http
POST /security/breakglass/generate
```

Response 200: `{ "token": "plaintext-token-once" }`

#### List Security Decisions

```http
GET /security/decisions?limit=50
```

Response 200: `{ "decisions": [ ... ] }`

#### Create Manual Decision

```http
POST /security/decisions
Content-Type: application/json
```

Payload:

```json
{ "ip": "203.0.113.5", "action": "block", "details": "manual temporary block" }
```

#### List Rulesets

```http
GET /security/rulesets
```

Response 200: `{ "rulesets": [ ... ] }`

#### Upsert Ruleset

```http
POST /security/rulesets
Content-Type: application/json
```

Payload:

```json
{
  "name": "owasp-crs-quick",
  "source_url": "https://example.com/owasp-crs.txt",
  "mode": "owasp",
  "content": "# raw rules"
}
```

Response 200: `{ "ruleset": { ... } }`

#### Delete Ruleset

```http
DELETE /security/rulesets/:id
```

Response 200: `{ "deleted": true }`

---

### Application URL Endpoints

#### Validate Application URL

Validates that a URL is properly formatted for use as the application's public URL.

```http
POST /settings/validate-url
Content-Type: application/json
Authorization: Bearer <admin-token>
```

**Request Body:**

```json
{
  "url": "https://charon.example.com"
}
```

**Required Fields:**

- `url` (string) - The URL to validate

**Response 200 (Valid URL):**

```json
{
  "valid": true,
  "normalized": "https://charon.example.com"
}
```

**Response 200 (Valid with Warning):**

```json
{
  "valid": true,
  "normalized": "http://charon.example.com",
  "warning": "Using http:// instead of https:// is not recommended for production environments"
}
```

**Response 400 (Invalid URL):**

```json
{
  "valid": false,
  "error": "URL must start with http:// or https:// and cannot include path components"
}
```

**Response 403:**

```json
{
  "error": "Admin access required"
}
```

**Validation Rules:**

- URL must start with `http://` or `https://`
- URL cannot include path components (e.g., `/admin`)
- Trailing slashes are automatically removed
- Port numbers are allowed (e.g., `:8080`)
- Warning is returned if using `http://` (insecure)

**Examples:**

```bash
# Valid HTTPS URL
curl -X POST http://localhost:8080/api/v1/settings/validate-url \
  -H "Content-Type: application/json" \
  -d '{"url": "https://charon.example.com"}'

# Valid with port
curl -X POST http://localhost:8080/api/v1/settings/validate-url \
  -H "Content-Type: application/json" \
  -d '{"url": "https://charon.example.com:8443"}'

# Invalid - no protocol
curl -X POST http://localhost:8080/api/v1/settings/validate-url \
  -H "Content-Type: application/json" \
  -d '{"url": "charon.example.com"}'

# Invalid - includes path
curl -X POST http://localhost:8080/api/v1/settings/validate-url \
  -H "Content-Type: application/json" \
  -d '{"url": "https://charon.example.com/admin"}'
```

---

#### Preview User Invite URL

Generates a preview of the invite URL that would be sent to a user, without actually creating the invitation.

```http
POST /users/preview-invite-url
Content-Type: application/json
Authorization: Bearer <admin-token>
```

**Request Body:**

```json
{
  "email": "newuser@example.com"
}
```

**Required Fields:**

- `email` (string) - Email address for the preview

**Response 200 (Configured):**

```json
{
  "preview_url": "https://charon.example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW",
  "base_url": "https://charon.example.com",
  "is_configured": true,
  "email": "newuser@example.com",
  "warning": false,
  "warning_message": ""
}
```

**Response 200 (Not Configured):**

```json
{
  "preview_url": "http://localhost:8080/accept-invite?token=SAMPLE_TOKEN_PREVIEW",
  "base_url": "http://localhost:8080",
  "is_configured": false,
  "email": "newuser@example.com",
  "warning": true,
  "warning_message": "Application URL not configured. The invite link may not be accessible from external networks."
}
```

**Response 400:**

```json
{
  "error": "email is required"
}
```

**Response 403:**

```json
{
  "error": "Admin access required"
}
```

**Field Descriptions:**

- `preview_url` - Complete invite URL with sample token
- `base_url` - The base URL being used (configured or fallback)
- `is_configured` - Whether Application URL is configured in settings
- `email` - Email address from the request (echoed back)
- `warning` - Boolean indicating if there's a configuration warning
- `warning_message` - Human-readable warning (empty if no warning)

**Use Cases:**

1. **Pre-flight check:** Verify invite URLs before creating users
2. **Configuration validation:** Confirm Application URL is set correctly
3. **UI preview:** Show users what invite link will look like
4. **Testing:** Validate invite flow without creating actual invitations

**Examples:**

```bash
# Preview invite URL
curl -X POST http://localhost:8080/api/v1/users/preview-invite-url \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com"}'

# Response when configured:
{
  "preview_url": "https://charon.example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW",
  "base_url": "https://charon.example.com",
  "is_configured": true,
  "email": "admin@example.com",
  "warning": false,
  "warning_message": ""
}
```

**JavaScript Example:**

```javascript
const previewInvite = async (email) => {
  const response = await fetch('http://localhost:8080/api/v1/users/preview-invite-url', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer <admin-token>'
    },
    body: JSON.stringify({ email })
  });

  const data = await response.json();

  if (data.warning) {
    console.warn(data.warning_message);
    console.log('Configure Application URL in System Settings');
  } else {
    console.log('Invite URL:', data.preview_url);
  }
};

previewInvite('newuser@example.com');
```

**Python Example:**

```python
import requests

def preview_invite(email, api_base='http://localhost:8080/api/v1'):
    response = requests.post(
        f'{api_base}/users/preview-invite-url',
        headers={'Content-Type': 'application/json'},
        json={'email': email}
    )

    data = response.json()

    if data.get('warning'):
        print(f"Warning: {data['warning_message']}")
    else:
        print(f"Invite URL: {data['preview_url']}")

    return data

preview_invite('admin@example.com')
```

---

#### Resend User Invite

Resend an invitation email to a pending user. Generates a new invite token and sends it to the user's email address.

```http
POST /users/:id/resend-invite
Authorization: Bearer <admin-token>
```

**Parameters:**

- `id` (path) - User ID (numeric)

**Response 200:**

```json
{
  "email_sent": true,
  "invite_url": "https://charon.example.com/accept-invite?token=abc123...",
  "expires_at": "2026-01-31T12:00:00Z"
}
```

**Response 400:**

```json
{
  "error": "User is not in pending status"
}
```

**Response 403:**

```json
{
  "error": "Admin access required"
}
```

**Response 404:**

```json
{
  "error": "User not found"
}
```

**Use Cases:**

- User didn't receive the original invitation email
- Invite token has expired and needs renewal
- User lost or deleted the invitation email

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/users/42/resend-invite \
  -H "Authorization: Bearer <admin-token>"
```

**JavaScript Example:**

```javascript
const resendInvite = async (userId) => {
  const response = await fetch(`http://localhost:8080/api/v1/users/${userId}/resend-invite`, {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer <admin-token>'
    }
  });

  const data = await response.json();

  if (data.email_sent) {
    console.log('Invitation resent successfully');
  } else {
    console.log('New invite created, but email could not be sent');
    console.log('Invite URL:', data.invite_url);
  }

  return data;
};

resendInvite(42);
```

---

#### Test URL Connectivity

Test if a URL is reachable from the server with comprehensive SSRF (Server-Side Request Forgery) protection.

```http
POST /settings/test-url
Content-Type: application/json
Authorization: Bearer <admin-token>
```

**Request Body:**

```json
{
  "url": "https://api.example.com"
}
```

**Required Fields:**

- `url` (string) - The URL to test for connectivity

**Response 200 (Reachable):**

```json
{
  "reachable": true,
  "latency": 145,
  "message": "URL is reachable",
  "error": ""
}
```

**Response 200 (Unreachable):**

```json
{
  "reachable": false,
  "latency": 0,
  "message": "",
  "error": "connection timeout after 5s"
}
```

**Response 400 (Invalid URL):**

```json
{
  "error": "invalid URL format"
}
```

**Response 403 (Security Block):**

```json
{
  "error": "URL resolves to a private IP address (blocked for security)",
  "details": "SSRF protection: private IP ranges are not allowed"
}
```

**Response 403 (Admin Required):**

```json
{
  "error": "Admin access required"
}
```

**Field Descriptions:**

- `reachable` - Boolean indicating if the URL is accessible
- `latency` - Response time in milliseconds (0 if unreachable)
- `message` - Success message describing the result
- `error` - Error message if the test failed (empty on success)

**Security Features:**

This endpoint implements comprehensive SSRF protection:

1. **DNS Resolution Validation** - Resolves hostname with 3-second timeout
2. **Private IP Blocking** - Blocks 13+ CIDR ranges:
   - RFC 1918 private networks (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`)
   - Loopback addresses (`127.0.0.0/8`, `::1/128`)
   - Link-local addresses (`169.254.0.0/16`, `fe80::/10`)
   - IPv6 Unique Local Addresses (`fc00::/7`)
   - Multicast and other reserved ranges
3. **Cloud Metadata Protection** - Blocks AWS (`169.254.169.254`) and GCP (`metadata.google.internal`) metadata endpoints
4. **Controlled HTTP Request** - HEAD request with 5-second timeout
5. **Limited Redirects** - Maximum 2 redirects allowed
6. **Admin-Only Access** - Requires authenticated admin user

**Use Cases:**

1. **Webhook validation:** Verify webhook endpoints before saving
2. **Application URL testing:** Confirm configured URLs are reachable
3. **Integration setup:** Test external service connectivity
4. **Health checks:** Verify upstream service availability

**Examples:**

```bash
# Test a public URL
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"url": "https://api.github.com"}'

# Response:
{
  "reachable": true,
  "latency": 152,
  "message": "URL is reachable",
  "error": ""
}

# Attempt to test a private IP (blocked)
curl -X POST http://localhost:8080/api/v1/settings/test-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"url": "http://192.168.1.1"}'

# Response:
{
  "error": "URL resolves to a private IP address (blocked for security)",
  "details": "SSRF protection: private IP ranges are not allowed"
}
```

**JavaScript Example:**

```javascript
const testURL = async (url) => {
  const response = await fetch('http://localhost:8080/api/v1/settings/test-url', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer <admin-token>'
    },
    body: JSON.stringify({ url })
  });

  const data = await response.json();

  if (data.reachable) {
    console.log(`✓ ${url} is reachable (${data.latency}ms)`);
  } else {
    console.error(`✗ ${url} failed: ${data.error}`);
  }

  return data;
};

testURL('https://api.example.com');
```

**Python Example:**

```python
import requests

def test_url(url, api_base='http://localhost:8080/api/v1'):
    response = requests.post(
        f'{api_base}/settings/test-url',
        headers={
            'Content-Type': 'application/json',
            'Authorization': 'Bearer <admin-token>'
        },
        json={'url': url}
    )

    data = response.json()

    if response.status_code == 403:
        print(f"Security block: {data.get('error')}")
    elif data.get('reachable'):
        print(f"✓ {url} is reachable ({data['latency']}ms)")
    else:
        print(f"✗ {url} failed: {data['error']}")

    return data

test_url('https://api.github.com')
```

**Security Considerations:**

- Only admin users can access this endpoint
- Private IPs and cloud metadata endpoints are always blocked
- DNS rebinding attacks are prevented by resolving before the HTTP request
- Request timeouts prevent slowloris-style attacks
- Limited redirects prevent redirect loops and excessive resource consumption
- Consider rate limiting this endpoint in production environments

---

### SSL Certificates

#### List All Certificates

```http
GET /certificates
```

**Response 200:**

```json
[
  {
    "id": 1,
    "uuid": "cert-uuid-123",
    "name": "My Custom Cert",
    "provider": "custom",
    "domains": "example.com, www.example.com",
    "expires_at": "2026-01-01T00:00:00Z",
    "created_at": "2025-01-01T10:00:00Z"
  }
]
```

#### Upload Certificate

```http
POST /certificates/upload
Content-Type: multipart/form-data
```

**Request Body:**

- `name` (required) - Certificate name
- `certificate_file` (required) - Certificate file (.crt or .pem)
- `key_file` (required) - Private key file (.key or .pem)

**Response 201:**

```json
{
  "id": 1,
  "uuid": "cert-uuid-123",
  "name": "My Custom Cert",
  "provider": "custom",
  "domains": "example.com"
}
```

#### Delete Certificate

Delete a certificate. Requires that the certificate is not currently in use by any proxy hosts.

```http
DELETE /certificates/:id
```

**Parameters:**

- `id` (path) - Certificate ID (numeric)

**Response 200:**

```json
{
  "message": "certificate deleted"
}
```

**Response 400:**

```json
{
  "error": "invalid id"
}
```

**Response 409:**

```json
{
  "error": "certificate is in use by one or more proxy hosts"
}
```

**Response 500:**

```json
{
  "error": "failed to delete certificate"
}
```

**Note:** A backup is automatically created before deletion. The certificate files are removed from disk along with the database record.

---

### Proxy Hosts

#### List All Proxy Hosts

```http
GET /proxy-hosts
```

**Response 200:**

```json
[
  {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "domain": "example.com, www.example.com",
    "forward_scheme": "http",
    "forward_host": "localhost",
    "forward_port": 8080,
    "ssl_forced": false,
    "http2_support": true,
    "hsts_enabled": false,
    "hsts_subdomains": false,
    "block_exploits": true,
    "websocket_support": false,
    "enabled": true,
    "enable_standard_headers": true,
    "remote_server_id": null,
    "created_at": "2025-01-18T10:00:00Z",
    "updated_at": "2025-01-18T10:00:00Z"
  }
]
```

#### Get Proxy Host

```http
GET /proxy-hosts/:uuid
```

**Parameters:**

- `uuid` (path) - Proxy host UUID

**Response 200:**

```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "domain": "example.com",
  "forward_scheme": "https",
  "forward_host": "backend.internal",
  "forward_port": 9000,
  "ssl_forced": true,
  "websocket_support": false,
  "enabled": true,
  "enable_standard_headers": true,
  "created_at": "2025-01-18T10:00:00Z",
  "updated_at": "2025-01-18T10:00:00Z"
}
```

**Response 404:**

```json
{
  "error": "Proxy host not found"
}
```

#### Create Proxy Host

```http
POST /proxy-hosts
Content-Type: application/json
```

**Request Body:**

```json
{
  "domain": "new.example.com",
  "forward_scheme": "http",
  "forward_host": "localhost",
  "forward_port": 3000,
  "ssl_forced": false,
  "http2_support": true,
  "hsts_enabled": false,
  "hsts_subdomains": false,
  "block_exploits": true,
  "websocket_support": false,
  "enabled": true,
  "enable_standard_headers": true,
  "remote_server_id": null
}
```

**Required Fields:**

- `domain` - Domain name(s), comma-separated
- `forward_host` - Target hostname or IP
- `forward_port` - Target port number

**Optional Fields:**

- `forward_scheme` - Default: `"http"`
- `ssl_forced` - Default: `false`
- `http2_support` - Default: `true`
- `hsts_enabled` - Default: `false`
- `hsts_subdomains` - Default: `false`
- `block_exploits` - Default: `true`
- `websocket_support` - Default: `false`
- `enabled` - Default: `true`
- `enable_standard_headers` - Default: `true` (for new hosts), `false` (for existing hosts migrated from older versions)
  - When `true`: Adds X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, X-Forwarded-Port headers
  - When `false`: Old behavior (headers only added for WebSocket or application-specific needs)
- `remote_server_id` - Default: `null`

**Response 201:**

```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440001",
  "domain": "new.example.com",
  "forward_scheme": "http",
  "forward_host": "localhost",
  "enable_standard_headers": true,
  "forward_port": 3000,
  "created_at": "2025-01-18T10:05:00Z",
  "updated_at": "2025-01-18T10:05:00Z"
}
```

**Response 400:**

```json
{
  "error": "domain is required"
}
```

#### Update Proxy Host

```http
PUT /proxy-hosts/:uuid
Content-Type: application/json
```

**Parameters:**

- `uuid` (path) - Proxy host UUID

**Request Body:** (all fields optional)

```json
{
  "domain": "updated.example.com",
  "forward_port": 8081,
  "ssl_forced": true
}
```

**Response 200:**

```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "domain": "updated.example.com",
  "forward_port": 8081,
  "ssl_forced": true,
  "updated_at": "2025-01-18T10:10:00Z"
}
```

#### Delete Proxy Host

```http
DELETE /proxy-hosts/:uuid
```

**Parameters:**

- `uuid` (path) - Proxy host UUID

**Response 204:** No content

**Response 404:**

```json
{
  "error": "Proxy host not found"
}
```

---

### Remote Servers

#### List All Remote Servers

```http
GET /remote-servers
```

**Query Parameters:**

- `enabled` (optional) - Filter by enabled status (`true` or `false`)

**Response 200:**

```json
[
  {
    "uuid": "660e8400-e29b-41d4-a716-446655440000",
    "name": "Docker Registry",
    "provider": "docker",
    "host": "registry.local",
    "port": 5000,
    "reachable": true,
    "last_checked": "2025-01-18T09:55:00Z",
    "enabled": true,
    "created_at": "2025-01-18T09:00:00Z",
    "updated_at": "2025-01-18T09:55:00Z"
  }
]
```

#### Get Remote Server

```http
GET /remote-servers/:uuid
```

**Parameters:**

- `uuid` (path) - Remote server UUID

**Response 200:**

```json
{
  "uuid": "660e8400-e29b-41d4-a716-446655440000",
  "name": "Docker Registry",
  "provider": "docker",
  "host": "registry.local",
  "port": 5000,
  "reachable": true,
  "enabled": true
}
```

#### Create Remote Server

```http
POST /remote-servers
Content-Type: application/json
```

**Request Body:**

```json
{
  "name": "Production API",
  "provider": "generic",
  "host": "api.prod.internal",
  "port": 8080,
  "enabled": true
}
```

**Required Fields:**

- `name` - Server name
- `host` - Hostname or IP
- `port` - Port number

**Optional Fields:**

- `provider` - One of: `generic`, `docker`, `kubernetes`, `aws`, `gcp`, `azure` (default: `generic`)
- `enabled` - Default: `true`

**Response 201:**

```json
{
  "uuid": "660e8400-e29b-41d4-a716-446655440001",
  "name": "Production API",
  "provider": "generic",
  "host": "api.prod.internal",
  "port": 8080,
  "reachable": false,
  "enabled": true,
  "created_at": "2025-01-18T10:15:00Z"
}
```

#### Update Remote Server

```http
PUT /remote-servers/:uuid
Content-Type: application/json
```

**Request Body:** (all fields optional)

```json
{
  "name": "Updated Name",
  "port": 8081,
  "enabled": false
}
```

**Response 200:**

```json
{
  "uuid": "660e8400-e29b-41d4-a716-446655440000",
  "name": "Updated Name",
  "port": 8081,
  "enabled": false,
  "updated_at": "2025-01-18T10:20:00Z"
}
```

#### Delete Remote Server

```http
DELETE /remote-servers/:uuid
```

**Response 204:** No content

#### Test Remote Server Connection

Test connectivity to a remote server.

```http
POST /remote-servers/:uuid/test
```

**Parameters:**

- `uuid` (path) - Remote server UUID

**Response 200:**

```json
{
  "reachable": true,
  "address": "registry.local:5000",
  "timestamp": "2025-01-18T10:25:00Z"
}
```

**Response 200 (unreachable):**

```json
{
  "reachable": false,
  "address": "offline.server:8080",
  "error": "connection timeout",
  "timestamp": "2025-01-18T10:25:00Z"
}
```

**Note:** This endpoint updates the `reachable` and `last_checked` fields on the remote server.

---

### Live Logs & Notifications

#### Stream Live Logs (WebSocket)

Connect to a WebSocket stream of live security logs. This endpoint uses WebSocket protocol for real-time bidirectional communication.

```http
GET /api/v1/logs/live
Upgrade: websocket
```

**Query Parameters:**

- `level` (optional) - Filter by log level. Values: `debug`, `info`, `warn`, `error`
- `source` (optional) - Filter by log source. Values: `cerberus`, `waf`, `crowdsec`, `acl`

**WebSocket Connection:**

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/logs/live?source=cerberus&level=error');

ws.onmessage = (event) => {
  const logEntry = JSON.parse(event.data);
  console.log(logEntry);
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Connection closed');
};
```

**Message Format:**

Each message received from the WebSocket is a JSON-encoded `LogEntry`:

```json
{
  "level": "error",
  "message": "WAF blocked request from 203.0.113.42",
  "timestamp": "2025-12-09T10:30:45Z",
  "source": "waf",
  "fields": {
    "ip": "203.0.113.42",
    "rule_id": "942100",
    "request_uri": "/api/users?id=1' OR '1'='1",
    "severity": "CRITICAL"
  }
}
```

**Field Descriptions:**

- `level` - Log severity: `debug`, `info`, `warn`, `error`
- `message` - Human-readable log message
- `timestamp` - ISO 8601 timestamp (RFC3339 format)
- `source` - Component that generated the log (e.g., `cerberus`, `waf`, `crowdsec`)
- `fields` - Additional structured data specific to the event type

**Connection Lifecycle:**

- Server sends a ping every 30 seconds to keep connection alive
- Client should respond to pings or connection may timeout
- Server closes connection if client stops reading
- Client can close connection by calling `ws.close()`

**Error Handling:**

- If upgrade fails, returns HTTP 400 with error message
- Authentication required (when auth is implemented)
- Rate limiting applies (when rate limiting is implemented)

**Example: Filter for critical WAF events only**

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/logs/live?source=waf&level=error');
```

---

#### Get Notification Settings

Retrieve current security notification settings.

```http
GET /api/v1/security/notifications/settings
```

**Response 200:**

```json
{
  "enabled": true,
  "min_log_level": "warn",
  "notify_waf_blocks": true,
  "notify_acl_denials": true,
  "notify_rate_limit_hits": false,
  "webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
  "email_recipients": "admin@example.com,security@example.com"
}
```

**Field Descriptions:**

- `enabled` - Master toggle for all notifications
- `min_log_level` - Minimum severity to trigger notifications. Values: `debug`, `info`, `warn`, `error`
- `notify_waf_blocks` - Send notifications for WAF blocking events
- `notify_acl_denials` - Send notifications for ACL denial events
- `notify_rate_limit_hits` - Send notifications for rate limit violations
- `webhook_url` (optional) - URL to POST webhook notifications (Discord, Slack, etc.)
- `email_recipients` (optional) - Comma-separated list of email addresses

**Response 404:**

```json
{
  "error": "Notification settings not configured"
}
```

---

#### Update Notification Settings

Update security notification settings. All fields are optional—only provided fields are updated.

```http
PUT /api/v1/security/notifications/settings
Content-Type: application/json
```

**Request Body:**

```json
{
  "enabled": true,
  "min_log_level": "error",
  "notify_waf_blocks": true,
  "notify_acl_denials": false,
  "notify_rate_limit_hits": false,
  "webhook_url": "https://discord.com/api/webhooks/123456789/abcdefgh",
  "email_recipients": "alerts@example.com"
}
```

**Security Considerations**:

Webhook URLs are validated to prevent SSRF attacks. Blocked destinations:

- Private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Cloud metadata endpoints (169.254.169.254)
- Loopback addresses (127.0.0.0/8)
- Link-local addresses

**Error Response**:

```json
{
  "error": "Invalid webhook URL: URL resolves to a private IP address (blocked for security)"
}
```

**All fields optional:**

- `enabled` (boolean) - Enable/disable all notifications
- `min_log_level` (string) - Must be one of: `debug`, `info`, `warn`, `error`
- `notify_waf_blocks` (boolean) - Toggle WAF block notifications
- `notify_acl_denials` (boolean) - Toggle ACL denial notifications
- `notify_rate_limit_hits` (boolean) - Toggle rate limit notifications
- `webhook_url` (string) - Webhook endpoint URL
- `email_recipients` (string) - Comma-separated email addresses

**Response 200:**

```json
{
  "message": "Settings updated successfully"
}
```

**Response 400:**

```json
{
  "error": "Invalid min_log_level. Must be one of: debug, info, warn, error"
}
```

**Response 500:**

```json
{
  "error": "Failed to update settings"
}
```

**Example: Enable notifications for critical errors only**

```bash
curl -X PUT http://localhost:8080/api/v1/security/notifications/settings \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "min_log_level": "error",
    "notify_waf_blocks": true,
    "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
  }'
```

**Webhook Payload Format:**

When notifications are triggered, Charon sends a POST request to the configured webhook URL:

```json
{
  "event_type": "waf_block",
  "severity": "error",
  "timestamp": "2025-12-09T10:30:45Z",
  "message": "WAF blocked SQL injection attempt",
  "details": {
    "ip": "203.0.113.42",
    "rule_id": "942100",
    "request_uri": "/api/users?id=1' OR '1'='1",
    "user_agent": "curl/7.68.0"
  }
}
```

---

### Import Workflow

#### Check Import Status

Check if there's an active import session.

```http
GET /import/status
```

**Response 200 (no session):**

```json
{
  "has_pending": false
}
```

**Response 200 (active session):**

```json
{
  "has_pending": true,
  "session": {
    "uuid": "770e8400-e29b-41d4-a716-446655440000",
    "filename": "Caddyfile",
    "state": "reviewing",
    "created_at": "2025-01-18T10:30:00Z",
    "updated_at": "2025-01-18T10:30:00Z"
  }
}
```

#### Get Import Preview

Get preview of hosts to be imported (only available when session state is `reviewing`).

```http
GET /import/preview
```

**Response 200:**

```json
{
  "hosts": [
    {
      "domain": "example.com",
      "forward_host": "localhost",
      "forward_port": 8080,
      "forward_scheme": "http"
    },
    {
      "domain": "api.example.com",
      "forward_host": "backend",
      "forward_port": 9000,
      "forward_scheme": "https"
    }
  ],
  "conflicts": [
    "example.com already exists"
  ],
  "errors": []
}
```

**Response 404:**

```json
{
  "error": "No active import session"
}
```

#### Upload Caddyfile

Upload a Caddyfile for import.

```http
POST /import/upload
Content-Type: application/json
```

**Request Body:**

```json
{
  "content": "example.com {\n    reverse_proxy localhost:8080\n}",
  "filename": "Caddyfile"
}
```

**Required Fields:**

- `content` - Caddyfile content

**Optional Fields:**

- `filename` - Original filename (default: `"Caddyfile"`)

**Response 201:**

```json
{
  "session": {
    "uuid": "770e8400-e29b-41d4-a716-446655440000",
    "filename": "Caddyfile",
    "state": "parsing",
    "created_at": "2025-01-18T10:35:00Z"
  }
}
```

**Response 400:**

```json
{
  "error": "content is required"
}
```

#### Upload Multiple Caddyfiles

Upload multiple Caddyfiles in a single request. Useful for importing configurations from multiple site files.

```http
POST /import/upload-multi
Content-Type: application/json
```

**Request Body:**

```json
{
  "files": [
    {
      "filename": "example.com.Caddyfile",
      "content": "example.com {\n    reverse_proxy localhost:8080\n}"
    },
    {
      "filename": "api.example.com.Caddyfile",
      "content": "api.example.com {\n    reverse_proxy localhost:9000\n}"
    }
  ]
}
```

**Required Fields:**

- `files` - Array of file objects (minimum 1)
  - `filename` (string, required) - Original filename
  - `content` (string, required) - Caddyfile content

**Response 200:**

```json
{
  "hosts": [
    {
      "domain": "example.com",
      "forward_host": "localhost",
      "forward_port": 8080,
      "forward_scheme": "http"
    },
    {
      "domain": "api.example.com",
      "forward_host": "localhost",
      "forward_port": 9000,
      "forward_scheme": "http"
    }
  ],
  "conflicts": [],
  "errors": [],
  "warning": ""
}
```

**Response 400 (validation error):**

```json
{
  "error": "files is required and must contain at least one file"
}
```

**Response 400 (parse error with warning):**

```json
{
  "error": "Caddyfile uses file_server which is not supported for import",
  "warning": "file_server directive detected - static file serving is not supported"
}
```

**TypeScript Example:**

```typescript
interface CaddyFile {
  filename: string;
  content: string;
}

const uploadCaddyfilesMulti = async (files: CaddyFile[]): Promise<ImportPreview> => {
  const { data } = await client.post<ImportPreview>('/import/upload-multi', { files });
  return data;
};

// Usage
const files = [
  { filename: 'site1.Caddyfile', content: 'site1.com { reverse_proxy :8080 }' },
  { filename: 'site2.Caddyfile', content: 'site2.com { reverse_proxy :9000 }' }
];
const preview = await uploadCaddyfilesMulti(files);
```

#### Commit Import

Commit the import after resolving conflicts.

```http
POST /import/commit
Content-Type: application/json
```

**Request Body:**

```json
{
  "session_uuid": "770e8400-e29b-41d4-a716-446655440000",
  "resolutions": {
    "example.com": "overwrite",
    "api.example.com": "keep"
  }
}
```

**Required Fields:**

- `session_uuid` - Active import session UUID
- `resolutions` - Map of domain to resolution strategy

**Resolution Strategies:**

- `"keep"` - Keep existing configuration, skip import
- `"overwrite"` - Replace existing with imported configuration
- `"skip"` - Same as keep

**Response 200:**

```json
{
  "imported": 2,
  "skipped": 1,
  "failed": 0
}
```

**Response 400:**

```json
{
  "error": "Invalid session or unresolved conflicts"
}
```

#### Cancel Import

Cancel an active import session.

```http
DELETE /import/cancel?session_uuid=770e8400-e29b-41d4-a716-446655440000
```

**Query Parameters:**

- `session_uuid` - Active import session UUID

**Response 204:** No content

---

## Rate Limiting

🚧 Rate limiting is not yet implemented.

Future rate limits:

- 100 requests per minute per IP
- 1000 requests per hour per IP

## Pagination

🚧 Pagination is not yet implemented.

Future pagination:

```http
GET /proxy-hosts?page=1&per_page=20
```

## Filtering and Sorting

🚧 Advanced filtering is not yet implemented.

Future filtering:

```http
GET /proxy-hosts?enabled=true&sort=created_at&order=desc
```

## Webhooks

🚧 Webhooks are not yet implemented.

Future webhook events:

- `proxy_host.created`
- `proxy_host.updated`
- `proxy_host.deleted`
- `remote_server.unreachable`
- `import.completed`

## SDKs

No official SDKs yet. The API follows REST conventions and can be used with any HTTP client.

### JavaScript/TypeScript Example

```typescript
const API_BASE = 'http://localhost:8080/api/v1';

// List proxy hosts
const hosts = await fetch(`${API_BASE}/proxy-hosts`).then(r => r.json());

// Create proxy host
const newHost = await fetch(`${API_BASE}/proxy-hosts`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    domain: 'example.com',
    forward_host: 'localhost',
    forward_port: 8080
  })
}).then(r => r.json());

// Test remote server
const testResult = await fetch(`${API_BASE}/remote-servers/${uuid}/test`, {
  method: 'POST'
}).then(r => r.json());
```

### Python Example

```python
import requests

API_BASE = 'http://localhost:8080/api/v1'

# List proxy hosts
hosts = requests.get(f'{API_BASE}/proxy-hosts').json()

# Create proxy host
new_host = requests.post(f'{API_BASE}/proxy-hosts', json={
    'domain': 'example.com',
    'forward_host': 'localhost',
    'forward_port': 8080
}).json()

# Test remote server
test_result = requests.post(f'{API_BASE}/remote-servers/{uuid}/test').json()
```

## Support

For API issues or questions:

- GitHub Issues: <https://github.com/Wikid82/charon/issues>
- Discussions: <https://github.com/Wikid82/charon/discussions>
