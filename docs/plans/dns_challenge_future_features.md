# DNS Challenge Future Features - Planning Document

**Issue:** #21 Follow-up - Future DNS Challenge Enhancements
**Status:** Planning Phase
**Version:** 1.0
**Date:** January 2, 2026

---

## Executive Summary

This document outlines the implementation plan for 5 future enhancements to Charon's DNS Challenge Support feature (Issue #21). These features were intentionally deferred from the initial MVP to accelerate beta release, but represent significant value-adds for production deployments requiring enterprise-grade security, multi-tenancy, and extensibility.

### Features Overview

| Feature | Business Value | User Demand | Complexity | Priority |
|---------|---------------|-------------|------------|----------|
| **Audit Logging** | High (Compliance) | High | Low | **P0 - Critical** |
| **Multi-Credential per Provider** | Medium | Medium | Medium | P1 |
| **Key Rotation Automation** | High (Security) | Low | High | P1 |
| **DNS Provider Auto-Detection** | Low | Medium | Medium | P2 |
| **Custom DNS Provider Plugins** | Low | Low | Very High | P3 |

**Recommended Implementation Order:**

1. Audit Logging (Security/Compliance baseline)
2. Key Rotation (Security hardening)
3. Multi-Credential (Advanced use cases)
4. Auto-Detection (UX improvement)
5. Custom Plugins (Extensibility for power users)

---

## 1. Audit Logging for Credential Operations

### 1.1 Business Case

**Problem:** Currently, there is no record of who accessed, modified, or used DNS provider credentials. This creates security blind spots and prevents forensic analysis of credential misuse or breach attempts.

**Impact:**

- **Compliance Risk:** SOC 2, GDPR, HIPAA all require audit trails for sensitive data access
- **Security Risk:** No ability to detect credential theft or unauthorized changes
- **Operational Risk:** Cannot diagnose certificate issuance failures retrospectively

**User Stories:**

- As a security auditor, I need to see all credential access events for compliance reporting
- As an administrator, I want alerts when credentials are accessed outside business hours
- As a developer, I need audit logs to debug failed certificate issuances

### 1.2 Technical Design

#### Database Schema

**Extend Existing `security_audits` Table:**

```sql
-- File: backend/internal/models/security_audit.go (extend existing)

ALTER TABLE security_audits ADD COLUMN event_category TEXT; -- 'dns_provider', 'certificate', etc.
ALTER TABLE security_audits ADD COLUMN resource_id INTEGER;  -- DNSProvider.ID
ALTER TABLE security_audits ADD COLUMN resource_uuid TEXT;  -- DNSProvider.UUID
ALTER TABLE security_audits ADD COLUMN ip_address TEXT;     -- Request originator IP
ALTER TABLE security_audits ADD COLUMN user_agent TEXT;     -- Browser/API client
```

**Model Extension:**

```go
type SecurityAudit struct {
    ID             uint      `json:"id" gorm:"primaryKey"`
    UUID           string    `json:"uuid" gorm:"uniqueIndex"`
    Actor          string    `json:"actor"`                    // User ID or "system"
    Action         string    `json:"action"`                   // "dns_provider_create", "credential_decrypt", etc.
    EventCategory  string    `json:"event_category"`           // "dns_provider"
    ResourceID     *uint     `json:"resource_id,omitempty"`    // DNSProvider.ID
    ResourceUUID   string    `json:"resource_uuid,omitempty"`  // DNSProvider.UUID
    Details        string    `json:"details" gorm:"type:text"` // JSON blob with event metadata
    IPAddress      string    `json:"ip_address"`               // Request IP
    UserAgent      string    `json:"user_agent"`               // Client identifier
    CreatedAt      time.Time `json:"created_at"`
}
```

#### Events to Log

| Event | Trigger | Details Captured |
|-------|---------|------------------|
| `dns_provider_create` | POST /api/v1/dns-providers | Provider name, type, is_default |
| `dns_provider_update` | PUT /api/v1/dns-providers/:id | Changed fields, old_value, new_value |
| `dns_provider_delete` | DELETE /api/v1/dns-providers/:id | Provider name, type, had_credentials |
| `credential_test` | POST /api/v1/dns-providers/:id/test | Provider name, test_result, error |
| `credential_decrypt` | Caddy config generation | Provider name, purpose ("certificate_issuance") |
| `certificate_issued` | Caddy webhook/polling | Domain, provider used, success/failure |
| `credential_export` | Future: Backup/export feature | Provider name, export_format |

#### Audit Service Integration

**File: `backend/internal/services/dns_provider_service.go`**

```go
// Add audit logging to all CRUD operations
func (s *dnsProviderService) Create(ctx context.Context, req CreateDNSProviderRequest) (*models.DNSProvider, error) {
    // ... existing create logic ...

    // Log audit event
    audit := &models.SecurityAudit{
        Actor:         getUserIDFromContext(ctx),
        Action:        "dns_provider_create",
        EventCategory: "dns_provider",
        ResourceID:    &provider.ID,
        ResourceUUID:  provider.UUID,
        Details:       fmt.Sprintf(`{"name":"%s","type":"%s","is_default":%t}`, provider.Name, provider.ProviderType, provider.IsDefault),
        IPAddress:     getIPFromContext(ctx),
        UserAgent:     getUserAgentFromContext(ctx),
    }
    s.securityService.LogAudit(audit) // Non-blocking, errors logged but not returned

    return provider, nil
}
```

**File: `backend/internal/caddy/manager.go`**

```go
// Log credential decryption for Caddy config generation
for _, provider := range dnsProviders {
    decryptedData, err := encryptor.Decrypt(provider.CredentialsEncrypted)
    if err != nil {
        continue
    }

    // Log audit event (system actor)
    audit := &models.SecurityAudit{
        Actor:         "system",
        Action:        "credential_decrypt",
        EventCategory: "dns_provider",
        ResourceID:    &provider.ID,
        ResourceUUID:  provider.UUID,
        Details:       fmt.Sprintf(`{"purpose":"certificate_issuance","success":true}`),
    }
    securityService.LogAudit(audit)
}
```

### 1.3 Frontend UI

