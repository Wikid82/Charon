
# DNS Future Features Implementation Plan

**Version:** 1.0.0
**Created:** January 3, 2026
**Status:** Ready for Implementation
**Source:** [dns_challenge_future_features.md](dns_challenge_future_features.md)

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Phase Breakdown](#phase-breakdown)
3. [Feature 1: Audit Logging for Credential Operations](#feature-1-audit-logging-for-credential-operations)
4. [Feature 2: Key Rotation Automation](#feature-2-key-rotation-automation)
5. [Feature 3: Multi-Credential per Provider](#feature-3-multi-credential-per-provider)
6. [Feature 4: DNS Provider Auto-Detection](#feature-4-dns-provider-auto-detection)
7. [Feature 5: Custom DNS Provider Plugins](#feature-5-custom-dns-provider-plugins)
8. [Code Sharing and Pattern Reuse](#code-sharing-and-pattern-reuse)
9. [Risk Assessment](#risk-assessment)
10. [Testing Strategy](#testing-strategy)

---

## Executive Summary

This document provides a detailed implementation plan for five DNS Challenge enhancement features. Each feature includes exact file locations, code patterns to follow, dependencies, and acceptance criteria.

### Implementation Order (Recommended)

| Phase | Feature | Priority | Estimated Hours | Dependencies |
|-------|---------|----------|-----------------|--------------|
| 1 | Audit Logging | P0 | 8-12 | None |
| 2 | Key Rotation Automation | P1 | 16-20 | Audit Logging |
| 3 | Multi-Credential per Provider | P1 | 12-16 | Audit Logging |
| 4 | DNS Provider Auto-Detection | P2 | 6-8 | None |
| 5 | Custom DNS Provider Plugins | P3 | 20-24 | All above |

### Existing Code Patterns to Follow

Based on codebase analysis:

- **Models:** Follow pattern in [backend/internal/models/dns_provider.go](../../backend/internal/models/dns_provider.go)
- **Services:** Follow pattern in [backend/internal/services/dns_provider_service.go](../../backend/internal/services/dns_provider_service.go)
- **Handlers:** Follow pattern in [backend/internal/api/handlers/dns_provider_handler.go](../../backend/internal/api/handlers/dns_provider_handler.go)
- **API Client:** Follow pattern in [frontend/src/api/dnsProviders.ts](../../frontend/src/api/dnsProviders.ts)
- **React Hooks:** Follow pattern in [frontend/src/hooks/useDNSProviders.ts](../../frontend/src/hooks/useDNSProviders.ts)

---

## Phase Breakdown

### Phase 1: Security Foundation (Week 1)

**Feature:** Audit Logging for Credential Operations

**Rationale:** Establishes compliance foundation. The existing `SecurityAudit` model ([backend/internal/models/security_audit.go](../../backend/internal/models/security_audit.go)) already exists but needs extension. The `SecurityService.LogAudit()` method ([backend/internal/services/security_service.go#L163](../../backend/internal/services/security_service.go#L163)) is already implemented.

**Blocking:** None - can start immediately

### Phase 2: Security Hardening (Week 2)

**Feature:** Key Rotation Automation

**Rationale:** Critical for security posture. Depends on Audit Logging to track rotation events.

**Blocking:** Audit Logging must be complete to track rotation events

### Phase 3: Advanced Features (Week 3)

**Feature:** Multi-Credential per Provider

**Rationale:** Enables zone-level security isolation. Can be implemented in parallel with Phase 2 if resources allow.

**Blocking:** Audit Logging (to track credential operations per zone)

### Phase 4: UX Enhancement (Week 3-4)

**Feature:** DNS Provider Auto-Detection

**Rationale:** Independent feature that improves user experience. Can be implemented in parallel with Phase 3.

**Blocking:** None - independent feature

### Phase 5: Extensibility (Week 4-5)

**Feature:** Custom DNS Provider Plugins

**Rationale:** Most complex feature. Benefits from mature audit/rotation systems and lessons learned from previous phases.

**Blocking:** All above features should be stable before adding plugin complexity

---

## Feature 1: Audit Logging for Credential Operations

### Overview

- **Priority:** P0 - Critical
- **Estimated Time:** 8-12 hours
- **Dependencies:** None

### File Inventory

#### Backend Files to Modify

| File | Changes |
|------|---------|
| [backend/internal/models/security_audit.go](../../backend/internal/models/security_audit.go) | Extend `SecurityAudit` struct with new fields |
| [backend/internal/services/dns_provider_service.go](../../backend/internal/services/dns_provider_service.go) | Add audit logging to CRUD operations |
| [backend/internal/services/security_service.go](../../backend/internal/services/security_service.go) | Add `ListAuditLogs()` method with filtering |
| [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go) | Register new audit log routes |

#### Backend Files to Create

| File | Purpose |
|------|---------|
| `backend/internal/api/handlers/audit_log_handler.go` | REST API handler for audit logs |
| `backend/internal/api/handlers/audit_log_handler_test.go` | Unit tests (85% coverage target) |
| `backend/internal/services/audit_log_service.go` | Service layer for audit log querying |
| `backend/internal/services/audit_log_service_test.go` | Unit tests (85% coverage target) |

#### Frontend Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/api/auditLogs.ts` | API client functions |
| `frontend/src/hooks/useAuditLogs.ts` | React Query hooks |
| `frontend/src/pages/AuditLogs.tsx` | Audit logs page component |
| `frontend/src/pages/__tests__/AuditLogs.test.tsx` | Component tests |

#### Database Migration

**Pattern:** GORM AutoMigrate (follows existing codebase pattern)

1. Update the `SecurityAudit` model in [backend/internal/models/security_audit.go](../../backend/internal/models/security_audit.go) with new fields and indexes
2. Run `db.AutoMigrate(&models.SecurityAudit{})` in the application initialization code
3. GORM will automatically add missing columns and indexes

**Note:** The project does not use standalone SQL migration files. All schema changes are managed through GORM's AutoMigrate feature.

### Model Extension

**File:** `backend/internal/models/security_audit.go`

```go
// SecurityAudit records admin actions or important changes related to security.
type SecurityAudit struct {
    ID            uint       `json:"id" gorm:"primaryKey"`
    UUID          string     `json:"uuid" gorm:"uniqueIndex"`
    Actor         string     `json:"actor" gorm:"index"`               // User ID or "system"
    Action        string     `json:"action"`                           // e.g., "dns_provider_create"
    EventCategory string     `json:"event_category" gorm:"index"`      // "dns_provider", "certificate", etc.
    ResourceID    *uint      `json:"resource_id,omitempty"`            // DNSProvider.ID
    ResourceUUID  string     `json:"resource_uuid,omitempty" gorm:"index"` // DNSProvider.UUID
    Details       string     `json:"details" gorm:"type:text"`         // JSON blob with event metadata
    IPAddress     string     `json:"ip_address,omitempty"`             // Request originator IP
    UserAgent     string     `json:"user_agent,omitempty"`             // Browser/API client
    CreatedAt     time.Time  `json:"created_at" gorm:"index"`
}
```

### API Endpoints

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| `GET` | `/api/v1/audit-logs` | `AuditLogHandler.List` | List with pagination and filtering |
| `GET` | `/api/v1/audit-logs/:uuid` | `AuditLogHandler.Get` | Get single audit event |
| `GET` | `/api/v1/dns-providers/:id/audit-logs` | `AuditLogHandler.ListByProvider` | Provider-specific audit history |
| `DELETE` | `/api/v1/audit-logs/cleanup` | `AuditLogHandler.Cleanup` | Manual cleanup (admin only) |

### Audit Events to Implement

| Event Action | Trigger Location | Details JSON |
|--------------|------------------|--------------|
| `dns_provider_create` | `DNSProviderService.Create()` | `{"name","type","is_default"}` |
| `dns_provider_update` | `DNSProviderService.Update()` | `{"changed_fields","old_values","new_values"}` |
| `dns_provider_delete` | `DNSProviderService.Delete()` | `{"name","type","had_credentials"}` |
| `credential_test` | `DNSProviderService.Test()` | `{"provider_name","test_result","error"}` |
| `credential_decrypt` | `DNSProviderService.GetDecryptedCredentials()` | `{"purpose","success"}` |

### Definition of Done

- [ ] SecurityAudit model extended with new fields and index tags
- [ ] GORM AutoMigrate adds columns and indexes successfully
- [ ] All DNS provider CRUD operations logged via buffered channel (capacity: 100)
- [ ] Credential decryption events logged asynchronously
- [ ] SecurityService extended with `ListAuditLogs()` filtering/pagination methods
- [ ] API endpoints return paginated audit logs
- [ ] Frontend AuditLogs page displays table with filters
- [ ] Route added to App.tsx router configuration
- [ ] Navigation link integrated in main menu
- [ ] Export to CSV/JSON functionality works
- [ ] Retention policy configurable (default: 90 days)
- [ ] Background cleanup job removes old logs
- [ ] Backend tests achieve ≥85% coverage
- [ ] Frontend tests achieve ≥85% coverage
- [ ] API documentation updated
- [ ] No performance regression (async audit logging <1ms overhead)

---

## Feature 2: Key Rotation Automation

### Overview

- **Priority:** P1
- **Estimated Time:** 16-20 hours
- **Dependencies:** Audit Logging (to track rotation events)

### File Inventory

#### Backend Files to Create

| File | Purpose |
|------|---------|
| `backend/internal/crypto/rotation_service.go` | Key rotation logic with multi-key support |
| `backend/internal/crypto/rotation_service_test.go` | Unit tests (85% coverage target) |
| `backend/internal/api/handlers/encryption_handler.go` | Admin API for key management |
| `backend/internal/api/handlers/encryption_handler_test.go` | Unit tests |

#### Backend Files to Modify

| File | Changes |
|------|---------|
| [backend/internal/crypto/encryption.go](../../backend/internal/crypto/encryption.go) | Add key version tracking capability |
| [backend/internal/models/dns_provider.go](../../backend/internal/models/dns_provider.go) | Add `KeyVersion` field |
| [backend/internal/services/dns_provider_service.go](../../backend/internal/services/dns_provider_service.go) | Use RotationService for encryption/decryption |
| [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go) | Register admin encryption routes |

#### Frontend Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/api/encryption.ts` | API client for encryption management |
| `frontend/src/hooks/useEncryption.ts` | React Query hooks |
| `frontend/src/pages/EncryptionManagement.tsx` | Admin page for key management |
| `frontend/src/pages/__tests__/EncryptionManagement.test.tsx` | Component tests |

#### Database Migration

**Pattern:** GORM AutoMigrate (follows existing codebase pattern)

1. Add `KeyVersion` field to `DNSProvider` model with gorm index tag
2. Run `db.AutoMigrate(&models.DNSProvider{})` in the application initialization code
3. GORM will automatically add the `key_version` column and index

**Note:** Key version tracking is managed purely through environment variables. No `encryption_keys` table is needed. This aligns with 12-factor principles and simplifies the architecture.

### Environment Variable Schema

**Version tracking is managed purely via environment variables** - no database table needed.

```bash
# Current encryption key (required) - Version 1 by default
CHARON_ENCRYPTION_KEY=<base64-encoded-32-byte-key>

# During rotation: new key to encrypt with (becomes Version 2)
CHARON_ENCRYPTION_KEY_V2=<base64-encoded-32-byte-key>

# Legacy keys for decryption only (up to 10 versions supported)
CHARON_ENCRYPTION_KEY_V1=<old-key>  # Previous version
CHARON_ENCRYPTION_KEY_V3=<even-older-key>  # If rotating multiple times
```

**Rotation Flow:**

1. Set `CHARON_ENCRYPTION_KEY_V2` with new key
2. Restart application (loads both keys)
3. Trigger `/api/v1/admin/encryption/rotate` endpoint
4. All credentials re-encrypted with V2
5. Rename: `CHARON_ENCRYPTION_KEY_V2` → `CHARON_ENCRYPTION_KEY`, old key → `CHARON_ENCRYPTION_KEY_V1`
6. Restart application

### RotationService Interface

```go
type RotationService interface {
    // DecryptWithVersion decrypts using the appropriate key version
    DecryptWithVersion(ciphertextB64 string, version int) ([]byte, error)

    // EncryptWithCurrentKey encrypts with current (or next during rotation) key
    EncryptWithCurrentKey(plaintext []byte) (ciphertext string, version int, err error)

    // RotateAllCredentials re-encrypts all credentials with new key
    RotateAllCredentials(ctx context.Context) (*RotationResult, error)

    // GetStatus returns current rotation status
    GetStatus() *RotationStatus

    // ValidateKeyConfiguration checks all configured keys
    ValidateKeyConfiguration() error
}

type RotationResult struct {
    TotalProviders   int       `json:"total_providers"`
    SuccessCount     int       `json:"success_count"`
    FailureCount     int       `json:"failure_count"`
    FailedProviders  []uint    `json:"failed_providers,omitempty"`
    Duration         string    `json:"duration"`
    NewKeyVersion    int       `json:"new_key_version"`
}

type RotationStatus struct {
    CurrentVersion   int       `json:"current_version"`
    NextKeyConfigured bool     `json:"next_key_configured"`
    LegacyKeyCount   int       `json:"legacy_key_count"`
    ProvidersOnCurrentVersion int `json:"providers_on_current_version"`
    ProvidersOnOlderVersions  int `json:"providers_on_older_versions"`
}
```

### API Endpoints

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| `GET` | `/api/v1/admin/encryption/status` | `EncryptionHandler.GetStatus` | Current key version, rotation status |
| `POST` | `/api/v1/admin/encryption/rotate` | `EncryptionHandler.Rotate` | Trigger credential re-encryption |
| `GET` | `/api/v1/admin/encryption/history` | `EncryptionHandler.GetHistory` | Key rotation audit log |
| `POST` | `/api/v1/admin/encryption/validate` | `EncryptionHandler.Validate` | Validate key configuration |

### Definition of Done

- [ ] RotationService created with multi-key support
- [ ] Legacy key loading from environment variables works
- [ ] Decrypt falls back to older key versions automatically
- [ ] Encrypt uses current (or NEXT during rotation) key
- [ ] RotateAllCredentials re-encrypts all providers
- [ ] Rotation progress tracking (% complete)
- [ ] Audit events logged for all rotation operations
- [ ] Admin UI shows rotation status and controls
- [ ] Zero-downtime rotation verified in testing
- [ ] Rollback procedure documented
- [ ] Backend tests achieve ≥85% coverage
- [ ] Frontend tests achieve ≥85% coverage
- [ ] Operations guide documents rotation procedure

---

## Feature 3: Multi-Credential per Provider

### Overview

- **Priority:** P1
- **Estimated Time:** 12-16 hours
- **Dependencies:** Audit Logging

### File Inventory

#### Backend Files to Create

| File | Purpose |
|------|---------|
| `backend/internal/models/dns_provider_credential.go` | New credential model |
| `backend/internal/models/dns_provider_credential_test.go` | Model tests |
| `backend/internal/services/credential_service.go` | CRUD for zone-specific credentials |
| `backend/internal/services/credential_service_test.go` | Unit tests (85% coverage target) |
| `backend/internal/api/handlers/credential_handler.go` | REST API for credentials |
| `backend/internal/api/handlers/credential_handler_test.go` | Unit tests |

#### Backend Files to Modify

| File | Changes |
|------|---------|
| [backend/internal/models/dns_provider.go](../../backend/internal/models/dns_provider.go) | Add `UseMultiCredentials` flag and `Credentials` relation |
| [backend/internal/services/dns_provider_service.go](../../backend/internal/services/dns_provider_service.go) | Add `GetCredentialForDomain()` method |
| [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go) | Register credential routes |

#### Frontend Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/api/credentials.ts` | API client for credentials |
| `frontend/src/hooks/useCredentials.ts` | React Query hooks |
| `frontend/src/components/CredentialManager.tsx` | Modal for managing credentials |
| `frontend/src/components/__tests__/CredentialManager.test.tsx` | Component tests |

#### Frontend Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/pages/DNSProviders.tsx` | Add multi-credential toggle and management UI |

#### Database Migration

**Pattern:** GORM AutoMigrate (follows existing codebase pattern)

1. Create `DNSProviderCredential` model in `backend/internal/models/dns_provider_credential.go` with gorm tags
2. Add `UseMultiCredentials` field to `DNSProvider` model
3. Run `db.AutoMigrate(&models.DNSProviderCredential{}, &models.DNSProvider{})` in the application initialization code
4. GORM will automatically create the table, foreign keys, and indexes

**Note:** The `key_version` field tracks which encryption key version was used. Versions are managed via environment variables only (see Feature 2).

### DNSProviderCredential Model

```go
// DNSProviderCredential represents a zone-specific credential set.
type DNSProviderCredential struct {
    ID                   uint       `json:"id" gorm:"primaryKey"`
    UUID                 string     `json:"uuid" gorm:"uniqueIndex;size:36"`
    DNSProviderID        uint       `json:"dns_provider_id" gorm:"index;not null"`
    DNSProvider          *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`

    Label                string     `json:"label" gorm:"not null;size:255"`
    ZoneFilter           string     `json:"zone_filter" gorm:"type:text"` // Comma-separated domains
    CredentialsEncrypted string     `json:"-" gorm:"type:text;not null"`
    Enabled              bool       `json:"enabled" gorm:"default:true"`

    PropagationTimeout   int        `json:"propagation_timeout" gorm:"default:120"`
    PollingInterval      int        `json:"polling_interval" gorm:"default:5"`
    KeyVersion           int        `json:"key_version" gorm:"default:1"`

    LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
    SuccessCount         int        `json:"success_count" gorm:"default:0"`
    FailureCount         int        `json:"failure_count" gorm:"default:0"`
    LastError            string     `json:"last_error,omitempty" gorm:"type:text"`

    CreatedAt            time.Time  `json:"created_at"`
    UpdatedAt            time.Time  `json:"updated_at"`
}
```

### Zone Matching Algorithm

```go
// GetCredentialForDomain selects the best credential match for a domain
// Priority: exact match > wildcard match > catch-all (empty zone_filter)
func (s *dnsProviderService) GetCredentialForDomain(ctx context.Context, providerID uint, domain string) (*models.DNSProviderCredential, error) {
    // 1. If not using multi-credentials, return default
    // 2. Find exact domain match
    // 3. Find wildcard match (*.example.com matches app.example.com)
    // 4. Find catch-all (empty zone_filter)
    // 5. Return error if no match
}
```

### API Endpoints

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| `GET` | `/api/v1/dns-providers/:id/credentials` | `CredentialHandler.List` | List all credentials |
| `POST` | `/api/v1/dns-providers/:id/credentials` | `CredentialHandler.Create` | Create credential |
| `GET` | `/api/v1/dns-providers/:id/credentials/:cred_id` | `CredentialHandler.Get` | Get single credential |
| `PUT` | `/api/v1/dns-providers/:id/credentials/:cred_id` | `CredentialHandler.Update` | Update credential |
| `DELETE` | `/api/v1/dns-providers/:id/credentials/:cred_id` | `CredentialHandler.Delete` | Delete credential |
| `POST` | `/api/v1/dns-providers/:id/credentials/:cred_id/test` | `CredentialHandler.Test` | Test credential |
| `POST` | `/api/v1/dns-providers/:id/enable-multi-credentials` | `CredentialHandler.EnableMulti` | Migrate to multi-credential mode |

### Definition of Done

- [ ] DNSProviderCredential model created
- [ ] Database migration runs without errors
- [ ] CRUD operations for credentials work
- [ ] Zone matching algorithm implemented and tested
- [ ] Credential selection integrated with Caddy config generation
- [ ] Migration from single to multi-credential mode works
- [ ] Backward compatibility maintained (providers default to single credential)
- [ ] Frontend CredentialManager modal functional
- [ ] Audit events logged for credential operations
- [ ] Backend tests achieve ≥85% coverage
- [ ] Frontend tests achieve ≥85% coverage
- [ ] Documentation updated with multi-credential setup guide

---

## Feature 4: DNS Provider Auto-Detection

### Overview

- **Priority:** P2
- **Estimated Time:** 6-8 hours
- **Dependencies:** None (can be developed in parallel)

### File Inventory

#### Backend Files to Create

| File | Purpose |
|------|---------|
| `backend/internal/services/dns_detection_service.go` | Nameserver lookup and pattern matching |
| `backend/internal/services/dns_detection_service_test.go` | Unit tests (85% coverage target) |
| `backend/internal/api/handlers/dns_detection_handler.go` | REST API for detection |
| `backend/internal/api/handlers/dns_detection_handler_test.go` | Unit tests |

#### Backend Files to Modify

| File | Changes |
|------|---------|
| [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go) | Register detection route |

#### Frontend Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/api/dnsDetection.ts` | API client for detection |
| `frontend/src/hooks/useDNSDetection.ts` | React Query hooks |

#### Frontend Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/pages/ProxyHosts.tsx` | Integrate auto-detection in form |

### Nameserver Pattern Database

```go
// BuiltInNameservers maps nameserver patterns to provider types
var BuiltInNameservers = map[string]string{
    // Cloudflare
    ".ns.cloudflare.com":      "cloudflare",

    // AWS Route 53
    ".awsdns":                 "route53",

    // DigitalOcean
    ".digitalocean.com":       "digitalocean",

    // Google Cloud DNS
    ".googledomains.com":      "googleclouddns",
    "ns-cloud":                "googleclouddns",

    // Azure DNS
    ".azure-dns":              "azure",

    // Namecheap
    ".registrar-servers.com":  "namecheap",

    // GoDaddy
    ".domaincontrol.com":      "godaddy",

    // Hetzner
    ".hetzner.com":            "hetzner",
    ".hetzner.de":             "hetzner",

    // Vultr
    ".vultr.com":              "vultr",

    // DNSimple
    ".dnsimple.com":           "dnsimple",
}
```

### DNSDetectionService Interface

```go
type DNSDetectionService interface {
    // DetectProvider identifies the DNS provider for a domain
    DetectProvider(domain string) (*DetectionResult, error)

    // SuggestConfiguredProvider finds a matching configured provider
    SuggestConfiguredProvider(ctx context.Context, domain string) (*models.DNSProvider, error)

    // GetNameserverPatterns returns current pattern database
    GetNameserverPatterns() map[string]string
}

type DetectionResult struct {
    Domain            string   `json:"domain"`
    Detected          bool     `json:"detected"`
    ProviderType      string   `json:"provider_type,omitempty"`
    Nameservers       []string `json:"nameservers"`
    Confidence        string   `json:"confidence"` // "high", "medium", "low", "none"
    SuggestedProvider *models.DNSProvider `json:"suggested_provider,omitempty"`
    Error             string   `json:"error,omitempty"`
}
```

### API Endpoint

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| `POST` | `/api/v1/dns-providers/detect` | `DNSDetectionHandler.Detect` | Detect provider for domain |

**Request:**

```json
{
  "domain": "example.com"
}
```

**Response:**

```json
{
  "domain": "example.com",
  "detected": true,
  "provider_type": "cloudflare",
  "nameservers": ["ns1.cloudflare.com", "ns2.cloudflare.com"],
  "confidence": "high",
  "suggested_provider": {
    "id": 1,
    "name": "Production Cloudflare",
    "provider_type": "cloudflare"
  }
}
```

### Frontend Integration

```tsx
// In ProxyHostForm.tsx
const { detectProvider, isDetecting } = useDNSDetection()

useEffect(() => {
  if (hasWildcardDomain && domain) {
    const baseDomain = domain.replace(/^\*\./, '')
    detectProvider(baseDomain).then(result => {
      if (result.suggested_provider) {
        setDNSProviderID(result.suggested_provider.id)
        toast.info(`Auto-detected: ${result.suggested_provider.name}`)
      }
    })
  }
}, [domain, hasWildcardDomain])
```

### Definition of Done

- [ ] DNSDetectionService created with pattern matching
- [ ] Nameserver lookup via `net.LookupNS()` works
- [ ] Results cached with 1-hour TTL
- [ ] Detection endpoint returns provider suggestions
- [ ] Frontend auto-fills DNS provider on wildcard domain entry
- [ ] Manual override available (user can change detected provider)
- [ ] Detection accuracy >95% for supported providers
- [ ] Backend tests achieve ≥85% coverage
- [ ] Frontend tests achieve ≥85% coverage
- [ ] Performance: detection <500ms per domain

---

## Feature 5: Custom DNS Provider Plugins

### Overview

- **Priority:** P3
- **Estimated Time:** 20-24 hours
- **Dependencies:** All above features (mature and stable before adding plugins)

**⚠️ Platform Limitation:** Go plugins (`.so` files) are **only supported on Linux and macOS**. Windows does not support Go's plugin system. For Windows deployments, use Docker containers (recommended) or compile all providers as built-in.

### File Inventory

#### Backend Files to Create

| File | Purpose |
|------|---------|
| `backend/pkg/dnsprovider/interface.go` | Plugin interface definition |
| `backend/pkg/dnsprovider/builtin.go` | Built-in providers adapter |
| `backend/internal/services/plugin_loader.go` | Plugin loading and management |
| `backend/internal/services/plugin_loader_test.go` | Unit tests |
| `backend/internal/api/handlers/plugin_handler.go` | REST API for plugin management |
| `backend/internal/api/handlers/plugin_handler_test.go` | Unit tests |

#### Example Plugin (separate directory)

| File | Purpose |
|------|---------|
| `plugins/powerdns/powerdns_plugin.go` | Example PowerDNS plugin |
| `plugins/powerdns/README.md` | Plugin documentation |

#### Documentation

| File | Purpose |
|------|---------|
| `docs/development/dns-plugins.md` | Plugin development guide |
| `docs/development/plugin-security.md` | Security guidelines for plugins |

#### Frontend Files to Create

| File | Purpose |
|------|---------|
| `frontend/src/api/plugins.ts` | API client for plugins |
| `frontend/src/hooks/usePlugins.ts` | React Query hooks |
| `frontend/src/pages/PluginManagement.tsx` | Admin page for plugin management |
| `frontend/src/pages/__tests__/PluginManagement.test.tsx` | Component tests |

### Plugin Interface

```go
// Package dnsprovider defines the interface for DNS provider plugins.
package dnsprovider

import "time"

// Provider is the interface that DNS provider plugins must implement.
type Provider interface {
    // GetType returns the provider type identifier (e.g., "custom_powerdns")
    GetType() string

    // GetMetadata returns provider metadata for UI
    GetMetadata() ProviderMetadata

    // ValidateCredentials checks if credentials are valid
    ValidateCredentials(credentials map[string]string) error

    // CreateTXTRecord creates a DNS TXT record for ACME challenge
    CreateTXTRecord(zone, name, value string, credentials map[string]string) error

    // DeleteTXTRecord removes a DNS TXT record after challenge
    DeleteTXTRecord(zone, name string, credentials map[string]string) error

    // GetPropagationTimeout returns recommended DNS propagation wait time
    GetPropagationTimeout() time.Duration
}

// ProviderMetadata describes a DNS provider plugin.
type ProviderMetadata struct {
    Type              string            `json:"type"`
    Name              string            `json:"name"`
    Description       string            `json:"description"`
    DocumentationURL  string            `json:"documentation_url"`
    CredentialFields  []CredentialField `json:"credential_fields"`
    Author            string            `json:"author"`
    Version           string            `json:"version"`
}

// CredentialField describes a credential input field.
type CredentialField struct {
    Name        string `json:"name"`
    Label       string `json:"label"`
    Type        string `json:"type"` // "text", "password", "textarea"
    Required    bool   `json:"required"`
    Placeholder string `json:"placeholder,omitempty"`
    Hint        string `json:"hint,omitempty"`
}
```

### Plugin Loader Service

```go
type PluginLoader interface {
    // LoadPlugins scans plugin directory and loads all valid plugins
    LoadPlugins() error

    // LoadPlugin loads a single plugin from path
    LoadPlugin(path string) error

    // GetProvider returns a loaded plugin provider
    GetProvider(providerType string) (dnsprovider.Provider, bool)

    // ListProviders returns metadata for all loaded plugins
    ListProviders() []dnsprovider.ProviderMetadata

    // VerifySignature validates plugin signature
    VerifySignature(pluginPath string, expectedSig string) error
}
```

### Security Requirements

1. **Signature Verification:** All plugins must have SHA-256 hash verified
2. **Allowlist:** Admin must explicitly enable each plugin
3. **Configuration file:** `config/plugins.yaml`

```yaml
dns_providers:
  - plugin: powerdns
    enabled: true
    verified_signature: "sha256:abcd1234..."
  - plugin: custom_internal
    enabled: false
```

### API Endpoints

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| `GET` | `/api/v1/admin/plugins` | `PluginHandler.List` | List loaded plugins |
| `GET` | `/api/v1/admin/plugins/:type` | `PluginHandler.Get` | Get plugin details |
| `POST` | `/api/v1/admin/plugins/:type/enable` | `PluginHandler.Enable` | Enable plugin |
| `POST` | `/api/v1/admin/plugins/:type/disable` | `PluginHandler.Disable` | Disable plugin |
| `POST` | `/api/v1/admin/plugins/reload` | `PluginHandler.Reload` | Reload all plugins |

### Definition of Done

- [ ] Plugin interface defined in `backend/pkg/dnsprovider/`
- [ ] PluginLoader service loads `.so` files from `CHARON_PLUGIN_DIR` (Linux/macOS only)
- [ ] Signature verification implemented
- [ ] Allowlist enforcement works
- [ ] Example PowerDNS plugin compiles and loads on Linux/macOS
- [ ] Built-in providers wrapped to use same interface
- [ ] Admin UI shows loaded plugins
- [ ] Plugin development guide written with platform limitations documented
- [ ] Docker deployment guide emphasizes Docker as primary target for plugins
- [ ] Windows compatibility note added (plugins not supported, use Docker)
- [ ] Backend tests achieve ≥85% coverage
- [ ] Frontend tests achieve ≥85% coverage
- [ ] Security review completed

---

## Code Sharing and Pattern Reuse

### Shared Components

Several features can share code to reduce duplication:

| Shared Pattern | Used By | Location |
|----------------|---------|----------|
| Pagination helper | Audit Logs, Credentials | `backend/internal/api/handlers/pagination.go` |
| Encryption/Decryption | DNS Service, Credential Service, Rotation Service | `backend/internal/crypto/` |
| Audit logging helper | All CRUD operations | `backend/internal/services/audit_helper.go` |
| SecurityService extension | Audit log filtering/pagination | `backend/internal/services/security_service.go` (extend existing) |
| Async audit channel | Non-blocking audit logging | Buffered channel (capacity: 100) in SecurityService |
| React Query key factory | All new hooks | `frontend/src/hooks/queryKeys.ts` |
| Table with filters component | Audit Logs, Credentials | `frontend/src/components/DataTable.tsx` |

### Audit Logging Helper

Create a reusable helper with **async buffered channel** for non-blocking audit logging:

```go
// backend/internal/services/audit_helper.go

type AuditHelper struct {
    securityService *SecurityService
    auditChan       chan *models.SecurityAudit // Buffered channel (capacity: 100)
}

func NewAuditHelper(securityService *SecurityService) *AuditHelper {
    h := &AuditHelper{
        securityService: securityService,
        auditChan:       make(chan *models.SecurityAudit, 100),
    }
    go h.processAuditEvents() // Background goroutine
    return h
}

func (h *AuditHelper) LogDNSProviderEvent(ctx context.Context, action string, provider *models.DNSProvider, details map[string]interface{}) {
    detailsJSON, _ := json.Marshal(details)
    audit := &models.SecurityAudit{
        Actor:         getUserIDFromContext(ctx),
        Action:        action,
        EventCategory: "dns_provider",
        ResourceID:    &provider.ID,
        ResourceUUID:  provider.UUID,
        Details:       string(detailsJSON),
        IPAddress:     getIPFromContext(ctx),
        UserAgent:     getUserAgentFromContext(ctx),
    }
    // Non-blocking send (drops if buffer full to avoid blocking main operations)
    select {
    case h.auditChan <- audit:
    default:
        // Log dropped event metric
    }
}

func (h *AuditHelper) processAuditEvents() {
    for audit := range h.auditChan {
        h.securityService.LogAudit(audit)
    }
}
```

### Frontend Query Key Factory

```typescript
// frontend/src/hooks/queryKeys.ts

export const queryKeys = {
  // DNS Providers
  dnsProviders: {
    all: ['dns-providers'] as const,
    lists: () => [...queryKeys.dnsProviders.all, 'list'] as const,
    detail: (id: number) => [...queryKeys.dnsProviders.all, 'detail', id] as const,
    credentials: (id: number) => [...queryKeys.dnsProviders.all, id, 'credentials'] as const,
  },

  // Audit Logs
  auditLogs: {
    all: ['audit-logs'] as const,
    list: (filters?: AuditLogFilters) => [...queryKeys.auditLogs.all, 'list', filters] as const,
    detail: (uuid: string) => [...queryKeys.auditLogs.all, 'detail', uuid] as const,
    byProvider: (id: number) => [...queryKeys.auditLogs.all, 'provider', id] as const,
  },

  // Encryption
  encryption: {
    all: ['encryption'] as const,
    status: () => [...queryKeys.encryption.all, 'status'] as const,
    history: () => [...queryKeys.encryption.all, 'history'] as const,
  },

  // Detection
  detection: {
    all: ['detection'] as const,
    result: (domain: string) => [...queryKeys.detection.all, domain] as const,
  },

  // Plugins
  plugins: {
    all: ['plugins'] as const,
    list: () => [...queryKeys.plugins.all, 'list'] as const,
    detail: (type: string) => [...queryKeys.plugins.all, 'detail', type] as const,
  },
}
```

---

## Risk Assessment

### Risk Matrix

| Feature | Risk Level | Main Risks | Mitigation |
|---------|------------|------------|------------|
| **Audit Logging** | Low | Log volume growth | Retention policy, indexed queries, async logging |
| **Key Rotation** | Medium | Data loss, downtime | Backup before rotation, rollback procedure, staged rollout |
| **Multi-Credential** | Medium | Zone mismatch, credential isolation | Thorough zone matching tests, fallback to catch-all |
| **Auto-Detection** | Low | False positives | Manual override, confidence scoring |
| **Custom Plugins** | High | Code execution, security | Signature verification, allowlist, security review |

### Breaking Changes Assessment

| Feature | Breaking Changes | Backward Compatibility |
|---------|------------------|------------------------|
| Audit Logging | None | Full - additive only |
| Key Rotation | None | Full - transparent to API consumers |
| Multi-Credential | None | Full - `use_multi_credentials` defaults to false |
| Auto-Detection | None | Full - opt-in feature |
| Custom Plugins | None | Full - built-in providers continue working |

### Rollback Procedures

1. **Audit Logging:** Drop new columns from `security_audits` table
2. **Key Rotation:** Keep old `CHARON_ENCRYPTION_KEY` configured, remove `_NEXT` and `_V*` vars
3. **Multi-Credential:** Set `use_multi_credentials=false` for all providers
4. **Auto-Detection:** Disable in frontend, remove detection routes
5. **Custom Plugins:** Disable all plugins via allowlist, remove plugin directory

---

## Testing Strategy

### Coverage Requirements

All new code must achieve ≥85% test coverage per project requirements.

### Test Categories

| Category | Tools | Focus Areas |
|----------|-------|-------------|
| Unit Tests (Backend) | `go test` | Services, handlers, crypto |
| Unit Tests (Frontend) | Jest, React Testing Library | Hooks, components |
| Integration Tests | `go test -tags=integration` | Database operations, API endpoints |
| E2E Tests | Playwright | Full user flows |

### Test Files Required

| Feature | Backend Tests | Frontend Tests |
|---------|---------------|----------------|
| Audit Logging | `audit_log_handler_test.go`, `security_service_test.go` (extend) | `AuditLogs.test.tsx` |
| Key Rotation | `rotation_service_test.go`, `encryption_handler_test.go` | `EncryptionManagement.test.tsx` |
| Multi-Credential | `credential_service_test.go`, `credential_handler_test.go` | `CredentialManager.test.tsx` |
| Auto-Detection | `dns_detection_service_test.go`, `dns_detection_handler_test.go` | `useDNSDetection.test.ts` |
| Custom Plugins | `plugin_loader_test.go`, `plugin_handler_test.go` | `PluginManagement.test.tsx` |

### Test Commands

```bash
# Backend tests
cd backend && go test ./... -cover -coverprofile=coverage.out

# Check coverage percentage
go tool cover -func=coverage.out | grep total

# Frontend tests
cd frontend && npm test -- --coverage

# Run specific test file
cd backend && go test -v ./internal/services/audit_log_service_test.go
```

---

## Appendix: Full File Tree

```
backend/
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── audit_log_handler.go          # NEW (Feature 1)
│   │   │   ├── audit_log_handler_test.go     # NEW (Feature 1)
│   │   │   ├── credential_handler.go         # NEW (Feature 3)
│   │   │   ├── credential_handler_test.go    # NEW (Feature 3)
│   │   │   ├── dns_detection_handler.go      # NEW (Feature 4)
│   │   │   ├── dns_detection_handler_test.go # NEW (Feature 4)
│   │   │   ├── dns_provider_handler.go       # EXISTING
│   │   │   ├── encryption_handler.go         # NEW (Feature 2)
│   │   │   ├── encryption_handler_test.go    # NEW (Feature 2)
│   │   │   └── plugin_handler.go             # NEW (Feature 5)
│   │   └── routes/
│   │       └── routes.go                     # MODIFY
│   ├── crypto/
│   │   ├── encryption.go                     # MODIFY (Feature 2)
│   │   ├── encryption_test.go                # EXISTING
│   │   ├── rotation_service.go               # NEW (Feature 2)
│   │   └── rotation_service_test.go          # NEW (Feature 2)
│   ├── models/
│   │   ├── dns_provider.go                   # MODIFY (Features 2, 3)
│   │   ├── dns_provider_credential.go        # NEW (Feature 3)
│   │   └── security_audit.go                 # MODIFY (Feature 1)
│   ├── services/
│   │   ├── audit_helper.go                   # NEW (shared, async buffered channel)
│   │   ├── credential_service.go             # NEW (Feature 3)
│   │   ├── credential_service_test.go        # NEW (Feature 3)
│   │   ├── dns_detection_service.go          # NEW (Feature 4)
│   │   ├── dns_detection_service_test.go     # NEW (Feature 4)
│   │   ├── dns_provider_service.go           # MODIFY (Features 1, 2, 3)
│   │   ├── plugin_loader.go                  # NEW (Feature 5)
│   │   ├── plugin_loader_test.go             # NEW (Feature 5)
│   │   └── security_service.go               # EXTEND (Feature 1: add ListAuditLogs)
├── pkg/
│   └── dnsprovider/
│       ├── interface.go                      # NEW (Feature 5)
│       └── builtin.go                        # NEW (Feature 5)
└── plugins/
    └── powerdns/
        ├── powerdns_plugin.go                # NEW (Feature 5)
        └── README.md                         # NEW (Feature 5)

frontend/
├── src/
│   ├── api/
│   │   ├── auditLogs.ts                      # NEW (Feature 1)
│   │   ├── credentials.ts                    # NEW (Feature 3)
│   │   ├── dnsDetection.ts                   # NEW (Feature 4)
│   │   ├── dnsProviders.ts                   # EXISTING
│   │   ├── encryption.ts                     # NEW (Feature 2)
│   │   └── plugins.ts                        # NEW (Feature 5)
│   ├── components/
│   │   ├── CredentialManager.tsx             # NEW (Feature 3)
│   │   ├── DataTable.tsx                     # NEW (shared)
│   │   └── __tests__/
│   │       └── CredentialManager.test.tsx    # NEW (Feature 3)
│   ├── hooks/
│   │   ├── queryKeys.ts                      # NEW (shared)
│   │   ├── useAuditLogs.ts                   # NEW (Feature 1)
│   │   ├── useCredentials.ts                 # NEW (Feature 3)
│   │   ├── useDNSDetection.ts                # NEW (Feature 4)
│   │   ├── useDNSProviders.ts                # EXISTING
│   │   ├── useEncryption.ts                  # NEW (Feature 2)
│   │   └── usePlugins.ts                     # NEW (Feature 5)
│   └── pages/
│       ├── AuditLogs.tsx                     # NEW (Feature 1)
│       ├── DNSProviders.tsx                  # MODIFY (Feature 3)
│       ├── EncryptionManagement.tsx          # NEW (Feature 2)
│       ├── PluginManagement.tsx              # NEW (Feature 5)
│       ├── ProxyHosts.tsx                    # MODIFY (Feature 4)
│       └── __tests__/
│           ├── AuditLogs.test.tsx            # NEW (Feature 1)
│           ├── EncryptionManagement.test.tsx # NEW (Feature 2)
│           └── PluginManagement.test.tsx     # NEW (Feature 5)

docs/
├── development/
│   ├── dns-plugins.md                        # NEW (Feature 5)
│   └── plugin-security.md                    # NEW (Feature 5)
└── plans/
    ├── dns_challenge_future_features.md      # EXISTING (source spec)
    └── dns_future_features_implementation.md # THIS FILE
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | January 3, 2026 | Planning Agent | Initial comprehensive implementation plan |
| 1.1.0 | January 3, 2026 | Planning Agent | Applied supervisor corrections: GORM AutoMigrate pattern, removed encryption_keys table, added Linux/macOS plugin limitation, consolidated AuditLogService into SecurityService, added router integration, specified buffered channel strategy |

---

**Status:** Ready for implementation. Begin with Phase 1 (Audit Logging).
