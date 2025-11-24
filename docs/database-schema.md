# Database Schema Documentation

CaddyProxyManager+ uses SQLite with GORM ORM for data persistence. This document describes the database schema and relationships.

## Overview

The database consists of 8 main tables:
- ProxyHost
- RemoteServer
- CaddyConfig
- SSLCertificate
- AccessList
- User
- Setting
- ImportSession

## Entity Relationship Diagram

```
┌─────────────────┐
│   ProxyHost     │
├─────────────────┤
│ UUID            │◄──┐
│ Domain          │   │
│ ForwardScheme   │   │
│ ForwardHost     │   │
│ ForwardPort     │   │
│ SSLForced       │   │
│ WebSocketSupport│   │
│ Enabled         │   │
│ RemoteServerID  │───┘ (optional)
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘
         │
         │ 1:1
         ▼
┌─────────────────┐
│  CaddyConfig    │
├─────────────────┤
│ UUID            │
│ ProxyHostID     │
│ RawConfig       │
│ GeneratedAt     │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘

┌─────────────────┐
│ RemoteServer    │
├─────────────────┤
│ UUID            │
│ Name            │
│ Provider        │
│ Host            │
│ Port            │
│ Reachable       │
│ LastChecked     │
│ Enabled         │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘

┌─────────────────┐
│ SSLCertificate  │
├─────────────────┤
│ UUID            │
│ Name            │
│ DomainNames     │
│ CertPEM         │
│ KeyPEM          │
│ ExpiresAt       │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘

┌─────────────────┐
│  AccessList     │
├─────────────────┤
│ UUID            │
│ Name            │
│ Addresses       │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘

┌─────────────────┐
│      User       │
├─────────────────┤
│ UUID            │
│ Email           │
│ PasswordHash    │
│ IsActive        │
│ IsAdmin         │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘

┌─────────────────┐
│    Setting      │
├─────────────────┤
│ UUID            │
│ Key             │ (unique)
│ Value           │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘

┌─────────────────┐
│ ImportSession   │
├─────────────────┤
│ UUID            │
│ Filename        │
│ State           │
│ CreatedAt       │
│ UpdatedAt       │
└─────────────────┘
```

## Table Details

### ProxyHost

Stores reverse proxy host configurations.

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `domain` | TEXT | Domain names (comma-separated) |
| `forward_scheme` | TEXT | http or https |
| `forward_host` | TEXT | Target server hostname/IP |
| `forward_port` | INTEGER | Target server port |
| `ssl_forced` | BOOLEAN | Force HTTPS redirect |
| `http2_support` | BOOLEAN | Enable HTTP/2 |
| `hsts_enabled` | BOOLEAN | Enable HSTS header |
| `hsts_subdomains` | BOOLEAN | Include subdomains in HSTS |
| `block_exploits` | BOOLEAN | Block common exploits |
| `websocket_support` | BOOLEAN | Enable WebSocket proxying |
| `enabled` | BOOLEAN | Proxy is active |
| `remote_server_id` | UUID | Foreign key to RemoteServer (nullable) |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

**Indexes:**
- Primary key on `uuid`
- Foreign key index on `remote_server_id`

**Relationships:**
- `RemoteServer`: Many-to-One (optional) - Links to remote Caddy instance
- `CaddyConfig`: One-to-One - Generated Caddyfile configuration

### RemoteServer

Stores remote Caddy server connection information.

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `name` | TEXT | Friendly name |
| `provider` | TEXT | generic, docker, kubernetes, aws, gcp, azure |
| `host` | TEXT | Hostname or IP address |
| `port` | INTEGER | Port number (default 2019) |
| `reachable` | BOOLEAN | Connection test result |
| `last_checked` | TIMESTAMP | Last connection test time |
| `enabled` | BOOLEAN | Server is active |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

**Indexes:**
- Primary key on `uuid`
- Index on `enabled` for fast filtering

### CaddyConfig

Stores generated Caddyfile configurations for each proxy host.

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `proxy_host_id` | UUID | Foreign key to ProxyHost |
| `raw_config` | TEXT | Generated Caddyfile content |
| `generated_at` | TIMESTAMP | When config was generated |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

**Indexes:**
- Primary key on `uuid`
- Unique index on `proxy_host_id`

### SSLCertificate

Stores SSL/TLS certificates (future enhancement).

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `name` | TEXT | Certificate name |
| `domain_names` | TEXT | Domains covered (comma-separated) |
| `cert_pem` | TEXT | Certificate in PEM format |
| `key_pem` | TEXT | Private key in PEM format |
| `expires_at` | TIMESTAMP | Certificate expiration |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

### AccessList

Stores IP-based access control lists (future enhancement).

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `name` | TEXT | List name |
| `addresses` | TEXT | IP addresses (comma-separated) |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

### User

Stores user authentication information (future enhancement).

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `email` | TEXT | Email address (unique) |
| `password_hash` | TEXT | Bcrypt password hash |
| `is_active` | BOOLEAN | Account is active |
| `is_admin` | BOOLEAN | Admin privileges |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

**Indexes:**
- Primary key on `uuid`
- Unique index on `email`

### Setting

Stores application-wide settings as key-value pairs.

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `key` | TEXT | Setting key (unique) |
| `value` | TEXT | Setting value (JSON string) |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

**Indexes:**
- Primary key on `uuid`
- Unique index on `key`

**Default Settings:**
- `app_name`: "CaddyProxyManager+"
- `default_scheme`: "http"
- `enable_ssl_by_default`: "false"

### ImportSession

Tracks Caddyfile import sessions.

| Column | Type | Description |
|--------|------|-------------|
| `uuid` | UUID | Primary key |
| `filename` | TEXT | Uploaded filename (optional) |
| `state` | TEXT | parsing, reviewing, completed, failed |
| `created_at` | TIMESTAMP | Creation timestamp |
| `updated_at` | TIMESTAMP | Last update timestamp |

**States:**
- `parsing`: Caddyfile is being parsed
- `reviewing`: Waiting for user to review/resolve conflicts
- `completed`: Import successfully committed
- `failed`: Import failed with errors

## Database Initialization

The database is automatically created and migrated when the application starts. Use the seed script to populate with sample data:

```bash
cd backend
go run ./cmd/seed/main.go
```

### Sample Seed Data

The seed script creates:
- 4 remote servers (Docker registry, API server, web app, database admin)
- 3 proxy hosts (app.local.dev, api.local.dev, docker.local.dev)
- 3 settings (app configuration)
- 1 admin user

## Migration Strategy

GORM AutoMigrate is used for schema migrations:

```go
db.AutoMigrate(
    &models.ProxyHost{},
    &models.RemoteServer{},
    &models.CaddyConfig{},
    &models.SSLCertificate{},
    &models.AccessList{},
    &models.User{},
    &models.Setting{},
    &models.ImportSession{},
)
```

This ensures the database schema stays in sync with model definitions.

## Backup and Restore

### Backup

```bash
cp backend/data/cpm.db backend/data/cpm.db.backup
```

### Restore

```bash
cp backend/data/cpm.db.backup backend/data/cpm.db
```

## Performance Considerations

- **Indexes**: All foreign keys and frequently queried columns are indexed
- **Connection Pooling**: GORM manages connection pooling automatically
- **SQLite Pragmas**: `PRAGMA journal_mode=WAL` for better concurrency
- **Query Optimization**: Use `.Preload()` for eager loading relationships

## Future Enhancements

- Multi-tenancy support with organization model
- Audit log table for tracking changes
- Certificate auto-renewal tracking
- Integration with Let's Encrypt
- Metrics and monitoring data storage