**New Page: `/security/audit-logs`**

- **Table View:**
  - Columns: Timestamp, Actor, Action, Resource, IP Address, Details
  - Filters: Date range, Event category, Actor, Action type
  - Search: Free-text search in Details field
  - Export: Download as CSV or JSON

- **Details Modal:**
  - Full event JSON
  - Related events (same resource_uuid)
  - Timeline visualization

**Integration:**

- Add "Audit Logs" link to Security page
- Add "View Audit History" button to DNS Provider edit form

### 1.4 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/audit-logs` | List audit logs (paginated, filterable) |
| `GET` | `/api/v1/audit-logs/:uuid` | Get single audit event |
| `GET` | `/api/v1/dns-providers/:id/audit-logs` | Get audit history for specific provider |

### 1.5 Implementation Checklist

- [ ] Extend SecurityAudit model with new fields
- [ ] Run database migration (add columns to security_audits table)
- [ ] Add audit logging to DNSProviderService CRUD operations
- [ ] Add audit logging to Caddy Manager credential decryption
- [ ] Create AuditLogService with filtering and pagination
- [ ] Create AuditLogHandler with REST endpoints
- [ ] Register audit log routes in routes.go
- [ ] Create frontend AuditLogs page with table and filters
- [ ] Add audit log API client functions
- [ ] Create React Query hooks for audit logs
- [ ] Add translations for audit log UI
- [ ] Write unit tests for audit logging (backend: 85% coverage)
- [ ] Write unit tests for audit log UI (frontend: 85% coverage)
- [ ] Update documentation with audit log usage
- [ ] Add retention policy configuration (e.g., 90 days)

### 1.6 Performance Considerations

**Audit Log Growth:** Audit logs can grow rapidly. Implement:

- **Automatic Cleanup:** Background job to delete logs older than retention period (default: 90 days, configurable)
- **Indexed Queries:** Add database indexes on `created_at`, `event_category`, `resource_uuid`, `actor`
- **Async Logging:** Audit logging must not block API requests (use buffered channel + goroutine)

**Estimated Implementation Time:** 8-12 hours

---

## 2. Multi-Credential per Provider (Zone-Specific Credentials)

### 2.1 Business Case

**Problem:** Large organizations manage multiple DNS zones (e.g., example.com, example.org, customers.example.com) with different API tokens for security isolation. Currently, Charon only supports one credential set per provider.

**Impact:**

- **Security:** Overly broad API tokens violate least privilege principle
- **Multi-Tenancy:** Cannot isolate customer zones with separate credentials
- **Operational Risk:** Credential compromise affects all zones

**User Stories:**

- As a managed service provider, I need separate API tokens for each customer's DNS zone
- As a security engineer, I want to rotate credentials for specific zones without affecting others
- As an administrator, I need zone-level access control for different teams

### 2.2 Technical Design

#### Database Schema Changes

**New Table: `dns_provider_credentials`**

```sql
CREATE TABLE dns_provider_credentials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT UNIQUE NOT NULL,
    dns_provider_id INTEGER NOT NULL,
    label TEXT NOT NULL,                     -- "Production Zone", "Customer ABC"
    zone_filter TEXT,                        -- "example.com,*.example.com" (comma-separated domains)
    credentials_encrypted TEXT NOT NULL,     -- AES-256-GCM encrypted JSON blob
    enabled BOOLEAN DEFAULT 1,
    propagation_timeout INTEGER DEFAULT 120,
    polling_interval INTEGER DEFAULT 5,
    last_used_at DATETIME,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (dns_provider_id) REFERENCES dns_providers(id) ON DELETE CASCADE
);

CREATE INDEX idx_dns_creds_provider ON dns_provider_credentials(dns_provider_id);
CREATE INDEX idx_dns_creds_zone ON dns_provider_credentials(zone_filter);
```

**Updated `dns_providers` Table:**

```sql
-- Add flag to indicate if provider uses multi-credentials
ALTER TABLE dns_providers ADD COLUMN use_multi_credentials BOOLEAN DEFAULT 0;

-- Keep existing credentials_encrypted for backward compatibility (default credential)
```

#### Model Changes

**New Model: `DNSProviderCredential`**

```go
// File: backend/internal/models/dns_provider_credential.go

type DNSProviderCredential struct {
    ID                   uint       `json:"id" gorm:"primaryKey"`
    UUID                 string     `json:"uuid" gorm:"uniqueIndex;size:36"`
    DNSProviderID        uint       `json:"dns_provider_id" gorm:"index;not null"`
    DNSProvider          *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`

    Label                string     `json:"label" gorm:"not null;size:255"`
    ZoneFilter           string     `json:"zone_filter" gorm:"type:text"` // Comma-separated domains
    CredentialsEncrypted string     `json:"-" gorm:"type:text;not null;column:credentials_encrypted"`
    Enabled              bool       `json:"enabled" gorm:"default:true"`

    PropagationTimeout   int        `json:"propagation_timeout" gorm:"default:120"`
    PollingInterval      int        `json:"polling_interval" gorm:"default:5"`

    LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
    SuccessCount         int        `json:"success_count" gorm:"default:0"`
    FailureCount         int        `json:"failure_count" gorm:"default:0"`
    LastError            string     `json:"last_error,omitempty" gorm:"type:text"`

    CreatedAt            time.Time  `json:"created_at"`
    UpdatedAt            time.Time  `json:"updated_at"`
}

