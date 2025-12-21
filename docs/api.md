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
