# Custom Certificate Upload & Management

**Issue**: #22 — Custom Certificate Upload & Management
**Date**: 2026-04-10
**Status**: Draft — Awaiting Approval
**Priority**: High
**Milestone**: Beta
**Labels**: high, beta, ssl
**Archived**: Previous plan (Nightly Build Vulnerability Remediation) → `docs/plans/archive/nightly-vuln-remediation-spec.md`

---

## 1. Executive Summary

Charon currently supports automatic certificate provisioning via Let's Encrypt/ZeroSSL (ACME) and has a rudimentary custom certificate upload flow (basic PEM upload with name). This plan enhances the certificate management system to support:

- **Full certificate validation pipeline** (format, chain, expiry, key matching)
- **Private key encryption at rest** using the existing `CHARON_ENCRYPTION_KEY` infrastructure
- **Multiple certificate formats** (PEM, PFX/PKCS12, DER)
- **Certificate chain/intermediate support**
- **Certificate assignment to proxy hosts** via the UI
- **Expiry warning notifications** using the existing notification infrastructure
- **Certificate export** with format conversion
- **Enhanced upload UI** with drag-and-drop, validation feedback, and chain preview

### Why This Matters

Users who bring their own certificates (enterprise CAs, internal PKI, wildcard certs from commercial providers) need a secure, validated workflow for importing, managing, and assigning certificates. Currently, private keys are stored in plaintext in the database, there is no format validation beyond basic PEM decoding, and the UI lacks chain support and export capabilities.

---

## 2. Current State Analysis

### 2.1 What Already Exists

| Component | Status | Location | Notes |
|-----------|--------|----------|-------|
| **SSLCertificate model** | Exists | `backend/internal/models/ssl_certificate.go` | Has `Certificate`, `PrivateKey` (plaintext), `Domains`, `ExpiresAt`, `Provider` |
| **CertificateService** | Exists | `backend/internal/services/certificate_service.go` | `UploadCertificate()`, `ListCertificates()`, `DeleteCertificate()`, `IsCertificateInUse()`, disk sync for ACME |
| **CertificateHandler** | Exists | `backend/internal/api/handlers/certificate_handler.go` | `List`, `Upload`, `Delete` endpoints |
| **API routes** | Exists | `backend/internal/api/routes/routes.go:664-675` | `GET/POST/DELETE /api/v1/certificates` |
| **Frontend API client** | Exists | `frontend/src/api/certificates.ts` | `getCertificates()`, `uploadCertificate()`, `deleteCertificate()` |
| **Certificates page** | Exists | `frontend/src/pages/Certificates.tsx` | Upload dialog (name + 2 files), list view |
| **CertificateList** | Exists | `frontend/src/components/CertificateList.tsx` | Table with sort, bulk delete, status display |
| **useCertificates hook** | Exists | `frontend/src/hooks/useCertificates.ts` | React Query wrapper |
| **Caddy TLS loading** | Exists | `backend/internal/caddy/config.go:418-453` | Custom certs loaded via `LoadPEM` in TLS app |
| **Caddy types** | Exists | `backend/internal/caddy/types.go:239-266` | `TLSApp`, `CertificatesConfig`, `LoadPEMConfig` |
| **Encryption service** | Exists | `backend/internal/crypto/encryption.go` | AES-256-GCM encrypt/decrypt with `CHARON_ENCRYPTION_KEY` |
| **Key rotation** | Exists | `backend/internal/crypto/rotation_service.go` | Multi-version key rotation for DNS provider credentials |
| **Notification service** | Exists | `backend/internal/services/notification_service.go` | `SendExternal()` with event types, `Create()` for in-app |
| **ProxyHost.CertificateID** | Exists | `backend/internal/models/proxy_host.go` | FK to SSLCertificate, already used in update handler |
| **Delete E2E tests** | Exists | `tests/certificate-delete.spec.ts`, `tests/certificate-bulk-delete.spec.ts` | Delete flow E2E coverage |
| **Config tests** | Exists | `backend/internal/caddy/config_test.go:1480-1600` | Custom cert loading via Caddy tested |

### 2.2 Gaps to Address

| Gap | Severity | Description |
|-----|----------|-------------|
| **Private keys stored in plaintext** | CRITICAL | `PrivateKey` field in `SSLCertificate` is stored as raw PEM. Must encrypt at rest. |
| **🔴 Active private key disclosure** | CRITICAL | The Upload handler (`certificate_handler.go:137`) returns the full `*SSLCertificate` struct via `c.JSON(http.StatusCreated, cert)`. Because the model has `json:"private_key"`, the raw PEM private key is sent to the client in every upload response. **This is an active vulnerability in production.** Commit 1 fixes this by changing the tag to `json:"-"`. |
| **Unsafe file read pattern** | HIGH | `certificate_handler.go:109` uses `certSrc.Read(certBytes)` which may return partial reads. Must use `io.ReadAll(io.LimitReader(src, 1<<20))` for safe, bounded reads. Commit 2 (task 2.1) fixes this. |
| **No certificate chain validation** | HIGH | Upload accepts any PEM without verifying chain or key-cert match. |
| **No format conversion** | HIGH | Only PEM is accepted. PFX/DER users cannot upload. |
| **No expiry warnings** | HIGH | No scheduled check or notification for upcoming certificate expiry. |
| **No certificate export** | MEDIUM | Users cannot download certs they uploaded (for migration, backup). |
| **No chain/intermediate storage** | MEDIUM | Model has single `Certificate` field; no dedicated chain field. |
| **No certificate detail view** | MEDIUM | Frontend shows only list; no detail/expand view showing SANs, issuer chain, fingerprint. |
| **`CertificateInfo` leaks numeric ID** | HIGH | `CertificateInfo.ID uint json:"id,omitempty"` in service — violates GORM security rules. |
| **Delete uses numeric ID in URL** | HIGH | `DELETE /certificates/:id` uses numeric ID; should use UUID. |

---

## 3. Requirements (EARS Notation)

### 3.1 Certificate Upload

| ID | Requirement |
|----|-------------|
| R-UP-01 | WHEN a user submits a certificate upload form with valid PEM, PFX, or DER files, THE SYSTEM SHALL parse and validate the certificate, encrypt the private key at rest, store the certificate in the database, and return the certificate metadata. |
| R-UP-02 | WHEN a user uploads a PFX/PKCS12 file with a password, THE SYSTEM SHALL decrypt the PFX, extract the certificate chain and private key, convert to PEM, and store them. |
| R-UP-03 | WHEN a user uploads a DER-encoded certificate, THE SYSTEM SHALL convert it to PEM format before storage. |
| R-UP-04 | WHEN a certificate upload contains intermediate certificates, THE SYSTEM SHALL store the full chain in order (leaf then intermediate then root). |
| R-UP-05 | IF a user uploads a certificate whose private key does not match the certificate's public key, THEN THE SYSTEM SHALL reject the upload with a descriptive error. |
| R-UP-06 | IF a user uploads an expired certificate, THEN THE SYSTEM SHALL warn but still allow storage (with status "expired"). |
| R-UP-07 | THE SYSTEM SHALL enforce a maximum upload size of 1MB per file to prevent abuse. |
| R-UP-08 | IF a user uploads a file that is not a valid certificate or key format, THEN THE SYSTEM SHALL reject the upload with a descriptive error. |

### 3.2 Certificate Validation

| ID | Requirement |
|----|-------------|
| R-VL-01 | WHEN a certificate is uploaded, THE SYSTEM SHALL verify the X.509 structure, extract the Common Name, SANs, issuer, serial number, and expiry date. |
| R-VL-02 | WHEN a certificate chain is provided, THE SYSTEM SHALL verify that each certificate in the chain is signed by the next certificate (leaf then intermediate then root). |
| R-VL-03 | WHEN a private key is uploaded, THE SYSTEM SHALL verify that the key matches the certificate's public key by comparing the public key modulus. |

### 3.3 Private Key Security

| ID | Requirement |
|----|-------------|
| R-PK-01 | THE SYSTEM SHALL encrypt all custom certificate private keys at rest using AES-256-GCM via the existing `CHARON_ENCRYPTION_KEY`. |
| R-PK-02 | THE SYSTEM SHALL decrypt private keys only when serving them to Caddy for TLS or when exporting. |
| R-PK-03 | THE SYSTEM SHALL never return private key content in API list/get responses. |
| R-PK-04 | WHEN `CHARON_ENCRYPTION_KEY` is rotated, THE SYSTEM SHALL re-encrypt all stored private keys during the rotation process. |

### 3.4 Certificate Assignment

| ID | Requirement |
|----|-------------|
| R-AS-01 | WHEN a user assigns a custom certificate to a proxy host, THE SYSTEM SHALL update the proxy host's `CertificateID` and reload Caddy configuration. |
| R-AS-02 | WHEN a custom certificate is assigned to a proxy host, THE SYSTEM SHALL use `LoadPEM` in Caddy's TLS app to serve the certificate for that domain. |
| R-AS-03 | THE SYSTEM SHALL prevent deletion of certificates that are assigned to one or more proxy hosts. |

### 3.5 Expiry Warnings

| ID | Requirement |
|----|-------------|
| R-EX-01 | THE SYSTEM SHALL check certificate expiry dates daily via a background scheduler. |
| R-EX-02 | WHEN a custom certificate will expire within 30 days, THE SYSTEM SHALL create an in-app warning notification. |
| R-EX-03 | WHEN a custom certificate will expire within 30 days AND external notification providers are configured, THE SYSTEM SHALL send an external notification (rate-limited to once per 24 hours per certificate). |
| R-EX-04 | WHEN a custom certificate has expired, THE SYSTEM SHALL update its status to "expired" and send a critical notification. |

### 3.6 Certificate Export

| ID | Requirement |
|----|-------------|
| R-EXP-01 | WHEN a user requests a certificate export, THE SYSTEM SHALL provide the certificate and chain in the requested format (PEM, PFX, DER). |
| R-EXP-02 | WHEN exporting in PFX format, THE SYSTEM SHALL prompt for a password and encrypt the PFX bundle. |
| R-EXP-03 | THE SYSTEM SHALL require authentication for all export operations. |
| R-EXP-04 | THE SYSTEM SHALL never include the private key in export unless explicitly requested with re-authentication. |

### 3.7 UI/UX

| ID | Requirement |
|----|-------------|
| R-UI-01 | THE SYSTEM SHALL support drag-and-drop file upload for certificate and key files. |
| R-UI-02 | WHEN a certificate is uploaded, THE SYSTEM SHALL display a preview showing: domains (CN + SANs), issuer, expiry date, chain depth, and key match status. |
| R-UI-03 | THE SYSTEM SHALL display an expiry warning badge on certificates expiring within 30 days. |
| R-UI-04 | THE SYSTEM SHALL provide a certificate detail view showing full metadata including fingerprint, serial number, issuer chain, and assigned hosts. |

---

## 4. Technical Architecture

