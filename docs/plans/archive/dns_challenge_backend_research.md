# DNS Challenge Backend Research - Issue #21

## Executive Summary

This document analyzes the Charon backend to understand how to implement DNS challenge support for wildcard certificates. The research covers current Caddy integration, existing model patterns, and proposes new models, API endpoints, and encryption strategies.

---

## 1. Current Caddy Integration Analysis

### 1.1 Configuration Generation Flow

The Caddy configuration is generated in [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go).

**Key function:** `GenerateConfig()` (line 17)

```go
func GenerateConfig(hosts []models.ProxyHost, storageDir, acmeEmail, frontendDir, sslProvider string,
    acmeStaging, crowdsecEnabled, wafEnabled, rateLimitEnabled, aclEnabled bool,
    adminWhitelist string, rulesets []models.SecurityRuleSet, rulesetPaths map[string]string,
    decisions []models.SecurityDecision, secCfg *models.SecurityConfig) (*Config, error)
```

### 1.2 Current TLS/ACME Configuration

**Location:** [config.go#L66-L105](../../backend/internal/caddy/config.go#L66-L105)

Current SSL provider handling:

- `letsencrypt` - ACME with Let's Encrypt
- `zerossl` - ZeroSSL module
- `both`/default - Both issuers as fallback

**Current Issuer Configuration:**

```go
switch sslProvider {
case "letsencrypt":
    acmeIssuer := map[string]any{
        "module": "acme",
        "email":  acmeEmail,
    }
    if acmeStaging {
        acmeIssuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
    }
    issuers = append(issuers, acmeIssuer)
// ...
}
```

### 1.3 TLS App Structure

**Location:** [backend/internal/caddy/types.go#L172-L199](../../backend/internal/caddy/types.go#L172-L199)

```go
type TLSApp struct {
    Automation   *AutomationConfig   `json:"automation,omitempty"`
    Certificates *CertificatesConfig `json:"certificates,omitempty"`
}

type AutomationConfig struct {
    Policies []*AutomationPolicy `json:"policies,omitempty"`
}

type AutomationPolicy struct {
    Subjects   []string `json:"subjects,omitempty"`
    IssuersRaw []any    `json:"issuers,omitempty"`
}
```

### 1.4 Config Application Flow

**Manager:** [backend/internal/caddy/manager.go](../../backend/internal/caddy/manager.go)

1. `ApplyConfig()` fetches proxy hosts from DB
2. Reads settings (ACME email, SSL provider)
3. Calls `GenerateConfig()` to build Caddy JSON
4. Validates configuration
5. Saves snapshot for rollback
6. Applies via Caddy admin API

---

## 2. Existing Model Patterns

### 2.1 Model Structure Convention

All models follow this pattern:

| Field | Type | Purpose |
|-------|------|---------|
| `ID` | `uint` | Primary key |
| `UUID` | `string` | External identifier |
| `Name` | `string` | Human-readable name |
| `Enabled` | `bool` | Active state |
| `CreatedAt`/`UpdatedAt` | `time.Time` | Timestamps |

### 2.2 Relevant Existing Models

#### SSLCertificate ([ssl_certificate.go](../../backend/internal/models/ssl_certificate.go))

```go
type SSLCertificate struct {
    ID          uint       `json:"id" gorm:"primaryKey"`
    UUID        string     `json:"uuid" gorm:"uniqueIndex"`
    Name        string     `json:"name" gorm:"index"`
    Provider    string     `json:"provider" gorm:"index"`        // "letsencrypt", "custom", "self-signed"
    Domains     string     `json:"domains" gorm:"index"`         // comma-separated
    Certificate string     `json:"certificate" gorm:"type:text"` // PEM-encoded
    PrivateKey  string     `json:"private_key" gorm:"type:text"` // PEM-encoded
    ExpiresAt   *time.Time `json:"expires_at,omitempty" gorm:"index"`
    AutoRenew   bool       `json:"auto_renew" gorm:"default:false"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}
```

#### SecurityConfig ([security_config.go](../../backend/internal/models/security_config.go))

- Stores global security settings
- Uses `gorm:"type:text"` for JSON blobs
- Has sensitive field (`BreakGlassHash`) with `json:"-"` tag

#### Setting ([setting.go](../../backend/internal/models/setting.go))

```go
type Setting struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    Key       string    `json:"key" gorm:"uniqueIndex"`
    Value     string    `json:"value" gorm:"type:text"`
    Type      string    `json:"type" gorm:"index"`     // "string", "int", "bool", "json"
    Category  string    `json:"category" gorm:"index"` // grouping
    UpdatedAt time.Time `json:"updated_at"`
}
```

#### NotificationProvider ([notification_provider.go](../../backend/internal/models/notification_provider.go))

- Stores webhook URLs and configs
- **Currently does NOT encrypt sensitive data** (URLs stored as plaintext)
- Uses JSON config for flexible provider-specific data

### 2.3 Password/Secret Handling Pattern

**Location:** [backend/internal/models/user.go](../../backend/internal/models/user.go)

Uses bcrypt for password hashing:

```go
func (u *User) SetPassword(password string) error {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    // ...
}
```

**Important:** Bcrypt is one-way hashing (good for passwords). DNS credentials need **reversible encryption** (AES-GCM).

---

## 3. Proposed New Models

### 3.1 DNSProvider Model

```go
// DNSProvider represents a DNS provider configuration for ACME DNS-01 challenges.
// Used for wildcard certificate issuance via DNS validation.
type DNSProvider struct {
    ID          uint      `json:"id" gorm:"primaryKey"`
    UUID        string    `json:"uuid" gorm:"uniqueIndex"`
    Name        string    `json:"name" gorm:"index;not null"`           // User-friendly name
    ProviderType string   `json:"provider_type" gorm:"index;not null"`  // cloudflare, route53, godaddy, etc.
    Enabled     bool      `json:"enabled" gorm:"default:true;index"`

    // Encrypted credentials stored as JSON blob
    // Contains provider-specific fields (api_key, api_secret, zone_id, etc.)
    CredentialsEncrypted string `json:"-" gorm:"type:text;column:credentials_encrypted"`

    // Propagation settings (DNS record TTL considerations)
    PropagationTimeout int `json:"propagation_timeout" gorm:"default:120"` // seconds
    PollingInterval    int `json:"polling_interval" gorm:"default:5"`      // seconds

    // Usage tracking
    LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
    SuccessCount  int        `json:"success_count" gorm:"default:0"`
    FailureCount  int        `json:"failure_count" gorm:"default:0"`

    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 3.2 DNSProviderCredential (Alternative: Separate Table)

If we want to support multiple credential sets per provider (e.g., different zones):

```go
// DNSProviderCredential stores encrypted credentials for a DNS provider.
type DNSProviderCredential struct {
    ID            uint   `json:"id" gorm:"primaryKey"`
    UUID          string `json:"uuid" gorm:"uniqueIndex"`
    DNSProviderID uint   `json:"dns_provider_id" gorm:"index;not null"`
    DNSProvider   *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`

    Label         string `json:"label" gorm:"index"`                      // "Production Zone", "Dev Account"
    ZoneID        string `json:"zone_id,omitempty"`                       // Optional zone restriction

    // Encrypted credential blob
    EncryptedData string `json:"-" gorm:"type:text;not null"`

    // Key derivation metadata (for key rotation)
    KeyVersion    int       `json:"key_version" gorm:"default:1"`
    EncryptedAt   time.Time `json:"encrypted_at"`

    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 3.3 Supported Provider Types

| Provider | Required Credentials | Caddy Module |
|----------|---------------------|--------------|
| Cloudflare | `api_token` or `api_key` + `email` | `cloudflare` |
| Route53 (AWS) | `access_key_id`, `secret_access_key`, `region` | `route53` |
| Google Cloud DNS | `service_account_json` | `googleclouddns` |
| DigitalOcean | `auth_token` | `digitalocean` |
| Namecheap | `api_user`, `api_key`, `client_ip` | `namecheap` |
| GoDaddy | `api_key`, `api_secret` | `godaddy` |
| Hetzner | `api_key` | `hetzner` |
| Vultr | `api_key` | `vultr` |
| DNSimple | `oauth_token`, `account_id` | `dnsimple` |
| Azure DNS | `tenant_id`, `client_id`, `client_secret`, `subscription_id`, `resource_group` | `azuredns` |

---

## 4. API Endpoint Design

### 4.1 DNS Provider Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/dns-providers` | List all DNS providers |
| `POST` | `/api/v1/dns-providers` | Create new DNS provider |
| `GET` | `/api/v1/dns-providers/:id` | Get provider details |
| `PUT` | `/api/v1/dns-providers/:id` | Update provider |
| `DELETE` | `/api/v1/dns-providers/:id` | Delete provider |
| `POST` | `/api/v1/dns-providers/:id/test` | Test DNS provider connectivity |
| `GET` | `/api/v1/dns-providers/types` | List supported provider types with required fields |

### 4.2 Request/Response Examples

#### Create DNS Provider Request

```json
{
  "name": "My Cloudflare Account",
  "provider_type": "cloudflare",
  "credentials": {
    "api_token": "xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  },
  "propagation_timeout": 120,
  "polling_interval": 5
}
```

#### List DNS Providers Response

```json
{
  "providers": [
    {
      "id": 1,
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "name": "My Cloudflare Account",
      "provider_type": "cloudflare",
      "enabled": true,
      "has_credentials": true,
      "propagation_timeout": 120,
      "polling_interval": 5,
      "last_used_at": "2026-01-01T10:30:00Z",
      "success_count": 15,
      "failure_count": 0,
      "created_at": "2025-12-01T08:00:00Z"
    }
  ]
}
```

### 4.3 Integration with Proxy Host

Extend `ProxyHost` model:

```go
type ProxyHost struct {
    // ... existing fields ...

    // DNS Challenge configuration
    DNSProviderID    *uint        `json:"dns_provider_id,omitempty" gorm:"index"`
    DNSProvider      *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`
    UseDNSChallenge  bool         `json:"use_dns_challenge" gorm:"default:false"`
}
```

---

## 5. Encryption Strategy

### 5.1 Recommended Approach: AES-256-GCM

**Why AES-GCM:**

- Authenticated encryption (provides confidentiality + integrity)
- Standard and well-vetted
- Fast on modern CPUs with AES-NI
- Used by industry standards (TLS 1.3, Google, AWS KMS)

### 5.2 Implementation Plan

#### New Package: `backend/internal/crypto/`

```go
// crypto/encryption.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"
)

