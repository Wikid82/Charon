# DNS Providers — Implementation Spec

This document was relocated from the former multi-topic [docs/plans/current_spec.md](../plans/current_spec.md) to keep the current plan index SSRF-only.

----

## 2. Scope & Acceptance Criteria

### In Scope

- DNSProvider model with encrypted credential storage
- API endpoints for DNS provider CRUD operations
- Provider connectivity testing (pre-save and post-save)
- Caddy DNS challenge configuration generation
- Frontend management UI for DNS providers
- Integration with proxy host creation (wildcard detection)
- Support for major DNS providers: Cloudflare, Route53, DigitalOcean, Google Cloud DNS, Namecheap, GoDaddy, Azure DNS, Hetzner, Vultr, DNSimple

### Out of Scope (Future Iterations)

- Multi-credential per provider (zone-specific credentials)
- Key rotation automation
- DNS provider auto-detection
- Custom DNS provider plugins

### Acceptance Criteria

- [ ] Users can add, edit, delete, and test DNS provider configurations
- [ ] Credentials are encrypted at rest using AES-256-GCM
- [ ] Credentials are **never** exposed in API responses (masked or omitted)
- [ ] Proxy hosts with wildcard domains can select a DNS provider
- [ ] Caddy successfully obtains wildcard certificates using DNS-01 challenge
- [ ] Backend unit test coverage ≥ 85%
- [ ] Frontend unit test coverage ≥ 85%
- [ ] User documentation completed
- [ ] All translations added for new UI strings

----