### 4.1 Database Model Changes

#### Modified: `SSLCertificate` (`backend/internal/models/ssl_certificate.go`)

```go
type SSLCertificate struct {
    ID                    uint       `json:"-" gorm:"primaryKey"`
    UUID                  string     `json:"uuid" gorm:"uniqueIndex"`
    Name                  string     `json:"name" gorm:"index"`
    Provider              string     `json:"provider" gorm:"index"`
    Domains               string     `json:"domains" gorm:"index"`
    CommonName            string     `json:"common_name"`                        // NEW
    Certificate           string     `json:"-" gorm:"type:text"`                // CHANGED: hide from JSON
    CertificateChain      string     `json:"-" gorm:"type:text"`                // NEW
    PrivateKeyEncrypted   string     `json:"-" gorm:"column:private_key_enc;type:text"` // NEW
    PrivateKey            string     `json:"-" gorm:"-"`                         // CHANGED: json:"-" fixes active private key disclosure (was json:"private_key"), gorm:"-" excludes from queries (column kept but values cleared)
    KeyVersion            int        `json:"-" gorm:"default:1"`                 // NEW
    Fingerprint           string     `json:"fingerprint"`                        // NEW
    SerialNumber          string     `json:"serial_number"`                      // NEW
    IssuerOrg             string     `json:"issuer_org"`                         // NEW
    KeyType               string     `json:"key_type"`                           // NEW — see KeyType values below
    ExpiresAt             *time.Time `json:"expires_at,omitempty" gorm:"index"`
    NotBefore             *time.Time `json:"not_before,omitempty"`               // NEW
    AutoRenew             bool       `json:"auto_renew" gorm:"default:false"`
    CreatedAt             time.Time  `json:"created_at"`
    UpdatedAt             time.Time  `json:"updated_at"`
}
```

**`KeyType` enum values**: `RSA-2048`, `RSA-4096`, `ECDSA-P256`, `ECDSA-P384`, `Ed25519`. Derived from the parsed private key at upload time.

**Migration strategy**: Add new columns with defaults. Migrate existing plaintext `PrivateKey` data to `PrivateKeyEncrypted` via a dedicated migration step. After migration, clear `private_key` values (set to empty string) but **do NOT drop the column** — SQLite < 3.35.0 does not support `ALTER TABLE DROP COLUMN`, and GORM's `DropColumn` has varying support. Add `gorm:"-"` tag to the `PrivateKey` field so GORM ignores it in all queries.

**Migration verification criteria**: No rows where `private_key != '' AND private_key_enc == ''`.