func (DNSProviderCredential) TableName() string {
    return "dns_provider_credentials"
}
```

**Updated `DNSProvider` Model:**

```go
type DNSProvider struct {
    // ... existing fields ...
    UseMultiCredentials  bool                     `json:"use_multi_credentials" gorm:"default:false"`
    Credentials          []DNSProviderCredential  `json:"credentials,omitempty" gorm:"foreignKey:DNSProviderID"`
}
```

#### Zone Matching Logic

**File: `backend/internal/services/dns_provider_service.go`**

```go
// GetCredentialForDomain selects the best credential match for a domain
func (s *dnsProviderService) GetCredentialForDomain(ctx context.Context, providerID uint, domain string) (*models.DNSProviderCredential, error) {
    var provider models.DNSProvider
    if err := s.db.Preload("Credentials").First(&provider, providerID).Error; err != nil {
        return nil, err
    }

    // If not using multi-credentials, return default
    if !provider.UseMultiCredentials || len(provider.Credentials) == 0 {
        return s.getDefaultCredential(&provider)
    }

    // Find best match: exact domain > wildcard > default
    var bestMatch *models.DNSProviderCredential
    for _, cred := range provider.Credentials {
        if !cred.Enabled {
            continue
        }

        zones := strings.Split(cred.ZoneFilter, ",")
        for _, zone := range zones {
            zone = strings.TrimSpace(zone)

            // Exact match
            if zone == domain {
                return &cred, nil
            }

            // Wildcard match (*.example.com matches app.example.com)
            if strings.HasPrefix(zone, "*.") {
                baseDomain := zone[2:] // Remove "*."
                if strings.HasSuffix(domain, "."+baseDomain) || domain == baseDomain {
                    bestMatch = &cred
                }
            }
        }
    }

    if bestMatch != nil {
        return bestMatch, nil
    }

    // Fallback to credential with empty zone_filter (catch-all)
    for _, cred := range provider.Credentials {
        if cred.Enabled && cred.ZoneFilter == "" {
            return &cred, nil
        }
    }

    return nil, fmt.Errorf("no credential found for domain %s", domain)
}
```

### 2.3 API Changes

**New Endpoints:**

```
POST   /api/v1/dns-providers/:id/credentials          # Create credential
GET    /api/v1/dns-providers/:id/credentials          # List credentials
GET    /api/v1/dns-providers/:id/credentials/:cred_id # Get credential
PUT    /api/v1/dns-providers/:id/credentials/:cred_id # Update credential
DELETE /api/v1/dns-providers/:id/credentials/:cred_id # Delete credential
POST   /api/v1/dns-providers/:id/credentials/:cred_id/test # Test credential
```

**Updated Endpoints:**

```
PUT /api/v1/dns-providers/:id
  # Add field: "use_multi_credentials": true
```

### 2.4 Frontend UI

**DNS Provider Form Changes:**

- Add toggle: "Use Multiple Credentials (Advanced)"
- When enabled:
  - Show "Manage Credentials" button → opens modal
  - Modal displays table of credentials with zone filters
  - Add/Edit credential with zone filter input (comma-separated domains)
  - Test button for each credential

**Credential Management Modal:**

```
┌───────────────────────────────────────────────────────────┐
│ Manage Credentials: Cloudflare Production                 │
├───────────────────────────────────────────────────────────┤
│ ┌───────────────────────────────────────────────────────┐ │
│ │ Label              │ Zones           │ Status │ Action│ │
│ ├───────────────────────────────────────────────────────┤ │
│ │ Main Zone          │ example.com     │ ✅ OK  │ [Edit]│ │
│ │ Customer A         │ *.customer-a... │ ✅ OK  │ [Edit]│ │
│ │ Staging            │ *.staging.exam..│ ⚠️ Warn│ [Edit]│ │
│ └───────────────────────────────────────────────────────┘ │
│                                       [+ Add Credential]   │
└───────────────────────────────────────────────────────────┘
```

### 2.5 Migration Strategy

**Backward Compatibility:**

- Existing providers continue using `credentials_encrypted` field (default credential)
- New field `use_multi_credentials` defaults to `false`
- When toggled on, existing credential is migrated to first `dns_provider_credentials` row with empty `zone_filter`

**Migration Code:**

```go
// backend/internal/services/dns_provider_service.go

func (s *dnsProviderService) EnableMultiCredentials(ctx context.Context, providerID uint) error {
    provider, err := s.Get(ctx, providerID)
    if err != nil {
        return err
    }

    // Migrate existing credential to multi-credential table
    if provider.CredentialsEncrypted != "" {
        migrated := &models.DNSProviderCredential{
            UUID:                 uuid.NewString(),
            DNSProviderID:        provider.ID,
            Label:                "Default (migrated)",
            ZoneFilter:           "", // Catch-all
            CredentialsEncrypted: provider.CredentialsEncrypted,
            Enabled:              true,
            PropagationTimeout:   provider.PropagationTimeout,
            PollingInterval:      provider.PollingInterval,
        }
        if err := s.db.Create(migrated).Error; err != nil {
            return err
        }
    }

    provider.UseMultiCredentials = true
    return s.db.Save(provider).Error
}
```

### 2.6 Implementation Checklist

- [ ] Create DNSProviderCredential model
- [ ] Add migration for dns_provider_credentials table
- [ ] Update DNSProvider model with UseMultiCredentials flag
- [ ] Implement GetCredentialForDomain zone matching logic
- [ ] Create CredentialService for CRUD operations
- [ ] Create CredentialHandler with REST endpoints
- [ ] Register credential routes in routes.go
- [ ] Update Caddy Manager to use zone-specific credentials
- [ ] Create frontend CredentialManager modal component
- [ ] Update DNSProviderForm with multi-credential toggle
- [ ] Add credential management API client functions
- [ ] Write unit tests for zone matching logic (85% coverage)
- [ ] Write unit tests for credential UI (85% coverage)
- [ ] Update documentation with multi-credential usage
- [ ] Add migration tool for existing providers

**Estimated Implementation Time:** 12-16 hours

---

## 3. Key Rotation Automation

### 3.1 Business Case

**Problem:** Changing `CHARON_ENCRYPTION_KEY` currently requires manual re-encryption of all DNS provider credentials and system downtime. This prevents regular key rotation, a critical security practice.

**Impact:**

- **Security Risk:** Key compromise affects all historical and current credentials
- **Compliance Risk:** Many security frameworks require periodic key rotation (e.g., PCI-DSS: every 12 months)
- **Operational Risk:** Key loss results in complete data loss (no recovery)

**User Stories:**

- As a security engineer, I need to rotate encryption keys annually without downtime
- As an administrator, I want to schedule key rotation during maintenance windows
- As a compliance officer, I need proof of key rotation for audit reports

### 3.2 Technical Design

#### Key Versioning Architecture

**Concept:** Support multiple encryption keys simultaneously with versioning

**Database Changes:**

```sql
-- Track active encryption key versions
CREATE TABLE encryption_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version INTEGER UNIQUE NOT NULL,        -- Monotonically increasing
    key_hash TEXT UNIQUE NOT NULL,          -- SHA-256 hash of the key (for identification, not storage)
    status TEXT NOT NULL,                   -- 'active', 'rotating', 'deprecated', 'retired'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    rotated_at DATETIME,
    retired_at DATETIME
);

