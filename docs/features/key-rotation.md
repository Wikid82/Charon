---
title: Encryption Key Rotation
description: Complete guide to rotating encryption keys for DNS provider credentials with zero downtime
---

# Encryption Key Rotation

Charon provides **automated encryption key rotation** for DNS provider credentials with zero downtime. This enterprise-grade feature allows administrators to rotate encryption keys periodically to meet security and compliance requirements while maintaining uninterrupted service.

## Table of Contents

- [Overview](#overview)
- [Why Key Rotation Matters](#why-key-rotation-matters)
- [Key Management Concepts](#key-management-concepts)
- [Accessing Key Management](#accessing-key-management)
- [Understanding Key Status](#understanding-key-status)
- [Rotating Encryption Keys](#rotating-encryption-keys)
- [Validating Key Configuration](#validating-key-configuration)
- [Viewing Rotation History](#viewing-rotation-history)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [API Reference](#api-reference)

---

## Overview

### What is Key Rotation?

Key rotation is the process of replacing encryption keys used to protect sensitive data (in this case, DNS provider API credentials) with new keys. Charon's key rotation system:

- **Re-encrypts all DNS provider credentials** with a new encryption key
- **Maintains zero downtime** during the rotation process
- **Supports multiple key versions** simultaneously for backward compatibility
- **Provides automatic fallback** to legacy keys if needed
- **Creates a full audit trail** of all key operations

### Zero-Downtime Design

Charon's rotation system ensures your DNS challenge certificates continue to work during key rotation:

1. **Multi-key support**: Current key + next key + up to 10 legacy keys can coexist
2. **Gradual migration**: New credentials use the new key, old credentials remain accessible via fallback
3. **Atomic operations**: Each provider's credentials are re-encrypted in a separate database transaction
4. **Automatic retry**: Failed re-encryptions are logged but don't block the rotation process

### Compliance Benefits

Key rotation addresses several compliance and security requirements:

- **PCI-DSS 3.2.1**: Requires cryptographic key changes at least annually
- **SOC 2 Type II**: Demonstrates strong key management controls
- **ISO 27001**: Aligns with cryptographic controls (A.10.1.2)
- **NIST 800-57**: Recommends periodic key rotation for long-lived keys
- **GDPR Article 32**: Demonstrates "state of the art" security measures

Regular key rotation reduces the impact of potential key compromise and limits the window of vulnerability.

---

## Why Key Rotation Matters

### Security Benefits

1. **Limits Exposure Window**: If a key is compromised, only data encrypted with that key is at risk. Regular rotation minimizes the amount of data protected by any single key.

2. **Reduces Cryptanalysis Risk**: Even with strong encryption (AES-256-GCM), limiting the amount of data encrypted under a single key reduces theoretical attack surfaces.

3. **Protects Against Key Leakage**: Keys can leak through logs, backups, or system dumps. Regular rotation ensures leaked keys become obsolete quickly.

4. **Demonstrates Due Diligence**: Regular key rotation shows auditors and stakeholders that security is taken seriously.

### When to Rotate Keys

You should rotate encryption keys:

- ✅ **Annually** (minimum) for compliance
- ✅ **Quarterly** (recommended) for enhanced security
- ✅ **Immediately** after any suspected key compromise
- ✅ **Before** security audits or compliance reviews
- ✅ **After** employee departures (if they had access to keys)
- ✅ **When** migrating to new infrastructure

---

## Key Management Concepts

### Key Lifecycle

Charon manages encryption keys in three states:

#### 1. Current Key (`CHARON_ENCRYPTION_KEY`)

- **Purpose**: Primary encryption key for new credentials
- **Version**: Always version 1 (unless during rotation)
- **Behavior**: All new DNS provider credentials are encrypted with this key
- **Required**: Yes — application won't start without it

```bash
export CHARON_ENCRYPTION_KEY="<32-byte-base64-encoded-key>"
```

#### 2. Next Key (`CHARON_ENCRYPTION_KEY_NEXT`)

- **Purpose**: Destination key for the next rotation
- **Version**: Becomes version 2 after rotation completes
- **Behavior**: When set, new credentials use this key instead of current key
- **Required**: No — only needed when preparing for rotation

```bash
export CHARON_ENCRYPTION_KEY_NEXT="<new-32-byte-base64-key>"
```

#### 3. Legacy Keys (`CHARON_ENCRYPTION_KEY_V1` through `CHARON_ENCRYPTION_KEY_V10`)

- **Purpose**: Fallback keys for decrypting older credentials
- **Version**: 1-10 (corresponds to environment variable suffix)
- **Behavior**: Automatic fallback during decryption if current key fails
- **Required**: No — but recommended to keep for at least 30 days after rotation

```bash
export CHARON_ENCRYPTION_KEY_V1="<old-32-byte-key>"
export CHARON_ENCRYPTION_KEY_V2="<older-32-byte-key>"
# ... up to V10
```

### Key Versioning System

Every encrypted credential stores its **key version** alongside the ciphertext. This enables:

- **Automatic fallback**: Charon knows which key to try first
- **Status reporting**: See how many credentials use which key version
- **Rotation tracking**: Verify rotation completed successfully

**Example**:

- Before rotation: All 15 DNS providers have `key_version = 1`
- After rotation: All 15 DNS providers have `key_version = 2`

### Environment Variable Schema

The complete key configuration looks like this:

```bash
# Required: Current encryption key
CHARON_ENCRYPTION_KEY="ABcdEF1234567890ABcdEF1234567890ABCDEFGH="

# Optional: Next key for rotation (set before triggering rotation)
CHARON_ENCRYPTION_KEY_NEXT="XyZaBcDeF1234567890XyZaBcDeF1234567890XY="

# Optional: Legacy keys for backward compatibility (keep for 30+ days)
CHARON_ENCRYPTION_KEY_V1="OldKey1234567890OldKey1234567890OldKey12=="
CHARON_ENCRYPTION_KEY_V2="OlderK1234567890OlderK1234567890OlderK1=="
```

**Key Format Requirements**:

- **Length**: 32 bytes (before base64 encoding)
- **Encoding**: Base64-encoded
- **Generation**: Use cryptographically secure random number generator

**Generate a new key**:

```bash
# Using OpenSSL
openssl rand -base64 32

# Using Python
python3 -c "import secrets, base64; print(base64.b64encode(secrets.token_bytes(32)).decode())"

# Using Node.js
node -e "console.log(require('crypto').randomBytes(32).toString('base64'))"
```

---

## Accessing Key Management

### Navigation Path

1. Log in as an **administrator** (key rotation is admin-only)
2. Navigate to **Security** → **Encryption Management** in the sidebar
3. The Encryption Key Management page displays

### Permission Requirements

**Admin Role Required**: Only users with `role = "admin"` can:

- View encryption status
- Trigger key rotation
- Validate key configuration
- View rotation history

Non-admin users receive a **403 Forbidden** error if they attempt to access encryption endpoints.

### UI Overview

The Encryption Management page includes:

1. **Status Cards** (top section)
   - Current Key Version
   - Providers Updated
   - Providers Outdated
   - Next Key Status

2. **Actions Section** (middle)
   - Rotate Encryption Key button
   - Validate Configuration button

3. **Environment Guide** (expandable)
   - Step-by-step rotation instructions
   - Environment variable examples

4. **Rotation History** (bottom)
   - Paginated audit log of past rotations
   - Timestamp, actor, action, and details

---

## Understanding Key Status

### Current Key Version

**What it shows**: The active key version in use.

**Possible values**:

- `Version 1` — Initial key (default state)
- `Version 2` — After first rotation
- `Version 3+` — After subsequent rotations

**What to check**: Ensure this matches your expectation after rotation.

### Providers Updated

**What it shows**: Number of DNS providers using the **current** key version.

**Example**: `15 Providers` — All providers are on the latest key.

**What to check**: After rotation, this should equal your total provider count.

### Providers Outdated

**What it shows**: Number of DNS providers using **older** key versions.

**Example**: `3 Providers` — Three providers still use legacy keys.

**What to check**:

- Should be **0** immediately after successful rotation
- If non-zero after rotation, check audit logs for errors

### Next Key Status

**What it shows**: Whether `CHARON_ENCRYPTION_KEY_NEXT` is configured.

**Possible values**:

- ✅ **Configured** — Ready for rotation
- ❌ **Not Configured** — Cannot rotate (next key not set)

**What to check**: Before rotating, ensure this shows "Configured".

### Legacy Keys Detected

**What it shows**: Number of legacy keys configured (V1-V10).

**Example**: `2 legacy keys detected` — You have V1 and V2 configured.

**What to check**: Keep legacy keys for at least 30 days after rotation for rollback capability.

---

## Rotating Encryption Keys

### Preparation Checklist

Before rotating keys, ensure:

- ✅ You have **admin access** to Charon
- ✅ You've **generated a new encryption key** (see [Key Versioning System](#key-versioning-system))
- ✅ You've **backed up your database** (critical!)
- ✅ You've **tested rotation in staging** first (if possible)
- ✅ You understand the **rollback procedure** (see [Troubleshooting](#troubleshooting))
- ✅ You've **scheduled a maintenance window** (optional but recommended)

### Step-by-Step Rotation Workflow

#### Step 1: Set the Next Key

**Action**: Configure `CHARON_ENCRYPTION_KEY_NEXT` environment variable.

**Docker Compose Example**:

```yaml
services:
  charon:
    environment:
      - CHARON_ENCRYPTION_KEY=${CHARON_ENCRYPTION_KEY}
      - CHARON_ENCRYPTION_KEY_NEXT=${CHARON_ENCRYPTION_KEY_NEXT}
```

**Docker CLI Example**:

```bash
docker run -d \
  -e CHARON_ENCRYPTION_KEY="ABcdEF1234567890ABcdEF1234567890ABCDEFGH=" \
  -e CHARON_ENCRYPTION_KEY_NEXT="XyZaBcDeF1234567890XyZaBcDeF1234567890XY=" \
  charon:latest
```

**Kubernetes Example**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: charon-encryption-keys
type: Opaque
data:
  CHARON_ENCRYPTION_KEY: <base64-of-base64-key>
  CHARON_ENCRYPTION_KEY_NEXT: <base64-of-base64-key>
```

**What happens**: Nothing yet. This just prepares the new key.

#### Step 2: Restart Charon

**Action**: Restart the application to load the new environment variable.

```bash
# Docker Compose
docker-compose restart charon

# Docker CLI
docker restart charon

# Kubernetes
kubectl rollout restart deployment/charon
```

**What happens**: Charon loads both current and next keys into memory.

**Verification**:

```bash
# Check logs for successful startup
docker logs charon 2>&1 | grep "encryption"
```

Expected output:

```
{"level":"info","msg":"Encryption keys loaded: current + next configured"}
```

#### Step 3: Validate Configuration (Optional but Recommended)

**Action**: Click **"Validate Configuration"** button in the Encryption Management UI.

**Alternative (API)**:

```bash
curl -X POST https://your-charon-instance/api/v1/admin/encryption/validate \
  -H "Authorization: Bearer <admin-token>"
```

**What happens**: Charon tests round-trip encryption with all configured keys (current, next, legacy).

**Success response**:

```json
{
  "status": "valid",
  "keys_tested": 2,
  "message": "All encryption keys validated successfully"
}
```

**What to check**: Ensure all keys pass validation before proceeding.

#### Step 4: Trigger Rotation

**Action**: Click **"Rotate Encryption Key"** button in the Encryption Management UI.

**Confirmation dialog**:

- Review the warning: "This will re-encrypt all DNS provider credentials with the new key. This operation cannot be undone."
- Check **"I understand"** checkbox
- Click **"Start Rotation"**

**Alternative (API)**:

```bash
curl -X POST https://your-charon-instance/api/v1/admin/encryption/rotate \
  -H "Authorization: Bearer <admin-token>"
```

**What happens**:

1. Charon fetches all DNS providers from the database
2. For each provider:
   - Decrypts credentials with current key
   - Re-encrypts credentials with next key
   - Updates `key_version` field to 2
   - Commits transaction
3. Returns detailed rotation result

**Success response**:

```json
{
  "total_providers": 15,
  "success_count": 15,
  "failure_count": 0,
  "failed_providers": [],
  "start_time": "2026-01-04T10:00:00Z",
  "end_time": "2026-01-04T10:00:02Z",
  "duration": "2.1s",
  "new_key_version": 2
}
```

**Success toast**: "Key rotation completed successfully: 15/15 providers rotated in 2.1s"

#### Step 5: Verify Rotation

**Action**: Refresh the Encryption Management page.

**What to check**:

- ✅ **Current Key Version**: Should now show `Version 2`
- ✅ **Providers Updated**: Should show `15 Providers` (your total count)
- ✅ **Providers Outdated**: Should show `0 Providers`

**Alternative (API)**:

```bash
curl https://your-charon-instance/api/v1/admin/encryption/status \
  -H "Authorization: Bearer <admin-token>"
```

**Expected response**:

```json
{
  "current_version": 2,
  "next_key_configured": true,
  "legacy_key_count": 0,
  "providers_by_version": {
    "2": 15
  },
  "providers_on_current_version": 15,
  "providers_on_older_versions": 0
}
```

#### Step 6: Promote Next Key to Current

**Action**: Update environment variables to make the new key permanent.

**Before**:

```bash
CHARON_ENCRYPTION_KEY="ABcdEF1234567890ABcdEF1234567890ABCDEFGH="  # Old key
CHARON_ENCRYPTION_KEY_NEXT="XyZaBcDeF1234567890XyZaBcDeF1234567890XY="  # New key
```

**After**:

```bash
CHARON_ENCRYPTION_KEY="XyZaBcDeF1234567890XyZaBcDeF1234567890XY="  # New key (promoted)
CHARON_ENCRYPTION_KEY_V1="ABcdEF1234567890ABcdEF1234567890ABCDEFGH="  # Old key (kept as legacy)
# Remove CHARON_ENCRYPTION_KEY_NEXT
```

**What happens**: The new key becomes the primary key, and the old key is retained for backward compatibility.

#### Step 7: Restart Again

**Action**: Restart Charon to load the new configuration.

```bash
docker-compose restart charon
```

**What happens**: Charon now uses the new key for future encryptions and keeps the old key for fallback.

**Verification**:

```bash
docker logs charon 2>&1 | grep "encryption"
```

Expected output:

```
{"level":"info","msg":"Encryption keys loaded: current + 1 legacy keys"}
```

#### Step 8: Wait 30 Days

**Action**: Keep the legacy key (`V1`) configured for at least 30 days.

**Why**: This provides a rollback window in case issues are discovered later.

**After 30 days**: Remove `CHARON_ENCRYPTION_KEY_V1` from your environment if no issues occurred.

### Monitoring Rotation Progress

**During rotation**:

- The UI shows a loading overlay with "Rotating..." message
- The rotation button is disabled
- You'll see a progress toast notification

**After rotation**:

- Success toast appears with provider count and duration
- Status cards update immediately
- Audit log entry is created

**If rotation takes longer than expected**:

- Check the backend logs: `docker logs charon -f`
- Look for errors like "Failed to decrypt provider X credentials"
- See [Troubleshooting](#troubleshooting) section

---

## Validating Key Configuration

### Why Validate?

Validation tests that all configured keys work correctly **before** triggering rotation. This prevents:

- ❌ Broken keys being used for rotation
- ❌ Credentials becoming inaccessible
- ❌ Failed rotations due to corrupted keys

### When to Validate

Run validation:

- ✅ **Before** every key rotation
- ✅ **After** changing environment variables
- ✅ **After** restoring from backup
- ✅ **Monthly** as part of routine maintenance

### How to Validate

**Via UI**:

1. Go to **Security** → **Encryption Management**
2. Click **"Validate Configuration"** button
3. Wait for validation to complete (usually < 1 second)

**Via API**:

```bash
curl -X POST https://your-charon-instance/api/v1/admin/encryption/validate \
  -H "Authorization: Bearer <admin-token>"
```

### What Validation Checks

Charon performs round-trip encryption for each configured key:

1. **Current Key Test**:
   - Encrypts test data with current key
   - Decrypts ciphertext
   - Verifies plaintext matches original

2. **Next Key Test** (if configured):
   - Encrypts test data with next key
   - Decrypts ciphertext
   - Verifies plaintext matches original

3. **Legacy Key Tests** (if configured):
   - Encrypts test data with each legacy key
   - Decrypts ciphertext
   - Verifies plaintext matches original

### Success Response

**UI**: Green success toast: "Key configuration is valid and ready for rotation"

**API Response**:

```json
{
  "status": "valid",
  "keys_tested": 3,
  "message": "All encryption keys validated successfully",
  "details": {
    "current_key": "valid",
    "next_key": "valid",
    "legacy_keys": ["v1: valid"]
  }
}
```

### Failure Response

**UI**: Red error toast: "Key configuration validation failed. Check errors below."

**API Response**:

```json
{
  "status": "invalid",
  "keys_tested": 3,
  "message": "Validation failed",
  "errors": [
    {
      "key": "next_key",
      "error": "decryption failed: cipher: message authentication failed"
    }
  ]
}
```

**Common errors**:

- `"decryption failed"` — Key is corrupted or not base64-encoded correctly
- `"key too short"` — Key is not 32 bytes after base64 decoding
- `"invalid base64"` — Key contains invalid base64 characters

### Fixing Validation Errors

**Error**: `"next_key: decryption failed"`

**Fix**:

1. Regenerate the next key: `openssl rand -base64 32`
2. Update `CHARON_ENCRYPTION_KEY_NEXT` environment variable
3. Restart Charon
4. Validate again

**Error**: `"key too short"`

**Fix**:

1. Ensure you're generating 32 bytes: `openssl rand -base64 32` (not `openssl rand 32`)
2. Verify base64 encoding is correct
3. Update environment variable
4. Restart Charon

**Error**: `"invalid base64"`

**Fix**:

1. Check for extra whitespace or newlines in the key
2. Ensure the key is properly quoted in docker-compose.yml
3. Re-copy the key carefully
4. Update environment variable
5. Restart Charon

---

## Viewing Rotation History

### Accessing Audit History

**Via UI**:

1. Go to **Security** → **Encryption Management**
2. Scroll to the **Rotation History** section at the bottom
3. View paginated list of rotation events

**Via API**:

```bash
curl "https://your-charon-instance/api/v1/admin/encryption/history?page=1&limit=20" \
  -H "Authorization: Bearer <admin-token>"
```

### Understanding Rotation Events

Charon logs the following encryption-related audit events:

#### 1. Key Rotation Started

**Event**: `encryption_key_rotation_started`

**When**: Immediately when rotation is triggered

**Details**:

```json
{
  "timestamp": "2026-01-04T10:00:00Z",
  "actor": "admin@example.com",
  "action": "encryption_key_rotation_started",
  "details": {
    "current_version": 1,
    "next_version": 2,
    "total_providers": 15
  }
}
```

#### 2. Key Rotation Completed

**Event**: `encryption_key_rotation_completed`

**When**: After all providers are successfully re-encrypted

**Details**:

```json
{
  "timestamp": "2026-01-04T10:00:02Z",
  "actor": "admin@example.com",
  "action": "encryption_key_rotation_completed",
  "details": {
    "total_providers": 15,
    "success_count": 15,
    "failure_count": 0,
    "duration": "2.1s",
    "new_key_version": 2
  }
}
```

#### 3. Key Rotation Failed

**Event**: `encryption_key_rotation_failed`

**When**: If rotation encounters critical errors

**Details**:

```json
{
  "timestamp": "2026-01-04T10:05:00Z",
  "actor": "admin@example.com",
  "action": "encryption_key_rotation_failed",
  "details": {
    "error": "CHARON_ENCRYPTION_KEY_NEXT not configured",
    "total_providers": 15,
    "success_count": 0,
    "failure_count": 15
  }
}
```

#### 4. Key Validation Success

**Event**: `encryption_key_validation_success`

**When**: After successful validation

**Details**:

```json
{
  "timestamp": "2026-01-04T09:55:00Z",
  "actor": "admin@example.com",
  "action": "encryption_key_validation_success",
  "details": {
    "keys_tested": 2,
    "message": "All encryption keys validated successfully"
  }
}
```

#### 5. Key Validation Failed

**Event**: `encryption_key_validation_failed`

**When**: If validation detects issues

**Details**:

```json
{
  "timestamp": "2026-01-04T09:50:00Z",
  "actor": "admin@example.com",
  "action": "encryption_key_validation_failed",
  "details": {
    "error": "next_key validation failed: decryption error"
  }
}
```

### Filtering History

**By page**:

```bash
curl "https://your-charon-instance/api/v1/admin/encryption/history?page=2&limit=10"
```

**By event category**: Encryption events are automatically filtered (`event_category = "encryption"`).

### Exporting History

**Via API** (JSON):

```bash
curl "https://your-charon-instance/api/v1/admin/encryption/history?page=1&limit=1000" \
  -H "Authorization: Bearer <admin-token>" \
  > encryption_audit_log.json
```

**Via UI** (future feature): CSV export coming soon.

---

## Best Practices

### Rotation Frequency Recommendations

| Environment | Rotation Frequency | Rationale |
|-------------|-------------------|-----------|
| **Production (High-Risk)** | Quarterly (every 3 months) | Meets most compliance requirements, reduces exposure window |
| **Production (Standard)** | Annually (every 12 months) | Minimum for PCI-DSS and SOC 2 compliance |
| **Staging/Testing** | As needed | Match production rotation schedule for testing |
| **Development** | Never (use test keys) | Not applicable for non-sensitive environments |

### Key Retention Policies

**Legacy Key Retention**:

- ✅ Keep legacy keys for **at least 30 days** after rotation
- ✅ Extend to **90 days** for high-risk environments
- ✅ Never delete legacy keys immediately after rotation

**Why**:

- Allows rollback if issues are discovered
- Supports disaster recovery from old backups
- Provides time to verify rotation success

**After Retention Period**:

1. Verify no issues occurred during retention window
2. Remove legacy key from environment variables
3. Restart Charon to apply changes
4. Document removal in audit log

### Backup Procedures

**Before Every Rotation**:

1. **Backup the database**:

   ```bash
   docker exec charon_db pg_dump -U charon charon_db > backup_before_rotation_$(date +%Y%m%d).sql
   ```

2. **Backup environment variables**:

   ```bash
   cp docker-compose.yml docker-compose.yml.backup_$(date +%Y%m%d)
   ```

3. **Test backup restoration**:

   ```bash
   # Restore database
   docker exec -i charon_db psql -U charon charon_db < backup_before_rotation_20260104.sql
   ```

**After Rotation**:

1. **Backup the new state**:

   ```bash
   docker exec charon_db pg_dump -U charon charon_db > backup_after_rotation_$(date +%Y%m%d).sql
   ```

2. **Store backups securely**:
   - Use encrypted storage (e.g., AWS S3 with SSE-KMS)
   - Keep backups for retention period (30-90 days)
   - Verify backup integrity monthly

### Testing in Staging First

**Before rotating production keys**:

1. ✅ Deploy exact production configuration to staging
2. ✅ Perform full rotation in staging
3. ✅ Verify all DNS providers still work
4. ✅ Test certificate renewal with newly rotated credentials
5. ✅ Monitor staging for 24-48 hours
6. ✅ Document any issues and resolution steps
7. ✅ Apply same procedure to production

**Staging checklist**:

- [ ] Same Charon version as production
- [ ] Same number of DNS providers
- [ ] Same encryption key length and format
- [ ] Same environment variable configuration
- [ ] Test ACME challenges post-rotation

### Rollback Procedures

If rotation fails or issues are discovered, follow this rollback procedure:

#### Immediate Rollback (< 1 hour after rotation)

**Scenario**: Rotation just completed but providers are failing.

**Steps**:

1. **Restore database from pre-rotation backup**:

   ```bash
   docker exec -i charon_db psql -U charon charon_db < backup_before_rotation_20260104.sql
   ```

2. **Revert environment variables**:

   ```bash
   cp docker-compose.yml.backup_20260104 docker-compose.yml
   ```

3. **Restart Charon**:

   ```bash
   docker-compose restart charon
   ```

4. **Verify restoration**:
   - Check encryption status shows old version
   - Test DNS provider connectivity
   - Review audit logs

#### Delayed Rollback (> 1 hour after rotation)

**Scenario**: Issues discovered hours or days after rotation.

**Steps**:

1. **Keep new key as legacy**:

   ```bash
   CHARON_ENCRYPTION_KEY="<old-key>"  # Revert to old key
   CHARON_ENCRYPTION_KEY_V2="<new-key>"  # Keep new key as legacy
   ```

2. **Restart Charon** — Credentials remain accessible via fallback

3. **Manually update affected providers**:
   - Edit each provider in the UI
   - Re-save to re-encrypt with old key
   - Or restore from backup selectively

4. **Document incident**:
   - What failed
   - Why rollback was needed
   - How to prevent in future

### Security Considerations

**Key Storage**:

- ❌ **NEVER** commit keys to version control
- ✅ Use environment variables or secrets manager
- ✅ Restrict access to key values (need-to-know basis)
- ✅ Audit access to secrets manager

**Key Generation**:

- ✅ Always use cryptographically secure RNG (`openssl`, `secrets`, `crypto`)
- ❌ Never use predictable sources (`date`, `rand()`, keyboard mashing)
- ✅ Generate keys on secure, trusted systems
- ✅ Never reuse keys across environments (prod vs staging)

**Key Transmission**:

- ✅ Use encrypted channels (SSH, TLS) to transmit keys
- ❌ Never send keys via email, Slack, or unencrypted chat
- ✅ Use secrets managers with RBAC (e.g., Vault, AWS Secrets Manager)
- ✅ Rotate keys immediately if transmission is compromised

**Access Control**:

- ✅ Limit key rotation to admin users only
- ✅ Require MFA for admin accounts
- ✅ Audit all key-related operations
- ✅ Review audit logs monthly

---

## Troubleshooting

### Common Issues and Solutions

#### Issue: Rotation Button Disabled

**Symptom**: "Rotate Encryption Key" button is grayed out.

**Possible causes**:

1. ❌ Next key not configured
2. ❌ Not logged in as admin
3. ❌ Rotation already in progress

**Solution**:

1. Check **Next Key Status** — should show "Configured"
2. Verify you're logged in as admin (check user menu)
3. Wait for in-progress rotation to complete
4. If none of above, check browser console for errors

#### Issue: Failed Rotations (Partial Success)

**Symptom**: Toast shows "Warning: 3 providers failed to rotate."

**Possible causes**:

1. ❌ Corrupted credentials in database
2. ❌ Missing key versions
3. ❌ Database transaction errors

**Solution**:

1. **Check audit logs** for specific errors:

   ```bash
   curl "https://your-charon-instance/api/v1/admin/encryption/history?page=1" \
     -H "Authorization: Bearer <admin-token>"
   ```

2. **Identify failed providers**:
   - Response includes `"failed_providers": [5, 12, 18]`
   - Note the provider IDs

3. **Manually fix failed providers**:
   - Go to **DNS Providers** → Edit each failed provider
   - Re-enter credentials
   - Save — this re-encrypts with current key

4. **Retry rotation**:
   - Validate configuration first
   - Trigger rotation again
   - All providers should succeed this time

#### Issue: Missing Keys After Restart

**Symptom**: After promoting next key, Charon won't start or credentials fail.

**Error log**:

```
{"level":"fatal","msg":"CHARON_ENCRYPTION_KEY not set"}
```

**Solution**:

1. **Check environment variables**:

   ```bash
   docker exec charon env | grep CHARON_ENCRYPTION
   ```

2. **Verify docker-compose.yml**:
   - Ensure `CHARON_ENCRYPTION_KEY` is set
   - Check for typos in variable names
   - Verify base64 encoding is correct

3. **Restart with corrected config**:

   ```bash
   docker-compose down
   docker-compose up -d
   ```

#### Issue: Version Mismatches

**Symptom**: Status shows "Providers Outdated: 15" even after rotation.

**Possible causes**:

1. ❌ Rotation didn't complete successfully
2. ❌ Database rollback occurred
3. ❌ Frontend cache showing stale data

**Solution**:

1. **Refresh the page** (hard refresh: Ctrl+Shift+R)

2. **Check API directly**:

   ```bash
   curl https://your-charon-instance/api/v1/admin/encryption/status \
     -H "Authorization: Bearer <admin-token>"
   ```

3. **Verify database state**:

   ```sql
   SELECT key_version, COUNT(*) FROM dns_providers GROUP BY key_version;
   ```

4. **If still outdated**, trigger rotation again

#### Issue: Validation Fails on Legacy Keys

**Symptom**: Validation shows errors for `CHARON_ENCRYPTION_KEY_V1`.

**Error**: `"v1: decryption failed"`

**Possible causes**:

1. ❌ Key was changed accidentally
2. ❌ Key is corrupted
3. ❌ Wrong key assigned to V1

**Solution**:

1. **Identify the correct key**:
   - Check your key rotation history
   - Review backup files
   - Consult secrets manager logs

2. **Update environment variable**:

   ```bash
   CHARON_ENCRYPTION_KEY_V1="<correct-old-key>"
   ```

3. **Restart Charon** and validate again

4. **If key is lost**:
   - Credentials encrypted with that key are **unrecoverable**
   - You'll need to re-enter credentials manually
   - Update affected DNS providers via UI

#### Issue: Rotation Takes Too Long

**Symptom**: Rotation running for > 5 minutes with many providers.

**Expected duration**:

- 1-10 providers: < 5 seconds
- 10-50 providers: < 30 seconds
- 50-100 providers: < 2 minutes

**Possible causes**:

1. ❌ Database performance issues
2. ❌ Database locks or contention
3. ❌ Network issues (if database is remote)

**Solution**:

1. **Check backend logs**:

   ```bash
   docker logs charon -f | grep "rotation"
   ```

2. **Look for slow queries**:

   ```bash
   docker logs charon | grep "slow query"
   ```

3. **Check database health**:

   ```bash
   docker exec charon_db pg_stat_activity
   ```

4. **If stuck**, restart Charon and retry:
   - Rotation is idempotent
   - Already-rotated providers will be skipped

### Getting Help

If you encounter issues not covered here:

1. **Check the logs**:

   ```bash
   docker logs charon -f
   ```

2. **Enable debug logging** (if needed):

   ```yaml
   environment:
     - LOG_LEVEL=debug
   ```

3. **Search existing issues**: [GitHub Issues](https://github.com/Wikid82/charon/issues)

4. **Open a new issue** with:
   - Charon version
   - Rotation error message
   - Relevant log excerpts (sanitize secrets!)
   - Steps to reproduce

5. **Join the community**: [GitHub Discussions](https://github.com/Wikid82/charon/discussions)

---

## API Reference

### Encryption Management Endpoints

All encryption management endpoints require **admin authentication**.

#### Get Encryption Status

**Endpoint**: `GET /api/v1/admin/encryption/status`

**Description**: Returns current encryption key status, provider distribution, and rotation readiness.

**Authentication**: Required (admin only)

**Request**:

```bash
curl https://your-charon-instance/api/v1/admin/encryption/status \
  -H "Authorization: Bearer <admin-token>"
```

**Success Response** (HTTP 200):

```json
{
  "current_version": 2,
  "next_key_configured": true,
  "legacy_key_count": 1,
  "providers_by_version": {
    "2": 15
  },
  "providers_on_current_version": 15,
  "providers_on_older_versions": 0
}
```

**Response Fields**:

- `current_version` (int): Active key version (1, 2, 3, etc.)
- `next_key_configured` (bool): Whether `CHARON_ENCRYPTION_KEY_NEXT` is set
- `legacy_key_count` (int): Number of legacy keys (V1-V10) configured
- `providers_by_version` (object): Breakdown of providers per key version
- `providers_on_current_version` (int): Count using latest key
- `providers_on_older_versions` (int): Count needing rotation

**Error Responses**:

- **401 Unauthorized**: Missing or invalid token
- **403 Forbidden**: Non-admin user
- **500 Internal Server Error**: Database or encryption service error

---

#### Rotate Encryption Keys

**Endpoint**: `POST /api/v1/admin/encryption/rotate`

**Description**: Triggers re-encryption of all DNS provider credentials with the next key.

**Authentication**: Required (admin only)

**Prerequisites**:

- `CHARON_ENCRYPTION_KEY_NEXT` must be configured
- Application must be restarted to load next key

**Request**:

```bash
curl -X POST https://your-charon-instance/api/v1/admin/encryption/rotate \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json"
```

**Success Response** (HTTP 200):

```json
{
  "total_providers": 15,
  "success_count": 15,
  "failure_count": 0,
  "failed_providers": [],
  "start_time": "2026-01-04T10:00:00Z",
  "end_time": "2026-01-04T10:00:02Z",
  "duration": "2.1s",
  "new_key_version": 2
}
```

**Partial Success Response** (HTTP 200):

```json
{
  "total_providers": 15,
  "success_count": 12,
  "failure_count": 3,
  "failed_providers": [5, 12, 18],
  "start_time": "2026-01-04T10:00:00Z",
  "end_time": "2026-01-04T10:00:15Z",
  "duration": "15.3s",
  "new_key_version": 2
}
```

**Response Fields**:

- `total_providers` (int): Total DNS providers in database
- `success_count` (int): Providers successfully re-encrypted
- `failure_count` (int): Providers that failed re-encryption
- `failed_providers` (array): IDs of failed providers
- `start_time` (string): ISO 8601 timestamp when rotation started
- `end_time` (string): ISO 8601 timestamp when rotation completed
- `duration` (string): Human-readable duration
- `new_key_version` (int): New key version after rotation

**Error Responses**:

- **400 Bad Request**: `CHARON_ENCRYPTION_KEY_NEXT` not configured

  ```json
  {
    "error": "Next key not configured. Set CHARON_ENCRYPTION_KEY_NEXT and restart."
  }
  ```

- **401 Unauthorized**: Missing or invalid token
- **403 Forbidden**: Non-admin user
- **500 Internal Server Error**: Critical failure during rotation

**Audit Events Created**:

- `encryption_key_rotation_started` — When rotation begins
- `encryption_key_rotation_completed` — When rotation succeeds
- `encryption_key_rotation_failed` — When rotation fails

---

#### Validate Key Configuration

**Endpoint**: `POST /api/v1/admin/encryption/validate`

**Description**: Tests round-trip encryption with all configured keys (current, next, legacy).

**Authentication**: Required (admin only)

**Request**:

```bash
curl -X POST https://your-charon-instance/api/v1/admin/encryption/validate \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json"
```

**Success Response** (HTTP 200):

```json
{
  "status": "valid",
  "keys_tested": 3,
  "message": "All encryption keys validated successfully",
  "details": {
    "current_key": "valid",
    "next_key": "valid",
    "legacy_keys": [
      {"version": 1, "status": "valid"}
    ]
  }
}
```

**Failure Response** (HTTP 400):

```json
{
  "status": "invalid",
  "keys_tested": 3,
  "message": "Validation failed",
  "errors": [
    {
      "key": "next_key",
      "error": "decryption failed: cipher: message authentication failed"
    }
  ]
}
```

**Response Fields**:

- `status` (string): `"valid"` or `"invalid"`
- `keys_tested` (int): Total keys tested
- `message` (string): Human-readable summary
- `details` (object): Per-key validation results
- `errors` (array): List of validation errors (if any)

**Error Responses**:

- **401 Unauthorized**: Missing or invalid token
- **403 Forbidden**: Non-admin user
- **500 Internal Server Error**: Validation service error

**Audit Events Created**:

- `encryption_key_validation_success` — When validation passes
- `encryption_key_validation_failed` — When validation fails

---

#### Get Rotation History

**Endpoint**: `GET /api/v1/admin/encryption/history`

**Description**: Returns paginated audit log of encryption-related events.

**Authentication**: Required (admin only)

**Query Parameters**:

- `page` (int, optional): Page number (default: 1)
- `limit` (int, optional): Results per page (default: 20, max: 100)

**Request**:

```bash
curl "https://your-charon-instance/api/v1/admin/encryption/history?page=1&limit=20" \
  -H "Authorization: Bearer <admin-token>"
```

**Success Response** (HTTP 200):

```json
{
  "events": [
    {
      "id": 42,
      "timestamp": "2026-01-04T10:00:02Z",
      "actor": "admin@example.com",
      "action": "encryption_key_rotation_completed",
      "event_category": "encryption",
      "details": {
        "total_providers": 15,
        "success_count": 15,
        "failure_count": 0,
        "duration": "2.1s",
        "new_key_version": 2
      }
    },
    {
      "id": 41,
      "timestamp": "2026-01-04T10:00:00Z",
      "actor": "admin@example.com",
      "action": "encryption_key_rotation_started",
      "event_category": "encryption",
      "details": {
        "current_version": 1,
        "next_version": 2,
        "total_providers": 15
      }
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total_events": 2,
    "total_pages": 1
  }
}
```

**Response Fields**:

- `events` (array): List of audit log entries
  - `id` (int): Audit log entry ID
  - `timestamp` (string): ISO 8601 timestamp
  - `actor` (string): Email of user who triggered event
  - `action` (string): Event type (see [Understanding Rotation Events](#understanding-rotation-events))
  - `event_category` (string): Always `"encryption"`
  - `details` (object): Event-specific metadata
- `pagination` (object): Pagination metadata
  - `page` (int): Current page number
  - `limit` (int): Results per page
  - `total_events` (int): Total events matching filter
  - `total_pages` (int): Total pages available

**Error Responses**:

- **400 Bad Request**: Invalid page or limit parameter
- **401 Unauthorized**: Missing or invalid token
- **403 Forbidden**: Non-admin user
- **500 Internal Server Error**: Database query error

---

### Authentication

All encryption management endpoints use **Bearer token authentication**.

**Obtaining a token**:

```bash
# Login to get token
curl -X POST https://your-charon-instance/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "your-password"
  }'

# Response includes token
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 1,
    "email": "admin@example.com",
    "role": "admin"
  }
}
```

**Using the token**:

```bash
curl https://your-charon-instance/api/v1/admin/encryption/status \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### Rate Limiting

Encryption management endpoints are not rate-limited by default, but general API rate limits may apply. Check your Charon configuration for rate limit settings.

---

## Cross-References

### Related Documentation

- **[Audit Logging](audit-logging.md)** — View detailed audit logs for all key operations
- **[DNS Providers](../guides/dns-providers/)** — Configure DNS providers whose credentials are encrypted
- **[Security Best Practices](../security.md)** — General security guidance for Charon
- **[Database Maintenance](../database-maintenance.md)** — Backup and recovery procedures
- **[API Documentation](../api.md)** — Complete API reference for all endpoints

### External Resources

- **[NIST 800-57 Part 1](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-57pt1r5.pdf)** — Key Management Recommendations
- **[PCI-DSS 3.2.1](https://www.pcisecuritystandards.org/)** — Requirement 3.6.4 (Cryptographic Key Management)
- **[OWASP Cryptographic Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html)**
- **[AES-GCM Encryption](https://en.wikipedia.org/wiki/Galois/Counter_Mode)** — Understanding the encryption algorithm
- **[Base64 Encoding](https://en.wikipedia.org/wiki/Base64)** — Key encoding format

---

## Summary

Encryption key rotation is a critical security practice that Charon makes easy with:

- ✅ **Zero-downtime rotation** — Services remain available throughout the process
- ✅ **Multi-key support** — Current + next + legacy keys coexist seamlessly
- ✅ **Admin-friendly UI** — No command-line expertise required
- ✅ **Complete audit trail** — Every key operation is logged
- ✅ **Automatic fallback** — Decryption tries all available keys
- ✅ **Validation tools** — Test keys before using them

**Next Steps**:

1. Review your organization's key rotation policy
2. Schedule your first rotation (test in staging first!)
3. Set a recurring reminder for future rotations
4. Document your rotation procedure
5. Monitor audit logs after each rotation

**Questions?** Join the discussion at [GitHub Discussions](https://github.com/Wikid82/charon/discussions).

---

*Last updated: January 4, 2026 | Charon Version: 0.1.0-beta*