// EncryptionService handles credential encryption/decryption
type EncryptionService struct {
    key []byte // 32 bytes for AES-256
}

// NewEncryptionService creates a service with the provided key
func NewEncryptionService(keyBase64 string) (*EncryptionService, error) {
    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil || len(key) != 32 {
        return nil, errors.New("invalid encryption key: must be 32 bytes base64 encoded")
    }
    return &EncryptionService{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (s *EncryptionService) Encrypt(plaintext []byte) (string, error) {
    block, err := aes.NewCipher(s.key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (s *EncryptionService) Decrypt(ciphertextB64 string) ([]byte, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
    if err != nil {
        return nil, err
    }

    block, err := aes.NewCipher(s.key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

### 5.3 Key Management

#### Environment Variable

```bash
CHARON_ENCRYPTION_KEY=<base64-encoded-32-byte-key>
```

#### Key Generation (one-time setup)

```bash
openssl rand -base64 32
```

#### Configuration Extension

```go
// config/config.go
type Config struct {
    // ... existing fields ...
    EncryptionKey string // From CHARON_ENCRYPTION_KEY
}
```

### 5.4 Security Considerations

1. **Key Storage:** Encryption key MUST be stored securely (env var, secrets manager)
2. **Key Rotation:** Include `KeyVersion` field for future key rotation support
3. **Memory Safety:** Zero out decrypted credentials after use where possible
4. **Audit Logging:** Log access to encrypted credentials (without logging the values)
5. **Backup Encryption:** Ensure database backups don't expose plaintext credentials

---

## 6. Files to Create/Modify

### 6.1 New Files to Create

| File | Purpose |
|------|---------|
| `backend/internal/crypto/encryption.go` | AES-GCM encryption service |
| `backend/internal/crypto/encryption_test.go` | Encryption unit tests |
| `backend/internal/models/dns_provider.go` | DNSProvider model |
| `backend/internal/services/dns_provider_service.go` | DNS provider CRUD + credential handling |
| `backend/internal/services/dns_provider_service_test.go` | Service unit tests |
| `backend/internal/api/handlers/dns_provider_handler.go` | API handlers |
| `backend/internal/api/handlers/dns_provider_handler_test.go` | Handler tests |

### 6.2 Files to Modify

| File | Changes |
|------|---------|
| `backend/internal/config/config.go` | Add `EncryptionKey` field |
| `backend/internal/models/proxy_host.go` | Add `DNSProviderID`, `UseDNSChallenge` fields |
| `backend/internal/caddy/types.go` | Add DNS challenge issuer types |
| `backend/internal/caddy/config.go` | Add DNS challenge configuration generation |
| `backend/internal/caddy/manager.go` | Load DNS providers when applying config |
| `backend/internal/api/routes/routes.go` | Register DNS provider routes |
| `backend/internal/api/handlers/proxyhost_handler.go` | Handle DNS provider association |

---

## 7. Caddy DNS Challenge Configuration

### 7.1 Target Caddy JSON Structure

For DNS-01 challenges, the TLS automation policy needs a `challenges` block:

```json
{
  "apps": {
    "tls": {
      "automation": {
        "policies": [
          {
            "subjects": ["*.example.com", "example.com"],
            "issuers": [
              {
                "module": "acme",
                "email": "admin@example.com",
                "challenges": {
                  "dns": {
                    "provider": {
                      "name": "cloudflare",
                      "api_token": "{env.CF_API_TOKEN}"
                    },
                    "propagation_timeout": 120000000000,
                    "resolvers": ["1.1.1.1:53"]
                  }
                }
              }
            ]
          }
        ]
      }
    }
  }
}
```

### 7.2 New Caddy Types

```go
// types.go additions

// DNSChallengeConfig configures DNS-01 challenge settings
type DNSChallengeConfig struct {
    Provider           map[string]any `json:"provider"`
    PropagationTimeout int64          `json:"propagation_timeout,omitempty"` // nanoseconds
    Resolvers          []string       `json:"resolvers,omitempty"`
}

// ChallengesConfig configures ACME challenge types
type ChallengesConfig struct {
    DNS *DNSChallengeConfig `json:"dns,omitempty"`
}

// Update AutomationPolicy
type AutomationPolicy struct {
    Subjects   []string          `json:"subjects,omitempty"`
    IssuersRaw []any             `json:"issuers,omitempty"`
    // Note: Challenges are configured per-issuer in IssuersRaw
}
```

---

## 8. Implementation Phases

### Phase 1: Foundation

1. Create encryption package
2. Create DNSProvider model
3. Database migration

### Phase 2: Service Layer

1. DNS provider service (CRUD)
2. Credential encryption/decryption
3. Provider connectivity testing

### Phase 3: API Layer

1. DNS provider handlers
2. Route registration
3. API validation

### Phase 4: Caddy Integration

1. Update config generation
2. DNS challenge issuer building
3. ProxyHost integration

### Phase 5: Testing & Documentation

1. Unit tests (>85% coverage)
2. Integration tests
3. API documentation

---

## 9. References

- [Caddy DNS Challenge Documentation](https://caddyserver.com/docs/automatic-https#dns-challenge)
- [Caddy JSON Structure](https://caddyserver.com/docs/json/)
- [ACME DNS-01 Challenge Spec](https://datatracker.ietf.org/doc/html/rfc8555#section-8.4)
- [Go crypto/cipher Package](https://pkg.go.dev/crypto/cipher)
- [Caddy DNS Provider Modules](https://github.com/caddy-dns)

---

*Document created: January 1, 2026*
*Issue: #21 - DNS Challenge Support*
*Priority: Critical*