-- Add key_version to encrypted data
ALTER TABLE dns_providers ADD COLUMN key_version INTEGER DEFAULT 1;
ALTER TABLE dns_provider_credentials ADD COLUMN key_version INTEGER DEFAULT 1;

CREATE INDEX idx_dns_providers_key_version ON dns_providers(key_version);
CREATE INDEX idx_dns_creds_key_version ON dns_provider_credentials(key_version);
```

#### Environment Variable Naming Convention

```bash
# Primary key (current)
CHARON_ENCRYPTION_KEY=<base64-key>

# Rotation: Old key (for decryption only)
CHARON_ENCRYPTION_KEY_V1=<old-base64-key>
CHARON_ENCRYPTION_KEY_V2=<older-base64-key>

# New key (for encryption)
CHARON_ENCRYPTION_KEY_NEXT=<new-base64-key>
```

#### Rotation Service

**File: `backend/internal/crypto/rotation_service.go`**

```go
type RotationService struct {
    db             *gorm.DB
    currentKey     *crypto.EncryptionService
    nextKey        *crypto.EncryptionService
    legacyKeys     map[int]*crypto.EncryptionService
    currentVersion int
}

func NewRotationService(db *gorm.DB) (*RotationService, error) {
    rs := &RotationService{
        db:         db,
        legacyKeys: make(map[int]*crypto.EncryptionService),
    }

    // Load current key
    currentKeyB64 := os.Getenv("CHARON_ENCRYPTION_KEY")
    if currentKeyB64 == "" {
        return nil, errors.New("CHARON_ENCRYPTION_KEY not set")
    }
    currentKey, err := crypto.NewEncryptionService(currentKeyB64)
    if err != nil {
        return nil, err
    }
    rs.currentKey = currentKey
    rs.currentVersion = 1 // Default version

    // Load legacy keys (V1, V2, etc.)
    for i := 1; i <= 10; i++ {
        keyEnvVar := fmt.Sprintf("CHARON_ENCRYPTION_KEY_V%d", i)
        if keyB64 := os.Getenv(keyEnvVar); keyB64 != "" {
            legacyKey, err := crypto.NewEncryptionService(keyB64)
            if err != nil {
                logger.Log().WithError(err).Warnf("Failed to load legacy key V%d", i)
                continue
            }
            rs.legacyKeys[i] = legacyKey
        }
    }

    // Load next key (for encryption during rotation)
    if nextKeyB64 := os.Getenv("CHARON_ENCRYPTION_KEY_NEXT"); nextKeyB64 != "" {
        nextKey, err := crypto.NewEncryptionService(nextKeyB64)
        if err != nil {
            logger.Log().WithError(err).Warn("Failed to load next encryption key")
        } else {
            rs.nextKey = nextKey
            rs.currentVersion += 1
        }
    }

    return rs, nil
}

// DecryptWithVersion decrypts data using the appropriate key version
func (rs *RotationService) DecryptWithVersion(ciphertextB64 string, version int) ([]byte, error) {
    if version == rs.currentVersion {
        return rs.currentKey.Decrypt(ciphertextB64)
    }

    if legacyKey, ok := rs.legacyKeys[version]; ok {
        return legacyKey.Decrypt(ciphertextB64)
    }

    return nil, fmt.Errorf("no key available for version %d", version)
}

// EncryptWithCurrentKey always uses the current (or next) key version
func (rs *RotationService) EncryptWithCurrentKey(plaintext []byte) (string, int, error) {
    keyToUse := rs.currentKey
    versionToUse := rs.currentVersion

    if rs.nextKey != nil {
        // During rotation, use next key for new encryptions
        keyToUse = rs.nextKey
        versionToUse = rs.currentVersion + 1
    }

    ciphertext, err := keyToUse.Encrypt(plaintext)
    return ciphertext, versionToUse, err
}

// RotateAllCredentials re-encrypts all DNS provider credentials with the next key
func (rs *RotationService) RotateAllCredentials(ctx context.Context) error {
    if rs.nextKey == nil {
        return errors.New("CHARON_ENCRYPTION_KEY_NEXT not set")
    }

    logger.Log().Info("Starting credential re-encryption with new key")

    // Fetch all providers
    var providers []models.DNSProvider
    if err := rs.db.Find(&providers).Error; err != nil {
        return err
    }

    successCount := 0
    errorCount := 0

    for _, provider := range providers {
        if provider.CredentialsEncrypted == "" {
            continue
        }

        // Decrypt with old key
        oldPlaintext, err := rs.DecryptWithVersion(provider.CredentialsEncrypted, provider.KeyVersion)
        if err != nil {
            logger.Log().WithError(err).Errorf("Failed to decrypt provider %d credentials", provider.ID)
            errorCount++
            continue
        }

        // Re-encrypt with new key
        newCiphertext, newVersion, err := rs.EncryptWithCurrentKey(oldPlaintext)
        if err != nil {
            logger.Log().WithError(err).Errorf("Failed to re-encrypt provider %d credentials", provider.ID)
            errorCount++
            continue
        }

        // Update database
        provider.CredentialsEncrypted = newCiphertext
        provider.KeyVersion = newVersion
        if err := rs.db.Save(&provider).Error; err != nil {
            logger.Log().WithError(err).Errorf("Failed to save provider %d with new credentials", provider.ID)
            errorCount++
            continue
        }

        successCount++
    }

    logger.Log().WithFields(map[string]interface{}{
        "success": successCount,
        "errors":  errorCount,
    }).Info("Credential re-encryption complete")

    if errorCount > 0 {
        return fmt.Errorf("rotation completed with %d errors", errorCount)
    }

    return nil
}
```

### 3.3 Rotation Workflow

**Step 1: Prepare New Key**

```bash
# Generate new key
openssl rand -base64 32