**Follow-up task** (outside this feature's 4 PRs): Drop `private_key` column in a future release once all deployments are confirmed migrated and SQLite version requirements are established.

#### Modified: `CertificateInfo` (`backend/internal/services/certificate_service.go`)

```go
type CertificateInfo struct {
    UUID         string    `json:"uuid"`
    Name         string    `json:"name,omitempty"`
    CommonName   string    `json:"common_name,omitempty"`
    Domains      string    `json:"domains"`
    Issuer       string    `json:"issuer"`
    IssuerOrg    string    `json:"issuer_org,omitempty"`
    Fingerprint  string    `json:"fingerprint,omitempty"`
    SerialNumber string    `json:"serial_number,omitempty"`
    KeyType      string    `json:"key_type,omitempty"`
    ExpiresAt    time.Time `json:"expires_at"`
    NotBefore    time.Time `json:"not_before,omitempty"`
    Status       string    `json:"status"`
    Provider     string    `json:"provider"`
    ChainDepth   int       `json:"chain_depth,omitempty"`
    HasKey       bool      `json:"has_key"`
    InUse        bool      `json:"in_use"`
}
```

### 4.2 API Endpoints

All endpoints under `/api/v1` require authentication (existing middleware).

#### Existing (Modified)

| Method | Path | Changes |
|--------|------|---------|
| `GET` | `/certificates` | Response uses `CertificateInfo` (UUID only, no numeric ID, new metadata fields) |
| `POST` | `/certificates` | Accept PEM, PFX, DER; encrypt private key; validate chain; return `CertificateInfo` |
| `DELETE` | `/certificates/:uuid` | CHANGED: Use UUID param instead of numeric ID |

#### UUID-to-uint Resolution for Certificate Assignment

The `ProxyHost.CertificateID` field is `*uint` — this **will not change** to UUID. It remains a numeric foreign key. When the certificate assignment endpoint receives a certificate UUID (from the UI/API), the handler **must resolve UUID → numeric ID** via a DB lookup (`SELECT id FROM ssl_certificates WHERE uuid = ?`) before setting `ProxyHost.CertificateID`. Implementers must NOT attempt to change the FK type to UUID.

#### New Endpoints

| Method | Path | Description | Request | Response |
|--------|------|-------------|---------|----------|
| `GET` | `/certificates/:uuid` | Get certificate detail | — | `CertificateDetail` (full metadata, chain info, assigned hosts) |
| `POST` | `/certificates/:uuid/export` | Export certificate | JSON body (format, include_key, pfx_password, password) | Binary file download |
| `PUT` | `/certificates/:uuid` | Update certificate metadata (name) | JSON body (name) | `CertificateInfo` |
| `POST` | `/certificates/validate` | Validate certificate without storing | Multipart (same as upload) | `ValidationResult` |

#### Request/Response Schemas

**Upload Request** (`POST /certificates`) — Multipart form:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Display name |
| `certificate_file` | file | Yes | Certificate file (.pem, .crt, .cer, .pfx, .p12, .der) |
| `key_file` | file | Conditional | Private key file (required for PEM/DER; not needed for PFX) |
| `chain_file` | file | No | Intermediate chain file (PEM) |
| `pfx_password` | string | Conditional | Password for PFX decryption |

**Upload Response** (`201 Created`):

```json
{
  "uuid": "a1b2c3d4-...",
  "name": "My Wildcard Cert",
  "common_name": "*.example.com",
  "domains": "*.example.com,example.com",
  "issuer": "custom",
  "issuer_org": "DigiCert Inc",
  "fingerprint": "AB:CD:EF:...",
  "serial_number": "03:A1:...",
  "key_type": "RSA-2048",
  "expires_at": "2027-04-10T00:00:00Z",
  "not_before": "2026-04-10T00:00:00Z",
  "status": "valid",
  "provider": "custom",
  "chain_depth": 2,
  "has_key": true,
  "in_use": false
}
```

**Certificate Detail Response** (`GET /certificates/:uuid`):

```json
{
  "uuid": "a1b2c3d4-...",
  "name": "My Wildcard Cert",
  "common_name": "*.example.com",
  "domains": "*.example.com,example.com",
  "issuer": "custom",
  "issuer_org": "DigiCert Inc",
  "fingerprint": "AB:CD:EF:...",
  "serial_number": "03:A1:...",
  "key_type": "RSA-2048",
  "expires_at": "2027-04-10T00:00:00Z",
  "not_before": "2026-04-10T00:00:00Z",
  "status": "valid",
  "provider": "custom",
  "chain_depth": 2,
  "has_key": true,
  "in_use": true,
  "assigned_hosts": [
    {"uuid": "host-uuid-1", "name": "My App", "domain_names": "app.example.com"}
  ],
  "chain": [
    {"subject": "*.example.com", "issuer": "DigiCert SHA2 Extended Validation Server CA", "expires_at": "2027-04-10T00:00:00Z"},
    {"subject": "DigiCert SHA2 Extended Validation Server CA", "issuer": "DigiCert Global Root CA", "expires_at": "2031-11-10T00:00:00Z"}
  ],
  "auto_renew": false,
  "created_at": "2026-04-10T12:00:00Z",
  "updated_at": "2026-04-10T12:00:00Z"
}
```

**Export Request** (`POST /certificates/:uuid/export`):

```json
{
  "format": "pem",
  "include_key": true,
  "pfx_password": "optional-for-pfx",
  "password": "current-user-password"
}
```

**R-EXP-04 Re-authentication Design**: When `include_key: true` is set, the request body **must** include the `password` field containing the current user's password. The export handler validates the password against the authenticated user's stored credentials before decrypting and returning the private key. If the password is missing or incorrect, the endpoint returns `403 Forbidden`. This prevents key exfiltration via stolen session tokens.

```json
// Example: export without key (no password required)
{
  "format": "pem",
  "include_key": false
}

// Example: export with key (password confirmation required)
{
  "format": "pem",
  "include_key": true,
  "password": "MyCurrentPassword123"
}
```

**Validation Response** (`POST /certificates/validate`):

```json
{
  "valid": true,
  "common_name": "*.example.com",
  "domains": ["*.example.com", "example.com"],
  "issuer_org": "DigiCert Inc",
  "expires_at": "2027-04-10T00:00:00Z",
  "key_match": true,
  "chain_valid": true,
  "chain_depth": 2,
  "warnings": ["Certificate expires in 365 days"],
  "errors": []
}
```

### 4.3 Service Layer Changes

#### Modified: `CertificateService` (`backend/internal/services/certificate_service.go`)

New/modified function signatures:

```go
// NewCertificateService — MODIFIED: add encryption service dependency
func NewCertificateService(dataDir string, db *gorm.DB, encSvc *crypto.EncryptionService) *CertificateService

// UploadCertificate — MODIFIED: accepts parsed content, validates, encrypts key
func (s *CertificateService) UploadCertificate(name string, certPEM string, keyPEM string, chainPEM string) (*CertificateInfo, error)

// GetCertificate — NEW: get single certificate detail by UUID
func (s *CertificateService) GetCertificate(uuid string) (*CertificateDetail, error)

// UpdateCertificate — NEW: update metadata (name)
func (s *CertificateService) UpdateCertificate(uuid string, name string) (*CertificateInfo, error)

// DeleteCertificate — MODIFIED: accept UUID instead of numeric ID
func (s *CertificateService) DeleteCertificate(uuid string) error

// IsCertificateInUse — MODIFIED: accept UUID instead of numeric ID
func (s *CertificateService) IsCertificateInUse(uuid string) (bool, error)

// ExportCertificate — NEW: export cert in requested format
func (s *CertificateService) ExportCertificate(uuid string, format string, includeKey bool) ([]byte, string, error)

// ValidateCertificate — NEW: validate without storing
func (s *CertificateService) ValidateCertificate(certPEM string, keyPEM string, chainPEM string) (*ValidationResult, error)

// GetDecryptedPrivateKey — NEW: internal only, decrypt key for Caddy/export
func (s *CertificateService) GetDecryptedPrivateKey(cert *models.SSLCertificate) (string, error)

// CheckExpiringCertificates — NEW: called by scheduler
func (s *CertificateService) CheckExpiringCertificates() ([]CertificateInfo, error)

// MigratePrivateKeys — NEW: one-time migration from plaintext to encrypted
func (s *CertificateService) MigratePrivateKeys() error
```

#### New: `CertificateValidator` (`backend/internal/services/certificate_validator.go`)

```go
// ParseCertificateInput handles PEM, PFX, and DER input parsing
func ParseCertificateInput(certData []byte, keyData []byte, chainData []byte, pfxPassword string) (*ParsedCertificate, error)

// ValidateKeyMatch checks that the private key matches the certificate public key
func ValidateKeyMatch(cert *x509.Certificate, key crypto.PrivateKey) error

// ValidateChain verifies the certificate chain from leaf to root.
// Uses x509.Certificate.Verify() with an intermediate cert pool to validate
// the chain against system roots (or provided root certificates).
func ValidateChain(leaf *x509.Certificate, intermediates []*x509.Certificate) error

// DetectFormat determines the certificate format from file content
func DetectFormat(data []byte) (string, error)

// ConvertDERToPEM converts DER-encoded certificate to PEM
func ConvertDERToPEM(derData []byte) (string, error)

// ConvertPFXToPEM extracts cert, key, and chain from PFX/PKCS12
func ConvertPFXToPEM(pfxData []byte, password string) (certPEM string, keyPEM string, chainPEM string, err error)

// ConvertPEMToPFX bundles cert, key, chain into PFX
func ConvertPEMToPFX(certPEM string, keyPEM string, chainPEM string, password string) ([]byte, error)

// ConvertPEMToDER converts PEM certificate to DER
func ConvertPEMToDER(certPEM string) ([]byte, error)

// ExtractCertificateMetadata extracts fingerprint, serial, issuer, key type, etc.
func ExtractCertificateMetadata(cert *x509.Certificate) *CertificateMetadata
```

### 4.4 Caddy Integration Changes

#### Modified: Config Generation (`backend/internal/caddy/config.go`)

The existing custom certificate loading logic (lines 418-453) needs modification to:

1. **Decrypt private keys** before passing to Caddy's `LoadPEM`
2. **Include certificate chain** in the `Certificate` field (full PEM chain)
3. **Add TLS automation policy** with `skip` for custom cert domains (prevent ACME from trying to issue for those domains)

Updated custom cert loading block:

```go
for _, cert := range customCerts {
    if cert.Certificate == "" || cert.PrivateKeyEncrypted == "" {
        logger.Log().WithField("cert", cert.Name).Warn("Custom certificate missing data, skipping")
        continue
    }

    decryptedKey, err := encSvc.Decrypt(cert.PrivateKeyEncrypted)
    if err != nil {
        logger.Log().WithError(err).WithField("cert", cert.Name).Warn("Failed to decrypt custom cert key, skipping")
        continue
    }

    fullCert := cert.Certificate
    if cert.CertificateChain != "" {
        fullCert = cert.Certificate + "\n" + cert.CertificateChain
    }

    loadPEM = append(loadPEM, LoadPEMConfig{
        Certificate: fullCert,
        Key:         string(decryptedKey),
        Tags:        []string{cert.UUID},
    })
}
```

Additionally, add a TLS automation policy that skips ACME for custom cert domains:

```go
if len(customCertDomains) > 0 {
    tlsPolicies = append(tlsPolicies, &AutomationPolicy{
        Subjects:   customCertDomains,
        IssuersRaw: nil,
    })
}
```

### 4.5 Encryption Strategy

**Private Key Encryption at Rest**:

1. On upload: `encSvc.Encrypt([]byte(keyPEM))` stores in `PrivateKeyEncrypted`
2. On Caddy config generation: `encSvc.Decrypt(cert.PrivateKeyEncrypted)` passes decrypted PEM to Caddy
3. On export: `encSvc.Decrypt(cert.PrivateKeyEncrypted)` converts to requested format
4. `KeyVersion` tracks which encryption key version was used (for rotation via `RotationService`)

**Migration**: Existing certificates with plaintext `PrivateKey` will be migrated to encrypted form during application startup if `CHARON_ENCRYPTION_KEY` is set. The migration:

- Reads `private_key` column
- Encrypts with current key
- Writes to `private_key_enc` column
- Sets `key_version = 1`
- Clears `private_key` column
- Logs migration progress

### 4.6 Certificate Format Handling

| Input Format | Detection | Processing |
|-------------|-----------|------------|
| **PEM** | Trial parse: `pem.Decode` succeeds | Direct parse via `pem.Decode` + `x509.ParseCertificate` |
| **PFX/PKCS12** | Trial parse: if PEM fails, attempt `pkcs12.Decode` | `pkcs12.Decode(pfxData, password)` then extract cert, key, chain and store as PEM |
| **DER** | Trial parse: if PEM and PFX fail, attempt `x509.ParseCertificate(raw)` | `x509.ParseCertificate(derBytes)` then convert to PEM for storage |

**Detection strategy**: Use trial-parse (not magic bytes). Try PEM decode first → if that fails, try PFX/PKCS12 decode → if that also fails, try raw DER parse via `x509.ParseCertificate`. This is more reliable than magic byte sniffing, especially for DER which shares ASN.1 structure with PFX.

**Dependencies**: Use `software.sslmate.com/src/go-pkcs12` for PFX handling (widely used, maintained).

### 4.7 Expiry Warning Scheduler

Add a background goroutine in `CertificateService` that runs daily:

```go
func (s *CertificateService) StartExpiryChecker(ctx context.Context, notificationSvc *NotificationService, warningDays int) {
    // Startup delay: avoid notification bursts during frequent restarts
    startupDelay := 5 * time.Minute
    select {
    case <-ctx.Done():
        return
    case <-time.After(startupDelay):
    }

    // Add random jitter (0-60 minutes) to stagger checks across instances/restarts
    jitter := time.Duration(rand.Int63n(int64(60 * time.Minute)))
    select {
    case <-ctx.Done():
        return
    case <-time.After(jitter):
    }

    s.checkExpiry(notificationSvc, warningDays)

    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkExpiry(notificationSvc, warningDays)
        }
    }
}
```

**Configuration**: `warningDays` is read from `CHARON_CERT_EXPIRY_WARNING_DAYS` environment variable at startup (default: `30`). The startup wiring reads the config value and passes it to `StartExpiryChecker`.

The checker:

1. Queries all custom certificates
2. For certs expiring within `warningDays` days: create warning notification + send external notification (rate-limited per cert per 24h)
3. For expired certs: update status to "expired" + send critical notification

---

## 5. Frontend Design

### 5.1 New/Modified Components

| Component | Type | Path | Description |
|-----------|------|------|-------------|
| `CertificateUploadDialog` | Modified | `frontend/src/components/dialogs/CertificateUploadDialog.tsx` | Extract from `Certificates.tsx`; add drag-and-drop, format detection, chain file, PFX password, validation preview |
| `CertificateDetailDialog` | New | `frontend/src/components/dialogs/CertificateDetailDialog.tsx` | Full metadata view, chain visualization, assigned hosts list, export button |
| `CertificateExportDialog` | New | `frontend/src/components/dialogs/CertificateExportDialog.tsx` | Format selector (PEM/PFX/DER), include-key toggle, PFX password field |
| `CertificateValidationPreview` | New | `frontend/src/components/CertificateValidationPreview.tsx` | Shows parsed cert info before upload confirmation |
| `CertificateChainViewer` | New | `frontend/src/components/CertificateChainViewer.tsx` | Visual chain display (leaf then intermediate then root) |
| `FileDropZone` | New | `frontend/src/components/ui/FileDropZone.tsx` | Reusable drag-and-drop file upload component |
| `CertificateList` | Modified | `frontend/src/components/CertificateList.tsx` | Add detail view button, export button, expiry warning badges, use UUID for actions |

### 5.2 API Client Updates (`frontend/src/api/certificates.ts`)

```typescript
export interface Certificate {
  uuid: string
  name?: string
  common_name?: string
  domains: string
  issuer: string
  issuer_org?: string
  fingerprint?: string
  serial_number?: string
  key_type?: string
  expires_at: string
  not_before?: string
  status: 'valid' | 'expiring' | 'expired' | 'untrusted'
  provider: string
  chain_depth?: number
  has_key: boolean
  in_use: boolean
}

export interface CertificateDetail extends Certificate {
  assigned_hosts: { uuid: string; name: string; domain_names: string }[]
  chain: { subject: string; issuer: string; expires_at: string }[]
  auto_renew: boolean
  created_at: string
  updated_at: string
}

export interface ValidationResult {
  valid: boolean
  common_name: string
  domains: string[]
  issuer_org: string
  expires_at: string
  key_match: boolean
  chain_valid: boolean
  chain_depth: number
  warnings: string[]
  errors: string[]
}

export async function getCertificateDetail(uuid: string): Promise<CertificateDetail>
export async function uploadCertificate(
  name: string, certFile: File, keyFile?: File, chainFile?: File, pfxPassword?: string
): Promise<Certificate>
export async function updateCertificate(uuid: string, name: string): Promise<Certificate>
export async function deleteCertificate(uuid: string): Promise<void>
export async function exportCertificate(
  uuid: string, format: string, includeKey: boolean, pfxPassword?: string
): Promise<Blob>
export async function validateCertificate(
  certFile: File, keyFile?: File, chainFile?: File, pfxPassword?: string
): Promise<ValidationResult>
```

### 5.3 Hook Updates (`frontend/src/hooks/useCertificates.ts`)

```typescript
export function useCertificates(options?: UseCertificatesOptions)
export function useCertificateDetail(uuid: string | null)
export function useUploadCertificate()
export function useUpdateCertificate()
export function useDeleteCertificate()
export function useExportCertificate()
export function useValidateCertificate()
```

### 5.4 Upload Flow UX

1. User clicks "Add Certificate"
2. **Upload Dialog** opens with:
   - Name input field
   - File drop zones (certificate file, key file, optional chain file)
   - Auto-format detection on file drop/select (show detected format badge: PEM/PFX/DER)
   - If PFX detected: show password field, hide key file input
   - "Validate" button calls `/certificates/validate` and shows `CertificateValidationPreview`
3. Validation preview shows: CN, SANs, issuer, expiry, chain depth, key-match status, warnings
4. User confirms and submits to `POST /certificates`
5. On success: toast + refresh list + close dialog

### 5.5 Expiry Warning Display

- Certificates expiring in 30 days or less: yellow warning badge + tooltip with days remaining
- Expired certificates: red expired badge
- The existing `status` field already provides `"expiring"` and `"expired"` values — the UI enhancement adds visual prominence

---

## 6. Security Considerations

### 6.1 Private Key Encryption

- **🔴 ACTIVE VULNERABILITY FIX**: The current Upload handler (`certificate_handler.go:137`) returns `c.JSON(http.StatusCreated, cert)` where `cert` is the full `*SSLCertificate` struct. Because `PrivateKey` currently has `json:"private_key"`, the raw PEM private key is disclosed to the client in every upload response. **Commit 1 fixes this** by changing the tag to `json:"-"`, immediately closing this private key disclosure vulnerability.
- All private keys encrypted at rest using AES-256-GCM
- Encryption uses the same `CHARON_ENCRYPTION_KEY` and rotation infrastructure as DNS provider credentials
- Keys are decrypted only in-memory when needed (Caddy reload, export)
- The `PrivateKey` field is hidden from JSON serialization (`json:"-"`) and excluded from GORM queries (`gorm:"-"`)
- The `PrivateKeyEncrypted` field is also hidden from JSON (`json:"-"`)

### 6.2 File Upload Security

- Maximum file size: 1MB per file (enforced in handler)
- File content validated (must parse as valid certificate/key/PFX)
- No path traversal risk: files are read into memory, never written to arbitrary paths
- Content-Type and extension validation (`.pem`, `.crt`, `.cer`, `.key`, `.pfx`, `.p12`, `.der`)
- PFX password is not stored; used only during parsing

### 6.3 GORM Model Security

- `SSLCertificate.ID` uses `json:"-"` (numeric ID hidden)
- `SSLCertificate.Certificate` uses `json:"-"` (PEM content hidden from list)
- `SSLCertificate.PrivateKey` uses `json:"-"` (transient, not persisted)
- `SSLCertificate.PrivateKeyEncrypted` uses `json:"-"` (encrypted, hidden)
- All API endpoints use UUID for identification
- `CertificateInfo` no longer exposes numeric `ID`

### 6.4 Export Security

- Export endpoint requires authentication (existing middleware)
- `include_key: true` requires **password re-confirmation** — the user must supply their current password in the request body; the handler validates it before decrypting the key (implements R-EXP-04)
- PFX export requires a password (enforced)
- Audit log entry for key exports (via notification service)

---

## 7. Implementation Phases (Tasks)

### Phase 1: Backend Foundation — Model, Encryption, Validation (Commit 1)

| # | Task | File(s) | Size | Description |
|---|------|---------|------|-------------|
| 1.1 | Add new fields to SSLCertificate model | `backend/internal/models/ssl_certificate.go` | S | Add `CommonName`, `CertificateChain`, `PrivateKeyEncrypted`, `KeyVersion`, `Fingerprint`, `SerialNumber`, `IssuerOrg`, `KeyType`, `NotBefore`. Hide sensitive fields from JSON. |
| 1.2 | Update AutoMigrate | `backend/internal/api/routes/routes.go` | S | Already migrates `SSLCertificate`; GORM auto-adds new columns. |
| 1.3 | Create certificate validator | `backend/internal/services/certificate_validator.go` | L | `ParseCertificateInput()`, `ValidateKeyMatch()`, `ValidateChain()`, `DetectFormat()`, format conversion functions. |
| 1.4 | Add `go-pkcs12` dependency | `backend/go.mod` | S | `go get software.sslmate.com/src/go-pkcs12` |
| 1.5 | Write private key migration function | `backend/internal/services/certificate_service.go` | M | `MigratePrivateKeys()` — encrypts existing plaintext keys. |
| 1.6 | Modify `UploadCertificate()` | `backend/internal/services/certificate_service.go` | L | Full validation pipeline, encrypt key, store chain, extract metadata. |
| 1.7 | Add `GetCertificate()` | `backend/internal/services/certificate_service.go` | M | Get single cert by UUID with full detail (assigned hosts, chain). |
| 1.8 | Add `ValidateCertificate()` | `backend/internal/services/certificate_service.go` | M | Validate without storing. |
| 1.9 | Modify `DeleteCertificate()` | `backend/internal/services/certificate_service.go` | S | Accept UUID instead of numeric ID. |
| 1.10 | Add `ExportCertificate()` | `backend/internal/services/certificate_service.go` | M | Decrypt key, convert to requested format. |
| 1.11 | Add `GetDecryptedPrivateKey()` | `backend/internal/services/certificate_service.go` | S | Internal decrypt helper. |
| 1.12 | Update `CertificateInfo` | `backend/internal/services/certificate_service.go` | S | Remove numeric ID, add new metadata fields. |
| 1.13 | Update `refreshCacheFromDB()` | `backend/internal/services/certificate_service.go` | M | Populate new fields (fingerprint, chain depth, has_key, in_use). |
| 1.14 | Add constructor changes | `backend/internal/services/certificate_service.go` | S | Accept `*crypto.EncryptionService` in `NewCertificateService`. |
| 1.15 | Unit tests for validator | `backend/internal/services/certificate_validator_test.go` | L | PEM/DER/PFX parsing, key match, chain validation, format detection. |
| 1.16 | Unit tests for upload | `backend/internal/services/certificate_service_test.go` | L | Upload with encryption, migration, export. |
| 1.17 | GORM security scan | — | S | Run `./scripts/scan-gorm-security.sh --check` on new model fields. |

### Phase 2: Backend API — Handlers, Routes, Caddy (Commit 2)

| # | Task | File(s) | Size | Description |
|---|------|---------|------|-------------|
| 2.1 | Update Upload handler | `backend/internal/api/handlers/certificate_handler.go` | L | Accept chain file, PFX password, detect format, call enhanced service. **Fix unsafe read**: replace `certSrc.Read(certBytes)` with `io.ReadAll(io.LimitReader(src, 1<<20))` for safe bounded reads (see Section 2.2 gaps). |
| 2.2 | Add Get handler | `backend/internal/api/handlers/certificate_handler.go` | M | `GET /certificates/:uuid` calls `GetCertificate()`. |
| 2.3 | Add Export handler | `backend/internal/api/handlers/certificate_handler.go` | M | `POST /certificates/:uuid/export` streams file download. |
| 2.4 | Add Update handler | `backend/internal/api/handlers/certificate_handler.go` | S | `PUT /certificates/:uuid` updates name. |
| 2.5 | Add Validate handler | `backend/internal/api/handlers/certificate_handler.go` | M | `POST /certificates/validate` validation-only endpoint. |
| 2.6 | Modify Delete handler | `backend/internal/api/handlers/certificate_handler.go` | S | Use UUID param instead of numeric ID. |
| 2.7 | Register new routes | `backend/internal/api/routes/routes.go` | S | Add new routes, pass encryption service. |
| 2.8 | Update Caddy config generation | `backend/internal/caddy/config.go` | M | Decrypt keys, include chains, skip ACME for custom cert domains. |
| 2.9 | Call migration on startup | `backend/internal/api/routes/routes.go` | S | Call `MigratePrivateKeys()` after service init. |
| 2.10 | Handler unit tests | `backend/internal/api/handlers/certificate_handler_test.go` | L | Test all new endpoints. |
| 2.11 | Caddy config tests | `backend/internal/caddy/config_test.go` | M | Update existing tests, add encrypted key test. |

### Phase 3: Expiry Warnings & Notifications (within Commit 2)

| # | Task | File(s) | Size | Description |
|---|------|---------|------|-------------|
| 3.1 | Add `CheckExpiringCertificates()` | `backend/internal/services/certificate_service.go` | M | Query custom certs expiring in 30 days or less. |
| 3.2 | Add `StartExpiryChecker()` | `backend/internal/services/certificate_service.go` | M | Background goroutine, daily tick, rate-limited notifications. |
| 3.3 | Wire scheduler on startup | `backend/internal/api/routes/routes.go` | S | Start goroutine with context from server. |
| 3.4 | Unit tests for expiry checker | `backend/internal/services/certificate_service_test.go` | M | Mock time, verify notification calls. |

### Phase 4: Frontend — Enhanced Upload, Detail, Export (Commit 3)

| # | Task | File(s) | Size | Description |
|---|------|---------|------|-------------|
| 4.1 | Create `FileDropZone` component | `frontend/src/components/ui/FileDropZone.tsx` | M | Reusable drag-and-drop with format badge. |
| 4.2 | Create `CertificateUploadDialog` | `frontend/src/components/dialogs/CertificateUploadDialog.tsx` | L | Full upload dialog with validation preview, chain, PFX. |
| 4.3 | Create `CertificateValidationPreview` | `frontend/src/components/CertificateValidationPreview.tsx` | M | Parsed cert preview before upload. |
| 4.4 | Create `CertificateDetailDialog` | `frontend/src/components/dialogs/CertificateDetailDialog.tsx` | L | Full metadata, chain, assigned hosts, export action. |
| 4.5 | Create `CertificateChainViewer` | `frontend/src/components/CertificateChainViewer.tsx` | M | Visual chain display. |
| 4.6 | Create `CertificateExportDialog` | `frontend/src/components/dialogs/CertificateExportDialog.tsx` | M | Format + key options. |
| 4.7 | Update `CertificateList` | `frontend/src/components/CertificateList.tsx` | M | Add detail/export buttons, use UUID, expiry badges. |
| 4.8 | Refactor `Certificates` page | `frontend/src/pages/Certificates.tsx` | M | Use new dialog components. |
| 4.9 | Update API client | `frontend/src/api/certificates.ts` | M | New functions, updated types. |
| 4.10 | Update hooks | `frontend/src/hooks/useCertificates.ts` | M | New hooks for detail, export, validate, update. |
| 4.11 | Add translations | `frontend/src/locales/en/translation.json` (+ other locales) | S | New keys for chain, export, validation messages. |
| 4.12 | Frontend unit tests | `frontend/src/components/__tests__/` | L | Tests for new components. |
| 4.13 | Vitest coverage | — | M | Ensure 85% coverage on new code. |

### Phase 5: E2E Tests & Hardening (Commit 4)

| # | Task | File(s) | Size | Description |
|---|------|---------|------|-------------|
| 5.1 | E2E: Certificate upload flow | `tests/certificate-upload.spec.ts` | L | Upload PEM cert + key, validate preview, verify list. |
| 5.2 | E2E: Certificate detail view | `tests/certificate-detail.spec.ts` | M | Open detail dialog, verify metadata, chain view. |
| 5.3 | E2E: Certificate export | `tests/certificate-export.spec.ts` | M | Export PEM, verify download blob. |
| 5.4 | E2E: Certificate assignment | `tests/certificate-assignment.spec.ts` | M | Assign cert to proxy host, verify Caddy reload. |
| 5.5 | Update existing delete tests | `tests/certificate-delete.spec.ts` | S | Use UUID instead of numeric ID. |
| 5.6 | CodeQL scans | — | S | Run Go + JS security scans. |
| 5.7 | GORM security scan | — | S | Final scan on all model changes. |
| 5.8 | Update documentation | `docs/features.md`, `CHANGELOG.md` | S | Document new capabilities. |

---

## 8. Commit Slicing Strategy

### Decision: 1 PR with 5 logical commits

**Rationale**: Single feature = single PR. Charon is a self-hosted tool where users track merged PRs to know when features are available. Merging partial PRs (e.g., backend-only) creates false confidence that a feature is complete, leading to user-filed issues and discussions asking why the feature is missing or broken. A single PR ensures the feature ships atomically — users see one merge and get the full capability.

Each commit maps to an implementation phase; this keeps the diff reviewable by walking through commits sequentially while guaranteeing the feature is never partially deployed.

### Commit Structure

| Commit | Phase | Scope | Key Files |
|--------|-------|-------|-----------|
| **Commit 1** | Backend Foundation | Tasks 1.1–1.17 | `backend/internal/models/ssl_certificate.go`, `backend/internal/services/certificate_validator.go`, `backend/internal/services/certificate_service.go`, `backend/go.mod`, `backend/go.sum`, test files |
| **Commit 2** | Backend API + Caddy + Expiry | Tasks 2.1–2.11, 3.1–3.4 | `backend/internal/api/handlers/certificate_handler.go`, `backend/internal/api/routes/routes.go`, `backend/internal/caddy/config.go`, test files |
| **Commit 3** | Frontend | Tasks 4.1–4.13 | `frontend/src/` components, pages, API client, hooks, locales |
| **Commit 4** | E2E Tests & Hardening | Tasks 5.1–5.7 | `tests/` E2E specs, CodeQL/GORM scans |
| **Commit 5** | Documentation | Task 5.8 | `docs/features.md`, `CHANGELOG.md` |

### Commit Descriptions

#### Commit 1: Backend Foundation (Model + Validator + Encryption)
- SSLCertificate model with all new fields and correct JSON tags
- Certificate validator: PEM, DER, PFX parsing, key-cert match, chain validation
- Private key encryption/decryption via `CHARON_ENCRYPTION_KEY`
- Migration function for existing plaintext keys
- Unit tests with 85% coverage on new code
- GORM security scan clean

#### Commit 2: Backend API (Handlers + Routes + Caddy + Expiry Checker)
- Upload endpoint accepts PEM/PFX/DER with safe bounded reads
- Get/Export/Validate/Update endpoints (UUID-based)
- Delete uses UUID instead of numeric ID
- Caddy loads encrypted custom certs with chain support
- Expiry checker: background goroutine, daily tick, notifications for certs expiring within 30 days
- Handler and Caddy config unit tests

#### Commit 3: Frontend (Upload + Detail + Export + UI Enhancements)
- Enhanced upload dialog with drag-and-drop, format detection, chain file, PFX password
- Validation preview before upload
- Certificate detail dialog with chain viewer
- Export dialog with format selection and key password confirmation
- List uses UUID for all operations, expiry warning badges
- Vitest coverage at 85% on new components

#### Commit 4: E2E Tests & Hardening
- E2E tests covering upload, detail, export, assignment flows
- Existing delete tests updated for UUID
- CodeQL Go + JS scans clean (no HIGH/CRITICAL)
- GORM security scan clean

#### Commit 5: Documentation
- `docs/features.md` updated with certificate management capabilities
- `CHANGELOG.md` updated

### PR-Level Validation Gates

The PR is merged only when **all** of the following pass:

- [ ] All backend unit tests pass with 85% coverage on new code
- [ ] All frontend Vitest tests pass with 85% coverage on new code
- [ ] All E2E tests pass (Firefox, Chromium, WebKit)
- [ ] GORM security scan clean (`./scripts/scan-gorm-security.sh --check`)
- [ ] CodeQL Go + JS scans: no HIGH/CRITICAL findings
- [ ] staticcheck pass
- [ ] TypeScript check pass
- [ ] Local patch coverage report generated and reviewed
- [ ] Documentation updated

### Rollback

Revert the single PR. All changes are additive (new columns, new endpoints, new components). Reverting removes the feature atomically with no partial state left in production.

---

## 9. Testing Strategy

### 9.1 Backend Unit Tests

| Test File | Coverage |
|-----------|----------|
| `backend/internal/services/certificate_validator_test.go` | PEM/DER/PFX parsing, key match (RSA + ECDSA), chain validation (valid/invalid/self-signed), format detection, error cases |
| `backend/internal/services/certificate_service_test.go` | Upload (all formats), encryption/decryption, migration, list (with new fields), get detail, export (all formats), delete by UUID, expiry checker, cache invalidation |
| `backend/internal/api/handlers/certificate_handler_test.go` | All endpoints: upload (multipart), get, export (file download), validate, update, delete; error cases (invalid format, missing key, expired cert) |
| `backend/internal/caddy/config_test.go` | Custom cert with encrypted key, chain inclusion, ACME skip for custom cert domains |

### 9.2 Frontend Unit Tests

| Test File | Coverage |
|-----------|----------|
| `frontend/src/components/__tests__/FileDropZone.test.tsx` | Drag-and-drop, file selection, format detection badge |
| `frontend/src/components/__tests__/CertificateUploadDialog.test.tsx` | Full upload flow, PFX mode toggle, validation preview |
| `frontend/src/components/__tests__/CertificateDetailDialog.test.tsx` | Metadata display, chain viewer, export action |
| `frontend/src/components/__tests__/CertificateExportDialog.test.tsx` | Format selection, key toggle, PFX password |
| `frontend/src/components/__tests__/CertificateList.test.tsx` | Updated: UUID-based actions, expiry badges, detail button |
| `frontend/src/hooks/__tests__/useCertificates.test.ts` | New hooks: detail, export, validate |

### 9.3 E2E Playwright Tests

| Spec File | Scenarios |
|-----------|-----------|
| `tests/certificate-upload.spec.ts` | Upload PEM cert + key, validate preview, verify list |
| `tests/certificate-detail.spec.ts` | Open detail dialog, verify metadata, chain view |
| `tests/certificate-export.spec.ts` | Export PEM, verify download blob |
| `tests/certificate-assignment.spec.ts` | Assign cert to proxy host, verify Caddy reload |
| `tests/certificate-delete.spec.ts` | Updated: UUID-based deletion |
| `tests/certificate-bulk-delete.spec.ts` | Updated: UUID-based bulk deletion |

#### Negative / Error Scenarios (Commit 4)

| Spec File | Scenarios |
|-----------|-----------|
| `tests/certificate-upload-errors.spec.ts` | Mismatched key/cert upload (expect error), invalid file upload (non-cert file), expired cert upload (expect warning + accept), oversized file upload (expect 413) |
| `tests/certificate-export-auth.spec.ts` | Export with `include_key: true` flow — verify password confirmation required, verify incorrect password rejected, verify export without key does not require password |

### 9.4 Security Scans

- GORM security scan (`./scripts/scan-gorm-security.sh --check`) — after Phase 1
- CodeQL Go scan — after Phase 2
- CodeQL JS scan — after Phase 3
- Trivy container scan — after final build

---

## 10. Config/Infrastructure Changes

### 10.1 No Changes Required

| File | Reason |
|------|--------|
| `.gitignore` | Uploaded certificates stored in database, not on disk. Existing `/data/` ignore covers Caddy runtime data. |
| `codecov.yml` | Existing configuration covers `backend/` and `frontend/src/`. |
| `.dockerignore` | No new file types to ignore. |
| `Dockerfile` | `go-pkcs12` dependency is a Go module pulled during build automatically. |

### 10.2 Environment Variables

| Variable | Status | Description |
|----------|--------|-------------|
| `CHARON_ENCRYPTION_KEY` | Existing | Required for private key encryption. Already used for DNS provider credentials. |
| `CHARON_ENCRYPTION_KEY_NEXT` | Existing | Used during key rotation. Rotation service already handles re-encryption. |
| `CHARON_CERT_EXPIRY_WARNING_DAYS` | New (optional) | Override default 30-day warning threshold. Default: `30`. Wired into `StartExpiryChecker()` at startup — see Section 4.7. |

### 10.3 Database Migration

GORM `AutoMigrate` handles additive column changes automatically. The private key migration from plaintext to encrypted is a one-time startup operation handled in code (see section 4.5).

**Migration sequence**:

1. GORM adds new columns (`common_name`, `certificate_chain`, `private_key_enc`, `key_version`, `fingerprint`, `serial_number`, `issuer_org`, `key_type`, `not_before`)
2. `MigratePrivateKeys()` runs once: reads `private_key`, encrypts to `private_key_enc`, clears `private_key`
3. Subsequent starts skip migration (checks if any rows have `private_key` non-empty and `private_key_enc` empty)

---

## 11. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| **Private key migration fails mid-way** | Low | High | Migration is transactional per-row. Idempotent — can be re-run. Original `private_key` column kept until migration verified complete. |
| **`CHARON_ENCRYPTION_KEY` not set** | Medium | High | Graceful degradation: upload/export of custom certs disabled when key not set. Clear error message in UI. ACME certs unaffected. |
| **PFX parsing edge cases** | Medium | Medium | Use well-maintained `go-pkcs12` library. Comprehensive test suite with real-world PFX files. Fall back to descriptive error messages. |
| **Caddy reload failure with bad cert** | Low | High | Caddy config generation validates cert/key pairing before including in config. Caddy itself validates on load and reports errors. Rollback logic already exists in Caddy manager. |
| **Breaking API change (numeric ID to UUID)** | Medium | Medium | Frontend and backend changes in separate PRs but deployed together. No external API consumers currently (self-hosted tool). Existing E2E tests catch regressions. |
| **Performance impact of encryption/decryption** | Low | Low | AES-256-GCM is hardware-accelerated on modern CPUs. Only custom certs are encrypted (typically fewer than 10 per instance). Caddy config generation is not a hot path. |
| **Large file upload DoS** | Low | Medium | 1MB file size limit enforced in handler. Gin's `MaxMultipartMemory` also provides protection. |

---

## 12. Acceptance Criteria (Definition of Done)

- [ ] Can upload custom certificates in PEM, PFX, and DER formats
- [ ] Certificate and key are validated before acceptance (format, key match, chain)
- [ ] Private keys are encrypted at rest using `CHARON_ENCRYPTION_KEY`
- [ ] Certificate detail view shows full metadata (CN, SANs, issuer, chain, fingerprint)
- [ ] Certificates can be assigned to proxy hosts
- [ ] Caddy serves custom certificates for assigned domains
- [ ] Expiry warnings fire as in-app and external notifications at 30 days
- [ ] Certificates can be exported in PEM, PFX, and DER formats
- [ ] All API endpoints use UUID (no numeric ID exposure)
- [ ] 85% test coverage on all new backend and frontend code
- [ ] E2E tests pass for upload, detail, export, assignment flows
- [ ] GORM security scan reports zero CRITICAL/HIGH findings
- [ ] CodeQL scans report zero HIGH/CRITICAL findings
- [ ] No plaintext private keys in database after migration

---

## Root Cause Analysis: E2E Certificate Test Failures (PR #928)

**Date**: 2026-06-24
**Scope**: 4 failing tests in `tests/core/certificates.spec.ts`
**Branch**: `feature/beta-release`

### Failing Tests

| # | Test Name | Line | Test Describe Block |
|---|-----------|------|---------------------|
| 1 | should validate required name field | L349 | Upload Dialog |
| 2 | should require certificate file | L375 | Upload Dialog |
| 3 | should require private key file | L400 | Upload Dialog |
| 4 | should reject empty friendly name | L776 | Form Validation |

---

### Root Cause Summary

There are **two layers** of failure. Layer 1 is the primary blocker in CI. Layer 2 contains test-logic defects that would surface even after Layer 1 is resolved.

#### Layer 1: Infrastructure — Disabled Submit Button Blocks All Validation Tests

**Classification**: Test Issue
**Severity**: CRITICAL — blocks all 4 tests

**Mechanism**:

The `CertificateUploadDialog` submit button is governed by:

```tsx
// frontend/src/components/dialogs/CertificateUploadDialog.tsx
const canSubmit = !!certFile && !!name.trim()

<Button type="submit" disabled={!canSubmit} ...>
```

All 4 failing tests attempt to click the Upload button **without providing a certificate file** (and tests 1/4 also leave the name empty). When `canSubmit` is `false`, the button renders with `disabled` attribute.

Playwright's `click()` performs actionability checks by default — it waits for the element to become **enabled** before clicking. On a disabled button, this waits until the default timeout (30s), then fails with a timeout error. The HTML5 form validation the tests expect to trigger **never fires** because the form is never submitted.

**Affected Tests**: All 4.

| Test | Name Empty | CertFile Null | canSubmit | Button State |
|------|-----------|---------------|-----------|--------------|
| 1 — validate required name | Yes | Yes | `false` | **Disabled** |
| 2 — require certificate file | No | Yes | `false` | **Disabled** |
| 3 — require private key file | N/A | N/A | N/A | **Disabled** |
| 4 — reject empty name | Yes | Yes | `false` | **Disabled** |

#### Layer 2a: `required` vs `aria-required` Attribute Mismatch

**Classification**: Test Issue
**Severity**: HIGH — tests 2 and 3 would fail even with a clickable button

**Mechanism**:

Tests 2 and 3 check for the native HTML `required` attribute on file inputs:

```ts
// tests/core/certificates.spec.ts L388
const certFileInput = dialog.locator('#cert-file');
const isRequired = await certFileInput.getAttribute('required');
expect(isRequired !== null).toBeTruthy();
```

But `FileDropZone` only sets `aria-required`, **not** the native `required` attribute:

```tsx
// frontend/src/components/ui/FileDropZone.tsx L102-L113
<input
  ref={inputRef}
  id={id}
  type="file"
  accept={accept}
  className="sr-only"
  aria-required={required}   // ← sets aria-required, NOT required
  tabIndex={-1}
/>
```

`getAttribute('required')` returns `null` → assertion `isRequired !== null` → `false` → test fails.

#### Layer 2b: Key File Not Required

**Classification**: Test Issue + Design Mismatch
**Severity**: HIGH — test 3 asserts a requirement that doesn't exist

**Mechanism**:

Test 3 expects `#key-file` to have a `required` attribute, but the key file `FileDropZone` is intentionally **not required**:

```tsx
// frontend/src/components/dialogs/CertificateUploadDialog.tsx L153-L161
<FileDropZone
  id="key-file"
  label={t('certificates.privateKeyFile')}
  accept=".pem,.key"
  file={keyFile}
  onFileChange={(f) => { ... }}
  // ← NO required prop
/>
```

The backend also treats `key_file` as optional:

```go
// backend/internal/api/handlers/certificate_handler.go ~L80
keyFileHeader, _ := c.FormFile("key_file") // optional
```

This is by design: PFX certificates bundle the private key, so a separate key file is not always needed. The test incorrectly assumes the key file is always required.

#### Layer 2c: Always-True Assertion (Test 1)

**Classification**: Test Issue (minor)
**Severity**: LOW — the assertion is meaningless

```ts
// tests/core/certificates.spec.ts L365
expect(isInvalid || true).toBeTruthy(); // always passes
```

The `|| true` makes this assertion vacuous. It should be:
```ts
expect(isInvalid).toBeTruthy();
```

---

### Files Involved

| File | Role | Issue |
|------|------|-------|
| [tests/core/certificates.spec.ts](tests/core/certificates.spec.ts) | E2E test file | All 4 failing tests |
| [frontend/src/components/dialogs/CertificateUploadDialog.tsx](frontend/src/components/dialogs/CertificateUploadDialog.tsx) | Upload form | Disabled button prevents form submission; key file not required |
| [frontend/src/components/ui/FileDropZone.tsx](frontend/src/components/ui/FileDropZone.tsx) | File input wrapper | Uses `aria-required` not `required` HTML attribute |
| [frontend/src/components/ui/Input.tsx](frontend/src/components/ui/Input.tsx) | Text input | Passes `required` to native `<input>` via spread props (correct) |
| [backend/internal/api/handlers/certificate_handler.go](backend/internal/api/handlers/certificate_handler.go) | Upload handler | `key_file` is optional — consistent with frontend |

---

### Remediation Plan

**Approach**: Fix the tests to match the actual (correct) frontend/backend behavior.
The frontend's disabled-button pattern and optional key file are correct design choices; the tests are wrong.

#### Commit 1: Fix validation tests to work with disabled submit button

**Scope**: `tests/core/certificates.spec.ts`

**Changes for Test 1 ("should validate required name field", L349)**:
- Remove the disabled-button click approach
- Instead, verify that the submit button is disabled when name is empty
- Test that the `required` attribute exists on the name `<input>` element
- Remove the vacuous `|| true` assertion

```ts
test('should validate required name field', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Verify submit is disabled with empty name', async () => {
    const dialog = page.getByRole('dialog');
    const nameInput = dialog.locator('#certificate-name');
    const submitButton = dialog.getByTestId('upload-certificate-submit');

    // Name input should have HTML5 required attribute
    await expect(nameInput).toHaveAttribute('required', '');

    // Submit button should be disabled when name is empty
    await expect(submitButton).toBeDisabled();
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

**Changes for Test 2 ("should require certificate file", L375)**:
- Replace `getAttribute('required')` with `getAttribute('aria-required')`
- Verify submit button is disabled without a cert file
- Remove the disabled-button click

```ts
test('should require certificate file', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Verify cert file is required', async () => {
    const dialog = page.getByRole('dialog');
    const nameInput = dialog.locator('#certificate-name');
    await nameInput.fill('Test Certificate');

    // FileDropZone uses aria-required, not HTML required
    const certFileInput = dialog.locator('#cert-file');
    await expect(certFileInput).toHaveAttribute('aria-required', 'true');

    // Submit should remain disabled without cert file
    const submitButton = dialog.getByTestId('upload-certificate-submit');
    await expect(submitButton).toBeDisabled();
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

**Changes for Test 3 ("should require private key file", L400)**:
- The key file is intentionally optional — rewrite to test that it is optional and
  only shown for non-PFX formats
- OR remove this test entirely and replace with a test that validates the key file
  is present but optional

```ts
test('should show optional private key file field', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Verify private key field is visible but optional', async () => {
    const dialog = page.getByRole('dialog');
    const keyFileInput = dialog.locator('#key-file');
    await expect(keyFileInput).toBeVisible();

    // Key file is optional - should NOT have aria-required="true"
    const ariaRequired = await keyFileInput.getAttribute('aria-required');
    expect(ariaRequired).not.toBe('true');
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

**Changes for Test 4 ("should reject empty friendly name", L776)**:
- Replace button click with submit-button disabled check

```ts
test('should reject empty friendly name', async ({ page }) => {
  await test.step('Verify upload blocked with empty name', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);

    const dialog = page.getByRole('dialog');
    const submitButton = dialog.getByTestId('upload-certificate-submit');

    // Submit should be disabled with empty name
    await expect(submitButton).toBeDisabled();

    // Dialog should remain open
    await expect(dialog).toBeVisible();

    await getCancelButton(page).click();
  });
});
```

#### Commit 2: (Optional) Add `required` HTML attribute to FileDropZone for native validation

**Scope**: `frontend/src/components/ui/FileDropZone.tsx`
**Decision**: Consider adding the native `required` attribute alongside `aria-required` for defense-in-depth. This is a frontend enhancement, not a test fix.

```tsx
<input
  ref={inputRef}
  id={id}
  type="file"
  accept={accept}
  className="sr-only"
  aria-required={required}
  required={required}          // ← ADD native required
  tabIndex={-1}
