# API Key Security Guidelines

## Overview

This document outlines security best practices for handling API keys and other sensitive credentials in Charon. These guidelines help prevent common vulnerabilities like CWE-312 (Cleartext Storage of Sensitive Information), CWE-315 (Cleartext Storage in Cookie), and CWE-359 (Exposure of Private Personal Information).

## Logging Best Practices

### NEVER Log Sensitive Credentials

**Critical Rule**: Never log sensitive credentials (API keys, tokens, passwords) in plaintext.

### Masking Implementation

Charon implements secure API key masking that shows only the first 4 and last 4 characters:

```go
// ✅ GOOD: Masked key
logger.Infof("API Key: %s", maskAPIKey(apiKey))
// Output: "API Key: abcd...xyz9"

// ❌ BAD: Full key exposure
logger.Infof("API Key: %s", apiKey)
// Output: "API Key: abcd1234567890xyz9" (SECURITY RISK!)
```

### Masking Rules

The `maskAPIKey()` function implements these rules:

1. **Empty keys**: Returns `[empty]`
2. **Short keys (< 16 chars)**: Returns `[REDACTED]`
3. **Normal keys (≥ 16 chars)**: Shows first 4 + last 4 characters (e.g., `abcd...xyz9`)

These rules ensure that:
- Keys cannot be reconstructed from logs
- Users can still identify which key was used (by prefix/suffix)
- Debugging remains possible without exposing secrets

## Key Storage

### File Storage Requirements

API keys must be stored with secure file permissions:

```go
// Save with restricted permissions (owner read/write only)
err := os.WriteFile(keyFile, []byte(apiKey), 0600)
```

**Required permissions**: `0600` (rw-------)
- Owner: read + write
- Group: no access
- Others: no access

### Storage Best Practices

1. **Use secure file permissions (0600)** for key files
2. **Store keys in environment variables** for production deployments
3. **Never commit keys to version control** (.gitignore all key files)
4. **Encrypt keys at rest** when possible
5. **Use separate keys per environment** (dev/staging/prod)

## Key Validation

### Format Validation

The `validateAPIKeyFormat()` function enforces these rules:

- **Length**: 16-128 characters
- **Charset**: Alphanumeric + underscore (`_`) + hyphen (`-`)
- **No spaces or special characters**

```go
// Valid keys
"api_key_1234567890123456"           // ✅
"api-key-ABCDEF1234567890"           // ✅
"1234567890123456"                   // ✅

// Invalid keys
"short"                               // ❌ Too short (< 16 chars)
strings.Repeat("a", 129)              // ❌ Too long (> 128 chars)
"api key with spaces"                 // ❌ Invalid characters
"api@key#special"                     // ❌ Invalid characters
```

### Validation Benefits

- Prevents weak/malformed keys
- Detects potential key corruption
- Provides early failure feedback
- Improves security posture

## Security Warnings

### Log Aggregation Risks

If logs are shipped to external services (CloudWatch, Splunk, Datadog, etc.):
- Masked keys are safe to log
- Full keys would be exposed across multiple systems
- Log retention policies apply to all destinations

### Error Message Safety

**Never include sensitive data in error messages**:

```go
// ✅ GOOD: Generic error
return fmt.Errorf("authentication failed")

// ❌ BAD: Leaks key info
return fmt.Errorf("invalid API key: %s", apiKey)
```

### HTTP Response Safety

**Never return API keys in HTTP responses**:

```go
// ✅ GOOD: Omit sensitive fields
c.JSON(200, gin.H{
    "status": "registered",
    "keyFile": "/path/to/key",  // Path only, not content
})

// ❌ BAD: Exposes key
c.JSON(200, gin.H{
    "apiKey": apiKey,  // SECURITY RISK!
})
```

## Key Rotation

### Rotation Best Practices

1. **Rotate keys regularly** (every 90 days recommended)
2. **Rotate immediately** after:
   - Suspected compromise
   - Employee offboarding
   - Log exposure incidents
   - Security audit findings
3. **Use graceful rotation**:
   - Generate new key
   - Update configuration
   - Test with new key
   - Revoke old key

### Rotation Procedure

1. Generate new bouncer in CrowdSec:
   ```bash
   cscli bouncers add new-bouncer-name
   ```

2. Update Charon configuration:
   ```bash
   # Update environment variable
   CHARON_SECURITY_CROWDSEC_API_KEY=new-key-here

   # Or update key file
   echo "new-key-here" > /path/to/bouncer.key
   chmod 0600 /path/to/bouncer.key
   ```

3. Restart Charon to apply new key

4. Revoke old bouncer:
   ```bash
   cscli bouncers delete old-bouncer-name
   ```

## Incident Response

### If Keys Are Exposed

If API keys are accidentally logged or exposed:

1. **Rotate the key immediately** (see rotation procedure above)
2. **Purge logs** containing the exposed key:
   - Local log files
   - Log aggregation services (CloudWatch, Splunk, etc.)
   - Backup archives
3. **Audit access logs** for unauthorized usage
4. **Update incident response procedures** to prevent recurrence
5. **Notify security team** following your organization's procedures

### Log Audit Procedure

To check if keys were exposed in logs:

```bash
# Search local logs (should find NO matches)
grep -r "full-api-key-pattern" /var/log/charon/

# Expected: No results (keys should be masked)

# If matches found, keys were exposed - follow incident response
```

## Compliance

### Standards Addressed

This implementation addresses:

- **CWE-312**: Cleartext Storage of Sensitive Information
- **CWE-315**: Cleartext Storage in Cookie
- **CWE-359**: Exposure of Private Personal Information
- **OWASP A02:2021**: Cryptographic Failures

### Compliance Frameworks

Key handling practices align with:

- **GDPR**: Personal data protection (Article 32)
- **PCI-DSS**: Requirement 3.4 (Render PAN unreadable)
- **SOC 2**: Security criteria (CC6.1 - Logical access controls)
- **ISO 27001**: A.9.4.3 (Password management)

## Testing

### Security Test Coverage

All API key handling functions have comprehensive unit tests:

```bash
# Run security tests
go test ./backend/internal/api/handlers -run TestMaskAPIKey -v
go test ./backend/internal/api/handlers -run TestValidateAPIKeyFormat -v
go test ./backend/internal/api/handlers -run TestSaveKeyToFile_SecurePermissions -v
```

### Test Scenarios

Tests cover:
- ✅ Empty keys → `[empty]`
- ✅ Short keys (< 16) → `[REDACTED]`
- ✅ Normal keys → `abcd...xyz9`
- ✅ Length validation (16-128 chars)
- ✅ Character set validation
- ✅ File permissions (0600)
- ✅ No full key exposure in logs

## References

- [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)
- [CWE-312: Cleartext Storage](https://cwe.mitre.org/data/definitions/312.html)
- [CWE-315: Cookie Storage](https://cwe.mitre.org/data/definitions/315.html)
- [CWE-359: Privacy Exposure](https://cwe.mitre.org/data/definitions/359.html)
- [NIST SP 800-63B: Digital Identity Guidelines](https://pages.nist.gov/800-63-3/sp800-63b.html)

## Updates

| Date | Change | Author |
|------|--------|--------|
| 2026-02-03 | Initial documentation for Sprint 0 security fix | GitHub Copilot |

---

**Last Updated**: 2026-02-03
**Next Review**: 2026-05-03 (Quarterly)