# Set as NEXT key (keep old key active)
export CHARON_ENCRYPTION_KEY_NEXT="<new-base64-key>"
```

**Step 2: Trigger Rotation**

```bash
# Via API (admin only)
curl -X POST https://charon.example.com/api/v1/admin/encryption/rotate \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Or via CLI tool (future)
charon-cli encryption rotate
```

**Step 3: Verify Re-encryption**

```bash
# Check rotation status
curl https://charon.example.com/api/v1/admin/encryption/status

# Response:
{
  "current_version": 2,
  "providers_rotated": 15,
  "providers_pending": 0,
  "rotation_status": "complete"
}
```

**Step 4: Promote New Key**

```bash
# Move old key to legacy
export CHARON_ENCRYPTION_KEY_V1="$CHARON_ENCRYPTION_KEY"

# Promote new key to current
export CHARON_ENCRYPTION_KEY="$CHARON_ENCRYPTION_KEY_NEXT"
unset CHARON_ENCRYPTION_KEY_NEXT

# Restart Charon (zero downtime - gradual pod replacement)
```

**Step 5: Retire Old Key (after grace period)**

```bash
# After 30 days, remove legacy key
unset CHARON_ENCRYPTION_KEY_V1
```

### 3.4 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/admin/encryption/status` | Current key version, rotation status |
| `POST` | `/api/v1/admin/encryption/rotate` | Trigger credential re-encryption |
| `GET` | `/api/v1/admin/encryption/history` | Key rotation audit log |

### 3.5 Implementation Checklist

- [ ] Create encryption_keys table
- [ ] Add key_version columns to dns_providers and dns_provider_credentials
- [ ] Create RotationService with multi-key support
- [ ] Implement DecryptWithVersion fallback logic
- [ ] Implement RotateAllCredentials background job
- [ ] Create admin encryption endpoints (status, rotate, history)
- [ ] Add rotation progress tracking (% complete)
- [ ] Create frontend admin page for key management
- [ ] Add monitoring alerts for rotation failures
- [ ] Write unit tests for rotation logic (85% coverage)
- [ ] Document rotation procedure in operations guide
- [ ] Add rollback procedure for failed rotations
- [ ] Implement automatic key version detection
- [ ] Create CLI tool for key rotation (optional)

**Estimated Implementation Time:** 16-20 hours

---

## 4. DNS Provider Auto-Detection

### 4.1 Business Case

**Problem:** Users must manually select DNS provider when creating wildcard proxy hosts. Many users don't know which DNS provider manages their domain's nameservers.

**Impact:**

- **UX Friction:** Users waste time checking DNS registrar/provider
- **Configuration Errors:** Selecting wrong provider causes certificate failures
- **Support Burden:** Common support question: "Which provider do I use?"

**User Stories:**

- As a user, I want Charon to automatically suggest the correct DNS provider for my domain
- As a support engineer, I want to reduce configuration errors from wrong provider selection
- As a developer, I want auto-detection to work even with custom nameservers

### 4.2 Technical Design

#### Nameserver Detection Service

**File: `backend/internal/services/dns_detection_service.go`**

```go
type DNSDetectionService struct {
    db             *gorm.DB
    nameserverDB   map[string]string // Nameserver pattern → provider_type
    cache          *cache.Cache      // Domain → detected provider (TTL: 1 hour)
}

// Nameserver pattern database (built-in)
var BuiltInNameservers = map[string]string{
    // Cloudflare
    ".ns.cloudflare.com":   "cloudflare",

    // AWS Route 53
    ".awsdns":              "route53",

    // DigitalOcean
    ".digitalocean.com":    "digitalocean",

    // Google Cloud DNS
    ".googledomains.com":   "googleclouddns",
    "ns-cloud":             "googleclouddns",

    // Azure DNS
    ".azure-dns":           "azure",

    // Namecheap
    ".registrar-servers.com": "namecheap",

    // GoDaddy
    ".domaincontrol.com":   "godaddy",

    // Hetzner
    ".hetzner.com":         "hetzner",
    ".hetzner.de":          "hetzner",

    // Vultr
    ".vultr.com":           "vultr",

    // DNSimple
    ".dnsimple.com":        "dnsimple",
}

func (s *DNSDetectionService) DetectProvider(domain string) (*DetectionResult, error) {
    // Check cache first
    if cached, found := s.cache.Get(domain); found {
        return cached.(*DetectionResult), nil
    }

    // Query nameservers for domain
    nameservers, err := net.LookupNS(domain)
    if err != nil {
        return &DetectionResult{
            Domain:     domain,
            Detected:   false,
            Error:      err.Error(),
        }, err
    }

    // Match nameservers against known patterns
    for _, ns := range nameservers {
        nsHost := strings.ToLower(ns.Host)
        for pattern, providerType := range s.nameserverDB {
            if strings.Contains(nsHost, pattern) {
                result := &DetectionResult{
                    Domain:       domain,
                    Detected:     true,
                    ProviderType: providerType,
                    Nameservers:  extractNSHosts(nameservers),
                    Confidence:   "high",
                }
                s.cache.Set(domain, result, 1*time.Hour)
                return result, nil
            }
        }
    }

    // No match found
    result := &DetectionResult{
        Domain:      domain,
        Detected:    false,
        Nameservers: extractNSHosts(nameservers),
        Confidence:  "none",
    }
    return result, nil
}

// SuggestConfiguredProvider checks if user has a provider configured matching detected type
func (s *DNSDetectionService) SuggestConfiguredProvider(ctx context.Context, domain string) (*models.DNSProvider, error) {
    detection, err := s.DetectProvider(domain)
    if err != nil || !detection.Detected {
        return nil, nil
    }

    // Find enabled provider matching detected type
    var provider models.DNSProvider
    err = s.db.Where("provider_type = ? AND enabled = ?", detection.ProviderType, true).First(&provider).Error
    if err != nil {
        return nil, nil // No matching provider configured
    }

    return &provider, nil
}

type DetectionResult struct {
    Domain       string   `json:"domain"`
    Detected     bool     `json:"detected"`
    ProviderType string   `json:"provider_type,omitempty"`
    Nameservers  []string `json:"nameservers"`
    Confidence   string   `json:"confidence"` // "high", "medium", "low", "none"
    Error        string   `json:"error,omitempty"`
}
```