/>
```

**Impact**: Minimal — the hidden file input is `sr-only` and never focused by users directly. Adding `required` provides native HTML5 validation as a fallback if the form's JS validation is bypassed.

---

### Validation Gates

- [ ] All 4 tests pass in Firefox (default E2E browser)
- [ ] All 4 tests pass in Chromium
- [ ] No regression in other certificate tests (upload, delete, export, list)
- [ ] FileDropZone unit tests updated if Commit 2 is applied

### Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Other tests depend on `getAttribute('required')` pattern | Low | Grep for `getAttribute('required')` across all test files |
| Disabled-button check is too permissive | Low | Tests still verify the submit guard logic is correct |
| Key file optionality confuses users | Medium | UX already shows key file section only for non-PFX; label doesn't say "required" |

---

## Appendix A: CI Test Failure Fix Plan (PR #928)

**Date**: 2026-04-13
**Branch**: `feature/beta-release`
**Trigger**: Two backend test failures blocking CI on PR #928.

---

### Failure 1: `TestCertificateHandler_Upload_MissingKeyFile_MultipartWithCert`

**File**: `backend/internal/api/handlers/certificate_handler_test.go` line 396
**Symptom**: Test expects a 400 response mentioning `key_file`, but receives `{"error":"failed to parse certificate input: failed to parse certificate PEM: failed to parse certificate: x509: malformed certificate"}`.

#### Root Cause

Two compounding issues:

1. **Malformed test fixture**: The test sends a dummy cert (`-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----`) that is not a valid X.509 certificate. The `UploadCertificate` service method immediately calls `ParseCertificateInput()` → `parsePEMInput()` → `parsePEMCertificates()`, which fails on the malformed data before any key presence check runs.

2. **Missing key_file validation**: The handler (line 127) treats `key_file` as optional and passes `keyPEM=""` to the service without complaint. The service's `UploadCertificate()` (line 434) calls `ParseCertificateInput()` which also treats a missing key as acceptable (PFX embeds keys). **There is no validation anywhere that requires `key_file` for PEM/DER uploads.** Even with a valid cert, this test would fail because the upload would succeed (201) instead of returning 400.

#### Required Changes

| # | File | Location | Change |
|---|------|----------|--------|
| 1a | `backend/internal/api/handlers/certificate_handler.go` | After line ~152 (after chain file read, before `h.service.UploadCertificate` call) | Add key_file validation for non-PFX formats. Use `services.DetectFormat(certBytes)` to detect format. If format is `FormatPEM` or `FormatDER`, require `keyPEM != ""`. Return `400 Bad Request` with `{"error":"key_file is required for PEM/DER certificate uploads"}`. |
| 1b | `backend/internal/api/handlers/certificate_handler_test.go` | Line ~420 (the `part.Write` call) | Replace the malformed dummy cert with a valid self-signed certificate using the existing `generateSelfSignedCertPEM()` helper (already defined at line ~478 in the same file). Use only the cert portion; do not include the key file. |
| 1c | `backend/internal/api/handlers/certificate_handler_test.go` | Line ~432 (the `key_file` assertion) | Update the expected error substring from `"key_file"` to `"key_file is required"` to match the new handler message. |

**Handler change detail** (insert after chain file block, before `h.service.UploadCertificate`):

```go
// Require key_file for PEM/DER formats (PFX embeds the key)
if keyPEM == "" {
    format := services.DetectFormat(certBytes)
    if format == services.FormatPEM || format == services.FormatDER {
        c.JSON(http.StatusBadRequest, gin.H{"error": "key_file is required for PEM/DER certificate uploads"})
        return
    }
}
```

**Test change detail** (replace dummy cert with valid cert):

```go
certPEM, _, err := generateSelfSignedCertPEM()
if err != nil {
    t.Fatalf("failed to generate cert: %v", err)
}
part, createErr := writer.CreateFormFile("certificate_file", "cert.pem")
if createErr != nil {
    t.Fatalf("failed to create form file: %v", createErr)
}
_, _ = part.Write([]byte(certPEM))
```

---

### Failure 2: `TestCertificateService_MigratePrivateKeys/migrates_plaintext_key`

**File**: `backend/internal/services/certificate_service_coverage_test.go` line ~500
**Symptom**: `duplicate column name: private_key` when executing `ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''`.

#### Root Cause

Copy-paste / merge artifact. The "migrates plaintext key" subtest contains a **duplicate** `ALTER TABLE ADD COLUMN` block. The code at lines 496–497 executes the first ALTER TABLE (succeeds), then lines 499–501 repeat it verbatim (fails with "duplicate column name"). Line 499 also contains a corrupted comment: `// Insert cert with plaintext key using raw SQL{}))` — the `{}))` is leftover garbage from a bad merge.

