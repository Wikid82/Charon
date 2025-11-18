# API Documentation

CaddyProxyManager+ REST API documentation. All endpoints return JSON and use standard HTTP status codes.

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
- `remote_server_id` - Default: `null`

**Response 201:**
```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440001",
  "domain": "new.example.com",
  "forward_scheme": "http",
  "forward_host": "localhost",
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
- GitHub Issues: https://github.com/Wikid82/CaddyProxyManagerPlus/issues
- Discussions: https://github.com/Wikid82/CaddyProxyManagerPlus/discussions