### 4.3 API Integration

**New Endpoint:**

```
POST /api/v1/dns-providers/detect
{
  "domain": "example.com"
}

Response:
{
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

### 4.4 Frontend Integration

**ProxyHostForm.tsx Enhancement:**

```tsx
// When user types a wildcard domain, trigger auto-detection
const [detectionResult, setDetectionResult] = useState<DetectionResult | null>(null)

useEffect(() => {
  if (hasWildcardDomain && formData.domain_names) {
    const domain = formData.domain_names.split(',')[0].trim().replace(/^\*\./, '')
    detectDNSProvider(domain).then(result => {
      setDetectionResult(result)
      if (result.suggested_provider) {
        setFormData(prev => ({
          ...prev,
          dns_provider_id: result.suggested_provider.id
        }))
        toast.info(`Auto-detected: ${result.suggested_provider.name}`)
      }
    })
  }
}, [formData.domain_names, hasWildcardDomain])

// UI: Show detection result
{detectionResult && detectionResult.detected && (
  <Alert variant="info">
    <Info className="h-4 w-4" />
    <AlertDescription>
      Detected DNS provider: <strong>{detectionResult.provider_type}</strong>
      <br />
      Nameservers: {detectionResult.nameservers.join(', ')}
    </AlertDescription>
  </Alert>
)}
```

### 4.5 Implementation Checklist

- [ ] Create DNSDetectionService with nameserver pattern matching
- [ ] Build nameserver pattern database (BuiltInNameservers)
- [ ] Add caching layer with 1-hour TTL
- [ ] Create detection endpoint (POST /api/v1/dns-providers/detect)
- [ ] Add suggestion logic (match detected type to configured providers)
- [ ] Integrate detection into ProxyHostForm (auto-fill DNS provider)
- [ ] Add manual override button (user can change auto-detected provider)
- [ ] Create admin page to view/edit nameserver patterns
- [ ] Add telemetry for detection accuracy (correct/incorrect suggestions)
- [ ] Write unit tests for nameserver pattern matching (85% coverage)
- [ ] Write unit tests for detection UI (85% coverage)
- [ ] Update documentation with auto-detection behavior
- [ ] Add fallback for custom nameservers (unknown providers)

**Estimated Implementation Time:** 6-8 hours

---

## 5. Custom DNS Provider Plugins

### 5.1 Business Case

**Problem:** Charon currently supports 10 major DNS providers. Organizations using niche or internal DNS providers (e.g., internal PowerDNS, custom DNS APIs) cannot use DNS-01 challenges without forking Charon.

**Impact:**

- **Vendor Lock-in:** Users with unsupported providers must switch DNS or manually manage certificates
- **Enterprise Blocker:** Large enterprises with internal DNS cannot adopt Charon
- **Community Growth:** Cannot leverage community contributions for new providers

**User Stories:**

- As a power user, I want to create a plugin for my custom DNS provider
- As an enterprise architect, I need to integrate Charon with our internal DNS API
- As a community contributor, I want to publish DNS provider plugins for others to use

### 5.2 Technical Design (Go Plugins)

#### Plugin Architecture

**Concept:** Use Go's `plugin` package for runtime loading of DNS provider implementations

**Plugin Interface:**

**File: `backend/pkg/dnsprovider/interface.go`**

```go
package dnsprovider

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

type ProviderMetadata struct {
    Type              string            `json:"type"`
    Name              string            `json:"name"`
    Description       string            `json:"description"`
    DocumentationURL  string            `json:"documentation_url"`
    CredentialFields  []CredentialField `json:"credential_fields"`
    Author            string            `json:"author"`
    Version           string            `json:"version"`
}

type CredentialField struct {
    Name        string `json:"name"`
    Label       string `json:"label"`
    Type        string `json:"type"` // "text", "password", "textarea"
    Required    bool   `json:"required"`
    Placeholder string `json:"placeholder"`
    Hint        string `json:"hint"`
}
```

**Example Plugin Implementation:**

**File: `plugins/powerdns/powerdns_plugin.go`**

```go
package main