**Current code (lines 496–504):**
```go
require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''").Error)

// Insert cert with plaintext key using raw SQL{}))
// MigratePrivateKeys uses raw SQL referencing private_key column (gorm:"-" tag)
require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''").Error)

// Insert cert with plaintext key using raw SQL
```

The `PrivateKey` field in the model has `gorm:"-"`, so `AutoMigrate` does not create the `private_key` column, meaning the first ALTER TABLE is required. The second is spurious.

#### Required Changes

| # | File | Location | Change |
|---|------|----------|--------|
| 2a | `backend/internal/services/certificate_service_coverage_test.go` | Lines ~499–501 | Delete the three duplicate/corrupted lines (the corrupted comment, the duplicate `gorm:"-"` comment, and the duplicate ALTER TABLE statement). |

**After fix, the block should read:**
```go
require.NoError(t, db.Exec("ALTER TABLE ssl_certificates ADD COLUMN private_key TEXT DEFAULT ''").Error)

// Insert cert with plaintext key using raw SQL
require.NoError(t, db.Exec(
```

---

### Commit Slicing Strategy

**Decision**: Single PR, single commit. Both fixes are small, scoped to test infrastructure + one handler validation guard, and have no cross-domain risk.

#### Commit 1: `fix(tests): resolve CI failures in certificate upload handler and migration tests`