## 3. Technical Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                               FRONTEND                                       │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │ DNSProviders    │  │ DNSProviderForm │  │ ProxyHostForm               │  │
│  │ Page            │  │ (Add/Edit)      │  │ (Wildcard + Provider Select)│  │
│  └────────┬────────┘  └────────┬────────┘  └─────────────┬───────────────┘  │
│           │                    │                         │                   │
│           └────────────────────┼─────────────────────────┘                   │
│                                ▼                                             │
│                    ┌───────────────────────┐                                 │
│                    │ api/dnsProviders.ts   │                                 │
│                    │ hooks/useDNSProviders │                                 │
│                    └───────────┬───────────┘                                 │
└────────────────────────────────┼─────────────────────────────────────────────┘
                                 │ HTTP/JSON
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                               BACKEND                                        │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                        API Layer (Gin Router)                          │ │
│  │  /api/v1/dns-providers/*  →  dns_provider_handler.go                   │ │
│  └────────────────────────────────┬───────────────────────────────────────┘ │
│                                   ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                      Service Layer                                      │
│  │  dns_provider_service.go  ←→  crypto/encryption.go (AES-256-GCM)       │
│  └────────────────────────────────┬───────────────────────────────────────┘ │
│                                   ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                        Data Layer (GORM)                               │
│  │  models/dns_provider.go  │  models/proxy_host.go (extended)            │
│  └────────────────────────────────┬───────────────────────────────────────┘ │
│                                   ▼                                          │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                     Caddy Integration                                   │
│  │  caddy/config.go  →  DNS Challenge Issuer Config  →  Caddy Admin API   │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           DNS PROVIDER                                       │
│                    (Cloudflare, Route53, etc.)                               │
│                    TXT Record: _acme-challenge.example.com                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Data Flow for DNS Challenge

```
1. User creates ProxyHost with *.example.com + selects DNSProvider
                              │
                              ▼
2. Backend validates request, fetches DNSProvider credentials (decrypted)
                              │
                              ▼
3. Caddy Manager generates config with DNS challenge issuer:
   {
     "module": "acme",
     "challenges": {
       "dns": {
         "provider": { "name": "cloudflare", "api_token": "..." }
       }
     }
   }
                              │
                              ▼
4. Caddy applies config → initiates ACME order → requests DNS challenge
                              │
                              ▼
5. Caddy's DNS provider module creates TXT record via DNS API
                              │
                              ▼
6. ACME server validates TXT record → issues certificate
                              │
                              ▼
7. Caddy stores certificate → serves HTTPS for *.example.com
```

----

## 4. Database Schema

### DNSProvider Model

```go
// File: backend/internal/models/dns_provider.go

// DNSProvider represents a DNS provider configuration for ACME DNS-01 challenges.
type DNSProvider struct {
    ID                   uint       `json:"id" gorm:"primaryKey"`
    UUID                 string     `json:"uuid" gorm:"uniqueIndex;size:36"`
    Name                 string     `json:"name" gorm:"index;not null;size:255"`
    ProviderType         string     `json:"provider_type" gorm:"index;not null;size:50"`
    Enabled              bool       `json:"enabled" gorm:"default:true;index"`
    IsDefault            bool       `json:"is_default" gorm:"default:false"`

    // Encrypted credentials (JSON blob, encrypted with AES-256-GCM)
    CredentialsEncrypted string     `json:"-" gorm:"type:text;column:credentials_encrypted"`

    // Propagation settings
    PropagationTimeout   int        `json:"propagation_timeout" gorm:"default:120"`  // seconds
    PollingInterval      int        `json:"polling_interval" gorm:"default:5"`       // seconds

    // Usage tracking
    LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
    SuccessCount         int        `json:"success_count" gorm:"default:0"`
    FailureCount         int        `json:"failure_count" gorm:"default:0"`
    LastError            string     `json:"last_error,omitempty" gorm:"type:text"`

    CreatedAt            time.Time  `json:"created_at"`
    UpdatedAt            time.Time  `json:"updated_at"`
}

// TableName specifies the database table name
func (DNSProvider) TableName() string {
    return "dns_providers"
}
```

### ProxyHost Extensions

```go
// File: backend/internal/models/proxy_host.go (additions)

type ProxyHost struct {
    // ... existing fields ...

    // DNS Challenge configuration
    DNSProviderID   *uint        `json:"dns_provider_id,omitempty" gorm:"index"`
    DNSProvider     *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`
    UseDNSChallenge bool         `json:"use_dns_challenge" gorm:"default:false"`
}
```

### Supported Provider Types

| Provider Type | Credential Fields | Caddy DNS Module |
|---------------|-------------------|------------------|
| `cloudflare` | `api_token` OR (`api_key`, `email`) | `cloudflare` |
| `route53` | `access_key_id`, `secret_access_key`, `region` | `route53` |
| `digitalocean` | `auth_token` | `digitalocean` |
| `googleclouddns` | `service_account_json`, `project` | `googleclouddns` |
| `namecheap` | `api_user`, `api_key`, `client_ip` | `namecheap` |
| `godaddy` | `api_key`, `api_secret` | `godaddy` |
| `azure` | `tenant_id`, `client_id`, `client_secret`, `subscription_id`, `resource_group` | `azuredns` |
| `hetzner` | `api_key` | `hetzner` |
| `vultr` | `api_key` | `vultr` |
| `dnsimple` | `oauth_token`, `account_id` | `dnsimple` |

----

## 5. API Specification

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/dns-providers` | List all DNS providers |
| `POST` | `/api/v1/dns-providers` | Create new DNS provider |
| `GET` | `/api/v1/dns-providers/:id` | Get provider details |
| `PUT` | `/api/v1/dns-providers/:id` | Update provider |
| `DELETE` | `/api/v1/dns-providers/:id` | Delete provider |
| `POST` | `/api/v1/dns-providers/:id/test` | Test saved provider |
| `POST` | `/api/v1/dns-providers/test` | Test credentials (pre-save) |
| `GET` | `/api/v1/dns-providers/types` | List supported provider types |

### Request/Response Schemas

#### Create DNS Provider

**Request:** `POST /api/v1/dns-providers`

```json
{
  "name": "Production Cloudflare",
  "provider_type": "cloudflare",
  "credentials": {
    "api_token": "xxxxxxxxxxxxxxxxxxxxxxxxxx"
  },
  "propagation_timeout": 120,
  "polling_interval": 5,
  "is_default": true
}
```

**Response:** `201 Created`

```json
{
  "id": 1,
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Production Cloudflare",
  "provider_type": "cloudflare",
  "enabled": true,
  "is_default": true,
  "has_credentials": true,
  "propagation_timeout": 120,
  "polling_interval": 5,
  "success_count": 0,
  "failure_count": 0,
  "created_at": "2026-01-01T12:00:00Z",
  "updated_at": "2026-01-01T12:00:00Z"
}
```

#### List DNS Providers

**Response:** `GET /api/v1/dns-providers` → `200 OK`

```json
{
  "providers": [
    {
      "id": 1,
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Production Cloudflare",
      "provider_type": "cloudflare",
      "enabled": true,
      "is_default": true,
      "has_credentials": true,
      "propagation_timeout": 120,
      "polling_interval": 5,
      "last_used_at": "2026-01-01T10:30:00Z",
      "success_count": 15,
      "failure_count": 0,
      "created_at": "2025-12-01T08:00:00Z",
      "updated_at": "2026-01-01T10:30:00Z"
    }
  ],
  "total": 1
}
```

#### Test DNS Provider

**Request:** `POST /api/v1/dns-providers/:id/test`

**Response:** `200 OK`

```json
{
  "success": true,
  "message": "DNS provider credentials validated successfully",
  "propagation_time_ms": 2340
}
```

**Error Response:** `400 Bad Request`

```json
{
  "success": false,
  "error": "Authentication failed: invalid API token",
  "code": "INVALID_CREDENTIALS"
}
```

#### Get Provider Types

**Response:** `GET /api/v1/dns-providers/types` → `200 OK`

```json
{
  "types": [
    {
      "type": "cloudflare",
      "name": "Cloudflare",
      "fields": [
        { "name": "api_token", "label": "API Token", "type": "password", "required": true, "hint": "Token with Zone:DNS:Edit permissions" }
      ],
      "documentation_url": "https://developers.cloudflare.com/api/tokens/"
    },
    {
      "type": "route53",
      "name": "Amazon Route 53",
      "fields": [
        { "name": "access_key_id", "label": "Access Key ID", "type": "text", "required": true },
        { "name": "secret_access_key", "label": "Secret Access Key", "type": "password", "required": true },
        { "name": "region", "label": "AWS Region", "type": "text", "required": true, "default": "us-east-1" }
      ],
      "documentation_url": "https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/dns-routing-traffic.html"
    }
  ]
}
```

----

## 6. Backend Implementation

### Phase 1: Encryption Package + DNSProvider Model (~2-3 hours)

**Objective:** Create secure credential storage foundation

#### Files to Create

| File | Description | Complexity |
|------|-------------|------------|
| `backend/internal/crypto/encryption.go` | AES-256-GCM encryption service | Medium |
| `backend/internal/crypto/encryption_test.go` | Encryption unit tests | Low |
| `backend/internal/models/dns_provider.go` | DNSProvider model + validation | Medium |

#### Implementation Details

**Encryption Service:**

```go
// backend/internal/crypto/encryption.go
package crypto

type EncryptionService struct {
    key []byte // 32 bytes for AES-256
}

func NewEncryptionService(keyBase64 string) (*EncryptionService, error)
func (s *EncryptionService) Encrypt(plaintext []byte) (string, error)
func (s *EncryptionService) Decrypt(ciphertextB64 string) ([]byte, error)
```

**Configuration Extension:**

```go
// backend/internal/config/config.go (add)
EncryptionKey string `env:"CHARON_ENCRYPTION_KEY"`
```

### Phase 2: Service Layer + Handlers (~2-3 hours)

**Objective:** Build DNS provider CRUD operations

#### Files to Create

| File | Description | Complexity |
|------|-------------|------------|
| `backend/internal/services/dns_provider_service.go` | DNS provider CRUD + crypto integration | High |
| `backend/internal/services/dns_provider_service_test.go` | Service unit tests | Medium |
| `backend/internal/api/handlers/dns_provider_handler.go` | HTTP handlers | Medium |
| `backend/internal/api/handlers/dns_provider_handler_test.go` | Handler unit tests | Medium |

#### Service Interface

```go
type DNSProviderService interface {
    List(ctx context.Context) ([]DNSProvider, error)
    Get(ctx context.Context, id uint) (*DNSProvider, error)
    Create(ctx context.Context, req CreateDNSProviderRequest) (*DNSProvider, error)
    Update(ctx context.Context, id uint, req UpdateDNSProviderRequest) (*DNSProvider, error)
    Delete(ctx context.Context, id uint) error
    Test(ctx context.Context, id uint) (*TestResult, error)
    TestCredentials(ctx context.Context, req CreateDNSProviderRequest) (*TestResult, error)
    GetDecryptedCredentials(ctx context.Context, id uint) (map[string]string, error)
}
```

### Phase 3: Caddy Integration (~2 hours)

**Objective:** Generate DNS challenge configuration for Caddy

#### Files to Modify

| File | Changes | Complexity |
|------|---------|------------|
| `backend/internal/caddy/types.go` | Add `DNSChallengeConfig`, `ChallengesConfig` types | Low |
| `backend/internal/caddy/config.go` | Add DNS challenge issuer generation logic | High |
| `backend/internal/caddy/manager.go` | Fetch DNS providers when applying config | Medium |
| `backend/internal/api/routes/routes.go` | Register DNS provider routes | Low |

#### Caddy Types Addition

```go
// backend/internal/caddy/types.go

type DNSChallengeConfig struct {
    Provider           map[string]any `json:"provider"`
    PropagationTimeout int64          `json:"propagation_timeout,omitempty"` // nanoseconds
    Resolvers          []string       `json:"resolvers,omitempty"`
}

type ChallengesConfig struct {
    DNS *DNSChallengeConfig `json:"dns,omitempty"`
}
```

----

## 7. Frontend Implementation

### Phase 1: API Client + Hooks (~1-2 hours)

**Objective:** Establish data layer for DNS providers

#### Files to Create

| File | Description | Complexity |
|------|-------------|------------|
| `frontend/src/api/dnsProviders.ts` | API client functions | Low |
| `frontend/src/hooks/useDNSProviders.ts` | React Query hooks | Low |
| `frontend/src/data/dnsProviderSchemas.ts` | Provider field definitions | Low |

### Phase 2: DNS Providers Page (~2-3 hours)

**Objective:** Complete management UI for DNS providers

#### Files to Create

| File | Description | Complexity |
|------|-------------|------------|
| `frontend/src/pages/DNSProviders.tsx` | DNS providers list page | Medium |
| `frontend/src/components/DNSProviderForm.tsx` | Add/edit provider form | High |
| `frontend/src/components/DNSProviderCard.tsx` | Provider card component | Low |

#### UI Wireframe

```
┌─────────────────────────────────────────────────────────────────┐
│ DNS Providers                              [+ Add Provider]     │
│ Configure DNS providers for wildcard certificate issuance      │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ ℹ️ DNS providers are required to issue wildcard certificates │ │
│ │ (e.g., *.example.com) via Let's Encrypt.                    │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ┌─────────────────────────┐  ┌─────────────────────────┐       │
│ │ ☁️ Cloudflare           │  │ 🔶 Route 53            │       │
│ │ Production Account      │  │ AWS Dev Account         │       │
│ │ ⭐ Default  ✅ Active   │  │ ✅ Active               │       │
│ │ Last used: 2 hours ago  │  │ Never used              │       │
│ │ Success: 15 | Failed: 0 │  │ Success: 0 | Failed: 0  │       │
│ │ [Edit] [Test] [Delete]  │  │ [Edit] [Test] [Delete]  │       │
│ └─────────────────────────┘  └─────────────────────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

### Phase 3: Integration with Certificates/Proxy Hosts (~1-2 hours)

**Objective:** Connect DNS providers to certificate workflows

#### Files to Create

| File | Description | Complexity |
|------|-------------|------------|
| `frontend/src/components/DNSProviderSelector.tsx` | Dropdown selector | Low |

#### Files to Modify

| File | Changes | Complexity |
|------|---------|------------|
| `frontend/src/App.tsx` | Add `/dns-providers` route | Low |
| `frontend/src/components/layout/Layout.tsx` | Add navigation link | Low |
| `frontend/src/components/ProxyHostForm.tsx` | Add DNS provider selector for wildcards | Medium |
| `frontend/src/locales/en/translation.json` | Add translation keys | Low |

----

## 8. Security Requirements

### Encryption at Rest

- **Algorithm:** AES-256-GCM (authenticated encryption)
- **Key:** 32-byte key loaded from `CHARON_ENCRYPTION_KEY` environment variable
- **Format:** Base64-encoded ciphertext with prepended nonce

### Key Management

```bash
# Generate key (one-time setup)
openssl rand -base64 32

# Set environment variable
export CHARON_ENCRYPTION_KEY="<base64-encoded-32-byte-key>"
```

- Key MUST be stored in environment variable or secrets manager
- Key MUST NOT be committed to version control
- Key rotation support via `key_version` field (future)

### API Security

- Credentials **NEVER** returned in API responses
- Response includes only `has_credentials: true/false` indicator
- Update requests with empty `credentials` preserve existing values
- Audit logging for all credential access (create, update, decrypt for Caddy)

### Database Security

- `credentials_encrypted` column excluded from JSON serialization (`json:"-"`)
- Database backups should be encrypted separately
- Consider column-level encryption for additional defense-in-depth

----

## 9. Testing Strategy

### Backend Unit Tests (>85% Coverage)

| Test File | Coverage Target | Key Test Cases |
|-----------|-----------------|----------------|
| `crypto/encryption_test.go` | 100% | Encrypt/decrypt roundtrip, invalid key, tampered ciphertext |
| `models/dns_provider_test.go` | 90% | Model validation, table name |
| `services/dns_provider_service_test.go` | 85% | CRUD operations, encryption integration, error handling |
| `handlers/dns_provider_handler_test.go` | 85% | HTTP methods, validation errors, auth required |

### Frontend Unit Tests (>85% Coverage)

| Test File | Coverage Target | Key Test Cases |
|-----------|-----------------|----------------|
| `api/dnsProviders.test.ts` | 90% | API calls, error handling |
| `hooks/useDNSProviders.test.ts` | 85% | Query/mutation behavior |
| `pages/DNSProviders.test.tsx` | 80% | Render states, user interactions |
| `components/DNSProviderForm.test.tsx` | 85% | Form validation, submission |

### Integration Tests

| Test | Description |
|------|-------------|
| `integration/dns_provider_test.go` | Full CRUD flow with database |
| `integration/caddy_dns_challenge_test.go` | Config generation with DNS provider |

### Manual Test Scenarios

1. **Happy Path:**
   - Create Cloudflare provider with valid API token
   - Test connection (expect success)
   - Create proxy host with `*.example.com`
   - Verify Caddy requests DNS challenge
   - Confirm certificate issued

2. **Error Handling:**
   - Create provider with invalid credentials → test fails
   - Delete provider in use by proxy host → error message
   - Attempt wildcard without DNS provider → validation error

3. **Security:**
   - GET provider → credentials NOT in response
   - Update provider without credentials → preserves existing
   - Audit log contains credential access events

----

## 10. Documentation Deliverables

### User Guide: DNS Providers

**Location:** `docs/guides/dns-providers.md`

**Contents:**

- What are DNS providers and why they're needed
- Setting up your first DNS provider
- Managing multiple providers
- Troubleshooting common issues

### Provider-Specific Setup Guides

**Location:** `docs/guides/dns-providers/`

| File | Provider |
|------|----------|
| `cloudflare.md` | Cloudflare (API token creation, permissions) |
| `route53.md` | AWS Route 53 (IAM policy, credentials) |
| `digitalocean.md` | DigitalOcean (token generation) |
| `google-cloud-dns.md` | Google Cloud DNS (service account setup) |
| `azure-dns.md` | Azure DNS (app registration, permissions) |

### Troubleshooting Guide

**Location:** `docs/troubleshooting/dns-challenges.md`

**Contents:**

- DNS propagation delays
- Permission/authentication errors
- Firewall considerations
- Debug logging

----

## 11. Risk Assessment

### Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Encryption key loss | Low | Critical | Document key backup procedures, test recovery |
| DNS provider API changes | Medium | Medium | Abstract provider logic, version-specific adapters |
| Caddy DNS module incompatibility | Low | High | Test against specific Caddy version, pin dependencies |
| Credential exposure in logs | Medium | High | Audit all logging, mask sensitive fields |
| Performance impact of encryption | Low | Low | AES-NI hardware acceleration, minimal overhead |

### Mitigations

1. **Key Loss:** Require key backup during initial setup, document recovery procedures
2. **API Changes:** Use provider abstraction layer, monitor upstream changes
3. **Caddy Compatibility:** Pin Caddy version, comprehensive integration tests
4. **Log Exposure:** Structured logging with field masking, security audit
5. **Performance:** Benchmark encryption operations, consider caching decrypted creds briefly

----

## 12. Phased Delivery Timeline

| Phase | Description | Estimated Time | Dependencies |
|-------|-------------|----------------|--------------|
| **Phase 1** | Foundation (Encryption pkg, DNSProvider model, migrations) | 2-3 hours | None |
| **Phase 2** | Backend Service + API (CRUD handlers, validation) | 2-3 hours | Phase 1 |
| **Phase 3** | Caddy Integration (DNS challenge config generation) | 2 hours | Phase 2 |
| **Phase 4** | Frontend UI (Pages, forms, integration) | 3-4 hours | Phase 2 API |
| **Phase 5** | Testing & Documentation (Unit tests, guides) | 2-3 hours | All phases |

**Total Estimated Time: 11-15 hours**

### Dependency Graph

```
Phase 1 (Foundation)
    │
    ├──► Phase 2 (Backend API)
    │        │
    │        ├──► Phase 3 (Caddy Integration)
    │        │
    │        └──► Phase 4 (Frontend UI)
    │                 │
    └─────────────────┴──► Phase 5 (Testing & Docs)
```

----

## 13. Files to Create

### Backend

| Path | Description |
|------|-------------|
| `backend/internal/crypto/encryption.go` | AES-256-GCM encryption service |
| `backend/internal/crypto/encryption_test.go` | Encryption unit tests |
| `backend/internal/models/dns_provider.go` | DNSProvider model definition |
| `backend/internal/services/dns_provider_service.go` | DNS provider business logic |
| `backend/internal/services/dns_provider_service_test.go` | Service unit tests |
| `backend/internal/api/handlers/dns_provider_handler.go` | HTTP handlers |
| `backend/internal/api/handlers/dns_provider_handler_test.go` | Handler unit tests |
| `backend/integration/dns_provider_test.go` | Integration tests |

### Frontend

| Path | Description |
|------|-------------|
| `frontend/src/api/dnsProviders.ts` | API client functions |
| `frontend/src/hooks/useDNSProviders.ts` | React Query hooks |
| `frontend/src/data/dnsProviderSchemas.ts` | Provider field definitions |
| `frontend/src/pages/DNSProviders.tsx` | DNS providers page |
| `frontend/src/components/DNSProviderForm.tsx` | Add/edit form |
| `frontend/src/components/DNSProviderCard.tsx` | Provider card component |
| `frontend/src/components/DNSProviderSelector.tsx` | Dropdown selector |

### Documentation

| Path | Description |
|------|-------------|
| `docs/guides/dns-providers.md` | User guide |
| `docs/guides/dns-providers/cloudflare.md` | Cloudflare setup |
| `docs/guides/dns-providers/route53.md` | AWS Route 53 setup |
| `docs/guides/dns-providers/digitalocean.md` | DigitalOcean setup |
| `docs/troubleshooting/dns-challenges.md` | Troubleshooting guide |

----

## 14. Files to Modify

### Backend

| Path | Changes |
|------|---------|
| `backend/internal/config/config.go` | Add `EncryptionKey` field |
| `backend/internal/models/proxy_host.go` | Add `DNSProviderID`, `UseDNSChallenge` fields |
| `backend/internal/caddy/types.go` | Add `DNSChallengeConfig`, `ChallengesConfig` types |
| `backend/internal/caddy/config.go` | Add DNS challenge issuer generation |
| `backend/internal/caddy/manager.go` | Load DNS providers when applying config |
| `backend/internal/api/routes/routes.go` | Register DNS provider routes |
| `backend/internal/api/handlers/proxyhost_handler.go` | Handle DNS provider association |
| `backend/cmd/server/main.go` | Initialize encryption service |

### Frontend

| Path | Changes |
|------|---------|
| `frontend/src/App.tsx` | Add `/dns-providers` route |
| `frontend/src/components/layout/Layout.tsx` | Add navigation link to DNS Providers |
| `frontend/src/components/ProxyHostForm.tsx` | Add DNS provider selector for wildcard domains |
| `frontend/src/locales/en/translation.json` | Add `dnsProviders.*` translation keys |

----

## 15. Definition of Done Checklist

### Backend

- [ ] `crypto/encryption.go` implemented with AES-256-GCM
- [ ] `DNSProvider` model created with all fields
- [ ] Database migration created and tested
- [ ] `DNSProviderService` implements full CRUD
- [ ] Credentials encrypted on save, decrypted on demand
- [ ] API handlers for all endpoints
- [ ] Input validation on all endpoints
- [ ] Credentials never exposed in API responses
- [ ] Unit tests pass with ≥85% coverage
- [ ] Integration tests pass

### Caddy Integration

- [ ] DNS challenge config generated correctly
- [ ] ProxyHost correctly associated with DNSProvider
- [ ] Wildcard domains use DNS-01 challenge
- [ ] Non-wildcard domains continue using HTTP-01

### Frontend

- [ ] API client functions implemented
- [ ] React Query hooks working
- [ ] DNS Providers page lists all providers
- [ ] Add/Edit form with dynamic fields per provider
- [ ] Test connection button functional
- [ ] Provider selector in ProxyHost form
- [ ] Wildcard domain detection triggers DNS provider requirement
- [ ] All translations added
- [ ] Unit tests pass with ≥85% coverage

### Security

- [ ] Encryption key documented in setup guide
- [ ] Credentials encrypted at rest verified
- [ ] API responses verified to exclude credentials
- [ ] Audit logging for credential operations
- [ ] Security review completed

### Documentation

- [ ] User guide written
- [ ] Provider-specific guides written (at least Cloudflare, Route53)
- [ ] Troubleshooting guide written
- [ ] API documentation updated
- [ ] CHANGELOG updated

### Final Validation

- [ ] End-to-end test: Create DNS provider → Create wildcard proxy → Certificate issued
- [ ] Error scenarios tested (invalid creds, deleted provider)
- [ ] UI reviewed for accessibility
- [ ] Performance acceptable (no noticeable delays)

----

*Consolidated from backend and frontend research documents*
*Ready for implementation*