import (
    "fmt"
    "time"
    "github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

type PowerDNSProvider struct{}

func (p *PowerDNSProvider) GetType() string {
    return "powerdns"
}

func (p *PowerDNSProvider) GetMetadata() dnsprovider.ProviderMetadata {
    return dnsprovider.ProviderMetadata{
        Type:        "powerdns",
        Name:        "PowerDNS",
        Description: "PowerDNS Authoritative Server with HTTP API",
        DocumentationURL: "https://doc.powerdns.com/authoritative/http-api/",
        CredentialFields: []dnsprovider.CredentialField{
            {
                Name:     "api_url",
                Label:    "PowerDNS API URL",
                Type:     "text",
                Required: true,
                Placeholder: "https://pdns.example.com:8081",
            },
            {
                Name:     "api_key",
                Label:    "API Key",
                Type:     "password",
                Required: true,
                Hint:     "X-API-Key header value",
            },
            {
                Name:     "server_id",
                Label:    "Server ID",
                Type:     "text",
                Required: false,
                Placeholder: "localhost",
            },
        },
        Author:  "Your Name",
        Version: "1.0.0",
    }
}

func (p *PowerDNSProvider) ValidateCredentials(credentials map[string]string) error {
    required := []string{"api_url", "api_key"}
    for _, field := range required {
        if credentials[field] == "" {
            return fmt.Errorf("missing required field: %s", field)
        }
    }

    // Optional: Make test API call
    // ...

    return nil
}

func (p *PowerDNSProvider) CreateTXTRecord(zone, name, value string, credentials map[string]string) error {
    apiURL := credentials["api_url"]
    apiKey := credentials["api_key"]
    serverID := credentials["server_id"]
    if serverID == "" {
        serverID = "localhost"
    }

    // Implement PowerDNS API call to create TXT record
    // POST /api/v1/servers/{server_id}/zones/{zone_id}
    // ...

    return nil
}

func (p *PowerDNSProvider) DeleteTXTRecord(zone, name string, credentials map[string]string) error {
    // Implement PowerDNS API call to delete TXT record
    // ...

    return nil
}

func (p *PowerDNSProvider) GetPropagationTimeout() time.Duration {
    return 60 * time.Second // PowerDNS is usually fast
}

// Required: Export symbol for Go plugin system
var Provider PowerDNSProvider
```

**Compile Plugin:**

```bash
go build -buildmode=plugin -o powerdns.so plugins/powerdns/powerdns_plugin.go
```

#### Plugin Loader Service

**File: `backend/internal/services/plugin_loader.go`**

```go
type PluginLoader struct {
    pluginDir  string
    providers  map[string]dnsprovider.Provider
    mu         sync.RWMutex
}

func NewPluginLoader(pluginDir string) *PluginLoader {
    return &PluginLoader{
        pluginDir: pluginDir,
        providers: make(map[string]dnsprovider.Provider),
    }
}

func (pl *PluginLoader) LoadPlugins() error {
    files, err := os.ReadDir(pl.pluginDir)
    if err != nil {
        return err
    }

    for _, file := range files {
        if !strings.HasSuffix(file.Name(), ".so") {
            continue
        }

        pluginPath := filepath.Join(pl.pluginDir, file.Name())
        if err := pl.LoadPlugin(pluginPath); err != nil {
            logger.Log().WithError(err).Warnf("Failed to load plugin: %s", file.Name())
            continue
        }
    }

    logger.Log().Infof("Loaded %d DNS provider plugins", len(pl.providers))
    return nil
}

func (pl *PluginLoader) LoadPlugin(path string) error {
    p, err := plugin.Open(path)
    if err != nil {
        return err
    }

    // Look up exported Provider symbol
    symbol, err := p.Lookup("Provider")
    if err != nil {
        return fmt.Errorf("plugin missing 'Provider' symbol: %w", err)
    }

    provider, ok := symbol.(dnsprovider.Provider)
    if !ok {
        return fmt.Errorf("symbol 'Provider' does not implement dnsprovider.Provider interface")
    }

    // Validate plugin
    metadata := provider.GetMetadata()
    if metadata.Type == "" || metadata.Name == "" {
        return fmt.Errorf("plugin metadata invalid")
    }

    pl.mu.Lock()
    pl.providers[provider.GetType()] = provider
    pl.mu.Unlock()

    logger.Log().WithFields(map[string]interface{}{
        "type":    metadata.Type,
        "name":    metadata.Name,
        "version": metadata.Version,
        "author":  metadata.Author,
    }).Info("Loaded DNS provider plugin")

    return nil
}

func (pl *PluginLoader) GetProvider(providerType string) (dnsprovider.Provider, bool) {
    pl.mu.RLock()
    defer pl.mu.RUnlock()
    provider, ok := pl.providers[providerType]
    return provider, ok
}

func (pl *PluginLoader) ListProviders() []dnsprovider.ProviderMetadata {
    pl.mu.RLock()
    defer pl.mu.RUnlock()

    metadata := make([]dnsprovider.ProviderMetadata, 0, len(pl.providers))
    for _, provider := range pl.providers {
        metadata = append(metadata, provider.GetMetadata())
    }
    return metadata
}
```

### 5.3 Security Considerations

**Plugin Sandboxing:** Go plugins run in the same process space as Charon, so:

- **Code Review:** All plugins must be reviewed before loading
- **Digital Signatures:** Use code signing to verify plugin authenticity
- **Allowlist:** Admin must explicitly enable each plugin via config

**Configuration:**

```yaml
# config/plugins.yaml
dns_providers:
  - plugin: powerdns
    enabled: true
    verified_signature: "sha256:abcd1234..."
  - plugin: custom_internal
    enabled: true
    verified_signature: "sha256:5678efgh..."
```

**Signature Verification:**

```go
func (pl *PluginLoader) VerifySignature(pluginPath string, expectedSig string) error {
    data, err := os.ReadFile(pluginPath)
    if err != nil {
        return err
    }

    hash := sha256.Sum256(data)
    actualSig := "sha256:" + hex.EncodeToString(hash[:])

    if actualSig != expectedSig {
        return fmt.Errorf("signature mismatch: expected %s, got %s", expectedSig, actualSig)
    }

    return nil
}
```

### 5.4 Plugin Marketplace (Future)

**Concept:** Community-driven plugin registry

- **Website:** <https://plugins.charon.io>
- **Submission:** Developers submit plugins via GitHub PR
- **Review:** Core team reviews code for security and quality
- **Signing:** Approved plugins signed with Charon's GPG key
- **Distribution:** Plugins downloadable as `.so` files with signatures

### 5.5 Alternative: gRPC Plugin System

**Pros:**

- Language-agnostic (write plugins in Python, Rust, etc.)
- Better sandboxing (separate process)
- Easier testing and development

**Cons:**

- More complex (requires gRPC server/client)
- Performance overhead (inter-process communication)
- More moving parts (plugin lifecycle management)

**Recommendation:** Start with Go plugins for simplicity, evaluate gRPC if community demand is high.

### 5.6 Implementation Checklist

- [ ] Define dnsprovider.Provider interface
- [ ] Create PluginLoader service
- [ ] Add plugin directory configuration (CHARON_PLUGIN_DIR)
- [ ] Implement plugin loading at startup
- [ ] Add signature verification for plugins
- [ ] Create example plugin (PowerDNS, Infoblox, or Bind)
- [ ] Update DNSProviderService to use plugin providers
- [ ] Add plugin management API endpoints (list, enable, disable)
- [ ] Create frontend admin page for plugin management
- [ ] Write plugin development guide (docs/development/dns-plugins.md)
- [ ] Create plugin SDK with helper functions
- [ ] Write unit tests for plugin loader (85% coverage)
- [ ] Add integration tests with example plugin
- [ ] Document plugin security best practices
- [ ] Create GitHub plugin template repository

**Estimated Implementation Time:** 20-24 hours

---

## Implementation Roadmap

### Phase 1: Security Baseline (P0)

**Duration:** 8-12 hours
**Features:** Audit Logging

**Justification:** Establishes compliance foundation before adding advanced features. Required for SOC 2, GDPR, HIPAA compliance.

**Deliverables:**

- [ ] SecurityAudit model extended with DNS provider fields
- [ ] Audit logging integrated into all DNS provider CRUD operations
- [ ] Audit log UI with filtering and export
- [ ] Documentation updated with audit log usage

---

### Phase 2: Security Hardening (P1)

**Duration:** 16-20 hours
**Features:** Key Rotation Automation

**Justification:** Critical for security posture. Must be implemented before first production deployment with sensitive customer data.

**Deliverables:**

- [ ] Encryption key versioning system
- [ ] RotationService with multi-key support
- [ ] Zero-downtime rotation workflow
- [ ] Admin UI for key management
- [ ] Operations guide with rotation procedures

---

### Phase 3: Advanced Use Cases (P1)

**Duration:** 12-16 hours
**Features:** Multi-Credential per Provider

**Justification:** Unlocks multi-tenancy and zone-level security isolation. High demand from MSPs and large enterprises.

**Deliverables:**

- [ ] DNSProviderCredential model and table
- [ ] Zone-specific credential matching logic
- [ ] Credential management UI
- [ ] Migration tool for existing providers
- [ ] Documentation with multi-tenant setup guide

---

### Phase 4: UX Improvement (P2)

**Duration:** 6-8 hours
**Features:** DNS Provider Auto-Detection

**Justification:** Reduces configuration errors and support burden. Nice-to-have for improving user experience.

**Deliverables:**

- [ ] DNSDetectionService with nameserver pattern matching
- [ ] Auto-detection integrated into ProxyHostForm
- [ ] Admin page for managing nameserver patterns
- [ ] Telemetry for detection accuracy

---

### Phase 5: Extensibility (P3)

**Duration:** 20-24 hours
**Features:** Custom DNS Provider Plugins

**Justification:** Enables community contributions and enterprise-specific integrations. Low priority unless significant community demand.

**Deliverables:**

- [ ] Plugin system architecture and interface
- [ ] PluginLoader service with signature verification
- [ ] Example plugin (PowerDNS or Infoblox)
- [ ] Plugin development guide and SDK
- [ ] Admin UI for plugin management

---

## Dependency Graph

```
Audit Logging (P0)
    │
    ├─────► Key Rotation (P1)
    │           │
    │           └─────► Multi-Credential (P1)
    │                       │
    │                       └─────► Custom Plugins (P3)
    │
    └─────► DNS Auto-Detection (P2)
```

**Explanation:**

- Audit Logging should be implemented first as it establishes the foundation for tracking all future features
- Key Rotation depends on audit logging to track rotation events
- Multi-Credential can be implemented in parallel with Key Rotation but benefits from audit logging
- DNS Auto-Detection is independent and can be implemented anytime
- Custom Plugins should be last as it's the most complex and benefits from mature audit/rotation systems

---

## Risk Assessment Matrix

| Feature | Security Risk | Complexity Risk | Maintenance Burden |
|---------|---------------|-----------------|---------------------|
| Audit Logging | Low | Low | Low (append-only logs) |
| Key Rotation | Medium (key mgmt) | High (zero-downtime) | Medium (periodic validation) |
| Multi-Credential | Medium (zone isolation) | Medium (matching logic) | Medium (zone updates) |
| DNS Auto-Detection | Low | Low | High (nameserver DB updates) |
| Custom Plugins | **High** (code exec) | **Very High** (sandboxing) | **High** (security reviews) |

**Mitigation Strategies:**

- **Key Rotation:** Extensive testing in staging, phased rollout, rollback plan documented
- **Multi-Credential:** Thorough zone matching tests, fallback to catch-all credential
- **Custom Plugins:** Mandatory code review, signature verification, allowlist-only loading, separate process space (gRPC alternative)

---

## Resource Requirements

### Development Time (Total: 62-80 hours)

- Audit Logging: 8-12 hours
- Key Rotation: 16-20 hours
- Multi-Credential: 12-16 hours
- Auto-Detection: 6-8 hours
- Custom Plugins: 20-24 hours

### Testing Time (Estimate: 40% of dev time)

- Unit tests: 25-32 hours
- Integration tests: 10-12 hours
- Security testing: 8-10 hours

### Documentation Time (Estimate: 20% of dev time)

- User guides: 8-10 hours
- API documentation: 4-6 hours
- Operations guides: 6-8 hours

**Total Project Time: 130-160 hours (~3-4 weeks for one developer)**

---

## Success Metrics

### Audit Logging

- 100% of DNS provider operations logged
- Audit log retention policy enforced automatically
- Zero performance impact (<1ms per log entry)

### Key Rotation

- Zero downtime during rotation
- 100% credential re-encryption success rate
- Rotation time <5 minutes for 100 providers

### Multi-Credential

- Zone matching accuracy >99%
- Support for 10+ credentials per provider
- No certificate issuance failures due to wrong credential

### DNS Auto-Detection

- Detection accuracy >95% for supported providers
- Auto-detection time <500ms per domain
- User override available for edge cases

### Custom Plugins

- Plugin loading time <100ms per plugin
- Zero crashes from malicious plugins (sandbox effective)
- >5 community-contributed plugins within 6 months

---

## Conclusion

These 5 features represent the natural evolution of Charon's DNS Challenge Support from MVP to enterprise-ready. The recommended implementation order prioritizes security and compliance (Audit Logging, Key Rotation) before advanced features (Multi-Credential, Auto-Detection, Custom Plugins).

**Next Steps:**

1. Review and approve this planning document
2. Create GitHub issues for each feature (link to this spec)
3. Begin implementation starting with Audit Logging (P0)
4. Establish automated testing and documentation standards
5. Monitor community feedback to adjust priorities

**Document Version:** 1.0
**Last Updated:** January 2, 2026
**Status:** Planning Phase - Awaiting Approval