| Aspect | Detail |
|--------|--------|
| **Scope** | 3 files — 1 handler source, 2 test files |
| **Files** | `backend/internal/api/handlers/certificate_handler.go`, `backend/internal/api/handlers/certificate_handler_test.go`, `backend/internal/services/certificate_service_coverage_test.go` |
| **Dependencies** | None — changes are self-contained |
| **Validation gate** | `go test ./backend/internal/api/handlers/ -run TestCertificateHandler_Upload_MissingKeyFile_MultipartWithCert -v` and `go test ./backend/internal/services/ -run TestCertificateService_MigratePrivateKeys -v` must both pass |
| **Regression check** | `go test ./backend/internal/api/handlers/ -run TestCertificateHandler_Upload` (all Upload tests) and `go test ./backend/internal/services/ -run TestCertificateService` (all service tests) |
| **Rollback** | Standard `git revert` — no schema or migration changes |

### Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| key_file validation breaks PFX upload flow | Low | The guard explicitly excludes `FormatPFX` via `DetectFormat()` |
| `generateSelfSignedCertPEM()` helper unavailable in test scope | None | Helper is already defined in the same test file (line ~478) |
| Other tests rely on key_file being optional | Low | Grep for handler Upload tests to confirm no other test sends cert-only |

---

## Amendment: Conditional Key File `aria-required` (PR #928 Review Comment)

**Source**: PR #928 review comment on `tests/core/certificates.spec.ts` lines 397–410.
**Date**: 2026-04-14
**Status**: Ready for implementation

> This E2E assertion treats the private key upload as always-optional, but the backend enforces `key_file` for PEM/DER uploads (only PFX can omit it). Once the UI is corrected to block PEM uploads without a key, this test should also be updated to reflect the conditional requirement (e.g., key required unless a .pfx/.p12 cert file is selected).

---

### A.1 Research Findings

#### A.1.1 Backend Enforcement (`backend/internal/api/handlers/certificate_handler.go`)

The `Upload` handler at `POST /api/v1/certificates` (registered in `backend/internal/api/routes/routes.go`) reads `key_file` as an **optional** multipart field initially, then enforces it conditionally:

```go
// Require key_file for non-PFX formats (PFX embeds the private key)
if keyPEM == "" {
    format := services.DetectFormat(certBytes)
    if format != services.FormatPFX {
        c.JSON(http.StatusBadRequest, gin.H{"error": "key_file is required for PEM/DER certificate uploads"})
        return
    }
}
```

Format detection (`backend/internal/services/certificate_validator.go`, `DetectFormat`) is **content-based** (trial-parse: PEM headers → PFX magic bytes → DER ASN.1 parse), not extension-based. The constants are:

| Constant | Value |
|----------|-------|
| `services.FormatPEM` | `"pem"` |
| `services.FormatDER` | `"der"` |
| `services.FormatPFX` | `"pfx"` |
| `services.FormatUnknown` | `"unknown"` |

Only `FormatPFX` bypasses the `key_file` requirement. All other formats (PEM, DER, Unknown) require it.

#### A.1.2 Frontend Component (`frontend/src/components/dialogs/CertificateUploadDialog.tsx`)

**State variables** (all in the `CertificateUploadDialog` function body):

| Variable | Type | Purpose |
|----------|------|---------|
| `certFile` | `File \| null` | Selected certificate file |
| `keyFile` | `File \| null` | Selected private key file |
| `chainFile` | `File \| null` | Selected chain/intermediate file |
| `name` | `string` | Friendly name text |
| `validationResult` | `ValidationResult \| null` | Server validation preview |

**Derived state** (computed directly, no `useMemo`):

| Variable | Expression | Meaning |
|----------|-----------|---------|
| `certFormat` | `detectFormat(certFile)` | `'PFX/PKCS#12'`, `'PEM'`, `'DER'`, `'KEY'`, or `null` |
| `isPfx` | `certFormat === 'PFX/PKCS#12'` | `true` when cert file has `.pfx` or `.p12` extension |
| `needsKeyFile` | `!!certFile && !isPfx && !keyFile` | `true` when cert selected, not PFX, but no key file |
| `canSubmit` | `!!certFile && !!name.trim() && !needsKeyFile` | Controls submit button disabled state |

**The `detectFormat` function** (module-level, not exported):

```ts
function detectFormat(file: File | null): string | null {
  if (!file) return null
  const ext = file.name.toLowerCase().split('.').pop()
  if (ext === 'pfx' || ext === 'p12') return 'PFX/PKCS#12'
  if (ext === 'pem' || ext === 'crt' || ext === 'cer') return 'PEM'
  if (ext === 'der') return 'DER'
  if (ext === 'key') return 'KEY'
  return null
}
```

**JSX structure** (relevant fragment):

```jsx
{!isPfx && (
  <>
    <FileDropZone
      id="key-file"
      label={t('certificates.privateKeyFile')}
      accept=".pem,.key"
      file={keyFile}
      onFileChange={(f) => {
        setKeyFile(f)
        setValidationResult(null)
      }}
      {/* ← required prop is ABSENT — this is the gap */}
    />
    <FileDropZone
      id="chain-file"
      ...
    />
  </>
)}
```

The key-file `FileDropZone` is **conditionally rendered** — it is completely absent from the DOM when `isPfx === true`.

#### A.1.3 `FileDropZone` Component (`frontend/src/components/ui/FileDropZone.tsx`)

**Props interface**:

```ts
interface FileDropZoneProps {
  id: string
  label: string
  accept?: string
  file: File | null
  onFileChange: (file: File | null) => void
  disabled?: boolean
  required?: boolean        // ← this prop exists
  formatBadge?: string | null
}
```

When `required={true}`, the component:
1. Renders a `*` asterisk next to the label: `{required && <span className="text-error ml-0.5" aria-hidden="true">*</span>}`
2. Sets `aria-required={required}` on the hidden `<input>` element: `<input ... aria-required={required} required={required} />`

The `<input>` element carries the ID used in Playwright locators (`#key-file`, `#cert-file`). It is `className="sr-only"` (visually hidden) but present in the DOM and accessible to Playwright's `setInputFiles()`.

#### A.1.4 The Gap

The key-file `FileDropZone` is **never passed `required={true}`**. As a result:

- `aria-required` is never `"true"` on `#key-file`
- The `*` asterisk never appears next to the "Private Key File" label
- Screen readers do not announce the field as required, even when a PEM/DER cert is selected
- The submit button is already disabled via `canSubmit` when `needsKeyFile` is `true`, but there is no accessible signal that the key file itself is the blocker

**The fix is a single-prop change** on the key-file `FileDropZone` in `CertificateUploadDialog.tsx`. No other frontend files require modification.

#### A.1.5 Current Playwright Test (`tests/core/certificates.spec.ts`, lines 397–410)

```ts
test('should show optional private key file field', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Verify private key field is visible but optional', async () => {
    const dialog = page.getByRole('dialog');
    const keyFileInput = dialog.locator('#key-file');
    await expect(keyFileInput).toBeVisible();

    // Key file is optional (PFX bundles the key) — should NOT be aria-required
    await expect(keyFileInput).not.toHaveAttribute('aria-required', 'true');
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

This test only covers the **default state** (no cert file selected). After the frontend fix, this test will still pass because `required={!!certFile}` evaluates to `false` when `certFile` is null. However, it is incomplete: it does not verify the conditional behavior when a cert file is selected.

---

### A.2 Requirements (EARS Notation)

| ID | Requirement |
|----|-------------|
| R-UI-KEY-01 | WHEN a user has not yet selected a certificate file, THE SYSTEM SHALL display the private key file input without `aria-required="true"`. |
| R-UI-KEY-02 | WHEN a user selects a PEM, DER, CRT, or CER certificate file, THE SYSTEM SHALL mark the private key file input with `aria-required="true"` and display a `*` indicator next to its label. |
| R-UI-KEY-03 | WHEN a user selects a PFX or P12 certificate file, THE SYSTEM SHALL hide the private key file input entirely (PFX bundles the private key). |
| R-UI-KEY-04 | WHILE the private key file input has `aria-required="true"` and no key file has been selected, THE SYSTEM SHALL keep the Upload submit button disabled. |
| R-UI-KEY-05 | THE SYSTEM SHALL ensure that assistive technology users receive the same conditional required-field signal as sighted users. |

---

### A.3 Phase 1: Frontend UI Fix

#### A.3.1 Scope

| Attribute | Value |
|-----------|-------|
| **File** | `frontend/src/components/dialogs/CertificateUploadDialog.tsx` |
| **Function** | `CertificateUploadDialog` (default export) |
| **Change type** | Single-prop addition to JSX |
| **Risk** | Minimal — `FileDropZone` already handles `required` prop |

#### A.3.2 Exact Change

**Location**: Inside the `{!isPfx && <> ... </>}` block, on the key-file `FileDropZone` element.

**Before**:

```tsx
<FileDropZone
  id="key-file"
  label={t('certificates.privateKeyFile')}
  accept=".pem,.key"
  file={keyFile}
  onFileChange={(f) => {
    setKeyFile(f)
    setValidationResult(null)
  }}
/>
```

**After**:

```tsx
<FileDropZone
  id="key-file"
  label={t('certificates.privateKeyFile')}
  accept=".pem,.key"
  file={keyFile}
  onFileChange={(f) => {
    setKeyFile(f)
    setValidationResult(null)
  }}
  required={!!certFile}
/>
```

**Why `!!certFile` and not `!!certFile && !isPfx`**: The key-file `FileDropZone` is only rendered inside the `{!isPfx && ...}` guard, so `!isPfx` is always `true` within that block. Using just `!!certFile` is semantically complete and avoids redundancy. The expression evaluates to:

| `certFile` | `isPfx` (outer guard) | Key-file rendered? | `required` value |
|------------|----------------------|-------------------|-----------------|
| `null` | `false` | Yes | `false` |
| `.pem` file | `false` | Yes | `true` |
| `.der` file | `false` | Yes | `true` |
| `.pfx` file | `true` | **No** (not rendered) | N/A |
| `.p12` file | `true` | **No** (not rendered) | N/A |

#### A.3.3 Downstream Effects

- `FileDropZone` with `required={true}` sets `aria-required="true"` on `<input id="key-file">` and renders `*` in the label — no other code changes needed.
- `needsKeyFile` and `canSubmit` are already correctly computed; submit stays disabled when `needsKeyFile` is `true`.
- The `needsKeyFile` error paragraph (`<p role="alert">`) already renders when `needsKeyFile` is `true` — this continues to work unchanged.
- No backend changes are needed. The backend enforcement is already correct.

---

### A.4 Phase 2: Playwright Test Update

#### A.4.1 Test File

`tests/core/certificates.spec.ts` — inside the `'Upload Custom Certificate'` describe block.

#### A.4.2 Replace the Existing Test

Replace the single test `'should show optional private key file field'` (lines ~397–418) with the three tests below.

**Test A — Default state (no cert file selected): key is visible but NOT required**

```ts
test('should show private key field as optional when no cert file is selected', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Verify private key field is visible but not aria-required', async () => {
    const dialog = page.getByRole('dialog');
    const keyFileInput = dialog.locator('#key-file');
    await expect(keyFileInput).toBeVisible();

    // No cert file selected — key file is not yet required
    await expect(keyFileInput).not.toHaveAttribute('aria-required', 'true');
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

**Rationale**: Preserves the original intent of the old test. When no cert is selected the field is optional/pending and must not carry `aria-required="true"`.

---

**Test B — PEM cert selected: key is `aria-required`**

```ts
test('should mark private key field as aria-required when a PEM cert file is selected', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Select a PEM certificate file', async () => {
    const dialog = page.getByRole('dialog');
    const certInput = dialog.locator('#cert-file');

    await certInput.setInputFiles({
      name: 'cert.pem',
      mimeType: 'application/x-pem-file',
      buffer: Buffer.from('-----BEGIN CERTIFICATE-----\nMIIBpDCCAQ2gAwIBAgIUtest\n-----END CERTIFICATE-----\n'),
    });
  });

  await test.step('Verify key file is now aria-required', async () => {
    const dialog = page.getByRole('dialog');
    const keyFileInput = dialog.locator('#key-file');

    // PEM cert selected → key file becomes required
    await expect(keyFileInput).toHaveAttribute('aria-required', 'true');

    // Submit must remain disabled without a key file
    const uploadButton = dialog.getByRole('button', { name: /upload/i });
    await expect(uploadButton).toBeDisabled();
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

**Notes on `setInputFiles`**: The `#cert-file` `<input>` is `className="sr-only"` (positioned off-screen, not `display:none`), so Playwright can interact with it directly. The buffer content does not need to be a valid certificate — only the **filename extension** (`cert.pem`) is used by `detectFormat()` to determine `certFormat`. The `.crt` and `.cer` extensions should produce the same result and can be added as parameterised variants if desired.

---

**Test C — PFX cert selected: key field is hidden**

```ts
test('should hide private key field entirely when a PFX cert file is selected', async ({ page }) => {
  await test.step('Open upload dialog', async () => {
    await getAddCertButton(page).click();
    await waitForDialog(page);
  });

  await test.step('Select a PFX certificate file', async () => {
    const dialog = page.getByRole('dialog');
    const certInput = dialog.locator('#cert-file');

    await certInput.setInputFiles({
      name: 'bundle.pfx',
      mimeType: 'application/x-pkcs12',
      buffer: Buffer.from('PFX placeholder'),
    });
  });

  await test.step('Verify key file input is not in the DOM', async () => {
    const dialog = page.getByRole('dialog');
    const keyFileInput = dialog.locator('#key-file');

    // PFX bundles the private key — key-file section is not rendered
    await expect(keyFileInput).not.toBeVisible();

    // PFX hint text should appear
    const pfxHint = dialog.getByText(/pfx|pkcs.?12|bundles/i);
    await expect(pfxHint).toBeVisible();
  });

  await test.step('Close dialog', async () => {
    await getCancelButton(page).click();
  });
});
```

**Notes**: `not.toBeVisible()` is the correct assertion when `#key-file` is **not rendered** (as opposed to rendered-but-hidden). Playwright treats a non-existent element as not visible. If the `pfxDetected` i18n key produces text like "This PFX file bundles the private key", the regex `/pfx|pkcs.?12|bundles/i` will match; adjust the pattern to match the actual translation string if needed.

---

#### A.4.3 Additional Tests to Consider (Non-Blocking)

| Test | Scenario | Value |
|------|----------|-------|
| `should clear aria-required when cert file is removed` | User selects `.pem` then clears it | Regression guard for reset path |
| `should accept .crt and .cer as non-PFX cert files` | Parameterised over `['cert.crt', 'cert.cer', 'cert.der']` | Extension coverage |
| `should accept .p12 as equivalent to .pfx` | Selects `bundle.p12` | P12 alias coverage |

These are optional for the PR but recommended for completeness.

---

### A.5 Acceptance Criteria

| ID | Criterion | Verification |
|----|-----------|-------------|
| AC-01 | `#key-file` input has `aria-required="true"` after a `.pem` cert file is selected | Playwright Test B passes |
| AC-02 | `#key-file` input does NOT have `aria-required="true"` before any cert file is selected | Playwright Test A passes |
| AC-03 | `#key-file` input is not in the DOM (not rendered) after a `.pfx` cert file is selected | Playwright Test C passes |
| AC-04 | `*` asterisk appears next to "Private Key File" label when a non-PFX cert is selected | Visual inspection / accessibility snapshot |
| AC-05 | Upload submit button remains disabled while `aria-required="true"` and no key file is provided | Verified in Playwright Test B step 3 |
| AC-06 | Screen reader announces "Private Key File, required" after PEM cert is selected | Manual NVDA/VoiceOver test |
| AC-07 | All existing certificate upload tests continue to pass | Full Playwright `certificates.spec.ts` run |
| AC-08 | TypeScript compilation has no errors | `tsc --noEmit` in `frontend/` |

---

### A.6 Commit Slicing Strategy

**Decision**: Single PR, two logically ordered commits. The frontend change and the test update are independent but sequenced for reviewer clarity — the test commit documents the expected behavior that the component commit implements.

#### Commit 1: Frontend conditional key requirement

| Aspect | Detail |
|--------|--------|
| **Message** | `fix(certificates): mark key file as aria-required for PEM/DER cert uploads` |
| **Scope** | 1 file |
| **File** | `frontend/src/components/dialogs/CertificateUploadDialog.tsx` |
| **Change** | Add `required={!!certFile}` prop to the key-file `FileDropZone` element |
| **Dependencies** | None — `FileDropZone` already supports the `required` prop |
| **Validation gate** | `tsc --noEmit` in `frontend/` passes; `npm run lint` passes |
| **Regression check** | Verify `#key-file` has no `aria-required` attr in default state; verify it has `aria-required="true"` after `setInputFiles` with a `.pem` file (manual browser check or existing Playwright run) |

#### Commit 2: Playwright test update

| Aspect | Detail |
|--------|--------|
| **Message** | `test(certificates): verify conditional aria-required on key file input` |
| **Scope** | 1 file |
| **File** | `tests/core/certificates.spec.ts` |
| **Change** | Replace `'should show optional private key file field'` (lines ~397–418) with Tests A, B, and C defined in §A.4.2 |
| **Dependencies** | Commit 1 must be merged first; Test B would fail against unpatched component |
| **Validation gate** | `npx playwright test tests/core/certificates.spec.ts --project=firefox` — all tests in `'Upload Custom Certificate'` describe block pass |
| **Regression check** | Full `certificates.spec.ts` run; no regressions in `'List View'` or `'Delete Certificate'` suites |
| **Rollback** | `git revert` both commits — no schema or infra changes |

#### Rollback Notes

Both commits touch frontend source and test files only. No backend changes, no database migrations, no Docker image changes. A revert of both commits is sufficient to restore prior behavior.

---

### A.7 Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `setInputFiles` fails on `sr-only` hidden input | Low | Test B/C never run | Input is off-screen via `clip`/`position:absolute`, not `display:none` — Playwright handles this. Verify with `expect(input).toBeAttached()` if needed. |
| `detectFormat` returns null for edge extensions | Low | `required` stays `false` (safe default) | Unknown extensions produce `certFormat = null`, `isPfx = false`, `required = !!certFile = true` — so unknown extensions default to requiring a key file, which matches backend behavior. |
| PFX hint text translation key mismatch | Low | Test C flakiness on `pfxHint` assertion | Use a broader regex or locate by `data-testid` if needed. |
| Existing test `'should show optional private key file field'` conflicts | None | N/A | It is fully replaced by Test A; rename ensures no duplicate test titles. |
